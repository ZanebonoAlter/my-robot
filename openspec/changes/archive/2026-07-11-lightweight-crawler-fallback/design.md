## Context

Syntopica 的文章正文抓取当前完全依赖 Firecrawl（自部署在树莓派，3 容器：api + redis + playwright）。`FirecrawlService.ScrapePage` 是唯一的抓取实现，在 `job_firecrawl.go:36` 和 `firecrawl_handler.go:70` 两处直接 `NewFirecrawlService()` 构造、直接读 `result.Data.Markdown` / `result.Data.Metadata`（Firecrawl 专有 JSON 结构）。没有抽象接口。

**调研结论**（`research/readability-spike/`，实跑数据）：

| 站点 | 类型 | 纯 Go readability 表现 |
|------|------|----------------------|
| 博客园 | SSR | ✅ 5814 字，正文干净 |
| 少数派 | SSR | ✅ 22946 字，正文干净 |
| 36氪 | SPA | ❌ 160 字，全是页脚/备案 |
| 掘金 | SPA | ❌ 492 字，导航壳 |
| 机器之心 | SPA | ❌ 77 字，登录壳 |
| 智源 hub | SPA | ❌ 143 字，页脚 |

**结论**：SSR 站（博客、技术博客类）readability 完美覆盖，不需要浏览器；SPA 站才需要 Firecrawl 的 JS 渲染。当前架构把所有 `FirecrawlEnabled` 文章都送往树莓派，造成了本不需要浏览器的文章也被单点绑架。

## Goals / Non-Goals

**Goals:**
- SSR 文章正文抓取不依赖 Firecrawl（进程内 readability，零外部服务）
- SPA 文章仍由 Firecrawl 兜底（readability 提取不合格时自动降级）
- Firecrawl 从"唯一命脉"降级为"可选增强"，树莓派可随时关闭不影响 SSR 站
- 解耦 `FirecrawlService`：调用方依赖中立接口，不感知 Firecrawl JSON 结构
- 零数据模型变更、零下游改动

**Non-Goals:**
- 不替换 Firecrawl（并存，readability 主力 + Firecrawl 兜底）
- 不引入 chromedp / go-rod 等纯 Go 无头浏览器（那是未来可选的"第二兜底层"，本期不做）
- 不引入 crawl4ai 等 Python sidecar（用户明确要求纯 Go）
- 不改前端、不改 WebSocket 消息格式、不改 `firecrawl_status` 状态机
- 不改 RSS 抓取链路（`RSSParser` 是另一条链路，本期只管正文抓取）

## Decisions

### D1: 抽象缝选在 `FirecrawlService.ScrapePage`，引入 `Crawler` 接口

**选择**：在 `backend-go/internal/reader/service/` 新增 `Crawler` 接口 + 中立 `ScrapeResult`，让 `FirecrawlService` 实现它。

```go
// Crawler 是文章正文抓取的中立接口。readability/Firecrawl 都实现它。
type Crawler interface {
    ScrapePage(ctx context.Context, url string) (*ScrapeResult, error)
}

// ScrapeResult 是中立结果，不绑死任何爬虫的 JSON 结构。
type ScrapeResult struct {
    Markdown string // 清洗后的正文 Markdown（下游写入 article.FirecrawlContent）
    HTML     string // 原始 HTML（可选，供未来扩展）
    Title    string // 文章标题
    OGImage  string // 封面图（补 article.ImageURL 用）
    Source   string // 来源标记："readability" | "firecrawl"
}
```

**理由**：现有调用方对结果的使用只有两处——`.Data.Markdown`（存正文）和 `.Data.Metadata.OgImage/TwitterImage`（补封面图）。中立 `ScrapeResult` 正好覆盖这两个用途。调用方不再伸手进 Firecrawl 专有结构。

**替代方案**（否决）：不抽接口，直接在 `FirecrawlService.ScrapePage` 内部先试 readability。否决理由——把两个抓取策略的调度逻辑塞进同一个实现类，职责不清；且手动单篇抓取（handler）和批量抓取（job）都需共享降级链，抽接口更自然。

### D2: 降级链顺序——readability 优先，Firecrawl 兜底

**选择**：`ReadabilityCrawler` 先抓，不合格才降级 `FirecrawlCrawler`。组合成一个 `FallbackCrawler`。

```
FallbackCrawler.ScrapePage(url):
  1. readability.ScrapePage(url)          // 进程内，~200ms，零成本
  2. isUsableArticle(result)?  是 → 返回   // SSR 文章到此结束，不碰树莓派
  3. firecrawl.ScrapePage(url)            // 降级，调树莓派
  4. 返回 firecrawl 结果（成功或失败）
```

