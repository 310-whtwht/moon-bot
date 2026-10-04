package trader

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/moomoo-trading/core/broker"
	"github.com/moomoo-trading/core/market"
	"github.com/moomoo-trading/core/risk"
	"github.com/moomoo-trading/core/strategy"
	"github.com/shopspring/decimal"
)

// Config holds the trader settings shared by all runners.
type Config struct {
	AccountLimits risk.Limits
	GlobalLimits  risk.Limits
	// MarginBuffer is the share of available margin one order may use.
	MarginBuffer float64
	Leverage     float64
	// WarmupBars is how many closed bars are replayed to prime indicators.
	WarmupBars int
	// MaxSignalAge: a signal is only acted on if its bar closed this recently.
	// Older signals (bot was down, data was late) are skipped.
	MaxSignalAge time.Duration
}

func (c Config) withDefaults() Config {
	if c.WarmupBars == 0 {
		c.WarmupBars = 500
	}
	if c.MaxSignalAge == 0 {
		c.MaxSignalAge = 5 * time.Minute
	}
	if c.Leverage == 0 {
		c.Leverage = 25
	}
	if c.MarginBuffer == 0 {
		c.MarginBuffer = 0.5
	}
	return c
}

// Runner trades one deployment.
//
// Version switching: the runner uses the strategy's active version. When the
// active version changes while a position is open, the runner keeps the
// version that opened the position until it is closed, so a position is always
// exited by the rules it was entered with.
type Runner struct {
	dep      Deployment
	broker   broker.Broker
	store    Store
	guard    Guard
	notifier Notifier
	cfg      Config
	now      func() time.Time
	logf     func(format string, args ...any)

	mu      sync.Mutex
	strat   strategy.Strategy
	version Version
	lastBar time.Time // open time of the last bar fed to the strategy
	pos     *Position
	loaded  bool
	// pendingExit is the reason of an exit that could not be executed yet
	// (market closed, broker error). It is retried on every poll and tick.
	pendingExit string
}

// SetDeployment updates the deployment settings (e.g. enabled flag).
func (r *Runner) SetDeployment(d Deployment) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dep = d
}

// HasPosition reports whether the runner currently holds a position.
func (r *Runner) HasPosition() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.pos != nil
}

// Poll processes bars that closed since the last call. Call it periodically.
func (r *Runner) Poll(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.loaded {
		pos, err := r.store.OpenPosition(ctx, r.dep.ID)
		if err != nil {
			return fmt.Errorf("load position: %w", err)
		}
		r.pos, r.loaded = pos, true
	}
	if err := r.retryPendingExit(ctx); err != nil {
		return err
	}
	if err := r.ensureStrategy(ctx); err != nil {
		return err
	}

	barLen, _ := r.dep.Timeframe.Duration()
	// The bar after lastBar cannot have closed before lastBar + 2 bars; skip the API call until then.
	if r.now().Before(r.lastBar.Add(2 * barLen)) {
		return nil
	}
	bars, err := r.closedBarsSince(ctx, r.lastBar)
	if err != nil {
		return err
	}
	for i, bar := range bars {
		sig := r.strat.OnBar(toStrategyBar(bar), r.view())
		r.lastBar = bar.OpenTime
		if sig.Action == strategy.Hold {
			continue
		}
		age := r.now().Sub(bar.OpenTime.Add(barLen))
		if i != len(bars)-1 || age > r.cfg.MaxSignalAge {
			r.logf("%s: skipped stale %s signal from bar %s (closed %s ago)",
				r.dep.Name, sig.Action, bar.OpenTime.Format(time.RFC3339), age.Round(time.Second))
			continue
		}
		if err := r.act(ctx, sig, bar); err != nil {
			r.notify("error", err.Error())
			return err
		}
	}
	return nil
}

