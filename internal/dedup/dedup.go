// Package dedup 实现跨来源新闻去重：
//  1. 规范化标题完全一致 → 直接判重
//  2. 否则按 token 集合的 Jaccard 相似度聚类（同语言阈值 0.6，跨语言 0.7）
package dedup

import (
	"strings"
)

// NormalizeTitle 规范化标题：小写、去标点、压缩空白。
func NormalizeTitle(t string) string {
	var sb strings.Builder
	prevSpace := false
	for _, r := range strings.ToLower(t) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' ||
			r >= '\u00c0' && r <= '\u024f' { // 拉丁扩展（变音字母）
			sb.WriteRune(r)
			prevSpace = false
		} else if !prevSpace {
			sb.WriteByte(' ')
			prevSpace = true
		}
	}
	return strings.TrimSpace(sb.String())
}

// tokenize 按空白切分。
func tokenize(t string) []string {
	return strings.Fields(t)
}

// Jaccard 计算两个 token 集合的 Jaccard 相似度。
func Jaccard(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	setA := make(map[string]bool, len(a))
	for _, t := range a {
		setA[t] = true
	}
	intersection := 0
	for _, t := range b {
		if setA[t] {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

// IsDuplicate 判断 b 是否为 a 的重复（基于规范化标题）。
func IsDuplicate(aTitle, bTitle, aLang, bLang string) bool {
	na, nb := NormalizeTitle(aTitle), NormalizeTitle(bTitle)
	if na == "" || nb == "" {
		return false
	}
	if na == nb {
		return true
	}
	threshold := 0.6
	if aLang != bLang {
		threshold = 0.7
	}
	return Jaccard(tokenize(na), tokenize(nb)) >= threshold
}
