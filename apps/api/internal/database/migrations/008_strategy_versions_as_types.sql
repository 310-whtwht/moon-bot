-- Phase 2b: strategy versions store a registered strategy type in `code`
-- (e.g. ema_cross) and numeric parameters in strategy_params.
-- Convert the seed version from the old Starlark placeholder.

UPDATE strategy_packages
SET name = 'EMA クロス（サンプル）', description = 'USD_JPY 1時間足向けの EMA クロス戦略のサンプル'
WHERE id = '11111111-1111-1111-1111-111111111111' AND name = 'Sample Strategy';

UPDATE strategy_versions
SET code = 'ema_cross', description = '既定パラメータ'
WHERE id = '22222222-2222-2222-2222-222222222222' AND code = 'print("hello")';

DELETE FROM strategy_params
WHERE version_id = '22222222-2222-2222-2222-222222222222' AND param_name = 'threshold';

INSERT INTO strategy_params (id, version_id, param_name, param_type, default_value, description, is_required)
SELECT * FROM (
    SELECT '33333333-3333-3333-3333-333333333301' AS id, '22222222-2222-2222-2222-222222222222' AS version_id, 'fast_period' AS param_name, 'number' AS param_type, '12' AS default_value, NULL AS description, TRUE AS is_required
    UNION ALL SELECT '33333333-3333-3333-3333-333333333302', '22222222-2222-2222-2222-222222222222', 'slow_period', 'number', '26', NULL, TRUE
    UNION ALL SELECT '33333333-3333-3333-3333-333333333303', '22222222-2222-2222-2222-222222222222', 'atr_period', 'number', '14', NULL, TRUE
    UNION ALL SELECT '33333333-3333-3333-3333-333333333304', '22222222-2222-2222-2222-222222222222', 'stop_atr_mult', 'number', '2', NULL, TRUE
    UNION ALL SELECT '33333333-3333-3333-3333-333333333305', '22222222-2222-2222-2222-222222222222', 'allow_short', 'number', '1', NULL, TRUE
) AS seed
WHERE EXISTS (SELECT 1 FROM strategy_versions WHERE id = '22222222-2222-2222-2222-222222222222' AND code = 'ema_cross')
ON DUPLICATE KEY UPDATE default_value = VALUES(default_value);

-- The seed backtest has no runnable snapshot; mark it failed so the UI does not show it as completed.
UPDATE backtests
SET status = 'failed', error = '旧形式のサンプルデータ（再実行してください）', results = NULL
WHERE id = '99999999-9999-9999-9999-999999999999' AND status = 'completed';
