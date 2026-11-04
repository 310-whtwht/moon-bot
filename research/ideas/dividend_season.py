# /// script
# requires-python = ">=3.11"
# dependencies = ["numpy", "pandas"]
# ///
"""配当の支払い時期に USD/JPY が上がるかを、FRED の日足（DEXJPUS、NY 正午）で長く見る。

窓は結果を見る前に決めた: 期末配当 6/27〜7/10、中間配当 12/1〜12/14、1995〜2025年。
同じ年の、同じ長さのほかの窓（重ならない）と比べる。ボットのバックテストではないので、ラボの判定には使わない。

  uv run research/ideas/dividend_season.py <DEXJPUS.csv>
"""

import sys

import numpy as np
import pandas as pd

WINDOWS = {"期末（6/27〜7/10）": ((6, 27), (7, 10)), "中間（12/1〜12/14）": ((12, 1), (12, 14))}


def main(path):
    s = pd.read_csv(path, index_col=0, parse_dates=True).iloc[:, 0]
    s = pd.to_numeric(s, errors="coerce").dropna()
    logp = np.log(s)
    rng = np.random.default_rng(1)
    for name, ((m1, d1), (m2, d2)) in WINDOWS.items():
        rets = []
        for y in range(1995, 2026):
            a = logp[: pd.Timestamp(y, m1, d1) - pd.Timedelta(days=1)]
            b = logp[: pd.Timestamp(y, m2, d2)]
            rets.append((b.iloc[-1] - a.iloc[-1]) * 100)
        rets = np.array(rets)
        n_days = (pd.Timestamp(2001, m2, d2) - pd.Timestamp(2001, m1, d1)).days + 1
        # 同じ長さの、でたらめな開始日の窓（1995〜2025年）
        days = logp["1995":"2025"]
        idx = days.index
        fake = []
        for _ in range(20000):
            start = idx[rng.integers(0, len(idx) - 30)]
            a = logp[:start - pd.Timedelta(days=1)].iloc[-1]
            b = logp[:start + pd.Timedelta(days=n_days - 1)].iloc[-1]
            fake.append((b - a) * 100)
        fake = np.array(fake)
        boot = np.array([rng.choice(fake, len(rets)).mean() for _ in range(5000)])
        p = ((boot >= rets.mean()).sum() + 1) / (len(boot) + 1)
        half = len(rets) // 2
        t = rets.mean() / rets.std(ddof=1) * np.sqrt(len(rets))
        print(f"{name}: {len(rets)}年、平均 {rets.mean():+.2f}%（ふだんの同じ長さ {fake.mean():+.2f}%）、"
              f"上がった年 {np.mean(rets > 0):.0%}（ふだん {np.mean(fake > 0):.0%}）、t {t:.2f}、p {p:.3f}")
        print(f"    前半 {rets[:half].mean():+.2f}% / 後半 {rets[half:].mean():+.2f}%、直近10年 {rets[-10:].mean():+.2f}%")


if __name__ == "__main__":
    main(sys.argv[1])
