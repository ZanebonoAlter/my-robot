## MODIFIED Requirements

### Requirement: daily_report_threads table（简化）
`daily_report_threads` 表 SHALL 移除 `status` 和 `prev_thread_id` 字段。Thread 简化为叙事下的具体事件条目，不再跨天追踪。

保留字段：id, report_id, section_id, title, summary, tag_ids(JSONB), confidence, related_article_ids(JSONB), created_at。

移除字段：status, prev_thread_id。

移除索引：
- `idx_daily_report_threads_prev_thread_id`

#### Scenario: 表结构变更
- **WHEN** 数据库迁移运行
- **THEN** `daily_report_threads` 表 SHALL 删除 `status` 列、`prev_thread_id` 列及相关索引

### Requirement: DailyReportThread GORM 模型（简化）
`DailyReportThread` 结构体 SHALL 移除 `Status` 和 `PrevThreadID` 字段。JSON 响应中 thread 对象 SHALL 包含 id、report_id、section_id、title、summary、tag_ids、confidence、related_article_ids，不包含 status 和 prev_thread_id。

#### Scenario: API 响应不含 status
- **WHEN** 查询日报详情 API 返回 thread 列表
- **THEN** 每条 thread SHALL 包含 id、title、summary、tag_ids、confidence，不包含 status 和 prev_thread_id

### Requirement: Thread 存储接口（简化）
系统 SHALL 简化 thread 存储接口：
- `SaveThreads(sectionID uint, reportID uint, threads []DailyReportThread) error`：批量保存线程（不设置 status 和 prev_thread_id）
- `GetThreadsBySection(sectionID uint) ([]DailyReportThread, error)`
- `GetThreadsByReport(reportID uint) ([]DailyReportThread, error)`
- `DeleteThreadsByReport(tx *gorm.DB, reportID uint) error`：接受 tx 参数以支持在 SaveReport 事务内调用
- `DeleteRelationsBySectionIDs(tx *gorm.DB, sectionIDs []uint) error`：删除涉及指定 section 的所有 relation 记录（from_section_id IN OR to_section_id IN）

#### Scenario: 批量保存线程不含 status
- **WHEN** 调用 SaveThreads(sectionID=10, reportID=5, threads=[...3 threads...])
- **THEN** 系统 SHALL 创建 3 条记录，每条只包含 title、summary、tag_ids、confidence、related_article_ids，不设置 status 和 prev_thread_id

#### Scenario: 重生成报告时清理旧 relation 记录
- **WHEN** SaveReport 检测到同一 board+date 已有旧报告，旧 section IDs = [10, 11, 12]
- **THEN** 系统 SHALL 在事务内删除所有 from_section_id IN (10,11,12) OR to_section_id IN (10,11,12) 的 relation 记录
- **THEN** 系统 SHALL 然后删除旧 threads 和旧 sections
- **THEN** 系统 SHALL 写入新 sections 和新 relations
