# Mac で Paper 運用を始める手順（Phase 5）

実際の GMOコインのレートを使い、**模擬約定（Paper）** で bot を連続稼働させます。お金は動きません。
API キーも不要です（Public API だけを使います）。

- 対象: `docker compose`（MySQL・Redis・API・bot・Web）を Mac 上で常時動かす
- 2026-10-04 に、この手順の起動〜割り当ての有効化〜自動復帰までを使い捨ての環境で確認済み

## 0. 前提

- Docker Desktop が起動していること
- ポート `3001`（Web）・`8081`（API）・`3308`（MySQL）・`6380`（Redis）が空いていること
- リポジトリ: `~/Desktop/Programming/Works/aoyama-create/moon-bot`（`main` を最新に）

## 1. 初回の準備

```bash
cd ~/Desktop/Programming/Works/aoyama-create/moon-bot
git checkout main && git pull
make env
```

`make env` は `.env.example` から `.env` を作り、`NEXTAUTH_SECRET` と `API_TOKEN` を自動生成します（既に `.env` があれば何もしません）。
続けて `.env` を開き、次の2行を自分の値に変えます。

```
MYSQL_ROOT_PASSWORD=（任意の文字列）
DB_PASSWORD=（任意の文字列）
```

> 以前の `.env` を使い続ける場合は、`API_TOKEN=` の行があるか確認してください。無ければ `openssl rand -hex 32` の結果を `API_TOKEN=` の後ろに書きます。

### Slack 通知（任意だが推奨）

Slack で Incoming Webhook を発行し、`.env` に追記します。約定・決済・エラー・Kill Switch・日次サマリ（朝6時）が届きます。

```
SLACK_WEBHOOK_URL=https://hooks.slack.com/services/XXX/YYY/ZZZ
```

## 2. 起動

```bash
make dev
```

初回はイメージのビルドに数分かかります。状態は次で確認します（5つとも `Up`、api は `healthy`）。

```bash
docker compose ps
```

| 確認先 | 内容 |
|---|---|
| http://localhost:3001/dashboard | ダッシュボード（開発モードのためログイン不要） |
| `docker compose logs -f bot` | bot のログ |

ダッシュボードの「Bot」が **稼働中** になっていれば、bot は動いています（起動直後の数十秒は「停止」と出ることがあります）。

## 3. 動かす戦略を決めて有効にする

1. **バージョンを確認・作成**: http://localhost:3001/strategies →「EMA クロス（サンプル）」。
   既定のバージョン 1.0.0 は fast 12 / slow 26 / ATR14 / 損切り 2×ATR / 売りあり。
   別のパラメータで動かすなら「New Version」で作り、「有効にする」をオンにします（評価は `docs/backtests/` を参照）。
2. **割り当てを有効にする**: ダッシュボードの「割り当て」で、`EMA クロス USD/JPY 1時間足（Paper）` のスイッチをオン。
3. **bot が拾ったことを確認**: 30秒以内に bot のログへ次の2行が出ます。

```
... runner started (paper USD_JPY 1h, 100 units, enabled=true)
... strategy ema_cross ready (version ..., 500 warmup bars, last bar ...)
```

これ以降、1時間足が確定するたびにシグナルを評価し、その結果を1行ずつログに残します（時刻は UTC）。

```
... bar 2026-10-05T02:00:00Z close 157.854 -> HOLD (flat) [fast=157.760 slow=157.789 diff=-0.029 atr=0.142]
```

`HOLD` は「何もしない」、`flat` は「建玉なし」です。`diff`（短期 EMA − 長期 EMA）の符号が変わった足で `ENTER_LONG` / `ENTER_SHORT` になります。
**売買は次の足から**です（起動直後の足は指標の準備に使います）。
EMA クロスは取引の少ない戦略なので、最初の約定まで数時間〜数日かかることがあります。

### 画面でバックテストも使う場合

過去データを DB に入れておきます（初回のみ・十数分）。

