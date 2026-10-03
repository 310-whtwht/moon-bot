package gmofx

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/moomoo-trading/core/broker"
	"github.com/moomoo-trading/core/market"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return New(Options{PublicURL: srv.URL, MinInterval: time.Millisecond})
}

func TestStatus(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/status", r.URL.Path)
		fmt.Fprint(w, `{"status":0,"data":{"status":"OPEN"},"responsetime":"2026-10-03T00:00:00.000Z"}`)
	})

	status, err := c.Status(context.Background())
	require.NoError(t, err)
	assert.Equal(t, market.StatusOpen, status)
}

func TestInstruments(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"status":0,"data":[{"symbol":"USD_JPY","tickSize":"0.001","minOpenOrderSize":"100","maxOrderSize":"500000","sizeStep":"1"}]}`)
	})

	instruments, err := c.Instruments(context.Background())
	require.NoError(t, err)
	require.Len(t, instruments, 1)
	in := instruments[0]
	assert.Equal(t, "gmo:USD_JPY", in.Key.String())
	assert.Equal(t, "JPY", in.QuoteCurrency)
	assert.Equal(t, "100", in.MinOrderSize.String())
	assert.Equal(t, "0.001", in.TickSize.String())
}

func TestTicks(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"status":0,"data":[{"symbol":"USD_JPY","ask":"157.907","bid":"157.807","timestamp":"2026-10-03T02:55:47.951776Z","status":"CLOSE"}]}`)
	})

	ticks, err := c.Ticks(context.Background())
	require.NoError(t, err)
	require.Len(t, ticks, 1)
	assert.Equal(t, "157.807", ticks[0].Bid.String())
	assert.Equal(t, "157.907", ticks[0].Ask.String())
	assert.Equal(t, market.StatusClosed, ticks[0].Status)
}

func TestAPIErrorEnvelope(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"status":5,"messages":[{"message_code":"ERR-5201","message_string":"MAINTENANCE"}]}`)
	})

	_, err := c.Status(context.Background())
	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, "ERR-5201", apiErr.Code)
	assert.Equal(t, "MAINTENANCE", apiErr.Message)
}

func TestTradingDate(t *testing.T) {
	cases := []struct {
		name string
		at   time.Time
		want string
	}{
		{"summer day start (06:00 JST)", time.Date(2026, 9, 30, 21, 0, 0, 0, time.UTC), "2026-10-01"},
		{"summer last bar of previous day", time.Date(2026, 9, 30, 20, 0, 0, 0, time.UTC), "2026-09-30"},
		{"winter day start (06:00 JST)", time.Date(2026, 1, 14, 21, 0, 0, 0, time.UTC), "2026-01-15"},
		{"winter last bar of previous day", time.Date(2026, 1, 14, 20, 0, 0, 0, time.UTC), "2026-01-14"},
		{"winter weekly open (07:00 JST)", time.Date(2025, 11, 2, 22, 0, 0, 0, time.UTC), "2025-11-03"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, TradingDate(tc.at).Format("2006-01-02"))
		})
	}
}

// klineJSON renders one kline with openTime in epoch milliseconds.
func klineJSON(open time.Time, price string) string {
	return fmt.Sprintf(`{"openTime":"%d","open":"%s","high":"%s","low":"%s","close":"%s"}`,
		open.UnixMilli(), price, price, price, price)
}

func TestBars_FetchesEachTradingDateAndFilters(t *testing.T) {
	d1 := time.Date(2026, 9, 30, 21, 0, 0, 0, time.UTC) // first bar of 2026-10-01
	var mu sync.Mutex
	var dates []string

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		assert.Equal(t, "/v1/klines", r.URL.Path)
		assert.Equal(t, "USD_JPY", q.Get("symbol"))
		assert.Equal(t, "ASK", q.Get("priceType"))
		assert.Equal(t, "1hour", q.Get("interval"))
		mu.Lock()
		dates = append(dates, q.Get("date"))
		mu.Unlock()

		switch q.Get("date") {
		case "20261001":
			fmt.Fprintf(w, `{"status":0,"data":[%s,%s]}`, klineJSON(d1, "157.1"), klineJSON(d1.Add(time.Hour), "157.2"))
		case "20261002":
			fmt.Fprintf(w, `{"status":0,"data":[%s]}`, klineJSON(d1.Add(24*time.Hour), "157.3"))
		default:
			t.Errorf("unexpected date %s", q.Get("date"))
		}
	})

	bars, err := c.Bars(context.Background(), broker.BarsRequest{
		Symbol:    "USD_JPY",
		Timeframe: market.TF1Hour,
		PriceType: market.PriceAsk,
		From:      d1.Add(time.Hour), // excludes the first bar
		To:        d1.Add(25 * time.Hour),
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"20261001", "20261002"}, dates)
	require.Len(t, bars, 2)
	assert.Equal(t, "157.2", bars[0].Close.String())
	assert.Equal(t, "157.3", bars[1].Close.String())
	assert.Equal(t, d1.Add(24*time.Hour), bars[1].OpenTime)
	assert.Equal(t, "gmo:USD_JPY", bars[1].Key.String())
}

func TestBars_YearlyParamForFourHourAndAbove(t *testing.T) {
	var got []string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.URL.Query().Get("date"))
		fmt.Fprintf(w, `{"status":0,"data":[%s]}`, klineJSON(time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC), "150"))
	})

	_, err := c.Bars(context.Background(), broker.BarsRequest{
		Symbol: "USD_JPY", Timeframe: market.TF4Hour, PriceType: market.PriceBid,
		From: time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"2025", "2026"}, got)
}

func TestBars_NoTradingIsNoData(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("date") == "20261004" {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"status_code": 404, "message": "Not found"}`)
			return
		}
		fmt.Fprint(w, `{"status":0,"data":[]}`)
	})

	_, err := c.Bars(context.Background(), broker.BarsRequest{
		Symbol: "USD_JPY", Timeframe: market.TF1Hour, PriceType: market.PriceBid,
		From: time.Date(2026, 10, 2, 21, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 10, 4, 21, 0, 0, 0, time.UTC),
	})
	assert.ErrorIs(t, err, broker.ErrNoData)
}

func TestBars_RejectsUnsupportedInput(t *testing.T) {
	c := New(Options{})
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

	_, err := c.Bars(context.Background(), broker.BarsRequest{Symbol: "USD_JPY", Timeframe: "2h", PriceType: market.PriceBid, From: from, To: from.Add(time.Hour)})
	assert.Error(t, err)
	_, err = c.Bars(context.Background(), broker.BarsRequest{Symbol: "USD_JPY", Timeframe: market.TF1Hour, PriceType: "MID", From: from, To: from.Add(time.Hour)})
	assert.Error(t, err)
	_, err = c.Bars(context.Background(), broker.BarsRequest{Symbol: "USD_JPY", Timeframe: market.TF1Hour, PriceType: market.PriceBid, From: from, To: from})
	assert.Error(t, err)
}

func TestThrottleSpacesRequests(t *testing.T) {
	var mu sync.Mutex
	var times []time.Time
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		times = append(times, time.Now())
		mu.Unlock()
		fmt.Fprint(w, `{"status":0,"data":{"status":"OPEN"}}`)
	}))
	defer srv.Close()

	c := New(Options{PublicURL: srv.URL, MinInterval: 50 * time.Millisecond})
	for i := 0; i < 3; i++ {
		_, err := c.Status(context.Background())
		require.NoError(t, err)
	}

	require.Len(t, times, 3)
	assert.GreaterOrEqual(t, times[2].Sub(times[0]), 95*time.Millisecond)
}
