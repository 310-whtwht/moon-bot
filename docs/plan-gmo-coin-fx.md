# 実装計画書 — GMOコイン 外国為替FX API 対応

- 作成日: 2026-10-03
- 対象リポジトリ: `310-whtwht/moon-bot`（基点コミット `250aa4c`）
- ステータス: ドラフト（レビュー待ち）

---

## 1. 目的とゴール

**目的**: moon-bot を「画面はあるが売買しない」状態から、**GMOコイン 外国為替FX API で実際に自動売買できる**状態にする。

**最終ゴール（Phase 6 完了時）**

- Bot が USD/JPY の確定足ごとに戦略を評価し、リスクチェックを通った注文だけを GMOコインへ発注する。
- 約定・建玉・損益が DB に残り、Web UI で確認できる。
- Kill Switch・日次損失上限・建玉照合で、異常時は自動停止して Slack に通知する。

## 2. 前提と決定事項

| 項目 | 決定 | 理由 |
|---|---|---|
| ブローカー | GMOコイン 外国為替FX API | 残高要件がない／REST + WebSocket で Go から直接扱える |
| 対象銘柄 | **USD_JPY の1銘柄から**開始 | 最小注文は100通貨（2026-10-03 に `/v1/symbols` で確認）。銘柄を増やすのは運用が安定してから |
| 時間軸 | **1時間足以上** | API 手数料（約定金額×0.002%、往復で約0.6pips 相当）があるため、短期売買は不利 |
| 戦略の実装方式 | **Go のネイティブ実装**（Starlark は後回し） | バックテストと本番で同じコードを使える。今は依存すら入っていない |
| 売買を実行する場所 | **`apps/bot` に一本化** | API サーバーは管理と閲覧だけにして、発注経路を1つにする |
| 検証方法 | **Paper（模擬約定）→ 本番少額** | GMOコイン FX API にはデモ環境の記載がない |
| moomoo 関連 | **既定案**: `broker/moomoo.go`（中身は全てスタブ）を削除する（Q5 で確定） | 使われていない。誤用を防ぐ |
| 損切り | v1 の戦略は**必ず損切り（ATR 基準）を持つ** | ブローカーの強制ロスカット（1通貨あたり0.05円の手数料）に頼らない |
| 時刻 | DB・足・ログは **UTC に統一**（表示だけ JST） | 今の DSN は `loc=Local` で、足の境界がずれる恐れがある |
| 既存データ | 開発用 DB だけで、本番データはない前提 | マイグレーションでリセットしても問題ない |

### API 仕様の要点（公式ドキュメント https://api.coin.z.com/fxdocs/ より）

- Base URL: Public `https://forex-api.coin.z.com/public/v1`、Private `…/private/v1`、WebSocket `wss://forex-api.coin.z.com/ws/public/v1` と `…/ws/private`
- 認証: ヘッダは `API-KEY` / `API-TIMESTAMP`（ミリ秒）/ `API-SIGN`。署名は HMAC-SHA256(secret, timestamp + method + path + body)
- レート制限: Private の GET は 6回/秒、POST は **1回/秒**。WebSocket の subscribe は 1回/秒
- 主な API:
  - Public: `status` / `ticker` / `klines` / `symbols`
  - Private: `account/assets` / `openPositions` / `order` / `closeOrder` / `orders/{id}`（変更）/ `executions` / `latestExecutions`、WebSocket 用トークンの取得
- klines: `priceType=BID|ASK`、`interval` は 1min〜1month。**取得できるのは 2023-10-28 以降**
- 注文: `executionType=MARKET|LIMIT|STOP`。`clientOrderId` を指定できる。決済は建玉 ID を指定して `closeOrder` を使う
- 取引ルール（USD_JPY、2026-10-03 に `/v1/symbols` の実際の応答で確認）: 最小新規注文 **100**、最大 500,000、注文単位 1、呼値 0.001。ドキュメントにある 10000 は例示の値。TRY / ZAR / MXN などは最小 10,000。**発注前に毎回 `/v1/symbols` を参照し、値は固定で持たない**
- **API 手数料: 約定金額×0.002%。無料期間は「初めて API キーを作成してから30日間」**

