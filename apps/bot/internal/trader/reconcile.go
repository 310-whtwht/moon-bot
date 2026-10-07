package trader

import (
	"context"
	"fmt"

	"github.com/moomoo-trading/core/broker"
	"github.com/shopspring/decimal"
)

// Reconcile runs a reconciliation now (used when the broker reports a fill).
func (r *Runner) Reconcile(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.loaded {
		return nil // the first poll loads the position and reconciles
	}
	return r.reconcile(ctx)
}

// reconcile compares the runner's position with what the broker actually
// holds. It runs at start-up, on an interval, and when a quote crosses a
// broker-side stop.
//
//   - Position closed at the broker (stop order, margin call, manual trade):
//     the closing fills are read and the position is settled in the database.
//   - Anything that cannot be explained (an unknown position at the broker, a
//     size difference, a position that vanished without a fill): new entries
//     on the broker are halted and a human is told.
//   - A protective stop that is no longer working is placed again.
func (r *Runner) reconcile(ctx context.Context) error {
	return r.reconcileWith(ctx, true)
}

// reconcileWith is reconcile with control over whether a missing protective
// stop is placed again (not wanted while the bot is about to close the position).
func (r *Runner) reconcileWith(ctx context.Context, maintainStop bool) error {
	r.lastReconcile = r.now()

	positions, err := r.broker.OpenPositions(ctx, r.dep.Symbol)
	if err != nil {
		return fmt.Errorf("reconcile: %w", err)
	}

	if r.pos == nil {
		if r.working != nil {
			// A position here would be the waiting entry filling: checkWorking picks it up.
			return nil
		}
		if len(positions) > 0 {
			r.mismatch(ctx, fmt.Sprintf("ブローカーに bot の知らない %s の建玉が %d 件あります", r.dep.Symbol, len(positions)))
		} else {
			r.lastMismatch = ""
		}
		return nil
	}

	var mine *broker.Position
	for i := range positions {
		if positions[i].PositionID == r.pos.BrokerPositionID {
			mine = &positions[i]
		}
	}
	switch {
	case mine == nil:
		return r.settleClosedAtBroker(ctx)
	case len(positions) > 1:
		r.mismatch(ctx, fmt.Sprintf("ブローカーに bot の知らない %s の建玉が %d 件あります", r.dep.Symbol, len(positions)-1))
		return nil
	case !mine.Size.Equal(r.pos.Units):
		r.mismatch(ctx, fmt.Sprintf("%s の建玉数量が一致しません（DB %s / ブローカー %s）",
			r.dep.Symbol, r.pos.Units.String(), mine.Size.String()))
		return nil
	}
	r.lastMismatch = ""
	if maintainStop {
		r.ensureProtectiveStop(ctx)
	}
	return nil
}

// mismatch halts new entries on the broker and reports once per distinct problem.
func (r *Runner) mismatch(ctx context.Context, reason string) {
	if reason == r.lastMismatch {
		return
	}
	r.lastMismatch = reason
	if r.halt != nil {
		r.halt(ctx, r.dep.Broker, "建玉の不一致: "+reason)
	}
	r.notify("error", "建玉の不一致を検出し、新規発注を停止しました: "+reason+"。ブローカーの画面で確認してください")
}

// settleClosedAtBroker records a position that the broker closed without the
// bot sending the close (typically the protective stop order).
func (r *Runner) settleClosedAtBroker(ctx context.Context) error {
	pos := r.pos
	history, ok := r.broker.(broker.ExecutionHistory)
	if !ok {
		r.mismatch(ctx, fmt.Sprintf("%s の建玉 %s がブローカーに存在しません", pos.Symbol, pos.BrokerPositionID))
		return nil
	}
	recent, err := history.RecentExecutions(ctx, pos.Symbol)
	if err != nil {
		return fmt.Errorf("reconcile: recent executions: %w", err)
	}

	var fill Fill
	notional := decimal.Zero
	for _, e := range recent {
		if e.SettleType != "CLOSE" || e.PositionID != pos.BrokerPositionID {
			continue
		}
		notional = notional.Add(e.Price.Mul(e.Size))
		fill.Size = fill.Size.Add(e.Size)
		fill.Fee = fill.Fee.Add(e.Fee)
		fill.BrokerOrderID = e.OrderID
		if e.ExecutedAt.After(fill.At) {
			fill.At = e.ExecutedAt
		}
	}
	if fill.Size.IsZero() {
		r.mismatch(ctx, fmt.Sprintf("%s の建玉 %s がブローカーに存在せず、決済の約定も見つかりません",
			pos.Symbol, pos.BrokerPositionID))
		return nil
	}
	if !fill.Size.Equal(pos.Units) {
		r.mismatch(ctx, fmt.Sprintf("%s の建玉 %s の決済数量が一致しません（建玉 %s / 約定 %s）",
			pos.Symbol, pos.BrokerPositionID, pos.Units.String(), fill.Size.String()))
		return nil
	}
	fill.Price = notional.Div(fill.Size)
	fill.BrokerPositionID = pos.BrokerPositionID

	side := broker.SideSell
	if pos.Side == broker.SideSell {
		side = broker.SideBuy
	}
	order := Order{
		ClientOrderID: r.clientOrderID("broker-"+pos.ID[:8], "close"), DeploymentID: r.dep.ID,
		Broker: pos.Broker, AccountID: pos.AccountID,
		StrategyID: pos.StrategyID, StrategyVersionID: pos.StrategyVersionID,
		Symbol: pos.Symbol, Side: side, SettleType: "close", Units: pos.Units,
		BrokerPositionID: pos.BrokerPositionID,
	}
	created, err := r.store.CreateOrder(ctx, order)
	if err != nil {
		return fmt.Errorf("reconcile: record order: %w", err)
	}
	if created {
		if err := r.store.MarkOrderFilled(ctx, order, fill); err != nil {
			return fmt.Errorf("reconcile: record fill: %w", err)
		}
	}

	gross := fill.Price.Sub(pos.OpenPrice).Mul(pos.Units)
	if pos.Side == broker.SideSell {
		gross = gross.Neg()
	}
	net := gross.Sub(pos.Fees).Sub(fill.Fee)
	if err := r.store.ClosePosition(ctx, pos.ID, fill.Price, net, fill.Fee, fill.At); err != nil {
		return fmt.Errorf("reconcile: record close: %w", err)
	}
	r.pos, r.pendingExit, r.lastMismatch = nil, "", ""
	r.notify("closed", fmt.Sprintf("%s %s closed @ %s (ブローカー側で決済: 逆指値など), P&L %s JPY",
		pos.Side, pos.Symbol, fill.Price.String(), net.StringFixed(0)))
	return nil
}

