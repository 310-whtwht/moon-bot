-- Allow several deployments on the same account and symbol (for example a
-- swing strategy and an intraday one on USD/JPY). Positions are already
-- tracked per deployment; the bot leaves the other deployments' positions out
-- when it compares its own with the broker.
ALTER TABLE deployments
    DROP INDEX uk_account_symbol,
    ADD INDEX idx_account_symbol (broker, account_id, symbol);
