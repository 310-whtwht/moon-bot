package gmofx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/moomoo-trading/core/broker"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testKey    = "test-key"
	testSecret = "test-secret"
)

var fixedNow = time.UnixMilli(1700000000000)

// recorded is one request seen by the fake API.
type recorded struct {
	Method, Path, Query, Body string
}

// fakeAPI verifies the signature of every request and answers from handlers
// keyed by "METHOD /path".
type fakeAPI struct {
	t        *testing.T
	mu       sync.Mutex
	requests []recorded
	handlers map[string]func(n int, w http.ResponseWriter, r recorded)
	counts   map[string]int
}

func newFakeAPI(t *testing.T) (*fakeAPI, *Private) {
	f := &fakeAPI{t: t, handlers: map[string]func(int, http.ResponseWriter, recorded){}, counts: map[string]int{}}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)

	p, err := NewPrivate(PrivateOptions{
		Options:    Options{MinInterval: time.Millisecond},
		APIKey:     testKey,
		APISecret:  testSecret,
		PrivateURL: srv.URL,
		AccountID:  "gmo-test",
		Now:        func() time.Time { return fixedNow },
	})
	require.NoError(t, err)
	return f, p
}

func (f *fakeAPI) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	rec := recorded{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Body: string(body)}

	// Every request must be signed over timestamp + method + path + body (no query).
	ts := r.Header.Get("API-TIMESTAMP")
	assert.Equal(f.t, testKey, r.Header.Get("API-KEY"))
	assert.Equal(f.t, Sign([]byte(testSecret), ts, r.Method, r.URL.Path, string(body)), r.Header.Get("API-SIGN"),
		"signature of %s %s", r.Method, r.URL.Path)

	key := r.Method + " " + r.URL.Path
	f.mu.Lock()
	f.requests = append(f.requests, rec)
	f.counts[key]++
	n := f.counts[key]
	h := f.handlers[key]
	f.mu.Unlock()

	if h == nil {
		f.t.Errorf("unexpected request %s", key)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	h(n, w, rec)
}

func (f *fakeAPI) on(key, response string) {
	f.handlers[key] = func(_ int, w http.ResponseWriter, _ recorded) { fmt.Fprint(w, response) }
}

func (f *fakeAPI) last() recorded {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.requests[len(f.requests)-1]
}

func (f *fakeAPI) count(key string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.counts[key]
}

func dec(s string) decimal.Decimal { return decimal.RequireFromString(s) }

func TestSign_MatchesReferenceValues(t *testing.T) {
	// Reference values computed independently with:
	//   printf '%s' '<text>' | openssl dgst -sha256 -hmac 'test-secret'
	assert.Equal(t, "8f56ba0a6e1c4d16d2d66082badb8f8743d8eca5a7190f24dda0d2a7c9058f7f",
		Sign([]byte(testSecret), "1700000000000", "POST", "/v1/order", `{"symbol":"USD_JPY"}`))
	assert.Equal(t, "983ff1a7fcefd50ecaf3a6efcd6a6bb2856ffb0c1f261c3d97a4e88923c3e1da",
		Sign([]byte(testSecret), "1700000000000", "GET", "/v1/account/assets", ""))
}

func TestClientOrderID(t *testing.T) {
	assert.Equal(t, "dddddddd1791194400open", ClientOrderID("dddddddd-1791194400-open"))
	assert.Equal(t, "", ClientOrderID("---"))
	long := ClientOrderID("dddddddd-retry-aaaaaaaa-29853240-close-and-more-text")
	assert.Len(t, long, 36)
	assert.Equal(t, "aaaaa29853240closeandmoretext", long[len(long)-29:], "the tail is kept")
}

func TestNewPrivate_RequiresCredentials(t *testing.T) {
	_, err := NewPrivate(PrivateOptions{APIKey: "k"})
	assert.Error(t, err)
}

