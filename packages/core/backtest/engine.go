// Package backtest replays historical BID/ASK candles through a strategy.
//
// Execution model (kept identical to how the bot will trade):
//   - Signals are computed on a closed bar and executed at the NEXT bar's open,
//     so no future data is used.
//   - Buys fill at ASK, sells fill at BID, so the spread is paid on every trade.
//   - The protective stop is checked against each bar's BID low (long) or
//     ASK high (short); a gap through the stop fills at the open.
//   - Fees are a rate on traded notional (GMO FX API fee: 0.002%).
//   - One position at a time; P&L is in the quote currency (JPY pairs only).
package backtest

import (
	"errors"
	"fmt"
	"time"

	"github.com/moomoo-trading/core/strategy"
)

// OHLC is one side of a candle.
type OHLC struct {
	Open, High, Low, Close float64
}

// Candle pairs BID and ASK prices for the same period.
type Candle struct {
	Time time.Time
	Bid  OHLC
	Ask  OHLC
}

// DefaultFeeRate is the GMO Coin FX API fee (0.002% of notional per fill).
const DefaultFeeRate = 0.00002

// DefaultLeverage is the maximum leverage for individual FX accounts in Japan.
const DefaultLeverage = 25.0

// Config describes one backtest run.
type Config struct {
	Strategy       strategy.Definition
	Params         strategy.Params
	Units          float64 // position size in base currency units
	InitialBalance float64 // in the quote currency (JPY)
	FeeRate        float64
	Leverage       float64
}

// Trade is a completed round trip.
type Trade struct {
	Side        strategy.Side `json:"side"`
	EntryTime   time.Time     `json:"entry_time"`
	EntryPrice  float64       `json:"entry_price"`
	ExitTime    time.Time     `json:"exit_time"`
	ExitPrice   float64       `json:"exit_price"`
	Units       float64       `json:"units"`
	Fees        float64       `json:"fees"`
	PnL         float64       `json:"pnl"` // net of fees
	EntryReason string        `json:"entry_reason"`
	ExitReason  string        `json:"exit_reason"` // signal, stop, end
}

// EquityPoint is the marked-to-market equity at a bar close.
type EquityPoint struct {
	Time   time.Time `json:"time"`
	Equity float64   `json:"equity"`
}

// Result is the outcome of a run.
type Result struct {
	Strategy       string          `json:"strategy"`
	Params         strategy.Params `json:"params"`
	From           time.Time       `json:"from"`
	To             time.Time       `json:"to"`
	Bars           int             `json:"bars"`
	SkippedEntries int             `json:"skipped_entries"` // entries refused for lack of margin
	Trades         []Trade         `json:"trades"`
	Equity         []EquityPoint   `json:"equity"`
	Metrics        Metrics         `json:"metrics"`
}

type openPosition struct {
	side        strategy.Side
	entryTime   time.Time
	entryPrice  float64
	stop        float64
	entryFee    float64
	entryReason string
}

type run struct {
	cfg     Config
	balance float64
	pos     *openPosition
	result  *Result
	inBars  int
}

// Run executes a backtest over candles sorted by time.
func Run(candles []Candle, cfg Config) (*Result, error) {
	if len(candles) == 0 {
		return nil, errors.New("backtest: no candles")
	}
	if cfg.Units <= 0 || cfg.InitialBalance <= 0 {
		return nil, errors.New("backtest: units and initial balance must be positive")
	}
	if cfg.FeeRate == 0 {
		cfg.FeeRate = DefaultFeeRate
	}
	if cfg.Leverage == 0 {
		cfg.Leverage = DefaultLeverage
	}
	strat, params, err := cfg.Strategy.New(cfg.Params)
	if err != nil {
		return nil, err
	}

	r := &run{
		cfg:     cfg,
		balance: cfg.InitialBalance,
		result: &Result{
			Strategy: cfg.Strategy.Type, Params: params,
			From: candles[0].Time, To: candles[len(candles)-1].Time, Bars: len(candles),
			Trades: []Trade{}, Equity: make([]EquityPoint, 0, len(candles)),
		},
	}

	var pending *strategy.Signal
	var pendingRef float64 // BID close the pending signal's stop was computed from
	for i, c := range candles {
		if i > 0 && !c.Time.After(candles[i-1].Time) {
			return nil, fmt.Errorf("backtest: candles not sorted at %s", c.Time)
		}

		if pending != nil {
			r.execute(*pending, pendingRef, c)
			pending = nil
		}
		r.checkStop(c)
		if r.pos != nil {
			r.inBars++
		}

		sig := strat.OnBar(strategy.Bar{Time: c.Time, Open: c.Bid.Open, High: c.Bid.High, Low: c.Bid.Low, Close: c.Bid.Close}, r.view())
		if f, ok := strat.(strategy.Failer); ok && f.Err() != nil {
			return nil, fmt.Errorf("backtest: strategy failed at %s: %w", c.Time.Format(time.RFC3339), f.Err())
		}
		if sig.Action != strategy.Hold {
			pending, pendingRef = &sig, c.Bid.Close
		}

		r.result.Equity = append(r.result.Equity, EquityPoint{Time: c.Time, Equity: r.markToMarket(c)})
	}

	if last := candles[len(candles)-1]; r.pos != nil {
		r.close(last.Time, r.exitPrice(last.Bid.Close, last.Ask.Close), "end")
		r.result.Equity[len(r.result.Equity)-1].Equity = r.balance
	}

	r.result.Metrics = computeMetrics(r.result, cfg.InitialBalance, r.inBars)
	return r.result, nil
}

