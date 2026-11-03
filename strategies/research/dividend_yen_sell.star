# 配当の円売りの買い（5分足向け）
#
# 海外の株主が受け取った円の配当を自国の通貨に替えるので、支払いの時期は USD/JPY が上がる、という仮説。
# 期末配当: 株主総会の集中日の翌平日から days 平日（集中日は東証の公表値）。
# 中間配当: 12月の最初の平日から days 平日（支払日は会社ごとで、12月上旬で近似）。
# 期間中の平日は毎日 entry_min に買い、exit_min に決済する。仲値（9:55）のあとに入る。
# カードは research/lab/hypotheses/H-0011-dividend-yen-sell.yaml

PARAMS = {
    "days": 10,
    "entry_min": 630,
    "exit_min": 900,
    "atr_period": 14,
    "stop_atr": 6.0,
}

# 3月期決算会社の定時株主総会の集中日（東証）
AGM_PEAK = {2024: 27, 2025: 27, 2026: 26}

def on_bar(bar, pos):
    a = atr(int(p.atr_period))
    today = bar.year * 10000 + bar.month * 100 + bar.day
    if state.get("today") != today:
        state["today"] = today
        peak = AGM_PEAK.get(bar.year)
        if bar.month == 6 and peak != None and bar.day == peak + 1:
            state["count"] = 0
        if bar.month == 7 and bar.day == 1 and state.get("count") == None:
            state["count"] = None
        if bar.month == 12 and state.get("dec") != bar.year:
            state["dec"] = bar.year
            state["count"] = 0
        if state.get("count") != None and bar.weekday <= 4:
            state["count"] += 1
            if state["count"] > p.days:
                state["count"] = None
    if a == None:
        return hold()

    closes_at = bar.hour * 60 + bar.minute + 5
    if pos != None:
        if closes_at >= p.exit_min:
            return exit(reason="配当の時間切れ")
        return hold()
    if state.get("count") != None and bar.weekday <= 4 and closes_at == p.entry_min:
        return buy(stop=bar.close - p.stop_atr * a, reason="配当の円売りの買い")
    return hold()
