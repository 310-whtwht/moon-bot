# /// script
# requires-python = ">=3.11"
# dependencies = ["numpy", "pandas", "pyyaml"]
# ///
"""戦略を検証する仕組みの入口。使い方は research/lab/README.md。

  uv run research/lab/lab.py flows              注文の流れの一覧を出す（案はここから作る）
  uv run research/lab/lab.py new <id-短い名前> <流れの id>   仮説カードを作る
  uv run research/lab/lab.py run <カードのファイル名>   検査を実行する
  uv run research/lab/lab.py status             一覧を出す

ボットのバックテストを実行して読むだけで、Paper・本番・データベースの設定は変えない。
"""

import csv
import datetime as dt
import hashlib
import json
import pathlib
import subprocess
import sys

import numpy as np
import pandas as pd
import yaml

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parent.parent
sys.path.insert(0, str(HERE))
import gates  # noqa: E402

CARDS = HERE / "hypotheses"
VERDICTS = HERE / "verdicts"
LEDGER = HERE / "ledger.csv"
CACHE = HERE / ".cache"
FLOWS = HERE / "flows.yaml"
LEGACY_CARDS = ("H-0002", "H-0003")  # 流れの台帳より前に作った、相手を説明できないカード
REQUIRED = ("id", "revision", "flow", "title", "why", "counterparty", "script", "symbol", "interval", "params", "neighbors")
TEMPLATE = """\
id: {id}
revision: 1
stage: idea            # idea → paper → live。変えられるのは人だけ
flow: {flow}           # flows.yaml の id。相手を説明できない案は作らない
title: ""
why: ""                # なぜ値が動くか。誰がどんな理由で注文を出すか
counterparty: ""       # 誰の損が自分の利益になるか。書けないなら作らない
script: research/xxx.star   # strategies/ からの場所
symbol: USD_JPY
others: []             # 参考に見る銘柄（合否には使わない）
interval: 5m
from: "2023-10-28"
slippage: 0.005
params: {{}}             # 動かす前に決める。結果を見てから変えるなら revision を上げる
neighbors: {{}}          # 例 hold_bars: [10, 14]。1つずつずらして試す
found_on_same_data: false   # 同じデータを見て思いついた案なら true
"""


def spec_hash(card):
    script = (ROOT / "strategies" / card["script"]).read_bytes()
    text = json.dumps({k: v for k, v in card.items() if k not in ("stage", "flow")}, sort_keys=True, ensure_ascii=False)
    return hashlib.sha256(text.encode() + script).hexdigest()[:12]


def read_flows():
    return {f["id"]: f for f in yaml.safe_load(FLOWS.read_text())}


def check_flow(card, known):
    """カードが、説明できる相手に紐づいているかを確かめる。"""
    flow = known.get(card["flow"])
    if flow is None:
        sys.exit(f"止めました: 流れ {card['flow']} は flows.yaml にありません。先に相手を書いてください。")
    if card["flow"] == "F-000" and card["id"] not in LEGACY_CARDS:
        sys.exit("止めました: F-000 は古いカード専用です。相手を説明できない案は作りません。")
    if flow["testable"] == "blocked":
        sys.exit(f"止めました: {card['flow']} は今のデータで確かめられません（要るもの: {flow.get('needs')}）。")


def read_ledger():
    if not LEDGER.exists():
        return []
    with LEDGER.open() as f:
        return list(csv.DictReader(f))


def append_ledger(row):
    new = not LEDGER.exists()
    with LEDGER.open("a", newline="") as f:
        w = csv.DictWriter(f, fieldnames=list(row))
        if new:
            w.writeheader()
        w.writerow(row)


def trials_so_far(rows, card):
    seen = {(r["id"], r["revision"], r["symbol"]) for r in rows}
    seen.add((card["id"], str(card["revision"]), card["symbol"]))
    return gates.PRIOR_TRIALS + len(seen)


def check_locked(rows, card, digest):
    """結果を見たあとにカードやスクリプトを変えたまま、同じ番号で試すことを止める。"""
    for r in rows:
        if r["id"] == card["id"] and r["revision"] == str(card["revision"]) and r["hash"] != digest:
            sys.exit(f"止めました: {card['id']} の revision {card['revision']} は、実行のあとに内容が変わっています。"
                     "revision を上げて、変えた理由を why に書いてください（新しい試行として数えます）。")


