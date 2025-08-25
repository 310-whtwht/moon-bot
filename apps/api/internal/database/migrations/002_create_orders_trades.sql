-- Create orders table (with idempotency key support)
CREATE TABLE IF NOT EXISTS orders (
    id VARCHAR(36) PRIMARY KEY,
    client_order_id VARCHAR(255) NOT NULL UNIQUE,
    strategy_id VARCHAR(36) NULL,
    symbol VARCHAR(50) NOT NULL,
    side ENUM('buy', 'sell') NOT NULL,
    order_type ENUM('market', 'limit', 'stop', 'stop_limit', 'trailing') NOT NULL,
    quantity DECIMAL(15, 6) NOT NULL,
    price DECIMAL(10, 4) NULL,
    stop_price DECIMAL(10, 4) NULL,
    status ENUM('pending', 'submitted', 'partial', 'filled', 'cancelled', 'rejected') NOT NULL DEFAULT 'pending',
    filled_quantity DECIMAL(15, 6) DEFAULT 0,
    avg_fill_price DECIMAL(10, 4) NULL,
    commission DECIMAL(10, 4) DEFAULT 0,
    broker_order_id VARCHAR(255) NULL,
    error_message TEXT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    
    INDEX idx_client_order_id (client_order_id),
    INDEX idx_strategy_id (strategy_id),
    INDEX idx_symbol (symbol),
    INDEX idx_status (status),
    INDEX idx_created_at (created_at),
    INDEX idx_broker_order_id (broker_order_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Create trades table
CREATE TABLE IF NOT EXISTS trades (
    id VARCHAR(36) PRIMARY KEY,
    order_id VARCHAR(36) NOT NULL,
    strategy_id VARCHAR(36) NULL,
    symbol VARCHAR(50) NOT NULL,
    side ENUM('buy', 'sell') NOT NULL,
    quantity DECIMAL(15, 6) NOT NULL,
    price DECIMAL(10, 4) NOT NULL,
    commission DECIMAL(10, 4) DEFAULT 0,
    broker_trade_id VARCHAR(255) NULL,
    trade_time TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    INDEX idx_order_id (order_id),
    INDEX idx_strategy_id (strategy_id),
    INDEX idx_symbol (symbol),
    INDEX idx_trade_time (trade_time),
    INDEX idx_broker_trade_id (broker_trade_id),
    
    FOREIGN KEY (order_id) REFERENCES orders(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;