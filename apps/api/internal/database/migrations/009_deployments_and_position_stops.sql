-- Phase 3: deployments (which strategy trades which instrument on which
-- account) and the columns the bot needs on positions and orders.

CREATE TABLE IF NOT EXISTS deployments (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    strategy_id VARCHAR(36) NOT NULL,
    broker VARCHAR(32) NOT NULL DEFAULT 'paper',
    account_id VARCHAR(64) NOT NULL DEFAULT 'default',
    symbol VARCHAR(50) NOT NULL,
    timeframe VARCHAR(8) NOT NULL,
    units DECIMAL(20, 8) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    -- One deployment per account and instrument: positions are tracked per deployment.
    UNIQUE KEY uk_account_symbol (broker, account_id, symbol),
    INDEX idx_enabled (enabled),

    FOREIGN KEY (strategy_id) REFERENCES strategy_packages(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

ALTER TABLE positions
    ADD COLUMN deployment_id VARCHAR(36) NULL AFTER id,
    ADD COLUMN stop_price DECIMAL(18, 8) NULL AFTER open_price,
    ADD COLUMN fees DECIMAL(18, 8) NOT NULL DEFAULT 0 AFTER realized_pnl,
    ADD INDEX idx_deployment_status (deployment_id, status),
    ADD INDEX idx_closed_at (closed_at);

ALTER TABLE orders
    ADD COLUMN deployment_id VARCHAR(36) NULL AFTER strategy_version_id,
    ADD INDEX idx_deployment_id (deployment_id);

-- Sample deployment: the sample EMA cross strategy on the paper account.
-- Disabled by default; enable it to start paper trading.
INSERT INTO deployments (id, name, strategy_id, broker, account_id, symbol, timeframe, units, enabled)
SELECT 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'EMA クロス USD/JPY 1時間足（Paper）', id, 'paper', 'default', 'USD_JPY', '1h', 100, FALSE
FROM strategy_packages WHERE id = '11111111-1111-1111-1111-111111111111'
ON DUPLICATE KEY UPDATE name = VALUES(name);