// placeProtectiveStop puts the position's stop at the broker, when the broker
// supports it. On failure the bot keeps watching the stop itself.
func (r *Runner) placeProtectiveStop(ctx context.Context, pos *Position) {
	stopper, ok := r.broker.(broker.ProtectiveStopper)
	if !ok {
		return
	}
	side := broker.SideSell
	if pos.Side == broker.SideSell {
		side = broker.SideBuy
	}
	// The minute in the ID lets a later re-placement use a fresh ID.
	tag := fmt.Sprintf("pstop-%s-%d", pos.ID[:8], r.now().Unix()/60)
	ack, err := stopper.PlaceProtectiveStop(ctx, broker.StopOrder{
		ClientOrderID: r.clientOrderID(tag, "stop"), Symbol: pos.Symbol, PositionID: pos.BrokerPositionID,
		Side: side, Size: pos.Units, StopPrice: pos.StopPrice,
	})
	if err != nil {
		pos.StopOrderID = ""
		r.notify("error", fmt.Sprintf("ブローカーに逆指値（%s）を置けませんでした: %v。bot 側の監視だけで損切りします",
			pos.StopPrice.String(), err))
		return
	}
	pos.StopOrderID = ack.OrderID
}

// ensureProtectiveStop re-places the broker-side stop when it is missing or
// no longer working (expired, cancelled by hand).
func (r *Runner) ensureProtectiveStop(ctx context.Context) {
	stopper, ok := r.broker.(broker.ProtectiveStopper)
	if !ok || r.pos == nil {
		return
	}
	if r.pos.StopOrderID != "" {
		active, err := stopper.OrderActive(ctx, r.pos.StopOrderID)
		if err != nil {
			r.logf("%s: check stop order %s: %v", r.dep.Name, r.pos.StopOrderID, err)
			return
		}
		if active {
			return
		}
		r.logf("%s: stop order %s is no longer working; placing a new one", r.dep.Name, r.pos.StopOrderID)
	}
	r.placeProtectiveStop(ctx, r.pos)
	if err := r.store.SetPositionStopOrder(ctx, r.pos.ID, r.pos.StopOrderID); err != nil {
		r.logf("%s: record stop order: %v", r.dep.Name, err)
	}
}

// cancelProtectiveStop removes the broker-side stop before the bot closes the
// position itself (the broker would refuse a close larger than what the stop
// order leaves free). It reports whether the position is still there to close.
func (r *Runner) cancelProtectiveStop(ctx context.Context) (stillOpen bool, err error) {
	pos := r.pos
	if pos.StopOrderID == "" {
		return true, nil
	}
	cancelErr := r.broker.Cancel(ctx, pos.StopOrderID)
	if cancelErr == nil {
		pos.StopOrderID = ""
		if err := r.store.SetPositionStopOrder(ctx, pos.ID, ""); err != nil {
			r.logf("%s: clear stop order: %v", r.dep.Name, err)
		}
		return true, nil
	}

	// The cancel failed: the stop may just have executed. Ask the broker.
	r.logf("%s: cancel stop order %s: %v; checking the position", r.dep.Name, pos.StopOrderID, cancelErr)
	if err := r.reconcileWith(ctx, false); err != nil {
		return false, err
	}
	if r.pos == nil {
		return false, nil // settled from the broker's fills
	}
	if r.lastMismatch != "" {
		return false, nil // halted; a human has to look
	}
	// Position still there and consistent: the stop order was already gone.
	pos.StopOrderID = ""
	_ = r.store.SetPositionStopOrder(ctx, pos.ID, "")
	return true, nil
}
