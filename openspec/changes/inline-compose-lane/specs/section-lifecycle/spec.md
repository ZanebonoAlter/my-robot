## MODIFIED Requirements

### Requirement: 手动建泳道编排态（预览 + 候选池 + 体检报告）

用户点击「新建泳道」SHALL 进入编排态。编排态 SHALL 为 lanes 视图上的 `composeMode` 布尔**叠加态**，SHALL NOT 切换 `viewMode`（不再使用 `viewMode='compose'` 全屏替换）。「新建泳道」入口 SHALL 同时存在于工作台工具条与 unassigned（待确认/未分类）泳道头部（主战场入口）。

**就地叠加布局**：编排态 SHALL 保持 lanes 泳道主视图可见——已有 active 泳道 SHALL 淡显（低 opacity、约 0.3）保留为背景参照，SHALL NOT 被整体盖掉。编排态 SHALL 提供顶部浮工具条（泳道名输入 + 已勾计数 + 「聚类质量」单卡 + 取消/保存）与右侧滑出候选侧边栏，SHALL NOT 提供原全屏三栏（预览时间轴 / 候选 section 池 / 体检报告三卡）。

**主战场就地勾选**：unassigned 泳道 SHALL 为编排主战场，其每个 section 节点 SHALL 渲染 checkbox 支持就地勾选。勾选/取消勾选 SHALL 实时重算聚合锚点（`aggregatePreview`，mean pooling）并重算全员 `cosineDistance`：节点 SHALL 标注 distance 数字 + 边框分层（`distanceTier`：good / boundary / outlier）+ 离群标黄（`outlierFlags`，distance > match_threshold × 1.3，标黄但 SHALL 保持勾选状态由用户决定，不自动移除）。同一天多节点 SHALL 维持纵向堆叠。

**active 泳道可勾走（成员移出）**：淡显 active 泳道中的 section 节点 SHALL 亦可勾选，勾选语义为「从原 active 泳道移出到新泳道」。勾选 active section 时节点 SHALL 实时标注「将从【原泳道名】移出」，原泳道名 SHALL 取自候选数据的 `persistentTopic.label`。

**聚类质量单卡**：顶部浮工具条 SHALL 提供单张「聚类质量」卡（取代原三卡），实时展示成员数 / 平均距离 / 离群数。数据计算：`aggregatePreview` 取 mean → 各选中向量 `cosineDistance(v, mean)` 求平均 → `outlierFlags(distances, threshold)` 计离群数。原「撞车检查」卡与「未来预期」卡 SHALL NOT 保留（未来预期 v1 本就未实现；撞车改为保存时动作提示，见下）。

**保存与移出确认**：编排态 SHALL 提供泳道名输入（`AppInput`）、保存/取消（`AppButton`），SHALL NOT 使用 `window.alert/prompt/confirm`。保存 SHALL 触发手动建泳道 API（`createManualLane`，label + 选中 section_ids）；section 重指新 topic 即自动离开原 active 泳道，无需额外 API。**存在 active 移入项时 SHALL 先弹二次确认**，列出全部将从原 active 泳道移出的 section（含原泳道名），用户确认后才提交；无移入项可直接保存。成功后 SHALL 退出编排态、刷新总览，新泳道以 active + source=manual 出现在 lanes，被勾 section 从 unassigned / 原 active 泳道移出。

**取消**：取消 SHALL 清空勾选与名字、退出编排态，SHALL 无副作用（不发 API）。

编排态 SHALL 一次只建一条新泳道（不支持并发编排多条）。

#### Scenario: 进入编排态不切视图

- **GIVEN** 用户在 lanes 视图查看现有泳道
- **WHEN** 用户点 unassigned 泳道头部「新建泳道」
- **THEN** SHALL 进入 `composeMode` 叠加态，`viewMode` SHALL 保持 `lanes` 不变；active 泳道 SHALL 淡显保留可见，SHALL NOT 被全屏视图盖掉

#### Scenario: unassigned 节点就地勾选实时贴合度

- **GIVEN** 编排态已勾选 unassigned 中 5 条 section
- **WHEN** 用户取消勾选其中 1 条
- **THEN** 聚合锚点 SHALL 用剩余 4 条重算（mean 向量），全员 distance SHALL 重算，节点 distance 数字 / 边框分层 / 离群标黄 SHALL 相应更新

#### Scenario: 离群标黄不自动删

- **GIVEN** 勾选 5 条，其中「荷兰扩 ASML 限制」到锚点距离 0.41 > 1.3×0.30
- **WHEN** 渲染节点
- **THEN** 该节点 SHALL 标黄 + 提示建议剔除，但 SHALL 保持勾选状态（不自动移除），由用户决定

