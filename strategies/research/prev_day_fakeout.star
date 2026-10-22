# 前日の高値・安値の「ダマシ」と「抜け」
#
# 前日の高値・安値は誰もが見る価格で、その外側に損切りが溜まる。
#   mode 1（ダマシの逆張り）: 前日高値を足の中で上抜けたのに、終値が高値の内側に戻ったら売り。
#                             安値も同様に買い。損切りを刈ったあとの反転をねらう。
#   mode 2（抜けの順張り）:   終値が前日高値の上（安値の下）で引けたら買い（売り）。
#                             損切りの連鎖で加速するほうをねらう。
# どちらも1日1回まで。損切りは ATR × stop_atr、exit_min（既定 翌 5:00）に決済。
# 1日の区切りは朝 6:00（日本時間）。

PARAMS = {
    "mode": 1,
    "stop_atr": 1.0,
    "exit_min": 300,
    "atr_period": 14,
}

def fx_minute(m):
    return (m - 360) % 1440

def on_bar(bar, pos):
    a = atr(int(p.atr_period))
    day = (bar.time + 3 * 3600) // 86400
    if state.get("day") != day:
        # 日が替わった: 今日までの高安値を「前日」として持ち越す
        state["prev_hi"], state["prev_lo"] = state.get("hi"), state.get("lo")
        state["day"], state["hi"], state["lo"], state["traded"] = day, bar.high, bar.low, False
        if pos != None:
            return exit(reason="日をまたいだので決済")
    else:
        state["hi"], state["lo"] = max(state["hi"], bar.high), min(state["lo"], bar.low)

    ph, pl = state.get("prev_hi"), state.get("prev_lo")
    if a == None or ph == None:
        return hold()
    explain("前日 " + num(pl) + "〜" + num(ph))

    if pos != None and fx_minute(bar.hour * 60 + bar.minute) >= fx_minute(p.exit_min):
        return exit(reason="時間切れ")
    if state["traded"]:
        return hold()

    stop = p.stop_atr * a
    if p.mode == 1:
        if bar.high > ph and bar.close < ph:
            state["traded"] = True
            return sell(stop=max(bar.high, bar.close + stop), reason="前日高値のダマシ")
        if bar.low < pl and bar.close > pl:
            state["traded"] = True
            return buy(stop=min(bar.low, bar.close - stop), reason="前日安値のダマシ")
    else:
        if bar.close > ph:
            state["traded"] = True
            return buy(stop=bar.close - stop, reason="前日高値の上抜け")
        if bar.close < pl:
            state["traded"] = True
            return sell(stop=bar.close + stop, reason="前日安値の下抜け")
    return hold()
