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

### Requirement: 话题总览工作台（占满 + 工具条 + 弹窗弃用）

前端 `BoardThreadBrowser` 的话题总览 SHALL 升级为占满 `tags-content` 区域的全局工作台（顶部栏 + 左侧版块栏 + 右侧 tabs 的真实布局内，占满 content 宽高），不再悬浮留白。

工作台 SHALL 提供工具条（取代原"日报列表 ↔ 话题总览"切换的单按钮），包含：
- 时间范围选择器（默认 14 天，可选 7 / 30 / 全部），切换时重载总览数据。
- 视图模式分段（时间线 timeline / 泳道 lanes）。
- 回刷归属（原 `TopicManageDialog` 能力迁入）。
- 合并预览（原 `TopicManageDialog` 能力迁入）。
- 新建泳道（进入编排态，见下条 Requirement）。

lanes 视图 SHALL 占满工作台主体：左侧泳道标签列（状态点 + 名 + 计数 + 跨度/最近日期）+ 右侧时间网格（按时间范围选择器窗口渲染列数）。同一天多条 section 节点 SHALL 维持纵向堆叠（基于现有 `subOffset` + 自适应 `laneH`，本变更不改算法，只要求布局撑满）。

系统 SHALL 弃用 `TopicManageDialog.vue`：其全部能力（回刷 / 重命名 / 归档 / 合并 / 删除）SHALL 迁移到工作台工具条与泳道 hover 操作菜单。hover 任意泳道 SHALL 出现就地操作（重命名 / 归档或恢复 / 删除），`source=manual` 与 `source=auto` 话题在 hover 操作上一视同仁。

#### Scenario: 总览占满 content

- **WHEN** 用户进入话题总览
- **THEN** lanes 视图 SHALL 占满 content 宽高（无悬浮留白），泳道多时纵向滚动

#### Scenario: 时间范围切换

- **WHEN** 用户将时间范围从"最近 14 天"切到"最近 30 天"
- **THEN** 总览 SHALL 重载并渲染 30 天的泳道与日期列

#### Scenario: 弹窗能力迁入工具条

- **WHEN** 用户点击工具条"回刷归属"
- **THEN** 系统 SHALL 触发回刷（原弹窗能力），SHALL NOT 打开 `TopicManageDialog`

#### Scenario: 泳道就地操作

- **WHEN** 用户 hover 某条 active 话题泳道
- **THEN** SHALL 出现重命名 / 归档 / 删除操作按钮；点击归档 SHALL 将该话题转 archived（不弹窗）

#### Scenario: 同天多节点纵向堆叠

- **GIVEN** 06-20 有 2 条 section 同属话题「中东局势」
- **WHEN** lanes 渲染该泳道
- **THEN** 06-20 列 SHALL 显示 2 个纵向堆叠的节点（横向位置相同、纵向错开），SHALL NOT 横向并列

### Requirement: 手动建泳道编排态（预览 + 候选池 + 体检报告）

用户点击工具条"新建泳道"SHALL 进入编排态（`viewMode='compose'`）。编排态 SHALL 提供三栏：预览泳道时间轴（顶）+ 候选 section 池（左）+ 体检报告（右）。

**预览泳道时间轴** SHALL 实时反映候选池中已勾选的 section：按 period_date 排列节点，节点三态按"该 section embedding 到当前选中集合聚合锚点的距离"判定——贴合（实心）/ 边界（虚线）/ 离群（黄框，距离 > match_threshold × 1.3）。同一天多节点 SHALL 纵向堆叠（同 lanes）。预览泳道 SHALL 支持按住拖拽平移查看更多日期。

**候选 section 池** SHALL 列出时间范围选择器窗口内的全部 section（默认最近 14 天），支持多选；每条 SHALL 显示标题、到聚合锚点的距离 + 贴合/边界/离群/远 标签、原属话题。离群 section SHALL 标黄背景 + "建议剔除"提示。

