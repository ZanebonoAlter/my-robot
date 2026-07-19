# 文章内容增强流程（Content Enrichment）

> 大功能：文章正文抓取（readability 主力 / Firecrawl 兜底）/ 内容补全（文章级 AI 整理稿）/ 手动触发。
> 跨端。互补：`architecture/backend.md` §具体数据链路示例、`flow/reading.md` §Article 打标签时机。

## 需求说明

文章入库时往往只有 RSS 摘要（`description`），正文为空或太短。内容增强解决「让每篇文章有可读全文 + 可读 AI 整理稿」：

- **正文抓取**：默认走进程内 readability（快、零外部依赖），SSR 站即够；SPA 站 readability 抓到空壳时自动降级 Firecrawl（树莓派渲染 JS）。
- **内容补全**：拿到正文后，按 feed 开关决定是否调用 LLM 生成文章级 `ai_content_summary`（整理稿），写回 articles 表。
- **手动兜底**：正文抓取与整理稿均可手动触发单篇 / 整 feed；force 模式可重生成。
- **状态可见**：前端通过 completion status / overview 接口看到抓取进度、补全进度与卡住原因。

> 注意：本 flow 的「内容补全」生成的是 **article 级** 整理稿（`articles.ai_content_summary`），不是 `ai_summaries` 表的 feed 聚合摘要，也不是日报。运行时 scheduler 名为 `content_completion`（兼容旧别名 `ai_summary`）。

## 链路设计

### 正文抓取双源降级链

```text
文章 URL
  ├─ 1. ReadabilityCrawler（进程内纯 Go，~200ms，零外部依赖）
  │     HTTP GET → go-readability 提正文 → html-to-markdown
  │     └─ isUsableArticle? (≥500字 + 垃圾词<3) → 是：采用，结束
  │
  └─ 2. FirecrawlCrawler（树莓派，JS 渲染兜底，仅 SPA 站需要）
        └─ readability 不合格时降级调用
```

- **SSR 站**（博客园、博客类）：readability 进程内搞定，不碰树莓派。
- **SPA 站**（36氪、掘金、智源）：readability 提到空壳，自动降级 Firecrawl 渲染 JS。
- 树莓派挂掉时，SSR 文章不受影响；仅 SPA 文章进入 `firecrawl_status=failed` 重试队列。

### 端到端处理链路（feed 刷新 → 抓正文 → 补整理稿）

```text
Feed refresh
  -> article created
  -> firecrawl_status = pending            (if feed.firecrawl_enabled)
  -> Firecrawl scheduler 抓全文
  -> firecrawl_status = completed
  -> summary_status = incomplete           (if feed.article_summary_enabled)
  -> ContentCompletion scheduler 基于 firecrawl_content 生成 ai_content_summary
  -> summary_status = complete
```

一句话：内容处理链路的核心对象始终是 `articles` 表，Firecrawl 与内容补全都在给 article 补字段；**不是**通过单独队列表驱动，而是通过 article 上的状态字段进入后续 scheduler 扫描范围。

### Firecrawl / 内容补全状态查询

```mermaid
sequenceDiagram
  participant UI as ArticleContentView
  participant CC as useContentCompletion
  participant API as content-completion API
  participant BE as 后端
  UI->>CC: getCompletionStatus(articleId)
  CC->>API: GET /content-completion/articles/:id/status
  API-->>CC: 抓取状态/整理状态/错误
  CC-->>UI: 展示状态
```

### 手动抓取全文

```text
ArticleContentView
  → useFirecrawlApi.crawlArticle(articleId)  (POST /api/firecrawl/article/:id)
  → 后端执行抓取 → 写回 firecrawl_content / firecrawl_status / firecrawl_crawled_at
  → 成功后 summary_status 设为 incomplete（触发后续补全）
  → 再次查询 completion status → UI 更新
```

### 手动生成整理稿

```text
ArticleContentView
  → completeArticle(articleId, { force: true })  (POST /api/content-completion/articles/:id/complete)
  → 后端生成 ai_content_summary → 更新 summary_status / summary_generated_at
  → 再次查询 completion status → UI 渲染整理稿
```

### 关键状态字段

feed 级开关：`firecrawl_enabled`、`article_summary_enabled`、`tagging_enabled`、`max_completion_retries`。

article 级状态：`firecrawl_status`（`pending`/`processing`/`completed`/`failed`）、`firecrawl_content`、`firecrawl_error`、`firecrawl_crawled_at`；`summary_status`（`incomplete`/`pending`/`complete`/`failed`）、`ai_content_summary`、`completion_attempts`、`completion_error`、`summary_generated_at`、`summary_processing_started_at`。

## 业务约束与不变量

> 本节同时是 `scripts/doc-impact.sh context` 的数据源——改 `internal/reader/`（crawler / content_completion / firecrawl）代码前会被自动 dump，必读。

