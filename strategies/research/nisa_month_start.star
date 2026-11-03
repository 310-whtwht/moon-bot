# 月初の積立の買い（5分足向け）
#
# 月初に集まった積立の資金が外貨に替えられるので、USD/JPY が買われる、という仮説。
# 月の2営業日目（平日を数える。祝日は考えない）の entry_min に買い、exit_min に決済する。
# カードは research/lab/hypotheses/H-0007-nisa-month-start.yaml

PARAMS = {
    "business_day": 2,
    "entry_min": 540,
    "exit_min": 900,
    "atr_period": 14,
    "stop_atr": 6.0,
}

def on_bar(bar, pos):
    a = atr(int(p.atr_period))
    today = bar.year * 10000 + bar.month * 100 + bar.day
    if state.get("today") != today:
        state["today"] = today
        if state.get("month") != bar.month:
            state["month"] = bar.month
            state["count"] = 0
        if bar.weekday <= 4:
            state["count"] = state.get("count", 0) + 1
    if a == None:
        return hold()

    closes_at = bar.hour * 60 + bar.minute + 5
    if pos != None:
        if closes_at >= p.exit_min:
            return exit(reason="積立の時間切れ")
        return hold()
    if bar.weekday <= 4 and state["count"] == p.business_day and closes_at == p.entry_min:
        return buy(stop=bar.close - p.stop_atr * a, reason="月初の積立の買い")
    return hold()
