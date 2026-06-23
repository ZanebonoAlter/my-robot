## Purpose

Section 级别的生命周期管理：通过 `daily_report_section_relations` 关系表拓扑结构动态推导 section 的 status（emerging/continuing/split/merge）和 ended 标记，提供 timeline 和 lifecycle API，前端展示话题总览 DAG 时间线和 Section Lifecycle Panel。

## Requirements

### Requirement: Section status 动态推导（基于关系表，两阶段）
系统 SHALL 从 `daily_report_section_relations` 表的拓扑关系动态推导 section 的 `status`（关系状态）和 `ended`（结束标记），不再存储 status 字段。

**阶段一：关系状态（status）**：
1. 无 from 关系（无 relation 的 to_section_id 指向它）→ `emerging`
2. 有多个 from 关系（多个旧 section 指向它，to 入度 > 1）→ `merge`
3. 前驱 section 还被其他 section 指向（from 出度 > 1，该 section 是分化出的子叙事之一）→ `split`
4. 有 from 关系且 from 出度 = 1 → `continuing`

**阶段二：结束标记（ended）**：
- 无 to 关系（无 relation 的 from_section_id 指向它）且不是最新一天 → `ended = true`
- 最新一天的 section 即使无 to 关系 SHALL NOT 被标记为 ended

#### Scenario: 新兴叙事
- **WHEN** section #50 无任何 relation 指向它
- **THEN** status SHALL 为 `emerging`，ended SHALL 为 false

#### Scenario: 延续叙事
- **WHEN** section #60 只被 section #50 指向，且 #50 只指向 #60
- **THEN** section #60 的 status SHALL 为 `continuing`

#### Scenario: 叙事分化
- **WHEN** section #50 同时被 section #60 和 #61 指向（#50 出度 = 2）
- **THEN** #60 和 #61 的 status SHALL 为 `split`（它们是从 #50 分化出的子叙事）

#### Scenario: 叙事合并
- **WHEN** section #70 同时被 section #50 和 #55 指向（to 入度 = 2）
- **THEN** section #70 的 status SHALL 为 `merge`

#### Scenario: 延续后结束
- **WHEN** section #50 有 from 关系（status=continuing），无后续 relation，且不是最新一天
- **THEN** section #50 的 status SHALL 为 `continuing`，ended SHALL 为 true

### Requirement: Section Timeline API（含关系）
`GET /api/semantic-boards/:id/section-timeline?days=14` SHALL 返回 sections 扁平列表和 relations 关系列表。SectionTimelineNode 不再包含 `prev_section_id` 字段。

响应格式：
```json
{
  "sections": [{id, report_id, period_date, cluster_label, status, ended, article_count, thread_count}],
  "relations": [{"from_id": 50, "to_id": 60, "distance": 0.25}]
}
```

#### Scenario: 查询板块 section 时间线
- **WHEN** 请求 `GET /api/semantic-boards/5/section-timeline?days=14`
- **THEN** 系统 SHALL 返回 sections 列表和 relations 列表

### Requirement: Section Lifecycle API（适配关系表）
`GET /api/daily-reports/sections/:id/lifecycle` SHALL 基于 `daily_report_section_relations` 表，从目标 section 沿 from/to 关系双向遍历，返回目标 section 所在的完整连通分量。

响应中的 `sections` SHALL 包含连通分量内全部 section；`relations` SHALL 只包含两端都位于该 `sections` 集合中的关系。status 和 ended SHALL 基于完整返回集合及其内部关系推导。

