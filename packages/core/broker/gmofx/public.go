// Package gmofx is the GMO Coin 外国為替FX API adapter.
// API reference: https://api.coin.z.com/fxdocs/
package gmofx

import (
	"context"
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
	"github.com/moomoo-trading/core/market"
	"github.com/shopspring/decimal"
)

const (
	// BrokerName is the identifier stored in InstrumentKey and DB rows.
	BrokerName = "gmo"

	DefaultPublicURL   = "https://forex-api.coin.z.com/public"
	DefaultPublicWSURL = "wss://forex-api.coin.z.com/ws/public/v1"

	// defaultMinInterval keeps public REST calls under the documented 6 GET/s.
	defaultMinInterval = 250 * time.Millisecond
)

// jst has no daylight saving, so a fixed zone is exact.
var jst = time.FixedZone("JST", 9*60*60)

// Options configures a Client. Zero values use the production defaults.
type Options struct {
	PublicURL   string
	PublicWSURL string
	HTTPClient  *http.Client
	// MinInterval is the minimum spacing between REST calls.
	MinInterval time.Duration
}

// Client implements broker.MarketData for GMO Coin FX.
type Client struct {
	publicURL   string
	publicWSURL string
	http        *http.Client
	minInterval time.Duration

	mu       sync.Mutex
	lastCall time.Time
}

var _ broker.MarketData = (*Client)(nil)

// New creates a public (unauthenticated) client.
func New(opts Options) *Client {
	c := &Client{
		publicURL:   strings.TrimRight(opts.PublicURL, "/"),
		publicWSURL: opts.PublicWSURL,
		http:        opts.HTTPClient,
		minInterval: opts.MinInterval,
	}
	if c.publicURL == "" {
		c.publicURL = DefaultPublicURL
	}
	if c.publicWSURL == "" {
		c.publicWSURL = DefaultPublicWSURL
	}
	if c.http == nil {
		c.http = &http.Client{Timeout: 15 * time.Second}
	}
	if c.minInterval == 0 {
		c.minInterval = defaultMinInterval
	}
	return c
}

func (c *Client) Name() string { return BrokerName }

// APIError is a non-success response from the API.
type APIError struct {
	HTTPStatus int
	Code       string
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("gmofx: http %d code=%s: %s", e.HTTPStatus, e.Code, e.Message)
}

type envelope struct {
	Status   int             `json:"status"`
	Data     json.RawMessage `json:"data"`
	Messages []struct {
		Code    string `json:"message_code"`
		Message string `json:"message_string"`
	} `json:"messages"`
}