func TestAssetsAndPositions(t *testing.T) {
	f, p := newFakeAPI(t)
	f.on("GET /v1/account/assets", `{"status":0,"data":[{"equity":"120947776","availableAmount":"89717102","balance":"116884885"}],"responsetime":"2019-03-19T02:15:06.055Z"}`)
	f.on("GET /v1/openPositions", `{"status":0,"data":{"list":[{"positionId":123456789,"symbol":"USD_JPY","side":"BUY","size":"10000","orderedSize":"0","price":"141.269","lossGain":"-1980","totalSwap":"0","timestamp":"2019-03-21T05:18:09.011Z"}]}}`)

	assets, err := p.Assets(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "120947776", assets.Equity.String())
	assert.Equal(t, "89717102", assets.AvailableMargin.String())
	assert.Equal(t, "gmo-test", p.AccountID())
	assert.Equal(t, "gmo", p.Name())

	positions, err := p.OpenPositions(context.Background(), "USD_JPY")
	require.NoError(t, err)
	assert.Equal(t, "symbol=USD_JPY", f.last().Query, "the query is sent but not signed")
	require.Len(t, positions, 1)
	assert.Equal(t, broker.Position{PositionID: "123456789", Symbol: "USD_JPY", Side: broker.SideBuy,
		Size: dec("10000"), OpenPrice: dec("141.269"), OpenedAt: time.Date(2019, 3, 21, 5, 18, 9, 11e6, time.UTC)}, positions[0])
}

func TestPlaceOpen_MarketOrder(t *testing.T) {
	f, p := newFakeAPI(t)
	f.on("POST /v1/order", `{"status":0,"data":[{"rootOrderId":123456789,"clientOrderId":"dddddddd1791194400open","orderId":123456789,"symbol":"USD_JPY","side":"BUY","orderType":"NORMAL","executionType":"MARKET","settleType":"OPEN","size":"100","status":"EXECUTED","timestamp":"2019-03-19T02:15:06.059Z"}]}`)

	ack, err := p.PlaceOpen(context.Background(), broker.OpenOrder{
		ClientOrderID: "dddddddd-1791194400-open", Symbol: "USD_JPY", Side: broker.SideBuy,
		Type: broker.OrderMarket, Size: dec("100"),
	})
	require.NoError(t, err)
	assert.Equal(t, "123456789", ack.OrderID)
	assert.Equal(t, "EXECUTED", ack.Status)

	var sent map[string]any
	require.NoError(t, json.Unmarshal([]byte(f.last().Body), &sent))
	assert.Equal(t, map[string]any{"symbol": "USD_JPY", "side": "BUY", "size": "100",
		"executionType": "MARKET", "clientOrderId": "dddddddd1791194400open"}, sent)
}

func TestPlaceClose_SettlesOnePosition(t *testing.T) {
	f, p := newFakeAPI(t)
	f.on("POST /v1/closeOrder", `{"status":0,"data":[{"rootOrderId":223456789,"orderId":223456789,"symbol":"USD_JPY","side":"SELL","settleType":"CLOSE","size":"100","status":"EXECUTED","timestamp":"2019-03-19T01:07:24.467Z"}]}`)

	stop := dec("149.5")
	_, err := p.PlaceClose(context.Background(), broker.CloseOrder{
		ClientOrderID: "x-1-close", Symbol: "USD_JPY", PositionID: "1000342", Side: broker.SideSell,
		Type: broker.OrderStop, Size: dec("100"), Price: &stop,
	})
	require.NoError(t, err)

	var sent map[string]any
	require.NoError(t, json.Unmarshal([]byte(f.last().Body), &sent))
	assert.Equal(t, "STOP", sent["executionType"])
	assert.Equal(t, "149.5", sent["stopPrice"])
	assert.Equal(t, "x1close", sent["clientOrderId"])
	assert.Equal(t, []any{map[string]any{"positionId": float64(1000342), "size": "100"}}, sent["settlePosition"],
		"positionId is a number, size a string")

	_, err = p.PlaceClose(context.Background(), broker.CloseOrder{PositionID: "paper-1", Type: broker.OrderMarket})
	assert.Error(t, err, "non-numeric position id")
	_, err = p.PlaceOpen(context.Background(), broker.OpenOrder{Type: broker.OrderLimit, Size: dec("100")})
	assert.Error(t, err, "limit without price")
}

