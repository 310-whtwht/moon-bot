# 週明けの窓（ギャップ）埋め
#
# 週末のニュースで、月曜の朝は金曜の終値から離れて始まることがある（窓）。
# 「窓は埋まる」と考える人が多く、窓を閉じる向きに動きやすいか、を試す。
#
#   週の最初の足（前の足から 24 時間以上空いた足）で、窓の大きさが min_gap × ATR 以上なら、
#   窓を閉じる向き（上に開いたら売り、下に開いたら買い）に入る。
#   金曜の終値に届いたら利確、損切りは ATR × stop_atr、hold_bars 本たったら決済。
#   mode 2 にすると逆向き（窓の方向に乗る）。

PARAMS = {
    "mode": 1,
    "min_gap": 0.5,
    "atr_period": 14,
    "stop_atr": 2.0,
    "hold_bars": 48,
}

def on_bar(bar, pos):
    prev_time = state.get("prev_time")
    prev_close = state.get("prev_close")
    state["prev_time"] = bar.time
    state["prev_close"] = bar.close
    a = atr(int(p.atr_period))
    if a == None or prev_time == None:
        return hold()

    if pos != None:
        state["held"] = state.get("held", 0) + 1
        target = state["target"]
        if target != None and ((pos.side == "long" and bar.close >= target) or (pos.side == "short" and bar.close <= target)):
            return exit(reason="窓埋め")
        if state["held"] >= p.hold_bars:
            return exit(reason="時間切れ")
        return hold()

    if bar.time - prev_time < 24 * 3600:
        return hold()
    gap = bar.open - prev_close
    explain("窓 %s" % num(gap))
    if abs(gap) < p.min_gap * a:
        return hold()
    state["held"] = 0
    state["target"] = prev_close
    sell_side = gap > 0
    if p.mode == 2:
        sell_side = not sell_side
        state["target"] = None  # 順張りは利確の目標を置かない（時間切れか損切り）
    if sell_side:
        return sell(stop=bar.close + p.stop_atr * a, reason="窓埋めの売り" if p.mode == 1 else "窓の向きに売り")
    return buy(stop=bar.close - p.stop_atr * a, reason="窓埋めの買い" if p.mode == 1 else "窓の向きに買い")
