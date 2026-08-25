# log-cleanup Delta

## ADDED Requirements

### Requirement: embedding_queues completed 保留策略

既有 scheduled log cleanup 作业 SHALL 同时清理 `embedding_queues` 中 status='completed' 且创建时间早于保留期（默认 30 天）的行，防止 completed 历史行无限累积。清理查询 MUST 有 created_at（或等价时间列）索引支撑。

#### Scenario: 定时清理移除过期 completed 行
- **WHEN** scheduled cleanup 运行且存在 status='completed'、创建时间 > 30 天的行
- **THEN** 这些行被删除，30 天内的 completed 行保留

#### Scenario: 无过期行时正常空跑
- **WHEN** scheduled cleanup 运行且无过期行
- **THEN** 作业正常完成，不报错
