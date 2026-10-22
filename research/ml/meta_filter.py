# /// script
# requires-python = ">=3.11"
# dependencies = ["numpy", "pandas", "scikit-learn"]
# ///
"""ルールのシグナルを機械学習で「通す／見送る」に振り分けられるかを、過去データで確かめる（メタラベリング）。

使い方:
  bot backtest -json -candles-out candles.csv ... > trades.json   # 取引とローソク足を書き出す
  uv run research/ml/meta_filter.py trades.json candles.csv 名前 [trades.json candles.csv 名前 ...]

やっていること:
  1. 取引ごとに、入る直前までの足だけから特徴量を作る（未来の足は使わない）
  2. 期間を3か月ごとに区切り、各区間を「それより前の取引だけで学習したモデル」で判定する（ウォークフォワード）
  3. 見送るしきい値も、学習側の期間の中だけで決める
  4. 見送らなかった取引の損益を、フィルター無し・ランダムに同じ割合を見送った場合と比べる

近似: 見送った取引のあとに別の取引が入る可能性は無視している（1日1回までのルールなら影響は小さい）。
"""

import json
import sys

import numpy as np
import pandas as pd
from sklearn.ensemble import HistGradientBoostingClassifier
from sklearn.linear_model import LogisticRegression
from sklearn.metrics import roc_auc_score
from sklearn.pipeline import make_pipeline
from sklearn.preprocessing import StandardScaler

MIN_TRAIN = 150  # これより少ない取引では学習しない
SKIP_GRID = [0.0, 0.1, 0.2, 0.3, 0.4, 0.5]  # 見送る割合の候補
JST = 9 * 3600


def load_trades(path):
    raw = json.load(open(path))
    r = raw[0] if isinstance(raw, list) else raw
    t = pd.DataFrame(r["trades"])
    t["entry_time"] = pd.to_datetime(t["entry_time"], utc=True)
    t["exit_time"] = pd.to_datetime(t["exit_time"], utc=True)
    return t.sort_values("entry_time").reset_index(drop=True)


