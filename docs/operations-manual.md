# 運用マニュアル

## 概要

moomoo 自動売買システムの運用に関する手順と注意事項を説明します。

## システム構成

### コンポーネント

- **Web UI** (`apps/web`): Next.js ベースの管理画面
- **API Server** (`apps/api`): Go + gin ベースの REST API
- **Bot Worker** (`apps/bot`): 戦略実行エンジン
- **Database**: MySQL 8.0
- **Cache**: Redis 7.0
- **Broker**: moomoo (Futu OpenD)

### ネットワーク構成

```
Internet
    ↓
Load Balancer (Nginx)
    ↓
Web UI (Next.js) ←→ API Server (Go)
    ↓
Database (MySQL) ←→ Cache (Redis)
    ↓
Bot Worker (Go) ←→ moomoo API
```

## 起動・停止手順

### 開発環境

#### 1. 環境準備

```bash
# 依存関係インストール
make install-deps

# 環境変数設定
cp .env.example .env
# .env ファイルを編集して必要な設定を行う
```

#### 2. データベース起動

```bash
# Docker Compose で MySQL と Redis を起動
docker-compose up -d

# データベースマイグレーション
make migrate
```

#### 3. アプリケーション起動

```bash
# 全サービス起動
make start

# 個別起動
make start-api    # API Server
make start-web    # Web UI
make start-bot    # Bot Worker
```

#### 4. 停止

```bash
# 全サービス停止
make stop

# 個別停止
make stop-api
make stop-web
make stop-bot

# データベース停止
docker-compose down
```

### 本番環境

#### 1. デプロイ

```bash
# ビルド
make build

# デプロイ
make deploy
```

#### 2. 起動確認

```bash
# ヘルスチェック
curl http://localhost:8080/healthz

# ログ確認
make logs
```

## 監視・ログ

### ヘルスチェック

#### エンドポイント

- `GET /healthz`: システム全体の健全性
- `GET /healthz/detailed`: 詳細なコンポーネント状態

#### 監視項目

```json
{
  "status": "healthy",
  "components": {
    "database": "healthy",
    "redis": "healthy", 
    "moomoo": "healthy",
    "strategy_engine": "healthy"
  },
  "metrics": {
    "lag_ms": 150,           // データ遅延
    "queue_size": 0,         // キュー滞留量
    "reconnect_count": 2,    // 再接続回数
    "vm_quota_hits": 0       // VM クォータヒット数
  }
}
```

### ログ管理

#### ログレベル

- `ERROR`: システムエラー、注文失敗
- `WARN`: 警告、リスク制限
- `INFO`: 通常の操作ログ
- `DEBUG`: デバッグ情報

#### ログファイル

```
logs/
├── api.log          # API Server ログ
├── bot.log          # Bot Worker ログ
├── web.log          # Web UI ログ
├── moomoo.log       # moomoo 接続ログ
└── strategy/        # 戦略別ログ
    ├── strategy_001.log
    └── strategy_002.log
```

#### ログローテーション

```bash
# ログローテーション（日次）
make rotate-logs

# ログアーカイブ（月次）
make archive-logs
```

### アラート設定

#### Slack 通知

```yaml
# config/notifications.yaml
slack:
  webhook_url: "https://hooks.slack.com/..."
  channels:
    alerts: "#trading-alerts"
    daily: "#trading-daily"
    errors: "#trading-errors"
```

#### 通知条件

- **サーキットイベント**: 即座に通知
- **日次集計**: 毎日 18:00 に通知
- **エラー**: 即座に通知（重要度に応じて）

## 戦略管理

### 戦略ライフサイクル

#### 1. 開発・テスト

```bash
# バックテスト実行
curl -X POST http://localhost:8080/backtests \
  -H "Content-Type: application/json" \
  -d '{
    "strategy_template": "ema_cross",
    "symbol": "AAPL",
    "start_date": "2023-01-01",
    "end_date": "2023-12-31"
  }'
```

#### 2. Paper Trading

```bash
# Paper Trading 開始
curl -X POST http://localhost:8080/strategies \
  -H "Content-Type: application/json" \
  -d '{
    "name": "EMA Cross Paper",
    "template": "ema_cross",
    "symbol": "AAPL",
    "mode": "paper"
  }'
```

