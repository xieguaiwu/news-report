#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/crypto_env.sh
DAY="$(date -u +%Y%m%d)"
OUT="out/crypto_${DAY}.jsonl"

# 轮 1：行情 + 安全（对 watchlist 内代币）
while read -r addr; do
  [[ -z "$addr" || "$addr" == \#* ]] && continue
  ./bin/news-report crypto --chain bsc --sym "$addr" --out "$OUT" || echo "warn: $addr failed" >&2
done < config/crypto_watchlist.txt

# 轮 2：注意力源（微博，过滤后入库）
./bin/news-report crypto --attention-only --out "$OUT" || echo "warn: attention failed" >&2

echo "crypto_daily: wrote $(wc -l < "$OUT") rows to $OUT"
