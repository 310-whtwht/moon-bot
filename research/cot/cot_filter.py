# /// script
# requires-python = ">=3.11"
# dependencies = ["numpy", "pandas"]
# ///
"""投機筋の偏り（CFTC の COT、円先物）で、取引の成績が変わるかを確かめる。

使い方:
  uv run research/cot/cot_filter.py <COT の JSON> <FRED のフォルダ> [名前=bot backtest -json の結果 ...]

COT の JSON は CFTC の公開 API から取る（円先物の市場コード 097741、従来形式・先物のみ）:
  https://publicreporting.cftc.gov/resource/6dca-aqww.json
  ?$where=cftc_contract_market_code='097741'&$order=report_date_as_yyyy_mm_dd&$limit=5000

やっていること:
  - 投機筋（非商業）の円の買い − 売りを建玉全体で割り、「円買いの偏り」とする
  - その値が、直近3年（156週）の中でどの位置か（0〜100%）を、その週までの値だけで出す
  - 値は火曜時点で、金曜の夕方（米国）に公開される。使えるのは翌週の月曜から（6日遅らせる）
  - 取引を、入った時点の位置で3つ（円売りに偏り / 中間 / 円買いに偏り）に分け、成績を比べる
  - USD/JPY の買いは「円売り」と同じ向き。円売りに偏っているときの USD/JPY の買いは「多数派と同じ向き」
"""

import json
import pathlib
import sys

import numpy as np
import pandas as pd

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent.parent / "longterm"))
from donchian_carry import monthly_lagged, run, series  # noqa: E402

WINDOW = 156


def load_cot(path):
    rows = json.load(open(path))
    c = pd.DataFrame(rows)
    c["date"] = pd.to_datetime(c.report_date_as_yyyy_mm_dd)
    for col in ("noncomm_positions_long_all", "noncomm_positions_short_all", "open_interest_all"):
        c[col] = pd.to_numeric(c[col])
    c["yen_net"] = (c.noncomm_positions_long_all - c.noncomm_positions_short_all) / c.open_interest_all
    c = c.sort_values("date").drop_duplicates("date").set_index("date")
    # 直近 WINDOW 週の中での位置（その週を含む過去だけで計算）
    c["pct"] = c.yen_net.rolling(WINDOW, min_periods=52).apply(lambda w: (w[:-1] < w[-1]).mean() * 100, raw=True)
    c.index = c.index + pd.Timedelta(days=6)  # 使えるようになる日
    return c


def position_at(cot, times):
    idx = cot.index.searchsorted(times, side="right") - 1
    out = np.full(len(times), np.nan)
    ok = idx >= 0
    out[ok] = cot.pct.values[idx[ok]]
    return out


def pf(r):
    loss = -r[r < 0].sum()
    return r[r > 0].sum() / loss if loss > 0 else float("inf")


def bucket(pct):
    return np.where(pct < 33.3, "円売りに偏り", np.where(pct > 66.7, "円買いに偏り", "中間"))


def report(name, t, value_col, unit):
    t = t.dropna(subset=["pct"]).copy()
    t["b"] = bucket(t.pct)
    half = t.entry < t.entry.iloc[0] + (t.entry.iloc[-1] - t.entry.iloc[0]) / 2
    print(f"\n### {name}（{len(t)}回）")
    for b in ("円売りに偏り", "中間", "円買いに偏り"):
        m = t.b == b
        r = t[value_col][m]
        h1, h2 = t[value_col][m & half], t[value_col][m & ~half]
        if len(r) == 0:
            continue
        print(f"  {b:8s} {len(r):4d}回 勝率 {np.mean(r > 0):.0%} PF {pf(r):.2f} 合計 {r.sum():+.1f}{unit}"
              f" | 前半 {len(h1)}回 PF {pf(h1):.2f} / 後半 {len(h2)}回 PF {pf(h2):.2f}")


def main():
    cot = load_cot(sys.argv[1])
    fred = sys.argv[2]
    print(f"COT: {cot.index[0].date()}〜{cot.index[-1].date()}、{len(cot)}週")

    # 1. ドンチャン・ブレイク（USD/JPY、金利差がプラスのときだけ買い、1986年〜）
    usdjpy = series(fred, "DEXJPUS")
    usdjpy = usdjpy[usdjpy.index >= "1986-01-01"]
    diff = monthly_lagged(series(fred, "FEDFUNDS"), usdjpy.index) - monthly_lagged(series(fred, "IRSTCI01JPM156N"), usdjpy.index)
    carry = diff - np.sign(diff) * 1.0
    t = run(usdjpy, carry, side_rule="carry", cost=0.03, direction=diff)
    t["pct"] = position_at(cot, t.entry.values)
    t["ret_pct"] = t.ret * 100
    report("ドンチャン・ブレイク USD/JPY 日足（1986〜、買いだけ）", t, "ret_pct", "%")

    # 2. bot backtest -json の取引
    for arg in sys.argv[3:]:
        name, path = arg.split("=", 1)
        raw = json.load(open(path))
        r = raw[0] if isinstance(raw, list) else raw
        t = pd.DataFrame(r["trades"])
        t["entry"] = pd.to_datetime(t.entry_time, utc=True).dt.tz_localize(None)
        t["pct"] = position_at(cot, t.entry.values)
        report(f"{name}（売り・買いの別なし）", t, "pnl", "円")
        if t.side.nunique() > 1:
            for side, label in (("LONG", "買い"), ("SHORT", "売り")):
                report(f"{name}（{label}だけ）", t[t.side == side], "pnl", "円")


if __name__ == "__main__":
    main()
