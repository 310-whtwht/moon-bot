package gmofx

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/moomoo-trading/core/broker"
	"github.com/shopspring/decimal"
)

const (
	DefaultPrivateWSURL = "wss://forex-api.coin.z.com/ws/private/v1"

	// Access tokens live 60 minutes; extending resets that to 60 minutes.
	defaultTokenExtendInterval = 30 * time.Minute
)

// wsToken gets an access token for the private WebSocket (POST /v1/ws-auth).
func (p *Private) wsToken(ctx context.Context) (string, error) {
	var token string
	if err := p.call(ctx, http.MethodPost, "/v1/ws-auth", nil, map[string]any{}, &token); err != nil {
		return "", err
	}
	if token == "" {
		return "", errors.New("gmofx: empty websocket token")
	}
	return token, nil
}

func (p *Private) extendWSToken(ctx context.Context, token string) error {
	return p.call(ctx, http.MethodPut, "/v1/ws-auth", nil, map[string]any{"token": token}, nil)
}

func (p *Private) deleteWSToken(ctx context.Context, token string) error {
	return p.call(ctx, http.MethodDelete, "/v1/ws-auth", nil, map[string]any{"token": token}, nil)
}

// executionEvent is a message on the executionEvents channel.
type executionEvent struct {
	Channel            string          `json:"channel"`
	OrderID            int64           `json:"orderId"`
	ExecutionID        int64           `json:"executionId"`
	PositionID         int64           `json:"positionId"`
	Symbol             string          `json:"symbol"`
	Side               string          `json:"side"`
	SettleType         string          `json:"settleType"`
	ExecutionPrice     decimal.Decimal `json:"executionPrice"`
	ExecutionSize      decimal.Decimal `json:"executionSize"`
	LossGain           decimal.Decimal `json:"lossGain"`
	Fee                decimal.Decimal `json:"fee"`
	ExecutionTimestamp time.Time       `json:"executionTimestamp"`
}

func (e executionEvent) execution() broker.Execution {
	return broker.Execution{
		ExecutionID: strconv.FormatInt(e.ExecutionID, 10),
		OrderID:     strconv.FormatInt(e.OrderID, 10),
		PositionID:  strconv.FormatInt(e.PositionID, 10),
		Symbol:      e.Symbol, Side: broker.Side(e.Side), SettleType: e.SettleType,
		Size: e.ExecutionSize, Price: e.ExecutionPrice,
		Fee: e.Fee.Abs(), RealizedPnL: e.LossGain,
		ExecutedAt: e.ExecutionTimestamp.UTC(),
	}
}

// SubscribeExecutions streams fills from the private WebSocket until ctx is
// cancelled, reconnecting (with a fresh token) and backing off on failure.
//
// It is a speed-up, not the source of truth: events can be missed while
// disconnected, so callers must still reconcile periodically.
func (p *Private) SubscribeExecutions(ctx context.Context) (<-chan broker.Execution, error) {
	out := make(chan broker.Execution, 64)

	go func() {
		defer close(out)
		delay := time.Second
		for {
			connected, err := p.streamExecutions(ctx, out)
			if ctx.Err() != nil {
				return
			}
			if connected {
				delay = time.Second
			}
			log.Printf("gmofx: execution stream ended (%v); reconnecting in %s", err, delay)

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

// streamExecutions runs one connection: token, connect, subscribe, read.
func (p *Private) streamExecutions(ctx context.Context, out chan<- broker.Execution) (connected bool, err error) {
	token, err := p.wsToken(ctx)
	if err != nil {
		return false, err
	}
	// Tokens are limited to five per key: give this one back when done.
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := p.deleteWSToken(cleanup, token); err != nil {
			log.Printf("gmofx: delete websocket token: %v", err)
		}
	}()

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, strings.TrimRight(p.privateWSURL, "/")+"/"+token, nil)
	if err != nil {
		return false, err
	}
	defer conn.Close()

	connCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := context.AfterFunc(connCtx, func() { conn.Close() })
	defer stop()

	extendDeadline := func() { _ = conn.SetReadDeadline(time.Now().Add(readTimeout)) }
	extendDeadline()
	conn.SetPingHandler(func(data string) error {
		extendDeadline()
		return conn.WriteControl(websocket.PongMessage, []byte(data), time.Now().Add(10*time.Second))
	})

	if err := conn.WriteJSON(map[string]string{"command": "subscribe", "channel": "executionEvents"}); err != nil {
		return false, err
	}

	// Keep the token alive; if it cannot be extended, drop the connection so
	// the outer loop reconnects with a new one.
	go func() {
		ticker := time.NewTicker(p.tokenExtendInterval)
		defer ticker.Stop()
		for {
			select {
			case <-connCtx.Done():
				return
			case <-ticker.C:
				if err := p.extendWSToken(connCtx, token); err != nil {
					log.Printf("gmofx: extend websocket token: %v", err)
					cancel()
					return
				}
			}
		}
	}()

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return true, err
		}
		extendDeadline()

		var ev executionEvent
		if err := json.Unmarshal(data, &ev); err != nil || ev.Channel != "executionEvents" {
			log.Printf("gmofx: ignoring private ws message: %s", data)
			continue
		}
		select {
		case out <- ev.execution():
		case <-ctx.Done():
			return true, ctx.Err()
		}
	}
}
