// Package llm — 摘要与解读功能。
// Summarize: 提取 5 条核心要点（每条 ≤25 字）。
// Brief: 四段式深度解读（背景/各方立场/影响/后续关注）。
package llm

import (
	"context"
	"fmt"
	"strings"
)

// 摘要系统提示词。
const summarySystemPrompt = `你是一名新闻编辑。阅读以下新闻，提取 5 条核心要点。每条 ≤25 个汉字。
用数字序号 1. 2. 3. 4. 5. 列出，不要其他内容。`

// 解读系统提示词。
const briefSystemPrompt = `你是一名资深国际新闻分析师。请对以下新闻进行四段式深度解读：

第一段「背景」：简述事件背景和来龙去脉
第二段「各方立场」：分析涉及各方的立场与利益诉求
第三段「影响」：分析事件对政策、市场、国际关系的可能影响
第四段「后续关注」：指出读者应持续关注的后续发展

每段 100-200 字。用 "## 背景"、"## 各方立场"、"## 影响"、"## 后续关注" 作为段落标题。`

// BriefResult 是四段式深度解读的结构化结果。
type BriefResult struct {
	Background string // 背景：事件来龙去脉
	Positions  string // 各方立场与利益分析
	Impact     string // 影响：对政策/市场/国际关系的可能影响
	Outlook    string // 后续关注点
}

// Summarize 生成 5 条要点摘要（每条 ≤25 字）。
// title 和 body 拼接为 LLM 输入；body 为空时仅用标题。
func (c *Client) Summarize(ctx context.Context, title, body string) ([]string, error) {
	if !c.Available() {
		return nil, ErrNoKey
	}
	userMsg := buildArticleMsg(title, body)
	resp, err := c.Chat(ctx, summarySystemPrompt, userMsg)
	if err != nil {
		return nil, err
	}
	return parseNumberedLines(resp), nil
}

// Brief 生成四段式深度解读。
// title 和 body 拼接为 LLM 输入；body 为空时仅用标题。
func (c *Client) Brief(ctx context.Context, title, body string) (*BriefResult, error) {
	if !c.Available() {
		return nil, ErrNoKey
	}
	userMsg := buildArticleMsg(title, body)
	resp, err := c.Chat(ctx, briefSystemPrompt, userMsg)
	if err != nil {
		return nil, err
	}
	return parseBriefResult(resp), nil
}

// buildArticleMsg 组装 LLM 输入：标题 + 正文（若有）。
func buildArticleMsg(title, body string) string {
	if body == "" {
		return fmt.Sprintf("标题：%s", title)
	}
	// 截断正文到 4000 字，避免超出模型上下文
	runes := []rune(body)
	if len(runes) > 4000 {
		body = string(runes[:4000])
	}
	return fmt.Sprintf("标题：%s\n\n正文：\n%s", title, body)
}

// parseNumberedLines 从 LLM 输出中提取数字序号行。
func parseNumberedLines(text string) []string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	var out []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// 匹配 "1." "2." "1)" "1、" 等前缀的数字序号行
		runes := []rune(trimmed)
		if len(runes) >= 2 &&
			runes[0] >= '1' && runes[0] <= '5' &&
			(runes[1] == '.' || runes[1] == ')' || runes[1] == '、') {
			out = append(out, trimmed)
		} else if len(out) > 0 {
			// 续行：附加到上一行（模型可能没有换行）
			out[len(out)-1] += " " + trimmed
		}
	}
	// 若未匹配到序号行，将整个输出作为一条
	if len(out) == 0 && text != "" {
		out = []string{text}
	}
	// 限制为最多 5 条
	if len(out) > 5 {
		out = out[:5]
	}
	return out
}

// parseBriefResult 从 LLM 输出中解析四段内容。
func parseBriefResult(text string) *BriefResult {
	r := &BriefResult{}
	currentKey := "" // 当前正在填充的段落标题
	var sb strings.Builder
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		switch trimmed {
		case "## 背景", "## 各方立场", "## 影响", "## 后续关注":
			// 保存前一段内容
			r.setSection(currentKey, sb.String())
			currentKey = trimmed
			sb.Reset()
			continue
		}
		if currentKey != "" {
			if sb.Len() > 0 {
				sb.WriteByte('\n')
			}
			sb.WriteString(line)
		}
	}
	// 保存最后一段
	r.setSection(currentKey, sb.String())
	return r
}

// setSection 根据段落标题写入对应字段。
func (r *BriefResult) setSection(key, content string) {
	content = strings.TrimSpace(content)
	if content == "" {
		return
	}
	switch key {
	case "## 背景":
		r.Background = content
	case "## 各方立场":
		r.Positions = content
	case "## 影响":
		r.Impact = content
	case "## 后续关注":
		r.Outlook = content
	}
}
