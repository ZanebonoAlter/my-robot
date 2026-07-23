## Why

持久化话题（persistent-topic）上线后，用户反馈的问题集中在"看不见、插不上手、看不清"三类：

1. **归属黑盒**：某条新闻为什么被分到某话题——相似度多少、AI 判断是否一致——这些依据从不暴露，导致误分类（如"以黎冲突"吸进"美伊战事"）无法被察觉，只能偶然撞见。后端其实已存 `topic_match_distance` / `topic_match_confidence`，但 LLM 的 `matched_topic_id` 是瞬态字段（`gorm:"-"`），用完即弃，前端从未消费任何理由数据。
2. **只能被动等话题涌现**：用户无法主动标记一个自己关心的议题持续追踪（如"美伊会不会真打起来"），话题只能自下而上从聚类冒出。
3. **话题画布密度过高**：画布默认 `COL_W=148px/天`、节点直径 14px、字号 10px，用户被迫放大到 150% 才能阅读；桌面画布完全不响应窗口宽度。

三者共同指向同一缺口：**用户在话题系统中只是旁观者，无法看（理由）、无法盯（主动追踪）、看不清（画布）**。本 change 把"可观测 + 用户主权 + 可读性"作为基座先立起来——让用户能看、能盯、能看清，再决定后续算法调优。

## What Changes

三块，独立可切片：

### A. 关注标记（Topic Watch）—— 用户主权

新增"关注标记"实体：用户写一句话描述关切（如"美伊会不会真打起来"），系统在每期日报生成时让 AI 判定当天各 section 是否命中该关注，命中在日报顶部独立栏位展示。

- **它是"过滤器"，不是"筐"**：与 `persistent_topic`（从聚类涌现、embedding 驱动、有生命周期门禁）刻意隔离——不共享归属逻辑、不参与 embedding AND-gate、不累积历史 section、无升级/归档门禁。命中只读、随时开关。
- **判定走 AI 单信号**（不走 embedding 双重确认）：关注是意图声明，不是聚类产物；AI 说命中即命中。
- **不侵入日报正文**：命中结果在日报顶部独立栏位展示标题+一句话，正文保持沉浸阅读。

### B. 归属理由可视化（Assignment Reasoning）—— 可观测

暴露每个 section 到其 `persistent_topic` 的归属依据，分三层：

1. **数据层**：持久化 `MatchedTopicID`（现为瞬态 `gorm:"-"`），让"AI 选了哪个话题"不再用完即弃。
2. **画布层**：话题泳道（`BoardThreadBrowser`）节点按置信度分样式——`anchor_hit` 实心 / 边界命中半实心 / `auto_new` 空心——一眼扫出可疑归属；hover 冒气泡显示人话理由（"与『以黎冲突』73% 相关，AI 也认同"）+ 原始数值。
3. **详情层**：点开话题在泳道侧栏展示该话题全部历史 section + 各自信度（复用 `getTopicLifeline`）。

所有理由信息只出现在"话题总览 → 话题泳道"探究区，**不侵入日报正文**（保持沉浸）。

### C. 画布密度（Canvas Density）—— 可读性

话题画布（`BoardThreadBrowser`）默认参数过密导致"要放大 150% 才能看清"：

- 放宽 `COL_W`（148 → 约 200）、上调节点标签与正文文字字号、默认缩放提到约 1.2-1.3。
- 接受横向滚动增加（换取无需放大即可阅读），为 B 的节点样式分层腾出空间（否则 14px 节点无法承载三种样式）。
- 桌面画布当前不响应窗口宽度（`viewportWidth` 仅用于判断移动端），本 change 暂不动响应式，只调默认密度。

## Capabilities

### New Capabilities

- `topic-watch`: 关注标记实体 + 日报生成时 AI 命中判定 + 日报顶部独立栏位展示（只读过滤器，与 persistent_topic 隔离）
- `topic-assignment-reasoning`: section 归属理由的持久化（`matched_topic_id` 落库）+ 画布节点样式分层 + hover 人话气泡 + 话题详情信度列表

### Modified Capabilities

- `section-lifecycle`: 新增话题总览画布（`BoardThreadBrowser`，2D SVG）默认可读性 requirement——放宽列宽/字号/默认缩放，改善"需放大到 150% 才能阅读"。注：`topic-graph-display-controls` 描述的是 3D TopicGraphCanvas，与本变更无关

## Impact

- **后端**
  - 新增：关注标记模型 + 命中记录（`board_topic_watches` / `topic_watch_hits`）、AI 命中判定服务（接入日报生成流程，批量一次问所有 section）、关注标记 CRUD + 命中查询 API。
  - 修改：`daily_report_sections` 持久化 `matched_topic_id` 列（版本化迁移，由瞬态转持久）；现有 section/timeline/lifeline 接口暴露理由字段。
- **前端**
  - `BoardThreadBrowser.vue`：画布密度参数 + 节点样式分层 + hover 气泡（话题泳道）。
  - 日报页：顶部关注标记栏（新组件）。
  - 话题泳道侧栏：话题历史 section + 信度展示。
- **AI 成本**：关注标记每期日报多一轮 AI 判定（单次批量请求涵盖所有 section，成本可控）；理由可视化无额外 AI 调用（用已存数据）。
