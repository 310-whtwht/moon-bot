# 日足のドンチャン・ブレイク（金利差の向きだけ）
#
# 終値が直近 entry_n 日の終値の最高値を超えたら買い、直近 exit_n 日の終値の最安値を割ったら決済する。
# 売りはその逆。どちらの向きで入るかは、金利差の向き（carry_sign）で決める。
#   carry_sign  1 … 買いだけ（相手通貨の金利 > 円の金利。2026-10 時点の USD/JPY はこれ）
#   carry_sign -1 … 売りだけ
#   carry_sign  0 … 新しく入らない（持っている建玉の決済だけ行う）
# 金利差の向きはボットには分からないので、政策金利が変わったら画面から手で変える。
#
# 長期データ（FRED、1985〜2026）での評価は docs/backtests/2026-10-27-longterm-donchian-cot.md。
# 研究と同じく終値だけで判断する。日中の高値・安値で入ったり決済したりはしない。
# 入るときの損切り（stop_atr × ATR）は、急な動きへの備え。研究には無い。

PARAMS = {
    "entry_n": 50,
    "exit_n": 20,
    "carry_sign": 1,
    "atr_period": 20,
    "stop_atr": 5.0,
}

def channel(n, pick):
    """直近 n 本（今の足を含まない）の終値の最高値または最安値。"""
    out = close(1)
    for i in range(2, n + 1):
        out = pick(out, close(i))
    return out

def on_bar(bar, pos):
    entry_n, exit_n = int(p.entry_n), int(p.exit_n)
    a = atr(int(p.atr_period))
    if bars() <= max(entry_n, exit_n) or a == None:
        return hold()

    c = close()
    hi_entry, lo_entry = channel(entry_n, max), channel(entry_n, min)
    hi_exit, lo_exit = channel(exit_n, max), channel(exit_n, min)

    if pos != None:
        if pos.side == "long":
            explain("買いを保有。決済ライン " + num(lo_exit))
            if c < lo_exit:
                return exit(reason="%d日安値を割った" % exit_n)
        else:
            explain("売りを保有。決済ライン " + num(hi_exit))
            if c > hi_exit:
                return exit(reason="%d日高値を超えた" % exit_n)
        return hold()

    explain("入るライン 買い " + num(hi_entry) + " / 売り " + num(lo_entry) + "（向き " + str(int(p.carry_sign)) + "）")
    if p.carry_sign > 0 and c > hi_entry:
        return buy(stop=c - p.stop_atr * a, reason="%d日高値を超えた" % entry_n)
    if p.carry_sign < 0 and c < lo_entry:
        return sell(stop=c + p.stop_atr * a, reason="%d日安値を割った" % entry_n)
    return hold()
