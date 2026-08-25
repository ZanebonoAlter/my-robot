## REMOVED Requirements

### Requirement: 板块叙事时间线 API
**Reason**: `GET /api/semantic-boards/:id/narratives` 为死路由——前端零调用（NarrativeGenerateDialog 实际调用 useDailyReportsApi().generateDailyReport），handler 仅剩对死表的只读查询。
**Migration**: 时间线叙事消费需求由日报页（board_daily_reports + daily_report_threads）承接；本 change 删除该路由，无兼容层。

### Requirement: 叙事卡片组件 BoardNarrativeTimeline
**Reason**: 前端组件已不存在于代码库。
**Migration**: 无。

### Requirement: 叙事卡片加载更多
**Reason**: 同上。
**Migration**: 无。

### Requirement: 取消 scope 分类
**Reason**: 对已删除功能的否定性约束，失去意义。
**Migration**: 无。
