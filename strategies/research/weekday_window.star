# 曜日と時刻で決まった窓だけ持つ（時間帯のくせを、実際の売値・買値で確かめる）
#
#   weekday:   入る曜日（日本時間、月曜=0）。-1 なら平日すべて
#   entry_min: 入る時刻（日本時間の分。5:30 = 330）。この時刻に閉じる足でシグナルを出し、次の足で約定する
#   exit_min:  決済する時刻（日本時間の分）。entry_min より小さければ翌日のその時刻
#   side:      1 = 買い / -1 = 売り
#   stop_atr:  損切り（ATR × stop_atr）。入るには必ず損切りが要るので、使わないときは大きくする
#
# research/seasonality/scan.py で見つけた候補を確かめるためのもの。

PARAMS = {
    "weekday": 3,
    "entry_min": 330,
    "exit_min": 450,
    "side": -1,
    "atr_period": 14,
    "stop_atr": 10.0,
}

def bar_minutes(bar):
    prev = state.get("prev_time")
    state["prev_time"] = bar.time
    if prev != None:
        gap = (bar.time - prev) // 60
        if gap > 0 and (state.get("len") == None or gap < state["len"]):
            state["len"] = gap
    return state.get("len")

def on_bar(bar, pos):
    length = bar_minutes(bar)
    a = atr(int(p.atr_period))
    if length == None or a == None:
        return hold()
    closes_at = (bar.hour * 60 + bar.minute + length) % 1440
    # 足が閉じる時刻の曜日（日付をまたぐ足は翌日）
    wd = (bar.weekday + (bar.hour * 60 + bar.minute + length) // 1440) % 7

    if pos != None and closes_at == p.exit_min:
        return exit(reason="時間切れ")
    if pos == None and closes_at == p.entry_min and (p.weekday < 0 and wd <= 4 or wd == p.weekday):
        if p.side > 0:
            return buy(stop=bar.close - p.stop_atr * a, reason="時間帯の買い")
        return sell(stop=bar.close + p.stop_atr * a, reason="時間帯の売り")
    return hold()
