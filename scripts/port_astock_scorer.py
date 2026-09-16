# scripts/port_astock_scorer.py（在 news-report 仓根执行）
import re
import pathlib

SRC = pathlib.Path("internal/astock/llmscore.go")  # 相对仓根
DST = pathlib.Path("internal/crypto/llmscore.go")  # 相对仓根
NL = chr(92) + "n"   # Go 源码字面量 \n
LF = chr(10)         # 真实换行

s = SRC.read_text()
s = s.replace("package astock", "package crypto")
s = s.replace("NewsItem", "AttentionItem")
s = s.replace("defaultUA", "userAgent")
s = s.replace('"astock:', '"crypto:')
s = s.replace("astock: ", "crypto: ")
s = s.replace("astock-llmscore-v1", "crypto-attention-v1")
s = s.replace("ASTOCK_LLM_BASE_URL", "CRYPTO_LLM_BASE_URL")
s = s.replace("ASTOCK_LLM_API_KEY", "CRYPTO_LLM_API_KEY")
s = s.replace("ASTOCK_LLM_MODEL", "CRYPTO_LLM_MODEL")
s = s.replace('const DefaultModel = "glm-5.3-flash"', 'const DefaultModel = "qwen3.8-flash"')

# 删除 astock 的 Score 类型（crypto.go 已定义）
s = re.sub(r"// Score 是 LLM 打分结果。\ntype Score struct \{.*?\n\}\n\n", "", s, flags=re.S)

# system prompt：Go 原始字符串（反引号），内部需要真实换行
prompt_lines = [
    "const scoreSystemPrompt = `你是加密 meme 币舆情打分器。对给定的文本，只输出一个 JSON 对象（无其他文字、无 markdown 代码块），字段如下：",
    '{"tone": 整数 -2..2（-2 极度看空 .. 2 极度看多）,',
    ' "narrative": 字符串，该文本绑定的叙事标签（如 "牛市来了"、"动物币"、"AI 概念"、"政治梗"），无法判断填 "其他",',
    ' "shill_score": 0..1，喊单/蛊惑强度——无实质信息却有明确买入诱导措辞则越高,',
    ' "specificity": 0..1，信息具体程度（有地址/数字/时间/来源则为高）,',
    ' "source_tier": "官方"|"媒体"|"自媒体"|"不明",',
    ' "black_score": 0..1，低级黑判定（论据缺失 + 情绪化渲染 + 恐慌/亢奋诱导）}`',
]
new_prompt = LF.join(prompt_lines) + LF
s = re.sub(r"const scoreSystemPrompt = `.*?`\n", lambda m: new_prompt, s, flags=re.S)

# user prompt：Go 解释字符串，内部用 NL
user_lines = [
    "func scoreUserPrompt(it AttentionItem) string {",
    '\treturn "链: " + it.Chain + "' + NL + '代币: " + it.Symbol + "' + NL + '来源: " + it.Source +',
    '\t\t"' + NL + '标题: " + it.Title + "' + NL + '正文: " + truncate(it.Text, 2000)',
    "}",
    "",
]
new_user = LF.join(user_lines)
s = re.sub(r"func scoreUserPrompt\(.*?\n\}\n", lambda m: new_user, s, flags=re.S)

assert "func scoreUserPrompt" in s and "it.Chain" in s

# parseScore：Kind → Narrative + ShillScore
old_lit = (
    "\tsc := Score{\n"
    '\t\tTone:        clampInt(raw["tone"], -2, 2),\n'
    '\t\tKind:        enumOf(raw["kind"], []string{"业绩", "监管", "重组", "传闻", "研报", "自媒体", "其他"}, "其他"),\n'
    '\t\tSpecificity: clampFloat(raw["specificity"]),\n'
    '\t\tSourceTier:  enumOf(raw["source_tier"], []string{"官方", "媒体", "自媒体", "不明"}, "不明"),\n'
    '\t\tBlackScore:  clampFloat(raw["black_score"]),\n'
    "\t}\n"
)
new_lit = (
    "\tsc := Score{\n"
    '\t\tTone:        clampInt(raw["tone"], -2, 2),\n'
    '\t\tNarrative:   strOf(raw["narrative"], "其他"),\n'
    '\t\tShillScore:  clampFloat(raw["shill_score"]),\n'
    '\t\tSpecificity: clampFloat(raw["specificity"]),\n'
    '\t\tSourceTier:  enumOf(raw["source_tier"], []string{"官方", "媒体", "自媒体", "不明"}, "不明"),\n'
    '\t\tBlackScore:  clampFloat(raw["black_score"]),\n'
    "\t}\n"
)
assert old_lit in s, "parseScore 结构体字面量未命中"
s = s.replace(old_lit, new_lit)

# 追加 strOf 辅助函数
s += LF.join([
    "",
    "// strOf 容错取自由文本字段（trim 后非空才算命中）。",
    "func strOf(v any, def string) string {",
    "\ts, ok := v.(string)",
    "\tif !ok {",
    "\t\treturn def",
    "\t}",
    "\ts = strings.TrimSpace(s)",
    '\tif s == "" {',
    "\t\treturn def",
    "\t}",
    "\treturn s",
    "}",
    "",
])

if '"strings"' not in s:
    s = s.replace("import (" + LF, "import (" + LF + '\t"strings"' + LF, 1)

DST.write_text(s)
print("wrote", DST, len(s.splitlines()), "lines")
