-- Phase 1: market data, positions and multi-broker columns for FX trading.
-- All DATETIME values are UTC.

-- OHLC bars per broker / symbol / timeframe / price type (BID and ASK are stored separately)
CREATE TABLE IF NOT EXISTS bars (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    broker VARCHAR(32) NOT NULL,
    symbol VARCHAR(50) NOT NULL,
    timeframe VARCHAR(8) NOT NULL,
    price_type ENUM('BID', 'ASK') NOT NULL,
    open_time DATETIME(3) NOT NULL,
    open DECIMAL(18, 8) NOT NULL,
    high DECIMAL(18, 8) NOT NULL,
    low DECIMAL(18, 8) NOT NULL,
    close DECIMAL(18, 8) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    UNIQUE KEY uk_bar (broker, symbol, timeframe, price_type, open_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Positions as tracked by the bot, reconciled against the broker
CREATE TABLE IF NOT EXISTS positions (
    id VARCHAR(36) PRIMARY KEY,
    broker VARCHAR(32) NOT NULL,
    account_id VARCHAR(64) NOT NULL,
    broker_position_id VARCHAR(255) NULL,
    symbol VARCHAR(50) NOT NULL,
    side ENUM('buy', 'sell') NOT NULL,
    quantity DECIMAL(20, 8) NOT NULL,
    open_price DECIMAL(18, 8) NOT NULL,
    close_price DECIMAL(18, 8) NULL,
    realized_pnl DECIMAL(20, 8) NULL,
    strategy_id VARCHAR(36) NULL,
    strategy_version_id VARCHAR(36) NULL,
    status ENUM('open', 'closed') NOT NULL DEFAULT 'open',
    opened_at DATETIME(3) NOT NULL,
    closed_at DATETIME(3) NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    UNIQUE KEY uk_broker_position (broker, account_id, broker_position_id),
    INDEX idx_symbol_status (broker, account_id, symbol, status),
    INDEX idx_strategy_version_id (strategy_version_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Orders: broker/account, settle type, position link, strategy version, FX precision
ALTER TABLE orders
    ADD COLUMN broker VARCHAR(32) NOT NULL DEFAULT 'paper' AFTER client_order_id,
    ADD COLUMN account_id VARCHAR(64) NOT NULL DEFAULT 'default' AFTER broker,
    ADD COLUMN strategy_version_id VARCHAR(36) NULL AFTER strategy_id,
    ADD COLUMN settle_type ENUM('open', 'close') NULL AFTER order_type,
    ADD COLUMN broker_position_id VARCHAR(255) NULL AFTER broker_order_id,
    MODIFY quantity DECIMAL(20, 8) NOT NULL,
    MODIFY price DECIMAL(18, 8) NULL,
    MODIFY stop_price DECIMAL(18, 8) NULL,
    MODIFY filled_quantity DECIMAL(20, 8) DEFAULT 0,
    MODIFY avg_fill_price DECIMAL(18, 8) NULL,
    MODIFY commission DECIMAL(18, 8) DEFAULT 0,
    ADD INDEX idx_broker_account (broker, account_id),
    ADD INDEX idx_strategy_version_id (strategy_version_id);

-- Trades: broker/account and FX precision
ALTER TABLE trades
    ADD COLUMN broker VARCHAR(32) NOT NULL DEFAULT 'paper' AFTER order_id,
    ADD COLUMN account_id VARCHAR(64) NOT NULL DEFAULT 'default' AFTER broker,
    MODIFY quantity DECIMAL(20, 8) NOT NULL,
    MODIFY price DECIMAL(18, 8) NOT NULL,
    MODIFY commission DECIMAL(18, 8) DEFAULT 0,
    ADD INDEX idx_broker_account (broker, account_id);

-- Trade traces: FX prices need more than 2 decimals
ALTER TABLE trade_traces
    MODIFY price DECIMAL(18, 8) NOT NULL;

-- Universe: GMO FX is the default data source; replace the stock seed symbol
ALTER TABLE universe_symbols
    MODIFY data_source VARCHAR(50) DEFAULT 'gmo';

DELETE FROM universe_symbols WHERE symbol = 'AAPL' AND data_source = 'seed';

INSERT INTO universe_symbols (id, symbol, name, exchange, asset_type, is_active, data_source, last_updated)
VALUES ('77777777-7777-7777-7777-777777777778', 'USD_JPY', '米ドル/円', 'GMO', 'forex', TRUE, 'gmo', NULL)
ON DUPLICATE KEY UPDATE asset_type = 'forex', data_source = 'gmo';
