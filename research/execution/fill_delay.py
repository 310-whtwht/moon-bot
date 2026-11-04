# /// script
# requires-python = ">=3.11"
# dependencies = ["numpy", "pandas"]
# ///
"""仲値後の売りで、約定が遅れると成績がどれだけ変わるかを、GMO の公開の1分足で見積もる。

ボットのバックテストは、判定の次の足の始値で約定すると仮定している。Paper では判定のあと
16〜23秒で約定した。秒の足はないので、1分遅れ・2分遅れを計算し、その間を目安にする。
損切りで閉じた取引は、バックテストの損益のまま使う（遅れの影響を見ていない）。

  uv run research/execution/fill_delay.py <bot backtest -json の出力>
"""

import json
import pathlib
import sys
import time
import urllib.request

import numpy as np
import pandas as pd

CACHE = pathlib.Path(__file__).resolve().parent.parent / "lab" / ".cache" / "klines"
URL = "https://forex-api.coin.z.com/public/v1/klines?symbol=USD_JPY&priceType={pt}&interval=1min&date={date}"


def klines(date, pt):
    """その日（日本時間）の1分足の始値を、UTC の時刻で引けるようにして返す。"""
    CACHE.mkdir(parents=True, exist_ok=True)
    path = CACHE / f"USD_JPY_{pt}_{date}.json"
    if not path.exists():
        with urllib.request.urlopen(URL.format(pt=pt, date=date), timeout=30) as r:
            body = json.load(r)
        if body.get("status") != 0:
            raise RuntimeError(body)
        path.write_text(json.dumps(body["data"]))
        time.sleep(0.3)
    rows = json.loads(path.read_text())
    return {pd.Timestamp(int(k["openTime"]), unit="ms", tz="UTC"): float(k["open"]) for k in rows}


def main(path):
    result = json.loads(pathlib.Path(path).read_text())[0]
    trades = result["trades"]
    slip = 0.005
    rows = []
    for t in trades:
        entry = pd.Timestamp(t["entry_time"]).tz_convert("UTC")
        exit_ = pd.Timestamp(t["exit_time"]).tz_convert("UTC")
        date = entry.tz_convert("Asia/Tokyo").strftime("%Y%m%d")
        bid, ask = klines(date, "BID"), klines(date, "ASK")
        timed = t["exit_reason"] == "signal"
        row = {"time": entry, "timed": timed, "bot": t["pnl"] / t["units"] * 100, "fee": t["fees"] / t["units"] * 100}
        for d in (0, 1, 2):
            e = bid.get(entry + pd.Timedelta(minutes=d))
            x = ask.get(exit_ + pd.Timedelta(minutes=d))
            row[f"entry{d}"] = None if e is None else e - slip
            row[f"exit{d}"] = None if x is None else x + slip
        rows.append(row)
    df = pd.DataFrame(rows)
    ok = df.dropna()
    print(f"取引 {len(df)}回（1分足がそろった {len(ok)}回、うち時刻での決済 {ok.timed.sum()}回）")

    def pnl(e, x):
        # 1通貨あたりの銭（手数料を引く）。損切りの取引はボットの損益のまま
        return np.where(ok.timed, (ok[f"entry{e}"] - ok[f"exit{x}"]) * 100 - ok.fee, ok.bot)

    base = pnl(0, 0)
    print(f"  1分足の始値で約定（バックテストと同じ仮定）: 平均 {base.mean():+.2f}銭（ボットの値 {ok.bot.mean():+.2f}銭）")
    for e, x, label in ((1, 0, "入りだけ1分遅れ"), (0, 1, "決済だけ1分遅れ"), (1, 1, "両方1分遅れ"), (2, 2, "両方2分遅れ")):
        r = pnl(e, x)
        pf = r[r > 0].sum() / -r[r < 0].sum()
        print(f"  {label}: 平均 {r.mean():+.2f}銭（{r.mean() - base.mean():+.2f}）、PF {pf:.2f}、勝率 {np.mean(r > 0):.0%}")


if __name__ == "__main__":
    main(sys.argv[1])
