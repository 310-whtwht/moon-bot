# Git ブランチ・コミットガイドライン

## 1. ブランチ戦略

### 1.1 ブランチ構成
```
main (production)
├── develop (staging)
├── feature/strategy-engine
├── feature/broker-adapter
├── feature/ui-components
├── bugfix/api-performance
├── hotfix/security-patch
└── release/v1.0.0
```

### 1.2 ブランチ命名規則
```bash
# 機能開発
feature/機能名-詳細
例: feature/ema-strategy-implementation
例: feature/broker-moomoo-adapter

# バグ修正
bugfix/修正内容-詳細
例: bugfix/api-response-timeout
例: bugfix/database-connection-leak

# ホットフィックス
hotfix/緊急修正内容
例: hotfix/security-vulnerability
例: hotfix/critical-api-error

# リリース準備
release/バージョン番号
例: release/v1.0.0
例: release/v1.1.0-beta

# ドキュメント
docs/ドキュメント内容
例: docs/api-specification
例: docs/deployment-guide
```

## 2. コミットメッセージ規約

### 2.1 コミットメッセージ形式
```
<type>(<scope>): <subject>

<body>

<footer>
```

### 2.2 タイプ一覧
```bash
# 新機能追加
feat: 新機能の追加

# バグ修正
fix: バグ修正

# ドキュメント
docs: ドキュメントの追加・更新

# スタイル変更
style: コードの意味に影響しない変更（空白、フォーマット等）

# リファクタリング
refactor: バグ修正や機能追加ではないコードの変更

# テスト
test: テストの追加・修正

# 設定変更
chore: ビルドプロセスや補助ツールの変更
```

### 2.3 スコープ例
```bash
# API関連
api: APIエンドポイントの変更
auth: 認証・認可の変更
db: データベース関連の変更

# フロントエンド
ui: UIコンポーネントの変更
web: Webアプリケーションの変更
style: スタイルの変更

# インフラ
infra: インフラストラクチャの変更
k8s: Kubernetes設定の変更
docker: Docker設定の変更

# 戦略・取引
strategy: 戦略エンジンの変更
broker: ブローカーアダプタの変更
trading: 取引ロジックの変更
```

### 2.4 コミットメッセージ例
```bash
# 新機能追加
feat(strategy): implement EMA crossover strategy

Add EMA crossover strategy with configurable parameters:
- Fast EMA period (default: 9)
- Slow EMA period (default: 21)
- ATR-based stop loss
- Volume filter

Closes #123

# バグ修正
fix(api): resolve timeout issue in order placement

- Increase timeout from 5s to 10s
- Add retry mechanism with exponential backoff
- Improve error handling for network issues

Fixes #456

# リファクタリング
refactor(db): optimize database queries for market data

- Add composite indexes for symbol and timestamp
- Implement query result caching
- Reduce N+1 query problems

# ドキュメント
docs(api): add comprehensive API documentation

- Add OpenAPI 3.0 specification
- Include request/response examples
- Document error codes and messages

# 設定変更
chore(ci): update GitHub Actions workflow

- Add security scanning step
- Update Node.js version to 18
- Add dependency vulnerability check
```

## 3. 開発ワークフロー

### 3.1 機能開発フロー
```bash
# 1. 最新のdevelopブランチを取得
git checkout develop
git pull origin develop

# 2. 機能ブランチを作成
git checkout -b feature/new-strategy

# 3. 開発・コミット
git add .
git commit -m "feat(strategy): implement new trading strategy"

# 4. プッシュ
git push origin feature/new-strategy

# 5. プルリクエスト作成
# GitHub/GitLabでプルリクエストを作成

# 6. レビュー・マージ
# レビュー後にdevelopブランチにマージ
```

### 3.2 ホットフィックスフロー
```bash
# 1. mainブランチからホットフィックスブランチを作成
git checkout main
git pull origin main
git checkout -b hotfix/critical-bug

# 2. 修正・コミット
git add .
git commit -m "fix(api): fix critical authentication bug"

# 3. mainブランチにマージ
git checkout main
git merge hotfix/critical-bug

# 4. タグ付け
git tag -a v1.0.1 -m "Release v1.0.1 - Critical bug fix"

# 5. developブランチにもマージ
git checkout develop
git merge hotfix/critical-bug

# 6. プッシュ
git push origin main develop
git push origin v1.0.1
```

### 3.3 リリースフロー
```bash
# 1. developブランチからリリースブランチを作成
git checkout develop
git pull origin develop
git checkout -b release/v1.0.0

# 2. バージョン番号更新
# package.json, go.mod等を更新

# 3. 最終テスト・修正
git add .
git commit -m "chore(release): prepare release v1.0.0"

# 4. mainブランチにマージ
git checkout main
git merge release/v1.0.0

# 5. タグ付け
git tag -a v1.0.0 -m "Release v1.0.0"

# 6. developブランチにマージ
git checkout develop
git merge release/v1.0.0

# 7. プッシュ
git push origin main develop
git push origin v1.0.0
```

## 4. プルリクエスト規約

### 4.1 プルリクエストテンプレート
```markdown
## 概要
<!-- 変更内容の概要を記載 -->

## 変更内容
- [ ] 新機能追加
- [ ] バグ修正
- [ ] リファクタリング
- [ ] ドキュメント更新
- [ ] その他

## 詳細
<!-- 変更内容の詳細を記載 -->

## テスト
- [ ] 単体テスト追加・更新
- [ ] 統合テスト追加・更新
- [ ] E2Eテスト追加・更新
- [ ] 手動テスト実施

## チェックリスト
- [ ] コードレビュー依頼
- [ ] テストが通ることを確認
- [ ] ドキュメント更新
- [ ] セキュリティチェック

## 関連Issue
Closes #123
Fixes #456
```

