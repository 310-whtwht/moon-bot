# /// script
# requires-python = ">=3.11"
# dependencies = ["numpy", "pandas"]
# ///
"""日足の新しい考え方8つを、1つの基準で確かめる。案と基準は、結果を見る前に決めた。

使い方:
  uv run research/ideas/round1.py <FRED の CSV のフォルダ>
必要な FRED の系列: portfolio.py のものに加えて DGS2、VIXCLS

合格の基準: 日次損益の t 値 3 以上、前半・後半とも黒字、年率 ÷ 最大DD 0.3 以上
約定はシグナルの翌日の値。DGS2 と VIXCLS は FX の値（NY 正午）より後に決まるので、さらに1日遅らせる。
建玉は資金に対してレバレッジ1倍。コストは往復 USD/JPY 3銭、ほか 5銭。スワップは (金利差 − 1%)。
"""

import pathlib
import sys

import numpy as np
import pandas as pd

ROOT = pathlib.Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT / "longterm"))
sys.path.insert(0, str(ROOT / "cot"))
from donchian_carry import monthly_lagged, series  # noqa: E402
from portfolio import daily_returns  # noqa: E402

SWAP_CUT = 1.0


def load(folder):
    usd = series(folder, "DEXJPUS")
    jp = series(folder, "IRSTCI01JPM156N")
    raw = {
        "USD/JPY": (usd, series(folder, "FEDFUNDS"), 0.03),
        "EUR/JPY": ((usd * series(folder, "DEXUSEU")).dropna(), series(folder, "IRSTCI01EZM156N"), 0.05),
        "GBP/JPY": ((usd * series(folder, "DEXUSUK")).dropna(), series(folder, "IRSTCI01GBM156N"), 0.05),
        "AUD/JPY": ((usd * series(folder, "DEXUSAL")).dropna(), series(folder, "IR3TIB01AUM156N"), 0.05),
    }
    out = {}
    for n, (price, rate, cost) in raw.items():
        price = price[price.index >= "1985-07-01"]
        out[n] = (price, monthly_lagged(rate, price.index) - monthly_lagged(jp, price.index), cost)
    return out


def from_signal(price, diff, cost, sig):
    """sig はその日の判断（+1 / 0 / -1 や小数）。翌日の値で約定する。"""
    pos = pd.Series(sig, index=price.index).fillna(0).shift(1).fillna(0).values
    p, d = price.values, diff.values
    gap = np.r_[1, np.diff(price.index.values) / np.timedelta64(1, "D")]
    held = np.r_[0, pos[:-1]]
    r = held * np.r_[0, p[1:] / p[:-1] - 1]
    r = r + (held * d - np.abs(held) * SWAP_CUT) / 100 * gap / 365
    r = r - np.abs(np.r_[0, np.diff(pos)]) * (cost / 2) / p
    return pd.Series(r, index=price.index)


def judge(name, r, start="1991-01-01"):
    r = r[r.index >= start]
    r = r[r.index <= "2026-12-31"]
    eq = r.cumsum()
    dd = (eq.cummax() - eq).max()
    years = (r.index[-1] - r.index[0]).days / 365.25
    ann = r.sum() / years
    t = r.mean() / r.std() * np.sqrt(len(r))
    mid = len(r) // 2
    a, b = r.iloc[:mid].sum(), r.iloc[mid:].sum()
    y = r.groupby(r.index.year).sum()
    ok = t >= 3 and a > 0 and b > 0 and ann / dd >= 0.3
    print(f"  {'○' if ok else '×'} {name}: 年 {ann * 100:+.2f}% DD {dd * 100:.0f}% 年÷DD {ann / dd:.2f} t {t:.1f} "
          f"前半 {a * 100:+.0f}% 後半 {b * 100:+.0f}% 負けた年 {(y < 0).sum()}/{len(y)}")


def basket(rs):
    return pd.DataFrame(rs).fillna(0).mean(axis=1)


