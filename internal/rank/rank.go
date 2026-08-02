// Package rank 实现条目排序：来源权重 × 新鲜度衰减 + 相关性加分。
package rank

import (
	"math"
	"time"
)

// Score 计算条目得分。
//   - weight:    来源权重 (0~1)
//   - kwScore:   分类关键词命中次数
//   - categorized: 是否命中专注分类
//   - age:       距发布时间
//   - halflife:  新鲜度半衰期（默认 12h）
func Score(weight float64, kwScore int, categorized bool, age, halflife time.Duration) float64 {
	if age < 0 {
		age = 0
	}
	if halflife <= 0 {
		halflife = 12 * time.Hour
	}
	fresh := math.Exp(-age.Hours() / halflife.Hours())
	relevance := math.Min(1, float64(kwScore)/3.0)
	score := weight*fresh + 0.15*relevance
	if categorized {
		score += 0.10
	}
	return score
}

// AgeLabel 生成人类可读的时长标签（"2h" / "35m" / "3d"）。
func AgeLabel(age time.Duration) string {
	if age < 0 {
		age = 0
	}
	switch {
	case age < time.Hour:
		m := int(age.Minutes())
		if m < 1 {
			return "now"
		}
		return itoa(m) + "m"
	case age < 24*time.Hour:
		return itoa(int(age.Hours())) + "h"
	default:
		return itoa(int(age.Hours()/24)) + "d"
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [8]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
