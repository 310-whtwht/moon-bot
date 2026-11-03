# スワップを受け取ったあとの売り（5分足向け）
#
# 3日分のスワップを受け取った参加者が建玉を閉じるので、高金利通貨が売られる、という仮説。
# 木曜 entry_min（既定 7:30、日本時間）に売り、exit_min（既定 9:30）に決済する。
# 3日分が水曜の NY クローズに付くと仮定した（GMO の実際の曜日は未確認）。
# カードは research/lab/hypotheses/H-0005-swap-after-grant.yaml

PARAMS = {
    "weekday": 3,
    "entry_min": 450,
    "exit_min": 570,
    "atr_period": 14,
    "stop_atr": 3.0,
}

def on_bar(bar, pos):
    a = atr(int(p.atr_period))
    if a == None:
        return hold()

    closes_at = bar.hour * 60 + bar.minute + 5
    if pos != None:
        if bar.weekday != p.weekday or closes_at >= p.exit_min:
            return exit(reason="スワップ後の時間切れ")
        return hold()

    if bar.weekday == p.weekday and closes_at == p.entry_min:
        return sell(stop=bar.close + p.stop_atr * a, reason="スワップ後の売り")
    return hold()
