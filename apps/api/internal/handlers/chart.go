package handlers

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/moomoo-trading/api/internal/database"
	"github.com/moomoo-trading/core/backtest"
	"github.com/moomoo-trading/core/broker"
	"github.com/moomoo-trading/core/market"
	"github.com/moomoo-trading/core/strategy"
)

const (
	defaultChartBars = 500 // the bot warms its indicators up on the same number
	maxChartBars     = 1000
)

var chartSymbol = regexp.MustCompile(`^[A-Z]{3}_[A-Z]{3}$`)

// barSource is the part of broker.MarketData the chart needs.
type barSource interface {
	Bars(ctx context.Context, req broker.BarsRequest) ([]market.Bar, error)
}

// barCache keeps recent bars per symbol and timeframe, so a chart that polls
// costs one upstream request per poll instead of one per trading day shown.
type barCache struct {
	src barSource

	mu      sync.Mutex
	entries map[string]*barCacheEntry
}

type barCacheEntry struct {
	from time.Time // start of the range the entry was loaded with
	bars []market.Bar
}

func newBarCache(src barSource) *barCache {
	return &barCache{src: src, entries: map[string]*barCacheEntry{}}
}

// lookback is how far back to load to get `count` bars: markets are open five
// days in seven, plus slack for a weekend at the edge.
func lookback(tf time.Duration, count int) time.Duration {
	return tf*time.Duration(count)*7/5 + 72*time.Hour
}

// recent returns up to `count` of the latest BID bars, including the one still
// forming. Only bars from the last cached one onwards are fetched again.
func (c *barCache) recent(ctx context.Context, symbol string, tf market.Timeframe, count int, now time.Time) ([]market.Bar, error) {
	dur, err := tf.Duration()
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	key := symbol + "|" + string(tf)
	from := now.Add(-lookback(dur, count))
	entry := c.entries[key]
	if entry == nil || len(entry.bars) == 0 || entry.from.After(from) {
		entry = &barCacheEntry{from: from}
	}

	fetchFrom := entry.from
	if n := len(entry.bars); n > 0 {
		fetchFrom = entry.bars[n-1].OpenTime // the last bar may have been still forming
	}
	fresh, err := c.src.Bars(ctx, broker.BarsRequest{
		Symbol: symbol, Timeframe: tf, PriceType: market.PriceBid,
		From: fetchFrom, To: now.Add(time.Second),
	})
	if err != nil && !errors.Is(err, broker.ErrNoData) {
		return nil, err
	}
	if len(fresh) > 0 {
		keep := len(entry.bars)
		for keep > 0 && !entry.bars[keep-1].OpenTime.Before(fresh[0].OpenTime) {
			keep--
		}
		entry.bars = append(entry.bars[:keep], fresh...)
	}
	if extra := len(entry.bars) - maxChartBars; extra > 0 {
		entry.bars = append([]market.Bar(nil), entry.bars[extra:]...)
	}
	c.entries[key] = entry

	bars := entry.bars
	if len(bars) > count {
		bars = bars[len(bars)-count:]
	}
	return append([]market.Bar(nil), bars...), nil
}

// ChartHandler serves what the live chart draws: bars, the positions the bot
// took on them and the parameters of the strategy that is trading.
type ChartHandler struct {
	repo  *database.BotRepository
	cache *barCache
	now   func() time.Time
}

func NewChartHandler(repo *database.BotRepository, src barSource) *ChartHandler {
	return &ChartHandler{repo: repo, cache: newBarCache(src), now: time.Now}
}

type chartBar struct {
	Time  int64   `json:"time"` // open time, Unix seconds (UTC)
	Open  float64 `json:"open"`
	High  float64 `json:"high"`
	Low   float64 `json:"low"`
	Close float64 `json:"close"`
}

// chartSignal is where the deployed strategy would have traded on the bars
// shown. It is a replay, not a record: real fills come from positions.
type chartSignal struct {
	Time  int64   `json:"time"` // open time of the bar it acts on, Unix seconds (UTC)
	Kind  string  `json:"kind"` // buy, sell, exit or stop
	Price float64 `json:"price"`
}