> **運用上の工夫**: Public API はキーなしで使える。そのため **Phase 1〜5（データ取得・バックテスト・Paper 運用）は API キーなしで進め**、キーは Phase 6 の直前に作成する。こうすると30日の無料期間を本番の立ち上げ期間に充てられる。

## 3. 目標アーキテクチャ

```
apps/web (Next.js)  ──HTTP──▶  apps/api (gin)  ──MySQL/Redis──┐
   閲覧・設定・Kill Switch          管理API・バックテスト受付       │
                                                                ▼
                                  apps/bot (worker) ◀── Redis Streams（指示/イベント）
                                    ├ scheduler   確定足ごとに起動
                                    ├ marketdata  klines(BID/ASK) 取得・保存
                                    ├ strategy    OnBar → Signal
                                    ├ risk        発注前チェック
                                    ├ execution   clientOrderId で冪等に発注
                                    ├ reconcile   ブローカー建玉と DB を照合
                                    └ broker      interface
                                         ├ paper  (ticker で擬似約定)
                                         └ gmofx  (REST + Private WS)

packages/core (Go module, 各 go.mod の replace で共有)
   broker interface / strategy / risk / backtest / 型定義
```

- **packages/core**: api と bot は別の Go モジュールで、`internal` パッケージは他モジュールから import できない。そこで共有モジュールを作り、各 `go.mod` の `replace` で参照して（`.gitignore` が `go.work` を除外しているため。`cd apps/api && go build` もそのまま使える）、戦略・リスク・バックテストのコードを両方から使う。README に書かれている `packages/shared` の役割をここで実体化する。
- **足の確定方式**: WebSocket の ticker から自前で足を組み立てない。**確定時刻の数秒後に klines REST で確定足を取得**する。こうすればバックテストと本番で同じデータを使え、組み立て処理のバグも入り込まない。ticker は Paper の約定判定と異常監視にだけ使う。
- **コスト**: BID と ASK の足を両方保存する。買いは ASK、売りは BID で約定したとみなし、スプレッドを再現する。API 手数料 0.002% もコストに含める。

### Broker interface（案）

```go
type Broker interface {
    Assets(ctx) (Assets, error)
    OpenPositions(ctx, symbol) ([]Position, error)
    PlaceOpen(ctx, OpenOrder) (OrderAck, error)     // settleType=OPEN
    PlaceClose(ctx, CloseOrder) (OrderAck, error)   // positionId 指定
    Cancel(ctx, orderID string) error
    Executions(ctx, orderID string) ([]Execution, error)
    SubscribeTicker(ctx, symbol) (<-chan Tick, error)
    SubscribeExecutions(ctx) (<-chan Execution, error)
    Status(ctx) (MarketStatus, error)               // OPEN/CLOSE/MAINTENANCE
}
```

## 4. フェーズ計画

工数は 1人日 = 実働6時間程度の目安。

### Phase 0 — 土台の修正（1〜1.5日）

| # | 作業 | 完了条件 |
|---|---|---|
| 0-1 | ビルドエラー3件を修正（`export/tax.go` の未使用 import と変数、`data/manager.go:174` の型エラー） | `go build ./... && go vet ./...` が通る |
| 0-2 | CI をリポジトリのルートに作り直す（`apps/web/.github` から移設）。Go の build / vet / test と web の lint / build を実行 | PR で CI が走り、通る |
| 0-3 | CI の Go バージョンを `go.mod` に合わせる（1.24） | 同上 |
| 0-4 | `make dev` に api を追加。`DB_DATABASE` と `DB_NAME` の不一致を解消 | `make dev` で web・api・db・redis が全て起動する |
| 0-5 | 直書きの秘密情報を `.env` に移し、`.env.example` を用意 | `moomoo123` と `NEXTAUTH_SECRET` がコードから消える |
| 0-6 | **Phase 1-1 に移動**（moomoo のスタブ型は `strategy` / `data` / `backtest` から参照されており、単独で消すとビルドが壊れるため。core に移す際に一緒に置き換える） | — |
| 0-7 | マイグレーションを流す仕組みを導入（golang-migrate など）。起動時か `make migrate` で既存の DB にも適用できるようにする | 既存のボリュームに 007 以降が当たる |

