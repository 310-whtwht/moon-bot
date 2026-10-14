# トレンドライン（自動で線を引く）
#
# 人が目で引くトレンドラインを、次のルールで機械的に引きます。
#
#   1. 山と谷を見つける
#      ある足の高値が、前後それぞれ pivot 本の足の高値より高ければ「山」、
#      安値が前後 pivot 本より低ければ「谷」とします。
#      山・谷は pivot 本あとの足が確定した時点で分かります（先の値は使いません）。
#   2. 線を引く
#      下降トレンドライン（上値の抵抗線）: 直近2つの山を結んだ線。2つ目の山が1つ目より低いときだけ使います。
#      上昇トレンドライン（下値の支持線）: 直近2つの谷を結んだ線。2つ目の谷が1つ目より高いときだけ使います。
#      古い方の点が max_age 本より前なら、その線は古いとして使いません。
#   3. 売買する（mode で選びます）
#      mode 1（ブレイクアウト・順張り）:
#        終値が下降トレンドラインを上に抜けたら買い、上昇トレンドラインを下に抜けたら売り。
#      mode 2（反発・逆張り）:
#        上昇トレンドラインに安値が触れて、終値が線の上で終わったら買い。
#        下降トレンドラインに高値が触れて、終値が線の下で終わったら売り。
#        「触れた」は、線から ATR × touch_atr 以内に近づいたことを言います。
#   損切りは、入った価格から ATR × stop_atr 離れた位置です。
#   決済は、反対のシグナル（同時に反対方向で入り直します）か、損切りです。

PARAMS = {
    "mode": 1,
    "pivot": 5,
    "max_age": 150,
    "atr_period": 14,
    "stop_atr": 2.0,
    "touch_atr": 0.3,
}

def find_pivots():
    """pivot 本前の足が山・谷かを調べ、見つかったら state に残す（直近3つまで）。"""
    k = int(p.pivot)
    if bars() < 2 * k + 1:
        return
    center_high, center_low = high(ago=k), low(ago=k)
    is_peak, is_trough = True, True
    for j in range(2 * k + 1):
        if j == k:
            continue
        # 同じ値が並んだときは、新しい方の足を山・谷とする（j < k が新しい側）
        if high(ago=j) > center_high or (j < k and high(ago=j) == center_high):
            is_peak = False
        if low(ago=j) < center_low or (j < k and low(ago=j) == center_low):
            is_trough = False
    index = bars() - 1 - k
    if is_peak:
        state["peaks"] = (state.get("peaks", []) + [(index, center_high)])[-3:]
    if is_trough:
        state["troughs"] = (state.get("troughs", []) + [(index, center_low)])[-3:]

def trendline(points, falling):
    """直近2点を結んだ線を (基準の足, 価格, 傾き) で返す。使えないときは None。"""
    if len(points) < 2:
        return None
    i1, p1 = points[-2]
    i2, p2 = points[-1]
    if falling and p2 >= p1:
        return None
    if not falling and p2 <= p1:
        return None
    if bars() - 1 - i1 > p.max_age:
        return None
    return (i2, p2, (p2 - p1) / (i2 - i1))

def line_at(line, ago):
    """線の、ago 本前の足での値。"""
    i2, p2, slope = line
    return p2 + slope * (bars() - 1 - ago - i2)

def on_bar(bar, pos):
    find_pivots()
    a = atr(int(p.atr_period))
    if a == None or close(ago=1) == None:
        return hold()

    resistance = trendline(state.get("peaks", []), True)
    support = trendline(state.get("troughs", []), False)
    explain("抵抗線 " + (num(line_at(resistance, 0)) if resistance else "なし") +
            " / 支持線 " + (num(line_at(support, 0)) if support else "なし"))

    stop = p.stop_atr * a
    can_buy = pos == None or pos.side == "short"
    can_sell = pos == None or pos.side == "long"

    if p.mode == 1:
        if resistance and can_buy and close(ago=1) <= line_at(resistance, 1) and bar.close > line_at(resistance, 0):
            return buy(stop=bar.close - stop, reason="下降トレンドラインを上抜け")
        if support and can_sell and close(ago=1) >= line_at(support, 1) and bar.close < line_at(support, 0):
            return sell(stop=bar.close + stop, reason="上昇トレンドラインを下抜け")
    else:
        touch = p.touch_atr * a
        if support and can_buy and bar.low <= line_at(support, 0) + touch and bar.close > line_at(support, 0):
            return buy(stop=bar.close - stop, reason="上昇トレンドラインで反発")
        if resistance and can_sell and bar.high >= line_at(resistance, 0) - touch and bar.close < line_at(resistance, 0):
            return sell(stop=bar.close + stop, reason="下降トレンドラインで反落")
    return hold()
