# /// script
# requires-python = ">=3.11"
# dependencies = ["numpy", "pandas"]
# ///
"""戦略の値を過去の成績で選び直し続ける（自己改善）と、値を固定するより良くなるかを確かめる。

使い方:
  uv run research/selfimprove/walkforward.py <FRED の CSV のフォルダ> <USD_JPY の5分足 CSV>

比べるやり方（どれも、その時点より前のデータだけで決める）:
  固定      : 最初に決めた値のまま
  選び直し  : 直近の期間で最も成績が良かった候補を、次の期間に使う
  全候補    : 候補すべてに等分する（選ばない）
  止める    : 固定の値で動かし、直近の成績がマイナスなら次の期間は動かさない

1. 日足のドンチャン（USD/JPY・GBP/JPY・AUD/JPY、等分、COT なし）
   候補は entry 20/50/100 × exit 10/20/40/60 の12通り。評価は 2000 年から。1年ごとに、直近10年のシャープで選ぶ
   固定は、最初の値（50/20）と今の値（50/40）。50/40 は結果を見て選んだ値なので、有利に出る
2. 仲値後の売り（USD/JPY 5分足、2023-10〜）
   候補は決済 10:05〜10:50 の10通り。評価は最初の12か月のあとから。1か月ごとに、直近12か月の平均で選ぶ
   見送りの日は gotobi_v2.star と同じ。日中の損切りは再現しない。切り替えのコストも入れない（選び直しに有利）
"""

import pathlib
import re
import sys

import numpy as np
import pandas as pd

ROOT = pathlib.Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT / "longterm"))
sys.path.insert(0, str(ROOT / "cot"))
from donchian_carry import monthly_lagged, series  # noqa: E402
from portfolio import daily_returns  # noqa: E402

JST = pd.Timedelta(hours=9)
SLIP = 0.005


def sharpe(r):
    s = r.std()
    return r.mean() / s if s > 0 else 0.0


def report(name, r, unit):
    """年ごとの合計で集計する。unit は表示の倍率。"""
    eq = r.cumsum()
    dd = (eq.cummax() - eq).max()
    years = (r.index[-1] - r.index[0]).days / 365.25
    y = r.groupby(r.index.year).sum()
    mid = len(r) // 2
    print(f"  {name}: 年 {r.sum() / years * unit:+.2f} 最大DD {dd * unit:.2f} 年÷DD {r.sum() / years / dd:.2f} "
          f"負けた年 {(y < 0).sum()}/{len(y)} | 前半 {r.iloc[:mid].sum() * unit:+.1f} 後半 {r.iloc[mid:].sum() * unit:+.1f}")


def donchian_candidates(folder):
    usd = series(folder, "DEXJPUS")
    jp = series(folder, "IRSTCI01JPM156N")
    pairs = {
        "USD/JPY": (usd, series(folder, "FEDFUNDS"), 0.03),
        "GBP/JPY": ((usd * series(folder, "DEXUSUK")).dropna(), series(folder, "IRSTCI01GBM156N"), 0.05),
        "AUD/JPY": ((usd * series(folder, "DEXUSAL")).dropna(), series(folder, "IR3TIB01AUM156N"), 0.05),
    }
    out = {}
    for entry_n in (20, 50, 100):
        for exit_n in (10, 20, 40, 60):
            rs = {}
            for n, (price, rate, cost) in pairs.items():
                price = price[price.index >= "1985-07-01"]
                diff = monthly_lagged(rate, price.index) - monthly_lagged(jp, price.index)
                rs[n] = daily_returns(price, diff, np.zeros(len(price)), exit_n, False, cost, entry_n)[0]
            out[(entry_n, exit_n)] = pd.DataFrame(rs).fillna(0).mean(axis=1)
    return pd.DataFrame(out)


def select_by_past(c, periods, lookback):
    """periods の区切りごとに、直前 lookback の成績が最良の列を選び、次の区切りまで使う。"""
    r, picks = pd.Series(0.0, index=c.index), []
    for start, end in zip(periods[:-1], periods[1:]):
        past = c[(c.index >= start - lookback) & (c.index < start)]
        best = past.apply(sharpe).idxmax()
        now = c.index[(c.index >= start) & (c.index < end)]
        r.loc[now] = c.loc[now, best].values
        picks.append(best)
    return r, picks