### Phase 1 — 共通コアと Public API・データ基盤（2〜3日）

| # | 作業 | 完了条件 |
|---|---|---|
| 1-1 | `packages/core` を作成し、`replace` で参照する。既存の `risk` を移し、moomoo のスタブと未使用の旧パッケージ（strategy / data / backtest）を削除する（0-6 をここで実施）。**api の Dockerfile のビルドコンテキストをルートに変更するのは、api が core を import する Phase 2 で行う** | bot から import でき、CI に core を追加する |
| 1-2 | `Broker` interface と型（Tick / Bar / Position / Order / Execution）を定義。**複数ブローカーを前提にする**: 数量は decimal で持つ。通貨・取引時間・最小単位は `Instrument` としてブローカーから受け取る。銘柄は `gmo:USD_JPY` のように名前空間付きで表す | interface に FX 固有の型が出てこない |
| 1-3 | `gmofx` の Public クライアント（`status` / `ticker` / `klines` / `symbols`）と Public WebSocket（ticker、自動再接続つき） | httptest のモックで正常系・異常系をテスト |
| 1-4 | DB マイグレーション 007: `bars` テーブル（symbol, interval, price_type, ts(UTC), OHLC）、`positions` テーブル（broker_position_id, symbol, side, size, open_price, status）、`orders` に `settle_type` と `broker_position_id` を追加。**`orders` / `positions` / `trades` / `bars` に `broker` と `account_id` の列、`orders` に `strategy_version_id` の列を追加する**（どのロジックが出した注文か追えるようにする）。価格の桁を `DECIMAL(12,5)` に拡張（`orders` / `trades` / `trade_traces`）。seed の銘柄を USD_JPY（`asset_type=forex`, `data_source=gmo`）に差し替える | `make reload` と migrate の両方で通る。Bruno テストも更新する |
| 1-5 | 過去データ取得コマンド `bot backfill -symbol USD_JPY -interval 1h -from 2023-10-28`（`make backfill`）。取引日の区切りは 06:00 JST（実データで確認）。1日ごとのリクエストを**1回/秒以下に間引き**、途中から再開できるようにする | USD_JPY の1時間足が BID と ASK の両方で揃う。週末以外の欠損がない |
| 1-6 | 乱数データ生成（`data/manager.go` の `rand`）を削除 | — |

### Phase 2 — 戦略とバックテスト（3〜4日）

> 2b 実装時の補足: 戦略バージョンは作成後に変更しない（変えるときは新バージョン）。バックテストは作成時に条件のスナップショットを保存するので、後からバージョンを変えても結果の再現性が保たれる。API の一覧系ルートは末尾スラッシュなしで登録する（Next.js のリライトとのリダイレクトループを避けるため）。
>
> 実装は2つの PR に分ける。**2a**: 2-1 / 2-2 / 2-3 / 2-5（core の指標・戦略・エンジン・評価指標、`bot backtest` CLI、実データでの評価）。**2b**: 2-1b / 2-4（API・Web の統合、ジョブ実行、戦略管理画面）と api の Dockerfile のビルドコンテキスト変更。

