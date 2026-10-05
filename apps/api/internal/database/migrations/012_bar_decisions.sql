-- What the strategy decided on each closed bar, including "no signal", so the
-- UI can show that bars are being processed and why nothing was traded.
-- All DATETIME values are UTC.
CREATE TABLE IF NOT EXISTS bar_decisions (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    deployment_id VARCHAR(36) NOT NULL,
    bar_time DATETIME(3) NOT NULL,
    close DECIMAL(18, 8) NOT NULL,
    -- HOLD, ENTER_LONG, ENTER_SHORT or EXIT: the strategy's signal, which the
    -- risk checks may still refuse (see orders for what was actually sent).
    action VARCHAR(16) NOT NULL,
    -- Side of the position held when the bar was judged: '', 'BUY' or 'SELL'.
    holding VARCHAR(8) NOT NULL DEFAULT '',
    -- Indicator values the strategy saw, as text.
    detail VARCHAR(255) NOT NULL DEFAULT '',
    decided_at DATETIME(3) NOT NULL,

    UNIQUE KEY uk_deployment_bar (deployment_id, bar_time),
    FOREIGN KEY (deployment_id) REFERENCES deployments(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