def switch_off(r, periods, lookback):
    """直近 lookback の合計がマイナスなら、次の区切りまで動かさない。"""
    out, off = r.copy(), 0
    for start, end in zip(periods[:-1], periods[1:]):
        if r[(r.index >= start - lookback) & (r.index < start)].sum() < 0:
            out[(out.index >= start) & (out.index < end)] = 0.0
            off += 1
    return out, off


def gotobi_days(index):
    """gotobi_v2.star と同じ判定。"""
    out = []
    for d in index:
        if d.weekday() > 4:
            continue
        dim = d.days_in_month
        days = [5, 10, 15, 20, 25, dim]
        if d.day in days or (d.weekday() == 4 and any(d.day + a <= dim and d.day + a in days for a in (1, 2))):
            out.append(d)
    return out


def gotobi_candidates(path):
    c = pd.read_csv(path, parse_dates=["time"]).set_index("time")
    star = (ROOT.parent / "strategies/research/gotobi_v2.star").read_text()
    skip = {int(x) for x in re.findall(r"\b20\d{6}\b", star)}
    jst = c.index + JST
    days = pd.DatetimeIndex(sorted(set(jst.tz_localize(None).normalize())))
    exits = list(range(5, 55, 5))
    rows = {}
    for d in gotobi_days(days):
        if int(d.strftime("%Y%m%d")) in skip:
            continue
        t0 = (d + pd.Timedelta(hours=9, minutes=55) - JST).tz_localize("UTC")
        if t0 not in c.index:
            continue
        row = {}
        for m in exits:
            t1 = t0 + pd.Timedelta(minutes=m)
            row[m] = c.bid_open[t0] - c.ask_open[t1] - SLIP if t1 in c.index else np.nan
        rows[t0] = row
    return pd.DataFrame(rows).T.dropna()


def main():
    folder, path = sys.argv[1], sys.argv[2]

    print("## 1. ドンチャン 2000〜（損益は資金に対する%、レバレッジ1倍）")
    c = donchian_candidates(folder)
    periods = list(pd.date_range("2000-01-01", "2027-01-01", freq="YS"))
    picked, picks = select_by_past(c, periods, pd.DateOffset(years=10))
    use = c.index >= "2000-01-01"
    report("固定 50/20（最初の値）", c.loc[use, (50, 20)], 100)
    report("固定 50/40（今の値・結果を見て選んだ）", c.loc[use, (50, 40)], 100)
    report("選び直し", picked[use], 100)
    report("全候補", c[use].mean(axis=1), 100)
    print("  選ばれた値:", " ".join(f"{a}/{b}" for a, b in picks))
    print(f"  12通りの年率: 最低 {c[use].sum().min() / 26.8 * 100:+.2f} 中央 {c[use].sum().median() / 26.8 * 100:+.2f} 最高 {c[use].sum().max() / 26.8 * 100:+.2f}")
    off, n = switch_off(c[(50, 40)], periods, pd.DateOffset(years=3))
    report(f"止める（直近3年がマイナスなら1年休む、{n}回）", off[use], 100)

    print("\n## 2. 仲値後の売り（損益は銭、1通貨あたりの合計）")
    g = gotobi_candidates(path)
    print(f"  {len(g)}回 {g.index[0].date()}〜{g.index[-1].date()}")
    periods = list(pd.date_range(g.index[0].normalize() + pd.DateOffset(months=12), g.index[-1] + pd.DateOffset(months=1), freq="MS"))
    picked, picks = select_by_past(g, periods, pd.DateOffset(months=12))
    use = g.index >= periods[0]
    report("固定 10:20（今の値）", g.loc[use, 25], 100)
    report("選び直し", picked[use], 100)
    report("全候補", g[use].mean(axis=1), 100)
    print("  選ばれた決済（9:55 から何分）:", " ".join(str(p) for p in picks))
    print("  候補ごとの合計（銭）:", {int(k): round(v * 100) for k, v in g[use].sum().items()})
    off, n = switch_off(g[25], periods, pd.DateOffset(months=6))
    report(f"止める（直近6か月がマイナスなら1か月休む、{n}回）", off[use], 100)


if __name__ == "__main__":
    main()
