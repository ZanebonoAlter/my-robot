## MODIFIED Requirements

### Requirement: 日报存储

系统 SHALL 提供 `SaveReport`（创建或更新日报+关联 sections）、`GetReport(boardID, date)`（查询单篇）、`ListReports(boardID, days)`（查询列表）三个存储接口。

`ListReports` SHALL 在 `days <= 0` 时使用 7 天默认值。对于任意正数 `days`，系统 SHALL 按请求天数查询，不得静默截断到 30 天。

#### Scenario: 保存日报和分组

- **WHEN** 流水线完成生成
- **THEN** 系统 SHALL 创建一条 BoardDailyReport 记录（status="done"）和多条 DailyReportSection 记录（每个聚类一条），并在事务中完成

#### Scenario: 查询最近 7 天日报

- **WHEN** 请求 `ListReports(boardID=5, days=7)`
- **THEN** 系统 SHALL 返回 board #5 最近 7 天的日报列表，按 period_date 倒序

#### Scenario: 查询超过 30 天日报

- **WHEN** 请求 `ListReports(boardID=5, days=42)`
- **THEN** 系统 SHALL 查询最近 42 天，不得将范围截断为 30 天

#### Scenario: 非正数天数使用默认值

- **WHEN** 请求 `ListReports(boardID=5, days=0)`
- **THEN** 系统 SHALL 查询最近 7 天

### Requirement: 日报查询 API

系统 SHALL 提供以下查询端点：
- `GET /api/semantic-boards/:id/daily-reports?days=7`：查询该 board 最近 N 天的日报列表
- `GET /api/daily-reports/:id`：查询单篇日报详情（含关联 sections，每个 section 通过 GORM Preload 包含 threads 列表）
- `GET /api/semantic-boards/:id/section-timeline?days=14`：查询板块 section 时间线（含关系）

`GET /api/semantic-boards/:id/daily-reports` SHALL 在未提供有效正数 `days` 时使用 7 天默认值，并对任意正数 `days` 查询实际请求范围，不得静默截断到 30 天。

`GetReportByID` SHALL 使用嵌套 Preload `"Sections.Threads"` 加载 section 及其关联线程。`DailyReportSection` 的 JSON 响应中 `threads` 字段 SHALL 包含 `[]DailyReportThread` 对象数组（每条含 id、report_id、section_id、title、summary、tag_ids、confidence），section 和 thread 均不含 status/prev_*_id 字段。

#### Scenario: 查询板块日报列表

- **WHEN** 请求 `GET /api/semantic-boards/5/daily-reports?days=7`
- **THEN** 系统 SHALL 返回 board #5 最近 7 天的日报列表，每条含 id、title、summary、period_date、status、cluster_count、article_count

#### Scenario: API 查询超过 30 天

- **WHEN** 请求 `GET /api/semantic-boards/5/daily-reports?days=42`
- **THEN** 系统 SHALL 返回最近 42 天范围内的全部匹配日报

#### Scenario: 查询日报详情含线程

- **WHEN** 请求 `GET /api/daily-reports/42`
- **THEN** 系统 SHALL 返回日报 #42 完整内容，每个 section 包含 threads 列表（每条含 id、title、summary、tag_ids、confidence、related_article_ids），section 和 thread 均不含 status/prev_*_id 字段

#### Scenario: 无日报时返回空

- **WHEN** 请求 `GET /api/semantic-boards/5/daily-reports?days=7`，但该 board 无日报
- **THEN** 系统 SHALL 返回空数组

### Requirement: 日报时间线组件 BoardDailyReportTimeline（报纸布局）

前端 SHALL 提供 `BoardDailyReportTimeline.vue` 组件，替代 `BoardNarrativeTimeline.vue`。组件 SHALL 展示板块日报列表，采用长滚动报纸布局。

**纸张尺寸**：`min(1100px, 92vw)` × `92vh`，单页长滚动（不分页）。

**布局结构**（从上到下）：
1. 报头：日期大标题
2. 今日重点：highlights 展示（title + reason）
3. **质量分区**：按 `best_tier` 将聚类分为区域
   - 核心事件（Tier 0-1）：双列 CSS Grid
   - 相关事件（Tier 2）：单列
   - 其他动态（Tier 3+）：单列
4. 每个分区显示区头标签 + 聚类数

**每个聚类卡片**：默认折叠状态，显示聚类名称、文章数、「N 条线索 ▸」文本。点击卡片或「N 条线索」文本 SHALL 展开显示所有线索（title + summary），线索不显示独立状态徽章。点击 section 的 header 区域（名称） SHALL 打开右侧 SectionLifecyclePanel。

组件顶部 SHALL 提供“话题总览”按钮，点击切换到 BoardThreadBrowser 视图展示板块级话题 DAG 时间线。

**线索文章浮窗**：使用 `@floating-ui/vue`，展示 `related_article_ids` 对应的文章标题列表。首批加载 5 篇，支持“加载更多”。点选文章→emit `openArticle(articleId)`。

`dynamics` 为空时前端不渲染“板块动态”区块。

“加载更早” SHALL 每次将 `days` 增加 7 并重新请求该完整时间范围。组件 SHALL 用响应替换当前列表，避免累计请求造成重复日报。

#### Scenario: 展示日报卡片列表

- **WHEN** 选中 board“AI与机器学习”，该 board 有 3 天的日报
- **THEN** BoardDailyReportTimeline SHALL 渲染 3 张日报卡片，按日期倒序

#### Scenario: 展开日报详情

- **WHEN** 用户点击某日报卡片
- **THEN** 组件 SHALL 展开长滚动报纸布局：highlights 列表、质量分区（核心/相关/其他）

#### Scenario: 核心事件双列布局

- **WHEN** 有 3 个聚类分别属于 tier 0、tier 2、tier 3
- **THEN** tier 0 聚类在“核心事件”双列区域，tier 2 在“相关事件”单列，tier 3 在“其他动态”单列

#### Scenario: 线索默认折叠

- **WHEN** 某聚类有 3 条线索
- **THEN** 该聚类卡片 SHALL 默认只显示文章数和“3 条线索 ▸”，不显示线索标题和摘要

#### Scenario: 展开线索详情

- **WHEN** 用户点击聚类卡片或“N 条线索 ▸”
- **THEN** 卡片 SHALL 展开显示全部线索的 title + summary + 文章图标，线索不显示独立状态徽章

#### Scenario: 切换到话题总览

- **WHEN** 用户点击“话题总览”按钮
- **THEN** SHALL 显示 BoardThreadBrowser 组件，展示话题 DAG 时间线

#### Scenario: 点击 section header 打开 Lifecycle Panel

- **WHEN** 用户点击聚类卡片的 header 区域（名称）
- **THEN** 系统 SHALL 在 viewport 右侧弹出 SectionLifecyclePanel，展示该 section 的跨天生命周期链

#### Scenario: 空状态

- **WHEN** 选中 board 但该 board 无日报
- **THEN** 组件 SHALL 展示“暂无日报”

#### Scenario: 加载超过 30 天

- **WHEN** 当前 `days=28`，用户连续两次点击“加载更早”
- **THEN** 组件 SHALL 依次以 `days=35` 和 `days=42` 请求，并展示对应范围内的日报

#### Scenario: 切换 board 重置时间范围

- **WHEN** 用户从一个 board 切换到另一个 board
- **THEN** 组件 SHALL 将 `days` 重置为 7 并加载新 board 的日报

