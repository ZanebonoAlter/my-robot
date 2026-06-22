## Context

`BoardDailyReportTimeline.vue` 当前同时承担日报列表、详情缓存、弹窗、日期导航、section 展开、文章浮窗和生命周期入口，单文件已超过 1100 行。详情容器使用 `width: min(1100px, 92vw)`，样式中存在大量硬编码 `rgba(...)`，在宽屏和 dark 主题下都暴露出明显问题。

`persistent-topic` 已提供本设计需要的数据契约：日报 section 的轻量 `persistent_topic`、`getTopicLifeline(topicId)`、identity relation、thread 的 `related_article_ids`。本 change 只消费这些能力，不改变数据库和后端 API。

视觉基线来自 `design-demos/daily-report-magazine.html`，但静态 demo 的假数据、直接 DOM 操作和布局脚本不能直接搬入 Vue；生产实现以当前组件状态、API 返回值和主题系统为准。

## Goals / Non-Goals

**Goals:**

- 把日报详情变成沉浸式、全屏、长滚动的杂志化阅读界面。
- 保持日报列表、日期导航、section 生命周期、话题总览和文章预览能力不回退。
- 在日报正文中直接呈现持久话题的跨日生命线及当日 threads。
- editorial/dark 双主题、宽中窄三档布局和键盘关闭行为可验证。
- 控制异步请求和组件复杂度，避免 topic/article 展开产生重复请求。

**Non-Goals:**

- 不新增独立路由、分享 URL 或服务端渲染详情页。
- 不修改日报生成、PersistentTopic 归属、lifeline 关系生成或文章 API。
- 不在本 change 中重做 BoardThreadBrowser、SectionLifecyclePanel 或 TopicDetectiveWall。
- 不引入新的字体、动画、CSS 框架或状态管理库。

## Decisions

### 1. 保留 overlay 状态模型，改为全屏阅读层

继续由 `BoardDailyReportTimeline` 管理选中日报和关闭行为，详情使用 `position: fixed; inset: 0` 占满 viewport；不新建路由。这样可以复用现有列表、缓存、Esc 和 `openArticle` 事件链，也避免为单用户局部阅读状态增加路由同步成本。

备选的独立详情路由更利于分享和浏览器历史，但当前产品没有分享/深链需求，且会扩大 TagsPage 路由与状态恢复范围，因此不采用。

### 2. 容器负责编排，视觉与局部交互按职责拆分

`BoardDailyReportTimeline` 保留列表加载、详情缓存、当前日期和顶层 overlay 生命周期。生产实现从中提取以下有独立输入/输出边界的组件：

- masthead/头条/highlights 展示；
- sticky 目录与历史日期边栏；
- topic section/card；
- mini lifeline（节点、identity edge、当日 thread 展开）。

文章标题缓存和 lifeline 缓存放在容器或单一 composable 中，子组件只通过 props/emits 读写。不会为了每个静态排版片段都创建组件。

备选的单文件重写能减少文件数，但会把新增的两类异步缓存和多层展开状态继续堆进现有 1100 行组件，维护风险过高。

### 3. 数据映射只使用现有真实字段

- masthead 使用 `selectedDetail` 的标题、日期、article/cluster 统计。
- 头条优先使用 `highlights[0]`；没有 highlight 时回退到 `avg_score` 最高、`best_tier` 最优的 section，不生成额外文案。
- highlights 按 API 顺序展示，最多三项；为空时整块隐藏。
- 正文继续使用 `qualityZones`，按 active / candidate / unassigned 分区，不回退到 tier 分区。
- 历史日期来自已加载的 `reports`，点击复用现有详情缓存。

刊头保留 editorial 气质，但不硬编码“号外 / Extra Edition”；eyebrow 使用中性的产品文案或真实报告状态。

### 4. lifeline 与文章请求采用按 ID 缓存

首次展开 active topic 时调用 `getTopicLifeline(topicId)`，结果按 topic id 缓存 loading/success/error；收起再展开不重复请求，显式重试才重新加载。展示窗口取当前日报日期向前七个自然日，API 仍保留完整返回。

