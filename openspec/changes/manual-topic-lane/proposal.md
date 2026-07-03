## Why

持久化话题（persistent-topic）上线后，话题系统对用户是"只读观察台"——能看，但不能编排。三个缺口：

1. **只能被动等话题涌现**：persistent_topic 全自动（聚类 + embedding AND-gate），用户无法主动声明"我觉得这几条 section 是同一个事"。误聚类（如把"美伊博弈"吸进"中东局势"）无法人工纠正，只能等下一期算法自己跑偏。
2. **话题管理是弹窗、只能维护已有话题**：现有 `TopicManageDialog`（回刷 / 重命名 / 归档 / 合并）全部针对已存在的 topic，**缺手动新建**；且弹窗入口隐蔽，缺乏"全局编排"的定位感。
3. **话题总览没占满、缺编排主权**：`BoardThreadBrowser` 的 lanes 视图悬浮在 content 区，留白明显，且 section→topic 归属用户完全无法干预。

三者共同指向：用户在话题系统里只是旁观者。本 change 把话题总览升级成占满式的「**全局话题编排工作台**」，让用户能手动新建泳道、串联 section、调整归属，把"编排主权"交给用户。

## What Changes

两块，可在同一 change 内切片交付：

### A. 话题总览工作台化（弃弹窗）—— section-lifecycle

把话题总览从"日报 tab 里的一个切换视图"升级为占满 content 的全局工作台：

- **弃用 `TopicManageDialog` 弹窗**：其能力（回刷归属 / 重命名 / 归档 / 合并）全部并入工作台工具条 + 泳道 hover 操作菜单，不再有独立弹窗。
- **lanes 视图占满 content**：修正"总览悬浮留白"缺陷，左侧泳道标签列 + 右侧时间网格纵向撑满；同天多节点维持纵向堆叠（现有 lanes 已有此行为，本变更不改算法，只补满布局）。
- **工具条取代切换按钮**：时间范围选择器（默认 14 天，可 7 / 30 / 全部）+ 视图模式（时间线 / 泳道）+ 回刷归属 + 合并预览 + **新建泳道**。
- **泳道就地操作**：hover 任意泳道出现重命名 / 归档 / 删除（取代原弹窗逐个操作）。

### B. 手动建泳道 + section 串联 —— persistent-topic

新增"编排态"：用户在工作台点"新建泳道"进入编排态，从全局 section 池（默认最近 14 天）勾选 N 条 section + 填泳道名 → 保存为新 `persistent_topic`。

- **新泳道直接 `active` + `source=manual`**：绕过 `upgrade_threshold` 连续命中门禁（用户主权声明，已有内容支撑），区别于自动涌现的 candidate。
- **embedding 来自选中 section 聚合**：取选中 section embedding 的平均向量（mean pooling，复用现有 backfill 的 average_link 思路），让新泳道立刻参与后续日报的自动归属（AND-gate）。
- **归属覆盖（单值）**：选中 section 的 `persistent_topic_id` 被改写到新泳道，`topic_match_confidence=manual`（新增第四态，标记"人工归属，非算法"）。UI 明确提示"N 条将从原话题移出"。
- **编排态预览泳道 + 体检报告**：实时反映选中——预览泳道复用 lanes 节点三态（实心=贴合 / 虚线=边界 / 黄框=离群建议剔除）+ 同天纵向堆叠；右侧体检报告给"聚类质量 / 撞车检查 / 离群提示"作为人工评判依据。
- **离群只标黄、不自动删**：检测到选中 section 与其余明显偏离时标黄提示，由用户决定是否剔除。

## Capabilities

### Modified Capabilities

- `persistent-topic`：手动建泳道（`source` 字段区分 auto/manual、直接 active 绕过门禁）、section 归属覆盖（`topic_match_confidence=manual` 第四态、embedding 聚合锚点）、手动泳道参与自动归属
- `section-lifecycle`：话题总览工作台化（弃 `TopicManageDialog` 弹窗、lanes 占满 content、工具条 + 泳道 hover 操作、时间范围选择器）、编排态预览泳道 + 体检报告

## Impact

- **后端**
  - 新增列：`board_persistent_topics.source`（auto/manual，默认 auto）；`topic_match_confidence` 枚举扩充 `manual`。
  - 新增端点：手动创建 topic（`POST /api/semantic-boards/:boardId/persistent-topics/manual`，body: label + section_ids[]）、批量改写 section 归属（事务内聚合 embedding + 建 topic + 改写各 section 的 persistent_topic_id/confidence）。
  - 复用：`CreateTopic`（已有）、`UpdateSectionTopicAssignment`（已有，扩展支持 confidence=manual）、embedding 聚合（参考 `daily_report_backfill_topics.go` 的 average_link）。
  - 既有端点改造：话题重命名 / 归档 / 删除 / 合并 从弹窗触发改为工作台触发（handler 复用，路由不变）。
- **前端**
  - `BoardThreadBrowser.vue`：lanes 占满布局 + 工具条 + 泳道 hover 操作菜单；编排态（`viewMode='compose'`）预览泳道时间轴 + 候选池 + 体检报告三栏。
  - 弃用 `TopicManageDialog.vue`（能力迁移完成后删除）。
  - 新组件：编排态体检报告（聚类质量 / 撞车检查 / 离群提示，纯函数可测）。
  - API 封装：`app/api/` 新增手动建泳道 + 批量改写归属方法（经 `ApiClient`，snake→camel normalizer）。
- **AI 成本**：手动建泳道零额外 AI 调用——embedding 聚合是纯向量运算（已有 section 向量取平均），归属改写是纯 DB 操作。
- **数据兼容**：`source` 列新增默认 auto，历史 topic 不受影响；`topic_match_confidence=manual` 是新增枚举值，老数据不含此值。
