## Why

Firecrawl 是当前唯一的文章正文抓取器，自部署在树莓派上（3 容器：api + redis + playwright），必须常驻可用。一旦树莓派挂掉或网络不通，所有依赖 `feed.FirecrawlEnabled` 的文章正文都拿不到，形成单点故障。但实测表明：博客园、少数派等传统 SSR 站点的正文写在 HTML 源码里，纯 Go readability 就能干净提取（5814 / 22946 字），根本不需要浏览器渲染；只有 36氪、掘金、机器之心、智源这类 SPA 站才必须靠 Firecrawl。这意味着大部分文章本不需要经过树莓派，却被强绑定到这个单点上。

## What Changes

- **新增 `ReadabilityCrawler`：纯 Go 进程内正文提取器**，基于 `go-shiori/go-readability`（Mozilla Readability.js 的 Go 移植）+ `JohannesKaufmann/html-to-markdown`，HTTP GET → readability 提正文 → 转 Markdown。零外部依赖，后端活着它就活着。
- **新增 `Crawler` 接口 + 双源降级链**：抓取时先跑 readability（进程内，~200ms），用「正文 ≥500 字 且 垃圾特征词命中 <3」判定合格；不合格才降级调 Firecrawl。SSR 文章绕开树莓派，SPA 文章仍走 Firecrawl。
- **解耦 `FirecrawlService`**：当前 `job_firecrawl.go` 和 `firecrawl_handler.go` 直接 `NewFirecrawlService()` 并伸手进 `result.Data.Markdown` / `result.Data.Metadata`（Firecrawl 专有 JSON 结构）。引入中立 `ScrapeResult` 接口后，调用方只依赖中立结果，Firecrawl 退成可替换实现之一。
- **不改数据模型、不改下游**：readability/Firecrawl 抓到的 Markdown 仍写入 `article.FirecrawlContent`（列名虽叫 firecrawl，实为"正文文本"列），下游 ContentCompletion / Tagger / DailyReport 零改动。
- **Firecrawl 从「唯一命脉」降级为「SPA 兜底」**：`docker-compose.firecrawl.yml` 的定位从"必需常驻"变为"可选增强"。树莓派挂了，SSR 文章（博客园、博客类）仍能正常抓取；只有 SPA 文章受影响。

## Capabilities

### New Capabilities
- `article-content-crawling`：文章正文抓取的契约——定义 `Crawler` 接口、中立 `ScrapeResult`、双源降级链（readability 优先 → Firecrawl 兜底）、正文质量校验阈值（≥500字 + 垃圾词检测）。

### Modified Capabilities
- `compose-firecrawl`：Firecrawl 栈从"唯一爬虫、必需常驻"调整为"SPA 兜底爬虫、可选增强"；readability 主力处理 SSR 站点，Firecrawl 仅在 readability 提取不合格时兜底。

## Impact

- **后端**
  - 新增 `backend-go/internal/reader/service/crawler.go`：`Crawler` 接口 + 中立 `ScrapeResult`（Markdown / HTML / Title / OGImage）。
  - 新增 `backend-go/internal/reader/service/readability_crawler.go`：`ReadabilityCrawler` 实现 + 正文质量校验（`isUsableArticle`：长度阈值 + 垃圾词检测）。
  - 修改 `backend-go/internal/reader/service/firecrawl_service.go`：`FirecrawlService` 实现 `Crawler` 接口，`ScrapePage` 返回中立 `ScrapeResult`（内部仍调 Firecrawl API，做格式转换）。
  - 修改 `backend-go/internal/admin/scheduler/job_firecrawl.go`：注入 `Crawler`（实际为 readability→firecrawl 降级链组合体），替换直接 `NewFirecrawlService` + 读 `.Data.Markdown`。
  - 修改 `backend-go/internal/reader/handler/firecrawl_handler.go`：手动单篇抓取走同一降级链。
  - 修改 `backend-go/internal/reader/wire.go`：导出 `Crawler` 接口与降级链构造函数。
  - 新增依赖：`github.com/go-shiori/go-readability`、`github.com/JohannesKaufmann/html-to-markdown`（+ 传递依赖 goquery / golang.org/x/net）。
- **前端**：无改动（`firecrawl_status` / `firecrawl_content` 字段语义不变，progress WebSocket 消息不变）。
- **数据兼容**：零 DDL 变更。`article.FirecrawlContent` 仍存正文 Markdown，`firecrawl_status` 状态机不变（pending/processing/completed/failed）。新增来源信息可选地记入 `firecrawl_error` 前缀（如 `readability_fallback:`），纯观测用，不影响逻辑。
- **部署影响**：`docker-compose.firecrawl.yml` 不删除、不修改，但变为可选。树莓派上的 Firecrawl 可随时关闭而不影响 SSR 站点的正文抓取。
