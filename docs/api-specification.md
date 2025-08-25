# API 仕様書

## 概要

moomoo 自動売買システムの REST API 仕様書です。

## 認証

### API キー認証

すべての API リクエストには API キーが必要です。

```http
Authorization: Bearer <api_key>
```

### 2FA 認証

セキュリティ上重要な操作には 2FA 認証が必要です。

```http
X-2FA-Token: <totp_token>
```

## エンドポイント

### ヘルスチェック

#### GET /healthz

システムの健全性を確認します。

**レスポンス:**
```json
{
  "status": "healthy",
  "timestamp": "2024-01-01T00:00:00Z",
  "components": {
    "database": "healthy",
    "redis": "healthy",
    "moomoo": "healthy",
    "strategy_engine": "healthy"
  },
  "metrics": {
    "lag_ms": 150,
    "queue_size": 0,
    "reconnect_count": 2,
    "vm_quota_hits": 0
  }
}
```

### 戦略管理

#### GET /strategies

戦略一覧を取得します。

**クエリパラメータ:**
- `status`: 戦略ステータス（active, paused, stopped）
- `page`: ページ番号（デフォルト: 1）
- `limit`: 1ページあたりの件数（デフォルト: 20）

**レスポンス:**
```json
{
  "strategies": [
    {
      "id": "strategy_001",
      "name": "EMA Cross Strategy",
      "status": "active",
      "symbol": "AAPL",
      "created_at": "2024-01-01T00:00:00Z",
      "updated_at": "2024-01-01T00:00:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 20,
    "total": 1
  }
}
```

#### POST /strategies

新しい戦略を作成します。

**リクエストボディ:**
```json
{
  "name": "EMA Cross Strategy",
  "template": "ema_cross",
  "symbol": "AAPL",
  "parameters": {
    "fast_period": 12,
    "slow_period": 26,
    "position_size": 0.25
  },
  "risk_settings": {
    "max_daily_drawdown": 0.01,
    "max_weekly_drawdown": 0.03
  }
}
```

#### PUT /strategies/{id}

戦略を更新します。

#### DELETE /strategies/{id}

戦略を削除します。

### 注文管理

#### GET /orders

注文一覧を取得します。

**クエリパラメータ:**
- `strategy_id`: 戦略ID
- `status`: 注文ステータス（pending, filled, cancelled）
- `symbol`: 銘柄コード
- `start_date`: 開始日
- `end_date`: 終了日

**レスポンス:**
```json
{
  "orders": [
    {
      "id": "order_001",
      "strategy_id": "strategy_001",
      "symbol": "AAPL",
      "side": "buy",
      "type": "market",
      "quantity": 100,
      "price": 150.00,
      "status": "filled",
      "filled_at": "2024-01-01T00:00:00Z",
      "created_at": "2024-01-01T00:00:00Z"
    }
  ]
}
```

#### POST /orders

新しい注文を作成します。

**リクエストボディ:**
```json
{
  "strategy_id": "strategy_001",
  "symbol": "AAPL",
  "side": "buy",
  "type": "market",
  "quantity": 100,
  "price": 150.00
}
```

### バックテスト

#### POST /backtests

バックテストを実行します。

**リクエストボディ:**
```json
{
  "strategy_template": "ema_cross",
  "symbol": "AAPL",
  "start_date": "2023-01-01",
  "end_date": "2023-12-31",
  "parameters": {
    "fast_period": 12,
    "slow_period": 26
  },
  "initial_capital": 100000
}
```

**レスポンス:**
```json
{
  "backtest_id": "bt_001",
  "status": "running",
  "progress": 0.5
}
```

#### GET /backtests/{id}

バックテスト結果を取得します。

**レスポンス:**
```json
{
  "id": "bt_001",
  "status": "completed",
  "results": {
    "total_return": 0.15,
    "sharpe_ratio": 1.2,
    "max_drawdown": 0.05,
    "win_rate": 0.65,
    "profit_factor": 1.8,
    "sqn": 2.1,
    "cagr": 0.12
  },
  "trades": [
    {
      "entry_date": "2023-01-15",
      "exit_date": "2023-01-20",
      "side": "buy",
      "quantity": 100,
      "entry_price": 150.00,
      "exit_price": 155.00,
      "pnl": 500.00
    }
  ]
}
```

### エクスポート

#### GET /exports/tax/{year}

税務用データをエクスポートします。

**クエリパラメータ:**
- `format`: 出力形式（tws, japan）
- `strategy_id`: 戦略ID（オプション）

**レスポンス:**
```json
{
  "download_url": "/downloads/tax_2023_tws.csv",
  "expires_at": "2024-01-01T23:59:59Z"
}
```

## エラーレスポンス

### 400 Bad Request
```json
{
  "error": "validation_error",
  "message": "Invalid parameters",
  "details": {
    "field": "symbol",
    "issue": "Symbol is required"
  }
}
```

### 401 Unauthorized
```json
{
  "error": "unauthorized",
  "message": "Invalid API key"
}
```

### 403 Forbidden
```json
{
  "error": "forbidden",
  "message": "2FA token required"
}
```

### 404 Not Found
```json
{
  "error": "not_found",
  "message": "Strategy not found"
}
```

### 500 Internal Server Error
```json
{
  "error": "internal_error",
  "message": "Internal server error",
  "request_id": "req_001"
}
```

## レート制限

- 通常の API リクエスト: 1000 requests/hour
- バックテスト実行: 10 requests/hour
- データエクスポート: 5 requests/hour

## WebSocket API

### リアルタイムデータ

#### 接続
```
ws://localhost:8080/ws/stream
```

#### 購読
```json
{
  "action": "subscribe",
  "channel": "trades",
  "strategy_id": "strategy_001"
}
```

#### データ形式
```json
{
  "channel": "trades",
  "data": {
    "strategy_id": "strategy_001",
    "symbol": "AAPL",
    "side": "buy",
    "quantity": 100,
    "price": 150.00,
    "timestamp": "2024-01-01T00:00:00Z"
  }
}
```