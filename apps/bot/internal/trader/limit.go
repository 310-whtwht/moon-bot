package trader

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moomoo-trading/core/broker"
	"github.com/moomoo-trading/core/broker/paper"
	"github.com/moomoo-trading/core/market"
	"github.com/shopspring/decimal"
)

const (
	// defaultLimitWait applies when a deployment does not say how long a limit entry may wait.
	defaultLimitWait = 30 * time.Second
	// workingCheckInterval spaces the fill checks of a waiting entry (quotes arrive far more often).
	workingCheckInterval = time.Second
	// maxCancelAttempts: after this many failed cancels the order is handed to a human.
	maxCancelAttempts = 3
)

// workingEntry is a limit entry that the broker accepted and has not filled yet.
type workingEntry struct {
	order         Order
	brokerOrderID string
	tag           string
	side          broker.Side
	limit         decimal.Decimal
	stopDistance  float64
	reason        string
	wait          time.Duration
	deadline      time.Time
	lastCheck     time.Time
	cancelFails   int

	// decision is the bar this entry came from; notes are what has been
	// recorded for it so far. The outcome of the wait is added to both.
	decision *BarDecision
	notes    []string
}

// placeLimit sends an entry as a limit order resting at the near side of the
// quote (buy at BID, sell at ASK) instead of crossing the spread, and leaves
// it waiting. The order has already passed the guard and the risk checks.
func (r *Runner) placeLimit(ctx context.Context, tag string, order Order, tick market.Tick, stopDistance float64, reason string) error {
	limit, plannedStop := tick.Bid, tick.Bid.Sub(decimal.NewFromFloat(stopDistance))
	if order.Side == broker.SideSell {
		limit, plannedStop = tick.Ask, tick.Ask.Add(decimal.NewFromFloat(stopDistance))
	}
	wait := r.dep.LimitWait
	if wait <= 0 {
		wait = defaultLimitWait
	}

	ack, err := r.broker.PlaceOpen(ctx, broker.OpenOrder{
		ClientOrderID: order.ClientOrderID, Symbol: order.Symbol, Side: order.Side,
		Type: broker.OrderLimit, Price: &limit, Size: order.Units, StrategyVersionID: r.version.ID,
	})
	if errors.Is(err, broker.ErrUnknownResult) {
		return r.unresolved(ctx, order, fmt.Errorf("%w: %v", errUnresolved, err))
	}
	if err != nil {
		r.notify("rejected", err.Error())
		return r.store.MarkOrderRejected(ctx, order.ClientOrderID, err.Error())
	}
	if err := r.store.MarkOrderWorking(ctx, order.ClientOrderID, ack.OrderID, limit, plannedStop); err != nil {
		// Still tracked in memory, so the wait below resolves it; a restart
		// before that would leave it to reconciliation.
		r.logf("%s: record working order %s: %v", r.dep.Name, order.ClientOrderID, err)
	}

	r.working = &workingEntry{
		order: order, brokerOrderID: ack.OrderID, tag: tag, side: order.Side, limit: limit,
		stopDistance: stopDistance, reason: reason, wait: wait, deadline: r.now().Add(wait),
		decision: r.decision,
	}
	r.note("pending", fmt.Sprintf("limit %s %s %s @ %s, waiting up to %s",
		order.Side, order.Units.String(), order.Symbol, limit.String(), wait))
	r.logf("%s: limit entry %s %s %s @ %s is working (order %s, up to %s)",
		r.dep.Name, order.Side, order.Units.String(), order.Symbol, limit.String(), ack.OrderID, wait)

	// It may have filled at once.
	return r.resolveWorking(ctx, false)
}

// checkWorking looks after a waiting limit entry: it books the fill when it
// comes, and withdraws the order when its time is up. With force the order is
// withdrawn now (a newer signal, or the position must be flat). Whatever
// happens is added to the record of the bar the entry came from.
func (r *Runner) checkWorking(ctx context.Context, force bool) error {
	w := r.working
	if w == nil {
		return nil
	}
	if !force && r.now().Sub(w.lastCheck) < workingCheckInterval {
		return nil
	}

	// Notices raised while resolving belong to the entry's own bar, not to
	// whatever bar is being processed now.
	current := r.outcomes
	r.outcomes = append([]string(nil), w.notes...)
	err := r.resolveWorking(ctx, force)
	if len(r.outcomes) != len(w.notes) {
		if w.decision != nil {
			d := *w.decision
			d.Result = strings.Join(r.outcomes, " / ")
			if len(d.Result) > maxResultLen {
				d.Result = d.Result[:maxResultLen]
			}
			r.recordBar(ctx, d)
		}
		w.notes = r.outcomes
	}
	r.outcomes = current
	return err
}

