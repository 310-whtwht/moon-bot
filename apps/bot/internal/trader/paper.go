package trader

import (
	"context"
	"fmt"

	"github.com/moomoo-trading/core/broker"
	"github.com/moomoo-trading/core/broker/paper"
	"github.com/shopspring/decimal"
)

// RestorePaper rebuilds the paper broker's state from the database after a
// restart: balance = initial + realised P&L of closed positions − entry fees
// of the positions still open; open positions are loaded as they were.
func RestorePaper(ctx context.Context, store Store, b *paper.Broker, initialBalance decimal.Decimal) error {
	realized, err := store.RealizedPnL(ctx, paper.BrokerName, b.AccountID())
	if err != nil {
		return fmt.Errorf("restore paper balance: %w", err)
	}
	open, err := store.OpenPositions(ctx, paper.BrokerName, b.AccountID())
	if err != nil {
		return fmt.Errorf("restore paper positions: %w", err)
	}

	balance := initialBalance.Add(realized)
	positions := make([]broker.Position, 0, len(open))
	for _, p := range open {
		balance = balance.Sub(p.Fees)
		positions = append(positions, broker.Position{
			PositionID: p.BrokerPositionID, Symbol: p.Symbol, Side: p.Side,
			Size: p.Units, OpenPrice: p.OpenPrice, OpenedAt: p.OpenedAt,
		})
	}
	b.Restore(balance, positions)
	return nil
}
