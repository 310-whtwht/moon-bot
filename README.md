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

### 自動売買（Paper）

bot は `deployments` テーブルの割り当て（どの戦略を、どの口座・銘柄・時間足・数量で動かすか）に従って売買します。
Phase 3 時点のブローカーは **paper**（模擬約定）だけで、GMOコインの実際のレートに対して約定させます。

- 足が確定するたびに、戦略の**有効なバージョン**でシグナルを計算し、リスク確認を通ったものだけ成行で発注します
- 起動直後やバージョン切り替え直後は、過去の足で指標を温めるだけで発注しません（売買は次の足から）
- 確定から時間が経ったシグナル（既定 5 分超）は実行しません
- 損切りはレート（WebSocket、加えて定期確認ごとの REST）で監視し、到達したら成行で決済します
- 建玉を持っている間は、その建玉を建てたバージョンのルールで決済します。新しいバージョンは建玉が無くなってから使います
- 同じ判断を二重に発注しないよう、注文には割り当て・足・動作から決まる `client_order_id` を付けます
- 決済が失敗・見送りになった場合は、次の確認で再試行します
- 割り当てを無効にしても建玉は自動では閉じません（決済と損切りは続け、新規だけ止めます）

サンプルの割り当て（EMA クロス / USD_JPY / 1時間足 / 100通貨）は無効で入っています。有効にするには:

```sql
UPDATE deployments SET enabled = TRUE WHERE id = 'dddddddd-dddd-dddd-dddd-dddddddddddd';
```

設定（bot の環境変数）:

| 変数 | 既定 | 内容 |
|---|---|---|
| `TRADER_ENABLED` | `true` | `false` で自動売買を止める（バックテストのジョブは動く） |
| `TRADER_POLL_INTERVAL` | `30s` | 足の確定と損切りを確認する間隔 |
| `PAPER_INITIAL_BALANCE` | `30000` | Paper 口座の初期資金（円） |
| `RISK_ACCOUNT_MAX_UNITS` | `1000` | 1建玉の最大数量 |
| `RISK_ACCOUNT_MAX_POSITIONS` | `1` | 口座あたりの同時建玉数 |
| `RISK_ACCOUNT_MAX_DAILY_LOSS` / `_WEEKLY_LOSS` | `500` / `1500` | 口座の日次・週次の損失上限（円） |
| `RISK_GLOBAL_MAX_POSITIONS` | `3` | 全口座合計の同時建玉数 |
| `RISK_GLOBAL_MAX_DAILY_LOSS` / `_WEEKLY_LOSS` | `1000` / `3000` | 全体の日次・週次の損失上限（円） |

損失の集計は取引日（06:00 JST 区切り）・取引週（月曜 06:00 JST 始まり）単位です。発注には利用可能な証拠金の 50% までしか使いません。

### 本番ブローカー（GMOコイン Private API）

既定では Paper だけが動きます。GMOコインの口座に実際に発注するには、bot に次の **すべて** を設定します（1つでも欠けると Paper のみ）。

| 変数 | 値 |
|---|---|
| `BROKER_MODE` | `live` |
| `LIVE_CONFIRM` | `yes` |
| `GMO_API_KEY` / `GMO_API_SECRET` | 会員ページで発行した API キー（権限は取引と照会のみ、IP 制限を有効に） |
| `GMO_ACCOUNT_ID` | 注文・建玉に付ける口座のラベル（任意、既定 `default`） |

- 起動時に口座残高を読めることを確認し、読めなければ起動を止めます
- 割り当て（`deployments`）の `broker` が `gmo` のものだけが本番に発注します。`paper` の割り当ては引き続き模擬約定です
- 1注文の数量はコード上の上限 `HardMaxLiveUnits`（10,000）を超えられません。加えて `RISK_*` の上限が効きます
- 発注の応答が失われた場合（タイムアウトなど）は**再送せず**、`clientOrderId` で約定を照合します。確定できないときは、そのブローカーの Kill Switch を自動で入れて通知します（解除は人が状態を確認してから）
- `clientOrderId` は GMO の制約（英数字のみ・36文字以内）に合わせて変換して送ります

**約定の即時検知（Private WebSocket）**: 本番ブローカー接続時は GMOコインの約定通知（`executionEvents`）を購読し、約定が届いたらその銘柄の建玉をすぐ照合します。ブローカー側で逆指値が執行された場合も、定期照合を待たずに精算されます。トークンは 30 分ごとに延長し、切断時は新しいトークンで再接続、終了時に削除します。通知は取りこぼしうるため、定期照合はそのまま続けます。

**損切り（ブローカー側の逆指値）**: 建玉を建てた直後に、損切り価格の逆指値（STOP の決済注文）を GMOコインに置きます。bot が止まっていても GMOコイン側で執行されます。