**体检报告** SHALL 提供三张卡片作为人工评判依据：
1. 聚类质量：选中数、平均两两距离、离群数、"一键剔除离群"按钮（用户主动点击，不自动删）。
2. 撞车检查：选中 section 中当前归属分布、将从原话题移出的数量（明确提示"N 条将从原话题移出"）、最近现有话题距离。
3. 未来预期（v1 淡显"规划中"，不实现）：历史 section 潜在命中预览。

编排态 SHALL 提供泳道名输入（`AppInput`，可改）、保存/取消（`AppButton`）、撞车确认（`AppDialog`），SHALL NOT 使用 `window.alert/prompt/confirm`。

保存 SHALL 触发手动建泳道 API（label + 选中 section_ids），成功后返回总览态并刷新（新泳道以 active + source=manual 出现在 lanes）。

#### Scenario: 预览泳道实时反映勾选

- **GIVEN** 编排态候选池勾选了 5 条 section
- **WHEN** 用户取消勾选其中 1 条
- **THEN** 预览泳道 SHALL 移除该节点并重排，聚合锚点 SHALL 重算（剩余 4 条的 mean 向量），其余节点三态可能变化

#### Scenario: 离群标黄不自动删

- **GIVEN** 勾选 5 条，其中「荷兰扩 ASML 限制」到锚点距离 0.41 > 1.3×0.30
- **WHEN** 渲染候选池与预览
- **THEN** 该条 SHALL 标黄 + "建议剔除"，但 SHALL 仍保持勾选状态（不自动移除），由用户决定

#### Scenario: 撞车明确提示移出

- **GIVEN** 勾选 5 条，其中 3 条当前归属「中东局势」
- **WHEN** 渲染体检报告撞车卡
- **THEN** SHALL 显示"3 条将从『中东局势』移出、归入新泳道（单值覆盖）"

#### Scenario: manual confidence 节点样式区分

- **GIVEN** 手动建好的 topic #20 下有 section（confidence=manual）
- **WHEN** 总览 lanes 渲染 #20 的节点
- **THEN** 该节点 SHALL 用独立样式（双环描边）区分于算法三态（实心/虚线/空心），hover 显示"人工归属"，SHALL NOT 套用算法 distance 三态样式

#### Scenario: 保存后返回总览

- **WHEN** 用户填名"美伊博弈"并点保存，API 成功
- **THEN** 系统 SHALL 退出编排态、刷新总览，"美伊博弈"泳道以 active 出现在 lanes 顶部

### Requirement: 编排态候选池语义搜索（渐进收敛排序）

候选 section 池在候选条目较多时 SHALL 提供自然语言语义搜索，帮用户从大量候选（默认时间窗口内全部 section）中快速定位相关条目，并通过渐进收敛的排序降低人工挑选成本：

- **搜索框**：候选池顶部 SHALL 提供自然语言搜索输入框（`AppInput`）。用户输入文本并停顿（debounce）后，系统 SHALL 调用文本嵌入端点（`POST /persistent-topics/embed-query`）获取查询向量，并按"查询向量 ↔ 各候选 embedding"的 cosine 距离升序重排未选中候选（最相关的浮顶）。
- **勾选即接管排序**：一旦用户勾选了任意候选，排序主信号 SHALL 切换为"已选集合的聚合向量（mean pooling，镜像 `aggregatePreview`）"——已选是用户确证的信号，优先级高于文本查询；此时未选中候选 SHALL 按到聚合锚点的距离升序重排。勾选更多候选时锚点 SHALL 持续重算、列表 SHALL 持续重排（渐进收敛）。
- **文本框降级**：勾选后文本搜索框 SHALL 保留可见但不再作为主排序信号；清空文本不影响已选聚合排序。
- **已选置顶分组**：已勾选候选 SHALL 置顶分组显示（标"已选 N"），未勾选候选 SHALL 按命中率排序在其下方，方便用户查看已选与待选。
- **默认排序**：未输入文本且未勾选任何候选时，候选池 SHALL 回退默认排序（按 `period_date` 倒序，最新在前）。
- **模型一致性**：文本嵌入端点 SHALL 复用与 section embedding 相同的全局模型（`CapabilityEmbedding`），保证 cosine 相似度可比。
- **失败降级**：文本嵌入端点调用失败或返回空向量时 SHALL NOT 阻断编排——回退默认排序并给出轻量错误提示，用户仍可手动浏览勾选。