| # | 作業 | 完了条件 |
|---|---|---|
| 2-1 | `Strategy` interface（`OnBar(history) Signal`）と、EMA クロス戦略の実装（ATR 基準の損切りを含む）。指標（EMA / ATR）は自前で実装し、単体テストを付ける | 既知の数値で指標の計算値が一致する |
| 2-1b | 戦略の管理方式を変更: `strategy_versions.code` には**戦略の種別キー**（例: `ema_cross`）を、`strategy_params` にはパラメータを保存する。Web の戦略作成・編集画面は、コードエディタからパラメータ入力フォームに置き換える | UI から戦略を作成・編集して、バックテストに使える |
| 2-2 | バックテストエンジンを本物にする: 実データの足を使い、ASK で買い BID で売り、API 手数料を含め、建玉は1つまで | 手計算した小さなシナリオと結果が一致する |
| 2-3 | 指標の式を修正: 年率は `(1+r)^(1/年数)-1`、CAGR、ドローダウンは O(n) で計算、Sharpe を追加 | 単体テスト |
| 2-4 | API の `POST /backtests` を受けたら Redis Stream `backtest_jobs` に積み、bot が処理して結果を DB に保存する | Web UI から実行して、結果が表示される |
| 2-5 | 2023-10〜現在の期間で EMA クロスを評価し、パラメータの感度を簡単に確認する | 結果を `docs/` に記録する（採否は人が判断） |

### Phase 3 — Bot 本体（Paper モード）（3〜4日）

> 実装は2つの PR に分ける。**3a**: 3-1〜3-4b（paper broker、リスク確認、発注・建玉管理、スケジューラ、統合テスト、`deployments` テーブル）。**3b**: 3-5〜3-7（Kill Switch、Slack 通知、Web ダッシュボード）。
>
> 3b 実装時の補足:
> - Kill Switch の状態は **DB（`kill_switches`）だけ**に持つ（計画では Redis のフラグも併用する想定だった）。新規の発注は発注直前に毎回 DB を確認するので即座に止まり、状態を読めないときは停止側に倒す。全決済は次のポーリング（既定30秒以内）で実行する
> - bot の生存確認は `bot_heartbeats`（ポーリングごとに更新）。ダッシュボードは最終更新から3間隔以内を「稼働中」とする
> - **API に認証が無い**（Kill Switch・割り当ての切り替えを含む）。API を外部公開する前に認証を入れる（Phase 7 の前提作業に追加）
> - 割り当ての新規作成・編集の画面は未実装（有効／無効の切り替えのみ）。追加は SQL で行う
>
> 3a 実装時の補足:
> - 足の確定は「確定時刻＋数秒に1回取得」ではなく、**一定間隔（既定30秒）のポーリング**で検出する（足の境界の揃え方に依存せず、取りこぼしても次の確認で拾える）。確定から5分を超えたシグナルは実行しない
> - 損切りは bot 側でレートを監視して成行決済する。**bot が止まっている間は損切りが効かない**ため、Phase 4 でブローカー側の逆指値注文を併用する
> - `clientOrderId` は `{割り当てID先頭8桁}-{足の開始時刻}-{open|close}`
> - 「どの戦略をどこで動かすか」を表す `deployments` テーブルを追加（計画時点では未定義だった）