def load_candles(path):
    c = pd.read_csv(path, parse_dates=["time"])
    c["mid_o"] = (c.bid_open + c.ask_open) / 2
    c["mid_h"] = (c.bid_high + c.ask_high) / 2
    c["mid_l"] = (c.bid_low + c.ask_low) / 2
    c["mid_c"] = (c.bid_close + c.ask_close) / 2
    prev = c.mid_c.shift(1)
    tr = np.maximum(c.mid_h - c.mid_l, np.maximum((c.mid_h - prev).abs(), (c.mid_l - prev).abs()))
    c["atr14"] = tr.ewm(alpha=1 / 14, adjust=False).mean()
    c["atr100"] = tr.ewm(alpha=1 / 100, adjust=False).mean()
    c["ema50"] = c.mid_c.ewm(span=50, adjust=False).mean()
    d = c.mid_c.diff()
    up = d.clip(lower=0).ewm(alpha=1 / 14, adjust=False).mean()
    dn = (-d.clip(upper=0)).ewm(alpha=1 / 14, adjust=False).mean()
    c["rsi14"] = 100 - 100 / (1 + up / dn.replace(0, np.nan))
    c["hi48"] = c.mid_h.rolling(48).max()
    c["lo48"] = c.mid_l.rolling(48).min()
    # 取引日（6:00 JST 区切り）の、その足までの高値・安値
    day = (c.time.astype("int64") // 10**9 + 3 * 3600) // 86400
    c["day_hi"] = c.mid_h.groupby(day).cummax()
    c["day_lo"] = c.mid_l.groupby(day).cummin()
    return c


def features(trades, c):
    times = c.time.values
    rows = []
    for k, t in trades.iterrows():
        e = np.searchsorted(times, t.entry_time.to_datetime64())  # 約定した足
        i = e - 1  # シグナルを出した足（ここまでの情報だけを使う）
        if i < 200 or e >= len(c):
            rows.append(None)
            continue
        s = 1.0 if t.side == "LONG" else -1.0
        b = c.iloc[i]
        atr = b.atr14
        sec = int(t.entry_time.timestamp()) + JST
        prev = trades.iloc[:k]
        prev = prev[prev.exit_time <= t.entry_time]
        rows.append({
            "side": s,
            "hour": (sec // 3600) % 24,
            "weekday": (sec // 86400 + 3) % 7,  # 0 = 月曜
            "vol": atr / b.mid_c * 1e4,
            "vol_regime": atr / b.atr100,
            "mom12": s * (b.mid_c - c.mid_c.iloc[i - 12]) / atr,
            "mom48": s * (b.mid_c - c.mid_c.iloc[i - 48]) / atr,
            "ema_dist": s * (b.mid_c - b.ema50) / atr,
            "ema_slope": s * (b.ema50 - c.ema50.iloc[i - 12]) / atr,
            "rsi": s * (b.rsi14 - 50),
            "range_pos": (b.mid_c - b.lo48) / max(b.hi48 - b.lo48, 1e-9) if s > 0 else (b.hi48 - b.mid_c) / max(b.hi48 - b.lo48, 1e-9),
            "day_range": (b.day_hi - b.day_lo) / atr,
            "body": s * (b.mid_c - b.mid_o) / atr,
            "spread": (c.ask_open.iloc[e] - c.bid_open.iloc[e]) / atr,
            "prev_win": float(prev.pnl.iloc[-1] > 0) if len(prev) else 0.5,
            "prev5_win": float((prev.pnl.tail(5) > 0).mean()) if len(prev) else 0.5,
        })
    keep = [r is not None for r in rows]
    X = pd.DataFrame([r for r in rows if r is not None])
    return X, trades[keep].reset_index(drop=True)


MODELS = {
    "ロジスティック回帰": lambda: make_pipeline(StandardScaler(), LogisticRegression(C=0.3, max_iter=1000)),
    "勾配ブースティング": lambda: HistGradientBoostingClassifier(max_depth=3, learning_rate=0.05, max_iter=150,
                                                       min_samples_leaf=20, l2_regularization=1.0, random_state=0),
}


def pf(p):
    loss = -p[p < 0].sum()
    return p[p > 0].sum() / loss if loss > 0 else float("inf")


def choose_skip(model_fn, X, y, pnl):
    """学習側の期間を時間順に5分割し、各分割を前だけで学習・判定した結果から、見送る割合を選ぶ。"""
    n = len(X)
    oof = np.full(n, np.nan)
    edges = np.linspace(0, n, 6).astype(int)
    for a, b in zip(edges[1:-1], edges[2:]):
        if a < 50 or y[:a].nunique() < 2:
            continue
        m = model_fn().fit(X.iloc[:a], y[:a])
        oof[a:b] = m.predict_proba(X.iloc[a:b])[:, 1]
    ok = ~np.isnan(oof)
    best, best_pf = 0.0, pf(pnl[ok])
    for q in SKIP_GRID[1:]:
        cut = np.quantile(oof[ok], q)
        kept = pnl[ok][oof[ok] >= cut]
        if pf(kept) > best_pf * 1.02:  # わずかな差では見送りを増やさない
            best, best_pf = q, pf(kept)
    return best


def evaluate(trades_path, candles_path, label):
    trades = load_trades(trades_path)
    X, trades = features(trades, load_candles(candles_path))
    y = (trades.pnl > 0).astype(int)
    pnl = trades.pnl.values
    block = trades.entry_time.dt.year.astype(str) + "Q" + trades.entry_time.dt.quarter.astype(str)
    print(f"\n### {label}（取引 {len(trades)}回）")
    rng = np.random.default_rng(0)
    for name, model_fn in MODELS.items():
        rows, probs_all, idx_all = [], [], []
        tot = {"base": [], "kept": [], "rand": []}
        for blk in block.unique():
            test = (block == blk).values
            first = np.argmax(test)
            if first < MIN_TRAIN:
                continue
            Xtr, ytr, ptr = X.iloc[:first], y[:first], pnl[:first]
            q = choose_skip(model_fn, Xtr, ytr, ptr)
            m = model_fn().fit(Xtr, ytr)
            ptr_prob = m.predict_proba(Xtr)[:, 1]
            cut = np.quantile(ptr_prob, q) if q > 0 else -1
            prob = m.predict_proba(X[test])[:, 1]
            keep = prob >= cut
            probs_all.extend(prob); idx_all.extend(np.where(test)[0])
            p = pnl[test]
            rand = [p[rng.random(len(p)) >= (1 - keep.mean())].sum() for _ in range(200)] if len(p) else [0]
            tot["base"].append(p); tot["kept"].append(p[keep]); tot["rand"].append(np.mean(rand))
            rows.append(f"  {blk}: 見送り{q:.0%}の設定 | 全部 {len(p):3d}回 {p.sum():+6.0f}円 PF {pf(p):.2f}"
                        f" | 通した {keep.sum():3d}回 {p[keep].sum():+6.0f}円 PF {pf(p[keep]):.2f}"
                        f" | ランダムに同じ数 {np.mean(rand):+6.0f}円")
        if not rows:
            print(f"- {name}: 学習に足りる取引が無い")
            continue
        base, kept = np.concatenate(tot["base"]), np.concatenate(tot["kept"])
        auc = roc_auc_score(y[idx_all], probs_all) if y[idx_all].nunique() > 1 else float("nan")
        print(f"- {name}（判定した期間の AUC {auc:.3f}。0.5 = 当てずっぽう）")
        print("\n".join(rows))
        print(f"  合計: 全部 {len(base)}回 {base.sum():+.0f}円 PF {pf(base):.2f}"
              f" | 通した {len(kept)}回 {kept.sum():+.0f}円 PF {pf(kept):.2f}"
              f" | ランダムに同じ数 {sum(tot['rand']):+.0f}円")


if __name__ == "__main__":
    args = sys.argv[1:]
    if not args or len(args) % 3:
        sys.exit("usage: meta_filter.py trades.json candles.csv 名前 [trades.json candles.csv 名前 ...]")
    for i in range(0, len(args), 3):
        evaluate(args[i], args[i + 1], args[i + 2])
