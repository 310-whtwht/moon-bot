# RSI とボリンジャーバンドの逆張り（Gemini の提案3。1時間足向け）
#
#   売り: 1本前の足で RSI(14) > rsi_hi かつ 高値がバンド(20, 2σ)の上限に触れ、いまの足が陰線なら売り
#   買い: 逆（RSI < 100 − rsi_hi、安値が下限に触れ、陽線）。sides 1 なら売りだけ（提案どおり）、2 なら両方
#   損切り: バンド(20, 3σ)の外側（売りは上限 3σ、買いは下限 3σ）
#   利確:   終値がバンドの中心線（SMA20）に届いたら決済
#   VIX などの外部データによる除外は、まだ入れていない

PARAMS = {
    "period": 20,
    "rsi_period": 14,
    "rsi_hi": 70,
    "sides": 1,
}

def sqrt(x):
    """平方根（Starlark には ** も math も無いので、ニュートン法で求める）。"""
    if x <= 0:
        return 0.0
    r = x if x > 1 else 1.0
    for _ in range(40):
        r = (r + x / r) / 2
    return r

def band(ago):
    n = int(p.period)
    m = sma(n, ago=ago)
    if m == None:
        return None
    s = 0.0
    for i in range(n):
        x = close(ago=ago + i)
        if x == None:
            return None
        s += (x - m) * (x - m)
    sd = sqrt(s / n)
    return m, sd

def on_bar(bar, pos):
    now, prev = band(0), band(1)
    r1 = rsi(int(p.rsi_period), ago=1)
    if now == None or prev == None or r1 == None:
        return hold()
    m, sd = now
    pm, psd = prev

    if pos != None:
        if (pos.side == "short" and bar.close <= m) or (pos.side == "long" and bar.close >= m):
            return exit(reason="中心線")
        return hold()

    if r1 > p.rsi_hi and high(ago=1) >= pm + 2 * psd and bar.close < bar.open:
        return sell(stop=m + 3 * sd, reason="買われすぎの売り")
    if p.sides == 2 and r1 < 100 - p.rsi_hi and low(ago=1) <= pm - 2 * psd and bar.close > bar.open:
        return buy(stop=m - 3 * sd, reason="売られすぎの買い")
    return hold()