| # | 作業 | 完了条件 |
|---|---|---|
| 3-1 | `paper` broker: ticker の BID / ASK で即時約定させ、手数料を計上する。建玉と資産はメモリと DB に持つ | 単体テスト |
| 3-2 | scheduler: 確定時刻＋数秒で klines を取得し、戦略を評価する。`status` が OPEN でなければ何もしない（週末やメンテナンス中） | 週末に発注しないことをテストで確認 |
| 3-3 | risk: 既存の RiskManager を移す。成行注文の価格見積もりを現在の ticker に置き換え、上限を足す（建玉数量・同時建玉数・日次損失・週次損失）。**上限は口座ごとと、円に換算した全体の2段**で持つ。**発注前に証拠金の余力を確認**する（`assets`） | 既存テストと追加テストが通る |
| 3-3b | 統合テスト: 偽の時計と Paper broker を使い、「足が確定 → 戦略 → リスク → 約定 → DB と Stream に記録」を通しで検証する | CI で実行される |
| 3-4 | execution: `clientOrderId = {strategy}-{symbol}-{barTime}` で冪等にする。DB に `pending` で記録してから発注し、応答で更新する | 同じ足では2回発注されない |
| 3-4b | 戦略の切り替え規則: 戦略やパラメータの変更は**建玉がないとき、または次の足から**反映する。保有中の建玉は、建てたときのバージョンの決済ルールで閉じる | 切り替えをまたぐテスト |
| 3-5 | Kill Switch: **ブローカーごとと全体の2階層**で、Redis のフラグと DB に状態を持ち、Web のヘッダ、CLI、環境変数のどれからでも止められる。発動時は新規発注を禁止し、任意で全決済する | 発動から数秒以内に発注が止まる |
| 3-6 | Slack 通知を実装（今は `notifications/manager.go:138` が TODO）。約定、エラー、Kill Switch 発動、日次サマリを送る | テスト用チャンネルで受信できる |
| 3-7 | Web: ダッシュボードに Bot の状態、建玉、当日損益、Kill Switch を表示する | 画面で確認できる |

### Phase 4 — Private API（本番接続の準備）（2〜3日）

> 実装は2つの PR に分ける。**4a**: 4-1 / 4-2 / 4-4b / 4-5（署名・レート制限・REST の各 API、結果不明の注文の照合、本番ガード）。**4b**: 4-3 / 4-4（Private WebSocket、建玉の照合）と、ブローカー側の逆指値注文、API の認証。
>
> 4a で公式ドキュメントから確認した仕様（着手前の要確認事項の回答）:
> - 署名対象は `timestamp + method + path + body`。path は `/v1` 始まり、GET はボディ空で**クエリ文字列は含めない**
> - WebSocket 用トークンは `POST / PUT / DELETE /v1/ws-auth`（有効60分、延長で60分に戻る、最大5個）。接続先は `wss://forex-api.coin.z.com/ws/private/v1/{token}`、チャンネルは `executionEvents` など
> - キャンセルは `POST /v1/cancelOrders`（`rootOrderIds` か `clientOrderIds`）。決済は `POST /v1/closeOrder` に `settlePosition: [{positionId(数値), size(文字列)}]`
> - `clientOrderId` は**英数字のみ・36文字以内**。注文を `clientOrderId` で直接引く API は無く、`latestExecutions`（直近1日・最大100件）と `activeOrders` に含まれる値で照合する
> - 成行注文の応答に約定は含まれないため、`executions?orderId=` を少し待って取得する
> - 時刻ずれは `ERR-5008` / `ERR-5009`、呼出上限は `ERR-5003`、定期／緊急メンテナンスは `ERR-5201` / `ERR-5202`、取引時間外は `ERR-5218`
> - 約定の `fee` は符号付き（コストが負）で返るため、正のコストに直して保存する（**実データでの確認は未実施**）

**着手前に確認すること**: Private WebSocket 用トークンの取得・延長 API の正確なパス、エラーコード一覧、定期メンテナンスの時間帯。いずれも公式ドキュメントで確認する。

**API キーはまだ作らない。** モックサーバーだけで実装とテストを済ませる。

