# /// script
# requires-python = ">=3.11"
# dependencies = ["numpy", "pandas"]
# ///
"""時間帯のくせ（決まった時刻に、決まった向きに動きやすいか）を網羅的に洗い出す。

使い方:
  bot backtest -candles-out candles.csv ...   # ローソク足を書き出す
  uv run research/seasonality/scan.py candles.csv [保有時間（分）=60]

日本時間の「曜日 × 開始時刻（30分刻み）」ごとに、保有時間だけ持ったときの値動き（中値）を集め、
次の条件をすべて満たすものだけを出す。
  - 前半・後半の両方で、平均が同じ向きで、スプレッドの中央値の2倍を超える
  - t 値の絶対値が 3 以上（たくさん試すので、偶然で出にくい厳しめの基準にする）
あわせて、ゴトー日・月末・指標日などの「日の種類」で分けた結果も出す。
"""

import sys

import numpy as np
import pandas as pd

JST = pd.Timedelta(hours=9)


def load(path):
    c = pd.read_csv(path, parse_dates=["time"])
    c["mid_o"] = (c.bid_open + c.ask_open) / 2
    c["mid_c"] = (c.bid_close + c.ask_close) / 2
    c["spread"] = c.ask_open - c.bid_open
    c["jst"] = c.time + JST
    return c.set_index("time")


def moves(c, minutes):
    """各足の始値で入り、minutes 後の足の始値で出たときの中値の変化。"""
    step = pd.Timedelta(minutes=minutes)
    later = c.mid_o.reindex(c.index + step).values
    out = pd.DataFrame({"move": later - c.mid_o.values, "spread": c.spread.values,
                        "jst": c.jst.values}, index=c.index).dropna()
    out["weekday"] = out.jst.dt.day_of_week
    out["slot"] = out.jst.dt.hour * 60 + (out.jst.dt.minute // 30) * 30
    out = out[out.jst.dt.minute % 30 == 0]  # 30分刻みの時刻だけ
    return out


def gotobi(d):
    days_in_month = d.days_in_month
    day = d.day
    targets = {5, 10, 15, 20, 25, days_in_month}
    if d.weekday() > 4:
        return False
    if day in targets:
        return True
    if d.weekday() == 4:
        return any(day + a <= days_in_month and day + a in targets for a in (1, 2))
    return False


def month_end(d):
    """月の最終営業日（土日だけ考える）。"""
    last = d + pd.offsets.BMonthEnd(0)
    return d.normalize() == last.normalize()


def report(df, group_cols, label, min_n=40):
    rows = []
    for key, g in df.groupby(group_cols):
        h1, h2 = g.move[g.first], g.move[~g.first]
        if len(h1) < min_n // 2 or len(h2) < min_n // 2:
            continue
        m, s = g.move.mean(), g.move.std(ddof=1)
        t = m / (s / np.sqrt(len(g))) if s > 0 else 0
        cost = g.spread.median()
        ok = (np.sign(h1.mean()) == np.sign(h2.mean()) and min(abs(h1.mean()), abs(h2.mean())) > 2 * cost
              and abs(t) >= 3)
        rows.append((key, len(g), m, h1.mean(), h2.mean(), t, cost, ok))
    r = pd.DataFrame(rows, columns=["key", "n", "mean", "h1", "h2", "t", "spread", "ok"])
    print(f"\n### {label}（{len(r)}通りのうち、条件を満たしたもの {r.ok.sum()}）")
    for _, x in r[r.ok].sort_values("t", key=abs, ascending=False).iterrows():
        print(f"  {x.key}: {x.n}回 平均 {x['mean'] * 100:+.2f}銭（前半 {x.h1 * 100:+.2f} / 後半 {x.h2 * 100:+.2f}）"
              f" t={x.t:+.1f} スプレッド {x.spread * 100:.2f}銭")


def main():
    path = sys.argv[1]
    minutes = int(sys.argv[2]) if len(sys.argv) > 2 else 60
    c = load(path)
    df = moves(c, minutes)
    names = ["月", "火", "水", "木", "金", "土", "日"]
    df["wd"] = df.weekday.map(lambda w: names[w])
    df["start"] = df.slot.map(lambda s: f"{s // 60:02d}:{s % 60:02d}")
    df["first"] = df.index < df.index[0] + (df.index[-1] - df.index[0]) / 2
    days = df.jst.dt.normalize()
    df["gotobi"] = days.map(gotobi)
    df["month_end"] = days.map(month_end)
    print(f"{path}: {len(df)}件、保有 {minutes}分")
    report(df, ["wd", "start"], "曜日 × 開始時刻")
    report(df, ["gotobi", "start"], "ゴトー日かどうか × 開始時刻", min_n=60)
    report(df, ["month_end", "start"], "月の最終営業日かどうか × 開始時刻", min_n=30)


if __name__ == "__main__":
    main()
