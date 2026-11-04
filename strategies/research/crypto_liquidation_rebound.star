# ロスカットのあとの戻り（仮想通貨、1時間足向け）
#
# レバレッジをかけた買いが急落で強制決済され、連鎖が終わると戻る、という仮説（FX では H-0008 で不合格）。
# lookback 本の下げが thresh_atr × ATR 以上で買い、hold_bars 本で決済する。
# カードは research/lab/hypotheses/H-0015-crypto-liquidation-rebound.yaml

PARAMS = {
    "lookback": 4,
    "thresh_atr": 8.0,
    "hold_bars": 12,
    "atr_period": 24,
    "stop_atr": 3.0,
}

def on_bar(bar, pos):
    a = atr(int(p.atr_period))
    closes = state.get("closes", [])
    closes.append(bar.close)
    if len(closes) > p.lookback + 1:
        closes.pop(0)
    state["closes"] = closes
    if a == None or len(closes) <= p.lookback:
        return hold()

    if pos != None:
        state["held"] = state.get("held", 0) + 1
        if state["held"] >= p.hold_bars:
            return exit(reason="戻りの時間切れ")
        return hold()

    if closes[0] - bar.close >= p.thresh_atr * a:
        state["held"] = 0
        return buy(stop=bar.close - p.stop_atr * a, reason="急落後の買い")
    return hold()