def backtest(card, symbol, slippage, params, candles=False):
    cmd = ["docker", "compose", "run", "--rm", "-T", "-v", f"{ROOT}/strategies:/strategies:ro", "-v", f"{CACHE}:/out",
           "bot", "backtest", "-script", f"/strategies/{card['script']}", "-symbol", symbol,
           "-interval", card["interval"], "-from", str(card["from"]), "-slippage", str(slippage), "-json"]
    for k, v in params.items():
        cmd += ["-param", f"{k}={v}"]
    if candles:
        cmd += ["-candles-out", f"/out/{symbol}_{card['interval']}.csv"]
    out = subprocess.run(cmd, cwd=ROOT, capture_output=True, text=True)
    if out.returncode != 0:
        sys.exit(f"バックテストに失敗しました:\n{out.stderr[-800:]}")
    results = json.JSONDecoder().raw_decode(out.stdout[out.stdout.index("[") :])[0]
    if not results:
        sys.exit(f"バックテストの結果が空です（スクリプトのエラーの可能性）:\n{out.stdout[-800:]}")
    return results[0]["trades"] or [], results[0]["metrics"]


def pnl_of(trades):
    return [t["pnl"] for t in trades]


def random_p(trades, csv_path):
    c = pd.read_csv(csv_path, parse_dates=["time"]).drop_duplicates("time").set_index("time")
    index = c.index
    pos = lambda key: np.clip(index.searchsorted(pd.to_datetime([t[key] for t in trades], format="ISO8601")), 0, len(index) - 1)
    entries, exits = pos("entry_time"), pos("exit_time")
    keep = exits > entries
    sides = np.array([1 if t["side"] == "LONG" else -1 for t in trades])
    return gates.random_entry_p(c.bid_open.values, c.ask_open.values, entries[keep], exits[keep], sides[keep])


def spread_note(trades, csv_path, key, label):
    """約定の時刻のスプレッドが普段より広いなら、負けの原因は仮説でなくコストかもしれない。"""
    c = pd.read_csv(csv_path, parse_dates=["time"]).drop_duplicates("time").set_index("time")
    spread = c.ask_open - c.bid_open
    at = np.clip(c.index.searchsorted(pd.to_datetime([t[key] for t in trades], format="ISO8601")), 0, len(c) - 1)
    usual, here = spread.median(), np.median(spread.values[at])
    if here > 3 * usual:
        return f"{label}の時刻のスプレッドが {here * 100:.1f}銭で、普段（{usual * 100:.1f}銭）の{here / usual:.0f}倍。成績はコストに左右されている"
    return None


def warnings(card, trades, csv_path):
    out = []
    if card.get("found_on_same_data"):
        out.append("同じデータを見て思いついた案。合格しても、まだ見ていない期間（Paper）で確かめるまで証拠にならない")
    notes = (spread_note(trades, csv_path, "entry_time", "入り"), spread_note(trades, csv_path, "exit_time", "決済"))
    return out + [n for n in notes if n]


