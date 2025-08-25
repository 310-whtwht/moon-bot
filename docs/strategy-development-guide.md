# 戦略開発ガイド

## 概要

moomoo 自動売買システムでの戦略開発について説明します。Starlark VM を使用して戦略を記述し、リアルタイムで実行できます。

## 戦略の基本構造

### 戦略テンプレート

```python
# 戦略の基本構造
def init():
    # 初期化処理
    pass

def on_bar(symbol, timeframe, data):
    # バー更新時の処理
    pass

def on_order_fill(order_id, symbol, side, quantity, price):
    # 注文約定時の処理
    pass
```

### パラメータ定義

```python
# パラメータの定義
params = {
    "fast_period": 12,      # 短期移動平均期間
    "slow_period": 26,      # 長期移動平均期間
    "position_size": 0.25   # ポジションサイズ（資金の25%）
}
```

## ビルトイン関数

### データアクセス

#### `data.get_price(symbol, timeframe, period)`
指定した銘柄・時間軸の価格データを取得します。

```python
# 最新の終値
close = data.get_price(symbol, "1m", 0)

# 1分前の終値
prev_close = data.get_price(symbol, "1m", 1)

# 5分前の高値
high_5m = data.get_price(symbol, "5m", 5, "high")
```

#### `data.get_volume(symbol, timeframe, period)`
出来高データを取得します。

```python
volume = data.get_volume(symbol, "1m", 0)
```

### テクニカル指標

#### `ind.ema(prices, period)`
指数移動平均を計算します。

```python
prices = [data.get_price(symbol, "1m", i) for i in range(20)]
ema_12 = ind.ema(prices, 12)
```

#### `ind.rsi(prices, period)`
RSI（相対力指数）を計算します。

```python
rsi_14 = ind.rsi(prices, 14)
```

#### `ind.atr(highs, lows, closes, period)`
ATR（平均真の範囲）を計算します。

```python
atr_14 = ind.atr(highs, lows, closes, 14)
```

#### `ind.bollinger_bands(prices, period, std_dev)`
ボリンジャーバンドを計算します。

```python
bb_upper, bb_middle, bb_lower = ind.bollinger_bands(prices, 20, 2)
```

### 注文機能

#### `order.market(symbol, side, quantity)`
成行注文を発注します。

```python
order_id = order.market("AAPL", "buy", 100)
```

#### `order.limit(symbol, side, quantity, price)`
指値注文を発注します。

```python
order_id = order.limit("AAPL", "sell", 100, 155.00)
```

#### `order.stop(symbol, side, quantity, stop_price)`
逆指値注文を発注します。

```python
order_id = order.stop("AAPL", "sell", 100, 145.00)
```

#### `order.cancel(order_id)`
注文をキャンセルします。

```python
order.cancel(order_id)
```

### リスク管理

#### `risk.get_position(symbol)`
現在のポジション情報を取得します。

```python
position = risk.get_position("AAPL")
if position:
    quantity = position["quantity"]
    avg_price = position["avg_price"]
```

#### `risk.get_account_value()`
口座残高を取得します。

```python
account_value = risk.get_account_value()
```

#### `risk.calculate_position_size(risk_per_trade)`
リスクに基づいてポジションサイズを計算します。

```python
# 1トレードあたり0.25%のリスク
size = risk.calculate_position_size(0.0025)
```

### 状態管理

#### `state.get(key)`
戦略の状態を取得します。

```python
last_signal = state.get("last_signal")
```

#### `state.set(key, value)`
戦略の状態を設定します。

```python
state.set("last_signal", "buy")
```

### ログ出力

#### `log.info(message)`
情報ログを出力します。

```python
log.info("EMA cross detected: buy signal")
```

#### `log.warn(message)`
警告ログを出力します。

```python
log.warn("Position size exceeds limit")
```

#### `log.error(message)`
エラーログを出力します。

```python
log.error("Failed to place order: " + str(error))
```

## 戦略例

### EMA クロス戦略

```python
def init():
    state.set("position", "flat")
    state.set("last_signal", None)

def on_bar(symbol, timeframe, data):
    if timeframe != "1m":
        return
    
    # 価格データ取得
    prices = []
    for i in range(30):
        price = data.get_price(symbol, "1m", i)
        if price is None:
            return
        prices.append(price)
    
    # EMA 計算
    ema_fast = ind.ema(prices, params["fast_period"])
    ema_slow = ind.ema(prices, params["slow_period"])
    
    if ema_fast is None or ema_slow is None:
        return
    
    current_position = state.get("position")
    last_signal = state.get("last_signal")
    
    # シグナル判定
    if ema_fast > ema_slow and last_signal != "buy":
        if current_position == "flat":
            # 買いシグナル
            size = risk.calculate_position_size(params["position_size"])
            order_id = order.market(symbol, "buy", size)
            state.set("position", "long")
            state.set("last_signal", "buy")
            log.info(f"Buy signal: {symbol} at {data.get_price(symbol, '1m', 0)}")
    
    elif ema_fast < ema_slow and last_signal != "sell":
        if current_position == "long":
            # 売りシグナル
            position = risk.get_position(symbol)
            if position and position["quantity"] > 0:
                order_id = order.market(symbol, "sell", position["quantity"])
                state.set("position", "flat")
                state.set("last_signal", "sell")
                log.info(f"Sell signal: {symbol} at {data.get_price(symbol, '1m', 0)}")

def on_order_fill(order_id, symbol, side, quantity, price):
    log.info(f"Order filled: {side} {quantity} {symbol} at {price}")
```

### RSI リバーサル戦略

