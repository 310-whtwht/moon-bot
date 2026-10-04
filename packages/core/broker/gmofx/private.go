package gmofx

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/moomoo-trading/core/broker"
	"github.com/shopspring/decimal"
)

const (
	DefaultPrivateURL = "https://forex-api.coin.z.com/private"

	// Documented limits per account: 6 GET/s and 1 POST/s. Stay just under them.
	privateGetInterval  = 180 * time.Millisecond
	privatePostInterval = 1100 * time.Millisecond

	// Error codes worth handling specifically (https://api.coin.z.com/fxdocs/).
	errRateLimit      = "ERR-5003"
	errTimestampLate  = "ERR-5008"
	errTimestampEarly = "ERR-5009"

	// maxClientOrderIDLen: clientOrderId is at most 36 alphanumeric characters.
	maxClientOrderIDLen = 36
)

// PrivateOptions configures the authenticated client.
type PrivateOptions struct {
	Options
	APIKey     string
	APISecret  string
	PrivateURL string
	// PrivateWSURL is the private WebSocket endpoint (the token is appended).
	PrivateWSURL string
	// TokenExtendInterval is how often the WebSocket token is extended.
	TokenExtendInterval time.Duration
	// AccountID labels this account on orders and positions (not sent to GMO).
	AccountID string
	Now       func() time.Time
}

// Private is the authenticated GMO Coin FX client. It implements
// broker.Broker: market data from the embedded public Client, trading through
// the private API.
type Private struct {
	*Client
	apiKey     string
	apiSecret  []byte
	privateURL string
	accountID  string
	now        func() time.Time

	privateWSURL        string
	tokenExtendInterval time.Duration

	getGate, postGate gate

	clockMu     sync.Mutex
	clockOffset time.Duration // server time − local time, learned from ERR-5008/5009
}

var (
	_ broker.Broker            = (*Private)(nil)
	_ broker.OrderLookup       = (*Private)(nil)
	_ broker.ProtectiveStopper = (*Private)(nil)
	_ broker.ExecutionHistory  = (*Private)(nil)
)

// gate spaces calls at least `interval` apart.
type gate struct {
	mu       sync.Mutex
	interval time.Duration
	last     time.Time
}

func (g *gate) wait(ctx context.Context) error {
	g.mu.Lock()
	wait := time.Until(g.last.Add(g.interval))
	if wait < 0 {
		wait = 0
	}
	g.last = time.Now().Add(wait)
	g.mu.Unlock()
	if wait == 0 {
		return nil
	}
	t := time.NewTimer(wait)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// NewPrivate creates the authenticated client.
func NewPrivate(opts PrivateOptions) (*Private, error) {
	if opts.APIKey == "" || opts.APISecret == "" {
		return nil, errors.New("gmofx: API key and secret are required")
	}
	p := &Private{
		Client:     New(opts.Options),
		apiKey:     opts.APIKey,
		apiSecret:  []byte(opts.APISecret),
		privateURL: strings.TrimRight(opts.PrivateURL, "/"),
		accountID:  opts.AccountID,
		now:        opts.Now,
		getGate:    gate{interval: privateGetInterval},
		postGate:   gate{interval: privatePostInterval},
	}
	if p.privateURL == "" {
		p.privateURL = DefaultPrivateURL
	}
	p.privateWSURL = opts.PrivateWSURL
	if p.privateWSURL == "" {
		p.privateWSURL = DefaultPrivateWSURL
	}
	p.tokenExtendInterval = opts.TokenExtendInterval
	if p.tokenExtendInterval == 0 {
		p.tokenExtendInterval = defaultTokenExtendInterval
	}
	if p.accountID == "" {
		p.accountID = "default"
	}
	if p.now == nil {
		p.now = time.Now
	}
	if opts.MinInterval > 0 { // tests shorten the spacing
		p.getGate.interval, p.postGate.interval = opts.MinInterval, opts.MinInterval
	}
	return p, nil
}

func (p *Private) AccountID() string { return p.accountID }

// Sign returns the API-SIGN header: hex HMAC-SHA256 of
// timestamp + method + path + body, where path starts with /v1 and the query
// string is never signed. Only POST signs its body: GET has none, and the
// PUT / DELETE calls (ws-auth) send a body but sign without it, as in the
// official examples. Use signedBody to pick the right text.
func Sign(secret []byte, timestamp, method, path, body string) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(timestamp + method + path + body))
	return hex.EncodeToString(mac.Sum(nil))
}

