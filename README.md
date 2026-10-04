# Moomoo トレーディングシステム

Next.js、Go、MySQL を使用した高度なアルゴリズム取引プラットフォーム。

## アーキテクチャ

このプロジェクトは以下のモノレポ構成です：

- `apps/web` - Next.js フロントエンドアプリケーション
- `apps/api` - Go REST API サーバー
- `apps/bot` - 戦略実行用の Go ワーカー
- `packages/core` - api / bot 共通の Go モジュール（ブローカー interface、GMOコイン FX アダプタ、市場データ、リスク管理）。各 `go.mod` の `replace` で参照

## クイックスタート

### 前提条件

- Docker と Docker Compose
- Go 1.21+
- Node.js 18+
- MySQL 8.0
- Redis 7.0

### 開発環境セットアップ

#### 方法 1: Makefile を使用（推奨）

1. **環境構築（推奨）**

   ```bash
   # クイックセットアップ（依存関係インストール + Docker起動）
   make setup

   # 完全セットアップ（依存関係インストール + Docker起動 + アプリケーションビルド）
   make setup BUILD=1
   ```

2. **開発環境の起動**

   ```bash
   make dev
   ```

   これにより以下が自動的に起動します：

   - MySQL データベース
   - Redis
   - Web アプリケーション（http://localhost:3001）
   - API サーバー（http://localhost:8081）

3. **個別サービスの起動（必要に応じて）**

   ```bash
   # API サーバーのみ起動
   make api-dev

   # Web アプリケーションのみ起動
   make web-dev

   # Bot ワーカーのみ起動
   make bot-dev
   ```

#### 方法 2: 手動起動

1. **インフラストラクチャの起動**

   ```bash
   docker compose up -d
   ```

2. **API サーバーの起動**

   ```bash
   cd apps/api
   go run main.go
   ```

3. **Web アプリケーションの起動**

   ```bash
   cd apps/web
   npm run dev
   ```

4. **Bot ワーカーの起動**
   ```bash
   cd apps/bot
   go run main.go
   ```

### その他の便利なコマンド

```bash
# ヘルプ表示
make help

# 環境構築
make setup          # クイックセットアップ（依存関係インストール + Docker起動）
make setup BUILD=1  # 完全セットアップ（依存関係インストール + Docker起動 + ビルド）

# 依存関係のインストール
make install-deps

# 全アプリケーションのビルド
make build

# テスト実行
make test

# コードフォーマット
make format

# Docker サービスの停止
make docker-down

# Docker ログの表示
make docker-logs

# データベースリセット
make db-reset

# ビルド成果物のクリーンアップ
make clean
```

## 機能

### Phase 1: 基盤（完了）

- [x] モノレポ構成
- [x] Docker 開発環境
- [x] MySQL データベーススキーマ
- [x] 基本的な API エンドポイント
- [x] Next.js フロントエンドセットアップ
- [x] Go ワーカーフレームワーク

### Phase 2: コア機能（完了）

- [x] Moomoo ブローカーアダプター
- [x] Starlark を使用した戦略エンジン
- [x] リスク管理システム
- [x] バックテストエンジン

### Phase 3: 高度な機能（完了）

- [x] リアルタイム監視
- [x] ペーパートレーディングワークフロー
- [x] パフォーマンス分析
- [x] 通知システム

## API エンドポイント

- `GET /healthz` - ヘルスチェック
- `GET /api/v1/orders` - 注文一覧
- `POST /api/v1/orders` - 注文作成
- `GET /api/v1/strategies` - 戦略一覧
- `POST /api/v1/strategies` - 戦略作成
- `GET /api/v1/backtests` - バックテスト一覧
- `POST /api/v1/backtests` - バックテスト作成

## 環境変数

秘密情報はリポジトリに含めず、ルートの `.env` で管理します（git 管理外）。
`make setup` または `make env` で `.env.example` から `.env` を作成し、`NEXTAUTH_SECRET` を自動生成します。
DB パスワードなどは `.env` を編集して設定してください。

> 既存の MySQL ボリューム（旧設定のパスワード `moomoo123` で初期化済み）を使い続ける場合は、
> `.env` の `DB_PASSWORD` を旧パスワードに合わせるか、`make reload` でボリュームを作り直してください。

### データベース

- `DB_HOST` - MySQL ホスト（デフォルト: localhost）
- `DB_PORT` - MySQL ポート（デフォルト: 3306。ホストから compose の MySQL に繋ぐ場合は 3308）
- `DB_USER` - MySQL ユーザー（デフォルト: moomoo）
- `DB_PASSWORD` - MySQL パスワード（必須・デフォルトなし）
- `MYSQL_ROOT_PASSWORD` - MySQL root パスワード（docker compose 用・必須）
- `AUTO_MIGRATE` - `true` なら API 起動時に未適用のマイグレーションを適用（compose の api は true）
- `DB_NAME` - MySQL データベース（デフォルト: moomoo_trading）

