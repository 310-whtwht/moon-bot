# NY 時間のロンドンレンジ抜け（Gemini の提案2。15分足向け）
#
#   レンジ:   日本時間 16:00〜21:00 の高値・安値
#   入り:     21:30〜23:30 に閉じる足で、終値がレンジの上を抜け、実体が ATR × body_atr 以上なら買い（下は売り）
#   損切り:   レンジの中値と ATR × sl_atr のうち、近いほう
#   利確:     終値が 建値 + tp_r × R に届いたら決済
#   時間決済: 入ってから hold_bars 本（15分足で4時間 = 16本）たったら決済
#   1日1回まで

PARAMS = {
    "range_start": 960,
    "range_end": 1260,
    "entry_from": 1290,
    "entry_to": 1410,
    "body_atr": 1.2,
    "atr_period": 14,
    "sl_atr": 1.2,
    "tp_r": 1.8,
    "hold_bars": 16,
}

def on_bar(bar, pos):
    a = atr(int(p.atr_period))
    if a == None:
        return hold()
    start = bar.hour * 60 + bar.minute
    closes = start + 15
    day = bar.year * 10000 + bar.month * 100 + bar.day
    if state.get("day") != day:
        state["day"], state["hi"], state["lo"], state["done"] = day, None, None, False

    if start >= p.range_start and closes <= p.range_end:
        state["hi"] = bar.high if state["hi"] == None else max(state["hi"], bar.high)
        state["lo"] = bar.low if state["lo"] == None else min(state["lo"], bar.low)

    if pos != None:
        state["held"] += 1
        r = state["r"]
        if (pos.side == "long" and bar.close >= pos.entry + p.tp_r * r) or (pos.side == "short" and bar.close <= pos.entry - p.tp_r * r):
            return exit(reason="利確")
        if state["held"] >= p.hold_bars:
            return exit(reason="時間切れ")
        return hold()

    hi, lo = state["hi"], state["lo"]
    if hi == None or state["done"] or closes < p.entry_from or closes > p.entry_to:
        return hold()
    body = abs(bar.close - bar.open)
    mid = (hi + lo) / 2
    if bar.close > hi and body >= p.body_atr * a:
        stop = max(mid, bar.close - p.sl_atr * a)
        state["r"], state["held"], state["done"] = bar.close - stop, 0, True
        return buy(stop=stop, reason="ロンドン高値抜け")
    if bar.close < lo and body >= p.body_atr * a:
        stop = min(mid, bar.close + p.sl_atr * a)
        state["r"], state["held"], state["done"] = stop - bar.close, 0, True
        return sell(stop=stop, reason="ロンドン安値抜け")
    return hold()
