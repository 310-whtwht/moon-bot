# ロスカットのあとの戻り（5分足向け）
#
# 買いに偏る個人の建玉が急落でロスカットされ、連鎖が終わると戻る、という仮説。
# 建玉の分布がないので、1時間の下げの大きさで近似する。
# lookback 本の下げが thresh_atr × ATR 以上で買い、hold_bars 本で決済する。
# スプレッドが開く時間帯（5〜8時台）には入らない。
# カードは research/lab/hypotheses/H-0008-liquidation-rebound.yaml

PARAMS = {
    "lookback": 12,
    "thresh_atr": 8.0,
    "hold_bars": 24,
    "atr_period": 14,
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

    if bar.weekday > 4 or (bar.hour >= 5 and bar.hour <= 8):
        return hold()
    if closes[0] - bar.close >= p.thresh_atr * a:
        state["held"] = 0
        return buy(stop=bar.close - p.stop_atr * a, reason="急落後の買い")
    return hold()
