// Package fetch 提供 HTTP 获取能力：代理、UA、超时、重试、压缩、robots.txt 尊重。
package fetch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"
)

const maxBody = 10 << 20 // 10 MB 响应上限

// Options 是 Fetcher 的配置。
type Options struct {
	Proxy     string        // 空 = 环境变量代理
	UserAgent string        // 空 = 默认 UA
	Timeout   time.Duration // 单次请求超时
	Retries   int           // 失败重试次数
	NoRobots  bool          // true = 跳过 robots.txt 检查
}

// Fetcher 是可并发使用的 HTTP 获取器。
type Fetcher struct {
	client *http.Client
	opts   Options

	mu     sync.Mutex
	robots map[string]*robotsRule // host -> 规则（含全部 UA 分组）
}

type robotsRule struct {
	groups  map[string][]*regexp.Regexp // user-agent(小写) -> disallow 模式；"*" 为默认组
	fetched time.Time
}

// New 创建 Fetcher。
func New(opts Options) *Fetcher {
	if opts.UserAgent == "" {
		opts.UserAgent = "news-report/1.0 (news aggregator)"
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 15 * time.Second
	}
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		MaxIdleConns:          32,
		MaxIdleConnsPerHost:   4,
		IdleConnTimeout:       60 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: opts.Timeout,
	}
	if opts.Proxy != "" {
		if pu, err := url.Parse(opts.Proxy); err == nil {
			transport.Proxy = http.ProxyURL(pu)
		}
	}
	return &Fetcher{
		client: &http.Client{Transport: transport, Timeout: opts.Timeout},
		opts:   opts,
		robots: map[string]*robotsRule{},
	}
}

// Bytes 获取 URL 内容（遵循重试与 robots 策略，使用默认 UA）。
func (f *Fetcher) Bytes(ctx context.Context, rawURL string) ([]byte, error) {
	return f.bytesWithUA(ctx, rawURL, f.opts.UserAgent)
}

// BytesUA 获取 URL 内容，使用指定 UA（用于站点显式白名单的抓取，如 UN 的 Feedfetcher-Google）。
func (f *Fetcher) BytesUA(ctx context.Context, rawURL, ua string) ([]byte, error) {
	return f.bytesWithUA(ctx, rawURL, ua)
}

func (f *Fetcher) bytesWithUA(ctx context.Context, rawURL, ua string) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("非法 URL %q: %w", rawURL, err)
	}
	if !f.opts.NoRobots && u.Scheme != "" {
		allowed, known := f.allowed(u, ua)
		if known && !allowed {
			return nil, fmt.Errorf("robots.txt 禁止抓取 %s", u.Path)
		}
	}

	var lastErr error
	for attempt := 0; attempt <= f.opts.Retries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(300*(1<<(attempt-1))) * time.Millisecond
			backoff += time.Duration(rand.Intn(150)) * time.Millisecond
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}
		body, err := f.tryOnce(ctx, u, ua)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !IsRetriable(err) {
			return nil, err
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	return nil, lastErr
}

// Request 构造带默认头部的请求（供需要自定义头的调用方使用）。
func (f *Fetcher) Request(ctx context.Context, rawURL string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", f.opts.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	// Accept-Encoding 同上：交给 transport 自动处理。
	return req, nil
}

// Do 执行请求（状态码校验由调用方负责）。
func (f *Fetcher) Do(req *http.Request) (*http.Response, error) {
	return f.client.Do(req)
}