1. **正文提取内容优先级**：展示层 `AIContentSummary → FirecrawlContent → Content → Description`；但**内容补全（`CompleteArticle`）只用 `firecrawl_content`**——为空直接失败（`No firecrawl content available`），不会退而用 `content`/`description`。故 Firecrawl 未完成的文章进不了补全链路。
2. **内容补全前置门禁（四道，全过才执行）**：① article 存在；② feed 存在且 `article_summary_enabled=true`（否则报 `AI summary not enabled for this feed`）；③ `firecrawl_status == completed`（否则报 `firecrawl not completed`）；④ 非 force 时 `completion_attempts < max_completion_retries`（超限标 `failed` 并返回 `max completion retries exceeded`）。
3. **补全状态机**：`incomplete → pending → (complete | failed)`。开始处理先 claim（置 `pending` + `completion_attempts += 1` + 清 `completion_error` + 记 `summary_processing_started_at`）；成功置 `complete` + 写 `ai_content_summary` + `summary_generated_at` + 清 lease；失败时若已达重试上限置 `failed`，否则回退 `incomplete`。
4. **重试上限与 force**：`max_completion_retries` 默认 **1**，可由 feed 覆盖；`force=true` 跳过上限检查并清空旧 `ai_content_summary` / `summary_generated_at` 重新生成。
5. **claim 乐观锁防并发重复处理**：`claimArticleForCompletion` 用条件 UPDATE（`WHERE` 带 `summary_status` + stale 判断，靠 `RowsAffected>0` 判定是否抢到）保证同一 article 不会被并发 scheduler / 手动触发同时处理；抢不到直接返回（非错误）。
6. **卡死租约恢复（lease + grace）**：`summary_processing_started_at` 超过 `30min`（lease）+ `2min`（clock skew grace）= 32min 视为 stale，可被重新 claim；`blocked_article_recovery` scheduler 配合把卡在 `processing`/`pending` 的文章重置。Firecrawl 同理有自己的 processing 超时恢复。
7. **Firecrawl 串行抓取 + 单批上限 50**：`FirecrawlScheduler` 每 300 秒轮询，只查 `feeds.firecrawl_enabled=true AND articles.firecrawl_status=pending`，单批最多 50 篇、**单线程串行**（`concurrency=1`）抓取，抓前置 `processing`、抓后置 `completed`/`failed`；进度经 `platform/ws` 广播 `firecrawl_progress`。
8. **补全完成后联动打标签**：补全成功且 `feed.tagging_enabled=true` 时，enqueue `tag_jobs`（reason=`summary_completed`）重新打标签（整理稿是更优的打标签输入）。
9. **调度器名兼容**：对外规范名为 `content_completion`，仍接受旧别名 `ai_summary`；它**不是** `ai_summaries` 表里的 feed 聚合摘要。

## 代码入口

- **后端 reader 域（正文抓取）**：`backend-go/internal/reader/service/crawler.go`（`Crawler` 接口 + 中立 `ScrapeResult`）、`readability_crawler.go`（进程内主力）、`fallback_crawler.go`（降级链）、`firecrawl_service.go`（Firecrawl 兜底）、`backend-go/internal/reader/handler/`（content_completion_handler、firecrawl handler）。
- **后端 reader 域（内容补全）**：`backend-go/internal/reader/service/content_completion_service.go`（`CompleteArticle` / `CompleteArticleWithForce` / `claimArticleForCompletion` / `ListReadyArticles` / `GetOverview`）、`backend-go/internal/reader/service/feed_service.go`（refresh 写入初始状态位）、`backend-go/internal/reader/routes.go`（`/content-completion/*`、`/firecrawl/*`）。
- **后端调度（admin 域）**：`backend-go/internal/admin/scheduler/job_firecrawl.go`、`job_content_completion.go`、`job_blocked_article_recovery.go`。
- **平台层**：`backend-go/internal/platform/airouter/`（补全走 `CapabilitySummary` 路由，失败回退 fallback AIService）、`backend-go/internal/platform/ws/`（进度广播）、`backend-go/internal/tagmanagement/`（补全后 enqueue tag job）。
- **前端**：`front/app/features/articles/components/ArticleContentView.vue`、`front/app/features/articles/composables/useContentCompletion.ts`、`front/app/features/shell/components/FeedLayoutShell.vue`（feed 开关编辑）、`front/app/utils/articleContentSource.ts`（内容来源切换）、`front/app/api/`。

## 变更溯源

| 日期 | 变更 | 摘要 | 归档位置 |
| ------ | ------ | ------ | ---------- |
| 2026-05-10 | global-settings-feed-controls | feed 级新增 `tagging_enabled` 开关与 Firecrawl / 内容补全 toggle 并列；Firecrawl 完成回调也检查 `tagging_enabled` 决定是否入 tag 队列 | [`openspec/changes/archive/2026-05-10-global-settings-feed-controls`](../../../openspec/changes/archive/2026-05-10-global-settings-feed-controls) |
| 2026-05-13 | backend-package-restructure | `contentprocessing/` → `reader/` 域（content_completion / firecrawl 归入 reader service）；旧 user-guide 的 `internal/domain/*`、`internal/jobs/` 路径已失效 | [`openspec/changes/archive/2026-05-13-backend-package-restructure`](../../../openspec/changes/archive/2026-05-13-backend-package-restructure) |
| 2026-07-11 | lightweight-crawler-fallback | readability 进程内主力 + Firecrawl 兜底降级链，消除树莓派单点 | [`openspec/changes/archive/2026-07-11-lightweight-crawler-fallback`](../../../openspec/changes/archive/2026-07-11-lightweight-crawler-fallback) |