| # | 作業 | 完了条件 |
|---|---|---|
| 4-1 | 署名処理とレート制限（GET 6回/秒、POST 1回/秒のトークンバケット）、時刻ずれへの対応 | 公式ドキュメントの署名例と同じ値になる |
| 4-2 | `assets` / `openPositions` / `order` / `closeOrder` / `cancel` / `executions` を実装 | httptest のモックで正常系・エラー・タイムアウト・429 をテスト |
| 4-3 | Private WebSocket（約定と建玉の通知）。トークンの取得と延長、再接続、切断中の取りこぼしを REST で補う処理 | 接続が切れても約定を取りこぼさない |
| 4-4 | reconcile: 起動時と N 分ごとに、ブローカーの建玉と DB を照合する。不一致なら Kill Switch を発動して通知 | 不一致を注入したテストで停止する |
| 4-4b | 結果が分からない注文の解決: 発注がタイムアウトしたら `activeOrders` / `latestExecutions` / `openPositions` で照合する。それでも確定できなければ新規発注を止めて通知する（自動で再送しない） | タイムアウトを注入したテストで二重発注が起きない |
| 4-5 | 本番ガード: `BROKER_MODE=live` と `LIVE_CONFIRM=yes` の両方が揃わないと gmofx を使わない。最大発注数量は設定値とコード内上限の小さいほうを使う | ガードのテスト |

### Phase 5 — Paper 運用（2週間以上・ほぼ待ち時間）

- 実際のレートで Paper モードを連続稼働させる（ローカルの Mac か OCI）。
- **合格基準**（全て満たしたら Phase 6 へ）:
  - 10営業日以上、プロセスの異常終了がない（あっても自動で復帰する）
  - 約定・建玉・損益の記録が、ticker から再計算した値と一致する
  - 週末とメンテナンスを跨いでも正しく停止・再開する
  - Kill Switch・日次損失上限・Slack 通知の発火を、実際に1回ずつ確認する

### Phase 6 — 本番（少額）（立ち上げ1日＋様子見）

| # | 作業 |
|---|---|
| 6-1 | GMOコインで FX 口座を開設し、資金を入金する（目安 1〜3万円。100通貨の必要証拠金は約630円＝157.9円×100÷25） |
| 6-2 | API キーを作成する。**権限は取引と照会に限り、IP 制限を有効にする**（固定 IP の実行環境が必要。下の Q2 を参照）。キーは `.env` か Secret 管理に置き、リポジトリには入れない |
| 6-3 | 上限を最小にして起動する: **100通貨・建玉1つ・日次損失上限を低く**（1pip の値動きで約1円）設定 |
| 6-4 | 最初の数日は約定のたびに、GMOコインの画面と DB を目視で照合する |
| 6-5 | 30日の無料期間中に、手数料を含めた実際のコストを測り、続けるかどうかを判断する |

### Phase 7 — 運用の安定化（任意）

- OCI への常時稼働デプロイ（既存の `deploy-oci.yml` を活用）、死活監視、ログの保管。
- 銘柄や戦略の追加、Starlark の再検討。

### Phase 8 — AI 拡張（任意・Phase 6 の安定稼働が前提）

AI は「判断」の部品として `Strategy` interface（またはその前段のフィルター）に差し込む。**どの AI もリスク確認を必ず通し、上限を超えることはできない**。導入は検証しやすい順に A → B → C とする。

| # | 段階 | 内容 | 検証方法 | 完了条件 |
|---|---|---|---|---|
| 8-A | AI フィルター | LLM が経済指標カレンダーやニュースを読み、「取引する・控える」を1日数回判断する。売買そのものは既存の戦略が行う | 判断ログを残し、Paper 運用で「フィルターあり／なし」を並べて比べる | 4週間以上の比較結果を人がレビューする |
| 8-B | 機械学習の戦略 | 価格から作った特徴量で学習したモデルが売買シグナルを出す。推論は Go 側、学習はオフラインで行う | 期間をずらしながら学習と検証を繰り返す（ウォークフォワード）。学習に使っていない期間で評価する | 検証期間でも期待値がプラスで、取引回数も十分にある |
| 8-C | LLM による直接判断 | 足が確定するたびに LLM が売買を判断する | **Paper での前向き検証に限る**（LLM は過去の相場を学習済みなので、バックテストでは公平に評価できない） | 人が承認するまで本番では使わない |

- 共通: プロンプトとモデルのバージョンを記録し、判断の根拠を残す。API 費用の上限を設定する。