### 認証（Web）

開発環境（`NODE_ENV=development`）では認証を省略します。本番（Vercel など）では次を設定します。

- `AUTH_SECRET` - セッションの暗号化キー（必須。`openssl rand -base64 32` で生成。旧名の `NEXTAUTH_SECRET` も可）
- `ADMIN_EMAIL` - 管理者のメールアドレス
- `ADMIN_PASSWORD_HASH` - パスワードの bcrypt ハッシュ
- `ADMIN_TOTP_SECRET` - 2FA（TOTP）のシークレット。設定するとログイン時に 6 桁コードが必須になる

`ADMIN_*` の 3 つは `cd apps/web && npm run auth:setup` で生成できます（メールアドレスとパスワードを入力すると、
ハッシュ・TOTP シークレット・認証アプリ登録用の otpauth URI を表示します。値はどこにも保存されません）。

#### Vercel への反映（`.env.prod`）

```bash
cd apps/web
vercel link                          # 初回のみ（Vercel プロジェクトに紐づける）
cp .env.prod.example .env.prod       # 値を記入（.env.prod は git 管理外）
npm run auth:setup                   # ADMIN_* を生成して .env.prod に貼り付け
npm run env:push                     # .env.prod の値を Vercel の Production に設定
```

- 値は標準入力で `vercel env add` に渡し、画面にもコマンド引数にも出しません。既存の変数は上書きし、Sensitive として保存します。
- `AUTH_SECRET` を空にしておくと初回に生成してファイルへ書き戻します（変えると全員ログアウト）。
- 反映には再デプロイが必要です（`vercel --prod`、または Vercel の画面で Redeploy）。

### Redis

- `REDIS_HOST` - Redis ホスト（デフォルト: localhost）
- `REDIS_PORT` - Redis ポート（デフォルト: 6379。ホストから compose の Redis に繋ぐ場合は 6380）
- `REDIS_PASSWORD` - Redis パスワード（デフォルト: 空）

### GMOコイン 外国為替FX（任意）

- `GMO_PUBLIC_URL` - Public REST API のベース URL（デフォルト: https://forex-api.coin.z.com/public）
- `GMO_PUBLIC_WS_URL` - Public WebSocket URL（デフォルト: wss://forex-api.coin.z.com/ws/public/v1）

## 開発

### データベースマイグレーション

`apps/api/internal/database/migrations` の SQL は API バイナリに埋め込まれ、適用済みのものは `schema_migrations` テーブルで管理されます。

- docker compose の api は起動時に自動適用します（`AUTO_MIGRATE=true`）。
- ホストから手動で適用する場合は `make migrate` を実行します。
- 新しいマイグレーションは `007_xxx.sql` のように連番で追加します（適用済みファイルは編集しない）。
- 旧方式（MySQL の初回起動時に適用）で作られた DB は、初回実行時に 001〜006 を適用済みとして記録します。

### 過去データの取得（backfill）

GMOコインの Public API（キー不要）から足データを取得し、`bars` テーブルに BID / ASK 別で保存します。
途中で止めても、再実行すると保存済みの最新の足から再開します。完了後に欠損（週末を除く）を表示します。

```bash
make backfill                                   # USD_JPY 1時間足を 2023-10-28 から現在まで
make backfill ARGS="-symbol EUR_JPY -interval 4h -from 2025-01-01"
```

- 取引日の区切りは 06:00 JST（GMO の klines の `date` 単位）。時刻はすべて UTC で保存します。
- リクエスト間隔は既定 1 秒（`-min-interval` で変更可）。1時間足の全期間で数十分かかります。

### バックテスト

保存済みの BID / ASK の足でストラテジーを検証します（先に backfill が必要）。

```bash
make backtest                                                         # ema_cross・既定パラメータ
make backtest ARGS="-param fast_period=8 -param slow_period=21"
make backtest ARGS="-sweep fast_period=8,12,20 -sweep slow_period=26,50,100 -from 2023-10-28 -to 2025-10-01"
make backtest ARGS="-json"                                            # 取引一覧・損益曲線を含む JSON
```

約定モデル（`packages/core/backtest`）:

- シグナルは確定足で計算し、**次の足の始値**で執行（未来のデータを使わない）
- 買いは ASK、売りは BID で約定（スプレッドを毎回負担）
- 損切りは BID の安値（買い）/ ASK の高値（売り）で判定し、窓開けで超えた場合は始値で約定
- 手数料は約定金額 × 0.002%（GMOコイン API 手数料）、証拠金はレバレッジ 25 倍で判定
- 建玉は1つまで、損益は円（JPY 建て通貨ペアのみ）

