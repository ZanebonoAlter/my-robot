## Purpose

**DEPRECATED** — Thread 级别血统追踪已移除。Lineage 追踪上移到 Section 级关系表（参见 section-relations）。
- `prev_thread_id` 赋值已移除
- 线程血统链查询 API（GET /api/daily-reports/threads/:id/lineage）已移除
- 板块线程时间线 API（GET /api/semantic-boards/:id/thread-timeline）已移除
- ThreadLineagePanel 组件已移除
- BoardThreadBrowser 已改造为叙事级 DAG 时间线（参见 section-lifecycle）
