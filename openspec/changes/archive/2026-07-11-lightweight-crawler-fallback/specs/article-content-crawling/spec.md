## ADDED Requirements

### Requirement: Crawler 接口定义中立抓取契约
系统 SHALL 在 `backend-go/internal/reader/service/` 定义 `Crawler` 接口和中立 `ScrapeResult` 结构，使正文抓取的调用方不感知具体爬虫（readability / firecrawl）的实现细节。

`ScrapeResult` SHALL 包含字段：`Markdown`（正文）、`Title`（标题）、`OGImage`（封面图）、`Source`（来源标记：`"readability"` 或 `"firecrawl"`）。

#### Scenario: 接口可被多种实现满足
- **WHEN** readability 实现和 firecrawl 实现各自实现 `Crawler` 接口的 `ScrapePage(ctx, url) (*ScrapeResult, error)`
- **THEN** 两者都满足接口契约，可互换注入到调用方

#### Scenario: 调用方不感知爬虫专有结构
- **WHEN** 调用方（job / handler）通过 `Crawler` 接口拿到 `ScrapeResult`
- **THEN** 读取 `result.Markdown` 和 `result.OGImage`，不伸手进 firecrawl 的 `result.Data.Metadata` 等专有结构

### Requirement: ReadabilityCrawler 提供进程内纯 Go 抓取
系统 SHALL 提供 `ReadabilityCrawler`，基于 `go-shiori/go-readability` + `JohannesKaufmann/html-to-markdown`，在 Go 进程内完成 HTTP GET → readability 正文提取 → Markdown 转换，零外部服务依赖。

#### Scenario: 抓取 SSR 站点成功
- **WHEN** `ReadabilityCrawler.ScrapePage` 抓取一个服务端渲染的文章 URL（如博客园文章）
- **THEN** 返回 `ScrapeResult{Markdown: <正文>, Source: "readability"}`，Markdown 长度 ≥500 字且垃圾词命中 <3

#### Scenario: 抓取 SPA 站点得到空壳
- **WHEN** `ReadabilityCrawler.ScrapePage` 抓取一个前端渲染的 SPA 文章 URL（如 36氪/掘金）
- **THEN** 返回的 Markdown 长度 <500 字 或垃圾词命中 ≥3，标记为不可用

#### Scenario: 后端存活即抓取可用
- **WHEN** Firecrawl 服务不可达或已关闭，但 backend-go 进程在运行
- **THEN** `ReadabilityCrawler` 仍可正常抓取 SSR 站点，不依赖任何外部服务

### Requirement: 正文质量校验阈值
系统 SHALL 在判定 readability 结果是否"合格可用"时，同时满足两个条件：正文长度 ≥500 字（按 rune 计）且垃圾特征词命中数 <3。

垃圾特征词清单 SHALL 至少包含：`备案`、`ICP`、`版权`、`Copyright`、`©`、`举报电话`、`All Rights`、`京ICP`、`登录去登录`。

#### Scenario: 合格正文被采纳
- **WHEN** readability 返回 Markdown 长度 5814 字，垃圾词命中 0 个
- **THEN** 判定为合格，采纳该结果，不降级 Firecrawl

#### Scenario: SPA 空壳被识破
- **WHEN** readability 返回 Markdown 长度 160 字（36氪页脚）
- **THEN** 因长度 <500，判定为不合格，触发降级 Firecrawl

#### Scenario: 垃圾词过量的假成功被识破
- **WHEN** readability 返回 Markdown 长度 650 字但包含 3 个垃圾特征词（备案/版权/ICP）
- **THEN** 因垃圾词命中 ≥3，判定为不合格，触发降级 Firecrawl

### Requirement: 双源降级链——readability 优先，Firecrawl 兜底
系统 SHALL 提供 `FallbackCrawler`，对每个 URL 先尝试 readability（进程内），仅当 readability 结果不合格时才降级调用 Firecrawl。

#### Scenario: SSR 文章不经过 Firecrawl
- **WHEN** 文章 URL 是 SSR 站点，readability 返回合格正文
- **THEN** 直接采纳 readability 结果，不调用 Firecrawl，Firecrawl 服务零负载

#### Scenario: SPA 文章降级到 Firecrawl
- **WHEN** 文章 URL 是 SPA 站点，readability 返回不合格结果（空壳/太短/垃圾词过量）
- **THEN** 自动降级调用 Firecrawl，采用 Firecrawl 的抓取结果

#### Scenario: 两个爬虫都失败
- **WHEN** readability 不合格且 Firecrawl 也失败（树莓派挂掉）
- **THEN** 返回 Firecrawl 的错误，文章进入 `firecrawl_status=failed` 重试队列，SSR 文章不受此影响

### Requirement: 抓取结果写入现有数据模型不改 schema
无论 readability 还是 Firecrawl 抓取成功，系统 SHALL 将结果写入 `article.FirecrawlContent`（Markdown 正文）和可选回填 `article.ImageURL`（当 RSS 未提供时）。`firecrawl_status` 状态机（pending/processing/completed/failed）不变。

#### Scenario: readability 成功写入
- **WHEN** readability 返回合格正文
- **THEN** `article.FirecrawlContent` 写入该 Markdown，`firecrawl_status=completed`，`firecrawl_crawled_at` 设为当前时间

#### Scenario: 封面图回填优先级不变
- **WHEN** readability/Firecrawl 返回了 OGImage 且 `article.ImageURL` 为空
- **THEN** 回填 `article.ImageURL`；RSS enclosure 仍有最高优先级（RSS 提供时不覆盖）
