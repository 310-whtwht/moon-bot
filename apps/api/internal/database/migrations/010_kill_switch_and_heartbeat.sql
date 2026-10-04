-- Phase 3b: kill switch (global and per broker) and the bot heartbeat.

-- scope is 'global' or a broker name (e.g. 'paper', 'gmo').
-- An active switch blocks new entries for its scope; exits and stop-losses
-- keep working. With close_positions the bot also closes open positions.
CREATE TABLE IF NOT EXISTS kill_switches (
    scope VARCHAR(32) PRIMARY KEY,
    active BOOLEAN NOT NULL DEFAULT FALSE,
    close_positions BOOLEAN NOT NULL DEFAULT FALSE,
    reason VARCHAR(255) NULL,
    updated_by VARCHAR(64) NULL,
    updated_at DATETIME(3) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO kill_switches (scope, active, close_positions, updated_at)
VALUES ('global', FALSE, FALSE, UTC_TIMESTAMP(3))
ON DUPLICATE KEY UPDATE scope = scope;

-- One row per bot instance, refreshed on every poll, so the UI can tell
-- whether the bot is alive.
CREATE TABLE IF NOT EXISTS bot_heartbeats (
    instance VARCHAR(128) PRIMARY KEY,
    started_at DATETIME(3) NOT NULL,
    last_seen_at DATETIME(3) NOT NULL,
    poll_interval_seconds INT NOT NULL,
    runners INT NOT NULL DEFAULT 0
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
