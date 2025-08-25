-- Create trade_traces table
CREATE TABLE IF NOT EXISTS trade_traces (
    id VARCHAR(36) PRIMARY KEY,
    strategy_id VARCHAR(255) NOT NULL,
    symbol VARCHAR(50) NOT NULL,
    side ENUM('buy', 'sell') NOT NULL,
    quantity INT NOT NULL,
    price DECIMAL(10, 2) NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    order_id VARCHAR(255) NOT NULL,
    trade_id VARCHAR(255) NOT NULL,
    parent_id VARCHAR(36) NULL,
    trace_id VARCHAR(255) NOT NULL UNIQUE,
    metadata JSON NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    
    INDEX idx_strategy_id (strategy_id),
    INDEX idx_symbol (symbol),
    INDEX idx_timestamp (timestamp),
    INDEX idx_trace_id (trace_id),
    INDEX idx_parent_id (parent_id),
    INDEX idx_order_id (order_id),
    INDEX idx_trade_id (trade_id),
    
    FOREIGN KEY (parent_id) REFERENCES trade_traces(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;