// retryPendingExit re-attempts an exit that failed or was deferred. The order
// ID changes every minute, so a rejected attempt does not block the next one.
func (r *Runner) retryPendingExit(ctx context.Context) error {
	if r.pendingExit == "" {
		return nil
	}
	if r.pos == nil {
		r.pendingExit = ""
		return nil
	}
	tag := fmt.Sprintf("retry-%s-%d", r.pos.ID[:8], r.now().Unix()/60)
	return r.close(ctx, tag, r.pendingExit)
}

// ensureStrategy builds the strategy on first use and when the active version
// changes while flat, then warms it up on history without trading.
func (r *Runner) ensureStrategy(ctx context.Context) error {
	want, err := r.wantedVersion(ctx)
	if err != nil {
		return err
	}
	if r.strat != nil && want.ID == r.version.ID {
		return nil
	}

	def, err := strategy.Lookup(want.Type)
	if err != nil {
		return fmt.Errorf("version %s: %w", want.ID, err)
	}
	strat, _, err := def.New(want.Params)
	if err != nil {
		return fmt.Errorf("version %s: %w", want.ID, err)
	}

	barLen, err := r.dep.Timeframe.Duration()
	if err != nil {
		return err
	}
	// Twice the bar count in calendar time covers weekend gaps.
	since := r.now().Add(-time.Duration(r.cfg.WarmupBars*2) * barLen)
	bars, err := r.closedBarsSince(ctx, since)
	if err != nil {
		return fmt.Errorf("warmup: %w", err)
	}
	if len(bars) > r.cfg.WarmupBars {
		bars = bars[len(bars)-r.cfg.WarmupBars:]
	}
	r.lastBar = since
	for _, bar := range bars {
		strat.OnBar(toStrategyBar(bar), nil) // prime indicators; signals are ignored
		r.lastBar = bar.OpenTime
	}

	if r.strat != nil {
		r.logf("%s: switched strategy version %s -> %s", r.dep.Name, r.version.ID, want.ID)
	}
	r.strat, r.version = strat, want
	r.logf("%s: strategy %s ready (version %s, %d warmup bars, last bar %s)",
		r.dep.Name, want.Type, want.ID, len(bars), r.lastBar.Format(time.RFC3339))
	return nil
}

// wantedVersion is the position's version while a position is open, else the active one.
func (r *Runner) wantedVersion(ctx context.Context) (Version, error) {
	if r.pos != nil && r.pos.StrategyVersionID != "" {
		if r.strat != nil && r.version.ID == r.pos.StrategyVersionID {
			return r.version, nil
		}
		return r.store.Version(ctx, r.pos.StrategyVersionID)
	}
	return r.store.ActiveVersion(ctx, r.dep.StrategyID)
}

// closedBarsSince returns BID bars that opened after `after` and have fully closed.
func (r *Runner) closedBarsSince(ctx context.Context, after time.Time) ([]market.Bar, error) {
	barLen, err := r.dep.Timeframe.Duration()
	if err != nil {
		return nil, err
	}
	now := r.now()
	from := after.Add(time.Nanosecond)
	if !from.Before(now) {
		return nil, nil
	}
	bars, err := r.broker.Bars(ctx, broker.BarsRequest{
		Symbol: r.dep.Symbol, Timeframe: r.dep.Timeframe, PriceType: market.PriceBid, From: from, To: now,
	})
	if errors.Is(err, broker.ErrNoData) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("bars: %w", err)
	}
	closed := bars[:0]
	for _, b := range bars {
		if !b.OpenTime.Add(barLen).After(now) {
			closed = append(closed, b)
		}
	}
	return closed, nil
}

func (r *Runner) view() *strategy.Position {
	if r.pos == nil {
		return nil
	}
	side := strategy.Long
	if r.pos.Side == broker.SideSell {
		side = strategy.Short
	}
	return &strategy.Position{Side: side, EntryPrice: r.pos.OpenPrice.InexactFloat64(), EntryTime: r.pos.OpenedAt}
}