func TestCancel(t *testing.T) {
	f, p := newFakeAPI(t)
	f.on("POST /v1/cancelOrders", `{"status":0,"data":{"success":[{"rootOrderId":123456789}]}}`)

	require.NoError(t, p.Cancel(context.Background(), "123456789"))
	assert.JSONEq(t, `{"rootOrderIds":[123456789]}`, f.last().Body)
	assert.Error(t, p.Cancel(context.Background(), "abc"))
}

const executionsJSON = `{"status":0,"data":{"list":[
 {"amount":"16215.999","executionId":92123912,"clientOrderId":"aaaaa","orderId":223456789,"positionId":2234567,"symbol":"USD_JPY","side":"SELL","settleType":"CLOSE","size":"10000","price":"141.251","lossGain":"15730","fee":"-30","settledSwap":"515.999","timestamp":"2020-11-24T21:27:04.764Z"},
 {"amount":"0","executionId":72123911,"clientOrderId":"bbbbb","orderId":123456789,"positionId":1234567,"symbol":"USD_JPY","side":"BUY","settleType":"OPEN","size":"10000","price":"141.269","lossGain":"0","fee":"0","settledSwap":"0","timestamp":"2020-11-24T19:47:51.234Z"}]}}`

func TestExecutions(t *testing.T) {
	f, p := newFakeAPI(t)
	f.on("GET /v1/executions", executionsJSON)

	execs, err := p.Executions(context.Background(), "223456789")
	require.NoError(t, err)
	assert.Equal(t, "orderId=223456789", f.last().Query)
	require.Len(t, execs, 2)
	e := execs[0]
	assert.Equal(t, "92123912", e.ExecutionID)
	assert.Equal(t, "2234567", e.PositionID)
	assert.Equal(t, "CLOSE", e.SettleType)
	assert.Equal(t, "141.251", e.Price.String())
	assert.Equal(t, "15730", e.RealizedPnL.String())
	assert.Equal(t, "30", e.Fee.String(), "fee is stored as a positive cost")
}

func TestFindOrder(t *testing.T) {
	f, p := newFakeAPI(t)
	f.on("GET /v1/latestExecutions", executionsJSON)
	f.on("GET /v1/activeOrders", `{"status":0,"data":{"list":[{"orderId":5,"clientOrderId":"working1","status":"ORDERED"}]}}`)

	fills, found, err := p.FindOrder(context.Background(), "USD_JPY", "bb-bbb") // sanitised to bbbbb
	require.NoError(t, err)
	assert.True(t, found)
	require.Len(t, fills, 1)
	assert.Equal(t, "1234567", fills[0].PositionID)
	assert.Equal(t, 0, f.count("GET /v1/activeOrders"), "no need to look further once fills are found")

	fills, found, err = p.FindOrder(context.Background(), "USD_JPY", "working-1")
	require.NoError(t, err)
	assert.True(t, found, "accepted and still working")
	assert.Empty(t, fills)

	_, found, err = p.FindOrder(context.Background(), "USD_JPY", "never-sent")
	require.NoError(t, err)
	assert.False(t, found)

	_, _, err = p.FindOrder(context.Background(), "USD_JPY", "--")
	assert.Error(t, err)
}

func TestAPIErrorIsReturnedWithCode(t *testing.T) {
	f, p := newFakeAPI(t)
	f.on("POST /v1/order", `{"status":1,"messages":[{"message_code":"ERR-201","message_string":"Trading margin is insufficient"}],"responsetime":"2019-03-19T02:15:06.059Z"}`)

	_, err := p.PlaceOpen(context.Background(), broker.OpenOrder{Symbol: "USD_JPY", Side: broker.SideBuy, Type: broker.OrderMarket, Size: dec("100")})
	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, "ERR-201", apiErr.Code)
	assert.NotErrorIs(t, err, broker.ErrUnknownResult, "a clear rejection is not an unknown result")
	assert.Equal(t, 1, f.count("POST /v1/order"), "orders are never retried")
}

