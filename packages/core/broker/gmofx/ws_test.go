package gmofx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubscribeTicks_DeliversAndReconnects(t *testing.T) {
	var connections atomic.Int32
	upgrader := websocket.Upgrader{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()
		n := connections.Add(1)

		var sub subscribeMessage
		if err := conn.ReadJSON(&sub); err != nil {
			t.Errorf("read subscribe: %v", err)
			return
		}
		assert.Equal(t, subscribeMessage{Command: "subscribe", Channel: "ticker", Symbol: "USD_JPY"}, sub)

		_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"ignored non-ticker frame"}`))
		bid := "157.80" + string(rune('0'+n))
		_ = conn.WriteMessage(websocket.TextMessage,
			[]byte(`{"symbol":"USD_JPY","ask":"157.9","bid":"`+bid+`","timestamp":"2026-10-05T00:00:00.000Z","status":"OPEN"}`))

		if n == 1 {
			return // drop the first connection to force a reconnect
		}
		// Keep the second connection open until the client goes away.
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	c := New(Options{PublicWSURL: "ws" + strings.TrimPrefix(srv.URL, "http")})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ticks, err := c.SubscribeTicks(ctx, []string{"USD_JPY"})
	require.NoError(t, err)

	first := <-ticks
	assert.Equal(t, "gmo:USD_JPY", first.Key.String())
	assert.Equal(t, "157.801", first.Bid.String())

	second := <-ticks
	assert.Equal(t, "157.802", second.Bid.String())
	assert.Equal(t, int32(2), connections.Load())

	cancel()
	for range ticks {
		// drain until the subscription closes the channel
	}
}