func toStrategyBar(b market.Bar) strategy.Bar {
	return strategy.Bar{
		Time: b.OpenTime, Open: b.Open.InexactFloat64(), High: b.High.InexactFloat64(),
		Low: b.Low.InexactFloat64(), Close: b.Close.InexactFloat64(),
	}
}

func (r *Runner) act(ctx context.Context, sig strategy.Signal, bar market.Bar) error {
	tag := fmt.Sprintf("%d", bar.OpenTime.Unix())
	switch sig.Action {
	case strategy.Exit:
		if r.pos != nil {
			return r.close(ctx, tag, "signal")
		}
	case strategy.EnterLong, strategy.EnterShort:
		side := broker.SideBuy
		if sig.Action == strategy.EnterShort {
			side = broker.SideSell
		}
		if r.pos != nil {
			if r.pos.Side == side {
				return nil
			}
			if err := r.close(ctx, tag, "signal"); err != nil {
				return err
			}
			if r.pos != nil {
				return nil // close did not happen (e.g. market closed); do not stack positions
			}
		}
		stopDistance := bar.Close.InexactFloat64() - sig.StopLoss
		if side == broker.SideSell {
			stopDistance = sig.StopLoss - bar.Close.InexactFloat64()
		}
		return r.open(ctx, tag, side, stopDistance, sig.Reason)
	}
	return nil
}

func (r *Runner) quote(ctx context.Context) (market.Tick, error) {
	ticks, err := r.broker.Ticks(ctx)
	if err != nil {
		return market.Tick{}, fmt.Errorf("quote: %w", err)
	}
	for _, t := range ticks {
		if t.Key.Symbol == r.dep.Symbol {
			return t, nil
		}
	}
	return market.Tick{}, fmt.Errorf("quote: no tick for %s", r.dep.Symbol)
}

// clientOrderID is deterministic per deployment, bar and action, so retrying
// the same decision can never place a second order.
func (r *Runner) clientOrderID(tag, action string) string {
	id := r.dep.ID
	if len(id) > 8 {
		id = id[:8]
	}
	return fmt.Sprintf("%s-%s-%s", id, tag, action)
}