func TestClockSkewIsCorrectedAndRetriedOnce(t *testing.T) {
	f, p := newFakeAPI(t)
	server := fixedNow.Add(90 * time.Second).UTC()
	f.handlers["GET /v1/account/assets"] = func(n int, w http.ResponseWriter, _ recorded) {
		if n == 1 {
			fmt.Fprintf(w, `{"status":5,"messages":[{"message_code":"ERR-5008","message_string":"timestamp too late"}],"responsetime":%q}`,
				server.Format(time.RFC3339Nano))
			return
		}
		fmt.Fprint(w, `{"status":0,"data":[{"equity":"1","availableAmount":"1"}]}`)
	}

	_, err := p.Assets(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, f.count("GET /v1/account/assets"))
	assert.Equal(t, 90*time.Second, p.clockOffset, "offset learned from the server's response time")
	assert.Equal(t, fmt.Sprint(server.UnixMilli()), p.timestamp(), "later requests use the corrected clock")
}

func TestRateLimit_RetriesGetButNeverPost(t *testing.T) {
	f, p := newFakeAPI(t)
	limited := `{"status":1,"messages":[{"message_code":"ERR-5003","message_string":"Requests are too many."}]}`
	f.handlers["GET /v1/account/assets"] = func(n int, w http.ResponseWriter, _ recorded) {
		if n == 1 {
			fmt.Fprint(w, limited)
			return
		}
		fmt.Fprint(w, `{"status":0,"data":[{"equity":"1","availableAmount":"1"}]}`)
	}
	f.on("POST /v1/order", limited)

	_, err := p.Assets(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, f.count("GET /v1/account/assets"))

	_, err = p.PlaceOpen(context.Background(), broker.OpenOrder{Symbol: "USD_JPY", Side: broker.SideBuy, Type: broker.OrderMarket, Size: dec("100")})
	assert.Error(t, err)
	assert.Equal(t, 1, f.count("POST /v1/order"))
}

func TestOrderWithoutAnswerIsUnknownResult(t *testing.T) {
	order := broker.OpenOrder{Symbol: "USD_JPY", Side: broker.SideBuy, Type: broker.OrderMarket, Size: dec("100")}

	t.Run("server error", func(t *testing.T) {
		f, p := newFakeAPI(t)
		f.handlers["POST /v1/order"] = func(_ int, w http.ResponseWriter, _ recorded) { w.WriteHeader(http.StatusBadGateway) }
		_, err := p.PlaceOpen(context.Background(), order)
		assert.ErrorIs(t, err, broker.ErrUnknownResult)
		assert.Equal(t, 1, f.count("POST /v1/order"))
	})

	t.Run("timeout", func(t *testing.T) {
		f, p := newFakeAPI(t)
		f.handlers["POST /v1/order"] = func(_ int, w http.ResponseWriter, _ recorded) {
			time.Sleep(300 * time.Millisecond)
			fmt.Fprint(w, `{"status":0,"data":[{"orderId":1}]}`)
		}
		p.http = &http.Client{Timeout: 50 * time.Millisecond}
		_, err := p.PlaceOpen(context.Background(), order)
		assert.ErrorIs(t, err, broker.ErrUnknownResult)
	})

	t.Run("a failed read-only call is a plain error", func(t *testing.T) {
		f, p := newFakeAPI(t)
		f.handlers["GET /v1/account/assets"] = func(_ int, w http.ResponseWriter, _ recorded) { w.WriteHeader(http.StatusBadGateway) }
		_, err := p.Assets(context.Background())
		require.Error(t, err)
		assert.NotErrorIs(t, err, broker.ErrUnknownResult)
	})
}

func TestSubscribeExecutionsNotYetSupported(t *testing.T) {
	_, p := newFakeAPI(t)
	_, err := p.SubscribeExecutions(context.Background())
	assert.ErrorIs(t, err, broker.ErrNotSupported)
}
