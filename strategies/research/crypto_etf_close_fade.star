# ETF の基準の時刻のあとの戻り（仮想通貨、1時間足向け）
#
# 米国の現物 ETF の設定・解約は、ニューヨーク 16:00 の価格に合わせて売買されるので、
# 15:00〜16:00 の動きは 16:00 のあとに戻る、という仮説。
# 米国の平日に、16:00（NY）に閉じる足の動きが thresh_atr × ATR 以上なら逆向きに入り、hold_bars 本で決済する。
# カードは research/lab/hypotheses/H-0016-crypto-etf-close-fade.yaml

PARAMS = {
    "thresh_atr": 1.5,
    "hold_bars": 2,
    "atr_period": 24,
    "stop_atr": 3.0,
}

def us_dst(bar):
    """米国の夏時間か。足の日付（日本時間）の前日を米国の日付とみなす（16:00 NY は日本時間の翌朝）。"""
    day, weekday, month = bar.day - 1, (bar.weekday + 6) % 7, bar.month
    if day == 0:
        # 日本時間の1日は、米国では前の月の末日。夏時間の切り替えは月末にないので、月で決まる
        return month - 1 >= 3 and month - 1 <= 10
    if month > 3 and month < 11:
        return True
    if month < 3 or month > 11:
        return False
    if month == 3:
        second_sunday = 8 + (6 - (weekday - (day - 8)) % 7) % 7
        return day >= second_sunday
    first_sunday = 1 + (6 - (weekday - (day - 1)) % 7) % 7
    return day < first_sunday

def on_bar(bar, pos):
    a = atr(int(p.atr_period))
    if a == None:
        return hold()

    if pos != None:
        state["held"] = state.get("held", 0) + 1
        if state["held"] >= p.hold_bars:
            return exit(reason="基準の時刻のあとの時間切れ")
        return hold()

    closes_utc = ((bar.time // 60) % 1440 + 60) % 1440
    fix_utc = (20 if us_dst(bar) else 21) * 60
    us_weekday = (bar.weekday + 6) % 7
    if closes_utc != fix_utc or us_weekday > 4:
        return hold()
    move = bar.close - bar.open
    if abs(move) < p.thresh_atr * a:
        return hold()
    state["held"] = 0
    if move > 0:
        return sell(stop=bar.close + p.stop_atr * a, reason="基準の時刻の上げの戻り")
    return buy(stop=bar.close - p.stop_atr * a, reason="基準の時刻の下げの戻り")
