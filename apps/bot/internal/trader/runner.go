package trader

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/moomoo-trading/core/broker"
	"github.com/moomoo-trading/core/broker/paper"
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
	// FillPollInterval / FillPollAttempts: after an order is accepted, its fills
	// are polled this often, this many times. Also used to look up an order
	// whose result is unknown.
	FillPollInterval time.Duration
	FillPollAttempts int
	// ReconcileInterval is how often positions are compared with the broker.
	ReconcileInterval time.Duration
}

// HardMaxLiveUnits caps the size of any order sent to a real-money broker,
// whatever the configuration says. Raise it deliberately, in code.
const HardMaxLiveUnits = 10000

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
	if c.FillPollInterval == 0 {
		c.FillPollInterval = 500 * time.Millisecond
	}
	if c.FillPollAttempts == 0 {
		c.FillPollAttempts = 10
	}
	if c.ReconcileInterval == 0 {
		c.ReconcileInterval = 5 * time.Minute
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
	// halt stops new entries on a broker (kill switch). Called when an
	// order's outcome cannot be determined.
	halt func(ctx context.Context, brokerName, reason string)

	mu      sync.Mutex
	strat   strategy.Strategy
	version Version
	lastBar time.Time // open time of the last bar fed to the strategy
	pos     *Position
	loaded  bool
	// pendingExit is the reason of an exit that could not be executed yet
	// (market closed, broker error). It is retried on every poll and tick.
	// strategyErr is the last strategy failure reported, so it is reported once.
	strategyErr string
	pendingExit string

	lastReconcile time.Time
	lastMismatch  string // the mismatch already reported, to avoid repeating it
	tickSize      decimal.Decimal
	tickLoaded    bool
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
	if r.now().Sub(r.lastReconcile) >= r.cfg.ReconcileInterval {
		// Not fatal: a failed check is logged and tried again on the next poll.
		if err := r.reconcile(ctx); err != nil {
			r.lastReconcile = time.Time{}
			r.logf("%s: %v", r.dep.Name, err)
		}
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
		r.logBar(ctx, bar, sig)
		if f, ok := r.strat.(strategy.Failer); ok && f.Err() != nil {
			// The strategy has stopped deciding (it only holds from here on):
			// say so once, loudly. An open position keeps its stop-loss.
			if msg := f.Err().Error(); msg != r.strategyErr {
				r.strategyErr = msg
				r.notify("error", "strategy stopped: "+msg)
			}
			continue
		}
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

// logBar records every bar the strategy has judged, including the ones with
// no signal, so the log and the UI show that bars are being processed and why
// nothing was traded.
func (r *Runner) logBar(ctx context.Context, bar market.Bar, sig strategy.Signal) {
	d := BarDecision{
		DeploymentID: r.dep.ID, BarTime: bar.OpenTime, Close: bar.Close,
		Action: string(sig.Action), DecidedAt: r.now(),
	}
	holding := "flat"
	if r.pos != nil {
		d.Holding = r.pos.Side
		holding = "holding " + string(r.pos.Side)
	}
	seen := ""
	if e, ok := r.strat.(strategy.Explainer); ok {
		d.Detail = e.Explain()
		seen = " [" + d.Detail + "]"
	}
	r.logf("%s: bar %s close %s -> %s (%s)%s",
		r.dep.Name, bar.OpenTime.Format(time.RFC3339), bar.Close.String(), sig.Action, holding, seen)

	if rec, ok := r.store.(BarRecorder); ok {
		if err := rec.RecordBar(ctx, d); err != nil {
			r.logf("%s: record bar decision: %v", r.dep.Name, err)
		}
	}
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

	def, err := strategy.Define(want.Type, want.Script)
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
	// A script that failed on history is kept (it only holds) rather than
	// retried on every poll: it will not run on live bars either. Activating a
	// fixed version replaces it.
	r.strategyErr = ""
	if f, ok := strat.(strategy.Failer); ok && f.Err() != nil {
		r.strategyErr = f.Err().Error()
		r.logf("%s: strategy stopped during warm-up: %s", r.dep.Name, r.strategyErr)
		r.notify("error", "strategy stopped: "+r.strategyErr)
	}
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

// deploymentKey is a short, stable stand-in for a deployment ID in order IDs.
// It is a hash of the whole ID rather than its first characters: deployment
// IDs can share a prefix (the seeded ones do), and two deployments acting on
// the same bar must never produce the same order ID.
func deploymentKey(id string) string {
	sum := sha256.Sum256([]byte(id))
	return hex.EncodeToString(sum[:4])
}

// clientOrderID is deterministic per deployment, bar and action, so retrying
// the same decision can never place a second order.
func (r *Runner) clientOrderID(tag, action string) string {
	return fmt.Sprintf("%s-%s-%s", deploymentKey(r.dep.ID), tag, action)
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
	if r.dep.Broker != paper.BrokerName && r.dep.Units.GreaterThan(decimal.NewFromInt(HardMaxLiveUnits)) {
		r.notify("skipped", fmt.Sprintf("entry refused: %s units exceeds the hard cap of %d for real-money brokers",
			r.dep.Units.String(), HardMaxLiveUnits))
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

	fill, err := r.send(ctx, order, func() (broker.OrderAck, error) {
		return r.broker.PlaceOpen(ctx, broker.OpenOrder{
			ClientOrderID: order.ClientOrderID, Symbol: order.Symbol, Side: side,
			Type: broker.OrderMarket, Size: order.Units, StrategyVersionID: r.version.ID,
		})
	})
	if errors.Is(err, errUnresolved) {
		return r.unresolved(ctx, order, err)
	}
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
	stop = r.roundToTick(ctx, stop)
	pos := Position{
		ID: uuid.NewString(), DeploymentID: r.dep.ID, Broker: r.dep.Broker, AccountID: r.dep.AccountID,
		BrokerPositionID: fill.BrokerPositionID, Symbol: r.dep.Symbol, Side: side, Units: fill.Size,
		OpenPrice: fill.Price, StopPrice: stop, Fees: fill.Fee,
		StrategyID: r.dep.StrategyID, StrategyVersionID: r.version.ID, OpenedAt: fill.At,
	}
	// The stop goes to the broker first, so the position row already carries
	// the stop order's ID when it is written.
	r.placeProtectiveStop(ctx, &pos)
	if err := r.store.InsertPosition(ctx, pos); err != nil {
		return fmt.Errorf("record position: %w", err)
	}
	r.pos = &pos
	r.notify("opened", fmt.Sprintf("%s %s %s @ %s, stop %s (%s)",
		side, fill.Size.String(), r.dep.Symbol, fill.Price.String(), stop.StringFixed(3), reason))
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

// errUnresolved: the order was sent but nobody can tell whether it executed.
var errUnresolved = errors.New("order outcome could not be determined")

// send places an order and collects its fill.
//
// A real broker reports fills a moment after accepting a market order, so
// they are polled. If the broker's answer never arrived (ErrUnknownResult),
// the order is looked up by its client order ID instead of being sent again.
// When neither gives an answer, errUnresolved is returned.
func (r *Runner) send(ctx context.Context, order Order, place func() (broker.OrderAck, error)) (Fill, error) {
	ack, err := place()
	var execs []broker.Execution
	switch {
	case errors.Is(err, broker.ErrUnknownResult):
		r.logf("%s: order %s: no answer from broker (%v); looking it up", r.dep.Name, order.ClientOrderID, err)
		execs, err = r.lookup(ctx, order)
		if err != nil {
			return Fill{}, fmt.Errorf("%w: %v", errUnresolved, err)
		}
	case err != nil:
		return Fill{}, err // a definite rejection
	default:
		execs, err = r.awaitFills(ctx, ack.OrderID)
		if err != nil {
			return Fill{}, fmt.Errorf("%w: order %s was accepted but %v", errUnresolved, ack.OrderID, err)
		}
	}

	// Size-weighted average price across partial fills.
	fill := Fill{BrokerOrderID: execs[0].OrderID, BrokerPositionID: execs[0].PositionID, At: execs[len(execs)-1].ExecutedAt}
	notional := decimal.Zero
	for _, e := range execs {
		notional = notional.Add(e.Price.Mul(e.Size))
		fill.Size = fill.Size.Add(e.Size)
		fill.Fee = fill.Fee.Add(e.Fee)
	}
	fill.Price = notional.Div(fill.Size)
	return fill, nil
}

func (r *Runner) pause(ctx context.Context) error {
	t := time.NewTimer(r.cfg.FillPollInterval)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// awaitFills polls the fills of an accepted order.
func (r *Runner) awaitFills(ctx context.Context, orderID string) ([]broker.Execution, error) {
	var lastErr error
	for attempt := 0; attempt < r.cfg.FillPollAttempts; attempt++ {
		if attempt > 0 {
			if err := r.pause(ctx); err != nil {
				return nil, err
			}
		}
		execs, err := r.broker.Executions(ctx, orderID)
		if err != nil {
			lastErr = err
			continue
		}
		if len(execs) > 0 {
			return execs, nil
		}
	}
	if lastErr != nil {
		return nil, fmt.Errorf("its fills could not be read: %v", lastErr)
	}
	return nil, errors.New("no fill was reported")
}

// lookup finds the fills of an order whose result is unknown.
func (r *Runner) lookup(ctx context.Context, order Order) ([]broker.Execution, error) {
	finder, ok := r.broker.(broker.OrderLookup)
	if !ok {
		return nil, errors.New("broker cannot look orders up")
	}
	var lastErr error
	known := false
	for attempt := 0; attempt < r.cfg.FillPollAttempts; attempt++ {
		if attempt > 0 {
			if err := r.pause(ctx); err != nil {
				return nil, err
			}
		}
		fills, found, err := finder.FindOrder(ctx, order.Symbol, order.ClientOrderID)
		if err != nil {
			lastErr = err
			continue
		}
		lastErr = nil
		known = known || found
		if len(fills) > 0 {
			return fills, nil
		}
	}
	switch {
	case lastErr != nil:
		return nil, fmt.Errorf("lookup failed: %v", lastErr)
	case known:
		return nil, errors.New("broker has the order but reports no fill")
	default:
		return nil, errors.New("broker has no trace of the order")
	}
}

// unresolved handles an order whose outcome is unknown: it is recorded, new
// entries on the broker are halted and a human is told. Nothing is re-sent.
func (r *Runner) unresolved(ctx context.Context, order Order, cause error) error {
	msg := fmt.Sprintf("order %s (%s %s %s): %v — 新規発注を停止しました。ブローカーの画面で状態を確認してください",
		order.ClientOrderID, order.SettleType, order.Side, order.Symbol, cause)
	if err := r.store.MarkOrderUnknown(ctx, order.ClientOrderID, cause.Error()); err != nil {
		r.logf("%s: record unknown order: %v", r.dep.Name, err)
	}
	if r.halt != nil {
		r.halt(ctx, r.dep.Broker, "発注結果が不明: "+order.ClientOrderID)
	}
	r.notify("error", msg)
	return cause
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
		if r.pendingExit != reason { // log once, not on every retry
			r.logf("%s: exit (%s) deferred, market %s; will retry", r.dep.Name, reason, tick.Status)
		}
		r.pendingExit = reason
		return nil
	}

	stillOpen, err := r.cancelProtectiveStop(ctx)
	if err != nil {
		r.pendingExit = reason
		return err
	}
	if !stillOpen {
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

	fill, err := r.send(ctx, order, func() (broker.OrderAck, error) {
		return r.broker.PlaceClose(ctx, broker.CloseOrder{
			ClientOrderID: order.ClientOrderID, Symbol: pos.Symbol, PositionID: pos.BrokerPositionID,
			Side: side, Type: broker.OrderMarket, Size: pos.Units,
		})
	})
	if errors.Is(err, errUnresolved) {
		// Do not keep retrying: if this close did execute, another close order
		// would act on a position that no longer exists.
		r.pendingExit = ""
		return r.unresolved(ctx, order, err)
	}
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

// ForceClose exits the current position at market (kill switch). If it cannot
// be closed now, it stays pending and is retried on every poll and tick.
func (r *Runner) ForceClose(ctx context.Context, reason string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.pos == nil {
		return nil
	}
	r.pendingExit = reason
	return r.retryPendingExit(ctx)
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
	if r.pos.StopOrderID != "" {
		// The broker holds the stop and executes it itself; sending our own
		// close would race with it. Just learn the outcome (at most every 5 s).
		if r.now().Sub(r.lastReconcile) < 5*time.Second {
			return nil
		}
		return r.reconcile(ctx)
	}
	// One order ID per position and minute: a burst of ticks cannot double-close.
	tag := fmt.Sprintf("stop-%s-%d", r.pos.ID[:8], r.now().Unix()/60)
	if err := r.close(ctx, tag, "stop"); err != nil {
		r.notify("error", err.Error())
		return err
	}
	return nil
}

// roundToTick rounds a price to the instrument's tick size (a stop price off
// the tick grid is rejected by real brokers). The tick size is read once.
func (r *Runner) roundToTick(ctx context.Context, price decimal.Decimal) decimal.Decimal {
	if !r.tickLoaded {
		r.tickLoaded = true
		instruments, err := r.broker.Instruments(ctx)
		if err != nil {
			r.tickLoaded = false
			r.logf("%s: instruments: %v", r.dep.Name, err)
		}
		for _, in := range instruments {
			if in.Key.Symbol == r.dep.Symbol {
				r.tickSize = in.TickSize
			}
		}
	}
	if !r.tickSize.IsPositive() {
		return price
	}
	return price.DivRound(r.tickSize, 0).Mul(r.tickSize)
}

func (r *Runner) notify(kind, msg string) {
	r.logf("%s: %s: %s", r.dep.Name, kind, msg)
	r.notifier.Notify(Event{Kind: kind, Deployment: r.dep, Message: msg, At: r.now()})
}
