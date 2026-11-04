# 週末の動きの戻り（仮想通貨、1時間足向け）
#
# CME の先物が閉まっている週末は、薄い板で個人の注文だけが値を動かし、機関が戻ると戻る、という仮説。
# 金曜 21:00 UTC（CME の引けの近く）から日曜 22:00 UTC（再開の近く）までの動きが thresh_atr × ATR 以上なら、
# 日曜 22:00 UTC の足の終わりに逆向きに入り、hold_bars 本で決済する。夏時間の1時間のずれは無視する。
# カードは research/lab/hypotheses/H-0014-crypto-weekend-fade.yaml

PARAMS = {
    "thresh_atr": 6.0,
    "hold_bars": 24,
    "atr_period": 24,
    "stop_atr": 6.0,
}

def on_bar(bar, pos):
    a = atr(int(p.atr_period))
    utc_min = (bar.time // 60) % 1440 + 60   # この足が閉じる時刻（UTC の分）
    utc_day = (bar.time // 86400 + 3) % 7     # UTC の曜日（月曜=0）。1970-01-01 は木曜
    if utc_min == 1440:
        utc_min, utc_day = 0, (utc_day + 1) % 7

    if utc_day == 4 and utc_min == 21 * 60:
        state["friday"] = bar.close
    if a == None:
        return hold()

    if pos != None:
        state["held"] = state.get("held", 0) + 1
        if state["held"] >= p.hold_bars:
            return exit(reason="週明けの時間切れ")
        return hold()

    if utc_day == 6 and utc_min == 22 * 60 and state.get("friday") != None:
        move = bar.close - state["friday"]
        state["friday"] = None
        if abs(move) < p.thresh_atr * a:
            return hold()
        state["held"] = 0
        if move > 0:
            return sell(stop=bar.close + p.stop_atr * a, reason="週末の上げの戻り")
        return buy(stop=bar.close - p.stop_atr * a, reason="週末の下げの戻り")
    return hold()