### Phase 9 — 米国株（moomoo など）対応（任意）

- Phase 1 の複数ブローカー前提の設計の上に、2つ目の broker アダプタを足す。
- moomoo の場合は OpenD と Python の中継サービス（`moomoo-api`）が必要になる。実装の軽さでは、REST で直接呼べるウィブル証券も候補として比較する。
- 追加の工数の目安は 6〜9人日。米国株の取引時間・USD 建て・円換算した全体リスクへの対応を含む。

## 5. スコープ外

- moomoo・OANDA など、他のブローカーの実装（設計上の布石は Phase 1 で入れる。実装は Phase 9）
- AI による判断（Phase 8）
- Starlark による戦略の DSL
- 複数ユーザー、税務エクスポートの改修
- 両建て、IFD / OCO を使う戦略（v1 は成行と建玉1つだけ）

## 6. リスクと対策

| リスク | 影響 | 対策 |
|---|---|---|
| デモ環境がない | 本番でしか見つからないバグ | Paper の broker、モックサーバーのテスト、本番は最小の上限から始める |
| 再送による二重発注 | 想定外の建玉 | `clientOrderId` の冪等性、DB への先行記録、reconcile |
| WebSocket の切断で約定を取りこぼす | 建玉の認識がずれる | REST の `executions` で補う、定期的な reconcile |
| API 手数料と短期売買の相性 | 期待値がマイナスになる | 1時間足以上、バックテストに手数料を含める |
| POST の 1回/秒 制限 | 決済が遅れる | 1銘柄・建玉1つから始める、送信キューで直列化する |
| メンテナンスと週末 | 発注エラーの連続 | `status` で判定してスキップする |
| API キーの漏えい | 不正取引 | 権限を最小にする、IP 制限、`.env` を git 管理外にする |
| データは 2023-10-28 以降しかない | バックテストの期間が約3年に限られる | 結果の過信を避け、Paper で再確認する |

## 7. 未決事項（着手前に決めたいこと）

| # | 問い | 既定案 |
|---|---|---|
| Q1 | 本番の運用資金はいくらか | 1〜3万円（100通貨から始め、段階的に増やす） |
| Q2 | Bot をどこで常時稼働させるか（Mac 常時起動 か OCI か） | Phase 5 までは Mac、本番は OCI（固定 IP で API の IP 制限が使える） |
| Q3 | 最初の戦略は EMA クロスでよいか | EMA クロス（1時間足）。採否は Phase 2 の結果で判断 |
| Q4 | Slack の通知先 | 専用チャンネルを1つ作る |
| Q5 | moomoo のスタブを削除してよいか | 削除する（git 履歴には残る） |
| Q6 | 「GMOコインを採用する」判断を company-os に ADR として残すか（moon-bot が company-os の管理対象かどうか） | 未定。ユーザーが判断する |

## 8. 工数の目安

| Phase | 工数 |
|---|---|
| 0 | 1〜1.5日 |
| 1 | 2〜3日 |
| 2 | 3〜4日（戦略管理の UI 変更を含む） |
| 3 | 3〜4日 |
| 4 | 2〜3日 |
| 5 | 2週間以上（主に待ち時間） |
| 6 | 1日＋様子見 |
| 8（任意） | A: 2〜3日 / B: 5日〜 / C: 3日〜（それぞれに検証期間が別途必要） |
| 9（任意） | 6〜9日 |
| **合計（Phase 0〜6 の実装）** | **約12〜16人日**（＋ Paper 運用期間） |

## 9. 進め方

- Phase ごとにブランチを切り、PR を作る（`[feat] …` 形式のコミット）。各 PR で CI が通ることを必須にする。
- Phase 2-5 のバックテスト結果と Phase 5 の合否は、人間が判断する。AI は数値を提示するところまでとする。

---

*本書はソフトウェアの実装計画であり、特定の投資判断を推奨するものではありません。*
