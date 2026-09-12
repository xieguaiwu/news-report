// Package astock 为 news-report 提供 A 股舆情分析能力：
//
//   - 数据源（sources.go）：东方财富个股新闻（stock_news_em 型，secid 分辨沪深）、
//     巨潮资讯公告列表（hisAnnouncement 接口）。
//   - 代码/简称规范化（symresolve.go）：输入 → sym（sz000001 形式，与 SHARK 对齐），
//     全量股票列表缓存于 data/stock_list.csv。
//   - LLM 舆情打分（llmscore.go）：OpenAI 兼容客户端，经环境变量注入凭据。
//
// 凭据注入纪律：base_url 与 api key 从 /root/.pi/agent/models.json 的 bai provider
// 条目读取（apiKey 为 $VAR 环境变量引用，回退 /root/.pi/agent/auth.json），仅经
// 环境变量 ASTOCK_LLM_BASE_URL / ASTOCK_LLM_API_KEY 注入，由 scripts/astock_env.sh
// 解析导出——值一律不 echo、不落盘、不提交。
//
// 代理要求：bai 网关直连被封锁，Go net/http 经 http.ProxyFromEnvironment 自动使用
// HTTPS_PROXY（运行前 source /root/.pi/env）。备用通道 dashscope/qwen 仅注释说明，
// 见 ASTOCK_NOTES.md。
package astock

// NewsItem 是从数据源抓到的一条原始新闻/公告（未打分）。
type NewsItem struct {
	Date       string `json:"date"`        // YYYY-MM-DD（北京时间）
	Sym        string `json:"sym"`         // sz000001 规范形
	SourceType string `json:"source_type"` // "news" | "announcement"
	Title      string `json:"title"`
	Text       string `json:"text"` // 正文摘要；公告源为标题
	URL        string `json:"url"`
	Media      string `json:"media,omitempty"` // 媒体名（东财检索补充；不进 JSONL）
}

const (
	SourceTypeNews         = "news"         // 东方财富个股新闻
	SourceTypeAnnouncement = "announcement" // 巨潮资讯公告
)

// Score 是 LLM 打分结果。
type Score struct {
	Tone        int     `json:"tone"`         // -2..2：-2 极度利空 .. 2 极度利多
	Kind        string  `json:"kind"`         // 业绩|监管|重组|传闻|研报|自媒体|其他
	Specificity float64 `json:"specificity"`  // 0..1 信息具体程度
	SourceTier  string  `json:"source_tier"`  // 官方|媒体|自媒体|不明
	BlackScore  float64 `json:"black_score"`  // 0..1 低级黑（论据缺失+情绪化+恐慌/亢奋诱导）
}

// Row 是 astock JSONL 输出的一行。字段顺序按任务规定；
// 未打分行 tone/specificity/black_score 为 null、kind/source_tier 为空串、
// scored_at 为空串；打分通道不可用时 model 标 MISSING_KEY。
type Row struct {
	Date          string   `json:"date"`
	Sym           string   `json:"sym"`
	SourceType    string   `json:"source_type"`
	Title         string   `json:"title"`
	Text          string   `json:"text"`
	URL           string   `json:"url"`
	Tone          *int     `json:"tone"`
	Kind          string   `json:"kind"`
	Specificity   *float64 `json:"specificity"`
	SourceTier    string   `json:"source_tier"`
	BlackScore    *float64 `json:"black_score"`
	ScoredAt      string   `json:"scored_at"`
	Model         string   `json:"model"`
	PromptVersion string   `json:"prompt_version"`
}

// MissingKeyModel 是打分通道不可用（未注入凭据）时 JSONL 的 model 标记。
const MissingKeyModel = "MISSING_KEY"
