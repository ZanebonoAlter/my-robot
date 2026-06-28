## ADDED Requirements

### Requirement: 日报阅读分区状态快照

系统 SHALL 为新生成的 `DailyReportSection` 持久化可空字段 `topic_status_at_report`，记录 section 完成 PersistentTopic 归属时对应话题的 `candidate|active` 状态。该字段 SHALL 与 `persistent_topic_id`、`topic_match_distance`、`topic_match_confidence` 在同一事务内写入，并通过日报详情 API 返回。

历史 section 缺少快照时，API SHALL 返回 null；系统 SHALL NOT 使用 PersistentTopic 的当前状态反推历史快照。

#### Scenario: 归属 active 话题时写入快照

- **GIVEN** section 在日报保存时归属到 status=active 的 PersistentTopic
- **WHEN** 系统持久化归属结果
- **THEN** section.topic_status_at_report SHALL 写为 active

#### Scenario: candidate 后续转 active 不改变历史快照

- **GIVEN** 某历史 section.topic_status_at_report=candidate
- **WHEN** 对应 PersistentTopic 后续由用户确认为 active
- **THEN** 该历史 section.topic_status_at_report SHALL 仍为 candidate

#### Scenario: 旧数据保守降级

- **GIVEN** 历史 section.topic_status_at_report 为 null，但其 PersistentTopic 当前为 active
- **WHEN** 查询并展示该历史日报
- **THEN** 系统 SHALL 将该 section 归入“其他动态”
- **AND** SHALL NOT 根据当前 active 状态把它放入“关心的话题”

## MODIFIED Requirements

### Requirement: 日报时间线组件 BoardDailyReportTimeline（报纸布局）
前端 SHALL 提供 `BoardDailyReportTimeline.vue` 组件，展示板块日报列表，并以全屏长滚动阅读层呈现选中日报。详情 SHALL 保持在 TagsPage 内，不新增独立路由。

**详情容器**：使用 `position: fixed; inset: 0` 占满 viewport，提供明确关闭按钮、Esc 关闭、背景滚动锁和焦点恢复。关闭详情 SHALL 返回原日报列表位置。

**宽屏布局**：大于 1100px 时采用 sticky 边栏 + 正文双栏结构。边栏包含本期目录、active 话题索引和历史日报日期；头条与 highlights 可跨正文通栏。721px–1100px SHALL 降为单栏并把目录/日期收拢到正文顶部；720px 及以下 SHALL 保持单栏且不得产生页面级横向溢出。

**主题**：详情的 surface、文字、边框、阴影和交互状态 SHALL 使用现有 semantic theme token，并同时支持 editorial/dark。topic 的稳定 `persistent_topic.color` MAY 用作身份强调色。组件 SHALL NOT 以固定浅色背景为前提硬编码黑色透明色。

**内容映射**：
1. masthead 展示真实日报标题、日期、article_count 和 cluster_count；刊头 SHALL NOT 固定声称“号外”。
2. 头条优先取 `highlights[0]`；highlights 为空时回退到质量最高的 section，且 SHALL NOT 生成 API 中不存在的新闻文案。
3. highlights 按 API 顺序最多展示三项；为空时不渲染该区块。
4. section SHALL 按 `topic_status_at_report` 分为“关心的话题”(active)和“其他动态”(candidate、archived、null)。页面 SHALL NOT 渲染“突发的新话题”或其他 candidate 专属主分区。分区内继续按 best_tier 升序、avg_score 降序排列；candidate 状态 SHALL NOT 提供额外排序权重。
5. `dynamics` 为空时不渲染“板块动态”区块。

**话题卡片与 mini 生命线**：active 话题卡片 SHALL 支持原位展开。首次展开调用 `getTopicLifeline(topicId)`，展示以当前日报日期为终点的最近七个自然日；加载成功后按 topic id 缓存，重复展开不再次请求。加载、失败、重试和空状态 SHALL 明确可见。

mini 生命线 SHALL 按日期列展示真实 section，同日多 section 合并为一个节点并显示数量。连线只使用响应中 `relation_type="identity"` 的真实关系，采用贝塞尔路径连接端点；空白日期不生成假节点，系统 SHALL NOT 根据日期相邻关系臆造连线或混入 similarity relation。节点点击 SHALL 原位展开当日 threads。

active topic SHALL 提供进入侦探墙完整生命线的出口；无 topic id 或设备不支持该入口时隐藏。现有“话题总览”入口 SHALL 继续打开 BoardThreadBrowser。

**thread 文章**：thread 有 `related_article_ids` 时 SHALL 可原位展开文章标题列表。前端只加载尚未缓存的 article id，相同文章跨 thread 复用缓存；单篇失败不阻塞其他文章并提供重试。点击文章 SHALL `emit('openArticle', articleId)`。

