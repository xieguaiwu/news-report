package astock

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

// sym 市场前缀（与 SHARK 对齐：sz000001 / sh600000 / bj920992）。
const (
	MarketSH = "sh" // 上海：600/601/603/605/688/689
	MarketSZ = "sz" // 深圳：000/001/002/003/300/301
	MarketBJ = "bj" // 北交所：43/83/87/92
)

// stockListCacheTTL 是 data/stock_list.csv 缓存的有效期。
const stockListCacheTTL = 7 * 24 * time.Hour

// StockEntry 是全量股票列表中的一条。
type StockEntry struct {
	Code  string // 6 位数字代码
	Name  string // A 股简称（如 平安银行）
	Market string // sh|sz|bj
	OrgID string // 巨潮 orgId（公告接口用；可为空）
}

// Resolver 持有全量股票列表，提供 代码/简称 → sym 解析。
type Resolver struct {
	byCode map[string]StockEntry
	byName map[string]StockEntry
}

// NewResolver 由条目构造（测试注入用）。
func NewResolver(entries []StockEntry) *Resolver {
	r := &Resolver{byCode: map[string]StockEntry{}, byName: map[string]StockEntry{}}
	for _, e := range entries {
		if e.Code == "" {
			continue
		}
		if e.Market == "" {
			e.Market = marketOfCode(e.Code)
		}
		r.byCode[e.Code] = e
		if e.Name != "" {
			r.byName[normalizeName(e.Name)] = e
		}
	}
	return r
}

// OrgID 返回 sym 对应的巨潮 orgId（无则空串）。
func (r *Resolver) OrgID(sym string) string {
	_, code, ok := SplitSym(sym)
	if !ok {
		return ""
	}
	return r.byCode[code].OrgID
}

// Len 返回列表条数。
func (r *Resolver) Len() int { return len(r.byCode) }

// Resolve 把用户输入（代码/简称/带市场前缀代码）解析为规范 sym。
// 规则：
//  1. 显式前缀（sz/sh/bj + 6 位数字）→ 直接采用（显式声明权威，允许列表外代码）；
//  2. 纯 6 位数字 → 先查列表（列表市场优先），无则按代码段推断市场；
//  3. 简称 → 精确匹配，再退化匹配（去 *、ST 前缀、退市尾标）。
func (r *Resolver) Resolve(input string) (string, error) {
	n := normalizeInput(input)
	if n == "" {
		return "", fmt.Errorf("astock: 空的股票代码/简称")
	}
	// 1) 显式市场前缀
	for _, m := range []string{MarketSH, MarketSZ, MarketBJ} {
		if strings.HasPrefix(n, m) && isCode6(n[len(m):]) {
			return m + n[len(m):], nil
		}
	}
	// 2) 纯代码
	if isCode6(n) {
		if e, ok := r.byCode[n]; ok && e.Market != "" {
			return e.Market + n, nil
		}
		if m := marketOfCode(n); m != "" {
			return m + n, nil
		}
		return "", fmt.Errorf("astock: 无法识别代码 %q（不在股票列表且市场不明）", input)
	}
	// 3）简称：先精确，再退化匹配（去 *、ST 前缀、退市尾标）
	if e, ok := r.byName[n]; ok {
		return e.Market + e.Code, nil
	}
	if e, ok := r.byName[normalizeName(n)]; ok {
		return e.Market + e.Code, nil
	}
	return "", fmt.Errorf("astock: 无法识别股票简称 %q", input)
}

// normalizeInput 去空白（含全角）、小写化。
func normalizeInput(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
}

// normalizeName 简称匹配键：去空白、去 *、小写、去 ST/st 前缀与「退」尾标。
func normalizeName(s string) string {
	n := normalizeInput(s)
	n = strings.ReplaceAll(n, "*", "")
	n = strings.TrimSuffix(n, "退")
	n = strings.TrimPrefix(n, "st")
	return n
}

