验证完成。所有测试通过代理 `127.0.0.1:7897` 执行，UA 为 Mozilla/5.0。

## 验证结果

### 台湾媒体（zh）

| URL | HTTP | content-type | 条目 | 可用 |
|---|---|---|---|---|
| `https://www.cna.com.tw/rss/aipl.xml`（及 firstnews/world/finance.xml、/rss/） | 404 | text/html | — | ❌ 路径已废弃 |
| `https://feeds.feedburner.com/rsscna/politics`（两岸/政治） | 200 | text/xml | 20 | ✅ |
| `https://feeds.feedburner.com/rsscna/intworld`（国际） | 200 | text/xml | 20 | ✅ |
| `https://feeds.feedburner.com/rsscna/mainland`（大陆） | 200 | text/xml | 20 | ✅ |
| `https://feeds.feedburner.com/rsscna/finance`（财经） | 200 | text/xml | 20 | ✅ |
| `https://www.rti.org.tw/rss/`、`/rss/aipl`、`/rss/news/world` 等 | 403 | text/html | — | ❌ 站点级 WAF 拦截 |
| `https://udn.com/rss/`、`/rss/breakingnews/1` | 404 | text/html | — | ❌ 联合报 RSS 已停用 |
| `https://news.ltn.com.tw/rss/politics.xml` | 200 | application/xml | 9 | ✅ |
| `https://news.ltn.com.tw/rss/world.xml` | 200 | application/xml | 19 | ✅ |
| `https://news.ltn.com.tw/rss/business.xml` | 200 | application/xml | 36 | ✅ |
| `https://news.ltn.com.tw/rss/society.xml` / `life.xml` / `entertainment.xml` | 200 | application/xml | 5–33 | ✅ |
| `https://ctee.com.tw/feed`、`/rss`、`/feed.xml`、`/rss/feed` | 403/404 | text/html/xml | — | ❌ 工商时报 WAF 拦截 |

### 美国政治（uspolitics）

| URL | HTTP | content-type | 条目 | 可用 |
|---|---|---|---|---|
| `https://feeds.npr.org/1014/rss.xml`（NPR Politics） | 200 | text/xml | 10 | ✅ |
| `https://rollcall.com/feed/`（Roll Call；www 版 301 到此） | 200 | application/rss+xml | 10 | ✅ |
| `https://www.politico.com/rss/politics.xml`、`/feed` | 403 | text/html（Cloudflare） | — | ❌ |
| `https://apnews.com/index.rss` | 401 | text/plain | — | ❌ 需凭证（"Invalid client credentials"） |
| `https://apnews.com/hub/politics/feed`、`/politics/feed` | 404 | text/html | — | ❌ |
| `https://axios.com/feed`（→www） | 301→403 | text/html（Cloudflare） | — | ❌ |
| `https://abcnews.go.com/abcnews/topstories`（301→abcnews.com） | 200 | text/xml | 25 | ✅（综合头条，非纯政治） |
| `https://abcnews.go.com/abcnews/politics`、`/abcnews/uspolitics` | 200 | text/html（无 RSS） | — | ❌ 无政治专属 feed |

### 结论

**推荐添加的确切可用 URL（9 个）**：
- 台湾：CNA 走 Feedburner（`rsscna/politics` 两岸政治、`intworld` 国际、`mainland` 大陆、`finance` 财经，另有 `local/social/sport/stars/technology/lifehealth/culture` 可选）；自由时报 `news.ltn.com.tw/rss/{politics,world,business,society}.xml`
- 美国：NPR `feeds.npr.org/1014/rss.xml`、Roll Call `rollcall.com/feed/`、ABC `abcnews.go.com/abcnews/topstories`

**不可用**：中央社旧路径、RTI（WAF）、联合报（RSS 已停）、工商时报（WAF）、Politico（Cloudflare）、AP（公共 RSS 已停，需付费凭证）、Axios（Cloudflare）、ABC 政治专属。

**注意细节**：
- CNA 首页 RSS 入口在 `/about/rss.aspx`，feed 全部托管在 Google Feedburner
- 自由时报 feed 以 UTF-8 BOM（`ef bb bf`）开头，解析器需先剥 BOM；ABC feed 无 XML 声明直接以 `<rss` 开头（均合法，但鲁棒解析器要处理）
- Roll Call 用非 www 域名（www 会 301）