thread 展开时只请求 `related_article_ids` 中尚未缓存的文章。相同 article id 在不同 thread 出现时复用标题缓存；失败项显示可重试状态，不阻塞其他文章。

不新增后端批量接口：当前每次仅展开一个局部 thread，前端去重缓存足以控制请求量；若真实数据证明单次 IDs 过多，再独立评估批量 API。

### 5. identity relation 是唯一连线事实来源

mini lifeline 只渲染 `relation_type === 'identity'` 且两个端点都在当前七日窗口中的关系。路径使用现有规则：

`M x1,y1 C midX,y1 midX,y2 x2,y2`

节点按日期列定位，同日多 section 合并为一个日期节点并显示数量角标。空白日不生成假节点；若真实 identity relation 跨过空白日，曲线直接连接真实端点。resize、topic 展开和节点详情高度变化后在 `nextTick` 重算路径。

不根据相邻日期臆造连线，也不在 mini 视图混入 similarity edge。

### 6. 主题和响应式全部服从现有系统

组件只使用现有语义 token；topic 自带的稳定 `persistent_topic.color` 仅作话题身份强调色。不得新增针对固定浅色背景的 `rgba(0,0,0,*)`。

- `> 1100px`：正文 + sticky 边栏双栏，头条/highlights 可通栏。
- `721px–1100px`：单栏正文，边栏折叠为顶部目录/日期控制。
- `<= 720px`：单栏、可换行标题、触控友好的横向 lifeline；不得因桌面端 `nowrap` 产生页面横向溢出。

宽屏 masthead 可使用 `clamp()` 和 `white-space: nowrap`；窄屏显式恢复换行。

### 7. 保留现有出口与可访问交互

顶层详情层保留明确关闭按钮、Esc 关闭、背景滚动锁和关闭后的焦点恢复。话题总览入口继续打开现有 BoardThreadBrowser；active topic 的 mini lifeline 另提供“在侦探墙打开完整生命线”入口，并复用现有 lifeline/侦探墙事件链。入口在无 topic id 或设备能力不足时隐藏。

所有可点击 header、节点和 thread 使用原生 button 或具备等价键盘/ARIA 语义，loading/error/empty 状态不能只靠颜色表达。

## Risks / Trade-offs

- **[组件拆分改变现有事件链]** → 先为关闭、日期切换、section 生命周期和 `openArticle` 写行为测试，再迁移模板。
- **[lifeline/article 展开触发 N+1]** → 以 topic id 和 article id 去重缓存，并只在用户展开后加载。
- **[SVG 坐标受字体、展开内容和 resize 影响]** → 将测量集中在 mini lifeline，`nextTick` 后计算，并用 `ResizeObserver`/resize 清理机制重绘。
- **[大标题 nowrap 在窄屏溢出]** → 只在宽屏启用 nowrap，720px 以下允许换行并限制字号。
- **[静态 demo 与真实数据结构不一致]** → 所有验收使用真实 API 或 fixture，demo 只用于视觉对照。
- **[旧 5000 端口后端未重启导致分类仍错误]** → 实施前先确认详情 API 返回 `persistent_topic`；缺失时阻止进入视觉验收。

## Migration Plan

1. 先补组件行为测试和真实 fixture，冻结现有关闭、日期、文章与 lifecycle 事件。
2. 抽取主题安全的全屏 shell 与视觉组件，保持旧数据流可用。
3. 接入 masthead、边栏和 topic 分区，再接入 lifeline 与文章内联展开。
4. 完成双主题、三档 viewport 和键盘流程浏览器验证。
5. 更新 reference 文档并执行 tasks.md 验证节全部命令。

本 change 没有数据库迁移。回滚时可恢复旧详情模板和样式，API 与存储不受影响。

## Open Questions

当前没有阻塞实施的问题。masthead 的最终微文案和视觉间距允许在浏览器验收中调整，但不得改变“不硬编码号外业务状态”和上述数据/交互契约。
