-- Create backtests table
CREATE TABLE IF NOT EXISTS backtests (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    strategy_id VARCHAR(36) NOT NULL,
    symbols JSON NOT NULL,
    start_date TIMESTAMP NOT NULL,
    end_date TIMESTAMP NOT NULL,
    parameters JSON NULL,
    status ENUM('pending', 'running', 'completed', 'failed', 'cancelled') NOT NULL DEFAULT 'pending',
    progress DECIMAL(5, 2) DEFAULT 0.00,
    results JSON NULL,
    error TEXT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    completed_at TIMESTAMP NULL,
    
    INDEX idx_strategy_id (strategy_id),
    INDEX idx_status (status),
    INDEX idx_created_at (created_at),
    INDEX idx_completed_at (completed_at),
    
    FOREIGN KEY (strategy_id) REFERENCES strategy_packages(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;