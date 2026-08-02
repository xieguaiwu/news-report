package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	c := Default()
	if c.Validate() != nil {
		t.Fatalf("默认配置应合法: %v", c.Validate())
	}
	if len(c.Languages) != 4 {
		t.Errorf("默认语言应为 4 种（en/de/fr/zh），实际 %v", c.Languages)
	}
	if len(c.Categories) != 4 || c.Categories[0] != "uspolitics" {
		t.Errorf("默认分类应为 4 种且 uspolitics 在前，实际 %v", c.Categories)
	}
	if c.Minutes != 1440 {
		t.Errorf("默认窗口应为 1440 分钟")
	}
	if c.Window().Hours() != 24 {
		t.Errorf("Window() 应为 24h")
	}
}

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "nope.yaml"))
	if err != nil {
		t.Fatalf("缺失文件应返回默认配置: %v", err)
	}
	if c.Minutes != 1440 {
		t.Error("应返回默认配置")
	}
}

func TestLoadAndOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `languages: [en, de]
minutes: 60
limit_per_category: 5
sources:
  bbc-world:
    enabled: false
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(c.Languages) != 2 || c.Languages[0] != "en" || c.Languages[1] != "de" {
		t.Errorf("语言覆盖失败: %v", c.Languages)
	}
	if c.Minutes != 60 {
		t.Errorf("minutes 覆盖失败: %d", c.Minutes)
	}
	if c.LimitPerCat != 5 {
		t.Errorf("limit 覆盖失败: %d", c.LimitPerCat)
	}
	// 未覆盖字段保持默认
	if c.HalflifeH != 12 {
		t.Errorf("未覆盖字段应保持默认: %v", c.HalflifeH)
	}
	ov, ok := c.Sources["bbc-world"]
	if !ok || ov.Enabled == nil || *ov.Enabled {
		t.Errorf("sources 覆盖失败（应解析为 enabled=false）: %+v", ov)
	}
}

func TestSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")
	c := Default()
	c.Minutes = 30
	c.Sources["bbc-world"] = SourceOverride{Enabled: boolPtr(false)}
	if err := c.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	c2, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c2.Minutes != 30 {
		t.Errorf("round-trip 失败: %d", c2.Minutes)
	}
	if c2.Sources["bbc-world"].Enabled == nil || *c2.Sources["bbc-world"].Enabled {
		t.Error("round-trip sources 覆盖失败")
	}
}

func TestValidateErrors(t *testing.T) {
	c := Default()
	c.Languages = []string{"ja"}
	if c.Validate() == nil {
		t.Error("不支持的语言应报错")
	}
	c = Default()
	c.Categories = []string{"sports"}
	if c.Validate() == nil {
		t.Error("不支持的分类应报错")
	}
	c = Default()
	c.TimeoutSec = 0
	if c.Validate() == nil {
		t.Error("timeout 为 0 应报错")
	}
	c = Default()
	c.Concurrency = 100
	if c.Validate() == nil {
		t.Error("concurrency 超限应报错")
	}
}

func boolPtr(b bool) *bool { return &b }
