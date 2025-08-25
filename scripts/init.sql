-- moomoo trading system database initialization

USE moomoo_trading;

-- Strategy packages table
CREATE TABLE strategy_packages (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    version VARCHAR(50) NOT NULL,
    code TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY unique_name_version (name, version)
);

-- Strategy versions table
CREATE TABLE strategy_versions (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    package_id BIGINT NOT NULL,
    version VARCHAR(50) NOT NULL,
    is_active BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (package_id) REFERENCES strategy_packages(id) ON DELETE CASCADE,
    UNIQUE KEY unique_package_version (package_id, version)
);

-- Strategy parameters table
CREATE TABLE strategy_params (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    version_id BIGINT NOT NULL,
    param_name VARCHAR(100) NOT NULL,
    param_type ENUM('int', 'float', 'string', 'bool') NOT NULL,
    default_value TEXT,
    min_value TEXT,
    max_value TEXT,
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (version_id) REFERENCES strategy_versions(id) ON DELETE CASCADE,
    UNIQUE KEY unique_version_param (version_id, param_name)
);

-- Orders table with idempotency key
CREATE TABLE orders (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    client_order_id VARCHAR(255) NOT NULL,
    strategy_id BIGINT,
    symbol VARCHAR(20) NOT NULL,
    side ENUM('BUY', 'SELL') NOT NULL,
    order_type ENUM('MARKET', 'LIMIT', 'STOP', 'STOP_LIMIT', 'TRAILING') NOT NULL,
    quantity DECIMAL(15,6) NOT NULL,
    price DECIMAL(15,6),
    stop_price DECIMAL(15,6),
    status ENUM('PENDING', 'SUBMITTED', 'PARTIAL', 'FILLED', 'CANCELLED', 'REJECTED') NOT NULL,
    filled_quantity DECIMAL(15,6) DEFAULT 0,
    avg_fill_price DECIMAL(15,6),
    commission DECIMAL(15,6) DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY unique_client_order_id (client_order_id)
);

-- Trades table
CREATE TABLE trades (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    order_id BIGINT NOT NULL,
    trade_id VARCHAR(255) NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    side ENUM('BUY', 'SELL') NOT NULL,
    quantity DECIMAL(15,6) NOT NULL,
    price DECIMAL(15,6) NOT NULL,
    commission DECIMAL(15,6) DEFAULT 0,
    trade_time TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (order_id) REFERENCES orders(id) ON DELETE CASCADE,
    UNIQUE KEY unique_trade_id (trade_id)
);

-- Audit logs table
CREATE TABLE audit_logs (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id VARCHAR(255),
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    resource_id VARCHAR(255),
    details JSON,
    ip_address VARCHAR(45),
    user_agent TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_user_action (user_id, action),
    INDEX idx_resource (resource_type, resource_id),
    INDEX idx_created_at (created_at)
);

-- Universe symbols table
CREATE TABLE universe_symbols (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    symbol VARCHAR(20) NOT NULL,
    exchange VARCHAR(10) NOT NULL,
    name VARCHAR(255),
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY unique_symbol_exchange (symbol, exchange)
);

-- Create indexes for better performance
CREATE INDEX idx_orders_strategy_symbol ON orders(strategy_id, symbol);
CREATE INDEX idx_orders_status ON orders(status);
CREATE INDEX idx_orders_created_at ON orders(created_at);
CREATE INDEX idx_trades_symbol_time ON trades(symbol, trade_time);
CREATE INDEX idx_universe_active ON universe_symbols(is_active);

-- Insert some sample universe symbols
INSERT INTO universe_symbols (symbol, exchange, name) VALUES
('AAPL', 'NASDAQ', 'Apple Inc.'),
('GOOGL', 'NASDAQ', 'Alphabet Inc.'),
('MSFT', 'NASDAQ', 'Microsoft Corporation'),
('TSLA', 'NASDAQ', 'Tesla, Inc.'),
('AMZN', 'NASDAQ', 'Amazon.com, Inc.');