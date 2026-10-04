#!/usr/bin/env bash
# Upload the variables in .env.prod to the Vercel project's Production environment.
#
#   cp .env.prod.example .env.prod   # then fill in the values (git-ignored)
#   npm run env:push                 # or: bash scripts/push-env.sh [file] [environment]
#
# - Values are piped to `vercel env add` via stdin, never passed as arguments
#   and never printed.
# - Existing variables are overwritten (--force) and stored as Sensitive.
# - An empty AUTH_SECRET is generated once and written back to the file, so
#   re-running keeps the same secret (changing it signs everyone out).
# - Requires the Vercel CLI, logged in and linked (`vercel link` in apps/web).
set -euo pipefail

cd "$(dirname "$0")/.."
FILE="${1:-.env.prod}"
TARGET="${2:-production}"

if [[ ! -f "$FILE" ]]; then
  echo "$FILE がありません。cp .env.prod.example $FILE で作成して値を入れてください。" >&2
  exit 1
fi
if ! command -v vercel >/dev/null 2>&1; then
  echo "Vercel CLI が見つかりません（npm i -g vercel）。" >&2
  exit 1
fi
if [[ ! -f .vercel/project.json ]]; then
  echo "このディレクトリが Vercel プロジェクトにリンクされていません。先に vercel link を実行してください。" >&2
  exit 1
fi

# Generate AUTH_SECRET once if it is present but empty.
if grep -qE '^AUTH_SECRET=[[:space:]]*$' "$FILE"; then
  generated="$(openssl rand -base64 32)"
  tmp="$(mktemp)"
  awk -v s="$generated" '/^AUTH_SECRET=[[:space:]]*$/ { print "AUTH_SECRET=" s; next } { print }' "$FILE" >"$tmp"
  cat "$tmp" >"$FILE"
  rm -f "$tmp"
  echo "AUTH_SECRET を生成して $FILE に書き込みました。"
fi

required=(AUTH_SECRET ADMIN_EMAIL ADMIN_PASSWORD_HASH)
names=() # works with macOS's bash 3.2 (no associative arrays)

while IFS= read -r line || [[ -n "$line" ]]; do
  # Skip blanks and comments.
  [[ -z "${line//[[:space:]]/}" || "$line" =~ ^[[:space:]]*# ]] && continue
  if [[ ! "$line" =~ ^([A-Z][A-Z0-9_]*)=(.*)$ ]]; then
    echo "読み取れない行があります（KEY=VALUE の形式にしてください）。" >&2
    exit 1
  fi
  key="${BASH_REMATCH[1]}"
  value="${BASH_REMATCH[2]}"
  # Strip one pair of surrounding quotes, so values with $ can be written as '...'.
  if [[ "$value" =~ ^\'(.*)\'$ || "$value" =~ ^\"(.*)\"$ ]]; then
    value="${BASH_REMATCH[1]}"
  fi
  if [[ -z "$value" ]]; then
    echo "skip  $key（値が空）"
    continue
  fi
  printf '%s' "$value" | vercel env add "$key" "$TARGET" --force --sensitive >/dev/null
  names+=("$key")
  echo "set   $key"
done <"$FILE"

missing=()
for key in "${required[@]}"; do
  case " ${names[*]:-} " in
    *" $key "*) ;;
    *) missing+=("$key") ;;
  esac
done
if ((${#missing[@]} > 0)); then
  echo "注意: 必須の変数が設定されていません: ${missing[*]}" >&2
fi

echo
echo "${#names[@]} 件を Vercel（$TARGET）に設定しました。反映には再デプロイが必要です（vercel --prod、または Vercel の画面で Redeploy）。"
