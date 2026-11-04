# /// script
# requires-python = ">=3.11"
# dependencies = ["numpy", "pandas"]
# ///
"""値動きの読み方（寄り付きの維持・ブレイク後の残り方・もみ合いの長さ）を、GMO の5分足で確かめる。

使い方:
  bot backtest -candles-out candles.csv ...   # 5分足を書き出す
  uv run research/priceaction/intraday.py candles.csv [...]

確かめること（どれも「その後の値動きが、条件で変わるか」を見る）:
  1. 寄り付きの維持: 東京 9:00・ロンドン 16:00・ニューヨーク 21:00（日本時間）の始値に対し、
     60分後に「一度も終値で割らずに上にいる」「割ったが取り戻した」「下にいる」で分け、その後4時間の値動きを見る
  2. 前日の高値・安値: 前日（7:00 区切り）の高値を終値で抜けたあと、30分後も上に残っているか、戻ったかで分け、
     翌 5:00 までの値動きを見る（6〜7時はスプレッドが広がるので避ける）
  3. 1時間足の20本高値ブレイク: 抜けた直後に入る場合と、3本後も外側に残っているときだけ入る場合を比べる。
     抜ける前のもみ合いの狭さ（20本の値幅 ÷ ATR(100)）でも分ける。保有は24時間
売りは買いの裏返し（下側）を同じように数え、まとめて「抜けた向きに乗る」取引として集計する。
約定は買いを ask、売りを bid で行う（スプレッド込み）。滑りは別に 0.5銭を引く。
"""

import sys

import numpy as np
import pandas as pd

JST = pd.Timedelta(hours=9)
SLIP = 0.005


def load(path):
    c = pd.read_csv(path, parse_dates=["time"]).set_index("time")
    c["mid_o"] = (c.bid_open + c.ask_open) / 2
    c["mid_c"] = (c.bid_close + c.ask_close) / 2
    c["mid_h"] = (c.bid_high + c.ask_high) / 2
    c["mid_l"] = (c.bid_low + c.ask_low) / 2
    return c


def fill(c, t, side, entry):
    """t の足の始値で約定した値。買いの入り・売りの決済は ask、逆は bid。"""
    use_ask = (side == 1) == entry
    return c.ask_open.get(t) if use_ask else c.bid_open.get(t)


def trade(c, t_in, t_out, side):
    a, b = fill(c, t_in, side, True), fill(c, t_out, side, False)
    if a is None or b is None or np.isnan(a) or np.isnan(b):
        return None
    return side * (b - a) - SLIP


def pf(r):
    r = np.asarray(r)
    loss = -r[r < 0].sum()
    return r[r > 0].sum() / loss if loss > 0 else float("inf")


