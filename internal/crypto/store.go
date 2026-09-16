package crypto

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// AppendJSONL 以追加方式写入 JSONL。文件必须与父目录先存在。
func AppendJSONL(path string, rows []Row) error {
	if len(rows) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	enc := json.NewEncoder(w)
	for _, r := range rows {
		if err := enc.Encode(r); err != nil { // Encode 自带换行
			return fmt.Errorf("encode: %w", err)
		}
	}
	return w.Flush()
}

// DedupRows 按 (source,kind,chain,token_addr,event_at) 去重，保留首次出现。
//
// ⚠️ 去重键不含 collected_at：同一事件在不同采集轮次应视为**同一条**，否则
// 提及量会被轮次频率放大。价格等时间序列状态变化不进 JSONL（见 T13 面板）。
func DedupRows(rows []Row) []Row {
	seen := make(map[string]struct{}, len(rows))
	out := make([]Row, 0, len(rows))
	for _, r := range rows {
		k := r.Source + "|" + r.Kind + "|" + r.Chain + "|" + r.TokenAddr + "|" +
			fmt.Sprint(r.EventAt)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, r)
	}
	return out
}
