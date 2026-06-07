## Why

Section（叙事）之间的跨天关系目前用 `prev_section_id` 单链表达，只能表示一对一延续。实际数据中经常出现一对多（叙事分化）和多对一（叙事合并），且 embedding 匹配容易误连不相关叙事。同时 Thread（线索）层面的 status/lineage 追踪意义不大——真正的追踪单位应该是叙事（Section）。

## What Changes

- **BREAKING** 移除 Thread 的 `status`、`prev_thread_id` 字段，Thread 简化为叙事下的具体事件条目，不再跨天追踪
- **BREAKING** 移除 Section 的 `prev_section_id` 单链字段
- 新增 `daily_report_section_relations` 关系表，存储 Section 之间的多对多跨天关系（from → to + embedding distance）
- Section status 改为动态推导：根据关系表拓扑判断 emerging / split / merge / continuing，并独立推导 ended 标记（无后继且非最新天）
- 聚类 prompt 已调整为叙事级标题（跨事件的解释性判断），本次变更适配关系表
- 前端 Gantt 图改造：以叙事为行，节点间连线支持分叉和合并

## Capabilities

### New Capabilities
- `section-relations`: Section 间多对多关系表、关系写入逻辑、关系拓扑查询
- `section-merge`: 同日 Section 两阶段合并（embedding 确定性合并 + LLM 仲裁灰色地带），消除聚类过碎问题

### Modified Capabilities
- `daily-report-system`: 移除 thread status/prev_thread_id，简化 thread 生成；section embedding 匹配改为写关系表
- `section-lifecycle`: status 推导改为基于关系表拓扑；lifecycle/timeline API 适配关系表；前端 Gantt 图支持 split/merge
- `thread-storage`: DailyReportThread 移除 status、prev_thread_id 字段及相关索引
- `thread-lineage`: Thread lineage API 和血统链功能移除（lineage 追踪上移到 section 级）

## Impact

- **数据库**：新增 `daily_report_section_relations` 表；`daily_report_threads` 删除 `status`、`prev_thread_id` 列和索引；`daily_report_sections` 删除 `prev_section_id` 列
- **后端**：`generator.go` 简化 thread 生成（移除 status/prev_thread_id）；`repository.go` 改写 section 匹配逻辑、新增关系表 CRUD、SaveReport upsert 清理旧 relation、BackfillSectionEmbeddings 改写为写 relation 表；handler 移除 thread lineage/timeline API、改造 section timeline/lifecycle API、报告详情 API 移除 section status/prev_section_id 字段
- **前端**：`BoardThreadBrowser.vue` 改为叙事 Gantt 图支持 split/merge；`ThreadLineagePanel.vue` 移除或替换为 section 级面板；`BoardDailyReportTimeline.vue` 移除 section status 徽章和 thread sitemap 入口；`SectionLifecyclePanel.vue` 适配 DAG 响应；API 类型定义更新（含 DailyReportSection 移除 status/prev_section_id）
