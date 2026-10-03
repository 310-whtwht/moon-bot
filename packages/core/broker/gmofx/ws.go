package gmofx

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
	"github.com/moomoo-trading/core/market"
)

const (
	// subscribeInterval respects the documented 1 subscribe/s limit per IP.
	subscribeInterval = 1100 * time.Millisecond
	// readTimeout: the server pings every minute and drops clients after 3 missed pongs.
	readTimeout       = 3 * time.Minute
	maxReconnectDelay = 30 * time.Second
)

type subscribeMessage struct {
	Command string `json:"command"`
	Channel string `json:"channel"`
	Symbol  string `json:"symbol"`
}

// SubscribeTicks streams ticker updates for symbols, reconnecting and
// re-subscribing with backoff until ctx is cancelled.
func (c *Client) SubscribeTicks(ctx context.Context, symbols []string) (<-chan market.Tick, error) {
	out := make(chan market.Tick, 256)

	go func() {
		defer close(out)
		delay := time.Second
		for {
			connected, err := c.streamTicks(ctx, symbols, out)
			if ctx.Err() != nil {
				return
			}
			if connected {
				delay = time.Second
			}
			log.Printf("gmofx: ticker stream ended (%v); reconnecting in %s", err, delay)

			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			delay *= 2
			if delay > maxReconnectDelay {
				delay = maxReconnectDelay
			}
		}
	}()

	return out, nil
}

// streamTicks runs one connection. connected reports whether the dial and
// subscriptions succeeded, which resets the reconnect backoff.
func (c *Client) streamTicks(ctx context.Context, symbols []string, out chan<- market.Tick) (connected bool, err error) {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, c.publicWSURL, nil)
	if err != nil {
		return false, err
	}
	defer conn.Close()

	// Unblock ReadMessage when ctx is cancelled.
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()

	extendDeadline := func() { _ = conn.SetReadDeadline(time.Now().Add(readTimeout)) }
	extendDeadline()
	conn.SetPingHandler(func(data string) error {
		extendDeadline()
		return conn.WriteControl(websocket.PongMessage, []byte(data), time.Now().Add(10*time.Second))
	})

	for i, symbol := range symbols {
		if i > 0 {
			select {
			case <-ctx.Done():
				return false, ctx.Err()
			case <-time.After(subscribeInterval):
			}
		}
		msg := subscribeMessage{Command: "subscribe", Channel: "ticker", Symbol: symbol}
		if err := conn.WriteJSON(msg); err != nil {
			return false, err
		}
	}

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return true, err
		}
		extendDeadline()

		var p tickerPayload
		if err := json.Unmarshal(data, &p); err != nil || p.Symbol == "" {
			// Non-ticker frames (errors, acks) are logged and skipped.
			log.Printf("gmofx: ignoring ws message: %s", data)
			continue
		}

		select {
		case out <- p.tick():
		case <-ctx.Done():
			return true, ctx.Err()
		}
	}
}
