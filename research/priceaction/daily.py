# /// script
# requires-python = ">=3.11"
# dependencies = ["numpy", "pandas"]
# ///
"""値動きの読み方を、日足のドンチャン・ブレイク（USD/JPY、金利差がプラスのときだけ買い）に当てはめて、
FRED の長期データ（1985-07〜）で確かめる。元のルールは research/longterm/donchian_carry.py。

使い方:
  uv run research/priceaction/daily.py <FRED の CSV を置いたフォルダ>

確かめること:
  1. 入った時点の状態で取引を分ける（見送り条件の候補）
     - 上の余地: 直近250日の高値までの距離（ATR の何倍か）。高値を更新中なら「上に何もない」
     - もみ合いの狭さ: 直近50日の値幅 ÷ ATR
     - 直前の2回が続けて負けたか
  2. 入り方: 抜けた翌日に入る（元のルール）と、k 日後も50日高値の上に残っているときだけ入る
  3. 決済: 20日安値を割ったら（元のルール）と、直近の押し目安値（前後5日で最も安い終値）を割ったら
ATR は「直近20日の、前日からの変化の絶対値の平均」で代わりにする（日中の高値・安値が無いため）。
"""

import pathlib
import sys

import numpy as np
import pandas as pd

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent.parent / "longterm"))
from donchian_carry import monthly_lagged, pf, series  # noqa: E402

COST = 0.03
SWAP_CUT = 1.0


def run(price, carry, direction, confirm=0, exit_mode="donchian", entry_n=50, exit_n=20, pivot=5):
    p = price.values
    dates = price.index
    s = price
    hi_entry = s.rolling(entry_n).max().shift(1).values
    lo_exit = s.rolling(exit_n).min().shift(1).values
    atr = s.diff().abs().rolling(20).mean().values
    hi250 = s.rolling(250).max().shift(1).values
    width50 = (s.rolling(50).max() - s.rolling(50).min()).shift(1).values
    # 押し目安値: 前後 pivot 日で最も安い終値。確定するのは pivot 日後
    is_pivot = s == s.rolling(2 * pivot + 1, center=True).min()
    pivot_low = s.where(is_pivot).shift(pivot).ffill().values
    c, d = carry.values, direction.values

    trades, pos, entry_i, action, pending = [], 0, None, None, None
    for i in range(len(p)):
        if action == "close":
            ret = (p[i] - p[entry_i]) / p[entry_i]
            days = (dates[i] - dates[entry_i]).days
            swap = np.nanmean(c[entry_i:i + 1]) / 100 * days / 365
            trades.append(dict(entry=dates[entry_i], exit=dates[i], ret=ret - COST / p[entry_i] + swap, **feat))
            pos = 0
        elif action == "open":
            pos, entry_i = 1, i
        action = None
        if np.isnan(hi_entry[i]) or np.isnan(atr[i]):
            continue
        if pos == 1:
            line = lo_exit[i]
            if exit_mode == "pivot" and not np.isnan(pivot_low[i]):
                line = pivot_low[i]
            if p[i] < line:
                action = "close"
            continue
        if pending is not None:
            level, k, f = pending
            if p[i] <= level:
                pending = None  # 内側に戻った: 見送る
            elif k + 1 >= confirm:
                action, feat, pending = "open", f, None
            else:
                pending = (level, k + 1, f)
            continue
        if p[i] > hi_entry[i] and d[i] > 0:
            room = (hi250[i] - p[i]) / atr[i] if not np.isnan(hi250[i]) else np.nan
            last2 = len(trades) >= 2 and trades[-1]["ret"] < 0 and trades[-2]["ret"] < 0
            f = dict(room=room, tight=width50[i] / atr[i], last2_lost=last2)
            if confirm == 0:
                action, feat = "open", f
            else:
                pending = (hi_entry[i], 0, f)
    return pd.DataFrame(trades)


def line(name, t):
    if t.empty:
        return f"  {name}: なし"
    r = t.ret
    eq = (1 + r).cumprod()
    dd = (1 - eq / eq.cummax()).max()
    mid = t.entry.iloc[len(t) // 2]
    a, b = r[t.entry < mid], r[t.entry >= mid]
    return (f"  {name}: {len(t)}回 勝率 {np.mean(r > 0):.0%} PF {pf(r):.2f} 合計 {r.sum() * 100:+.0f}% "
            f"最大DD {dd * 100:.0f}% | 前半 PF {pf(a):.2f} 後半 PF {pf(b):.2f}")


def main():
    folder = sys.argv[1]
    price = series(folder, "DEXJPUS")
    price = price[price.index >= "1985-07-01"]
    jp, us = series(folder, "IRSTCI01JPM156N"), series(folder, "FEDFUNDS")
    diff = monthly_lagged(us, price.index) - monthly_lagged(jp, price.index)
    carry = diff - np.sign(diff) * SWAP_CUT

    base = run(price, carry, diff)
    print(f"USD/JPY {price.index[0].date()}〜{price.index[-1].date()} 往復 {COST * 100:.0f}銭、スワップ込み")
    print(line("元のルール", base))

    print("\n### 1. 入った時点の状態で分ける")
    blue = base.room <= 0
    print(line("250日高値を更新中（上に何もない）", base[blue]))
    q = base.room[~blue].median()
    print(line(f"上に250日高値あり・近い（ATR の {q:.0f}倍以内）", base[~blue & (base.room <= q)]))
    print(line(f"上に250日高値あり・遠い（{q:.0f}倍超）", base[~blue & (base.room > q)]))
    t1, t2 = base.tight.quantile([1 / 3, 2 / 3])
    print(line(f"もみ合いが狭い（50日の値幅 ≦ ATR×{t1:.0f}）", base[base.tight <= t1]))
    print(line("もみ合いが中間", base[(base.tight > t1) & (base.tight < t2)]))
    print(line(f"もみ合いが広い（≧ ATR×{t2:.0f}）", base[base.tight >= t2]))
    print(line("直前に2回続けて負けた", base[base.last2_lost]))
    print(line("それ以外", base[~base.last2_lost]))

    print("\n### 2. 入り方（抜けたあと k 日、50日高値の上に残っていたら入る）")
    for k in (1, 2, 3, 5):
        print(line(f"{k}日残ったら入る", run(price, carry, diff, confirm=k)))

    print("\n### 3. 決済（押し目安値を割ったら）")
    for n in (3, 5, 10):
        print(line(f"押し目安値（前後{n}日）を割ったら決済", run(price, carry, diff, exit_mode="pivot", pivot=n)))


if __name__ == "__main__":
    main()
