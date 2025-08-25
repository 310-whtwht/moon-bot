# APIエンドポイント仕様

## 1. 共通仕様

### 1.1 基本情報
- **Base URL**: `https://api.trading-system.com/v1`
- **Content-Type**: `application/json`
- **認証**: Bearer Token（JWT）
- **文字エンコーディング**: UTF-8

### 1.2 共通レスポンス形式
```json
{
  "success": true,
  "data": {},
  "error": null,
  "request_id": "req_1234567890abcdef",
  "timestamp": "2024-01-01T00:00:00Z"
}
```

### 1.3 エラーレスポンス形式
```json
{
  "success": false,
  "data": null,
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "パラメータが不正です",
    "details": {
      "field": "symbol",
      "reason": "銘柄コードは必須です"
    }
  },
  "request_id": "req_1234567890abcdef",
  "timestamp": "2024-01-01T00:00:00Z"
}
```

### 1.4 HTTPステータスコード
- `200`: 成功
- `201`: 作成成功
- `400`: バリデーションエラー
- `401`: 認証エラー
- `403`: 認可エラー
- `404`: リソース未発見
- `409`: 競合状態
- `422`: 処理不可能
- `429`: レート制限
- `500`: サーバーエラー

## 2. 認証・認可

### 2.1 ログイン
```http
POST /auth/login
```

**リクエスト**
```json
{
  "email": "user@example.com",
  "password": "password123",
  "totp_code": "123456"
}
```

**レスポンス**
```json
{
  "success": true,
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "expires_in": 3600,
    "user": {
      "id": 1,
      "email": "user@example.com",
      "name": "User Name",
      "two_factor_enabled": true
    }
  }
}
```

### 2.2 トークン更新
```http
POST /auth/refresh
```

