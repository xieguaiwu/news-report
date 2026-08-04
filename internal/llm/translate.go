// Package llm — 翻译功能：自动按段落边界分块，块间有重叠，逐块调 Chat 后拼接。
package llm

import (
	"context"
	"strings"
)

// 翻译系统提示词（硬编码常量，按计划规范）。
const translateSystemPrompt = `你是一名专业新闻译者。将以下新闻文本翻译成中文。要求：
1. 保留专有名词原文（人名、地名、机构名首次出现时保留原文并括号标注中文）
2. 保留所有数字、百分比、日期精确不变
3. 保留直接引语的原意和语气
4. 专业术语准确翻译
5. 仅输出译文，不要加任何解释或注释`

// Translate 翻译文本到目标语言。
// 自动分块：≤ MaxChars 的文本按段落边界分块，块大小 ≤ ChunkSize，重叠 Overlap 字符。
// 每个块独立调用 Chat，结果拼接返回。
// sourceLang 为源语言代码（如 "en", "de", "fr"），仅用于缓存 key 区分。
func (c *Client) Translate(ctx context.Context, text, sourceLang string) (string, error) {
	if !c.Available() {
		return "", ErrNoKey
	}
	if text == "" {
		return "", nil
	}

	// 截断：取前 MaxChars 字符
	maxChars := c.cfg.MaxChars
	if maxChars <= 0 {
		maxChars = 4000
	}
	runes := []rune(text)
	if len(runes) > maxChars {
		runes = runes[:maxChars]
	}
	body := string(runes)

	// 分块
	chunks := chunkText(body, c.cfg.ChunkSize, c.cfg.Overlap)
	if len(chunks) == 0 {
		return "", nil
	}

	var results []string
	for _, chunk := range chunks {
		resp, err := c.Chat(ctx, translateSystemPrompt, chunk)
		if err != nil {
			return "", err
		}
		results = append(results, resp)
	}
	return strings.Join(results, "\n\n"), nil
}

// chunkText 按段落边界分块，确保每块 ≤ maxChars，重叠 overlapChars。
// 采用段落级贪心合并 + 重叠策略，避免逐字符滑动。
func chunkText(text string, maxChars, overlapChars int) []string {
	if maxChars <= 0 {
		maxChars = 1500
	}
	if overlapChars < 0 {
		overlapChars = 0
	}
	if overlapChars >= maxChars {
		overlapChars = maxChars / 4
	}

	// 按空行分段
	paragraphs := splitParagraphs(text)
	if len(paragraphs) == 0 {
		return nil
	}

	var chunks []string
	cur := ""
	curRuneLen := 0

	for _, p := range paragraphs {
		pLen := len([]rune(p))
		// 如果当前段+新段超过限制，闭合当前块
		if curRuneLen+pLen > maxChars && cur != "" {
			chunks = append(chunks, cur)
			// 重叠：取 cur 末尾 overlapChars 字符
			if overlapChars > 0 {
				overlapText := lastNRunes(cur, overlapChars)
				cur = overlapText + "\n\n" + p
				curRuneLen = len([]rune(overlapText)) + 2 + pLen
			} else {
				cur = p
				curRuneLen = pLen
			}
		} else {
			if cur != "" {
				cur += "\n\n" + p
				curRuneLen += 2 + pLen
			} else {
				cur = p
				curRuneLen = pLen
			}
		}
	}
	if cur != "" {
		chunks = append(chunks, cur)
	}
	return chunks
}

// splitParagraphs 按 \n\n 分段，过滤空段。
func splitParagraphs(text string) []string {
	raw := strings.Split(text, "\n\n")
	var out []string
	for _, p := range raw {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// lastNRunes 取字符串末尾 n 个 rune（中文友好）。
func lastNRunes(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[len(runes)-n:])
}
