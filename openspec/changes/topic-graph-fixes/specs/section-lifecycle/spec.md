## MODIFIED Requirements

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

### Requirement: Section Lifecycle Panel（垂直 DAG，d3-dag 布局）

`SectionLifecyclePanel` SHALL 使用 d3-dag Sugiyama 布局算法 + 自定义 SVG 渲染，以 git log --graph 风格展示话题的完整上下游关系。

**布局引擎**：d3-dag `sugiyama()` 算法，rankdir=TB（从上到下按时间分层），自动计算节点 x/y 坐标和边路径。

**渲染要求**：
- 以选中话题为中心，展示后端返回的完整连通分量
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
- **THEN** 面板 SHALL 显示单个节点“独立话题”，无连线
