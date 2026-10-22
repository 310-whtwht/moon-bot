# 仲値（9:55）とゴトー日（USD/JPY 向け）
#
# 輸入企業のドル買いが、東京の仲値（9:55 公示）に向けて集まる。とくに決済の多い
# 5・10日（ゴトー日）は朝にドルが買われやすく、仲値のあとは戻りやすい、という見方を試す。
#
#   mode 1: 仲値前の買い   … entry_min（既定 8:00）に買い、9:55 に決済
#   mode 2: 仲値後の売り   … 9:55 に売り、after_min（既定 11:30）に決済
#   mode 3: 両方           … 仲値前に買い、9:55 に売りへドテンし、after_min に決済
#   gotobi_only 1: ゴトー日だけ売買する / 0: 平日は毎日売買する（効果を比べるための対照）
#
# ゴトー日: 5・10・15・20・25日と月末。土日に当たるときは前の金曜日。日本の祝日は考慮しない。
# 時刻は日本時間の分（8:00 = 480）。足の長さで割り切れる時刻を指定する（5分足・15分足向け）。

PARAMS = {
    "mode": 1,
    "gotobi_only": 1,
    "entry_min": 480,
    "fix_min": 595,
    "after_min": 690,
    "atr_period": 14,
    "stop_atr": 3.0,
}

def bar_minutes(bar):
    """足の長さ（分）。連続する足の時刻の差のうち最小のもの。"""
    prev = state.get("prev_time")
    state["prev_time"] = bar.time
    if prev != None:
        gap = (bar.time - prev) // 60
        if gap > 0 and (state.get("len") == None or gap < state["len"]):
            state["len"] = gap
    return state.get("len")

def is_gotobi(bar):
    if bar.weekday > 4:
        return False
    days = [5, 10, 15, 20, 25, bar.days_in_month]
    if bar.day in days:
        return True
    # 土日に当たるゴトー日は、前の金曜日に繰り上がる
    if bar.weekday == 4:
        for ahead in (1, 2):
            if bar.day + ahead <= bar.days_in_month and (bar.day + ahead) in days:
                return True
    return False

def on_bar(bar, pos):
    length = bar_minutes(bar)
    a = atr(int(p.atr_period))
    if length == None or a == None:
        return hold()

    # この足が閉じる時刻（日本時間の分）。シグナルは次の足の始まりで約定する。
    closes_at = bar.hour * 60 + bar.minute + length
    target_day = is_gotobi(bar) or p.gotobi_only == 0
    explain("ゴトー日" if is_gotobi(bar) else "通常日")

    if closes_at == p.entry_min and target_day and bar.weekday <= 4 and p.mode in (1, 3) and pos == None:
        return buy(stop=bar.close - p.stop_atr * a, reason="仲値前の買い")
    if closes_at == p.fix_min:
        if p.mode in (2, 3) and target_day and bar.weekday <= 4:
            return sell(stop=bar.close + p.stop_atr * a, reason="仲値後の売り")
        if pos != None:
            return exit(reason="仲値で決済")
    if closes_at == p.after_min and pos != None:
        return exit(reason="仲値後の時間切れ")
    return hold()