```python
def init():
    state.set("position", "flat")

def on_bar(symbol, timeframe, data):
    if timeframe != "5m":
        return
    
    # 価格データ取得
    prices = []
    for i in range(20):
        price = data.get_price(symbol, "5m", i)
        if price is None:
            return
        prices.append(price)
    
    # RSI 計算
    rsi = ind.rsi(prices, 14)
    if rsi is None:
        return
    
    current_position = state.get("position")
    
    # シグナル判定
    if rsi < 30 and current_position == "flat":
        # 買いシグナル（オーバーソールド）
        size = risk.calculate_position_size(params["position_size"])
        order_id = order.market(symbol, "buy", size)
        state.set("position", "long")
        log.info(f"RSI oversold buy signal: {symbol} RSI={rsi:.2f}")
    
    elif rsi > 70 and current_position == "long":
        # 売りシグナル（オーバーボート）
        position = risk.get_position(symbol)
        if position and position["quantity"] > 0:
            order_id = order.market(symbol, "sell", position["quantity"])
            state.set("position", "flat")
            log.info(f"RSI overbought sell signal: {symbol} RSI={rsi:.2f}")
```

### ブレイクアウト戦略

```python
def init():
    state.set("position", "flat")
    state.set("breakout_level", None)

def on_bar(symbol, timeframe, data):
    if timeframe != "15m":
        return
    
    # 価格データ取得
    highs = []
    lows = []
    closes = []
    
    for i in range(20):
        high = data.get_price(symbol, "15m", i, "high")
        low = data.get_price(symbol, "15m", i, "low")
        close = data.get_price(symbol, "15m", i)
        
        if high is None or low is None or close is None:
            return
        
        highs.append(high)
        lows.append(low)
        closes.append(close)
    
    current_position = state.get("position")
    breakout_level = state.get("breakout_level")
    current_price = closes[0]
    
    # ブレイクアウトレベル計算（過去20日間の最高値）
    if breakout_level is None:
        breakout_level = max(highs[1:])  # 最新を除く
        state.set("breakout_level", breakout_level)
    
    # ブレイクアウト判定
    if current_price > breakout_level and current_position == "flat":
        # 上向きブレイクアウト
        size = risk.calculate_position_size(params["position_size"])
        order_id = order.market(symbol, "buy", size)
        state.set("position", "long")
        log.info(f"Breakout buy signal: {symbol} at {current_price}")
    
    elif current_price < breakout_level * 0.95 and current_position == "long":
        # ストップロス（5%下）
        position = risk.get_position(symbol)
        if position and position["quantity"] > 0:
            order_id = order.market(symbol, "sell", position["quantity"])
            state.set("position", "flat")
            log.info(f"Stop loss: {symbol} at {current_price}")
```

## ベストプラクティス

### 1. エラーハンドリング

```python
def on_bar(symbol, timeframe, data):
    try:
        # 戦略ロジック
        pass
    except Exception as e:
        log.error(f"Error in strategy: {str(e)}")
        # エラー時の処理（ポジションクローズなど）
```

### 2. データ検証

```python
def on_bar(symbol, timeframe, data):
    # データの有効性チェック
    price = data.get_price(symbol, "1m", 0)
    if price is None or price <= 0:
        log.warn(f"Invalid price data for {symbol}")
        return
```

### 3. リスク管理

```python
def on_bar(symbol, timeframe, data):
    # ポジションサイズ制限
    max_position = risk.get_account_value() * 0.1  # 最大10%
    current_position = risk.get_position(symbol)
    
    if current_position and current_position["quantity"] > max_position:
        log.warn(f"Position size exceeds limit for {symbol}")
        return
```

### 4. パフォーマンス最適化

```python
def on_bar(symbol, timeframe, data):
    # 不要な計算を避ける
    if timeframe != "1m":
        return
    
    # キャッシュを活用
    cached_ema = state.get("cached_ema")
    if cached_ema is None:
        # 初回計算
        cached_ema = calculate_ema()
        state.set("cached_ema", cached_ema)
```

## デバッグとテスト

### ログ出力

```python
def on_bar(symbol, timeframe, data):
    # デバッグ情報
    log.info(f"Processing {symbol} at {timeframe}")
    log.info(f"Current price: {data.get_price(symbol, timeframe, 0)}")
    log.info(f"Position: {state.get('position')}")
```

### バックテスト

戦略は必ずバックテストで検証してからライブ運用に移行してください。

```bash
# バックテスト実行
curl -X POST http://localhost:8080/backtests \
  -H "Content-Type: application/json" \
  -d '{
    "strategy_template": "ema_cross",
    "symbol": "AAPL",
    "start_date": "2023-01-01",
    "end_date": "2023-12-31",
    "parameters": {
      "fast_period": 12,
      "slow_period": 26
    }
  }'
```

## 制限事項

1. **メモリ使用量**: 戦略は 100MB 以下のメモリ使用量に制限
2. **実行時間**: 1回の `on_bar` 処理は 100ms 以下
3. **API 呼び出し**: 1分間に最大 60回の注文発注
4. **データアクセス**: 過去 1000 バーまでのデータアクセス可能

## トラブルシューティング

### よくある問題

1. **データが取得できない**
   - 銘柄コードが正しいか確認
   - 時間軸が正しいか確認
   - データ期間が範囲内か確認

2. **注文が発注されない**
   - 口座残高が不足していないか確認
   - リスク制限に引っかかっていないか確認
   - 注文パラメータが正しいか確認

3. **戦略が動作しない**
   - ログを確認してエラーがないか確認
   - パラメータが正しく設定されているか確認
   - 初期化処理が正しく実行されているか確認