# 早朝の動きの戻り（5分足向け）
#
# 6〜7時のスプレッドの拡大で損切りが不利な値で約定し、その分の動きが戻る、という仮説。
# start_min（既定 5:30、日本時間）から entry_min（既定 7:05）の動きが ATR × thresh_atr を超えたら、
# 逆向きに入り、exit_min（既定 8:30）に決済する。カードは research/lab/hypotheses/H-0006-early-fade.yaml

PARAMS = {
    "start_min": 330,
    "entry_min": 425,
    "exit_min": 510,
    "thresh_atr": 3.0,
    "atr_period": 14,
    "stop_atr": 3.0,
}

def on_bar(bar, pos):
    a = atr(int(p.atr_period))
    if a == None:
        return hold()

    closes_at = bar.hour * 60 + bar.minute + 5
    if closes_at == p.start_min:
        state["start_day"] = bar.day
        state["start"] = bar.close

    if pos != None:
        if closes_at >= p.exit_min:
            return exit(reason="早朝の時間切れ")
        return hold()

    if closes_at == p.entry_min and state.get("start_day") == bar.day:
        move = bar.close - state["start"]
        if move > p.thresh_atr * a:
            return sell(stop=bar.close + p.stop_atr * a, reason="早朝の上げの戻り")
        if move < -p.thresh_atr * a:
            return buy(stop=bar.close - p.stop_atr * a, reason="早朝の下げの戻り")
    return hold()