func (f *Fetcher) tryOnce(ctx context.Context, u *url.URL, ua string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml, text/xml, text/html;q=0.9, */*;q=0.5")
	// 注意：不手动设置 Accept-Encoding——Go transport 会自动加 gzip 并透明解压；
	// 手动设置会禁用自动解压，导致拿到压缩二进制。

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}
	if resp.StatusCode >= 400 {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, err
	}
	if len(data) >= maxBody {
		return nil, errors.New("响应超过 10MB 上限")
	}
	return data, nil
}

// ── robots.txt ────────────────────────────────────────────────

var robotsCacheTTL = 24 * time.Hour

// allowed 返回 (是否允许, 是否可判定)。robots.txt 获取失败视为允许（对只读抓取更宽容）。
func (f *Fetcher) allowed(u *url.URL, ua string) (bool, bool) {
	host := strings.ToLower(u.Host)
	f.mu.Lock()
	rule, ok := f.robots[host]
	known := ok && rule != nil
	f.mu.Unlock()

	if known && time.Since(rule.fetched) < robotsCacheTTL {
		return f.checkRule(rule, u, ua)
	}

	rule = f.fetchRobots(host, u.Scheme)
	f.mu.Lock()
	f.robots[host] = rule
	f.mu.Unlock()
	if rule == nil {
		return true, false // 无法判定 → 允许
	}
	return f.checkRule(rule, u, ua)
}

// checkRule 按 UA 分组匹配 robots 规则（精确 UA 优先，其次 *）。
func (f *Fetcher) checkRule(rule *robotsRule, u *url.URL, ua string) (bool, bool) {
	patterns := f.patternsFor(rule, ua)
	for _, re := range patterns {
		if re.MatchString(u.Path) {
			return false, true
		}
	}
	return true, true
}

func (f *Fetcher) patternsFor(rule *robotsRule, ua string) []*regexp.Regexp {
	if rule == nil || rule.groups == nil {
		return nil
	}
	// robots 匹配规则：User-Agent token（第一个空白前的部分，忽略 /version）
	token := strings.ToLower(strings.Fields(ua)[0])
	if i := strings.IndexByte(token, '/'); i > 0 {
		token = token[:i]
	}
	if g, ok := rule.groups[token]; ok {
		return g
	}
	if g, ok := rule.groups["*"]; ok {
		return g
	}
	return nil
}

func (f *Fetcher) fetchRobots(host, scheme string) *robotsRule {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if scheme == "" {
		scheme = "https"
	}
	u := scheme + "://" + host + "/robots.txt"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", f.opts.UserAgent)
	resp, err := f.client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
	if err != nil {
		return nil
	}
	return parseRobots(string(data))
}

// parseRobots 解析 robots.txt：按 User-agent 分组存储 Disallow 模式。
func parseRobots(body string) *robotsRule {
	rule := &robotsRule{fetched: time.Now(), groups: map[string][]*regexp.Regexp{}}
	group := "*"
	for _, rawLine := range strings.Split(body, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		val := strings.TrimSpace(parts[1])
		switch key {
		case "user-agent":
			group = strings.ToLower(val)
			if _, ok := rule.groups[group]; !ok {
				rule.groups[group] = nil
			}
		case "disallow":
			if val == "" {
				continue // 空 Disallow = 允许全部
			}
			if re := globToRegexp(val); re != nil {
				if rule.groups[group] == nil {
					rule.groups[group] = []*regexp.Regexp{}
				}
				rule.groups[group] = append(rule.groups[group], re)
			}
		}
	}
	return rule
}

// globToRegexp 把 robots 模式（支持 * 与 $）转成正则。
func globToRegexp(pattern string) *regexp.Regexp {
	var sb strings.Builder
	sb.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*':
			sb.WriteString(".*")
		case '$':
			sb.WriteString("$")
		default:
			sb.WriteString(regexp.QuoteMeta(string(pattern[i])))
		}
	}
	re, err := regexp.Compile(sb.String())
	if err != nil {
		return nil
	}
	return re
}

// ── 工具 ──────────────────────────────────────────────────────

// IsRetriable 判断错误是否值得重试（网络/5xx/超时）。
func IsRetriable(err error) bool {
	if err == nil {
		return false
	}
	var uerr *url.Error
	if errors.As(err, &uerr) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "HTTP 5") || strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "connection") || strings.Contains(msg, "EOF")
}

// ResolveURL 把相对地址解析为绝对地址。
func ResolveURL(base, ref string) string {
	bu, err := url.Parse(base)
	if err != nil {
		return ref
	}
	ru, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return bu.ResolveReference(ru).String()
}

// LangHeader 为指定语言设置 Accept-Language。
func LangHeader(lang string) string {
	switch lang {
	case "de":
		return "de-DE,de;q=0.9,en;q=0.8"
	case "fr":
		return "fr-FR,fr;q=0.9,en;q=0.8"
	default:
		return "en-US,en;q=0.9,de;q=0.5,fr;q=0.5"
	}
}

// StripHTMLTags 移除字符串中的 HTML 标签（用于 feed 摘要清洗）。
var tagRe = regexp.MustCompile(`<[^>]+>`)

func StripHTMLTags(s string) string {
	return strings.TrimSpace(tagRe.ReplaceAllString(s, ""))
}

// CleanPath 是 path.Clean 的导出包装（供外部使用）。
func CleanPath(p string) string { return path.Clean(p) }

var _ = os.Getenv // 保留 os 导入（未来可能读取 NEWS_REPORT_NO_ROBOTS）
