package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// OrderRepository handles database operations for orders and trades
type OrderRepository struct {
	db *sql.DB
}

// NewOrderRepository creates a new order repository
func NewOrderRepository(db *sql.DB) *OrderRepository {
	return &OrderRepository{db: db}
}

// CreateOrder creates a new order
func (r *OrderRepository) CreateOrder(ctx context.Context, order *Order) error {
	order.ID = uuid.New().String()
	order.CreatedAt = time.Now()
	order.UpdatedAt = time.Now()

	query := `
		INSERT INTO orders (id, client_order_id, strategy_id, symbol, side, order_type, quantity, price, stop_price, status, filled_quantity, avg_fill_price, commission, broker_order_id, error_message, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		order.ID, order.ClientOrderID, order.StrategyID, order.Symbol, order.Side, order.OrderType,
		order.Quantity, order.Price, order.StopPrice, order.Status, order.FilledQuantity,
		order.AvgFillPrice, order.Commission, order.BrokerOrderID, order.ErrorMessage,
		order.CreatedAt, order.UpdatedAt)
	return err
}

// GetOrderByID retrieves an order by ID
func (r *OrderRepository) GetOrderByID(ctx context.Context, id string) (*Order, error) {
	query := `
		SELECT id, client_order_id, strategy_id, symbol, side, order_type, quantity, price, stop_price, status, filled_quantity, avg_fill_price, commission, broker_order_id, error_message, created_at, updated_at
		FROM orders WHERE id = ?
	`

	var order Order
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&order.ID, &order.ClientOrderID, &order.StrategyID, &order.Symbol, &order.Side, &order.OrderType,
		&order.Quantity, &order.Price, &order.StopPrice, &order.Status, &order.FilledQuantity,
		&order.AvgFillPrice, &order.Commission, &order.BrokerOrderID, &order.ErrorMessage,
		&order.CreatedAt, &order.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return &order, nil
}

// GetOrderByClientOrderID retrieves an order by client order ID
func (r *OrderRepository) GetOrderByClientOrderID(ctx context.Context, clientOrderID string) (*Order, error) {
	query := `
		SELECT id, client_order_id, strategy_id, symbol, side, order_type, quantity, price, stop_price, status, filled_quantity, avg_fill_price, commission, broker_order_id, error_message, created_at, updated_at
		FROM orders WHERE client_order_id = ?
	`

	var order Order
	err := r.db.QueryRowContext(ctx, query, clientOrderID).Scan(
		&order.ID, &order.ClientOrderID, &order.StrategyID, &order.Symbol, &order.Side, &order.OrderType,
		&order.Quantity, &order.Price, &order.StopPrice, &order.Status, &order.FilledQuantity,
		&order.AvgFillPrice, &order.Commission, &order.BrokerOrderID, &order.ErrorMessage,
		&order.CreatedAt, &order.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return &order, nil
}

// ListOrders retrieves orders with filtering
func (r *OrderRepository) ListOrders(ctx context.Context, strategyID, symbol, status *string, limit, offset int) ([]*Order, error) {
	query := `
		SELECT id, client_order_id, strategy_id, symbol, side, order_type, quantity, price, stop_price, status, filled_quantity, avg_fill_price, commission, broker_order_id, error_message, created_at, updated_at
		FROM orders WHERE 1=1
	`
	var args []interface{}

	if strategyID != nil {
		query += " AND strategy_id = ?"
		args = append(args, *strategyID)
	}
	if symbol != nil {
		query += " AND symbol = ?"
		args = append(args, *symbol)
	}
	if status != nil {
		query += " AND status = ?"
		args = append(args, *status)
	}

	query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []*Order
	for rows.Next() {
		var order Order
		err := rows.Scan(
			&order.ID, &order.ClientOrderID, &order.StrategyID, &order.Symbol, &order.Side, &order.OrderType,
			&order.Quantity, &order.Price, &order.StopPrice, &order.Status, &order.FilledQuantity,
			&order.AvgFillPrice, &order.Commission, &order.BrokerOrderID, &order.ErrorMessage,
			&order.CreatedAt, &order.UpdatedAt)
		if err != nil {
			return nil, err
		}
		orders = append(orders, &order)
	}

	return orders, nil
}

// UpdateOrder updates an order
func (r *OrderRepository) UpdateOrder(ctx context.Context, order *Order) error {
	order.UpdatedAt = time.Now()

	query := `
		UPDATE orders
		SET status = ?, filled_quantity = ?, avg_fill_price = ?, commission = ?, broker_order_id = ?, error_message = ?, updated_at = ?
		WHERE id = ?
	`

	_, err := r.db.ExecContext(ctx, query,
		order.Status, order.FilledQuantity, order.AvgFillPrice, order.Commission,
		order.BrokerOrderID, order.ErrorMessage, order.UpdatedAt, order.ID)
	return err
}

// CreateTrade creates a new trade
func (r *OrderRepository) CreateTrade(ctx context.Context, trade *Trade) error {
	trade.ID = uuid.New().String()
	trade.CreatedAt = time.Now()

	query := `
		INSERT INTO trades (id, order_id, strategy_id, symbol, side, quantity, price, commission, broker_trade_id, trade_time, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		trade.ID, trade.OrderID, trade.StrategyID, trade.Symbol, trade.Side,
		trade.Quantity, trade.Price, trade.Commission, trade.BrokerTradeID, trade.TradeTime, trade.CreatedAt)
	return err
}

// GetTradesByOrderID retrieves all trades for an order
func (r *OrderRepository) GetTradesByOrderID(ctx context.Context, orderID string) ([]*Trade, error) {
	query := `
		SELECT id, order_id, strategy_id, symbol, side, quantity, price, commission, broker_trade_id, trade_time, created_at
		FROM trades WHERE order_id = ?
		ORDER BY trade_time ASC
	`

	rows, err := r.db.QueryContext(ctx, query, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trades []*Trade
	for rows.Next() {
		var trade Trade
		err := rows.Scan(
			&trade.ID, &trade.OrderID, &trade.StrategyID, &trade.Symbol, &trade.Side,
			&trade.Quantity, &trade.Price, &trade.Commission, &trade.BrokerTradeID, &trade.TradeTime, &trade.CreatedAt)
		if err != nil {
			return nil, err
		}
		trades = append(trades, &trade)
	}

	return trades, nil
}

// ListTrades retrieves trades with filtering
func (r *OrderRepository) ListTrades(ctx context.Context, strategyID, symbol *string, limit, offset int) ([]*Trade, error) {
	query := `
		SELECT id, order_id, strategy_id, symbol, side, quantity, price, commission, broker_trade_id, trade_time, created_at
		FROM trades WHERE 1=1
	`
	var args []interface{}

	if strategyID != nil {
		query += " AND strategy_id = ?"
		args = append(args, *strategyID)
	}
	if symbol != nil {
		query += " AND symbol = ?"
		args = append(args, *symbol)
	}

	query += " ORDER BY trade_time DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trades []*Trade
	for rows.Next() {
		var trade Trade
		err := rows.Scan(
			&trade.ID, &trade.OrderID, &trade.StrategyID, &trade.Symbol, &trade.Side,
			&trade.Quantity, &trade.Price, &trade.Commission, &trade.BrokerTradeID, &trade.TradeTime, &trade.CreatedAt)
		if err != nil {
			return nil, err
		}
		trades = append(trades, &trade)
	}

	return trades, nil
}