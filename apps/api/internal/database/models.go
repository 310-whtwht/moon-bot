package database

import (
	"database/sql"
	"encoding/json"
	"time"
)

// StrategyPackage represents a strategy package
type StrategyPackage struct {
	ID          string    `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Description *string   `json:"description" db:"description"`
	Author      string    `json:"author" db:"author"`
	IsPublic    bool      `json:"is_public" db:"is_public"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// StrategyVersion represents a strategy version
type StrategyVersion struct {
	ID          string    `json:"id" db:"id"`
	PackageID   string    `json:"package_id" db:"package_id"`
	Version     string    `json:"version" db:"version"`
	Code        string    `json:"code" db:"code"`
	Description *string   `json:"description" db:"description"`
	IsActive    bool      `json:"is_active" db:"is_active"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// StrategyParam represents a strategy parameter
type StrategyParam struct {
	ID           string  `json:"id" db:"id"`
	VersionID    string  `json:"version_id" db:"version_id"`
	ParamName    string  `json:"param_name" db:"param_name"`
	ParamType    string  `json:"param_type" db:"param_type"`
	DefaultValue *string `json:"default_value" db:"default_value"`
	Description  *string `json:"description" db:"description"`
	IsRequired   bool    `json:"is_required" db:"is_required"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// Order represents a trading order
type Order struct {
	ID             string          `json:"id" db:"id"`
	ClientOrderID  string          `json:"client_order_id" db:"client_order_id"`
	StrategyID     *string         `json:"strategy_id" db:"strategy_id"`
	Symbol         string          `json:"symbol" db:"symbol"`
	Side           string          `json:"side" db:"side"`
	OrderType      string          `json:"order_type" db:"order_type"`
	Quantity       float64         `json:"quantity" db:"quantity"`
	Price          *float64        `json:"price" db:"price"`
	StopPrice      *float64        `json:"stop_price" db:"stop_price"`
	Status         string          `json:"status" db:"status"`
	FilledQuantity float64         `json:"filled_quantity" db:"filled_quantity"`
	AvgFillPrice   *float64        `json:"avg_fill_price" db:"avg_fill_price"`
	Commission     float64         `json:"commission" db:"commission"`
	BrokerOrderID  *string         `json:"broker_order_id" db:"broker_order_id"`
	ErrorMessage   *string         `json:"error_message" db:"error_message"`
	CreatedAt      time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at" db:"updated_at"`
}

// Trade represents a trade execution
type Trade struct {
	ID            string    `json:"id" db:"id"`
	OrderID       string    `json:"order_id" db:"order_id"`
	StrategyID    *string   `json:"strategy_id" db:"strategy_id"`
	Symbol        string    `json:"symbol" db:"symbol"`
	Side          string    `json:"side" db:"side"`
	Quantity      float64   `json:"quantity" db:"quantity"`
	Price         float64   `json:"price" db:"price"`
	Commission    float64   `json:"commission" db:"commission"`
	BrokerTradeID *string   `json:"broker_trade_id" db:"broker_trade_id"`
	TradeTime     time.Time `json:"trade_time" db:"trade_time"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

// AuditLog represents an audit log entry
type AuditLog struct {
	ID         string          `json:"id" db:"id"`
	Level      string          `json:"level" db:"level"`
	Category   string          `json:"category" db:"category"`
	Message    string          `json:"message" db:"message"`
	StrategyID *string         `json:"strategy_id" db:"strategy_id"`
	Symbol     *string         `json:"symbol" db:"symbol"`
	OrderID    *string         `json:"order_id" db:"order_id"`
	TradeID    *string         `json:"trade_id" db:"trade_id"`
	Metadata   json.RawMessage `json:"metadata" db:"metadata"`
	CreatedAt  time.Time       `json:"created_at" db:"created_at"`
}

// UniverseSymbol represents a symbol in the trading universe
type UniverseSymbol struct {
	ID          string     `json:"id" db:"id"`
	Symbol      string     `json:"symbol" db:"symbol"`
	Name        *string    `json:"name" db:"name"`
	Exchange    string     `json:"exchange" db:"exchange"`
	AssetType   string     `json:"asset_type" db:"asset_type"`
	IsActive    bool       `json:"is_active" db:"is_active"`
	DataSource  string     `json:"data_source" db:"data_source"`
	LastUpdated *time.Time `json:"last_updated" db:"last_updated"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
}