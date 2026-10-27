-- Outside data that strategy scripts read by name with data(name), such as
-- the CFTC positioning of speculators (cot_jpy_pct). The bot refreshes it.
-- available_at is when a value became public (UTC): a strategy deciding at t
-- sees the latest value with available_at <= t.
CREATE TABLE IF NOT EXISTS data_series (
    name VARCHAR(64) NOT NULL,
    available_at DATETIME NOT NULL,
    value DOUBLE NOT NULL,
    PRIMARY KEY (name, available_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
