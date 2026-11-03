"""戦略の合否を決める検査。入出力は数値だけで、ボットにも画面にも触らない。

基準を変えるのは、結果を見る前に、人の判断で行う。候補に合わせて緩めない。
"""

import math
from dataclasses import dataclass

import numpy as np

MIN_TRADES = 30
MIN_PF = 1.3
MIN_PF_STRESS = 1.1
MIN_PF_CONCENTRATION = 1.1
MIN_NEIGHBOR_SHARE = 2 / 3
MIN_NEIGHBOR_MEDIAN = 1.1
MAX_RANDOM_P = 0.01
STRESS_SLIPPAGE = 3
# ハーネスより前に手で試した数の概算（数えていないので、多めに見積もった値）
PRIOR_TRIALS = 100


@dataclass
class Gate:
    name: str
    passed: bool
    detail: str


def pf(pnl):
    r = np.asarray(pnl, dtype=float)
    win, loss = r[r > 0].sum(), -r[r < 0].sum()
    if loss > 0:
        return float(win / loss)
    return float("inf") if win > 0 else 0.0


def t_value(pnl):
    r = np.asarray(pnl, dtype=float)
    if len(r) < 2 or r.std(ddof=1) == 0:
        return 0.0
    return float(r.mean() / r.std(ddof=1) * math.sqrt(len(r)))


def required_t(trials):
    """試した数のうち最良のものが、偶然で出る t 値の目安。試すほど上がる。"""
    return math.sqrt(2 * math.log(max(trials, 2)))


def evaluate(pnl, stress_pnl, neighbor_pfs, random_p, trials):
    """pnl は取引順の損益。すべての検査の結果を返す（途中で止めない）。"""
    r = np.asarray(pnl, dtype=float)
    n, third = len(r), len(r) // 3
    need = required_t(trials)
    t = t_value(r)
    a, b = r[: n // 2], r[n // 2 :]
    top_removed = np.sort(r)[:-3] if n > 3 else r[:0]
    share = np.mean([x > 1 for x in neighbor_pfs]) if neighbor_pfs else 0.0
    median = float(np.median(neighbor_pfs)) if neighbor_pfs else 0.0
    last = r[n - third :] if third else r[:0]
    return [
        Gate("回数", n >= MIN_TRADES, f"{n}回（{MIN_TRADES}回以上）"),
        Gate("偶然でないか", t >= need, f"t {t:.2f}（{trials}通り試した分で {need:.2f} 以上）"),
        Gate("利益の厚み", pf(r) >= MIN_PF, f"PF {pf(r):.2f}（{MIN_PF} 以上）"),
        Gate("前半と後半", pf(a) > 1 and pf(b) > 1, f"PF {pf(a):.2f} / {pf(b):.2f}（どちらも 1 超）"),
        Gate("最近の3分の1", pf(last) > 1, f"PF {pf(last):.2f}（1 超）"),
        Gate("大勝ち頼みでないか", pf(top_removed) >= MIN_PF_CONCENTRATION,
             f"大きい3回を除いて PF {pf(top_removed):.2f}（{MIN_PF_CONCENTRATION} 以上）"),
        Gate("コストへの強さ", pf(stress_pnl) >= MIN_PF_STRESS,
             f"滑り{STRESS_SLIPPAGE}倍で PF {pf(stress_pnl):.2f}（{MIN_PF_STRESS} 以上）"),
        Gate("値を少しずらす", bool(neighbor_pfs) and share >= MIN_NEIGHBOR_SHARE and median >= MIN_NEIGHBOR_MEDIAN,
             f"{len(neighbor_pfs)}通り、PF>1 が {share:.0%}、中央 {median:.2f}" if neighbor_pfs else "未指定"),
        Gate("でたらめな売買に勝つか", random_p <= MAX_RANDOM_P, f"p {random_p:.4f}（{MAX_RANDOM_P} 以下）"),
    ]


def random_entry_p(bid, ask, entries, exits, sides, n=2000, seed=1):
    """同じ向き・同じ保有本数で入る場所だけをでたらめにしたとき、実際の成績以上が出る割合。

    bid・ask は足の始値の配列、entries・exits は足の番号。入る時刻の良さだけを比べるので、中値で計算する。
    """
    mid = (np.asarray(bid) + np.asarray(ask)) / 2
    entries, exits, sides = map(np.asarray, (entries, exits, sides))
    actual = (sides * (mid[exits] - mid[entries])).mean()
    bars = exits - entries
    rng = np.random.default_rng(seed)
    start = rng.integers(0, len(mid) - bars.max() - 1, size=(n, len(bars)))
    fake = (sides * (mid[start + bars] - mid[start])).mean(axis=1)
    return float(((fake >= actual).sum() + 1) / (n + 1))
