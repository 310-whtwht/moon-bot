# 週末前の解消（5分足向け）
#
# 週末を持ち越したくない参加者が建玉を閉じるので、その週に動いた向きと逆に動く、という仮説。
# 土曜 entry_min（既定 3:00、日本時間）に、月曜の始値から見て逆向きに入り、exit_min（既定 5:00）に決済する。
# 6時台はスプレッドが広がるので避ける。カードは research/lab/hypotheses/H-0004-friday-unwind.yaml

PARAMS = {
    "entry_min": 180,
    "exit_min": 300,
    "atr_period": 14,
    "stop_atr": 3.0,
}

def on_bar(bar, pos):
    if bar.weekday == 0 and bar.hour >= 7 and state.get("week") != bar.day:
        state["week"] = bar.day
        state["open"] = bar.open

    a = atr(int(p.atr_period))
    if a == None:
        return hold()

    closes_at = bar.hour * 60 + bar.minute + 5
    if pos != None:
        if bar.weekday != 5 or closes_at >= p.exit_min:
            return exit(reason="週末前の時間切れ")
        return hold()

    if bar.weekday == 5 and closes_at == p.entry_min and state.get("open") != None:
        if bar.close > state["open"]:
            return sell(stop=bar.close + p.stop_atr * a, reason="今週の上げの解消")
        return buy(stop=bar.close - p.stop_atr * a, reason="今週の下げの解消")
    return hold()
