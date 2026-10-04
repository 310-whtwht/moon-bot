package gmofx

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSignedBody_OnlyPostSignsItsBody(t *testing.T) {
	assert.Equal(t, `{"a":1}`, signedBody(http.MethodPost, `{"a":1}`))
	assert.Equal(t, "", signedBody(http.MethodPut, `{"token":"x"}`), "as in the official ws-auth examples")
	assert.Equal(t, "", signedBody(http.MethodDelete, `{"token":"x"}`))
	assert.Equal(t, "", signedBody(http.MethodGet, ""))
}

const executionEventJSON = `{"channel":"executionEvents","amount":"0","rootOrderId":123456789,"orderId":123456789,"clientOrderId":"abc123","executionId":72123911,"symbol":"USD_JPY","settleType":"CLOSE","orderType":"NORMAL","executionType":"STOP","side":"SELL","executionPrice":"138.963","executionSize":"100","positionId":555,"lossGain":"-104.7","settledSwap":"0","fee":"-0.3","orderPrice":"139","orderExecutedSize":"100","orderSize":"100","msgType":"ER","orderTimestamp":"2019-03-19T02:15:06.081Z","executionTimestamp":"2019-03-19T02:15:06.081Z"}`

// wsHarness is a fake private API (token calls) plus a fake private WebSocket.
type wsHarness struct {
	mu          sync.Mutex
	issued      []string // tokens handed out
	connected   []string // tokens used to connect
	subscribes  []map[string]string
	extended    []string
	deleted     []string
	dropFirst   bool
	connections int
}

func (h *wsHarness) snapshot() (issued, connected, extended, deleted []string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.issued...), append([]string(nil), h.connected...),
		append([]string(nil), h.extended...), append([]string(nil), h.deleted...)
}

func newWSHarness(t *testing.T, extendEvery time.Duration) (*wsHarness, *Private) {
	h := &wsHarness{}
	upgrader := websocket.Upgrader{}

	ws := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.URL.Path, "/ws/private/v1/")
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()

		var sub map[string]string
		if err := conn.ReadJSON(&sub); err != nil {
			return
		}
		h.mu.Lock()
		h.connected = append(h.connected, token)
		h.subscribes = append(h.subscribes, sub)
		h.connections++
		n, drop := h.connections, h.dropFirst
		h.mu.Unlock()

		_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"not an execution"}`))
		_ = conn.WriteMessage(websocket.TextMessage, []byte(executionEventJSON))
		if n == 1 && drop {
			return // server drops the first connection
		}
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	t.Cleanup(ws.Close)

	f, _ := newFakeAPI(t)
	tokenBody := func(r recorded) string {
		var b struct {
			Token string `json:"token"`
		}
		_ = json.Unmarshal([]byte(r.Body), &b)
		return b.Token
	}
	f.handlers["POST /v1/ws-auth"] = func(n int, w http.ResponseWriter, _ recorded) {
		token := fmt.Sprintf("token-%d", n)
		h.mu.Lock()
		h.issued = append(h.issued, token)
		h.mu.Unlock()
		fmt.Fprintf(w, `{"status":0,"data":%q}`, token)
	}
	f.handlers["PUT /v1/ws-auth"] = func(_ int, w http.ResponseWriter, r recorded) {
		h.mu.Lock()
		h.extended = append(h.extended, tokenBody(r))
		h.mu.Unlock()
		fmt.Fprint(w, `{"status":0}`)
	}
	f.handlers["DELETE /v1/ws-auth"] = func(_ int, w http.ResponseWriter, r recorded) {
		h.mu.Lock()
		h.deleted = append(h.deleted, tokenBody(r))
		h.mu.Unlock()
		fmt.Fprint(w, `{"status":0}`)
	}

	// A client pointed at the fake API and the fake WebSocket.
	p, err := NewPrivate(PrivateOptions{
		Options: Options{MinInterval: time.Millisecond},
		APIKey:  testKey, APISecret: testSecret, PrivateURL: f.url,
		PrivateWSURL:        "ws" + strings.TrimPrefix(ws.URL, "http") + "/ws/private/v1",
		TokenExtendInterval: extendEvery,
		Now:                 func() time.Time { return fixedNow },
	})
	require.NoError(t, err)
	return h, p
}

func TestSubscribeExecutions_TokenLifecycleAndEvents(t *testing.T) {
	h, p := newWSHarness(t, 40*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events, err := p.SubscribeExecutions(ctx)
	require.NoError(t, err)

	var got = <-events
	assert.Equal(t, "72123911", got.ExecutionID)
	assert.Equal(t, "123456789", got.OrderID)
	assert.Equal(t, "555", got.PositionID)
	assert.Equal(t, "USD_JPY", got.Symbol)
	assert.Equal(t, "CLOSE", got.SettleType)
	assert.Equal(t, "138.963", got.Price.String())
	assert.Equal(t, "100", got.Size.String())
	assert.Equal(t, "-104.7", got.RealizedPnL.String())
	assert.Equal(t, "0.3", got.Fee.String(), "fee stored as a positive cost")

	issued, connected, _, _ := h.snapshot()
	assert.Equal(t, []string{"token-1"}, issued)
	assert.Equal(t, []string{"token-1"}, connected, "the token is the last path segment of the URL")
	h.mu.Lock()
	assert.Equal(t, map[string]string{"command": "subscribe", "channel": "executionEvents"}, h.subscribes[0])
	h.mu.Unlock()

	require.Eventually(t, func() bool {
		_, _, extended, _ := h.snapshot()
		return len(extended) >= 2
	}, 2*time.Second, 10*time.Millisecond, "the token is extended on an interval")
	_, _, extended, _ := h.snapshot()
	assert.Equal(t, "token-1", extended[0])

	cancel()
	for range events {
		// drain until the stream closes
	}
	require.Eventually(t, func() bool {
		_, _, _, deleted := h.snapshot()
		return len(deleted) == 1
	}, 2*time.Second, 10*time.Millisecond, "the token is deleted on shutdown")
	_, _, _, deleted := h.snapshot()
	assert.Equal(t, []string{"token-1"}, deleted)
}

func TestSubscribeExecutions_ReconnectsWithANewToken(t *testing.T) {
	h, p := newWSHarness(t, time.Hour)
	h.dropFirst = true
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	events, err := p.SubscribeExecutions(ctx)
	require.NoError(t, err)

	<-events // from the first connection, which the server then drops
	<-events // from the second connection

	issued, connected, _, deleted := h.snapshot()
	assert.Equal(t, []string{"token-1", "token-2"}, issued)
	assert.Equal(t, []string{"token-1", "token-2"}, connected)
	assert.Contains(t, deleted, "token-1", "the dropped connection's token is given back")
}
