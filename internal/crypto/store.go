package crypto

import (
	"bufio"
	"bytes"
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

// ReadJSONL 读取 JSONL 文件。空文件返回空切片、不报错。
func ReadJSONL(path string) ([]Row, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 8<<20)
	var out []Row
	line := 0
	for sc.Scan() {
		line++
		b := bytes.TrimSpace(sc.Bytes())
		if len(b) == 0 {
			continue
		}
		var r Row
		if err := json.Unmarshal(b, &r); err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		out = append(out, r)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}
	return out, nil
}

// WriteJSONLAtomic 以「临时文件 + rename」原子替换写入。补打分必须用这个，
// 不能用 AppendJSONL——否则同一批会被追加两遍。
func WriteJSONLAtomic(path string, rows []Row) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("create tmp: %w", err)
	}
	w := bufio.NewWriter(f)
	enc := json.NewEncoder(w)
	for _, r := range rows {
		if err := enc.Encode(r); err != nil {
			_ = f.Close()
			_ = os.Remove(tmp)
			return fmt.Errorf("encode: %w", err)
		}
	}
	if err := w.Flush(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("flush: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("sync: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("close: %w", err)
	}
	return os.Rename(tmp, path)
}

// RowToItem 把已落盘的行还原成待打分的观测，供 --score-only 使用。
func RowToItem(r Row) AttentionItem {
	return AttentionItem{
		ObservedAt: r.EventAt,
		Chain:      r.Chain,
		TokenAddr:  r.TokenAddr,
		Symbol:     r.Symbol,
		Source:     r.Source,
		Kind:       r.Kind,
		Title:      r.Title,
		Text:       r.Text,
		URL:        r.URL,
		Metrics:    r.Metrics,
	}
}

// LoadTgOffset 读取 Telegram getUpdates 的 offset。文件不存在返回 0。
func LoadTgOffset(path string) (int64, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	var st struct {
		Offset int64 `json:"offset"`
	}
	if err := json.Unmarshal(b, &st); err != nil {
		return 0, fmt.Errorf("tg_offset parse: %w", err)
	}
	return st.Offset, nil
}

// SaveTgOffset 原子写 offset。不落盘会导致同一批频道消息被每轮重复追加，
// 使「提及量」被轮次频率放大（见计划 §9 P0-6）。
func SaveTgOffset(path string, offset int64) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(struct {
		Offset int64 `json:"offset"`
	}{offset})
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
