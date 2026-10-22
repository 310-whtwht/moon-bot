# 東京時間のレンジを、欧州時間に抜けたら順張り
#
# 東京時間は値幅が小さく、欧州勢が入ると東京の高値・安値の外に溜まった損切りを
# 巻き込んで動き出しやすい、という見方を試す。
#
#   1. range_start〜range_end（既定 9:00〜15:00）の高値・安値を「東京レンジ」とする
#   2. range_end〜cutoff（既定 15:00〜21:00）に、終値がレンジの上（下）に
#      ATR × buffer 以上抜けたら買い（売り）。1日1回まで
#   3. exit_min（既定 翌 4:00）に決済。損切りは、レンジの反対側か ATR × stop_atr の近いほう
#      （stop_mode 1 = レンジの反対側、2 = ATR）
#
# 時刻は日本時間の分（9:00 = 540）。1日の区切りは朝 6:00。
# 欧州の夏時間・冬時間で欧州勢の入る時刻は1時間ずれるが、ここでは固定の時刻で見る。

PARAMS = {
    "range_start": 540,
    "range_end": 900,
    "cutoff": 1260,
    "exit_min": 240,
    "buffer": 0.0,
    "stop_mode": 1,
    "stop_atr": 1.5,
    "atr_period": 14,
}

def fx_minute(m):
    """日本時間の分を、朝 6:00 を 0 とする分に直す。"""
    return (m - 360) % 1440

def on_bar(bar, pos):
    a = atr(int(p.atr_period))
    if a == None:
        return hold()
    day = (bar.time + 3 * 3600) // 86400  # 朝 6:00（日本時間）で区切った日
    now = fx_minute(bar.hour * 60 + bar.minute)
    start, end, cutoff, done = fx_minute(p.range_start), fx_minute(p.range_end), fx_minute(p.cutoff), fx_minute(p.exit_min)

    if state.get("day") != day:
        state["day"] = day
        state["hi"], state["lo"], state["traded"] = None, None, False
        if pos != None:
            return exit(reason="日をまたいだので決済")

    if start <= now and now < end:
        state["hi"] = bar.high if state["hi"] == None else max(state["hi"], bar.high)
        state["lo"] = bar.low if state["lo"] == None else min(state["lo"], bar.low)
        return hold()

    hi, lo = state["hi"], state["lo"]
    if hi == None:
        return hold()
    explain("東京レンジ " + num(lo) + "〜" + num(hi))

    if pos != None and now >= done:
        return exit(reason="時間切れ")
    if state["traded"] or now < end or now >= cutoff:
        return hold()

    margin = p.buffer * a
    if bar.close > hi + margin:
        state["traded"] = True
        stop = lo if p.stop_mode == 1 else bar.close - p.stop_atr * a
        return buy(stop=min(stop, bar.close - 0.1 * a), reason="東京レンジの上抜け")
    if bar.close < lo - margin:
        state["traded"] = True
        stop = hi if p.stop_mode == 1 else bar.close + p.stop_atr * a
        return sell(stop=max(stop, bar.close + 0.1 * a), reason="東京レンジの下抜け")
    return hold()