#### Scenario: 勾走 active section 标移出提示

- **GIVEN** 编排态淡显背景含 active 泳道「中东局势」，其中 1 条 section 被勾选
- **WHEN** 该节点被勾选
- **THEN** 节点 SHALL 实时标注「将从【中东局势】移出」，顶部计数 SHALL 反映「N 个来自现有泳道，将移出」

#### Scenario: 聚类质量单卡实时

- **GIVEN** 编排态勾选 5 条
- **THEN** 顶部「聚类质量」卡 SHALL 显示成员数 5 / 平均距离 / 离群数，随勾选实时更新；SHALL NOT 显示原「撞车检查」「未来预期」卡

#### Scenario: 保存前移出二次确认

- **GIVEN** 勾选含 3 条来自 active 泳道「中东局势」的 section
- **WHEN** 用户点保存
- **THEN** SHALL 先弹二次确认列出「3 条将从『中东局势』移出」，用户确认后才调 `createManualLane`；无移入项时 SHALL 直接保存不弹确认

#### Scenario: 保存后新泳道出现并移出

- **WHEN** 用户填名「美伊博弈」并确认保存，API 成功
- **THEN** SHALL 退出编排态、刷新总览，「美伊博弈」以 active 出现在 lanes；被勾的 unassigned section 与从「中东局势」勾走的 section SHALL 均归入新泳道、从原位置移出

#### Scenario: 取消无副作用

- **GIVEN** 编排态勾选若干 section 并填名
- **WHEN** 用户点取消
- **THEN** SHALL 清空勾选与名字、退出编排态，SHALL NOT 发起任何 API 请求

#### Scenario: manual confidence 节点样式区分

- **GIVEN** 手动建好的 topic #20 下有 section（confidence=manual）
- **WHEN** 总览 lanes 渲染 #20 的节点
- **THEN** 该节点 SHALL 用独立样式（双环描边）区分于算法三态（实心/虚线/空心），hover 显示「人工归属」，SHALL NOT 套用算法 distance 三态样式

### Requirement: 编排态候选池语义搜索（渐进收敛排序）

编排态 SHALL 在右侧候选侧边栏提供自然语言语义搜索，帮用户从大量未归类 section（unassigned 主战场）中快速定位相关条目，并通过渐进收敛的贴合度分层降低人工挑选成本（原「候选池列表排序」演化为「就地节点分层 + 侧边栏搜索」）：

- **侧边栏搜索框**：右侧候选侧边栏 SHALL 提供自然语言搜索输入框（`AppInput`）。用户输入文本并停顿（debounce）后，系统 SHALL 调用文本嵌入端点（`POST /persistent-topics/embed-query`）获取查询向量，并按「查询向量 ↔ 各未勾选 section embedding」的 cosine 距离对 unassigned 节点做相关度高亮/置顶提示（最相关的视觉强提示）。
- **勾选即接管主信号**：一旦用户勾选任意 section，主信号 SHALL 切换为「已选集合的聚合向量（mean pooling，镜像 `aggregatePreview`）」——已选是用户确证信号，优先级高于文本查询；此时所有节点 SHALL 按到聚合锚点的距离重算 distance/tier（取代查询向量排序）。勾选更多时锚点 SHALL 持续重算、节点分层 SHALL 持续更新（渐进收敛）。
- **搜索框降级**：勾选后搜索框 SHALL 保留可见但不再作为主信号；清空文本不影响已选聚合分层。
- **默认分层**：未输入文本且未勾选任何 section 时，节点 SHALL 回退默认（按 `period_date` 倒序的自然 lanes 布局，无额外相关度分层）。
- **模型一致性**：文本嵌入端点 SHALL 复用与 section embedding 相同的全局模型（`CapabilityEmbedding`），保证 cosine 相似度可比。
- **失败降级**：文本嵌入端点失败或返回空向量时 SHALL NOT 阻断编排——回退默认分层并给轻量错误提示，用户仍可手动浏览勾选。

#### Scenario: 文本搜索冷启动高亮

- **GIVEN** 编排态 unassigned 有 40 条 section，未勾选任何
- **WHEN** 用户在侧边栏搜索框输入「半导体出口管制」并停顿
- **THEN** SHALL 调用嵌入端点获取查询向量，与「半导体」语义相近的未勾选节点 SHALL 视觉强提示（高亮/置顶提示），SHALL 不改变 `viewMode`

#### Scenario: 勾选后聚合向量接管分层

