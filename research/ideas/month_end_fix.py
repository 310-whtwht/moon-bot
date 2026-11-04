# /// script
# requires-python = ">=3.11"
# dependencies = ["numpy", "pandas"]
# ///
"""round2.py で基準を通った「月末のフィキシングのあとに買う」を、偶然かどうか疑って確かめる。

使い方:
  uv run research/ideas/month_end_fix.py <5分足 CSV> [...]
"""

import pathlib
import sys

import numpy as np
import pandas as pd

ROOT = pathlib.Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT / "priceaction"))
from intraday import load, pf, trade  # noqa: E402
from round2 import judge  # noqa: E402


def fix_time(d):
    return (pd.Timestamp(d.date(), tz="Europe/London") + pd.Timedelta(hours=16)).tz_convert("UTC")


def rows(c, days, side=1, hold=60, shift=0):
    out = []
    for d in days:
        t0 = fix_time(d) + pd.Timedelta(minutes=shift)
        t1 = t0 + pd.Timedelta(minutes=hold)
        if t0 in c.index and t1 in c.index:
            out.append((t0, trade(c, t0, t1, side)))
    return out


def main():
    for path in sys.argv[1:]:
        c = load(path)
        c = c[~c.index.duplicated()]
        bd = pd.bdate_range(c.index[0].normalize().tz_localize(None), c.index[-1].normalize().tz_localize(None))
        s = pd.Series(bd, index=bd)
        last = s.groupby(s.index.to_period("M")).transform("max") == s
        pos = np.where(last.values)[0]
        print(f"\n## {pathlib.Path(path).name}")
        judge("月末 60分", rows(c, bd[pos]))
        print("### 月末以外の日（比べる相手）")
        judge("月末以外すべて", rows(c, bd[~last.values]))
        print("### 日をずらす（偽物なら、ここでも出る）")
        for k in (-3, -2, -1, 1, 2, 3):
            judge(f"月末{k:+d}営業日", rows(c, bd[np.clip(pos + k, 0, len(bd) - 1)]))
        print("### 時間の長さと、始める時刻")
        for hold in (15, 30, 90, 120, 180):
            judge(f"{hold}分", rows(c, bd[pos], hold=hold))
        for shift in (-30, -15, 15, 30):
            judge(f"{shift:+d}分から60分", rows(c, bd[pos], shift=shift))
        r = pd.DataFrame(rows(c, bd[pos]), columns=["time", "ret"]).set_index("time").ret
        print("### 年ごと（銭）", (r.groupby(r.index.year).sum() * 100).round(0).to_dict())
        print(f"### 外した場合: 最大の1回 {r.max() * 100:.0f}銭、大きい3回を除いた PF {pf(r.sort_values().iloc[:-3].values):.2f}")
        print("### 月ごと（銭）", (r.groupby(r.index.month).mean() * 100).round(0).to_dict())


if __name__ == "__main__":
    main()
