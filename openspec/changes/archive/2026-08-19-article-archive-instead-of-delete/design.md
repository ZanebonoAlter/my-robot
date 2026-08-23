# Design: article-archive-instead-of-delete

## Context

现状：`FeedService.CleanupOldArticles`（`backend-go/internal/reader/service/feed_service.go:143`）在每次 RSS 刷新后按 `MaxArticles`（全库实测均为 100）物理删除超限文章，`pub_date DESC` 排序、favorite 免死。日报 `daily_report_threads.related_article_ids` 指向 `articles.id`，对删除零保护——实测 4136 条引用中 3891 条（94%）已死链。

存储构成（实测）：articles 表 229 MB 中 GIN 全文索引占 181 MB（79%），1912 篇文章的真实文本仅 ~48 MB（均 26 KB/篇：content 9.7K + description 9.4K + firecrawl 均摊 4.4K + ai_summary 2.9K）。删除文章回收的存储远小于直觉，而损失的是日报线索的原文可回看性。

约束：单用户系统、本地 Docker PostgreSQL；不改日报生成链路；不新增配置项。

## Goals / Non-Goals

**Goals:**

- 日报线索点开的文章永远可读（未来新引用零死链）
- 超限文章保留原文（title/description/content/firecrawl_content/ai_content_summary/link 等全部文本字段），ID 不变
- reader 列表/统计的默认视图不因归档文章而变化（归档文章不可见）
- 删除行为替换后，topic tags 边、reading_behaviors 的清理语义与今日完全一致（消费方零影响）

**Non-Goals:**

- 复活已被物理删除的 3891 条历史死链（数据已不存在，接受）
- 前端新增"归档管理"界面或"已归档 N 篇"提示（未来需要再做，API 留 `archived=true` 查询参数即可）
- 归档滚动窗口/总量上限（~14 MB/周慢增长，单用户年 ~730 MB 可接受；未来需要时作为独立 change）
- per-feed 归档开关等差异化配置（YAGNI）
- gzip 压缩归档文本（省 19 KB/篇但读路径要解压，性价比低）

## Decisions

### D1: DELETE → UPDATE 降级，衍生数据清除、内容全保留

归档时对超限文章执行：

```
UPDATE articles SET archived = true, search_vector = NULL WHERE id IN (...)
DELETE FROM article_topic_tags WHERE article_id IN (...)   → CleanupOrphanedTags（复用现有）
DELETE FROM reading_behaviors WHERE article_id IN (...)
```

**保留全部文本字段**（含 firecrawl_content、ai_content_summary）：归档语义 = "保留内容、清除结构索引"。曾考虑清 firecrawl_content（均摊省 4.4 KB/篇，17%），但 firecrawl 正文通常比 RSS description 完整，为省 17% 引入"归档文章缺正文"的不一致不值得。

**search_vector 置 NULL**：GIN 索引不索引 NULL 值，181 MB 索引在 autovacuum 回收后不再随归档增长。归档文章本来也不应再被全文搜索命中。

**tags 边删除**（与今日级联删除等价）：已逐一核查全部读方——日报生成查询全部带 `pub_date` 当天窗口（orchestrator:357/423/458/505，归档文章必然在窗口外）；reader 标签筛选 JOIN 的候选集来自已过滤列表；偏好画像权重源是 reading_behaviors（同样被删，等价）；lifeline 走 sections/relations 不碰此表。**结论：边的存活窗口（100 篇活跃窗口 ≈ 3.5 周）在改动前后完全一致。**

### D2: `archived bool` 字段，无新索引

Migration 加 `articles.archived boolean NOT NULL DEFAULT false`（PG11+ metadata-only，瞬时完成）。不加索引：布尔低基数，且活跃行占绝大多数（过滤条件 `archived = false` 的选择性与全表扫描相近），现有 `feed_id`/`pub_date` 索引已覆盖列表查询。

### D3: archived 过滤的查询清单（穷举定案）

**加 `archived = false`（用户可见的列表与统计口径）**：

| 查询点 | 位置 |
|---|---|
| `GetArticles` 列表 base query | `reader/handler/article_handler.go`（含 search/标签筛选，子查询 tag_count 同步） |
| `GetArticlesStats`（total/unread/favorite） | `reader/handler/article_handler.go` + `reader/repository/repository.go GetArticleStats` |
| feed 列表附带统计（article_count/unread） | `reader/handler/feed_handler.go:92` 与 `:185` |

