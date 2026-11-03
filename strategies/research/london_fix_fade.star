# ロンドン16時のフィキシング後の戻り（5分足向け）
#
# 公示の前1時間の偏った注文が止まると、その動きが戻る、という仮説。
# 16:00（ロンドン時間）の前 window 本の動きが thresh_atr × ATR 以上なら、
# 公示の wait 本あとに逆向きへ入り、hold_bars 本で決済する。
# カードは research/lab/hypotheses/H-0010-london-fix-fade.yaml

PARAMS = {
    "window": 12,
    "thresh_atr": 3.0,
    "wait": 1,
    "hold_bars": 12,
    "atr_period": 14,
    "stop_atr": 3.0,
}

def is_bst(bar):
    """英国の夏時間。3月と10月の最終日曜の前後を、日本時間の日付で判定する。"""
    if bar.month < 3 or bar.month > 10:
        return False
    if bar.month > 3 and bar.month < 10:
        return True
    sunday = bar.day + (6 - bar.weekday)
    last_sunday = 25 + (sunday - 25) % 7
    return bar.day >= last_sunday if bar.month == 3 else bar.day < last_sunday

def on_bar(bar, pos):
    a = atr(int(p.atr_period))
    closes = state.get("closes", [])
    closes.append(bar.close)
    if len(closes) > p.window + p.wait + 1:
        closes.pop(0)
    state["closes"] = closes
    if a == None or len(closes) <= p.window + p.wait:
        return hold()

    if pos != None:
        state["held"] = state.get("held", 0) + 1
        if state["held"] >= p.hold_bars:
            return exit(reason="フィキシング後の時間切れ")
        return hold()

    fix_utc = 15 * 60 if is_bst(bar) else 16 * 60
    now_utc = (bar.time // 60) % 1440 + 5
    if bar.weekday == 0 or bar.weekday == 6 or now_utc != fix_utc + 5 * p.wait:
        return hold()
    move = closes[-1 - int(p.wait)] - closes[0]
    if abs(move) < p.thresh_atr * a:
        return hold()
    state["held"] = 0
    if move > 0:
        return sell(stop=bar.close + p.stop_atr * a, reason="フィキシング後の売り")
    return buy(stop=bar.close - p.stop_atr * a, reason="フィキシング後の買い")