戦略は `packages/core/strategy` に Go で実装し、`Register` で登録します（パラメータの範囲・既定値も定義）。

### 新しい API エンドポイントの追加

1. `apps/api/internal/handlers/` にハンドラー関数を追加
2. `apps/api/main.go` でルートを登録
3. `apps/web/src/` に対応するフロントエンドコンポーネントを追加

### 戦略開発

戦略は Starlark で記述され、データベースに保存されます。Bot ワーカーは市場イベントに基づいて戦略を実行します。

## デプロイ

### モックモードでの動作確認

バックエンドをデプロイせずにアプリケーションを動作確認するには、モックモードを使用できます：

```bash
# 環境変数を設定
export NEXT_PUBLIC_USE_MOCK=true

# フロントエンドのみ起動
cd apps/web
npm run dev
```

### Vercel でのデプロイ

#### フロントエンド（Next.js）

```bash
# Vercel CLI のインストール
npm i -g vercel

# デプロイ
cd apps/web
vercel
```

#### バックエンド（Go API）

```bash
# API ディレクトリに移動
cd apps/api

# Vercel にデプロイ
vercel
```

### OCI での本格デプロイ

本格的な本番環境へのデプロイについては、詳細な手順書を参照してください：

📖 **[OCI デプロイメントガイド](./docs/oci-deployment-guide.md)**

このガイドには以下が含まれています：

- OCI プロジェクトの初期設定
- ネットワーク・セキュリティ設定
- データベース・Redis の設定
- アプリケーションのデプロイ
- ロードバランサーの設定
- 監視・ログ設定
- セキュリティ設定
- バックアップ・ディザスタリカバリ
- CI/CD パイプライン
- 運用監視
- トラブルシューティング
- コスト最適化
- セキュリティ監査
- パフォーマンスチューニング

## テスト

### 単体テスト

```bash
# Go テスト
cd apps/api
go test ./...

# Next.js テスト
cd apps/web
npm test
```

### 統合テスト

```bash
# Bruno を使用したAPIテスト
bruno test
```

### E2E テスト

```bash
# Playwright を使用したE2Eテスト
cd apps/web
npm run test:e2e
```

## 監視とログ

### ヘルスチェック

```bash
# API ヘルスチェック
curl http://localhost:8081/healthz

# 詳細なヘルスチェック
curl http://localhost:8081/healthz/detailed
```

### ログの確認

```bash
# Docker ログ
docker-compose logs -f api
docker-compose logs -f web
docker-compose logs -f bot

# アプリケーションログ
tail -f logs/app.log
```

## セキュリティ

### 認証

- NextAuth.js を使用した認証
- 2FA（TOTP）必須設定
- API キー管理（AES-GCM + OCI Vault）

### 監査

- トレード ID 鎖状トレース
- 完全な監査ログ
- 可視化ツール

## パフォーマンス

### 最適化

- Redis Streams を使用したイベント処理
- データベース接続プール
- キャッシュ戦略
- CDN 設定

### 監視

- リアルタイムメトリクス
- アラート設定
- パフォーマンスダッシュボード

## トラブルシューティング

### よくある問題

1. **データベース接続エラー**

   ```bash
   # MySQL の状態確認
   docker-compose ps mysql
   docker-compose logs mysql
   ```

2. **Redis 接続エラー**

   ```bash
   # Redis の状態確認
   docker-compose ps redis
   docker-compose logs redis
   ```

3. **API 接続エラー**
   ```bash
   # API の状態確認
   curl http://localhost:8081/healthz
   ```

### ログの確認

```bash
# アプリケーションログ
docker-compose logs -f

# 特定のサービスのログ
docker-compose logs -f api
docker-compose logs -f web
docker-compose logs -f bot
```

## 貢献

1. このリポジトリをフォーク
2. 機能ブランチを作成 (`git checkout -b feature/amazing-feature`)
3. 変更をコミット (`git commit -m 'Add some amazing feature'`)
4. ブランチにプッシュ (`git push origin feature/amazing-feature`)
5. プルリクエストを作成

## ライセンス

MIT License

## サポート

問題や質問がある場合は、以下をご確認ください：

- [GitHub Issues](https://github.com/your-repo/issues)
- [ドキュメント](./docs/)
- [OCI デプロイメントガイド](./docs/oci-deployment-guide.md)

## 更新履歴

### v1.0.0 (2024-01-15)

- 初期リリース
- 基本的な取引機能
- 戦略エンジン
- バックテスト機能
- Web UI
- モニタリング機能