```bash
docker compose run --rm bot backfill -from 2023-10-28 -min-interval 400ms
```

## 4. 動かし続けるために

- **Mac をスリープさせない**: 「システム設定 → バッテリー（またはロック画面）」で、電源接続中はスリープしない設定に。スリープ中は bot も止まり、Paper の損切りも効きません。
- **自動復帰**: 各コンテナは `restart: unless-stopped` です。プロセスが落ちても、Mac や Docker Desktop を再起動しても自動で戻ります（Docker Desktop の「ログイン時に起動」をオンに）。
- 再起動後は、DB から残高と建玉を復元して続きから動きます。

## 5. 日々の確認

| 見るもの | 場所 |
|---|---|
| bot が生きているか | ダッシュボードの「Bot」／Slack の日次サマリ |
| 建玉・決済・損益 | ダッシュボード |
| レートと約定をチャートで見る | http://localhost:3001/chart（実レートをリアルタイム表示。約定の位置・建値・損切りを重ねて表示） |
| 注文の履歴（見送り・拒否の理由も） | http://localhost:3001/orders |
| エラー | Slack、`docker compose logs --since 24h bot \| grep -iE "error\|failed\|rejected"` |

## 6. 止める・再開する

| やりたいこと | 方法 |
|---|---|
| 新規の発注だけ止める | ヘッダーの「Kill Switch」→「停止する」を2回押す。CLI なら `docker compose run --rm bot kill on -reason "理由"` |
| 建玉も閉じて止める | 上と同じ画面で「保有中の建玉も成行で決済する」にチェック。CLI なら `kill on -close` |
| 再開する | 「停止中」→「解除して取引を再開」。CLI なら `docker compose run --rm bot kill off` |
| 状態を見る | `docker compose run --rm bot kill status` |
| 全部を止める | `make docker-down`（データは残ります） |
| データごと消して最初から | `make reload`（**建玉・履歴も消えます**） |

## 7. 更新するとき

```bash
git pull
make dev
```

`make dev` はイメージを作り直して入れ替えます。DB の変更（マイグレーション）は API の起動時に自動で当たります。
建玉を持ったまま再起動しても、復元して続行します。

## 8. 合格基準（Phase 5）

2週間（10営業日）以上動かし、次をすべて満たしたら Phase 6（本番・少額）に進みます。

- [ ] プロセスの異常終了がない（あっても自動で復帰している）
- [ ] 約定・建玉・損益の記録が妥当（約定価格がその時刻のレートと合う、損益の計算が合う）
- [ ] 週末（土曜 6時〜月曜 6〜7時）とメンテナンスを跨いで、止まらず・誤発注せずに再開する
- [ ] Kill Switch の発動と解除、日次損失の上限での見送り、Slack 通知を、それぞれ実際に1回ずつ確認する
- [ ] 約定の回数と損益の傾向が、同じ期間のバックテストと大きく違わない

## 困ったとき

| 症状 | 確認 |
|---|---|
| `make dev` が `set API_TOKEN in .env` で止まる | `.env` に `API_TOKEN=` の行を足す（`openssl rand -hex 32`） |
| api が `unhealthy` のまま | `docker compose logs api`。DB パスワードを変えた場合、以前のボリュームと合っていない可能性 → `.env` を元に戻すか `make reload` |
| ダッシュボードが「取得できませんでした」 | `docker compose ps` で api が動いているか。`docker compose logs web` |
| Bot が「停止」のまま | `docker compose logs bot`。DB・Redis に繋がらないと起動時に終了し、再起動を繰り返します |
| 割り当てを有効にしても動かない | bot のログに `runner started` が出ているか。戦略に「有効なバージョン」があるか |
| 約定しない | 正常な場合が多い（シグナル待ち）。`/orders` に `rejected` があれば理由を確認 |

---

*これはソフトウェアの検証手順です。Paper の成績は実際の取引の成果を保証しません。*