#### Scenario: 文本搜索冷启动排序

- **GIVEN** 编排态候选池有 40 条候选，未勾选任何
- **WHEN** 用户在搜索框输入"半导体出口管制"并停顿
- **THEN** 系统 SHALL 调用嵌入端点获取查询向量，未选中候选 SHALL 按到查询向量的 cosine 距离升序重排，标题与"半导体"语义相近的候选浮顶

#### Scenario: 勾选后聚合向量接管排序

- **GIVEN** 用户已输入搜索文本并勾选了 2 条相关候选
- **WHEN** 候选池重排
- **THEN** 排序主信号 SHALL 切换为这 2 条的聚合向量（mean pooling），未选中候选 SHALL 按到聚合锚点的距离重排；文本查询向量不再决定主排序

#### Scenario: 渐进收敛

- **GIVEN** 勾选 2 条，列表按聚合锚点排序
- **WHEN** 用户再勾选 1 条
- **THEN** 聚合锚点 SHALL 用 3 条重算，未选中候选 SHALL 按新锚点距离重排

#### Scenario: 已选置顶分组

- **GIVEN** 勾选了 3 条候选
- **THEN** 这 3 条 SHALL 置顶显示为一组（标"已选 3"），其余未选候选 SHALL 在其下方按命中率排序

#### Scenario: 清空回退默认排序

- **GIVEN** 用户清空搜索框且取消全部勾选
- **THEN** 候选池 SHALL 回退按 `period_date` 倒序的默认排序

#### Scenario: 搜索失败不阻断

- **WHEN** 嵌入端点返回错误或空向量
- **THEN** 候选池 SHALL 回退默认排序 + 显示轻量错误提示，SHALL NOT 阻塞勾选与保存流程

### Requirement: 编排态候选话题引导（连续命中候选的一键激活/并入）

编排态 SHALL 在候选 section 池上方提供「连续命中候选话题」引导区，把 board 内 `status=candidate` 的持久化话题直接摆出来，作为比「从 section 池逐条勾选」更直接的编排入口（迁移自原 `TopicManageDialog` 的候选激活能力）。

引导区 SHALL 列出当前 board 的全部 candidate 话题，每条 SHALL 显示 label、连续命中天数（`consecutive_hits`）、所含 section 数（`section_count`）。已达 `upgrade_threshold`（`can_activate=true`）的候选 SHALL 置顶并高亮，与未达标候选视觉区分。

每条候选 SHALL 提供两个动作：
1. **确认启用**：仅 `can_activate=true`（`consecutive_hits >= upgrade_threshold`）时可点。点击 SHALL 调用话题状态更新（status→active），成功后 SHALL 经事件通知父组件刷新总览（新 active 话题以泳道出现在 lanes）。未达标时该按钮 SHALL 禁用并提示「需先满足连续多天出现条件」。
2. **并入新泳道**：点击 SHALL 把候选 section 池中 `persistent_topic_id` 等于该候选的 section 全部加入当前编排选中集（并入正在新建的泳道），为纯前端操作，SHALL NOT 调用任何 API。窗口外或不在此候选池中的 section SHALL 不受影响。

board 无 candidate 话题时，引导区 SHALL 隐藏（不占位）。

#### Scenario: 引导区列出连续命中候选

- **GIVEN** board 有候选话题「美伊博弈」（consecutive_hits=3，section_count=4，can_activate=true）与「油价波动」（consecutive_hits=1，section_count=1，can_activate=false）
- **WHEN** 用户进入编排态
- **THEN** 引导区 SHALL 列出两条候选，「美伊博弈」因 can_activate=true SHALL 置顶高亮，「油价波动」的「确认启用」按钮 SHALL 禁用

#### Scenario: 一键激活达标候选

- **GIVEN** 候选「美伊博弈」can_activate=true
- **WHEN** 用户点「确认启用」
- **THEN** 系统 SHALL 调用状态更新将其转 active，成功后 SHALL 刷新总览，「美伊博弈」以 active 泳道出现在 lanes

