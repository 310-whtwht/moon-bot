-- Seed data for initial development and testing
INSERT INTO strategy_packages (id, name, description, author, is_public, created_at, updated_at) VALUES
  ('11111111-1111-1111-1111-111111111111', 'Sample Strategy', 'Seed strategy package', 'tester', TRUE, NOW(), NOW());

INSERT INTO strategy_versions (id, package_id, version, code, description, is_active, created_at, updated_at) VALUES
  ('22222222-2222-2222-2222-222222222222', '11111111-1111-1111-1111-111111111111', '1.0.0', 'print("hello")', 'Initial version', TRUE, NOW(), NOW());

INSERT INTO strategy_params (id, version_id, param_name, param_type, default_value, description, is_required, created_at, updated_at) VALUES
  ('33333333-3333-3333-3333-333333333333', '22222222-2222-2222-2222-222222222222', 'threshold', 'number', '"10"', 'sample param', TRUE, NOW(), NOW());

INSERT INTO universe_symbols (id, symbol, name, exchange, asset_type, is_active, data_source, last_updated, created_at, updated_at) VALUES
  ('77777777-7777-7777-7777-777777777777', 'AAPL', 'Apple Inc.', 'NASDAQ', 'stock', TRUE, 'seed', NOW(), NOW(), NOW());

INSERT INTO orders (id, client_order_id, strategy_id, symbol, side, order_type, quantity, price, stop_price, status, filled_quantity, avg_fill_price, commission, broker_order_id, error_message, created_at, updated_at) VALUES
  ('44444444-4444-4444-4444-444444444444', 'client-1', '11111111-1111-1111-1111-111111111111', 'AAPL', 'buy', 'market', 10, NULL, NULL, 'filled', 10, 150.00, 1.25, 'broker1', NULL, NOW(), NOW());

INSERT INTO trades (id, order_id, strategy_id, symbol, side, quantity, price, commission, broker_trade_id, trade_time, created_at) VALUES
  ('55555555-5555-5555-5555-555555555555', '44444444-4444-4444-4444-444444444444', '11111111-1111-1111-1111-111111111111', 'AAPL', 'buy', 10, 150.00, 1.25, 'trade1', NOW(), NOW());

INSERT INTO audit_logs (id, level, category, message, strategy_id, symbol, order_id, trade_id, metadata, created_at) VALUES
  ('66666666-6666-6666-6666-666666666666', 'info', 'seed', 'seed log', '11111111-1111-1111-1111-111111111111', 'AAPL', '44444444-4444-4444-4444-444444444444', '55555555-5555-5555-5555-555555555555', '{}', NOW());

INSERT INTO trade_traces (id, strategy_id, symbol, side, quantity, price, timestamp, order_id, trade_id, parent_id, trace_id, metadata, created_at, updated_at) VALUES
  ('88888888-8888-8888-8888-888888888888', '11111111-1111-1111-1111-111111111111', 'AAPL', 'buy', 10, 150.00, NOW(), '44444444-4444-4444-4444-444444444444', '55555555-5555-5555-5555-555555555555', NULL, 'trace-1', '{}', NOW(), NOW());

INSERT INTO backtests (id, name, strategy_id, symbols, start_date, end_date, parameters, status, progress, results, error, created_at, updated_at, completed_at) VALUES
  ('99999999-9999-9999-9999-999999999999', 'Sample Backtest', '11111111-1111-1111-1111-111111111111', '["AAPL"]', NOW() - INTERVAL 30 DAY, NOW(), '{}', 'completed', 100.00, '{}', NULL, NOW(), NOW(), NOW());

