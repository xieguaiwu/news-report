#!/usr/bin/env bash
# 凭据注入（零落盘、零回显）。照抄 scripts/astock_env.sh 的解析顺序。
set -euo pipefail

MODELS_JSON="${MODELS_JSON:-$HOME/.pi/agent/models.json}"
AUTH_JSON="${AUTH_JSON:-$HOME/.pi/agent/auth.json}"

resolve_provider() {  # $1 = provider 名
  python3 - "$MODELS_JSON" "$1" <<'PY'
import json, os, sys
path, name = sys.argv[1], sys.argv[2]
try:
    cfg = json.load(open(path))["providers"][name]
except Exception:
    sys.exit(0)
base = cfg.get("baseUrl", "")
key = cfg.get("apiKey", "")
if key.startswith("$"):
    key = os.environ.get(key[1:], "")
print(base); print(key)
PY
}

readarray -t BAI < <(resolve_provider bai)
export CRYPTO_LLM_BASE_URL="${CRYPTO_LLM_BASE_URL:-${BAI[0]:-}}"
export CRYPTO_LLM_API_KEY="${CRYPTO_LLM_API_KEY:-${BAI[1]:-}}"

if [[ -z "${CRYPTO_LLM_API_KEY:-}" && -f "$AUTH_JSON" ]]; then
  export CRYPTO_LLM_API_KEY="$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1])).get("bai",{}).get("key",""))' "$AUTH_JSON")"
fi

export CRYPTO_LLM_MODEL="${CRYPTO_LLM_MODEL:-qwen3.8-flash}"
export HTTPS_PROXY="${HTTPS_PROXY:-http://127.0.0.1:7897}"
export HTTP_PROXY="${HTTP_PROXY:-$HTTPS_PROXY}"

# 只报告是否就绪，不打印任何值。
echo "crypto_env: llm_base=$([[ -n "${CRYPTO_LLM_BASE_URL:-}" ]] && echo set || echo MISSING)" \
     "llm_key=$([[ -n "${CRYPTO_LLM_API_KEY:-}" ]] && echo set || echo MISSING)" \
     "tg=$([[ -n "${CRYPTO_TG_BOT_TOKEN:-}" ]] && echo set || echo MISSING)" \
     "model=$CRYPTO_LLM_MODEL"
