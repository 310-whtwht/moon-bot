# 介入を警戒する価格帯の売り（5分足向け）
#
# 直近の介入の水準に近づくと買いが減る、という仮説。介入の後の暴落を狙うものではない。
# その時点で知られている直近の水準の zone 円以内なら entry_min に売り、hold_bars 本で決済する。
# 2024年7月より後の介入の有無は未確認（カードに記載）。
# カードは research/lab/hypotheses/H-0009-intervention-zone.yaml

PARAMS = {
    "zone": 1.0,
    "entry_min": 540,
    "hold_bars": 288,
    "stop_yen": 1.0,
}

# （この日より後に有効, 水準）: 2022/10、2024/4、2024/7 の介入の高値の近辺
LEVELS = [(20221021, 151.9), (20240429, 160.2), (20240711, 161.8)]

def level(today):
    out = None
    for since, price in LEVELS:
        if today > since:
            out = price
    return out

def on_bar(bar, pos):
    if pos != None:
        state["held"] = state.get("held", 0) + 1
        if state["held"] >= p.hold_bars:
            return exit(reason="価格帯の時間切れ")
        return hold()

    closes_at = bar.hour * 60 + bar.minute + 5
    today = bar.year * 10000 + bar.month * 100 + bar.day
    near = level(today)
    if bar.weekday <= 4 and closes_at == p.entry_min and near != None and bar.close >= near - p.zone:
        state["held"] = 0
        return sell(stop=bar.close + p.stop_yen, reason="介入の価格帯の売り")
    return hold()