// ClientOrderID converts an internal order ID to GMO's format: alphanumeric
// only, at most 36 characters.
func ClientOrderID(id string) string {
	var b strings.Builder
	for _, r := range id {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			b.WriteRune(r)
		}
	}
	out := b.String()
	if len(out) > maxClientOrderIDLen {
		// Keep the tail: it holds the bar time and action, which make the ID unique.
		out = out[len(out)-maxClientOrderIDLen:]
	}
	return out
}

// signedBody is the body text that goes into the signature for a method.
func signedBody(method, body string) string {
	if method == http.MethodPost {
		return body
	}
	return ""
}

func (p *Private) timestamp() string {
	p.clockMu.Lock()
	offset := p.clockOffset
	p.clockMu.Unlock()
	return strconv.FormatInt(p.now().Add(offset).UnixMilli(), 10)
}

// call performs one signed request. Orders (POST) are never retried here:
// a transport failure after sending is reported as broker.ErrUnknownResult.
func (p *Private) call(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	bodyText := ""
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyText = string(raw)
	}

	const attempts = 2 // one retry for clock skew, and for rate limits on GET
	for attempt := 1; ; attempt++ {
		err := p.once(ctx, method, path, query, bodyText, out)
		var apiErr *APIError
		if !errors.As(err, &apiErr) || attempt == attempts {
			return err
		}
		switch {
		case apiErr.Code == errTimestampLate || apiErr.Code == errTimestampEarly:
			if !p.syncClock(apiErr.ServerTime) {
				return err
			}
		case apiErr.Code == errRateLimit && method == http.MethodGet:
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Second):
			}
		default:
			return err
		}
	}
}

// syncClock sets the offset so the next timestamp matches the server's clock.
func (p *Private) syncClock(server time.Time) bool {
	if server.IsZero() {
		return false
	}
	p.clockMu.Lock()
	p.clockOffset = server.Sub(p.now())
	p.clockMu.Unlock()
	return true
}

