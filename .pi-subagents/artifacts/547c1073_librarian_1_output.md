调研完成。所有关键机制已实测确认。

---

# Google News RSS 搜索接口调研报告（find 转载功能）

## 1. URL 格式与引号短语 ✓ 实测可用

```
https://news.google.com/rss/search?q=QUERY&hl=en-US&gl=US&ceid=US:en
```

- URL 编码引号短语 **可用**：`q=%22EU+sanctions+package%22` → 返回 200，channel title 回显 `""EU sanctions package"" - Google News`，结果集与不带引号版本不同（引号确实生效为精确短语）
- 每次最多 **100 条**（上限截断，无分页参数）
- `when=1d` / `when=1h` 参数被接受且会改变结果集，但**不是严格时间过滤**（`when=1h` 的结果里仍有 7 月 3 日的条目，实测 oldest pubDate 三种查询完全相同）→ 只作软排序提示
- 响应 ~140KB（100 条），单请求 ~1s

## 2. RSS 结构（channel/item 实测）

```xml
<rss version="2.0" xmlns:media="http://search.yahoo.com/mrss/">
  <channel>
    <generator>NFE/5.0</generator>
    <title>""EU sanctions package"" - Google News</title>   <!-- 查询回显 -->
    <link>https://news.google.com/search?q=...&amp;hl=...&amp;ceid=...</link>  <!-- Web UI 链接 -->
    <language>en-US</language>
    <webMaster>news-webmaster@google.com</webMaster>
    <copyright>...solely for rendering Google News results within a personal feed reader
               for personal, non-commercial use...</copyright>   <!-- ⚠️ 版权声明 -->
    <lastBuildDate>Sun, 02 Aug 2026 16:20:43 GMT</lastBuildDate>
    <item>
      <title>The Irish Independent's View: ... - Irish Independent</title>  <!-- 标题 + " - 来源名" -->
      <link>https://news.google.com/rss/articles/CBMiiAJ...?oc=5</link>     <!-- ← Google 跳转链接，非原站 -->
      <guid isPermaLink="false">CBMiiAJ...</guid>                          <!-- 同 token，无 oc 参数 -->
      <pubDate>Fri, 31 Jul 2026 05:30:00 GMT</pubDate>                     <!-- RFC822 GMT -->
      <description>&lt;a href="https://news.google.com/rss/articles/...?oc=5" target="_blank"&gt;标题&lt;/a&gt;&amp;nbsp;&amp;nbsp;&lt;font color="#6f6f6f"&gt;Irish Independent&lt;/font&gt;</description>
      <source url="https://www.independent.ie">Irish Independent</source>  <!-- ★ 唯一带原站信息的字段，仅域名 -->
    </item>
```

**关键结论**：`<link>` 一律是 `news.google.com/rss/articles/CBMi...?oc=5` 跳转链接，**绝无原站链接**。RSS 里唯一能拿到原站信息的字段是 `<source url>`——但只有**域名**（如 `https://www.independent.ie`），没有文章路径。

## 3. 跳转机制实测（curl -sI / -L）

```
hop 1: 302  (content-length: 0)
       Location: 同一 /rss/articles/... 链接 + 按请求方 GeoIP 追加 &hl=en-SG&gl=SG&ceid=SG:en
       ⚠️ 我请求时传的 hl=en-US 被 GeoIP 覆盖成 SG
hop 2: 200  ~594KB Google News SPA（纯 JS 应用，HTML 里不含原站 URL）
```

- **`curl -L` 永远停在 Google 的 SPA 页，到不了原站**——最终跳到原文的跳转由 SPA 的 JavaScript 执行（内部 batchexecute API 解析 token），初始 HTML 中搜索不到任何原站域名/URL
- `oc=5`（Web UI 语义"直接打开原文"）在 RSS 跳转链接上服务端不区分：oc=1/3/5/无参 行为完全相同

**拿原站 URL 的三条路径：**
1. ❌ 纯 HTTP 跟随重定向：不可行（停在 SPA）
2. ⚠️ 解码 guid：token = base64url(protobuf)，实测结构 = `field 1 varint=19`（版本号）+ `field 4 string`（`AU_yq` 开头的 **AES 加密载荷**）。**新版格式不含明文 URL**。社区解码器存在（`SSujitX/google-news-url-decoder`, 294★，从 Google News 页面 JS 提取 AES key 解密）——可行但**脆弱**，Google 改方案即失效，不建议作为依赖
3. ✅ 无头浏览器跟随：chromedp 打开 `<link>` 等 JS 跳转后取最终 URL（本项目已有 chromedp 经验）

## 4. 限流实测

- 本机 IP：10 次 0.5s 间隔 + 40 次无间隔连发，**全部 HTTP 200，无 429、无限流头**
- ⚠️ 注意：未测数据中心 IP；feed 自带版权声明限定"个人非商业用途的个人阅读器使用"——本项目属个人聚合器，合规，但不应再分发 feed 内容

## 5. 建议实现方式（find 转载）

1. **搜索**：标题 URL 编码（含引号短语 `%22...%22`），固定 `hl=en-US&gl=US&ceid=US:en`
2. **解析**：每 item 取 title（注意自带 ` - 来源名` 后缀，做标题匹配时要剥离）、link、source url
3. **过滤**：用 `<source url>` 域名排除原站自身 → 其余即转载候选；用 `pubDate` 排序
4. **链接输出策略**：
   - **最小可行**：直接给出 `<link>`（Google 跳转链接），用户点击后 JS 跳到免费镜像——零额外请求
   - **需要聚合器自抓全文**（go-readability 深度阅读）：必须走无头浏览器跟随 JS 跳转拿最终 URL，**不要**用普通 HTTP redirect 跟随（-L 无效）
5. 匹配转载时注意：Google 结果按"相关度+时间"混合排序，同标题可能有多个来源副本，可全部列出让用户选

---