// resolveWorking does the work of checkWorking, noting what happens in r.outcomes.
func (r *Runner) resolveWorking(ctx context.Context, force bool) error {
	w := r.working
	w.lastCheck = r.now()

	execs, err := r.broker.Executions(ctx, w.brokerOrderID)
	if err != nil {
		r.logf("%s: limit entry %s: read fills: %v", r.dep.Name, w.brokerOrderID, err)
	} else if filledSize(execs).GreaterThanOrEqual(w.order.Units) {
		return r.fillWorking(ctx, execs, true)
	}
	if !force && r.now().Before(w.deadline) {
		return nil
	}

	// Time is up, or the order is no longer wanted: withdraw it, then look
	// once more, because it can fill while the cancel is on its way.
	cancelErr := r.broker.Cancel(ctx, w.brokerOrderID)
	execs, err = r.broker.Executions(ctx, w.brokerOrderID)
	switch {
	case err != nil:
		return r.abandonWorking(ctx, fmt.Errorf("limit order %s: fills could not be read after cancelling: %v", w.brokerOrderID, err))
	case len(execs) > 0:
		return r.fillWorking(ctx, execs, cancelErr == nil)
	case cancelErr != nil:
		w.cancelFails++
		if w.cancelFails < maxCancelAttempts && !force {
			r.logf("%s: limit entry %s: cancel failed (%v); trying again", r.dep.Name, w.brokerOrderID, cancelErr)
			return nil
		}
		return r.abandonWorking(ctx, fmt.Errorf("limit order %s could not be cancelled: %v", w.brokerOrderID, cancelErr))
	}

	why := fmt.Sprintf("not filled within %s", w.wait)
	if force {
		why = "withdrawn before it filled"
	}
	if err := r.store.MarkOrderCancelled(ctx, w.order.ClientOrderID, why); err != nil {
		r.logf("%s: record cancelled order: %v", r.dep.Name, err)
	}
	r.working = nil
	r.notify("cancelled", fmt.Sprintf("limit %s %s @ %s: %s", w.side, w.order.Symbol, w.limit.String(), why))

	if !force && r.dep.LimitFallback == FallbackMarket {
		// The market order is a new decision to the broker: its own order ID,
		// and the guard, the spread limit and the risk checks all run again.
		return r.open(ctx, w.tag+"m", w.side, w.stopDistance, w.reason, true)
	}
	return nil
}

// fillWorking books the position a filled limit entry created. cancelled
// tells whether any unfilled remainder is known to be withdrawn.
func (r *Runner) fillWorking(ctx context.Context, execs []broker.Execution, remainderGone bool) error {
	w := r.working
	r.working = nil

	fill := fillOf(execs)
	order := w.order
	partial := fill.Size.LessThan(order.Units)
	order.Units = fill.Size
	if err := r.store.MarkOrderFilled(ctx, order, fill); err != nil {
		// The position is not on record: reconciliation will find it at the
		// broker, halt new entries and call for a human.
		return fmt.Errorf("record fill: %w", err)
	}
	if partial {
		r.note("partial", fmt.Sprintf("filled %s of %s; the rest was cancelled", fill.Size.String(), w.order.Units.String()))
	}

	// The stop keeps the distance the strategy chose, measured from where the
	// position can be closed now. Without a quote, from the fill price.
	tick, err := r.quote(ctx)
	if err != nil {
		tick = market.Tick{Bid: fill.Price, Ask: fill.Price}
	}
	if err := r.enter(ctx, fill, w.side, tick, w.stopDistance, w.reason); err != nil {
		return err
	}
	if partial && !remainderGone {
		// Part of the order may still be working at the broker.
		msg := fmt.Sprintf("指値注文 %s は一部だけ約定し、残りを取り消せませんでした", w.brokerOrderID)
		if r.halt != nil {
			r.halt(ctx, r.dep.Broker, msg)
		}
		r.notify("error", msg+"。新規発注を停止しました。ブローカーの画面で確認してください")
	}
	return nil
}

// abandonWorking gives a limit entry up to a human: its state at the broker
// is unknown, so new entries are halted and nothing more is sent.
func (r *Runner) abandonWorking(ctx context.Context, cause error) error {
	w := r.working
	r.working = nil
	return r.unresolved(ctx, w.order, cause)
}

// withdrawLeftoverOrders runs once at start-up: limit entries that were
// waiting when the bot stopped are cancelled, because nothing is watching
// them any more.
func (r *Runner) withdrawLeftoverOrders(ctx context.Context) {
	orders, err := r.store.WorkingOrders(ctx, r.dep.ID)
	if err != nil {
		r.logf("%s: load working orders: %v", r.dep.Name, err)
		return
	}
	for _, o := range orders {
		cancelErr := r.broker.Cancel(ctx, o.BrokerOrderID)
		execs, err := r.broker.Executions(ctx, o.BrokerOrderID)
		switch {
		case err == nil && len(execs) == 0 && (cancelErr == nil || r.dep.Broker == paper.BrokerName):
			// The paper broker forgets working orders when it restarts, so
			// its "unknown order" answer to the cancel means the same thing.
			if err := r.store.MarkOrderCancelled(ctx, o.ClientOrderID, "left working when the bot stopped; withdrawn at start-up"); err != nil {
				r.logf("%s: record cancelled order: %v", r.dep.Name, err)
			}
			r.logf("%s: withdrew limit entry %s left over from the last run", r.dep.Name, o.BrokerOrderID)
		case err == nil && len(execs) > 0:
			// The position exists at the broker but not here: the
			// reconciliation that follows halts new entries on seeing it.
			_ = r.store.MarkOrderUnknown(ctx, o.ClientOrderID, "filled while the bot was not running")
			r.notify("error", fmt.Sprintf("指値注文 %s が bot の停止中に約定していました。建玉をブローカーの画面で確認してください", o.BrokerOrderID))
		default:
			_ = r.store.MarkOrderUnknown(ctx, o.ClientOrderID, fmt.Sprintf("state at start-up unknown (cancel: %v, fills: %v)", cancelErr, err))
			r.notify("error", fmt.Sprintf("前回の稼働で残った指値注文 %s の状態を確認できません。ブローカーの画面で確認してください", o.BrokerOrderID))
		}
	}
}

func filledSize(execs []broker.Execution) decimal.Decimal {
	total := decimal.Zero
	for _, e := range execs {
		total = total.Add(e.Size)
	}
	return total
}