// isCode6 是否 6 位纯数字。
func isCode6(s string) bool {
	if len(s) != 6 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// marketOfCode 按代码段推断市场（列表缺失时的兜底规则）。
func marketOfCode(code string) string {
	switch {
	case strings.HasPrefix(code, "6"):
		return MarketSH
	case strings.HasPrefix(code, "0"), strings.HasPrefix(code, "3"):
		return MarketSZ
	case strings.HasPrefix(code, "43"), strings.HasPrefix(code, "83"),
		strings.HasPrefix(code, "87"), strings.HasPrefix(code, "92"):
		return MarketBJ
	}
	return ""
}

// SplitSym 把 "sz000001" 拆成 ("sz","000001")。输入须为规范 sym（小写）。
func SplitSym(sym string) (market, code string, ok bool) {
	if len(sym) != 8 {
		return "", "", false
	}
	m, c := sym[:2], sym[2:]
	if !isCode6(c) {
		return "", "", false
	}
	switch m {
	case MarketSH, MarketSZ, MarketBJ:
		return m, c, true
	}
	return "", "", false
}

// SecID 返回东方财富 secid（1.=沪，0.=深/北），供 getListInfo 接口使用。
func SecID(sym string) (string, error) {
	m, code, ok := SplitSym(sym)
	if !ok {
		return "", fmt.Errorf("astock: 非法 sym %q（应为 sh/sz/bj + 6 位代码）", sym)
	}
	switch m {
	case MarketSH:
		return "1." + code, nil
	case MarketSZ, MarketBJ:
		return "0." + code, nil
	}
	return "", fmt.Errorf("astock: 未知市场 %q", m)
}

// ── 全量股票列表：缓存 + 远端 ─────────────────────────────────

// LoadStockList 加载全量股票列表：优先读 cachePath 缓存（stockListCacheTTL 内有效）；
// 过期/缺失时拉取远端（主源：巨潮 szse_stock.json 单请求全量含 orgId；
// 备源：东方财富 clist 分页，无 orgId）并写回缓存。远端全失败但存在旧缓存时
// 仍用旧缓存（降级）。
func LoadStockList(cachePath string, src *Source) (*Resolver, error) {
	if entries, mtime, ok := loadStockListCache(cachePath); ok && time.Since(mtime) < stockListCacheTTL {
		return NewResolver(entries), nil
	}
	var entries []StockEntry
	var errs []error
	if src != nil {
		e, err := src.FetchStockListCNInfo(context.Background())
		if err == nil {
			entries = e
		} else {
			errs = append(errs, fmt.Errorf("巨潮列表: %w", err))
			e2, err2 := src.FetchStockListEM(context.Background())
			if err2 == nil {
				entries = e2
			} else {
				errs = append(errs, fmt.Errorf("东财列表: %w", err2))
			}
		}
	}
	if len(entries) == 0 {
		// 降级：用旧缓存（若有）
		if entries2, _, ok := loadStockListCache(cachePath); ok {
			return NewResolver(entries2), errors.Join(append(errs, errors.New("astock: 远端列表失败，使用旧缓存降级"))...)
		}
		return nil, errors.Join(errs...)
	}
	if err := saveStockListCache(cachePath, entries); err != nil {
		// 缓存写失败不影响解析
		if src != nil {
			src.logf("astock: 写股票列表缓存失败: %v", err)
		}
	}
	return NewResolver(entries), nil
}

// loadStockListCache 读 CSV 缓存：code,name,market,orgid。
func loadStockListCache(path string) ([]StockEntry, time.Time, bool) {
	f, err := os.Open(path)
	if err != nil {
		return nil, time.Time{}, false
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, time.Time{}, false
	}
	rd := csv.NewReader(f)
	rd.FieldsPerRecord = -1
	records, err := rd.ReadAll()
	if err != nil || len(records) < 1 {
		return nil, time.Time{}, false
	}
	start := 0
	if len(records[0]) >= 2 && records[0][0] == "code" {
		start = 1
	}
	var entries []StockEntry
	for _, rec := range records[start:] {
		if len(rec) < 3 || !isCode6(rec[0]) {
			continue
		}
		e := StockEntry{Code: rec[0], Name: rec[1], Market: rec[2]}
		if len(rec) >= 4 {
			e.OrgID = rec[3]
		}
		if e.Market == "" {
			e.Market = marketOfCode(e.Code)
		}
		entries = append(entries, e)
	}
	return entries, fi.ModTime(), len(entries) > 0
}

// saveStockListCache 写 CSV 缓存（原子替换）。
func saveStockListCache(path string, entries []StockEntry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	w := csv.NewWriter(f)
	_ = w.Write([]string{"code", "name", "market", "orgid"})
	for _, e := range entries {
		_ = w.Write([]string{e.Code, e.Name, e.Market, e.OrgID})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}


// FetchStockListCNInfo 主源：巨潮 szse_stock.json（单请求全量，含 orgId）。
// 过滤 category ∈ {A股, CDR}（排除 B股 900/200）。
func (s *Source) FetchStockListCNInfo(ctx context.Context) ([]StockEntry, error) {
	body, err := s.get(ctx, s.cnStockListURL())
	if err != nil {
		return nil, fmt.Errorf("astock: 巨潮股票列表失败: %w", err)
	}
	var resp struct {
		StockList []struct {
			Code     string `json:"code"`
			Category string `json:"category"`
			OrgID    string `json:"orgId"`
			ZWJC     string `json:"zwjc"`
		} `json:"stockList"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("astock: 巨潮股票列表解析失败: %w", err)
	}
	var out []StockEntry
	for _, it := range resp.StockList {
		if it.Category != "A股" && it.Category != "CDR" {
			continue
		}
		out = append(out, StockEntry{Code: it.Code, Name: it.ZWJC, Market: marketOfCode(it.Code), OrgID: it.OrgID})
	}
	if len(out) == 0 {
		return nil, errors.New("astock: 巨潮股票列表为空")
	}
	return out, nil
}

// FetchStockListEM 备源：东方财富 clist 全量（分页，pz 上限 100/页，无 orgId）。
func (s *Source) FetchStockListEM(ctx context.Context) ([]StockEntry, error) {
	var out []StockEntry
	for pn := 1; pn <= 200; pn++ {
		u := fmt.Sprintf("%s?pn=%d&pz=100&po=1&np=1&fltt=2&invt=2&fid=f12&fs=%s&fields=f12,f14",
			s.emCList(), pn, url.QueryEscape("m:0+t:6,m:0+t:80,m:1+t:2,m:1+t:23,m:0+t:81+s:2048"))
		body, err := s.get(ctx, u)
		if err != nil {
			if pn == 1 {
				return nil, fmt.Errorf("astock: 东财股票列表失败: %w", err)
			}
			break // 后续页失败：用已取到的
		}
		var resp struct {
			Data struct {
				Total int `json:"total"`
				Diff  []struct {
					Code string `json:"f12"`
					Name string `json:"f14"`
				} `json:"diff"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			break
		}
		for _, it := range resp.Data.Diff {
			if isCode6(it.Code) {
				out = append(out, StockEntry{Code: it.Code, Name: it.Name, Market: marketOfCode(it.Code)})
			}
		}
		if len(resp.Data.Diff) == 0 || len(out) >= resp.Data.Total {
			break
		}
	}
	if len(out) == 0 {
		return nil, errors.New("astock: 东财股票列表为空")
	}
	return out, nil
}
