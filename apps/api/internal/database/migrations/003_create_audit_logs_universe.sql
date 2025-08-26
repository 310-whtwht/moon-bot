-- Create audit_logs table
CREATE TABLE IF NOT EXISTS audit_logs (
    id VARCHAR(36) PRIMARY KEY,
    level ENUM('debug', 'info', 'warn', 'error') NOT NULL,
    category VARCHAR(100) NOT NULL,
    message TEXT NOT NULL,
    strategy_id VARCHAR(36) NULL,
    symbol VARCHAR(50) NULL,
    order_id VARCHAR(36) NULL,
    trade_id VARCHAR(36) NULL,
    metadata JSON NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    INDEX idx_level (level),
    INDEX idx_category (category),
    INDEX idx_strategy_id (strategy_id),
    INDEX idx_symbol (symbol),
    INDEX idx_order_id (order_id),
    INDEX idx_trade_id (trade_id),
    INDEX idx_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Create universe_symbols table
CREATE TABLE IF NOT EXISTS universe_symbols (
    id VARCHAR(36) PRIMARY KEY,
    symbol VARCHAR(50) NOT NULL UNIQUE,
    name VARCHAR(255) NULL,
    exchange VARCHAR(50) NOT NULL,
    asset_type ENUM('stock', 'etf', 'option', 'future', 'forex', 'crypto') NOT NULL,
    is_active BOOLEAN DEFAULT TRUE,
    data_source VARCHAR(50) DEFAULT 'moomoo',
    last_updated TIMESTAMP NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    
    INDEX idx_symbol (symbol),
    INDEX idx_exchange (exchange),
    INDEX idx_asset_type (asset_type),
    INDEX idx_is_active (is_active),
    INDEX idx_data_source (data_source)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;