#### Scenario: 查询 section 生命周期含多跳分叉
- **WHEN** 存在 #40 → #50、#50 → #60、#50 → #61，用户请求 #60 的 lifecycle
- **THEN** 系统 SHALL 返回 sections [#40, #50, #60, #61] 和这三条内部 relations

#### Scenario: 查询 section 生命周期含合并
- **WHEN** 存在 #50 → #70、#55 → #70、#70 → #80，用户请求 #50 的 lifecycle
- **THEN** 系统 SHALL 返回 sections [#50, #55, #70, #80] 和这三条内部 relations

#### Scenario: 不返回连通分量外的关系
- **WHEN** 返回 sections 集合为 [#50, #60, #70]
- **THEN** 每条返回 relation 的 from_id 和 to_id SHALL 都位于该集合中

#### Scenario: 孤立 section
- **WHEN** section #20 无任何 relation
- **THEN** 系统 SHALL 返回 sections [#20]，relations 为空

#### Scenario: 环形关系终止遍历
- **WHEN** 存在 #50 → #60、#60 → #70、#70 → #50
- **THEN** 系统 SHALL 各返回三个 section 一次且遍历正常终止

### Requirement: 话题总览组件（DAG 时间线，d3-dag 布局）
前端 `BoardThreadBrowser` SHALL 使用 d3-dag Sugiyama 布局算法 + 自定义 SVG 渲染，展示话题之间的关系网络。

**布局引擎**：d3-dag `sugiyama()` 算法，rankdir=LR（从左到右按时间），自动计算节点 x/y 坐标和边路径。

**渲染要求**：
- 横轴 = 日期列（最近 14 天），节点按 `period_date` 分配到对应列
- 纵轴 = lane（话题链），分支时子话题分配到新 lane，合并时 lane 汇聚
- 节点 = 圆形，颜色按 status：emerging=绿、continuing=蓝、split=橙、merge=紫
- ended=true 的节点用降低透明度或灰色边框标记，保留 status 对应的填充色
- 连线 = SVG path 贝塞尔曲线，基于 d3-dag 计算的边路径点渲染
- 同一天多个 to 节点指向同一 from → 分叉（多条曲线从同一节点扇出到不同 lane）
- 多个 from 节点指向同一 to → 合并（多条曲线汇聚到同一节点）
- 点击节点弹出详情卡片

**无关系的节点**：不参与 DAG 布局，单独以独立圆点形式显示在对应日期列。

#### Scenario: 展示叙事分化
- **WHEN** section #50 (06-01) 被 section #60 和 #61 (06-02) 同时指向（#50 出度 = 2）
- **THEN** DAG SHALL 将 #60 和 #61 分配到不同 lane，从 #50 扇出两条贝塞尔曲线分别到 #60 和 #61，#60 和 #61 显示为橙色

#### Scenario: 展示叙事合并
- **WHEN** section #70 (06-02) 被 section #50 和 #55 (06-01) 同时指向（to 入度 = 2）
- **THEN** DAG SHALL 从 #50 和 #55 所在 lane 各画一条贝塞尔曲线汇聚到 #70，#70 显示为紫色

#### Scenario: 展示已结束叙事
- **WHEN** section #50 (06-01, status=continuing) 无后续 relation，最新天为 06-03
- **THEN** DAG SHALL 以蓝色显示 #50（continuing），但用降低透明度或灰色边框标记 ended=true

#### Scenario: 展示独立话题
- **WHEN** section #80 无任何 relation（emerging，无后续）
- **THEN** DAG SHALL 在对应日期列显示一个独立圆点，不画连线

### Requirement: Section Lifecycle Panel（垂直 DAG，d3-dag 布局）
`SectionLifecyclePanel` SHALL 使用 d3-dag Sugiyama 布局算法 + 自定义 SVG 渲染，以 git log --graph 风格展示话题的完整上下游关系。

**布局引擎**：d3-dag `sugiyama()` 算法，rankdir=TB（从上到下按时间分层），自动计算节点 x/y 坐标和边路径。

**渲染要求**：
- 以选中话题为中心，展示后端 BFS 返回的完整连通子图
- 同一天（同层）的节点水平排列，跨天通过边连接
- 分支点：一个节点扇出多条边到多个子节点（不同 lane）
- 合并点：多条边汇聚到一个节点
- 当前选中的话题（sectionId）高亮显示
- hover 节点时使用共享的有界 BFS 高亮节点和内部关系边
- 节点显示日期、cluster_label、status 徽章、文章/线索数量
- 节点可点击 emit navigate 跳转到对应日报
- 面板宽度 320px，SVG 区域支持垂直滚动
- ended=true 的节点降低透明度

**无关系的孤立话题**：显示单个节点，无连线。

#### Scenario: 展示分叉生命周期
- **WHEN** section #50 有两个后续 section #60 和 #61
- **THEN** 面板 SHALL 将 #60 和 #61 排列在不同 lane，从 #50 扇出两条边分别到 #60 和 #61

#### Scenario: 展示合并生命周期
- **WHEN** section #70 被 section #50 和 #55 同时指向
- **THEN** 面板 SHALL 从 #50 和 #55 各画一条边汇聚到 #70

#### Scenario: 展示多跳生命周期
- **WHEN** API 返回 #40 → #50 → #60 → #70
- **THEN** 面板 SHALL 展示全部四个节点和三条关系边

#### Scenario: 悬停高亮多跳关系
- **WHEN** 面板显示 #50 → #60 → #70，用户悬停 #50
- **THEN** #50、#60、#70 及两条关系边 SHALL 高亮

#### Scenario: 孤立话题
- **WHEN** section #20 无任何 relation
- **THEN** 面板 SHALL 显示单个节点 "独立话题"，无连线

### Requirement: 报纸视图 section 卡片适配
`BoardDailyReportTimeline` 的报纸视图中，section 卡片 SHALL 移除 status 徽章显示。section 的 status 仅在话题总览（BoardThreadBrowser DAG 时间线）和 SectionLifecyclePanel 中展示。

#### Scenario: 报纸视图不显示 section status
- **WHEN** 用户打开日报详情报纸视图
- **THEN** 每个 section 卡片 SHALL 不显示 status 徽章（emerging/continuing 等）

### Requirement: Thread 列表移除 Lineage 入口
`BoardDailyReportTimeline` 的 thread 列表中，SHALL 移除 sitemap 图标按钮（ThreadLineagePanel 入口）。Thread 不再支持血统追踪。

#### Scenario: thread 不显示 lineage 入口
- **WHEN** 用户展开某 section 的 thread 列表
- **THEN** 每个 thread 条目 SHALL 不显示 sitemap 图标按钮
