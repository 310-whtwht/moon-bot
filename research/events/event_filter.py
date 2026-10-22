# /// script
# requires-python = ">=3.11"
# dependencies = ["numpy", "pandas"]
# ///
"""経済指標の発表（us_jp_events.csv）にかかる取引を見送ると、成績が良くなるかを確かめる。

使い方:
  uv run research/events/event_filter.py check candles.csv          # 発表日が値動きと合っているかを確かめる
  uv run research/events/event_filter.py eval trades.json 名前 [trades.json 名前 ...]

eval では、次の2つの見送り方を比べる。
  near : 入る30分前から入って4時間後までに発表がある取引を見送る
         （決済の時刻で区切ると、長く持った＝勝った取引ほど発表を含みやすく、未来の情報が混ざるため使わない）
  day  : 発表がある取引日（6:00 JST 区切り）の取引をすべて見送る
見送った取引の合計と、ランダムに同じ数を見送った場合（1000回）を比べ、
「ランダムでこれ以上良くなる割合」を出す。小さいほど、偶然ではない。
"""

import json
import pathlib
import sys

import numpy as np
import pandas as pd

EVENTS = pd.read_csv(pathlib.Path(__file__).with_name("us_jp_events.csv"), parse_dates=["time"])
BEFORE = pd.Timedelta(minutes=30)
AFTER = pd.Timedelta(hours=4)


def trading_day(t):
    """6:00 JST（21:00 UTC）で区切った取引日。"""
    return (pd.Series(t) + pd.Timedelta(hours=3)).dt.floor("D")


def check(candles_path):
    c = pd.read_csv(candles_path, parse_dates=["time"])
    mid = (c.bid_close + c.ask_close) / 2
    c["move"] = (mid - (c.bid_open + c.ask_open) / 2).abs()
    by_time = c.set_index("time")
    print("発表の足の値動き ÷ 同じ時刻の足の中央値（発表がある日を除く）")
    for name, g in EVENTS.groupby("event"):
        ratios = []
        for t in g.time:
            if t not in by_time.index:
                continue
            same = c[(c.time.dt.hour == t.hour) & (c.time.dt.minute == t.minute) & ~c.time.dt.date.isin(g.time.dt.date)]
            ratios.append(by_time.loc[t, "move"] / same.move.median())
        r = np.array(ratios)
        if len(r):
            print(f"  {name}: {len(r)}回 中央値 {np.median(r):.1f}倍、1.5倍未満 {np.sum(r < 1.5)}回")


def load_trades(path):
    raw = json.load(open(path))
    r = raw[0] if isinstance(raw, list) else raw
    t = pd.DataFrame(r["trades"])
    t["entry_time"] = pd.to_datetime(t["entry_time"], utc=True)
    t["exit_time"] = pd.to_datetime(t["exit_time"], utc=True)
    return t


def near_mask(t, times):
    return np.array([((times >= (a - BEFORE).to_datetime64()) & (times <= (a + AFTER).to_datetime64())).any()
                     for a in t.entry_time])


def pf(p):
    loss = -p[p < 0].sum()
    return p[p > 0].sum() / loss if loss > 0 else float("inf")


def evaluate(path, label):
    t = load_trades(path)
    ev = EVENTS.time.values
    near = near_mask(t, ev)
    day = trading_day(t.entry_time).isin(trading_day(EVENTS.time)).values
    pnl = t.pnl.values
    rng = np.random.default_rng(0)
    print(f"\n### {label}（{len(t)}回 {pnl.sum():+.0f}円 PF {pf(pnl):.2f}）")
    half = (t.entry_time < t.entry_time.iloc[0] + (t.entry_time.iloc[-1] - t.entry_time.iloc[0]) / 2).values
    for name, skip in [("near", near), ("day", day)]:
        k = skip.sum()
        kept = pnl[~skip]
        rand = np.array([np.delete(pnl, rng.choice(len(pnl), k, replace=False)).sum() for _ in range(1000)])
        print(f"  {name}: 見送り {k}回（{pnl[skip].sum():+.0f}円 PF {pf(pnl[skip]):.2f}）"
              f" → 残り {len(kept)}回 {kept.sum():+.0f}円 PF {pf(kept):.2f}"
              f" | ランダムでこれ以上 {np.mean(rand >= kept.sum()):.0%}")
        for hname, h in [("前半", half), ("後半", ~half)]:
            print(f"      {hname}: 全部 {h.sum()}回 {pnl[h].sum():+.0f}円 PF {pf(pnl[h]):.2f}"
                  f" → 見送り {(h & skip).sum()}回 {pnl[h & skip].sum():+.0f}円、残り PF {pf(pnl[h & ~skip]):.2f}")
        if k:
            for e in sorted(EVENTS.event.unique()):
                times = EVENTS.time[EVENTS.event == e].values
                if name == "near":
                    m = near_mask(t, times)
                else:
                    m = trading_day(t.entry_time).isin(trading_day(EVENTS.time[EVENTS.event == e])).values
                if m.any():
                    print(f"      {e}: {m.sum()}回 {pnl[m].sum():+.0f}円 PF {pf(pnl[m]):.2f}")


if __name__ == "__main__":
    if len(sys.argv) == 3 and sys.argv[1] == "check":
        check(sys.argv[2])
    elif len(sys.argv) >= 4 and sys.argv[1] == "eval" and len(sys.argv) % 2 == 0:
        for i in range(2, len(sys.argv), 2):
            evaluate(sys.argv[i], sys.argv[i + 1])
    else:
        sys.exit(__doc__)