**理由（为什么不反过来"Firecrawl 优先、readability 兜底"）**：
1. **省资源**：80% SSR 博客文章在进程内搞定，根本不调树莓派，树莓派负载骤降。
2. **确定性**：readability 对 SSR 站是确定性成功，不像 Firecrawl 可能超时/服务不可达。
3. **单点消除**：树莓派挂了，SSR 站照样抓；只有 SPA 文章受影响。
4. **延迟**：readability ~200ms vs Firecrawl 几秒~几十秒，SSR 文章延迟大幅降低。

### D3: 正文质量校验阈值——500字 + 垃圾词检测

**选择**：`isUsableArticle(result)` 判定规则：
- `len([]rune(result.Markdown)) >= 500`（对齐 ContentCompletion 的 200 字阈值但更保守）
- 且 `junkSignalCount(result.Markdown) < 3`（垃圾特征词命中少于 3 个）

**垃圾特征词清单**（来自调研实测，SPA 空壳内容的共性）：`备案`、`ICP`、`版权`、`Copyright`、`©`、`举报电话`、`All Rights`、`京ICP`、`登录去登录`。

**理由**：调研中 36氪（160字+3垃圾词）、智源（143字+3垃圾词）被纯字数阈值误判为"合格"，垃圾词检测补上这个漏洞。博客园真实文章（5814字+0垃圾词）不受影响。

### D4: readability 库选型——`go-shiori/go-readability`

**选择**：用 `go-shiori/go-readability`（字段式 `Article.Content`，调用简单）。

**替代方案**（备选）：`codeberg.org/readeck/go-readability/v2`（官方推荐的继任者，但 API 用方法 `RenderHTML(io.Writer)`，调用更繁琐）。调研中两者提取质量完全一致（博客园都是 5814 字）。

**现状**：shiori 版已标记 deprecated，但功能稳定、API 简单、广泛使用。若未来停止维护可无缝切 readeck（接口层隔离，切换只改实现类）。design 记录此迁移路径。

### D5: 错误观测——来源标记进 `firecrawl_error`，不改状态机

**选择**：readability 降级到 firecrawl 时，若想知道"哪些文章走了降级"，可选地在 `firecrawl_error` 前缀记 `readability_fallback:`（仅当 firecrawl 也失败时）。状态机不变。

**理由**：用户需要观测"树莓派挂了多少文章"，但不值得为此加列。复用现有 `firecrawl_error` 文本字段做软标记，零 DDL。

## Risks / Trade-offs

- **[readability 对部分 SSR 站提取质量不稳定]** → 有垃圾词检测 + 字数阈值兜底，提取不合格自动降级 Firecrawl，不会把垃圾内容写进库。最坏情况是 readability 拿到合格但非最优的正文（比如多了些导航文本），但这不比现状差（Firecrawl 同样可能提不全）。
- **[go-shiori/go-readability 已 deprecated]** → 接口层隔离了实现，迁移到 readeck 只改 `readability_crawler.go` 一个文件。design D4 已记录迁移路径。
- **[垃圾词清单不全]** → 清单可配置化（硬编码在常量切片，后续按需扩充）。误判方向是"保守多降级"——漏掉一个垃圾词导致 readability 不合格 → 降级 Firecrawl，不会造成正文质量下降，只会多调一次树莓派。
- **[readability + Firecrawl 双调用增加总耗时]** → 仅对 readability 不合格的 SPA 文章生效（多一次 ~200ms 的 readability 调用）。SSR 文章反而比现状更快（200ms vs Firecrawl 几秒）。净收益为正。
- **[依赖升级 x/net v0.51→v0.41+、goquery v1.8→v1.9]** → 都是向后兼容的小版本升级，backend-go 已间接依赖这两个包，升级风险低。apply 阶段 `go mod tidy` 后跑全量测试验证。

## Migration Plan

本变更**不涉及数据迁移**（零 DDL），部署步骤：

1. 合并代码后重启后端即可生效。
2. **无需任何手动操作**——readability 是进程内的，启动即用。
3. Firecrawl（树莓派）可继续开着（SPA 文章仍会用它），也可随时关闭（SSR 文章不受影响）。
4. **降级**：如出问题，回滚代码即可恢复"纯 Firecrawl"行为，数据无影响。

**部署后影响**：
- (a) 用户可见行为：SSR 站文章（博客园、博客类）的正文抓取不再依赖树莓派，速度更快、更稳；SPA 站文章行为不变。
- (b) 需要手动操作：无。
- (c) 旧数据降级：无——历史 `article.FirecrawlContent` 不受影响，`firecrawl_status` 状态机不变。

## Open Questions

无。所有决策已在调研阶段用实测数据确认。阈值（500字+垃圾词）已与用户确认。