### 4.2 レビューガイドライン
```markdown
## レビューポイント

### コード品質
- [ ] 可読性
- [ ] 保守性
- [ ] パフォーマンス
- [ ] セキュリティ

### 機能
- [ ] 要件を満たしているか
- [ ] エラーハンドリング
- [ ] エッジケース対応

### テスト
- [ ] テストカバレッジ
- [ ] テストの品質
- [ ] テストの実行可能性

### ドキュメント
- [ ] README更新
- [ ] API仕様書更新
- [ ] コメント追加
```

## 5. コミット前チェック

### 5.1 事前チェックスクリプト
```bash
#!/bin/bash
# scripts/pre-commit.sh

set -e

echo "Running pre-commit checks..."

# 1. コードフォーマットチェック
echo "1. Checking code format..."
if command -v gofmt &> /dev/null; then
    gofmt -l -w .
fi

if command -v prettier &> /dev/null; then
    npx prettier --write .
fi

# 2. リンター実行
echo "2. Running linters..."
if command -v golangci-lint &> /dev/null; then
    golangci-lint run
fi

if command -v eslint &> /dev/null; then
    npx eslint --fix .
fi

# 3. テスト実行
echo "3. Running tests..."
go test ./...
npm test

# 4. セキュリティチェック
echo "4. Security checks..."
if command -v gosec &> /dev/null; then
    gosec ./...
fi

if command -v npm-audit &> /dev/null; then
    npm audit --audit-level moderate
fi

# 5. ビルドチェック
echo "5. Build checks..."
go build ./...
npm run build

echo "Pre-commit checks completed successfully"
```

### 5.2 Git Hooks設定
```bash
# .git/hooks/pre-commit
#!/bin/bash
exec scripts/pre-commit.sh
```

```bash
# .git/hooks/commit-msg
#!/bin/bash
# コミットメッセージの形式チェック
commit_regex='^(feat|fix|docs|style|refactor|test|chore)(\(.+\))?: .{1,50}'

if ! grep -qE "$commit_regex" "$1"; then
    echo "ERROR: Invalid commit message format."
    echo "Expected format: <type>(<scope>): <subject>"
    echo "Example: feat(strategy): implement EMA crossover"
    exit 1
fi
```

## 6. ブランチ保護ルール

### 6.1 mainブランチ保護
```yaml
# .github/branch-protection.yml
name: Branch Protection Rules

on:
  push:
    branches: [main, develop]

jobs:
  branch-protection:
    runs-on: ubuntu-latest
    steps:
    - name: Check branch protection
      run: |
        # mainブランチは直接プッシュ禁止
        if [ "${{ github.ref }}" = "refs/heads/main" ]; then
          echo "Direct push to main branch is not allowed"
          exit 1
        fi
```

### 6.2 プルリクエスト必須設定
```yaml
# .github/pull-request.yml
name: Pull Request Checks

on:
  pull_request:
    branches: [main, develop]

jobs:
  checks:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
    
    - name: Run tests
      run: |
        go test ./...
        npm test
    
    - name: Run linters
      run: |
        golangci-lint run
        npx eslint .
    
    - name: Check code coverage
      run: |
        go test -coverprofile=coverage.out ./...
        go tool cover -func=coverage.out
```

## 7. バージョン管理

### 7.1 セマンティックバージョニング
```bash
# メジャーバージョン（破壊的変更）
v1.0.0 -> v2.0.0

# マイナーバージョン（新機能追加）
v1.0.0 -> v1.1.0

# パッチバージョン（バグ修正）
v1.0.0 -> v1.0.1
```

### 7.2 タグ付けスクリプト
```bash
#!/bin/bash
# scripts/tag-release.sh

set -e

VERSION=$1
MESSAGE=$2

if [ -z "$VERSION" ]; then
    echo "Usage: $0 <version> [message]"
    exit 1
fi

# タグ作成
if [ -z "$MESSAGE" ]; then
    git tag -a "v$VERSION" -m "Release v$VERSION"
else
    git tag -a "v$VERSION" -m "$MESSAGE"
fi

# タグプッシュ
git push origin "v$VERSION"

echo "Tag v$VERSION created and pushed"
```

## 8. トラブルシューティング

### 8.1 よくある問題と解決方法
```bash
# 1. コミットメッセージの修正
git commit --amend -m "新しいコミットメッセージ"

# 2. 最後のコミットを取り消し
git reset --soft HEAD~1

# 3. ブランチ名の変更
git branch -m 古い名前 新しい名前

# 4. リモートブランチの削除
git push origin --delete ブランチ名

# 5. マージコンフリクトの解決
git status  # コンフリクトファイル確認
# 手動でコンフリクト解決
git add .
git commit -m "Resolve merge conflicts"
```

### 8.2 緊急時の対応
```bash
# 1. 緊急修正の適用
git checkout main
git pull origin main
git checkout -b hotfix/emergency-fix
# 修正
git commit -m "fix: emergency fix for critical issue"
git checkout main
git merge hotfix/emergency-fix
git push origin main

# 2. 誤ったコミットの取り消し
git revert コミットハッシュ

# 3. 強制プッシュ（注意: チームと相談）
git push --force-with-lease origin ブランチ名
```