**リクエスト**
```json
{
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

### 2.3 ログアウト
```http
POST /auth/logout
```

## 3. 戦略管理

### 3.1 戦略一覧取得
```http
GET /strategies
```

**クエリパラメータ**
- `category`: 戦略カテゴリ（trend_following, mean_reversion, breakout, oscillator）
- `is_active`: アクティブ状態（true/false）
- `page`: ページ番号（デフォルト: 1）
- `limit`: 取得件数（デフォルト: 20、最大: 100）

**レスポンス**
```json
{
  "success": true,
  "data": {
    "strategies": [
      {
        "id": 1,
        "name": "EMA Cross Strategy",
        "description": "EMAクロス戦略",
        "category": "trend_following",
        "is_active": true,
        "latest_version": "1.0.0",
        "created_at": "2024-01-01T00:00:00Z",
        "updated_at": "2024-01-01T00:00:00Z"
      }
    ],
    "pagination": {
      "page": 1,
      "limit": 20,
      "total": 100,
      "total_pages": 5
    }
  }
}
```

### 3.2 戦略詳細取得
```http
GET /strategies/{id}
```

**レスポンス**
```json
{
  "success": true,
  "data": {
    "id": 1,
    "name": "EMA Cross Strategy",
    "description": "EMAクロス戦略",
    "category": "trend_following",
    "is_active": true,
    "versions": [
      {
        "id": 1,
        "version": "1.0.0",
        "is_active": true,
        "created_at": "2024-01-01T00:00:00Z"
      }
    ],
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z"
  }
}
```

### 3.3 戦略作成
```http
POST /strategies
```

**リクエスト**
```json
{
  "name": "New Strategy",
  "description": "新しい戦略",
  "category": "trend_following",
  "script_content": "def on_bar(context, data):\n    # 戦略ロジック\n    pass",
  "schema_definition": {
    "type": "object",
    "properties": {
      "fast_period": {
        "type": "number",
        "default": 9,
        "minimum": 1,
        "maximum": 100
      },
      "slow_period": {
        "type": "number",
        "default": 21,
        "minimum": 1,
        "maximum": 200
      }
    }
  }
}
```

### 3.4 戦略更新
```http
PUT /strategies/{id}
```

### 3.5 戦略削除
```http
DELETE /strategies/{id}
```

## 4. バックテスト

### 4.1 バックテスト実行
```http
POST /backtests
```

**リクエスト**
```json
{
  "strategy_version_id": 1,
  "symbols": ["AAPL", "GOOGL"],
  "start_date": "2024-01-01",
  "end_date": "2024-01-31",
  "initial_capital": 100000,
  "parameters": {
    "fast_period": 9,
    "slow_period": 21,
    "risk_per_trade": 0.0025
  },
  "slippage_model": {
    "type": "fixed",
    "value": 0.001
  }
}
```

**レスポンス**
```json
{
  "success": true,
  "data": {
    "backtest_id": "bt_1234567890abcdef",
    "status": "running",
    "progress": 0,
    "estimated_completion": "2024-01-01T00:05:00Z"
  }
}
```

### 4.2 バックテスト結果取得
```http
GET /backtests/{id}
```

**レスポンス**
```json
{
  "success": true,
  "data": {
    "id": "bt_1234567890abcdef",
    "status": "completed",
    "strategy_version_id": 1,
    "symbols": ["AAPL", "GOOGL"],
    "start_date": "2024-01-01",
    "end_date": "2024-01-31",
    "initial_capital": 100000,
    "final_capital": 105000,
    "total_return": 0.05,
    "sharpe_ratio": 1.2,
    "max_drawdown": 0.03,
    "win_rate": 0.65,
    "total_trades": 100,
    "profit_factor": 1.5,
    "equity_curve": [
      {
        "date": "2024-01-01",
        "equity": 100000,
        "drawdown": 0
      }
    ],
    "monthly_returns": [
      {
        "month": "2024-01",
        "return": 0.05
      }
    ],
    "trades": [
      {
        "trade_id": "trade_123",
        "symbol": "AAPL",
        "side": "long",
        "entry_date": "2024-01-01",
        "exit_date": "2024-01-02",
        "entry_price": 150.0,
        "exit_price": 155.0,
        "quantity": 100,
        "pnl": 500.0
      }
    ],
    "created_at": "2024-01-01T00:00:00Z",
    "completed_at": "2024-01-01T00:05:00Z"
  }
}
```

## 5. 注文管理

### 5.1 注文作成
```http
POST /orders
```

**リクエスト**
```json
{
  "client_order_id": "order_1234567890abcdef",
  "strategy_version_id": 1,
  "symbol": "AAPL",
  "side": "buy",
  "order_type": "market",
  "quantity": 100,
  "time_in_force": "day"
}
```

**レスポンス**
```json
{
  "success": true,
  "data": {
    "id": 1,
    "client_order_id": "order_1234567890abcdef",
    "broker_order_id": "broker_123",
    "strategy_version_id": 1,
    "symbol": "AAPL",
    "side": "buy",
    "order_type": "market",
    "quantity": 100,
    "status": "submitted",
    "filled_quantity": 0,
    "avg_fill_price": null,
    "commission": 0,
    "created_at": "2024-01-01T00:00:00Z"
  }
}
```

### 5.2 注文一覧取得
```http
GET /orders
```

**クエリパラメータ**
- `symbol`: 銘柄コード
- `status`: 注文状態
- `strategy_version_id`: 戦略バージョンID
- `start_date`: 開始日
- `end_date`: 終了日
- `page`: ページ番号
- `limit`: 取得件数

### 5.3 注文詳細取得
```http
GET /orders/{id}
```

### 5.4 注文キャンセル
```http
DELETE /orders/{id}
```

## 6. 取引履歴

### 6.1 取引一覧取得
```http
GET /trades
```

**クエリパラメータ**
- `symbol`: 銘柄コード
- `strategy_version_id`: 戦略バージョンID
- `start_date`: 開始日
- `end_date`: 終了日
- `page`: ページ番号
- `limit`: 取得件数

**レスポンス**
```json
{
  "success": true,
  "data": {
    "trades": [
      {
        "id": 1,
        "trade_id": "trade_1234567890abcdef",
        "strategy_version_id": 1,
        "symbol": "AAPL",
        "side": "long",
        "quantity": 100,
        "open_price": 150.0,
        "close_price": 155.0,
        "realized_pnl": 500.0,
        "commission": 5.0,
        "opened_at": "2024-01-01T00:00:00Z",
        "closed_at": "2024-01-02T00:00:00Z"
      }
    ],
    "pagination": {
      "page": 1,
      "limit": 20,
      "total": 100,
      "total_pages": 5
    }
  }
}
```

### 6.2 取引詳細取得
```http
GET /trades/{id}
```

## 7. 市場データ

### 7.1 市場データ取得
```http
GET /market-data/{symbol}
```

**クエリパラメータ**
- `timeframe`: 時間足（1m, 5m, 15m, 1d）
- `start_date`: 開始日時
- `end_date`: 終了日時
- `limit`: 取得件数（最大: 1000）

**レスポンス**
```json
{
  "success": true,
  "data": {
    "symbol": "AAPL",
    "timeframe": "1d",
    "data": [
      {
        "ts_utc": "2024-01-01T00:00:00Z",
        "open": 150.0,
        "high": 155.0,
        "low": 149.0,
        "close": 154.0,
        "volume": 1000000,
        "adjusted_close": 154.0
      }
    ]
  }
}
```

### 7.2 市場データ同期
```http
POST /market-data/sync
```

**リクエスト**
```json
{
  "symbols": ["AAPL", "GOOGL"],
  "timeframes": ["1m", "5m", "1d"],
  "start_date": "2024-01-01",
  "end_date": "2024-01-31"
}
```

## 8. 監査ログ

### 8.1 監査ログ取得
```http
GET /audit-logs
```

**クエリパラメータ**
- `trade_id`: 取引ID
- `stage`: ステージ（input, signal, order, fill, close）
- `event_type`: イベントタイプ
- `start_date`: 開始日時
- `end_date`: 終了日時
- `page`: ページ番号
- `limit`: 取得件数

**レスポンス**
```json
{
  "success": true,
  "data": {
    "logs": [
      {
        "id": 1,
        "trade_id": "trade_1234567890abcdef",
        "stage": "signal",
        "event_type": "ema_cross",
        "event_data": {
          "fast_ema": 150.5,
          "slow_ema": 149.8,
          "signal": "buy"
        },
        "ts_utc": "2024-01-01T00:00:00Z"
      }
    ],
    "pagination": {
      "page": 1,
      "limit": 20,
      "total": 100,
      "total_pages": 5
    }
  }
}
```

## 9. システム管理

### 9.1 ヘルスチェック
```http
GET /healthz
```

**レスポンス**
```json
{
  "success": true,
  "data": {
    "status": "healthy",
    "opend": {
      "state": "connected",
      "subscriptions": 50
    },
    "db": {
      "pool": "healthy",
      "latency": 5
    },
    "redis": {
      "ping": "pong",
      "lag": 0
    },
    "streams": {
      "pending": 0,
      "maxlen": 1000
    },
    "goroutines": 150,
    "build": {
      "version": "1.0.0",
      "commit": "abc123",
      "build_time": "2024-01-01T00:00:00Z"
    }
  }
}
```

### 9.2 システム設定取得
```http
GET /system/configs
```

### 9.3 システム設定更新
```http
PUT /system/configs/{key}
```

## 10. エクスポート

### 10.1 PnLエクスポート
```http
POST /exports/pnl
```

**リクエスト**
```json
{
  "format": "tws", // tws または japan
  "start_date": "2024-01-01",
  "end_date": "2024-12-31",
  "strategy_version_ids": [1, 2]
}
```

**レスポンス**
```json
{
  "success": true,
  "data": {
    "export_id": "exp_1234567890abcdef",
    "status": "processing",
    "download_url": null
  }
}
```

### 10.2 エクスポート結果取得
```http
GET /exports/{id}
```

**レスポンス**
```json
{
  "success": true,
  "data": {
    "id": "exp_1234567890abcdef",
    "status": "completed",
    "format": "tws",
    "download_url": "https://api.trading-system.com/v1/exports/exp_1234567890abcdef/download",
    "file_size": 1024,
    "created_at": "2024-01-01T00:00:00Z",
    "completed_at": "2024-01-01T00:01:00Z"
  }
}
```

## 11. レート制限

### 11.1 制限値
- **認証エンドポイント**: 10回/分
- **戦略管理**: 100回/分
- **バックテスト**: 10回/分
- **注文管理**: 1000回/分
- **市場データ**: 1000回/分
- **監査ログ**: 100回/分
- **エクスポート**: 10回/分

### 11.2 レート制限ヘッダー
```
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 999
X-RateLimit-Reset: 1640995200
```

## 12. WebSocket API

### 12.1 接続
```
wss://api.trading-system.com/v1/ws
```

**認証**
```json
{
  "type": "auth",
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

### 12.2 購読
```json
{
  "type": "subscribe",
  "channels": ["orders", "trades", "market_data"]
}
```

### 12.3 イベント形式
```json
{
  "type": "order_update",
  "data": {
    "order_id": 1,
    "status": "filled",
    "filled_quantity": 100,
    "avg_fill_price": 150.0
  },
  "timestamp": "2024-01-01T00:00:00Z"
}
```