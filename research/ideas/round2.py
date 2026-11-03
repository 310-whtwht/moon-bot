# /// script
# requires-python = ">=3.11"
# dependencies = ["numpy", "pandas"]
# ///
"""決まった理由で注文が出る時間帯の3案を、5分足で確かめる。案と基準は、結果を見る前に決めた。

使い方:
  uv run research/ideas/round2.py <USD_JPY の5分足 CSV> <GBP_JPY の5分足 CSV>

合格の基準: 30回以上、t 値 3 以上、PF 1.3 以上、前半・後半とも PF 1 超
1. 指標のあとの流れ: 雇用統計・CPI・FOMC の発表から15分の向きに入り、60分・240分持つ
2. 月末のロンドン・フィキシング（16:00 ロンドン）: 月の最後の平日の、前60分と後60分を買い・売りの両方で
3. 仲値後の売り（ゴトー日、決済 10:20）を、仲値前（8:00〜9:55）の動きの向きで分ける
約定は intraday.py と同じ（スプレッド込み、滑り 0.5銭）。
"""

import pathlib
import re
import sys

import numpy as np
import pandas as pd

ROOT = pathlib.Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT / "priceaction"))
sys.path.insert(0, str(ROOT / "selfimprove"))
from intraday import JST, load, pf, trade  # noqa: E402
from walkforward import gotobi_days  # noqa: E402


def judge(name, rows):
    rows = [x for x in rows if x[1] is not None]
    if len(rows) < 5:
        print(f"  × {name}: {len(rows)}回")
        return
    df = pd.DataFrame(rows, columns=["time", "ret"]).sort_values("time")
    r = df.ret.values
    t = r.mean() / r.std() * np.sqrt(len(r))
    a, b = r[: len(r) // 2], r[len(r) // 2:]
    ok = len(r) >= 30 and t >= 3 and pf(r) >= 1.3 and pf(a) > 1 and pf(b) > 1
    print(f"  {'○' if ok else '×'} {name}: {len(r)}回 勝率 {np.mean(r > 0):.0%} PF {pf(r):.2f} 平均 {r.mean() * 100:+.1f}銭 "
          f"t {t:.1f} | 前半 {pf(a):.2f} 後半 {pf(b):.2f}")


def event_drift(c, times, wait=15, hold=60):
    rows = []
    for t0 in times:
        t1, t2 = t0 + pd.Timedelta(minutes=wait), t0 + pd.Timedelta(minutes=wait + hold)
        if t0 not in c.index or t1 not in c.index or t2 not in c.index:
            continue
        side = 1 if c.mid_o[t1] > c.mid_o[t0] else -1
        rows.append((t0, trade(c, t1, t2, side)))
    return rows


def month_end_fix(c):
    days = pd.date_range(c.index[0].normalize(), c.index[-1].normalize(), freq="BME")
    out = []
    for d in days:
        fix = pd.Timestamp(d.date(), tz="Europe/London") + pd.Timedelta(hours=16)
        out.append(fix.tz_convert("UTC").tz_localize(None).tz_localize("UTC"))
    return out


def gotobi_split(c):
    star = (ROOT.parent / "strategies/research/gotobi_v2.star").read_text()
    skip = {int(x) for x in re.findall(r"\b20\d{6}\b", star)}
    days = pd.DatetimeIndex(sorted(set((c.index + JST).tz_localize(None).normalize())))
    up, down = [], []
    for d in gotobi_days(days):
        if int(d.strftime("%Y%m%d")) in skip:
            continue
        t0 = (d + pd.Timedelta(hours=8) - JST).tz_localize("UTC")
        t1, t2 = t0 + pd.Timedelta(minutes=115), t0 + pd.Timedelta(minutes=140)
        if t0 not in c.index or t1 not in c.index or t2 not in c.index:
            continue
        (up if c.mid_o[t1] > c.mid_o[t0] else down).append((t0, trade(c, t1, t2, -1)))
    return up, down


def main():
    ev = pd.read_csv(ROOT / "events/us_jp_events.csv", parse_dates=["time"])
    for path in sys.argv[1:]:
        c = load(path)
        c = c[~c.index.duplicated()]
        print(f"\n## {pathlib.Path(path).name}")
        print("### 1. 指標のあとの流れ（15分の向きに入る）")
        for name in ("us_nfp", "us_cpi", "fomc"):
            times = ev.time[ev.event == name]
            for hold in (60, 240):
                judge(f"{name} {hold}分", event_drift(c, times, hold=hold))
        print("### 2. 月末のフィキシング")
        fixes = [f for f in month_end_fix(c) if f in c.index]
        for name, a, b in (("前60分", -60, 0), ("後60分", 0, 60)):
            for side, s in ((1, "買い"), (-1, "売り")):
                rows = [(f, trade(c, f + pd.Timedelta(minutes=a), f + pd.Timedelta(minutes=b), side)) for f in fixes
                        if f + pd.Timedelta(minutes=a) in c.index and f + pd.Timedelta(minutes=b) in c.index]
                judge(f"{name} {s}", rows)
        print("### 3. 仲値後の売りを、仲値前の動きで分ける")
        up, down = gotobi_split(c)
        judge("全部", up + down)
        judge("仲値前に上げた日", up)
        judge("仲値前に下げた日", down)


if __name__ == "__main__":
    main()