func (r *run) view() *strategy.Position {
	if r.pos == nil {
		return nil
	}
	return &strategy.Position{Side: r.pos.side, EntryPrice: r.pos.entryPrice, EntryTime: r.pos.entryTime}
}

// exitPrice picks the side of the quote that closes the current position.
func (r *run) exitPrice(bid, ask float64) float64 {
	if r.pos.side == strategy.Long {
		return bid
	}
	return ask
}

func (r *run) execute(sig strategy.Signal, refClose float64, c Candle) {
	switch sig.Action {
	case strategy.Exit:
		if r.pos != nil {
			r.close(c.Time, r.exitPrice(c.Bid.Open, c.Ask.Open), "signal")
		}
	case strategy.EnterLong, strategy.EnterShort:
		side := strategy.Long
		if sig.Action == strategy.EnterShort {
			side = strategy.Short
		}
		if r.pos != nil {
			if r.pos.side == side {
				return
			}
			r.close(c.Time, r.exitPrice(c.Bid.Open, c.Ask.Open), "signal")
		}
		// Keep the stop distance the strategy chose, re-anchored at the actual fill.
		distance := refClose - sig.StopLoss
		entry, stop := c.Ask.Open, c.Bid.Open-distance
		if side == strategy.Short {
			distance = sig.StopLoss - refClose
			entry, stop = c.Bid.Open, c.Ask.Open+distance
		}
		if r.cfg.Units*entry/r.cfg.Leverage > r.balance {
			r.result.SkippedEntries++
			return
		}
		fee := entry * r.cfg.Units * r.cfg.FeeRate
		r.balance -= fee
		r.pos = &openPosition{side: side, entryTime: c.Time, entryPrice: entry, stop: stop, entryFee: fee, entryReason: sig.Reason}
	}
}

func (r *run) checkStop(c Candle) {
	if r.pos == nil {
		return
	}
	switch r.pos.side {
	case strategy.Long:
		if c.Bid.Low <= r.pos.stop {
			r.close(c.Time, minf(r.pos.stop, c.Bid.Open), "stop")
		}
	case strategy.Short:
		if c.Ask.High >= r.pos.stop {
			r.close(c.Time, maxf(r.pos.stop, c.Ask.Open), "stop")
		}
	}
}

func (r *run) close(at time.Time, price float64, reason string) {
	p := r.pos
	gross := (price - p.entryPrice) * r.cfg.Units
	if p.side == strategy.Short {
		gross = -gross
	}
	exitFee := price * r.cfg.Units * r.cfg.FeeRate
	r.balance += gross - exitFee
	r.result.Trades = append(r.result.Trades, Trade{
		Side: p.side, EntryTime: p.entryTime, EntryPrice: p.entryPrice,
		ExitTime: at, ExitPrice: price, Units: r.cfg.Units,
		Fees: p.entryFee + exitFee, PnL: gross - p.entryFee - exitFee,
		EntryReason: p.entryReason, ExitReason: reason,
	})
	r.pos = nil
}

func (r *run) markToMarket(c Candle) float64 {
	if r.pos == nil {
		return r.balance
	}
	if r.pos.side == strategy.Long {
		return r.balance + (c.Bid.Close-r.pos.entryPrice)*r.cfg.Units
	}
	return r.balance + (r.pos.entryPrice-c.Ask.Close)*r.cfg.Units
}

func minf(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
