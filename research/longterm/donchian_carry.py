# /// script
# requires-python = ">=3.11"
# dependencies = ["numpy", "pandas"]
# ///
"""日足のドンチャン・ブレイク（50日高値で買い、20日安値で決済）を、FRED の長期データで確かめる。

使い方:
  uv run research/longterm/donchian_carry.py <FRED の CSV を置いたフォルダ>

必要な CSV（https://fred.stlouisfed.org/graph/fredgraph.csv?id=<系列> で取れる）:
  DEXJPUS（ドル円）、DEXUSEU（ユーロドル）、FEDFUNDS（米国の政策金利）、
  IRSTCI01JPM156N（日本の短期金利）、IRSTCI01EZM156N（ユーロ圏の短期金利）

前提と近似:
  - レートは1日1つ（ニューヨーク正午の値）。シグナルはその日の値で判断し、翌日の値で約定する（先の値は使わない）
  - 損切りは「直近 exit_n 日の最安値を終値で割ったら翌日に決済」。日中の高値・安値は無いので、日中の損切りは再現しない
  - 取引のコスト: 往復で cost 円（1ユーロ・1ドルあたり）。既定は 3銭、厳しめに 10銭も見る
  - 金利: 月次の値を1か月遅らせて使う（その月の値は月が終わるまで分からないため）
  - スワップ: 持っている日数 × (相手通貨の金利 − 円の金利 − 1%) を価格に掛けて足す（1% は業者の取り分の見積もり）
  - 損益は「入った値に対する %」で表す（数量に依らない）
"""

import sys

import numpy as np
import pandas as pd


def series(folder, name):
    s = pd.read_csv(f"{folder}/{name}.csv", parse_dates=["observation_date"], index_col="observation_date").iloc[:, 0]
    return pd.to_numeric(s, errors="coerce").dropna()


def monthly_lagged(s, index):
    """月次の値を1か月遅らせ、日次に広げる。"""
    lagged = s.copy()
    lagged.index = lagged.index + pd.offsets.MonthBegin(1)
    return lagged.reindex(index, method="ffill")


def run(price, carry, entry_n=50, exit_n=20, side_rule="long", cost=0.03, use_carry=True, direction=None):
    """side_rule: long（買いだけ）/ carry（金利差がプラスなら買いだけ、マイナスなら売りだけ）/ both（買いも売りも）
    carry はスワップ（業者の取り分を引いた後）、direction は向きの判定に使う金利差そのもの。"""
    p = price.values
    dates = price.index
    hi_entry = price.rolling(entry_n).max().shift(1).values  # 買いの入り: 直近 entry_n 日の最高値
    lo_entry = price.rolling(entry_n).min().shift(1).values  # 売りの入り
    lo_exit = price.rolling(exit_n).min().shift(1).values  # 買いの決済: 直近 exit_n 日の最安値
    hi_exit = price.rolling(exit_n).max().shift(1).values  # 売りの決済
    c = carry.values if carry is not None else np.zeros(len(p))
    d = direction.values if direction is not None else c
    trades = []
    pos, entry_i = 0, None
    action = None  # 翌日に行う動作: "close" / 1（買い）/ -1（売り）
    for i in range(len(p)):
        # 前の日に決めた動作を、この日の値で行う
        if action == "close":
            ret = pos * (p[i] - p[entry_i]) / p[entry_i]
            days = (dates[i] - dates[entry_i]).days
            swap = pos * np.nanmean(c[entry_i:i + 1]) / 100 * days / 365 if use_carry else 0.0
            trades.append((dates[entry_i], dates[i], pos, ret - cost / p[entry_i] + swap, swap))
            pos = 0
        elif action in (1, -1):
            pos, entry_i = action, i
        action = None
        if np.isnan(hi_entry[i]):
            continue
        # この日の値で、翌日の動作を決める
        if pos == 1 and p[i] < lo_exit[i]:
            action = "close"
        elif pos == -1 and p[i] > hi_exit[i]:
            action = "close"
        elif pos == 0:
            up, down = p[i] > hi_entry[i], p[i] < lo_entry[i]
            if side_rule == "long" and up:
                action = 1
            elif side_rule == "carry" and up and d[i] > 0:
                action = 1
            elif side_rule == "carry" and down and d[i] < 0:
                action = -1
            elif side_rule == "both":
                action = 1 if up else (-1 if down else None)
    return pd.DataFrame(trades, columns=["entry", "exit", "side", "ret", "swap"])


def pf(r):
    loss = -r[r < 0].sum()
    return r[r > 0].sum() / loss if loss > 0 else float("inf")


def report(name, t, splits):
    if t.empty:
        print(f"  {name}: 取引なし")
        return
    r = t.ret
    eq = (1 + r).cumprod()
    dd = (1 - eq / eq.cummax()).max()
    line = (f"  {name}: {len(t)}回 勝率 {np.mean(r > 0):.0%} PF {pf(r):.2f} 1回平均 {r.mean() * 100:+.2f}%"
            f" 合計 {r.sum() * 100:+.0f}% 最大DD {dd * 100:.0f}% うちスワップ {t.swap.sum() * 100:+.0f}%")
    print(line)
    parts = []
    for a, b in zip(splits[:-1], splits[1:]):
        m = (t.entry >= a) & (t.entry < b)
        if m.any():
            parts.append(f"{a[:4]}〜: {m.sum()}回 PF {pf(r[m]):.2f} {r[m].sum() * 100:+.0f}%")
    print("      " + " | ".join(parts))


def main():
    folder = sys.argv[1]
    usdjpy = series(folder, "DEXJPUS")
    eurusd = series(folder, "DEXUSEU")
    eurjpy = (usdjpy * eurusd).dropna()
    us, jp, ez = series(folder, "FEDFUNDS"), series(folder, "IRSTCI01JPM156N"), series(folder, "IRSTCI01EZM156N")
    swap_cut = 1.0  # 業者の取り分（%）

    cases = [
        ("USD/JPY", usdjpy, us, ["1971", "1985-07", "1995", "2005", "2015", "2027"], "1985-07-01"),
        ("EUR/JPY", eurjpy, ez, ["1999", "2005", "2015", "2027"], "1999-01-01"),
    ]
    for name, price, base_rate, splits, rate_from in cases:
        price = price[price.index >= splits[0]]
        diff = monthly_lagged(base_rate, price.index) - monthly_lagged(jp, price.index)
        carry = diff - np.sign(diff) * swap_cut
        for cost in (0.03, 0.10):
            print(f"\n### {name} {price.index[0].date()}〜{price.index[-1].date()}  コスト 往復{cost * 100:.0f}銭")
            full = price
            rated = price[price.index >= rate_from]
            c_rated = carry[carry.index >= rate_from]
            d_rated = diff[diff.index >= rate_from]
            report("提案1: 金利差がプラスのときだけ買い（スワップ込み）",
                   run(rated, c_rated, side_rule="carry", cost=cost, direction=d_rated), [s for s in splits if s >= rate_from[:4]] or splits)
            report("買いだけ（金利の判定なし、スワップ込み）", run(rated, c_rated, side_rule="long", cost=cost),
                   [s for s in splits if s >= rate_from[:4]] or splits)
            report("買いも売りも（スワップ込み）", run(rated, c_rated, side_rule="both", cost=cost),
                   [s for s in splits if s >= rate_from[:4]] or splits)
            if name == "USD/JPY":
                report("参考: 1971年から、買いも売りも（スワップなし）",
                       run(full, None, side_rule="both", cost=cost, use_carry=False), splits)


if __name__ == "__main__":
    main()