#### 3. Live Trading

```bash
# Live Trading 開始（確認ダイアログ付き）
curl -X POST http://localhost:8080/strategies/{id}/activate \
  -H "X-2FA-Token: <totp_token>"
```

### 戦略監視

#### リアルタイム監視

```bash
# 戦略状態確認
curl http://localhost:8080/strategies

# 特定戦略の詳細
curl http://localhost:8080/strategies/{id}
```

#### パフォーマンス監視

- **PnL**: リアルタイム損益
- **MaxDD**: 最大ドローダウン
- **勝率**: 取引勝率
- **SQN**: システム品質数

### 緊急停止

#### Kill Switch

```bash
# 全戦略停止
curl -X POST http://localhost:8080/strategies/kill \
  -H "X-2FA-Token: <totp_token>"

# 特定戦略停止
curl -X POST http://localhost:8080/strategies/{id}/stop \
  -H "X-2FA-Token: <totp_token>"
```

#### ポジションクローズ

```bash
# 全ポジションクローズ
curl -X POST http://localhost:8080/positions/close-all \
  -H "X-2FA-Token: <totp_token>"
```

## データ管理

### ヒストリカルデータ

#### データ同期

```bash
# 日次データ同期
make sync-data

# 特定銘柄のデータ同期
make sync-data SYMBOL=AAPL
```

#### データ検証

```bash
# データ整合性チェック
make verify-data

# 欠損データ補完
make fix-data
```

### バックアップ

#### データベースバックアップ

```bash
# 日次バックアップ
make backup-db

# 手動バックアップ
make backup-db MANUAL=true
```

#### 設定バックアップ

```bash
# 設定ファイルバックアップ
make backup-config

# 戦略設定バックアップ
make backup-strategies
```

## トラブルシューティング

### よくある問題

#### 1. moomoo 接続エラー

**症状**: データが取得できない、注文が発注されない

**確認項目**:
- Futu OpenD が起動しているか
- 認証情報が正しいか
- ネットワーク接続が正常か

**対処法**:
```bash
# 接続状態確認
curl http://localhost:8080/healthz

# ログ確認
tail -f logs/moomoo.log

# 再接続
curl -X POST http://localhost:8080/moomoo/reconnect
```

#### 2. 戦略エラー

**症状**: 戦略が動作しない、エラーログが出力される

**確認項目**:
- 戦略コードに構文エラーがないか
- パラメータが正しく設定されているか
- データが正常に取得できているか

**対処法**:
```bash
# 戦略ログ確認
tail -f logs/strategy/{strategy_id}.log

# 戦略再起動
curl -X POST http://localhost:8080/strategies/{id}/restart
```

#### 3. データベースエラー

**症状**: API エラー、データが保存されない

**確認項目**:
- MySQL が起動しているか
- ディスク容量が不足していないか
- 接続数制限に達していないか

**対処法**:
```bash
# データベース状態確認
docker-compose exec mysql mysql -u root -p

# ログ確認
docker-compose logs mysql
```

### 緊急時対応

#### システムダウン時

1. **即座の対応**
   - 全戦略停止
   - ポジション確認
   - ログ収集

2. **原因調査**
   - ログ分析
   - システム状態確認
   - 外部要因確認

3. **復旧手順**
   - システム再起動
   - データ整合性確認
   - 戦略再開（段階的）

#### データ不整合時

1. **影響範囲確認**
   - 不整合データの特定
   - 影響を受ける戦略の特定

2. **データ修復**
   - 欠損データの補完
   - 重複データの削除

3. **検証**
   - データ整合性チェック
   - 戦略動作確認

## 定期メンテナンス

### 日次作業

- ログ確認・ローテーション
- パフォーマンス監視
- バックアップ実行

### 週次作業

- データベース最適化
- ディスク容量確認
- セキュリティアップデート確認

### 月次作業

- システムアップデート
- パフォーマンス分析
- 設定見直し

## セキュリティ

### アクセス制御

- 2FA 必須設定
- API キーの定期的な更新
- IP アドレス制限

### 監査ログ

- 全操作のログ記録
- トレード ID 鎖状トレース
- 定期的な監査

### データ保護

- API キーの暗号化
- 通信の暗号化（TLS）
- バックアップの暗号化