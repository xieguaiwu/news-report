# 消息源清单（SOURCES.md）

状态说明：
- ✅ 已验证可用（2026-08 实测）
- ⚠️ 候选：可能受 Cloudflare 反爬/网络影响，失败静默降级
- 付费：🔒 软付费墙（RSS 摘要免费，全文需订阅）｜ 免费：全文免费

## EN · 通讯社（wire，权重 1.0）

| ID | 来源 | Feed | 状态 |
|---|---|---|---|
| reuters-world | Reuters World | arc/outboundfeeds/world | ⚠️ 候选 |
| reuters-business | Reuters Business | arc/outboundfeeds/business | ⚠️ 候选 |
| reuters-markets | Reuters Markets | arc/outboundfeeds/markets | ⚠️ 候选 |
| ap-news | AP News | apnews.com/feed | ⚠️ 候选 |

## EN · 老牌媒体（legacy，权重 0.9）

| ID | 来源 | Feed | 状态 |
|---|---|---|---|
| bbc-world / bbc-business / bbc-tech | BBC 三频道 | feeds.bbci.co.uk | ✅ 免费 |
| guardian-world/business/tech | The Guardian 三频道 | theguardian.com | ✅ 免费 |
| nyt-world/business/economy/tech | NYT 四频道 | rss.nytimes.com | ✅ 🔒 |
| wsj-world / wsj-markets | WSJ | feeds.a.dj.com | ✅ 🔒 |
| wsj-tech | WSJ Tech | feeds.a.dj.com | ⚠️ 403 |
| wapo-world / wapo-business | Washington Post | feeds.washingtonpost.com | ✅ 较慢 |
| economist-finance | The Economist | economist.com/rss | ⚠️ 🔒 |

## EN · 机构/智库/政治（specialist，权重 1.0）

| ID | 来源 | Feed | 状态 |
|---|---|---|---|
| npr-politics | NPR Politics | feeds.npr.org/1014 | ✅ 免费 |
| rollcall | Roll Call（国会） | rollcall.com/feed | ✅ 免费 |
| abc-news | ABC News 头条 | abcnews.go.com | ✅ 综合 |
| thehill | The Hill | thehill.com/feed | ✅ 免费 |
| politico-eu | Politico Europe | politico.eu/feed | ⚠️ 403 |
| euractiv | EURACTIV（欧盟政策） | euractiv.com/feed | ⚠️ 403 |
| brookings | Brookings 智库 | RSS 已停 → scrape 兜底 | ✅ 免费 |
| cfr / piie / csis / chathamhouse | 外交/经济/安全智库 | /rss.xml | ⚠️ 候选 |
| un-news | UN News | feed/subscribe + Feedfetcher UA | ✅ 免费 |
| eia | 美国能源署 | robots 自禁 /rss | ⚠️ --no-robots |
| imf / fed / ecb | IMF/美联储/ECB | 各官网 | ⚠️ imf/ecb 候选 |

## DE · 老牌媒体（legacy）

| ID | 来源 | Feed | 状态 |
|---|---|---|---|
| dw-en / dw-de | Deutsche Welle 英/德 | rss.dw.com | ✅ 免费 |
| tagesschau | 德国电视一台新闻 | tagesschau.de/xml/rss2 | ✅ 免费 |
| spiegel-tops / spiegel-wirtschaft | Der Spiegel | spiegel.de | ✅ 免费 |
| zeit-all / zeit-politik / zeit-wirtschaft | Die Zeit | newsfeed.zeit.de | ✅ 🔒部分 |
| faz-politik / faz-wirtschaft | FAZ | faz.net/rss | ✅ 🔒部分 |
| handelsblatt / handelsblatt-wirtschaft | Handelsblatt | contentexport/feed | ✅ 🔒 |
| sz-top / sz-wirtschaft | Süddeutsche | rss.sueddeutsche.de | ✅ 🔒部分 |

## FR · 老牌媒体（legacy）

| ID | 来源 | Feed | 状态 |
|---|---|---|---|
| lemonde-international/economie/entreprises | Le Monde | rss_full.xml | ✅ 免费多数 |
| figaro-international / figaro-economie | Le Figaro | figaro rss | ✅ 🔒部分 |
| france24-fr / france24-en | France 24 | france24.com/rss | ✅ 免费 |
| rfi-fr | RFI | rfi.fr/fr/rss | ⚠️ 候选 |
| lesechos | Les Échos | rss.xml | ⚠️ 403 🔒 |
| lepoint / franceinfo | Le Point / franceinfo | rss | ⚠️ 候选 |

## ZH · 台湾媒体（legacy）

| ID | 来源 | Feed | 状态 |
|---|---|---|---|
| cna-politics | 中央社 兩岸/政治 | feeds.feedburner.com/rsscna/politics | ✅ 免费 |
| cna-world | 中央社 國際 | rsscna/intworld | ✅ 免费 |
| cna-mainland | 中央社 大陸 | rsscna/mainland | ✅ 免费 |
| cna-finance | 中央社 財經 | rsscna/finance | ✅ 免费 |
| ltn-politics | 自由時報 政治 | news.ltn.com.tw/rss/politics.xml | ✅ 免费 |
| ltn-world | 自由時報 國際 | rss/world.xml | ✅ 免费 |
| ltn-business | 自由時報 財經 | rss/business.xml | ✅ 免费 |

> 未收录：RTI（WAF 403）、聯合報（RSS 已停用）、工商時報（WAF）、Politico US/Axios（Cloudflare）、
> Bloomberg/FT（付费且无公开 RSS）、AP 分类 RSS（需商业凭证）。

## Google News 聚合（--google-news 可选）

3 分类 × 3 语言 × 2 查询 = 18 个搜索源（权重 0.75），覆盖政策/经济/产业热词。
