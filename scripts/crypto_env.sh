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

# ── Telegram Bot token ──────────────────────────────────────────────
# 凭据真值源优先 rbw（与 MEMORY 红线一致）。环境变量已设则不覆盖。
# rbw 未安装 / 未解锁 / 无此条目时录得空串，脚本继续跑（TG 腿降级为跳过）。
#
# 一次性准备（用户侧动作）：
#   1) 在 Telegram 里找 @BotFather → /newbot → 拿到 token
#   2) rbw add api/telegram-bot   （粘 token）
#   3) 把 bot 拉进要监控的频道/群（Bot 只能读自己所在的会话）
if [[ -z "${CRYPTO_TG_BOT_TOKEN:-}" ]] && command -v rbw >/dev/null 2>&1; then
  _tg="$(rbw get api/telegram-bot 2>/dev/null || true)"
  if [[ -n "$_tg" ]]; then
    export CRYPTO_TG_BOT_TOKEN="$_tg"
  fi
  unset _tg
fi

# 只报告是否就绪，不打印任何值。
echo "crypto_env: llm_base=$([[ -n "${CRYPTO_LLM_BASE_URL:-}" ]] && echo set || echo MISSING)" \
     "llm_key=$([[ -n "${CRYPTO_LLM_API_KEY:-}" ]] && echo set || echo MISSING)" \
     "tg=$([[ -n "${CRYPTO_TG_BOT_TOKEN:-}" ]] && echo set || echo MISSING)" \
     "model=$CRYPTO_LLM_MODEL"

if [[ -z "${CRYPTO_TG_BOT_TOKEN:-}" ]]; then
  echo "crypto_env: 注：Telegram 未配置（rbw get api/telegram-bot 为空）——注意力腿将只跑微博。" >&2
fi
