-- A second paper deployment on a pair that moves more than USD/JPY, on a
-- shorter timeframe, so signals, fills and stop-outs come often enough to
-- exercise the bot. Disabled by default; enable it from the dashboard.
INSERT INTO universe_symbols (id, symbol, name, exchange, asset_type, is_active, data_source, last_updated)
VALUES ('77777777-7777-7777-7777-777777777779', 'GBP_JPY', '英ポンド/円', 'GMO', 'forex', TRUE, 'gmo', NULL)
ON DUPLICATE KEY UPDATE asset_type = 'forex', data_source = 'gmo';

INSERT INTO deployments (id, name, strategy_id, broker, account_id, symbol, timeframe, units, enabled)
SELECT 'dddddddd-dddd-dddd-dddd-ddddddddddd2', 'EMA クロス GBP/JPY 15分足（Paper・動作確認用）', id, 'paper', 'default', 'GBP_JPY', '15m', 100, FALSE
FROM strategy_packages WHERE id = '11111111-1111-1111-1111-111111111111'
ON DUPLICATE KEY UPDATE deployments.symbol = deployments.symbol;
