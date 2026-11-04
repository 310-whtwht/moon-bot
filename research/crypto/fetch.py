# /// script
# requires-python = ">=3.11"
# ///
"""GMO コインのレバレッジ取引の1時間足を取り、ボットの -candles-in の形式で保存する。

公開の足は約定値だけで BID・ASK がないので、両方に同じ値を入れる。スプレッドと手数料は、
バックテストの -fee-rate で保守的に見積もる（research/lab/hypotheses の crypto カードを参照）。

  uv run research/crypto/fetch.py BTC_JPY 2021-01-01
"""

import csv
import datetime as dt
import json
import pathlib
import sys
import time
import urllib.error
import urllib.request

OUT = pathlib.Path(__file__).resolve().parent.parent / "lab" / ".cache" / "crypto"
URL = "https://api.coin.z.com/public/v1/klines?symbol={symbol}&interval=1hour&date={date}"


def day(symbol, date):
    raw = OUT / "raw" / symbol / f"{date}.json"
    if raw.exists():
        return json.loads(raw.read_text())
    for attempt in range(5):
        try:
            with urllib.request.urlopen(URL.format(symbol=symbol, date=date), timeout=30) as r:
                body = json.load(r)
            break
        except urllib.error.HTTPError as e:
            # 足のない日は 404 で返る（本文は status 2・ERR-5207）
            body = json.loads(e.read() or b"{}")
            break
        except OSError:
            time.sleep(2 + attempt * 3)
    else:
        raise RuntimeError(f"{symbol} {date}: 取得に失敗しました")
    time.sleep(0.25)
    if body.get("status") != 0:
        if any(m.get("message_code") == "ERR-5207" for m in body.get("messages", [])):
            rows = []  # その日は足がない（上場前など）
        else:
            raise RuntimeError(f"{symbol} {date}: {body}")
    else:
        rows = body["data"]
    raw.parent.mkdir(parents=True, exist_ok=True)
    if date < dt.date.today().strftime("%Y%m%d"):
        raw.write_text(json.dumps(rows))
    return rows


def main(symbol, start):
    d, today = dt.date.fromisoformat(start), dt.date.today()
    bars = {}
    while d < today:
        for k in day(symbol, d.strftime("%Y%m%d")):
            bars[int(k["openTime"])] = k
        d += dt.timedelta(days=1)
    path = OUT / f"{symbol}_1h.csv"
    with path.open("w", newline="") as f:
        w = csv.writer(f)
        w.writerow(["time", "bid_open", "bid_high", "bid_low", "bid_close", "ask_open", "ask_high", "ask_low", "ask_close", "quote_rate"])
        for t in sorted(bars):
            k = bars[t]
            ohlc = [k["open"], k["high"], k["low"], k["close"]]
            w.writerow([dt.datetime.fromtimestamp(t / 1000, dt.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"), *ohlc, *ohlc, 0])
    print(f"{symbol}: {len(bars)}本 → {path}")


if __name__ == "__main__":
    main(sys.argv[1], sys.argv[2])