def run(name):
    CACHE.mkdir(exist_ok=True)
    path = CARDS / name
    card = yaml.safe_load(path.read_text())
    missing = [k for k in REQUIRED if card.get(k) in (None, "")]
    if missing:
        sys.exit(f"カードが足りません: {', '.join(missing)}")
    check_flow(card, read_flows())
    rows, digest = read_ledger(), spec_hash(card)
    check_locked(rows, card, digest)
    trials = trials_so_far(rows, card)
    slip, params = card.get("slippage", 0.005), card["params"]

    print(f"{card['id']} r{card['revision']} {card['title']}（{card['symbol']} {card['interval']}）")
    trades, metrics = backtest(card, card["symbol"], slip, params, candles=True)
    if not trades:
        sys.exit("取引が1回もありません")
    stress, _ = backtest(card, card["symbol"], slip * gates.STRESS_SLIPPAGE, params)
    neighbor_pfs = []
    for key, values in card["neighbors"].items():
        for v in values:
            t, _ = backtest(card, card["symbol"], slip, {**params, key: v})
            neighbor_pfs.append(gates.pf(pnl_of(t)))
            print(f"  {key}={v}: PF {neighbor_pfs[-1]:.2f}")
    p = random_p(trades, CACHE / f"{card['symbol']}_{card['interval']}.csv")
    result = gates.evaluate(pnl_of(trades), pnl_of(stress), neighbor_pfs, p, trials)
    passed = all(g.passed for g in result)

    for g in result:
        print(f"  {'○' if g.passed else '×'} {g.name}: {g.detail}")
    others = {}
    for s in card.get("others") or []:
        t, _ = backtest(card, s, slip, params)
        others[s] = {"trades": len(t), "pf": round(gates.pf(pnl_of(t)), 2)}
        print(f"  参考 {s}: {len(t)}回 PF {others[s]['pf']}")
    notes = warnings(card, trades, CACHE / f"{card['symbol']}_{card['interval']}.csv")
    for w in notes:
        print(f"  注意: {w}")
    print(f"判定: {'合格（Paper の候補。進めるかは人が決める）' if passed else '不合格'}")

    now = dt.datetime.now(dt.timezone.utc).isoformat(timespec="seconds")
    append_ledger(dict(time=now, id=card["id"], revision=card["revision"], symbol=card["symbol"], hash=digest,
                       trades=len(trades), pf=round(gates.pf(pnl_of(trades)), 2), t=round(gates.t_value(pnl_of(trades)), 2),
                       trials=trials, passed=passed))
    VERDICTS.mkdir(exist_ok=True)
    (VERDICTS / f"{card['id']}-r{card['revision']}.json").write_text(json.dumps({
        "id": card["id"], "revision": card["revision"], "time": now, "hash": digest, "trials": trials,
        "passed": passed, "gates": [g.__dict__ for g in result], "metrics": metrics, "others": others, "notes": notes,
    }, ensure_ascii=False, indent=2, default=lambda x: bool(x) if isinstance(x, np.bool_) else str(x)) + "\n")


def status():
    latest = {}
    rows = read_ledger()
    for r in rows:
        latest[r["id"]] = r
    print(f"試行 {len({(r['id'], r['revision'], r['symbol']) for r in rows})} 通り（手で試した概算 {gates.PRIOR_TRIALS} を除く）")
    for path in sorted(CARDS.glob("*.yaml")):
        card = yaml.safe_load(path.read_text())
        r = latest.get(card["id"])
        state = "未実行" if r is None else f"{'合格' if r['passed'] == 'True' else '不合格'} {r['trades']}回 PF {r['pf']} t {r['t']}"
        print(f"  {card['id']} [{card.get('stage', 'idea')}] {card['title']}: {state}")


def new(name, flow):
    path = CARDS / f"{name}.yaml"
    if path.exists():
        sys.exit(f"もうあります: {path}")
    check_flow({"id": "", "flow": flow}, read_flows())
    path.write_text(TEMPLATE.format(id="-".join(name.split("-")[:2]), flow=flow))
    print(path)


def flows():
    latest = {r["id"]: r for r in read_ledger()}
    cards = [yaml.safe_load(p.read_text()) for p in sorted(CARDS.glob("*.yaml"))]
    for fid, f in read_flows().items():
        if fid == "F-000":
            continue
        print(f"{fid} [{f['testable']}] {f['actor']}: {f['expect']}")
        mine = [c for c in cards if c.get("flow") == fid]
        for c in mine:
            r = latest.get(c["id"])
            state = "未実行" if r is None else ("合格" if r["passed"] == "True" else "不合格")
            print(f"    {c['id']} {c['title']}: {state}")
        if not mine:
            print(f"    カードなし{'（要るもの: ' + f['needs'] + '）' if f.get('needs') else ''}")


def main():
    commands = {"new": new, "run": run, "status": status, "flows": flows}
    if len(sys.argv) < 2 or sys.argv[1] not in commands:
        sys.exit(__doc__)
    commands[sys.argv[1]](*sys.argv[2:])


if __name__ == "__main__":
    main()
