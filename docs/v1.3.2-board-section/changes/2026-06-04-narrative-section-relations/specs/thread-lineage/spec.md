## REMOVED Requirements

### Requirement: prev_thread_id 赋值
**Reason**: Thread 不再跨天追踪，lineage 追踪上移到 Section 级关系表
**Migration**: 使用 `daily_report_section_relations` 表追踪叙事间关系

### Requirement: 线程血统链查询 API
**Reason**: Thread lineage 不再需要，`GET /api/daily-reports/threads/:id/lineage` 端点移除
**Migration**: 使用 `GET /api/daily-reports/sections/:id/lifecycle` 查询叙事级生命周期

### Requirement: 板块线程时间线 API
**Reason**: Thread timeline 不再需要，`GET /api/semantic-boards/:id/thread-timeline` 端点移除
**Migration**: 使用 `GET /api/semantic-boards/:id/section-timeline` 查询叙事级时间线

### Requirement: Thread detail panel 组件 (View A)
**Reason**: ThreadLineagePanel 组件移除，改为 SectionLifecyclePanel
**Migration**: 点击 section header 打开 SectionLifecyclePanel 查看叙事级生命周期

### Requirement: Board thread browser 组件 (View B)
**Reason**: BoardThreadBrowser 已改造为叙事级 Gantt 图，不再按 thread 分组
**Migration**: BoardThreadBrowser 改为按 section/relation 分组，支持 split/merge 可视化
