# Proposal: article-archive-instead-of-delete

## Why

feed 的 `CleanupOldArticles`（`feed_service.go:143`）按 `MaxArticles=100` 物理删除超限文章，而日报 thread 的 `related_article_ids`（jsonb 指向 `articles.id`）对被删文章**零保护**。实测（2026-08-18）：日报共 4136 条文章引用，**3891 条（94%）已死链**——用户在日报里点开线索，原文早已不存在。同时存储分析表明删除一篇文章只回收 26 KB 原文文本，其余 94 KB 是 GIN 检索索引（181 MB）与衍生数据，"删原文省空间"的收益本就远低于直觉。

## What Changes

- **归档代替删除**：`CleanupOldArticles` 超限文章不再 `DELETE`，改为 `UPDATE ... SET archived = true` 并清除衍生数据（topic tags 边、reading behaviors、firecrawl 内容、`search_vector` 置 NULL）。文章行与原文字段（title/description/content/link）永久保留，ID 不变。
- **新增 `articles.archived` 布尔字段**（migration），带 partial index 支撑列表过滤。
- **列表/统计默认过滤归档**：`GetArticles` 等 reader 查询默认 `WHERE archived = false`；用户可传 `archived=true` 显式查看。
- **日报引用反查不过滤归档**：日报线索点开即得原文（死链复活，含未来新引用）。
- **不新增任何配置项**：`MaxArticles` 语义自然延伸为"活跃窗口大小"（超过即归档），favorite 免死、0=无限 语义保持不变。per-feed 归档开关等差异化需求 YAGNI，未来需要时向后兼容追加。
- **BREAKING**（内部语义）：超限文章的 topic tags 边仍被删除（与今日级联删除行为等价，见 design 论证），偏好画像/lifeline/日报生成均不受影响。

## Capabilities

### New Capabilities
- `article-retention`: 文章保留与归档策略——超限文章的降级语义（保留原文、清除衍生数据）、`archived` 字段生命周期、归档文章对 reader 列表/搜索/统计的可见性规则、日报引用对归档文章的读取豁免。

### Modified Capabilities
- `feed-settings-ui`: "最大文章数"需求的行为语义从"超出物理删除"改为"超出归档降级"；`0=无限`、`9999 兼容`、favorite 免死等既有 Requirement 保持不变，需更新涉及删除措辞的场景。

## Impact

- **代码**（全部后端 `backend-go/`，前端零改动）：
  - `internal/models/article.go`：加 `Archived` 字段
  - `internal/reader/service/feed_service.go`：`CleanupOldArticles` 重写（DELETE→UPDATE 降级）
  - `internal/reader/handler/article_handler.go`：列表/统计查询加 archived 过滤
  - `internal/platform/database/postgres_migrations.go`：新增 migration（字段 + partial index）
- **数据**：已有 1912 篇文章全部 `archived=false` 起步，无需回填；已被物理删除的 3891 条历史死链无法复活（接受）。
- **存储**：慢增长 ~14 MB/周（26 KB/篇 × ~540 篇/周），GIN 索引不再随归档增长；可选阀门（清 firecrawl_content、归档滚动窗口）留待 design 决策。
- **文档**：`docs/reference/flow/` 文章链路、`docs/reference/database/` 需同步（走 doc-impact 门禁）。