#### Scenario: 并入新泳道选中相关 section

- **GIVEN** 编排态已勾选 1 条 section，候选「美伊博弈」另有 3 条 section 在当前候选池窗口内
- **WHEN** 用户点「美伊博弈」的「并入新泳道」
- **THEN** 那 3 条 section SHALL 被加入选中集（选中数变 4），预览泳道与体检报告 SHALL 实时重算；SHALL NOT 发起任何 API 请求

#### Scenario: 无候选时引导区隐藏

- **GIVEN** board 无 status=candidate 话题
- **THEN** 编排态引导区 SHALL 隐藏，不占用布局空间

#### Scenario: 已中断候选单列分组

- **GIVEN** 候选话题「断连续」consecutive_hits=0（近期未命中），「美伊博弈」consecutive_hits=1
- **WHEN** 渲染引导区
- **THEN** 引导区 SHALL 分两组：「正在连续命中」（含「美伊博弈」）与「已中断·近期未命中」（含「断连续」）；已中断组 SHALL 显示「近期未命中」而非「连续 0 天」，且 SHALL NOT 提供「确认启用」按钮（仅「并入新泳道」），视觉弱化

### Requirement: 编排态已勾选 section 查看线索

编排态候选 section 池中，**已被勾选**的 section SHALL 提供就地「查看线索」入口，让用户在决定是否将某条 section 串进新泳道前，能看到该 section 包含哪些线索（thread）及其文章数，解决「勾选前不知道这条 section 具体讲什么」的判断缺口。未勾选的 section SHALL NOT 显示该入口。

点选「查看线索」SHALL 就地在候选条目下方展开线索列表（不弹窗、不跳转）。列表数据 SHALL 通过复用日报详情端点（`getDailyReportDetail(report_id)`）加载——编排态候选 section 的 `report_id` SHALL 由 `compose-candidates` 端点随 section 一并返回。每条线索 SHALL 显示标题与文章数；编排态 SHALL NOT 在线索列表内再展开文章正文（编排态聚焦「挑 section」，读文章走总览）。

线索加载 SHALL 有 loading 态与失败降级（失败时给出轻量提示，SHALL NOT 阻断勾选/保存流程）。再次点选同一 section 的「查看线索」SHALL 收起；切换到另一已勾选 section SHALL 切换内容（单选展开）。

取消勾选某条 section SHALL 收起其线索区（避免展开态悬空）。

#### Scenario: 已勾选才显示入口

- **GIVEN** 候选池有 section A（未勾选）与 B（已勾选）
- **THEN** B 下方 SHALL 出现「查看线索」入口，A SHALL NOT 出现该入口

#### Scenario: 就地展开看线索

- **GIVEN** section B 已勾选
- **WHEN** 用户点 B 的「查看线索」
- **THEN** SHALL 就地在 B 下方展开线索列表（不弹窗/不跳转），调用 `getDailyReportDetail(B.report_id)` 拿到线索，每条显示标题 + 文章数

#### Scenario: 不展开文章正文

- **THEN** 编排态线索列表 SHALL 只显示线索标题与文章数，SHALL NOT 提供文章正文展开入口

#### Scenario: 切换单选

- **GIVEN** B 的线索已展开
- **WHEN** 用户点另一已勾选 section C 的「查看线索」
- **THEN** 线索内容 SHALL 切换为 C 的线索，SHALL NOT 同时展开两条

#### Scenario: 取消勾选收起

- **GIVEN** B 的线索已展开
- **WHEN** 用户取消勾选 B
- **THEN** B 的线索区 SHALL 收起（不悬空）

#### Scenario: 加载失败不阻断

- **WHEN** `getDailyReportDetail` 返回错误
- **THEN** SHALL 显示轻量错误提示，SHALL NOT 阻断勾选与保存流程

#### Scenario: report_id 由端点返回

- **WHEN** `compose-candidates` 返回候选 section
- **THEN** 每条 section SHALL 含 `report_id` 字段（供编排态查线索复用）

