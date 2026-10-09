-- Requests to download price history for a symbol and timeframe, made from
-- the UI and carried out by the bot one at a time. All DATETIME values are UTC.
CREATE TABLE IF NOT EXISTS data_imports (
    id VARCHAR(36) PRIMARY KEY,
    broker VARCHAR(32) NOT NULL DEFAULT 'gmo',
    symbol VARCHAR(50) NOT NULL,
    timeframe VARCHAR(8) NOT NULL,
    from_date DATE NOT NULL,
    status ENUM('pending', 'running', 'completed', 'failed', 'cancelled') NOT NULL DEFAULT 'pending',
    bars_stored INT NOT NULL DEFAULT 0,
    -- Where the download has got to, e.g. "ASK 2025-06-01".
    progress VARCHAR(255) NOT NULL DEFAULT '',
    error TEXT NULL,
    created_at DATETIME(3) NOT NULL,
    started_at DATETIME(3) NULL,
    finished_at DATETIME(3) NULL,

    INDEX idx_status_created (status, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