func (r *Runner) open(ctx context.Context, tag string, side broker.Side, stopDistance float64, reason string) error {
	if !r.dep.Enabled {
		r.logf("%s: entry skipped (deployment disabled)", r.dep.Name)
		return nil
	}
	if ok, why := r.guard.EntriesAllowed(ctx, r.dep.Broker); !ok {
		r.notify("skipped", "entry blocked: "+why)
		return nil
	}
	if stopDistance <= 0 {
		r.notify("skipped", "entry without a valid stop-loss was refused")
		return nil
	}

	tick, err := r.quote(ctx)
	if err != nil {
		return err
	}
	if tick.Status != market.StatusOpen {
		r.logf("%s: entry skipped (market %s)", r.dep.Name, tick.Status)
		return nil
	}
	price := tick.Ask
	if side == broker.SideSell {
		price = tick.Bid
	}

	order := Order{
		ClientOrderID: r.clientOrderID(tag, "open"), DeploymentID: r.dep.ID,
		Broker: r.dep.Broker, AccountID: r.dep.AccountID,
		StrategyID: r.dep.StrategyID, StrategyVersionID: r.version.ID,
		Symbol: r.dep.Symbol, Side: side, SettleType: "open", Units: r.dep.Units,
	}
	created, err := r.store.CreateOrder(ctx, order)
	if err != nil {
		return fmt.Errorf("record order: %w", err)
	}
	if !created {
		r.logf("%s: order %s already exists, not sending again", r.dep.Name, order.ClientOrderID)
		return nil
	}

	if err := r.checkRisk(ctx, price); err != nil {
		var rej *risk.Rejection
		if errors.As(err, &rej) {
			r.notify("rejected", rej.Error())
			return r.store.MarkOrderRejected(ctx, order.ClientOrderID, rej.Error())
		}
		_ = r.store.MarkOrderRejected(ctx, order.ClientOrderID, err.Error())
		return err
	}

	fill, err := r.send(ctx, func() (broker.OrderAck, error) {
		return r.broker.PlaceOpen(ctx, broker.OpenOrder{
			ClientOrderID: order.ClientOrderID, Symbol: order.Symbol, Side: side,
			Type: broker.OrderMarket, Size: order.Units, StrategyVersionID: r.version.ID,
		})
	})
	if err != nil {
		r.notify("rejected", err.Error())
		return r.store.MarkOrderRejected(ctx, order.ClientOrderID, err.Error())
	}
	if err := r.store.MarkOrderFilled(ctx, order, fill); err != nil {
		return fmt.Errorf("record fill: %w", err)
	}

	// Keep the stop distance the strategy chose, anchored at the price the
	// position can actually be closed at.
	dist := decimal.NewFromFloat(stopDistance)
	stop := tick.Bid.Sub(dist)
	if side == broker.SideSell {
		stop = tick.Ask.Add(dist)
	}
	pos := Position{
		ID: uuid.NewString(), DeploymentID: r.dep.ID, Broker: r.dep.Broker, AccountID: r.dep.AccountID,
		BrokerPositionID: fill.BrokerPositionID, Symbol: r.dep.Symbol, Side: side, Units: order.Units,
		OpenPrice: fill.Price, StopPrice: stop, Fees: fill.Fee,
		StrategyID: r.dep.StrategyID, StrategyVersionID: r.version.ID, OpenedAt: fill.At,
	}
	if err := r.store.InsertPosition(ctx, pos); err != nil {
		return fmt.Errorf("record position: %w", err)
	}
	r.pos = &pos
	r.notify("opened", fmt.Sprintf("%s %s %s @ %s, stop %s (%s)",
		side, order.Units.String(), r.dep.Symbol, fill.Price.String(), stop.StringFixed(3), reason))
	return nil
}

func (r *Runner) checkRisk(ctx context.Context, price decimal.Decimal) error {
	assets, err := r.broker.Assets(ctx)
	if err != nil {
		return fmt.Errorf("assets: %w", err)
	}
	now := r.now()
	account, global, err := r.store.Exposure(ctx, r.dep.Broker, r.dep.AccountID, risk.TradingDayStart(now), risk.TradingWeekStart(now))
	if err != nil {
		return fmt.Errorf("exposure: %w", err)
	}
	return risk.CheckOpen(risk.OpenRequest{
		Symbol: r.dep.Symbol, Units: r.dep.Units.InexactFloat64(), Price: price.InexactFloat64(),
		Leverage: r.cfg.Leverage, AvailableMarginJPY: assets.AvailableMargin.InexactFloat64(),
		MarginBuffer: r.cfg.MarginBuffer,
	}, r.cfg.AccountLimits, r.cfg.GlobalLimits, account, global)
}

// send places an order and collects its fill.
func (r *Runner) send(ctx context.Context, place func() (broker.OrderAck, error)) (Fill, error) {
	ack, err := place()
	if err != nil {
		return Fill{}, err
	}
	execs, err := r.broker.Executions(ctx, ack.OrderID)
	if err != nil {
		return Fill{}, fmt.Errorf("executions for %s: %w", ack.OrderID, err)
	}
	if len(execs) == 0 {
		return Fill{}, fmt.Errorf("order %s accepted but not filled", ack.OrderID)
	}
	// Size-weighted average price across partial fills.
	fill := Fill{BrokerOrderID: ack.OrderID, BrokerPositionID: execs[0].PositionID, At: execs[len(execs)-1].ExecutedAt}
	notional, size := decimal.Zero, decimal.Zero
	for _, e := range execs {
		notional = notional.Add(e.Price.Mul(e.Size))
		size = size.Add(e.Size)
		fill.Fee = fill.Fee.Add(e.Fee)
	}
	fill.Price = notional.Div(size)
	return fill, nil
}