func (p *Private) once(ctx context.Context, method, path string, query url.Values, bodyText string, out any) error {
	g := &p.getGate
	if method != http.MethodGet {
		g = &p.postGate
	}
	if err := g.wait(ctx); err != nil {
		return err
	}

	u := p.privateURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	var reader io.Reader
	if bodyText != "" {
		reader = bytes.NewReader([]byte(bodyText))
	}
	req, err := http.NewRequestWithContext(ctx, method, u, reader)
	if err != nil {
		return err
	}
	ts := p.timestamp()
	req.Header.Set("API-KEY", p.apiKey)
	req.Header.Set("API-TIMESTAMP", ts)
	req.Header.Set("API-SIGN", Sign(p.apiSecret, ts, method, path, signedBody(method, bodyText)))
	if bodyText != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	sent := method != http.MethodGet
	resp, err := p.http.Do(req)
	if err != nil {
		if sent {
			return fmt.Errorf("gmofx: %s %s: %w (%v)", method, path, broker.ErrUnknownResult, err)
		}
		return fmt.Errorf("gmofx: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		if sent {
			return fmt.Errorf("gmofx: read %s: %w (%v)", path, broker.ErrUnknownResult, err)
		}
		return fmt.Errorf("gmofx: read %s: %w", path, err)
	}
	if resp.StatusCode >= 500 && sent {
		return fmt.Errorf("gmofx: %s %s: %w (http %d)", method, path, broker.ErrUnknownResult, resp.StatusCode)
	}

	var env struct {
		Status       int             `json:"status"`
		Data         json.RawMessage `json:"data"`
		ResponseTime time.Time       `json:"responsetime"`
		Messages     []struct {
			Code    string `json:"message_code"`
			Message string `json:"message_string"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		if resp.StatusCode != http.StatusOK {
			return &APIError{HTTPStatus: resp.StatusCode, Message: strings.TrimSpace(string(raw))}
		}
		return fmt.Errorf("gmofx: decode %s: %w", path, err)
	}
	if resp.StatusCode != http.StatusOK || env.Status != 0 {
		apiErr := &APIError{HTTPStatus: resp.StatusCode, Code: strconv.Itoa(env.Status), ServerTime: env.ResponseTime}
		if len(env.Messages) > 0 {
			apiErr.Code, apiErr.Message = env.Messages[0].Code, env.Messages[0].Message
		}
		return apiErr
	}
	if out == nil || len(env.Data) == 0 {
		return nil
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		return fmt.Errorf("gmofx: decode %s data: %w", path, err)
	}
	return nil
}

// --- account -----------------------------------------------------------------

func (p *Private) Assets(ctx context.Context) (broker.Assets, error) {
	var data []struct {
		Equity          decimal.Decimal `json:"equity"`
		AvailableAmount decimal.Decimal `json:"availableAmount"`
	}
	if err := p.call(ctx, http.MethodGet, "/v1/account/assets", nil, nil, &data); err != nil {
		return broker.Assets{}, err
	}
	if len(data) == 0 {
		return broker.Assets{}, errors.New("gmofx: empty assets response")
	}
	return broker.Assets{Currency: "JPY", Equity: data[0].Equity, AvailableMargin: data[0].AvailableAmount}, nil
}

func (p *Private) OpenPositions(ctx context.Context, symbol string) ([]broker.Position, error) {
	q := url.Values{}
	if symbol != "" {
		q.Set("symbol", symbol)
	}
	var data struct {
		List []struct {
			PositionID int64           `json:"positionId"`
			Symbol     string          `json:"symbol"`
			Side       string          `json:"side"`
			Size       decimal.Decimal `json:"size"`
			Price      decimal.Decimal `json:"price"`
			Timestamp  time.Time       `json:"timestamp"`
		} `json:"list"`
	}
	if err := p.call(ctx, http.MethodGet, "/v1/openPositions", q, nil, &data); err != nil {
		return nil, err
	}
	out := make([]broker.Position, 0, len(data.List))
	for _, d := range data.List {
		out = append(out, broker.Position{
			PositionID: strconv.FormatInt(d.PositionID, 10), Symbol: d.Symbol, Side: broker.Side(d.Side),
			Size: d.Size, OpenPrice: d.Price, OpenedAt: d.Timestamp.UTC(),
		})
	}
	return out, nil
}

// --- orders ------------------------------------------------------------------

type orderResponse struct {
	OrderID       int64     `json:"orderId"`
	ClientOrderID string    `json:"clientOrderId"`
	Status        string    `json:"status"`
	Timestamp     time.Time `json:"timestamp"`
}

func (p *Private) ack(data []orderResponse) (broker.OrderAck, error) {
	if len(data) == 0 {
		return broker.OrderAck{}, fmt.Errorf("gmofx: order accepted without an order id: %w", broker.ErrUnknownResult)
	}
	d := data[0]
	return broker.OrderAck{
		OrderID: strconv.FormatInt(d.OrderID, 10), ClientOrderID: d.ClientOrderID,
		Status: d.Status, AcceptedAt: d.Timestamp.UTC(),
	}, nil
}

func orderFields(typ broker.OrderType, price *decimal.Decimal) (map[string]any, error) {
	fields := map[string]any{"executionType": string(typ)}
	switch typ {
	case broker.OrderMarket:
	case broker.OrderLimit:
		if price == nil {
			return nil, errors.New("gmofx: limit order needs a price")
		}
		fields["limitPrice"] = price.String()
	case broker.OrderStop:
		if price == nil {
			return nil, errors.New("gmofx: stop order needs a price")
		}
		fields["stopPrice"] = price.String()
	default:
		return nil, fmt.Errorf("gmofx: unsupported order type %q", typ)
	}
	return fields, nil
}

// PlaceOpen opens a position (POST /v1/order).
func (p *Private) PlaceOpen(ctx context.Context, order broker.OpenOrder) (broker.OrderAck, error) {
	body, err := orderFields(order.Type, order.Price)
	if err != nil {
		return broker.OrderAck{}, err
	}
	body["symbol"] = order.Symbol
	body["side"] = string(order.Side)
	body["size"] = order.Size.String()
	if id := ClientOrderID(order.ClientOrderID); id != "" {
		body["clientOrderId"] = id
	}

	var data []orderResponse
	if err := p.call(ctx, http.MethodPost, "/v1/order", nil, body, &data); err != nil {
		return broker.OrderAck{}, err
	}
	return p.ack(data)
}

// PlaceClose closes (part of) one position (POST /v1/closeOrder).
func (p *Private) PlaceClose(ctx context.Context, order broker.CloseOrder) (broker.OrderAck, error) {
	positionID, err := strconv.ParseInt(order.PositionID, 10, 64)
	if err != nil {
		return broker.OrderAck{}, fmt.Errorf("gmofx: invalid position id %q", order.PositionID)
	}
	body, err := orderFields(order.Type, order.Price)
	if err != nil {
		return broker.OrderAck{}, err
	}
	body["symbol"] = order.Symbol
	body["side"] = string(order.Side)
	body["settlePosition"] = []map[string]any{{"positionId": positionID, "size": order.Size.String()}}
	if id := ClientOrderID(order.ClientOrderID); id != "" {
		body["clientOrderId"] = id
	}

	var data []orderResponse
	if err := p.call(ctx, http.MethodPost, "/v1/closeOrder", nil, body, &data); err != nil {
		return broker.OrderAck{}, err
	}
	return p.ack(data)
}

// Cancel cancels a working order (POST /v1/cancelOrders).
func (p *Private) Cancel(ctx context.Context, orderID string) error {
	id, err := strconv.ParseInt(orderID, 10, 64)
	if err != nil {
		return fmt.Errorf("gmofx: invalid order id %q", orderID)
	}
	return p.call(ctx, http.MethodPost, "/v1/cancelOrders", nil, map[string]any{"rootOrderIds": []int64{id}}, nil)
}

// --- executions --------------------------------------------------------------

type executionPayload struct {
	ExecutionID   int64           `json:"executionId"`
	ClientOrderID string          `json:"clientOrderId"`
	OrderID       int64           `json:"orderId"`
	PositionID    int64           `json:"positionId"`
	Symbol        string          `json:"symbol"`
	Side          string          `json:"side"`
	SettleType    string          `json:"settleType"`
	Size          decimal.Decimal `json:"size"`
	Price         decimal.Decimal `json:"price"`
	LossGain      decimal.Decimal `json:"lossGain"`
	Fee           decimal.Decimal `json:"fee"`
	Timestamp     time.Time       `json:"timestamp"`
}

func (e executionPayload) execution() broker.Execution {
	return broker.Execution{
		ExecutionID: strconv.FormatInt(e.ExecutionID, 10),
		OrderID:     strconv.FormatInt(e.OrderID, 10),
		PositionID:  strconv.FormatInt(e.PositionID, 10),
		Symbol:      e.Symbol, Side: broker.Side(e.Side), SettleType: e.SettleType,
		Size: e.Size, Price: e.Price,
		// GMO reports the fee as a signed cash flow (a cost is negative); store the cost as positive.
		Fee:         e.Fee.Abs(),
		RealizedPnL: e.LossGain,
		ExecutedAt:  e.Timestamp.UTC(),
	}
}

// Executions returns the fills of an order (GET /v1/executions?orderId=).
// A market order's fills can appear a moment after the order is accepted, so
// an empty result does not mean the order failed.
func (p *Private) Executions(ctx context.Context, orderID string) ([]broker.Execution, error) {
	q := url.Values{}
	q.Set("orderId", orderID)
	var data struct {
		List []executionPayload `json:"list"`
	}
	if err := p.call(ctx, http.MethodGet, "/v1/executions", q, nil, &data); err != nil {
		return nil, err
	}
	out := make([]broker.Execution, 0, len(data.List))
	for _, e := range data.List {
		out = append(out, e.execution())
	}
	return out, nil
}

// FindOrder looks an order up by client order ID in the latest executions
// (last day, up to 100) and in the working orders.
func (p *Private) FindOrder(ctx context.Context, symbol, clientOrderID string) ([]broker.Execution, bool, error) {
	want := ClientOrderID(clientOrderID)
	if want == "" {
		return nil, false, errors.New("gmofx: empty client order id")
	}

	latest, err := p.latestExecutions(ctx, symbol)
	if err != nil {
		return nil, false, err
	}
	var fills []broker.Execution
	for _, e := range latest {
		if e.ClientOrderID == want {
			fills = append(fills, e.execution())
		}
	}
	if len(fills) > 0 {
		return fills, true, nil
	}

	q := url.Values{}
	q.Set("symbol", symbol)

	var active struct {
		List []struct {
			ClientOrderID string `json:"clientOrderId"`
		} `json:"list"`
	}
	if err := p.call(ctx, http.MethodGet, "/v1/activeOrders", q, nil, &active); err != nil {
		return nil, false, err
	}
	for _, o := range active.List {
		if o.ClientOrderID == want {
			return nil, true, nil // accepted and still working, no fills yet
		}
	}
	return nil, false, nil
}

// latestExecutions returns the fills of the last day, newest first (up to 100).
func (p *Private) latestExecutions(ctx context.Context, symbol string) ([]executionPayload, error) {
	q := url.Values{}
	q.Set("symbol", symbol)
	var data struct {
		List []executionPayload `json:"list"`
	}
	if err := p.call(ctx, http.MethodGet, "/v1/latestExecutions", q, nil, &data); err != nil {
		return nil, err
	}
	return data.List, nil
}

// RecentExecutions lists the fills of the last day for a symbol (up to 100).
func (p *Private) RecentExecutions(ctx context.Context, symbol string) ([]broker.Execution, error) {
	latest, err := p.latestExecutions(ctx, symbol)
	if err != nil {
		return nil, err
	}
	out := make([]broker.Execution, 0, len(latest))
	for _, e := range latest {
		out = append(out, e.execution())
	}
	return out, nil
}

// PlaceProtectiveStop places a STOP close order on one position. It stays at
// GMO and executes there, so the stop works while the bot is down.
func (p *Private) PlaceProtectiveStop(ctx context.Context, order broker.StopOrder) (broker.OrderAck, error) {
	price := order.StopPrice
	return p.PlaceClose(ctx, broker.CloseOrder{
		ClientOrderID: order.ClientOrderID, Symbol: order.Symbol, PositionID: order.PositionID,
		Side: order.Side, Type: broker.OrderStop, Size: order.Size, Price: &price,
	})
}

// activeStatuses are the order states in which an order can still execute.
var activeStatuses = map[string]bool{"WAITING": true, "ORDERED": true, "MODIFYING": true}

// OrderActive reports whether an order is still working (GET /v1/orders?orderId=).
func (p *Private) OrderActive(ctx context.Context, orderID string) (bool, error) {
	q := url.Values{}
	q.Set("orderId", orderID)
	var data struct {
		List []struct {
			Status string `json:"status"`
		} `json:"list"`
	}
	if err := p.call(ctx, http.MethodGet, "/v1/orders", q, nil, &data); err != nil {
		return false, err
	}
	for _, o := range data.List {
		if activeStatuses[o.Status] {
			return true, nil
		}
	}
	return false, nil
}
