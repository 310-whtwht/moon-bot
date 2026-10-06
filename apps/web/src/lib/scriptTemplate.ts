/**
 * Starting point for a script strategy: the built-in EMA cross, written as a
 * script. See docs/strategy-scripts.md for everything a script can use.
 */
export const SCRIPT_TEMPLATE = `# 売買ルールのスクリプト（Starlark: Python に似た書き方）
# 確定した足ごとに on_bar が1回呼ばれます。

# パラメータ（数値のみ）。バージョンごとに値を変えられます。
PARAMS = {"fast": 12, "slow": 26, "atr_period": 14, "stop_atr": 2.0}

def on_bar(bar, pos):
    fast, slow = ema(p.fast), ema(p.slow)                  # 今の足の値
    fast1, slow1 = ema(p.fast, ago=1), ema(p.slow, ago=1)  # 1本前の値
    a = atr(p.atr_period)
    if None in (fast, slow, fast1, slow1, a):
        return hold()  # 足が足りず、まだ計算できない

    explain("fast=" + num(fast) + " slow=" + num(slow))  # 判定の記録に残すメモ

    crossed_up = fast1 <= slow1 and fast > slow
    crossed_down = fast1 >= slow1 and fast < slow

    # 新規には必ず損切り価格（stop）を付けます。
    if crossed_up and (pos == None or pos.side == "short"):
        return buy(stop=bar.close - p.stop_atr * a, reason="golden cross")
    if crossed_down and (pos == None or pos.side == "long"):
        return sell(stop=bar.close + p.stop_atr * a, reason="dead cross")
    return hold()
`;
