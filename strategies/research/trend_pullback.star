# 上位足のトレンドに沿った押し目買い・戻り売り（Gemini の提案1。4時間足向け）
#
#   トレンド: 日足の EMA20 / EMA80 と ADX(14) の代わりに、4時間足の EMA(20×6) / EMA(80×6) と ADX(14×6) を使う
#             EMA(短) > EMA(長) かつ ADX > adx_min なら上昇トレンド（逆は下降）
#   押し目:   1本前の足が 4時間足の EMA20 に触れた
#   入り:     いまの足が陽線で、1本前の高値を終値で上抜けた（包み足）。売りは逆
#   損切り:   直近5本の安値 − ATR × sl_atr（売りは逆）。この幅を 1R とする
#   利確:     終値が 建値 + tp_r × R に届いたら決済
#   建値撤退: 終値が 建値 + be_r × R に一度届いたら、その後に終値が建値を割ったら決済
#   （利確と建値撤退は終値で判断する。足の途中では動かさない）

PARAMS = {
    "day_bars": 6,
    "fast": 20,
    "slow": 80,
    "adx_period": 14,
    "adx_min": 20,
    "pull_ema": 20,
    "atr_period": 14,
    "sl_atr": 1.0,
    "tp_r": 2.5,
    "be_r": 1.5,
}

def update_adx(n):
    """Wilder の ADX を、足ごとに state で更新する。"""
    h, l, c = high(), low(), close()
    ph, pl, pc = high(ago=1), low(ago=1), close(ago=1)
    if ph == None:
        return None
    tr = max(h - l, abs(h - pc), abs(l - pc))
    up, down = h - ph, pl - l
    pdm = up if up > down and up > 0 else 0.0
    mdm = down if down > up and down > 0 else 0.0
    if state.get("tr") == None:
        state["tr"], state["pdm"], state["mdm"], state["k"] = 0.0, 0.0, 0.0, 0
    state["k"] += 1
    if state["k"] <= n:
        state["tr"] += tr
        state["pdm"] += pdm
        state["mdm"] += mdm
        return None
    state["tr"] = state["tr"] - state["tr"] / n + tr
    state["pdm"] = state["pdm"] - state["pdm"] / n + pdm
    state["mdm"] = state["mdm"] - state["mdm"] / n + mdm
    if state["tr"] <= 0:
        return None
    pdi, mdi = 100 * state["pdm"] / state["tr"], 100 * state["mdm"] / state["tr"]
    dx = 100 * abs(pdi - mdi) / (pdi + mdi) if pdi + mdi > 0 else 0.0
    if state.get("adx") == None:
        state["dxs"] = state.get("dxs", []) + [dx]
        if len(state["dxs"]) < n:
            return None
        total = 0.0
        for x in state["dxs"]:
            total += x
        state["adx"] = total / n
    else:
        state["adx"] = (state["adx"] * (n - 1) + dx) / n
    return state["adx"]

def on_bar(bar, pos):
    d = int(p.day_bars)
    adx = update_adx(int(p.adx_period) * d)
    fast, slow = ema(int(p.fast) * d), ema(int(p.slow) * d)
    pull = ema(int(p.pull_ema), ago=1)
    a = atr(int(p.atr_period))
    lo, hi = lowest(5), highest(5)
    if None in (adx, fast, slow, pull, a, lo, hi, close(ago=1)):
        return hold()
    explain("ADX %s" % num(adx))

    if pos != None:
        r = state["r"]
        if pos.side == "long":
            if bar.close >= pos.entry + p.tp_r * r:
                return exit(reason="利確")
            if bar.close >= pos.entry + p.be_r * r:
                state["be"] = True
            if state.get("be") and bar.close <= pos.entry:
                return exit(reason="建値撤退")
        else:
            if bar.close <= pos.entry - p.tp_r * r:
                return exit(reason="利確")
            if bar.close <= pos.entry - p.be_r * r:
                state["be"] = True
            if state.get("be") and bar.close >= pos.entry:
                return exit(reason="建値撤退")
        return hold()

    trend_up = fast > slow and adx > p.adx_min
    trend_down = fast < slow and adx > p.adx_min
    bull = bar.close > bar.open and bar.close > high(ago=1) and close(ago=1) < open(ago=1)
    bear = bar.close < bar.open and bar.close < low(ago=1) and close(ago=1) > open(ago=1)
    if trend_up and low(ago=1) <= pull and bull:
        stop = lo - p.sl_atr * a
        state["r"], state["be"] = bar.close - stop, False
        return buy(stop=stop, reason="押し目買い")
    if trend_down and high(ago=1) >= pull and bear:
        stop = hi + p.sl_atr * a
        state["r"], state["be"] = stop - bar.close, False
        return sell(stop=stop, reason="戻り売り")
    return hold()