// replaySignals runs the strategy over the bars with the backtest engine, so
// entries, reversals and stop-outs follow the same rules as a backtest. Only
// BID bars are at hand: the spread and fees are ignored, which is fine for
// showing where signals fall but not for judging profit.
func replaySignals(typ, script string, params map[string]float64, bars []market.Bar) []chartSignal {
	def, err := strategy.Define(typ, script)
	if err != nil || len(bars) == 0 {
		return nil
	}
	candles := make([]backtest.Candle, 0, len(bars))
	for _, b := range bars {
		ohlc := backtest.OHLC{
			Open: b.Open.InexactFloat64(), High: b.High.InexactFloat64(),
			Low: b.Low.InexactFloat64(), Close: b.Close.InexactFloat64(),
		}
		candles = append(candles, backtest.Candle{Time: b.OpenTime, Bid: ohlc, Ask: ohlc})
	}
	// One unit against a balance that never runs out: only the timing matters.
	result, err := backtest.Run(candles, backtest.Config{
		Strategy: def, Params: strategy.Params(params), Units: 1, InitialBalance: 1e12,
	})
	if err != nil {
		return nil
	}

	signals := []chartSignal{}
	for i, t := range result.Trades {
		kind := "buy"
		if t.Side == strategy.Short {
			kind = "sell"
		}
		signals = append(signals, chartSignal{Time: t.EntryTime.Unix(), Kind: kind, Price: t.EntryPrice})

		reversed := i+1 < len(result.Trades) && result.Trades[i+1].EntryTime.Equal(t.ExitTime)
		switch {
		case t.ExitReason == "stop":
			signals = append(signals, chartSignal{Time: t.ExitTime.Unix(), Kind: "stop", Price: t.ExitPrice})
		case t.ExitReason == "signal" && !reversed:
			signals = append(signals, chartSignal{Time: t.ExitTime.Unix(), Kind: "exit", Price: t.ExitPrice})
		}
		// "end" means the position would still be open: nothing to mark.
	}
	return signals
}

// GetChart returns BID bars (the price type the bot's strategies see).
func (h *ChartHandler) GetChart(c *gin.Context) {
	ctx := c.Request.Context()

	symbol := c.DefaultQuery("symbol", "USD_JPY")
	if !chartSymbol.MatchString(symbol) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid symbol"})
		return
	}
	tf := market.Timeframe(c.DefaultQuery("timeframe", "1h"))
	if _, err := tf.Duration(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid timeframe"})
		return
	}
	count := defaultChartBars
	if raw := c.Query("count"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > maxChartBars {
			c.JSON(http.StatusBadRequest, gin.H{"error": "count must be between 1 and 1000"})
			return
		}
		count = n
	}

	bars, err := h.cache.recent(ctx, symbol, tf, count, h.now().UTC())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to load bars from the broker"})
		return
	}
	out := make([]chartBar, 0, len(bars))
	for _, b := range bars {
		out = append(out, chartBar{
			Time: b.OpenTime.Unix(),
			Open: b.Open.InexactFloat64(), High: b.High.InexactFloat64(),
			Low: b.Low.InexactFloat64(), Close: b.Close.InexactFloat64(),
		})
	}

	since := h.now().UTC()
	if len(bars) > 0 {
		since = bars[0].OpenTime
	}
	positions, err := h.repo.ChartPositions(ctx, symbol, since)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load positions"})
		return
	}
	strategy, err := h.repo.DeployedStrategy(ctx, symbol, string(tf))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load the deployed strategy"})
		return
	}

	decisions, err := h.repo.BarDecisions(ctx, symbol, string(tf), 48)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load bar decisions"})
		return
	}

	var signals []chartSignal
	if strategy != nil {
		signals = replaySignals(strategy.Type, strategy.Script, strategy.Params, bars)
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"signals":   signals,
		"decisions": decisions,
		"symbol":    symbol,
		"timeframe": tf,
		"bars":      out,
		"positions": positions,
		"strategy":  strategy,
	}})
}
