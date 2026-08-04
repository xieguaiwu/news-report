# Task for prometheus

为 news-report 项目设计 LLM 集成方案的详细实现计划。

背景：news-report 是一个 Go CLI/TUI 多语言新闻聚合器（EN/DE/FR/ZH），已有分类、搜索、全文阅读、复制链接等功能。

需要实现的功能（按优先级）：

1. 公共底座 `internal/llm/`：
   - OpenAI 兼容 /chat/completions 客户端
   - config.yaml 新增 llm 段：base_url、api_key（支持 {env:VAR} 引用）、model、target_lang（默认 zh）、timeout
   - 无 key 时优雅降级（提示用户配置，不报错崩溃）
   - 输出缓存：key=sha256(URL+lang+指令)，存 ~/.cache/news-report/llm/

2. `t` 键一键翻译（阅读器视图）：
   - 按 t 触发异步翻译 → 正文替换为译文 → Esc 回原文
   - 分块翻译：默认前 4000 字，按段落分块 ≤1500 字，块间重叠 50 字
   - 系统提示词："专业新闻译者，保留专有名词/数字/引语，输出中文"
   - 缓存命中直接显示，零成本

3. `x` 键摘要（列表视图） / `d` 键解读（阅读器视图）：
   - x：当前条目 5 条要点摘要（≤25 字/条）
   - d：四段式深度解读（背景/各方立场/影响/后续关注点）
   - 复用已抓取的 reader.body，无需额外抓取
   - 无正文时自动先 fetch 再调 LLM

请产出：
- internal/llm/ 包的文件结构和接口设计
- config.yaml llm 段的结构
- tui.go 中需要新增/修改的关键代码位置和逻辑
- 缓存策略细节
- 错误处理和降级逻辑
- 预计新增文件清单和行数估算

注意：不要输出完整代码，输出的是详细的实现计划，供 builder agent 照着写代码。

项目当前文件树：
- main.go (656行)
- internal/
  - article/ (article.go + test)
  - cache/ (cache.go + test)
  - classify/ (classify.go + test)
  - config/ (config.go + test)
  - dedup/ (dedup.go + test)
  - feed/ (feed.go + test)
  - fetch/ (fetch.go + test)
  - gnews/ (gnews.go + test)
  - output/ (output.go + test)
  - rank/ (rank.go + test)
  - report/ (report.go + test)
  - s2t/ (s2t.go + test)
  - scrape/ (scrape.go + test)
  - sources/ (sources.go + test)
  - store/ (store.go + test)
  - tui/ (tui.go ~900行 + test)

请阅读上面列出的关键文件（特别是 config.go、tui.go、cache.go），然后产出实现计划。输出中文。

## Acceptance Contract
Acceptance level: attested
Completion is not accepted from prose alone. End with a structured acceptance report.

Criteria:
- criterion-1: Return a concise result and residual risks when applicable

Required evidence: manual-notes, residual-risks

Finish with a fenced JSON block tagged `acceptance-report` in this shape:
Use empty arrays when no items apply; array fields contain strings unless object entries are shown.
`criteriaSatisfied[].status` must be exactly one of: satisfied, not-satisfied, not-applicable.
`commandsRun[].result` must be exactly one of: passed, failed, not-run.
`manualNotes` and `notes` are optional strings; an empty string means no note and does not satisfy `manual-notes` evidence.
```acceptance-report
{
  "criteriaSatisfied": [
    {
      "id": "criterion-1",
      "status": "satisfied",
      "evidence": "specific proof"
    }
  ],
  "changedFiles": [
    "src/file.ts"
  ],
  "testsAddedOrUpdated": [
    "test/file.test.ts"
  ],
  "commandsRun": [
    {
      "command": "command",
      "result": "passed",
      "summary": "short result"
    }
  ],
  "validationOutput": [
    "validation output or concise summary"
  ],
  "residualRisks": [
    "none"
  ],
  "noStagedFiles": true,
  "diffSummary": "short description of the diff",
  "reviewFindings": [
    "blocker: file.ts:12 - issue found, or no blockers"
  ],
  "manualNotes": "anything else the parent should know"
}
```