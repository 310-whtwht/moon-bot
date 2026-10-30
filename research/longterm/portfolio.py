# /// script
# requires-python = ">=3.11"
# dependencies = ["numpy", "pandas"]
# ///
"""日足のドンチャン・ブレイクを、円の4銘柄でまとめて動かしたときの成績を見る。

使い方:
  uv run research/longterm/portfolio.py <FRED の CSV のフォルダ> <CFTC の円先物の JSON>

必要な FRED の系列: DEXJPUS、DEXUSEU、DEXUSUK、DEXUSAL、FEDFUNDS、IRSTCI01JPM156N、IRSTCI01EZM156N、
  IRSTCI01GBM156N、IR3TIB01AUM156N（豪は 3か月金利。政策金利の系列は 1990 年からしかないため）

ルール（銘柄ごと、同じ額）: 金利差がプラスのときだけ、50日高値で買い、exit_n 日安値で決済。売りはしない。
COT あり: 円の投機筋の位置が 66.7 より上（円買いに偏り）のときは入らない。どの銘柄でも円の COT を使う。
シグナルはその日の値で判断し、翌日の値で約定する。1銘柄の建玉 = 資金の 1/銘柄数（レバレッジ 1倍）。
コストは往復 USD/JPY 3銭、ほか 5銭。スワップは (金利差 − 1%)。日中の損切りは再現しない。
"""

import pathlib
import sys

import numpy as np
import pandas as pd

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))
sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent.parent / "cot"))
from cot_filter import load_cot, position_at  # noqa: E402
from donchian_carry import monthly_lagged, series  # noqa: E402

SWAP_CUT = 1.0


def daily_returns(price, diff, cot_pct, exit_n, use_cot, cost, entry_n=50):
    """銘柄の日々の損益（建玉に対する割合）を返す。"""
    p = price.values
    hi = price.rolling(entry_n).max().shift(1).values
    lo = price.rolling(exit_n).min().shift(1).values
    d = diff.values
    carry = d - np.sign(d) * SWAP_CUT
    gap = np.r_[1, np.diff(price.index.values) / np.timedelta64(1, "D")]
    pos = np.zeros(len(p))
    trades = 0
    for i in range(1, len(p)):
        pos[i] = pos[i - 1]  # pos[i] は i の値で約定した後の建玉（i−1 の判断による）
        if i < 2 or np.isnan(hi[i - 1]) or np.isnan(lo[i - 1]):
            continue
        s, h, l = p[i - 1], hi[i - 1], lo[i - 1]
        if pos[i - 1] == 1 and s < l:
            pos[i] = 0
        elif pos[i - 1] == 0 and s > h and d[i - 1] > 0 and not (use_cot and cot_pct[i - 1] > 66.7):
            pos[i] = 1
            trades += 1
    r = np.r_[0, p[1:] / p[:-1] - 1] * np.r_[0, pos[:-1]]
    r = r + np.r_[0, pos[:-1]] * carry / 100 * gap / 365
    r = r - np.abs(np.r_[0, np.diff(pos)]) * (cost / 2) / p
    return pd.Series(r, index=price.index), trades


def stats(name, r, trades=None):
    eq = r.cumsum()
    dd = (eq.cummax() - eq).max()
    years = (r.index[-1] - r.index[0]).days / 365.25
    ann = r.sum() / years
    vol = r.std() * np.sqrt(252)
    y = r.groupby(r.index.year).sum()
    n = f"{trades}回（年 {trades / years:.1f}回） " if trades is not None else ""
    mid = len(r) // 2
    print(f"  {name}: {n}年 {ann * 100:+.1f}% 最大DD {dd * 100:.0f}% 年÷DD {ann / dd:.2f} シャープ {ann / vol:.2f} "
          f"負けた年 {(y < 0).sum()}/{len(y)} 最悪の年 {y.min() * 100:+.0f}% "
          f"| 前半 年{r.iloc[:mid].sum() / years * 200:+.1f}% 後半 年{r.iloc[mid:].sum() / years * 200:+.1f}%")


def main():
    folder, cot_path = sys.argv[1], sys.argv[2]
    usd = series(folder, "DEXJPUS")
    jp = series(folder, "IRSTCI01JPM156N")
    pairs = {
        "USD/JPY": (usd, series(folder, "FEDFUNDS"), 0.03),
        "EUR/JPY": ((usd * series(folder, "DEXUSEU")).dropna(), series(folder, "IRSTCI01EZM156N"), 0.05),
        "GBP/JPY": ((usd * series(folder, "DEXUSUK")).dropna(), series(folder, "IRSTCI01GBM156N"), 0.05),
        "AUD/JPY": ((usd * series(folder, "DEXUSAL")).dropna(), series(folder, "IR3TIB01AUM156N"), 0.05),
    }
    cot = load_cot(cot_path)
    for start, names in (("1987-01-01", ["USD/JPY", "GBP/JPY", "AUD/JPY"]),
                         ("1999-06-01", ["USD/JPY", "EUR/JPY", "GBP/JPY", "AUD/JPY"]),
                         ("2015-01-01", ["USD/JPY", "EUR/JPY", "GBP/JPY", "AUD/JPY"])):
        print(f"\n## {start}〜 {len(names)}銘柄")
        for exit_n, use_cot in ((20, False), (20, True), (40, False), (40, True)):
            print(f"\n### {exit_n}日安値で決済・COT {'あり' if use_cot else 'なし'}")
            rs, total = {}, 0
            for n in names:
                price, rate, cost = pairs[n]
                # 判断の指標を温めるため、開始より前から動かして切り出す
                price = price[price.index >= "1985-07-01"]
                diff = monthly_lagged(rate, price.index) - monthly_lagged(jp, price.index)
                pct = position_at(cot, price.index.values)
                r, _ = daily_returns(price, diff, pct, exit_n, use_cot, cost)
                r = r[r.index >= start]
                rs[n] = r
                stats(n, r)
            df = pd.DataFrame(rs).fillna(0)
            stats("等分の合計", df.mean(axis=1))
            print("  月次の相関:", df.resample("ME").sum().corr().round(2).values[np.triu_indices(len(names), 1)].tolist())


if __name__ == "__main__":
    main()
