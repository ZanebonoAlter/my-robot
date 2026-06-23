## Why

当前的 `MatchAndSaveRelations` 采用增量贪心匹配：每次新报告保存时，对每个新 section 查询全历史 embedding 匹配，依赖已有 relation 做"中间天延续"检查。这导致两个结构性问题：

1. **累积误差**：每次匹配依赖已有 relation 的正确性，一旦某步判断有误（如错误地将两个不相关 section 连在一起），后续所有依赖它的判断都会偏，越传越远。
2. **merge 泛滥**：在稠密版块（如 AI 技术，71 sections / 6 天）中，embedding 距离普遍在 0.15-0.35 之间密集分布，competitive filter 无法有效区分真伪，导致 31/71 个 section 被误标为 merge，仅 2 个为 continuing。

实测数据验证（二分图模拟 vs 当前增量）：
- Board 2853（伊朗局势）：36→20 条 relation，merge 9→1，continuing 6→13
- Board 3639（AI 技术）：118→46 条 relation，merge 31→4，continuing 2→31，d>0.25 噪声 50→19

## What Changes

- **替换** `MatchAndSaveRelations` 的增量匹配算法为**相邻天二分图最优匹配**（匈牙利算法 + penalty 机制 + Phase 2 split/merge 检测 + Phase 3 skip-day 补查）
- **重写** `BackfillRelations` 为按相邻天对执行二分图匹配的全量重建
- **删除** `shouldWriteRelation`、`competitiveFilter`、`hasContinuationInIntermediateDays`（被新算法取代）
- **修改** `SaveReport` 的 relation 写入流程：新报告保存后触发该 board 的全量重连（在 SaveReport 事务中同步完成）
- **保留** `DeriveSectionStatuses`（从完整 relation 图推导 status，逻辑不变）

## Capabilities

### New Capabilities

- `bipartite-relation-matching`: 相邻天二分图最优匹配算法——Phase 1 匈牙利 1:1（带 penalty 允许不匹配）、Phase 2 gap-based split/merge 检测、Phase 3 skip-day 补查（阈值 0.20）

### Modified Capabilities

- `section-relations`: 关系写入逻辑从增量贪心匹配改为相邻天二分图匹配；删除 `shouldWriteRelation` / `competitiveFilter` / `hasContinuationInIntermediateDays`；`SaveReport` 中 `MatchAndSaveRelations` 替换为同步 board 级全量重连（`RebuildBoardRelations(tx, boardID)`）；`BackfillRelations` 重写为薄包装调用 `RebuildBoardRelations`

## Impact

- **后端**：`repository.go` 的 `MatchAndSaveRelations` 完全重写为 `RebuildBoardRelations`（二分图匹配）；`BackfillRelations` 重写；删除 `shouldWriteRelation`、`competitiveFilter`、`hasContinuationInIntermediateDays`、`matchCandidate`；新增 `hungarianAssignment`、`detectSplitMerge`、`skipDayReconnect`
- **后端**：`SaveReport` 中 relation 写入改为在 SaveReport 事务内调用 `RebuildBoardRelations(tx, boardID)`（同步），同时移除旧的当天 relation 删除逻辑
- **数据库**：无需 schema 变更，`daily_report_section_relations` 表结构不变
- **前端**：无需特殊处理。重连在事务中同步完成，前端通过 WebSocket "completed" 事件后查询 timeline API 自然获取最新 relation
- **API**：`POST /api/daily-reports/backfill-relations` 端点保留，内部调用新的 `BackfillAllRelations`
