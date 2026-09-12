package astock

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// testEntries 离线注入列表（覆盖沪深北 + 名称匹配 + orgId）。
func testEntries() []StockEntry {
	return []StockEntry{
		{Code: "000001", Name: "平安银行", Market: MarketSZ, OrgID: "gssz0000001"},
		{Code: "000002", Name: "万科A", Market: MarketSZ},
		{Code: "000166", Name: "申万宏源", Market: MarketSZ, OrgID: "qsgn0000301"},
		{Code: "002594", Name: "比亚迪", Market: MarketSZ},
		{Code: "300059", Name: "东方财富", Market: MarketSZ, OrgID: "9900010488"},
		{Code: "300750", Name: "宁德时代", Market: MarketSZ},
		{Code: "600000", Name: "浦发银行", Market: MarketSH, OrgID: "gssh0600000"},
		{Code: "601398", Name: "工商银行", Market: MarketSH},
		{Code: "688981", Name: "中芯国际", Market: MarketSH},
		{Code: "920992", Name: "中科美菱", Market: MarketBJ},
		{Code: "000950", Name: "*ST重形", Market: MarketSZ},
	}
}

// TestResolve20Samples 20 组样本：代码/前缀/简称/归一化/错误路径。
func TestResolve20Samples(t *testing.T) {
	r := NewResolver(testEntries())
	cases := []struct {
		in    string
		want  string
		isErr bool
	}{
		// ── 纯代码（列表内，列表市场优先）1-5
		{"000001", "sz000001", false},
		{"600000", "sh600000", false},
		{"000166", "sz000166", false},
		{"300059", "sz300059", false},
		{"688981", "sh688981", false},
		// ── 显式市场前缀（大小写/列表外代码均接受）6-9
		{"SZ000001", "sz000001", false},
		{"sh600000", "sh600000", false},
		{"Bj920992", "bj920992", false},
		{"sh688111", "sh688111", false}, // 列表外，显式前缀权威
		// ── 归一化：空白/换行 10-11
		{"  000166 ", "sz000166", false},
		{"sz\t000001", "sz000001", false},
		// ── 简称精确匹配 12-15
		{"平安银行", "sz000001", false},
		{"申万宏源", "sz000166", false},
		{"东方财富", "sz300059", false},
		{"浦发银行", "sh600000", false},
		// ── 简称退化匹配（去 * / st / 退）16-17
		{"*ST重形", "sz000950", false},
		{"重形", "sz000950", false},
		// ── 列表外纯代码，按代码段推断市场 18-19
		{"601999", "sh601999", false},
		{"300999", "sz300999", false},
		// ── 错误路径 20-24
		{"", "", true},
		{"999999", "", true},  // 9 段推断不出市场且不在列表
		{"xyz", "", true},     // 非法
		{"12345", "", true},   // 非 6 位
		{"不存在股份", "", true}, // 未知简称
	}
	for i, c := range cases {
		got, err := r.Resolve(c.in)
		if c.isErr {
			if err == nil {
				t.Errorf("样本 %d: Resolve(%q) 期望报错，得到 %q", i+1, c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("样本 %d: Resolve(%q) 报错: %v", i+1, c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("样本 %d: Resolve(%q) = %q, want %q", i+1, c.in, got, c.want)
		}
	}
	if len(cases) < 20 {
		t.Fatalf("样本数不足 20: %d", len(cases))
	}
}

// TestSecID secid 沪深分辨。
func TestSecID(t *testing.T) {
	cases := []struct {
		sym, want string
		isErr     bool
	}{
		{"sz000001", "0.000001", false},
		{"sh600000", "1.600000", false},
		{"sz300059", "0.300059", false},
		{"sh688981", "1.688981", false},
		{"bj920992", "0.920992", false},
		{"000001", "", true},
		{"xx000001", "", true},
	}
	for _, c := range cases {
		got, err := SecID(c.sym)
		if c.isErr {
			if err == nil {
				t.Errorf("SecID(%q) 期望报错，得到 %q", c.sym, got)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("SecID(%q) = %q,%v want %q", c.sym, got, err, c.want)
		}
	}
}

// TestOrgID orgId 查询。
func TestOrgID(t *testing.T) {
	r := NewResolver(testEntries())
	if got := r.OrgID("sz000166"); got != "qsgn0000301" {
		t.Errorf("OrgID(sz000166) = %q", got)
	}
	if got := r.OrgID("sh600000"); got != "gssh0600000" {
		t.Errorf("OrgID(sh600000) = %q", got)
	}
	if got := r.OrgID("bad"); got != "" {
		t.Errorf("OrgID(bad) = %q, want 空", got)
	}
}

// TestStockListCacheRoundTrip CSV 缓存读写回路。
func TestStockListCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stock_list.csv")
	if err := saveStockListCache(path, testEntries()); err != nil {
		t.Fatalf("save: %v", err)
	}
	entries, mtime, ok := loadStockListCache(path)
	if !ok {
		t.Fatal("缓存读取失败")
	}
	if mtime.IsZero() || time.Since(mtime) > time.Minute {
		t.Errorf("mtime 异常: %v", mtime)
	}
	r := NewResolver(entries)
	sym, err := r.Resolve("申万宏源")
	if err != nil || sym != "sz000166" {
		t.Errorf("roundtrip resolve = %q, %v", sym, err)
	}
	if got := r.OrgID(sym); got != "qsgn0000301" {
		t.Errorf("roundtrip orgid = %q", got)
	}
}

// TestLoadStockListStaleCache 旧缓存 + 无网络注入 → 降级用旧缓存。
func TestLoadStockListStaleCache(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stock_list.csv")
	if err := saveStockListCache(path, testEntries()); err != nil {
		t.Fatal(err)
	}
	// 把 mtime 拨老
	old := time.Now().Add(-8 * 24 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	// src 指向不可达地址 → 两个远端都失败
	src := NewSource()
	src.EMListAPI = "http://127.0.0.1:1"
	src.EMCListURL = "http://127.0.0.1:1"
	src.CNStockList = "http://127.0.0.1:1"
	r, err := LoadStockList(path, src)
	if err == nil {
		t.Log("远端失败但未返回错误（可接受）")
	}
	if r == nil || r.Len() == 0 {
		t.Fatalf("期望旧缓存降级，得到 r=%v err=%v", r, err)
	}
	if sym, _ := r.Resolve("000001"); sym != "sz000001" {
		t.Errorf("降级解析失败: %q", sym)
	}
}

// TestLoadStockListFreshCache 缓存新鲜时不发网络请求（src 为 nil 也应成功）。
func TestLoadStockListFreshCache(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stock_list.csv")
	if err := saveStockListCache(path, testEntries()); err != nil {
		t.Fatal(err)
	}
	r, err := LoadStockList(path, nil)
	if err != nil {
		t.Fatalf("新鲜缓存 + nil 源应成功: %v", err)
	}
	if r.Len() != len(testEntries()) {
		t.Errorf("条数 = %d, want %d", r.Len(), len(testEntries()))
	}
}