- **GIVEN** 用户已输入搜索文本并勾选了 2 条相关 section
- **WHEN** 节点分层重算
- **THEN** 主信号 SHALL 切换为这 2 条的聚合向量（mean pooling），所有节点 SHALL 按到聚合锚点的 distance/tier 重算；文本查询向量不再决定主信号

#### Scenario: 渐进收敛

- **GIVEN** 勾选 2 条，节点按聚合锚点分层
- **WHEN** 用户再勾选 1 条
- **THEN** 聚合锚点 SHALL 用 3 条重算，节点分层 SHALL 按新锚点更新

#### Scenario: 清空回退默认

- **GIVEN** 用户清空搜索框且取消全部勾选
- **THEN** 节点 SHALL 回退默认 lanes 布局（按 `period_date` 倒序），无额外相关度分层

#### Scenario: 搜索失败不阻断

- **WHEN** 嵌入端点返回错误或空向量
- **THEN** SHALL 回退默认分层 + 显示轻量错误提示，SHALL NOT 阻塞勾选与保存流程

### Requirement: 编排态候选话题引导（连续命中候选的一键激活/并入）

编排态 SHALL 在右侧候选侧边栏提供「连续命中候选话题」区（原「候选池上方引导区」演化为侧边栏），把 board 内 `status=candidate` 的持久化话题摆出来，作为比「从 unassigned 逐条勾选」更直接的编排入口（迁移自原 `TopicManageDialog` 的候选激活能力）。

侧边栏 SHALL 列出当前 board 的全部 candidate 话题，每条 SHALL 显示 label、连续命中天数（`consecutive_hits`）、所含 section 数（`section_count`）。已达 `upgrade_threshold`（`can_activate=true`）的候选 SHALL 置顶并高亮，与未达标候选视觉区分。

每条候选 SHALL 提供两个动作：
1. **确认启用**：仅 `can_activate=true`（`consecutive_hits >= upgrade_threshold`）时可点。点击 SHALL 调用话题状态更新（status→active），成功后 SHALL 刷新总览（新 active 话题以泳道出现在 lanes）。未达标时该按钮 SHALL 禁用并提示「需先满足连续多天出现条件」。
2. **采纳（并入新泳道）**：点击 SHALL 预填新泳道名输入框（用该候选 label）+ 把 unassigned 中与该候选 centroid 距离在 `matchThreshold` 内的 section 预勾加入当前选中集（一键起步；预勾数量超过上限时 SHALL 截断并提示）。此为纯前端操作，SHALL NOT 调用任何 API。窗口外或不在此候选范围内的 section SHALL 不受影响。

board 无 candidate 话题时，侧边栏该区 SHALL 隐藏（不占位）。

#### Scenario: 侧边栏列出连续命中候选

- **GIVEN** board 有候选话题「美伊博弈」（consecutive_hits=3，section_count=4，can_activate=true）与「油价波动」（consecutive_hits=1，section_count=1，can_activate=false）
- **WHEN** 用户进入编排态
- **THEN** 右侧侧边栏 SHALL 列出两条候选，「美伊博弈」因 can_activate=true SHALL 置顶高亮，「油价波动」的「确认启用」按钮 SHALL 禁用

#### Scenario: 一键激活达标候选

- **GIVEN** 候选「美伊博弈」can_activate=true
- **WHEN** 用户点「确认启用」
- **THEN** SHALL 调用状态更新将其转 active，成功后 SHALL 刷新总览，「美伊博弈」以 active 泳道出现在 lanes

#### Scenario: 采纳预填名并预勾相关 section

- **GIVEN** 编排态已勾选 1 条 section，候选「美伊博弈」centroid 附近有 3 条 unassigned section 在 matchThreshold 内
- **WHEN** 用户点「美伊博弈」的「采纳」
- **THEN** 泳道名输入框 SHALL 预填「美伊博弈」，那 3 条 section SHALL 被预勾加入选中集（选中数变 4），聚合锚点与聚类质量卡 SHALL 实时重算；SHALL NOT 发起任何 API 请求

#### Scenario: 无候选时侧边栏该区隐藏

- **GIVEN** board 无 status=candidate 话题
- **THEN** 右侧侧边栏候选话题区 SHALL 隐藏，不占用布局空间

#### Scenario: 已中断候选单列分组

- **GIVEN** 候选话题「断连续」consecutive_hits=0（近期未命中），「美伊博弈」consecutive_hits=1
- **WHEN** 渲染侧边栏候选区
- **THEN** SHALL 分两组：「正在连续命中」（含「美伊博弈」）与「已中断·近期未命中」（含「断连续」）；已中断组 SHALL 显示「近期未命中」而非「连续 0 天」，且 SHALL NOT 提供「确认启用」按钮（仅「采纳」），视觉弱化