// throttle blocks until minInterval has passed since the previous call.
func (c *Client) throttle(ctx context.Context) error {
	c.mu.Lock()
	wait := time.Until(c.lastCall.Add(c.minInterval))
	if wait < 0 {
		wait = 0
	}
	c.lastCall = time.Now().Add(wait)
	c.mu.Unlock()

	if wait == 0 {
		return nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (c *Client) getPublic(ctx context.Context, path string, query url.Values, out any) error {
	if err := c.throttle(ctx); err != nil {
		return err
	}

	u := c.publicURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("gmofx: GET %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return fmt.Errorf("gmofx: read %s: %w", path, err)
	}

	if resp.StatusCode != http.StatusOK {
		var e struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(body, &e)
		return &APIError{HTTPStatus: resp.StatusCode, Message: e.Message}
	}

	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return fmt.Errorf("gmofx: decode %s: %w", path, err)
	}
	if env.Status != 0 {
		apiErr := &APIError{HTTPStatus: resp.StatusCode, Code: strconv.Itoa(env.Status)}
		if len(env.Messages) > 0 {
			apiErr.Code = env.Messages[0].Code
			apiErr.Message = env.Messages[0].Message
		}
		return apiErr
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		return fmt.Errorf("gmofx: decode %s data: %w", path, err)
	}
	return nil
}

// Status returns the exchange status (OPEN / CLOSE / MAINTENANCE).
func (c *Client) Status(ctx context.Context) (market.MarketStatus, error) {
	var data struct {
		Status string `json:"status"`
	}
	if err := c.getPublic(ctx, "/v1/status", nil, &data); err != nil {
		return "", err
	}
	return market.MarketStatus(data.Status), nil
}

// Instruments returns the trading rules for every symbol.
func (c *Client) Instruments(ctx context.Context) ([]market.Instrument, error) {
	var data []struct {
		Symbol           string          `json:"symbol"`
		MinOpenOrderSize decimal.Decimal `json:"minOpenOrderSize"`
		MaxOrderSize     decimal.Decimal `json:"maxOrderSize"`
		SizeStep         decimal.Decimal `json:"sizeStep"`
		TickSize         decimal.Decimal `json:"tickSize"`
	}
	if err := c.getPublic(ctx, "/v1/symbols", nil, &data); err != nil {
		return nil, err
	}

	instruments := make([]market.Instrument, 0, len(data))
	for _, d := range data {
		quote := ""
		if _, q, ok := strings.Cut(d.Symbol, "_"); ok {
			quote = q
		}
		instruments = append(instruments, market.Instrument{
			Key:           market.InstrumentKey{Broker: BrokerName, Symbol: d.Symbol},
			AssetClass:    market.AssetForex,
			QuoteCurrency: quote,
			MinOrderSize:  d.MinOpenOrderSize,
			MaxOrderSize:  d.MaxOrderSize,
			SizeStep:      d.SizeStep,
			TickSize:      d.TickSize,
		})
	}
	return instruments, nil
}

type tickerPayload struct {
	Symbol    string          `json:"symbol"`
	Ask       decimal.Decimal `json:"ask"`
	Bid       decimal.Decimal `json:"bid"`
	Timestamp time.Time       `json:"timestamp"`
	Status    string          `json:"status"`
}

func (p tickerPayload) tick() market.Tick {
	return market.Tick{
		Key:    market.InstrumentKey{Broker: BrokerName, Symbol: p.Symbol},
		Bid:    p.Bid,
		Ask:    p.Ask,
		Time:   p.Timestamp.UTC(),
		Status: market.MarketStatus(p.Status),
	}
}

// Ticks returns the latest quote of every symbol.
func (c *Client) Ticks(ctx context.Context) ([]market.Tick, error) {
	var data []tickerPayload
	if err := c.getPublic(ctx, "/v1/ticker", nil, &data); err != nil {
		return nil, err
	}
	ticks := make([]market.Tick, 0, len(data))
	for _, d := range data {
		ticks = append(ticks, d.tick())
	}
	return ticks, nil
}

var intervals = map[market.Timeframe]string{
	market.TF1Min:   "1min",
	market.TF5Min:   "5min",
	market.TF15Min:  "15min",
	market.TF30Min:  "30min",
	market.TF1Hour:  "1hour",
	market.TF4Hour:  "4hour",
	market.TF8Hour:  "8hour",
	market.TF12Hour: "12hour",
	market.TF1Day:   "1day",
}

// yearlyParam reports whether the klines date parameter is YYYY (4h and above)
// rather than YYYYMMDD (1h and below).
func yearlyParam(tf market.Timeframe) bool {
	switch tf {
	case market.TF4Hour, market.TF8Hour, market.TF12Hour, market.TF1Day:
		return true
	}
	return false
}

// TradingDate returns the GMO trading date a bar opening at t belongs to.
// The klines date parameter covers 06:00 JST to 06:00 JST the next day
// (verified against live data; the weekly open is 07:00 JST in winter).
func TradingDate(t time.Time) time.Time {
	d := t.In(jst).Add(-6 * time.Hour)
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
}

// Bars returns bars with OpenTime in [req.From, req.To), fetching every
// trading date (or year, for 4h and above) the range touches.
// Returns broker.ErrNoData if the range contains no bars.
func (c *Client) Bars(ctx context.Context, req broker.BarsRequest) ([]market.Bar, error) {
	interval, ok := intervals[req.Timeframe]
	if !ok {
		return nil, fmt.Errorf("gmofx: unsupported timeframe %q", req.Timeframe)
	}
	if req.PriceType != market.PriceBid && req.PriceType != market.PriceAsk {
		return nil, fmt.Errorf("gmofx: unsupported price type %q", req.PriceType)
	}
	if !req.From.Before(req.To) {
		return nil, fmt.Errorf("gmofx: empty range %s..%s", req.From, req.To)
	}

	var params []string
	first := TradingDate(req.From)
	last := TradingDate(req.To.Add(-time.Nanosecond))
	if yearlyParam(req.Timeframe) {
		for y := first.Year(); y <= last.Year(); y++ {
			params = append(params, strconv.Itoa(y))
		}
	} else {
		for d := first; !d.After(last); d = d.AddDate(0, 0, 1) {
			params = append(params, d.Format("20060102"))
		}
	}

	var bars []market.Bar
	for _, date := range params {
		chunk, err := c.klines(ctx, req.Symbol, interval, req.Timeframe, req.PriceType, date)
		if err != nil {
			return nil, err
		}
		for _, b := range chunk {
			if !b.OpenTime.Before(req.From) && b.OpenTime.Before(req.To) {
				bars = append(bars, b)
			}
		}
	}
	if len(bars) == 0 {
		return nil, broker.ErrNoData
	}
	return bars, nil
}

func (c *Client) klines(ctx context.Context, symbol, interval string, tf market.Timeframe, pt market.PriceType, date string) ([]market.Bar, error) {
	q := url.Values{}
	q.Set("symbol", symbol)
	q.Set("priceType", string(pt))
	q.Set("interval", interval)
	q.Set("date", date)

	var data []struct {
		OpenTime string          `json:"openTime"`
		Open     decimal.Decimal `json:"open"`
		High     decimal.Decimal `json:"high"`
		Low      decimal.Decimal `json:"low"`
		Close    decimal.Decimal `json:"close"`
	}
	if err := c.getPublic(ctx, "/v1/klines", q, &data); err != nil {
		// Dates without trading (weekends, holidays) answer 404.
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.HTTPStatus == http.StatusNotFound {
			return nil, nil
		}
		return nil, err
	}

	key := market.InstrumentKey{Broker: BrokerName, Symbol: symbol}
	bars := make([]market.Bar, 0, len(data))
	for _, d := range data {
		ms, err := strconv.ParseInt(d.OpenTime, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("gmofx: invalid openTime %q: %w", d.OpenTime, err)
		}
		bars = append(bars, market.Bar{
			Key:       key,
			Timeframe: tf,
			PriceType: pt,
			OpenTime:  time.UnixMilli(ms).UTC(),
			Open:      d.Open,
			High:      d.High,
			Low:       d.Low,
			Close:     d.Close,
		})
	}
	return bars, nil
}
