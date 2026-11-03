# /// script
# requires-python = ">=3.11"
# dependencies = ["numpy", "pytest"]
# ///
"""検査そのものの試験。 uv run research/lab/test_gates.py"""

import pathlib
import sys

import numpy as np
import pytest

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))
import gates  # noqa: E402


def passed(pnl, **kw):
    args = dict(stress_pnl=pnl, neighbor_pfs=[1.5, 1.6, 1.4], random_p=0.001, trials=100)
    args.update(kw)
    return {g.name: g.passed for g in gates.evaluate(pnl, args.pop("stress_pnl"), **args)}


GOOD = list(np.tile([10.0, 12.0, -5.0, 9.0, 11.0, -4.0, 8.0, 10.0, -3.0, 9.0], 6))


def test_good_series_passes_everything():
    assert all(passed(GOOD).values())


def test_more_trials_require_more_t():
    assert gates.required_t(1000) > gates.required_t(100) > gates.required_t(10)


def test_few_trades_fail():
    assert not passed(GOOD[:20])["回数"]


def test_one_big_win_fails_concentration():
    pnl = [-1.0, 1.0] * 20 + [500.0, 400.0, 300.0]
    assert not passed(pnl)["大勝ち頼みでないか"]


def test_profit_only_in_first_half_fails():
    pnl = [10.0, -1.0] * 20 + [-10.0, 1.0] * 20
    assert not passed(pnl)["前半と後半"]


def test_missing_neighbors_fail():
    assert not passed(GOOD, neighbor_pfs=[])["値を少しずらす"]


def test_cost_stress_fails():
    assert not passed(GOOD, stress_pnl=[-1.0, 1.0] * 30)["コストへの強さ"]


def test_random_entry_detects_real_timing_and_rejects_luck():
    rng = np.random.default_rng(0)
    mid = 100 + np.cumsum(rng.normal(0, 0.1, 5000))
    # 上がる直前を選んで買う（未来を見た売買）
    k = 5
    best = np.argsort(mid[:-k] - mid[k:])[:50]
    assert gates.random_entry_p(mid, mid, best, best + k, np.ones(50)) <= 0.01
    # でたらめに買う
    pick = rng.integers(0, 4900, 50)
    assert gates.random_entry_p(mid, mid, pick, pick + k, np.ones(50)) > 0.01


if __name__ == "__main__":
    sys.exit(pytest.main([__file__, "-q"]))