def summary(name, rows):
    if not rows:
        print(f"  {name}: なし")
        return
    df = pd.DataFrame(rows, columns=["time", "ret"])
    r = df.ret.values
    mid = df.time.iloc[len(df) // 2]
    h1, h2 = df[df.time < mid].ret, df[df.time >= mid].ret
    print(f"  {name}: {len(r)}回 勝率 {np.mean(r > 0):.0%} PF {pf(r):.2f} 平均 {r.mean() * 100:+.1f}銭 "
          f"| 前半 PF {pf(h1):.2f} 後半 PF {pf(h2):.2f}")


def opening_hold(c, open_hhmm, check_min=60, hold_min=240):
    """寄り付きの始値に対する60分間の振る舞いで分け、抜けた向きに乗ったときの損益を集める。"""
    out = {k: [] for k in ("held", "reclaimed", "above_dipped", "fail")}
    jst = c.index + JST
    starts = c.index[(jst.hour == open_hhmm[0]) & (jst.minute == open_hhmm[1]) & (jst.day_of_week < 5)]
    for t0 in starts:
        t_chk, t_out = t0 + pd.Timedelta(minutes=check_min), t0 + pd.Timedelta(minutes=check_min + hold_min)
        if t_chk not in c.index or t_out not in c.index:
            continue
        w = c.loc[t0:t_chk - pd.Timedelta(minutes=5)]
        o, now = c.mid_o[t0], c.mid_o[t_chk]
        for side in (1, -1):
            closes = side * (w.mid_c.values - o)
            if side * (now - o) <= 0:
                continue  # その向きの外側にいない
            if (closes >= 0).all():
                kind = "held"  # 一度も終値で反対側に行かずに外側にいる
            elif closes[0] < 0 and side * (w.mid_c.values[-1] - o) > 0 and (closes[: len(closes) // 2] < 0).all():
                kind = "reclaimed"  # 前半は反対側、後半で取り戻した
            else:
                kind = "above_dipped"
            r = trade(c, t_chk, t_out, side)
            if r is not None:
                out[kind].append((t0, r))
    return out


def prev_day_break(c, wait_min=30):
    """前日（7:00 JST 区切り）の高値・安値を終値で抜けたあと、wait_min 後に外側に残っているかで分ける。"""
    jst_day = (c.index + JST - pd.Timedelta(hours=7)).normalize()
    days = pd.Series(jst_day, index=c.index)
    hi = c.mid_h.groupby(jst_day).max()
    lo = c.mid_l.groupby(jst_day).min()
    out = {"stay": [], "back": []}
    for d, g in c.groupby(jst_day):
        prev = d - pd.Timedelta(days=1)
        while prev not in hi.index and prev > hi.index[0]:
            prev -= pd.Timedelta(days=1)
        if prev not in hi.index or len(g) < 100:
            continue
        # 決済は 5:00（日本時間）。6〜7時はスプレッドが大きく広がるので避ける
        before = g.index[((g.index + JST).hour < 5) | ((g.index + JST).hour >= 7)]
        if len(before) == 0:
            continue
        t_end = before[-1] + pd.Timedelta(minutes=5)
        if t_end not in c.index:
            continue
        for side, level in ((1, hi[prev]), (-1, lo[prev])):
            hit = g.index[side * (g.mid_c.values - level) > 0]
            if len(hit) == 0:
                continue
            t_chk = hit[0] + pd.Timedelta(minutes=5 + wait_min)
            if t_chk not in g.index or t_chk >= t_end:
                continue
            stay = side * (c.mid_o[t_chk] - level) > 0
            r = trade(c, t_chk, t_end, side)
            if r is not None:
                out["stay" if stay else "back"].append((d, r))
    return out


def hourly_breakout(c, n=20, confirm=3, hold=24):
    h = c.resample("1h").agg({"mid_o": "first", "mid_h": "max", "mid_l": "min", "mid_c": "last"}).dropna()
    tr = pd.concat([h.mid_h - h.mid_l, (h.mid_h - h.mid_c.shift()).abs(), (h.mid_l - h.mid_c.shift()).abs()], axis=1).max(axis=1)
    atr = tr.rolling(100).mean()
    hi, lo = h.mid_c.rolling(n).max().shift(1), h.mid_c.rolling(n).min().shift(1)
    tight = (h.mid_h.rolling(n).max() - h.mid_l.rolling(n).min()).shift(1) / atr
    idx = h.index
    out = {"now": [], "stay": [], "back": [], "now_tight": [], "now_wide": []}
    q = tight.quantile([1 / 3, 2 / 3]).values
    busy_until = None
    for i in range(len(idx) - confirm - hold - 1):
        t = idx[i]
        if np.isnan(atr.iloc[i]) or (busy_until is not None and t < busy_until):
            continue
        for side, level in ((1, hi.iloc[i]), (-1, lo.iloc[i])):
            if side * (h.mid_c.iloc[i] - level) <= 0:
                continue
            t_in = idx[i + 1]
            r = trade(c, t_in, t_in + pd.Timedelta(hours=hold), side)
            if r is None:
                continue
            out["now"].append((t, r))
            if tight.iloc[i] <= q[0]:
                out["now_tight"].append((t, r))
            elif tight.iloc[i] >= q[1]:
                out["now_wide"].append((t, r))
            t_c = idx[i + confirm + 1]
            stay = side * (h.mid_c.iloc[i + confirm] - level) > 0
            r2 = trade(c, t_c, t_c + pd.Timedelta(hours=hold), side)
            if r2 is not None:
                out["stay" if stay else "back"].append((t, r2))
            busy_until = t + pd.Timedelta(hours=hold)
            break
    return out


def main():
    for path in sys.argv[1:]:
        c = load(path)
        print(f"\n## {path}  {c.index[0].date()}〜{c.index[-1].date()}")
        print("\n### 1. 寄り付きの維持（60分後に、抜けた向きへ4時間）")
        for name, hm in (("東京 9:00", (9, 0)), ("ロンドン 16:00", (16, 0)), ("NY 21:00", (21, 0))):
            o = opening_hold(c, hm)
            print(f" {name}")
            summary("一度も割らずに外側を維持", o["held"])
            summary("割ったが後半で取り戻した", o["reclaimed"])
            summary("外側だが途中で割った", o["above_dipped"])
        print("\n### 2. 前日の高値・安値を抜けたあと、30分後（翌 5:00 まで保有）")
        o = prev_day_break(c)
        summary("外側に残った → 抜けた向きに乗る", o["stay"])
        summary("内側に戻った → 抜けた向きに乗る（負けるなら逆張りの候補）", o["back"])
        print("\n### 3. 1時間足 20本ブレイク（24時間保有）")
        o = hourly_breakout(c)
        summary("抜けた直後に入る", o["now"])
        summary("3本後も外側 → そこで入る", o["stay"])
        summary("3本後に内側へ戻った → そこで入る", o["back"])
        summary("直後に入る・もみ合いが狭い（下1/3）", o["now_tight"])
        summary("直後に入る・もみ合いが広い（上1/3）", o["now_wide"])


if __name__ == "__main__":
    main()
