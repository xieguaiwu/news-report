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

// ── LLM 配置测试 ────────────────────────────────────────────────

func TestDefaultLLMConfig(t *testing.T) {
	c := Default()
	if c.LLM.BaseURL != "https://api.deepseek.com/v1" {
		t.Errorf("默认 BaseURL 应为 DeepSeek，实际 %q", c.LLM.BaseURL)
	}
	if c.LLM.Model != "deepseek-chat" {
		t.Errorf("默认 Model 应为 deepseek-chat，实际 %q", c.LLM.Model)
	}
	if c.LLM.TargetLang != "zh" {
		t.Errorf("默认 TargetLang 应为 zh，实际 %q", c.LLM.TargetLang)
	}
	if c.LLM.TimeoutSec != 60 {
		t.Errorf("默认 TimeoutSec 应为 60，实际 %d", c.LLM.TimeoutSec)
	}
	if c.LLM.MaxChars != 4000 {
		t.Errorf("默认 MaxChars 应为 4000，实际 %d", c.LLM.MaxChars)
	}
	// APIKey 默认为 {env:DEEPSEEK_API_KEY} 引用（未设置环境变量时解析为空）
	if c.LLM.APIKey != "{env:DEEPSEEK_API_KEY}" {
		t.Errorf("默认 APIKey 应引用环境变量，实际 %q", c.LLM.APIKey)
	}
}

func TestLLMTimeoutValidation(t *testing.T) {
	c := Default()
	c.LLM.TimeoutSec = 400
	if err := c.Validate(); err == nil {
		t.Error("TimeoutSec=400 应报错")
	}
	c.LLM.TimeoutSec = -1
	if err := c.Validate(); err == nil {
		t.Error("TimeoutSec=-1 应报错")
	}
	c.LLM.TimeoutSec = 60
	if err := c.Validate(); err != nil {
		t.Errorf("TimeoutSec=60 应合法: %v", err)
	}
}

func TestLLMMaxCharsValidation(t *testing.T) {
	c := Default()
	c.LLM.MaxChars = -1
	if err := c.Validate(); err == nil {
		t.Error("MaxChars=-1 应报错")
	}
	c.LLM.MaxChars = 0
	if err := c.Validate(); err != nil {
		t.Errorf("MaxChars=0 应合法: %v", err)
	}
}

func TestResolveEnvRefs(t *testing.T) {
	os.Setenv("TEST_LLM_KEY", "sk-test-key-123")
	os.Setenv("TEST_LLM_URL", "https://custom.api.com/v1")
	defer os.Unsetenv("TEST_LLM_KEY")
	defer os.Unsetenv("TEST_LLM_URL")

	c := &Config{}
	c.LLM.APIKey = "{env:TEST_LLM_KEY}"
	c.LLM.BaseURL = "{env:TEST_LLM_URL}"
	c.resolveEnvRefs()

	if c.LLM.APIKey != "sk-test-key-123" {
		t.Errorf("APIKey 解析失败: %q", c.LLM.APIKey)
	}
	if c.LLM.BaseURL != "https://custom.api.com/v1" {
		t.Errorf("BaseURL 解析失败: %q", c.LLM.BaseURL)
	}
}

func TestResolveEnvMissing(t *testing.T) {
	c := &Config{}
	c.LLM.APIKey = "{env:NONEXISTENT_VAR_12345}"
	c.resolveEnvRefs()
	if c.LLM.APIKey != "" {
		t.Errorf("不存在的环境变量应替换为空，实际 %q", c.LLM.APIKey)
	}
}

func TestLLMParamInConfigFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `llm:
  base_url: "https://my-api.com/v1"
  model: "gpt-4o"
  target_lang: "en"
  timeout: 120
  max_chars: 2000
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.LLM.BaseURL != "https://my-api.com/v1" {
		t.Errorf("BaseURL 覆盖失败: %q", c.LLM.BaseURL)
	}
	if c.LLM.Model != "gpt-4o" {
		t.Errorf("Model 覆盖失败: %q", c.LLM.Model)
	}
	if c.LLM.TargetLang != "en" {
		t.Errorf("TargetLang 覆盖失败: %q", c.LLM.TargetLang)
	}
	if c.LLM.TimeoutSec != 120 {
		t.Errorf("TimeoutSec 覆盖失败: %d", c.LLM.TimeoutSec)
	}
	if c.LLM.MaxChars != 2000 {
		t.Errorf("MaxChars 覆盖失败: %d", c.LLM.MaxChars)
	}
	// 未指定字段应保持默认（经 resolveEnv 解析后为实际环境变量值）
	if c.LLM.APIKey == "" {
		t.Error("未设置的 APIKey 应从默认 {env:DEEPSEEK_API_KEY} 解析为环境变量值")
	}
}