def main():
    folder = sys.argv[1]
    m = load(folder)
    trio = ["USD/JPY", "GBP/JPY", "AUD/JPY"]
    two = series(folder, "DGS2").reindex(m["USD/JPY"][0].index).ffill().shift(1)
    vix = series(folder, "VIXCLS").reindex(m["USD/JPY"][0].index).ffill().shift(1)

    base = {n: daily_returns(m[n][0], m[n][1], np.zeros(len(m[n][0])), 40, False, m[n][2])[0] for n in trio}
    print("## 比べる相手")
    judge("ドンチャン 50/40 3銘柄", basket(base))

    print("\n## 1. 米2年金利の勢い（20日の変化の向きに USD/JPY を持つ、売りあり）")
    price, diff, cost = m["USD/JPY"]
    sig = np.sign(two - two.shift(20))
    judge("米2年 20日", from_signal(price, diff, cost, sig))

    print("\n## 2. 12か月の勢い（月末に判定、売りあり、4銘柄）")
    rs = {}
    for n, (price, diff, cost) in m.items():
        s = np.sign(price / price.shift(252) - 1)
        last = pd.Series(price.index.to_period("M"), index=price.index)
        s = s.where(last != last.shift(-1)).ffill()
        rs[n] = from_signal(price, diff, cost, s)
    judge("12か月の勢い", basket(rs))

    print("\n## 3. ドンチャン + VIX（VIX 25 超の日は持たない）")
    rs = {n: base[n] * (vix.reindex(base[n].index).shift(1) <= 25) for n in trio}
    judge("VIX で外す（コストは入れていない）", basket(rs))

    print("\n## 4. ドンチャン + 建玉の調整（値動きが大きいほど小さく。目標 年8%、上限2倍）")
    rs = {}
    for n in trio:
        vol = m[n][0].pct_change().rolling(60).std() * np.sqrt(252)
        rs[n] = base[n] * (0.08 / vol).clip(upper=2).shift(2).fillna(0)
    judge("建玉の調整", basket(rs))

    print("\n## 5. 上昇中の押し目買い（200日線の上で、3日で ATR×2 下げたら買い、5日保有、4銘柄）")
    rs = {}
    for n, (price, diff, cost) in m.items():
        atr = price.diff().abs().rolling(20).mean()
        hit = (price > price.rolling(200).mean()) & (price - price.shift(3) < -2 * atr)
        rs[n] = from_signal(price, diff, cost, hit.astype(float).rolling(5, min_periods=1).max())
    judge("押し目買い", basket(rs))

    print("\n## 6. EUR/GBP の行き過ぎの戻り（60日の z が ±2 で逆張り、±0.5 で決済、スワップなし、往復 0.07%）")
    eg = (series(folder, "DEXUSEU") / series(folder, "DEXUSUK")).dropna()
    z = ((eg - eg.rolling(60).mean()) / eg.rolling(60).std()).values
    pos, now = np.zeros(len(eg)), 0
    for i in range(len(eg)):
        if now == 0 and abs(z[i]) > 2:
            now = -np.sign(z[i])
        elif now != 0 and abs(z[i]) < 0.5:
            now = 0
        pos[i] = now
    held = np.r_[0, pos[:-1]]
    held = np.r_[0, held[:-1]]
    r = held * np.r_[0, eg.values[1:] / eg.values[:-1] - 1] - np.abs(np.r_[0, np.diff(held)]) * 0.00035
    judge("EUR/GBP", pd.Series(r, index=eg.index), "2000-01-01")

    print("\n## 7. 月またぎ（月末2日と月初2日だけ買う、USD/JPY）")
    price, diff, cost = m["USD/JPY"]
    per = pd.Series(price.index.to_period("M"), index=price.index)
    from_end = per.groupby(per).transform(lambda x: np.arange(len(x))[::-1])
    from_start = per.groupby(per).transform(lambda x: np.arange(len(x)))
    # シグナルの翌日に約定するので、月末3日前の値で判断する
    tom = ((from_end <= 3) & (from_end >= 1)) | (from_start <= 0)
    judge("買い", from_signal(price, diff, cost, tom.astype(float)))
    judge("売り", from_signal(price, diff, cost, -tom.astype(float)))

    print("\n## 8. VIX の急騰（5日で 30% 上昇したら AUD/JPY を5日）")
    price, diff, cost = m["AUD/JPY"]
    v = vix.reindex(price.index).ffill()
    hit = (v / v.shift(5) > 1.3).astype(float).rolling(5, min_periods=1).max()
    judge("売り（リスク回避が続く）", from_signal(price, diff, cost, -hit))
    judge("買い（行き過ぎが戻る）", from_signal(price, diff, cost, hit))


if __name__ == "__main__":
    main()