**section 生命周期**：点击 section header SHALL 继续打开右侧 SectionLifecyclePanel，不因新增 topic mini 生命线而移除 section 维度入口。

“加载更早” SHALL 每次将 `days` 增加 7 并重新请求该完整时间范围。组件 SHALL 用响应替换当前列表，避免累计请求造成重复日报。切换 board SHALL 将 `days` 重置为 7，并清理与旧 board 相关的展开状态。

#### Scenario: 展示日报卡片列表
- **WHEN** 选中 board “AI与机器学习”，该 board 有 3 天的日报
- **THEN** BoardDailyReportTimeline SHALL 渲染 3 张日报卡片，按日期倒序

#### Scenario: 打开全屏日报详情
- **WHEN** 用户点击某日报卡片
- **THEN** 组件 SHALL 在当前 TagsPage 内打开占满 viewport 的长滚动日报阅读层
- **AND** 关闭后 SHALL 返回原日报列表位置

#### Scenario: 宽屏双栏与窄屏降级
- **WHEN** 分别以 1440px、1000px 和 720px viewport 打开同一日报
- **THEN** 1440px SHALL 显示 sticky 边栏与正文双栏
- **AND** 1000px 与 720px SHALL 使用单栏降级，720px 不产生页面级横向溢出

#### Scenario: 双主题渲染
- **WHEN** 用户在 editorial 与 dark 主题间切换
- **THEN** 日报 surface、文字、边框、SVG 和交互状态 SHALL 使用对应主题语义 token 并保持可读

#### Scenario: candidate 不再形成注意力分区
- **WHEN** 日报包含 2 个 topic_status_at_report=active、1 个 candidate 和 1 个 null section
- **THEN** 页面 SHALL 在“关心的话题”展示 2 个 section
- **AND** SHALL 在“其他动态”展示 candidate 与 null 共 2 个 section
- **AND** 页面 SHALL NOT 渲染“突发的新话题”分区或 candidate 状态徽章

#### Scenario: 分区使用报告时快照
- **GIVEN** 某 section.topic_status_at_report=active，而对应 PersistentTopic 当前已 archived
- **WHEN** 用户打开该历史日报
- **THEN** section SHALL 仍显示在“关心的话题”

#### Scenario: 展开并缓存话题生命线
- **WHEN** 用户首次展开一个 active 话题卡片并在加载完成后收起、再次展开
- **THEN** 系统 SHALL 仅调用一次 `getTopicLifeline(topicId)`
- **AND** 再次展开 SHALL 使用缓存结果

#### Scenario: identity 连线跨越空白日期
- **WHEN** lifeline 在周一和周三有节点、周二无节点，且响应包含周一到周三的 identity relation
- **THEN** mini 生命线 SHALL 用一条贝塞尔路径连接周一和周三的真实节点
- **AND** SHALL NOT 为周二生成假节点
- **AND** 该跨天连线 SHALL 以弱化不透明度呈现，与相邻节点的强连线区分

#### Scenario: lifeline 加载失败
- **WHEN** `getTopicLifeline(topicId)` 返回错误
- **THEN** 话题卡片 SHALL 显示局部错误和重试操作，不关闭日报详情或影响其他话题

#### Scenario: 展开 thread 文章并复用缓存
- **WHEN** 两个 thread 引用同一个 article id，用户依次展开两个 thread
- **THEN** 前端 SHALL 只为该 article id 请求一次文章详情
- **AND** 点击文章 SHALL 发出 `openArticle(articleId)`

#### Scenario: 进入完整话题生命线
- **WHEN** active topic 具有 topic id 且当前设备支持侦探墙入口
- **THEN** 页面 SHALL 显示“在侦探墙打开完整生命线”操作并进入对应 topic lifeline

#### Scenario: 键盘关闭详情
- **WHEN** 日报详情打开且用户按 Esc
- **THEN** 详情 SHALL 关闭、背景滚动锁 SHALL 解除，并将焦点恢复到打开详情的日报卡片

#### Scenario: 空状态
- **WHEN** 选中 board 但该 board 无日报
- **THEN** 组件 SHALL 展示“暂无日报”

#### Scenario: 加载超过 30 天
- **WHEN** 当前 `days=28`，用户连续两次点击“加载更早”
- **THEN** 组件 SHALL 依次以 `days=35` 和 `days=42` 请求，并展示对应范围内的日报

#### Scenario: 切换 board 重置状态
- **WHEN** 用户从一个 board 切换到另一个 board
- **THEN** 组件 SHALL 将 `days` 重置为 7、加载新 board 日报，并清理旧 board 的 topic/thread 展开状态