**不加过滤（按 ID 单点 / 队列 / dedupe / 日报豁免）**：

| 查询点 | 理由 |
|---|---|
| `GetArticle` / `GetArticleWithStats` / `GetArticleWithTagCount`（按 ID 详情） | **日报线索点开的入口**，必须能读归档文章 |
| `RefreshFeed` title dedupe（`PluckArticlesTitlesByFeed`） | 归档文章 title 必须参与去重，否则 RSS 老条目重复入库 |
| `UpdateArticle` / `BulkUpdateArticles`（按 ID） | 操作目标来自已过滤列表，归档文章不可达 |
| `ListArticlesIncomplete` / `CountPendingArticles` / `ListArticlesForCompletion`（补全队列） | 按状态查询；归档文章即使被队列补全也无害（写入的是被保留的文本字段） |
| `job_blocked_article_recovery` / `runtime.go` 启动重置（按状态） | 归档文章状态为 completed，天然不被捞取 |
| 日报生成时间窗查询（orchestrator 全部） | 当天窗口与归档集互斥；保持现有语义 |
| 日报引用反查（`daily_report_repository.go:452/585`） | 本 change 的核心目标，显式豁免 |

### D4: `CleanupOldArticles` 重写要点

- **count 与候选排序都必须 `WHERE archived = false`**——否则归档行占用窗口计数，每次刷新会持续把活跃文章误归档（窗口侵蚀）。这是本 change 最容易犯的实现错误。
- favorite 免死、`maxArticles <= 0 || >= 9999` 跳过、`pub_date DESC` 排序语义全部保持。
- 幂等：已归档文章不再进入候选集，重复执行无副作用。
- `RefreshFeed` 单次入库数上限逻辑（`articlesAdded >= feed.MaxArticles`）不变。

### D5: API 与前端

- `GET /api/articles` 支持 `archived` 查询参数（默认 `false`；`true` 返回归档集，为未来管理界面预留）。
- 前端零改动：默认行为（活跃列表）与今天完全一致。feed 设置面板"最大文章数"的文案措辞更新（"超出删除"→"超出归档"）归入前端小改，随本 change 一并做。

## Risks / Trade-offs

- **[漏加过滤的查询口 → 归档文章漏进列表/统计]** → D3 清单已穷举（基于全仓 `Model(&models.Article{})` / `FROM articles` grep）；tasks 中含 grep 一致性验证步骤。
- **[CleanupOldArticles count 忘过滤 archived → 窗口侵蚀]** → 单元测试覆盖"存在归档行时活跃计数正确"；归档幂等性测试。
- **[GIN 索引回收依赖 autovacuum，短期内 181 MB 不缩]** → 接受（不增长即达标）；必要时手动 `VACUUM ANALYZE articles`。
- **[articles 表持续增长]** → ~14 MB/周（540 篇 × 26 KB），年 ~730 MB。单用户可接受；滚动窗口阀门留作未来 change。
- **[补生成历史日报时窗口内文章已被归档（极边缘：高频 feed 当天挤出 100 篇窗口）]** → 接受：日报引用反查豁免归档，文章仍可读，仅不参与聚类输入。
- **[Trade-off] 归档文章失去标签/搜索/行为数据** → 与今日物理删除的行为完全等价，无回归；且换来原文永久可读。

## Migration Plan

1. Migration（单个）：`ALTER TABLE articles ADD COLUMN archived boolean NOT NULL DEFAULT false`。无回填（存量 1912 篇全部视为活跃，与现状一致）。
2. 部署后首个 RSS 刷新周期内，各 feed 逐步建立归档集；无需手动触发。
3. 回滚：`ALTER TABLE articles DROP COLUMN archived`（归档信息丢失但文章仍在；衍生数据无法恢复——与回滚到旧行为的删除语义一致）。
4. 部署后用户可见变化：feed 文章列表不再因清理而"消失旧文章可见的原文链接"（日报线索从死链变为可读原文）；文章总数统计口径变为"活跃数"（不含归档）。

## Open Questions

无——探索阶段疑点（边的消费方影响、存储账本、配置必要性）均已用代码与实测数据定案。
