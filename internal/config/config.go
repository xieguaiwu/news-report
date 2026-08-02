// Package config 管理 news-report 的运行时配置。
// 优先级：内置默认值 < 用户配置文件 (~/.config/news-report/config.yaml 或 --config 指定) < CLI 标志。
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	DefaultConfigPath = "~/.config/news-report/config.yaml"
	DefaultCacheDir   = "~/.cache/news-report"
	Version           = "0.1.0"
)

// SourceOverride 允许用户针对单个来源做覆盖（不需要删掉整个内置注册表）。
type SourceOverride struct {
	Enabled *bool    `yaml:"enabled"` // nil = 保持内置默认
	Weight  *float64 `yaml:"weight"`  // nil = 保持内置默认
	Feeds   []string `yaml:"feeds"`   // 非空时替换内置 feed 列表（按顺序尝试）
}

// Config 是完整运行配置。yaml.v3 反序列化时只覆盖文件中出现的字段。
type Config struct {
	Languages   []string                  `yaml:"languages"`  // en / de / fr
	Categories  []string                  `yaml:"categories"` // politics / economy / industry
	Minutes     int                       `yaml:"minutes"`    // 新鲜度窗口（分钟），0 = 不限窗口
	LimitPerCat int                       `yaml:"limit_per_category"`
	TotalLimit  int                       `yaml:"total_limit"`
	Concurrency int                       `yaml:"concurrency"`
	TimeoutSec  int                       `yaml:"timeout_seconds"`
	Retries     int                       `yaml:"retries"`
	HalflifeH   float64                   `yaml:"halflife_hours"` // 新鲜度衰减半衰期
	Proxy       string                    `yaml:"proxy"`          // 留空 = 使用环境变量代理
	UserAgent   string                    `yaml:"user_agent"`
	CacheDir    string                    `yaml:"cache_dir"`
	StoreDays   int                       `yaml:"store_days"` // 已读记录保留天数
	ShowSeen    bool                      `yaml:"show_seen"`  // true = 重复显示已报告过的条目
	StrictFocus bool                      `yaml:"strict_focus"`
	GoogleNews  bool                      `yaml:"google_news"` // 额外广度来源（谷歌新闻聚合）
	Fulltext    int                       `yaml:"fulltext"`    // 每个分类抓取全文的条数（0 = 不抓）
	FulltextMax int                       `yaml:"fulltext_max_chars"`
	ReportTitle string                    `yaml:"report_title"`
	Sources     map[string]SourceOverride `yaml:"sources"`
}

// Default 返回内置默认配置。所有字段均有合理值，保证开箱即用。
func Default() *Config {
	return &Config{
		Languages:   []string{"en", "de", "fr"},
		Categories:  []string{"politics", "economy", "industry"},
		Minutes:     1440, // 24 小时
		LimitPerCat: 12,
		TotalLimit:  80,
		Concurrency: 12,
		TimeoutSec:  15,
		Retries:     2,
		HalflifeH:   12,
		Proxy:       "",
		UserAgent:   "news-report/" + Version + " (news aggregator; +https://github.com/news-report)",
		CacheDir:    DefaultCacheDir,
		StoreDays:   7,
		ShowSeen:    false,
		StrictFocus: false,
		GoogleNews:  false,
		Fulltext:    0,
		FulltextMax: 3000,
		ReportTitle: "News Report",
		Sources:     map[string]SourceOverride{},
	}
}

// expandPath 把 ~ 展开成家目录。
func expandPath(p string) string {
	if p == "~" || len(p) > 1 && p[:2] == "~/" {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

// Load 读取配置文件（路径为空时用默认路径）。
// 文件不存在时返回默认配置；文件存在但解析失败时返回错误。
func Load(path string) (*Config, error) {
	cfg := Default()
	if path == "" {
		path = DefaultConfigPath
	}
	path = expandPath(path)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return nil, fmt.Errorf("读取配置 %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析配置 %s: %w", path, err)
	}
	return cfg, nil
}

// Save 把配置写入指定路径（原子写：先写临时文件再改名）。
func (c *Config) Save(path string) error {
	if path == "" {
		path = DefaultConfigPath
	}
	path = expandPath(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// CachePath 返回缓存目录（确保目录存在）。
func (c *Config) CachePath() string {
	p := expandPath(c.CacheDir)
	_ = os.MkdirAll(p, 0o755)
	return p
}

// Window 返回新鲜度窗口时长。
func (c *Config) Window() time.Duration {
	if c.Minutes <= 0 {
		return 0
	}
	return time.Duration(c.Minutes) * time.Minute
}

// Validate 检查配置合法性，返回第一个错误。
func (c *Config) Validate() error {
	if len(c.Languages) == 0 {
		return errors.New("languages 不能为空")
	}
	for _, l := range c.Languages {
		switch l {
		case "en", "de", "fr":
		default:
			return fmt.Errorf("不支持的语言 %q（仅支持 en/de/fr）", l)
		}
	}
	if len(c.Categories) == 0 {
		return errors.New("categories 不能为空")
	}
	for _, cat := range c.Categories {
		switch cat {
		case "politics", "economy", "industry":
		default:
			return fmt.Errorf("不支持的分类 %q（仅支持 politics/economy/industry）", cat)
		}
	}
	if c.TimeoutSec <= 0 {
		return errors.New("timeout_seconds 必须 > 0")
	}
	if c.Retries < 0 || c.Retries > 5 {
		return errors.New("retries 必须在 0-5 之间")
	}
	if c.Concurrency <= 0 || c.Concurrency > 64 {
		return errors.New("concurrency 必须在 1-64 之间")
	}
	if c.HalflifeH <= 0 {
		return errors.New("halflife_hours 必须 > 0")
	}
	if c.LimitPerCat < 0 || c.TotalLimit < 0 {
		return errors.New("limit_per_category / total_limit 不能为负数")
	}
	return nil
}
