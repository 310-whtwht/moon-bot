-- What a trade was expected to do next to what it did, so slippage, R
-- multiples and exit reasons can be read back (trade export, drift check).
--   expected_open_price / expected_close_price: the quote each order was sent
--     at (NULL when the bot did not send it, e.g. a stop executed at the broker)
--   initial_stop_price: the stop when the position was opened (risk = |open - stop| x units)
--   exit_reason: signal, stop, broker, or the kill switch's reason
ALTER TABLE positions
    ADD COLUMN expected_open_price DECIMAL(18, 8) NULL AFTER open_price,
    ADD COLUMN initial_stop_price DECIMAL(18, 8) NULL AFTER stop_price,
    ADD COLUMN expected_close_price DECIMAL(18, 8) NULL AFTER close_price,
    ADD COLUMN exit_reason VARCHAR(64) NULL AFTER realized_pnl,
    ADD INDEX idx_paper_evidence (strategy_version_id, symbol, broker, status);