- 逆指値の価格は銘柄の呼値（USD_JPY は 0.001）に丸めます
- bot が自分で決済するとき（シグナル・Kill Switch）は、先に逆指値を取り消してから成行で決済します
- 逆指値を置けなかった場合は通知し、bot 側のレート監視だけで損切りします（Paper は常にこの方式）
- 逆指値が失効・取消されていたら、次の照合で置き直します

**建玉の照合**: 起動時と `TRADER_RECONCILE_INTERVAL`（既定 5 分）ごと、およびレートが逆指値を越えたときに、DB の建玉とブローカーの建玉を突き合わせます。

| 状況 | 動き |
|---|---|
| ブローカー側で建玉が決済済み（逆指値の執行・ロスカット・手動決済）で、決済の約定が見つかる | 約定価格で DB の建玉を閉じ、損益を記録して通知 |
| ブローカーに bot の知らない建玉がある／数量が違う／建玉が消えたのに決済の約定が見つからない | そのブローカーの Kill Switch を入れて通知（同じ内容は繰り返さない）。人がブローカーの画面で確認してから解除する |

> **注意**: 実際の GMOコイン Private API での動作確認はまだ行っていません（モックでの検証のみ）。本番に出す前に、最小数量で発注・逆指値・決済・照合を一通り確かめてください。約定の照合は直近1日・最大100件の約定一覧（`latestExecutions`）に依存します。

### Kill Switch・通知・ダッシュボード

**Kill Switch** は新規の発注だけを止めます（決済と損切りは止めません）。範囲は「全体（global）」か、ブローカー単位（`paper` / `gmo`）です。

| 止め方 | 方法 |
|---|---|
| Web | ヘッダーの「Kill Switch」→「停止する」を **2回押す**（1回目は準備、5秒以内の2回目で発動）。「保有中の建玉も成行で決済する」を選ぶと全決済 |
| CLI | `cd apps/bot && go run . kill on -reason "理由"`（`-close` で全決済、`-scope paper` でブローカー単位）／`kill off`／`kill status` |
| 環境変数 | bot に `KILL_SWITCH=true`（DB に繋がらなくても効く。解除は変数を外して再起動） |

- 新規の発注は、発注の直前に毎回スイッチを確認して止めます。状態を読めない場合は安全側（停止）に倒します
- 全決済は次の定期確認（既定30秒以内）で実行します。市場が閉じている間は保留し、開いたら決済します

**Slack 通知**: bot に `SLACK_WEBHOOK_URL`（Incoming Webhook の URL）を設定すると、新規約定・決済・注文見送り・エラー・Kill Switch の発動と解除・日次サマリ（取引日が切り替わる 06:00 JST）を送ります。送信は裏で行い、Slack が遅くても売買は待ちません。

**ダッシュボード**（`/dashboard`）: bot の稼働状況（生存確認）、当日・今週の確定損益、割り当ての有効／無効の切り替え、保有中の建玉、最近の決済を表示します（`GET /api/v1/bot/status`）。

**API の認証**: `/api/v1/*` はすべて `Authorization: Bearer <API_TOKEN>` が必要です（`/healthz` を除く）。

- ブラウザは API を直接呼びません。Web サーバー（`apps/web/src/middleware.ts`）が、ログイン済みであることを確認してから、サーバー側だけが持つ `API_TOKEN` を付けて API（`API_URL`）へ転送します。ブラウザが送った `Authorization` は捨てます
- API は `ENVIRONMENT=production` で `API_TOKEN` が未設定なら起動しません。開発環境で未設定の場合は警告を出して認証なしで動きます
- `make setup` / `make env` が `.env` に `API_TOKEN` を生成します。docker compose は api と web の両方に渡します
- Web を Vercel で動かす場合は、Vercel 側に `API_URL`（API の公開 URL）と `API_TOKEN` を設定します
- Bruno で API を直接叩くときは、環境の `authToken` に同じトークンを入れてください

### Web からのバックテスト（ジョブ）

1. 戦略画面で「New Version」から、戦略の種類（`/api/v1/strategy-types`）とパラメータを選んでバージョンを作る
   （`strategy_versions.code` に種類、`strategy_params` に解決済みのパラメータを保存。作成後は変更せず、新しいバージョンを作る）
2. 「Backtest」でバージョン・銘柄・時間足・期間・数量・初期資金を指定して実行
3. API は条件のスナップショットを `backtests.parameters` に保存し、Redis Stream `backtest_jobs` に積む
4. bot（`make bot-dev` / compose の `bot`）がコンシューマーグループ `backtest_workers` で受け取り、実行して `results` に保存
5. 結果ページは実行中に自動更新し、完了後に評価指標・損益曲線・取引一覧を表示

bot が止まっている間に積まれたジョブは、起動時に処理されます。キャンセルされたジョブは実行しません。

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
