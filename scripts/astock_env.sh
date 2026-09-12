#!/usr/bin/env bash
# astock_env.sh — A股舆情打分通道凭据注入（source 本文件使用，勿直接执行）
#
#   source scripts/astock_env.sh
#
# 解析逻辑（值一律不 echo、不落盘、不提交）：
#   1. 读 /root/.pi/agent/models.json 的 providers.bai 条目：baseUrl + apiKey；
#   2. apiKey 为 "$VAR" 环境变量引用时，从当前环境解析该变量；
#   3. 环境缺失时回退 /root/.pi/agent/auth.json 的 bai.key（同为 pi 凭据库）；
#   4. export ASTOCK_LLM_BASE_URL / ASTOCK_LLM_API_KEY / ASTOCK_LLM_MODEL。
#
# 代理要求：bai 网关直连被封锁；Go net/http 经 http.ProxyFromEnvironment 自动使用
# HTTPS_PROXY —— 本脚本在 HTTPS_PROXY 未设置时自动 source /root/.pi/env。
#
# 备用通道（未启用，仅注释）：dashscope/qwen token-plan compatible-mode
# （models.json providers.qwen：baseUrl https://token-plan.cn-beijing.maas.aliyuncs.com/compatible-mode/v1，
#  同样走 $VAR 引用解析；切换需覆盖上面三个 ASTOCK_LLM_* 变量，模型用 qwen 系）。

_self="${BASH_SOURCE[0]}"
chmod 600 "$_self" 2>/dev/null

_parsed="$(python3 - <<'PY'
import json, os, sys

def load(path):
    with open(path) as f:
        return json.load(f)

try:
    bai = load('/root/.pi/agent/models.json')['providers']['bai']
except Exception as e:
    print('ERR', '读取 models.json 失败: %s' % e); sys.exit(0)

base = bai.get('baseUrl', '').rstrip('/')
key = bai.get('apiKey', '')
src = 'models.json'

if key.startswith('$'):
    name = key[1:]
    env = os.environ.get(name, '')
    if env:
        key, src = env, 'models.json->$' + name
    else:
        try:
            key, src = load('/root/.pi/agent/auth.json')['bai']['key'], 'auth.json(bai.key)'
        except Exception as e:
            print('ERR', 'env %s 未设置且 auth.json 回退失败: %s' % (name, e)); sys.exit(0)

if not base or not key:
    print('ERR', 'baseUrl 或 apiKey 为空'); sys.exit(0)

print('OK', base, key, src)
PY
)"

if [ "${_parsed%% *}" != "OK" ]; then
    echo "astock_env: ✗ ${_parsed#ERR }" >&2
    unset _parsed
    return 1 2>/dev/null || exit 1
fi

ASTOCK_LLM_BASE_URL="$(printf '%s' "$_parsed" | cut -d' ' -f2)"
ASTOCK_LLM_API_KEY="$(printf '%s' "$_parsed" | cut -d' ' -f3)"
_src="$(printf '%s' "$_parsed" | cut -d' ' -f4)"
export ASTOCK_LLM_BASE_URL ASTOCK_LLM_API_KEY
export ASTOCK_LLM_MODEL="${ASTOCK_LLM_MODEL:-glm-5.3-flash}"
unset _parsed

# 代理：bai 网关需 HTTPS_PROXY
if [ -z "${HTTPS_PROXY:-}" ] && [ -f /root/.pi/env ]; then
    . /root/.pi/env
fi

echo "astock_env: ✓ ASTOCK_LLM_BASE_URL / ASTOCK_LLM_API_KEY 已导出（bai/${ASTOCK_LLM_MODEL}, 来源 ${_src}, key 不回显）"
echo "astock_env: 代理 HTTPS_PROXY=${HTTPS_PROXY:+已设置}${HTTPS_PROXY:-未设置（bai 网关将不可达）}"
