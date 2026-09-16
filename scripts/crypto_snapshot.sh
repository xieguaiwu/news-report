#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/crypto_env.sh
DAY="$(date -u +%Y%m%d)"
OUT="out/crypto_pairs_${DAY}.jsonl"

while read -r addr _rest; do
  [[ -z "$addr" || "$addr" == \#* ]] && continue
  ./bin/news-report crypto --chain bsc --sym "$addr" --out "$OUT" \
    || echo "warn: snapshot $addr failed" >&2
done < config/crypto_watchlist.txt

if [ -f "$OUT" ]; then
  echo "crypto_snapshot: $(wc -l < "$OUT") rows"
else
  echo "crypto_snapshot: no rows collected (output not created)"
fi
