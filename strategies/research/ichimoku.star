# 一目均衡表
#
# 転換線 = 直近 tenkan 本の高値と安値の真ん中、基準線 = 直近 kijun 本の真ん中。
# 雲（先行スパン）は kijun 本前に計算した値: 先行スパン1 = (転換線 + 基準線) / 2、先行スパン2 = 直近 senkou 本の真ん中。
# 遅行スパンは、いまの終値を kijun 本前の終値と比べる。
#
#   mode 1: 雲抜け       … 終値が雲の上に抜けたら買い、下に抜けたら売り。雲の中に戻ったら決済
#   mode 2: 好転・逆転   … 転換線が基準線を上抜けたら買い、下抜けたら売り。反対の交差で決済
#                          cloud_filter 1 なら、雲の上でだけ買い・雲の下でだけ売る
#   mode 3: 三役好転・逆転 … 転換線 > 基準線、終値 > 雲、遅行スパン > kijun 本前の終値 がそろったら買い（逆は売り）。
#                          終値が基準線の反対側に出たら決済
#   損切りは ATR × stop_atr（どの mode も）

PARAMS = {
    "mode": 1,
    "tenkan": 9,
    "kijun": 26,
    "senkou": 52,
    "cloud_filter": 0,
    "atr_period": 14,
    "stop_atr": 2.0,
}

def mid(n, ago):
    h = highest(int(n), ago=ago)
    l = lowest(int(n), ago=ago)
    if h == None or l == None:
        return None
    return (h + l) / 2

def lines(ago):
    """ago 本前の時点の、転換線・基準線・雲の上端・雲の下端・遅行スパンの比較相手。"""
    k = int(p.kijun)
    t = mid(p.tenkan, ago)
    b = mid(p.kijun, ago)
    t_old = mid(p.tenkan, ago + k)
    b_old = mid(p.kijun, ago + k)
    s2 = mid(p.senkou, ago + k)
    past = close(ago=ago + k)
    if None in (t, b, t_old, b_old, s2, past):
        return None
    s1 = (t_old + b_old) / 2
    return {"t": t, "b": b, "top": max(s1, s2), "bottom": min(s1, s2), "past": past, "c": close(ago=ago)}

def sanyaku(x):
    """三役好転なら 1、三役逆転なら -1、どちらでもなければ 0。"""
    if x["t"] > x["b"] and x["c"] > x["top"] and x["c"] > x["past"]:
        return 1
    if x["t"] < x["b"] and x["c"] < x["bottom"] and x["c"] < x["past"]:
        return -1
    return 0

def on_bar(bar, pos):
    now = lines(0)
    prev = lines(1)
    a = atr(int(p.atr_period))
    if now == None or prev == None or a == None:
        return hold()
    c = now["c"]
    explain("転換 %s 基準 %s 雲 %s〜%s" % (num(now["t"]), num(now["b"]), num(now["bottom"]), num(now["top"])))

    long_in, short_in = False, False
    if p.mode == 1:
        long_in = prev["c"] <= prev["top"] and c > now["top"]
        short_in = prev["c"] >= prev["bottom"] and c < now["bottom"]
        if pos != None and pos.side == "long" and c < now["top"]:
            return exit(reason="雲に戻った")
        if pos != None and pos.side == "short" and c > now["bottom"]:
            return exit(reason="雲に戻った")
    elif p.mode == 2:
        up = prev["t"] <= prev["b"] and now["t"] > now["b"]
        down = prev["t"] >= prev["b"] and now["t"] < now["b"]
        long_in = up and (p.cloud_filter == 0 or c > now["top"])
        short_in = down and (p.cloud_filter == 0 or c < now["bottom"])
        if pos != None and pos.side == "long" and down:
            return exit(reason="逆転")
        if pos != None and pos.side == "short" and up:
            return exit(reason="好転")
    else:
        s, s_prev = sanyaku(now), sanyaku(prev)
        long_in = s == 1 and s_prev != 1
        short_in = s == -1 and s_prev != -1
        if pos != None and pos.side == "long" and c < now["b"]:
            return exit(reason="基準線割れ")
        if pos != None and pos.side == "short" and c > now["b"]:
            return exit(reason="基準線超え")

    if long_in and (pos == None or pos.side == "short"):
        return buy(stop=bar.close - p.stop_atr * a, reason="一目の買い")
    if short_in and (pos == None or pos.side == "long"):
        return sell(stop=bar.close + p.stop_atr * a, reason="一目の売り")
    return hold()