// close exits the current position at market. It is never blocked by the
// guard or the enabled flag.
func (r *Runner) close(ctx context.Context, tag, reason string) error {
	pos := r.pos
	tick, err := r.quote(ctx)
	if err != nil {
		return err
	}
	if tick.Status != market.StatusOpen {
		r.logf("%s: exit (%s) deferred, market %s", r.dep.Name, reason, tick.Status)
		r.pendingExit = reason
		return nil
	}

	side := broker.SideSell
	if pos.Side == broker.SideSell {
		side = broker.SideBuy
	}
	order := Order{
		ClientOrderID: r.clientOrderID(tag, "close"), DeploymentID: r.dep.ID,
		Broker: pos.Broker, AccountID: pos.AccountID,
		StrategyID: pos.StrategyID, StrategyVersionID: pos.StrategyVersionID,
		Symbol: pos.Symbol, Side: side, SettleType: "close", Units: pos.Units,
		BrokerPositionID: pos.BrokerPositionID,
	}
	created, err := r.store.CreateOrder(ctx, order)
	if err != nil {
		return fmt.Errorf("record order: %w", err)
	}
	if !created {
		r.logf("%s: order %s already exists, not sending again", r.dep.Name, order.ClientOrderID)
		return nil
	}

	fill, err := r.send(ctx, func() (broker.OrderAck, error) {
		return r.broker.PlaceClose(ctx, broker.CloseOrder{
			ClientOrderID: order.ClientOrderID, Symbol: pos.Symbol, PositionID: pos.BrokerPositionID,
			Side: side, Type: broker.OrderMarket, Size: pos.Units,
		})
	})
	if err != nil {
		_ = r.store.MarkOrderRejected(ctx, order.ClientOrderID, err.Error())
		r.pendingExit = reason
		return fmt.Errorf("close position: %w", err)
	}
	if err := r.store.MarkOrderFilled(ctx, order, fill); err != nil {
		return fmt.Errorf("record fill: %w", err)
	}

	gross := fill.Price.Sub(pos.OpenPrice).Mul(pos.Units)
	if pos.Side == broker.SideSell {
		gross = gross.Neg()
	}
	net := gross.Sub(pos.Fees).Sub(fill.Fee)
	if err := r.store.ClosePosition(ctx, pos.ID, fill.Price, net, fill.Fee, fill.At); err != nil {
		return fmt.Errorf("record close: %w", err)
	}
	r.pos, r.pendingExit = nil, ""
	r.notify("closed", fmt.Sprintf("%s %s closed @ %s (%s), P&L %s JPY",
		pos.Side, pos.Symbol, fill.Price.String(), reason, net.StringFixed(0)))
	return nil
}

// OnTick checks the protective stop against a new quote and closes the
// position when it is hit.
func (r *Runner) OnTick(ctx context.Context, tick market.Tick) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.pos == nil || tick.Key.Symbol != r.dep.Symbol || tick.Status != market.StatusOpen {
		return nil
	}
	if r.pendingExit != "" {
		return r.retryPendingExit(ctx)
	}
	hit := tick.Bid.LessThanOrEqual(r.pos.StopPrice)
	if r.pos.Side == broker.SideSell {
		hit = tick.Ask.GreaterThanOrEqual(r.pos.StopPrice)
	}
	if !hit {
		return nil
	}
	// One order ID per position and minute: a burst of ticks cannot double-close.
	tag := fmt.Sprintf("stop-%s-%d", r.pos.ID[:8], r.now().Unix()/60)
	if err := r.close(ctx, tag, "stop"); err != nil {
		r.notify("error", err.Error())
		return err
	}
	return nil
}

func (r *Runner) notify(kind, msg string) {
	r.logf("%s: %s: %s", r.dep.Name, kind, msg)
	r.notifier.Notify(Event{Kind: kind, Deployment: r.dep, Message: msg, At: r.now()})
}
