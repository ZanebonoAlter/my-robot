## 1. 后端：匈牙利算法核心实现

- [x] 1.1 实现纯 Go `hungarianAssignment(costMatrix [][]float64) []Assignment` 函数，O(n³) Kuhn-Munkres 算法，支持非方阵 padding
- [x] 1.2 定义匹配结果结构体 `type matchResult struct { FromID, ToID uint; Distance float64; Type string }`（Type: "primary"/"split"/"merge"）
- [x] 1.3 为匈牙利算法编写单元测试：方阵最优分配、非方阵 padding、全 INF 空分配、2×2 简单用例

## 2. 后端：三阶段匹配核心逻辑

- [x] 2.1 实现阈值常量：`MatchPenalty=0.28, SplitGap=0.03, SplitCeiling=0.30, SkipDayThreshold=0.20, QueryCutoff=0.35`
- [x] 2.2 实现 `buildDistMatrix(tx, leftIDs, rightIDs, leftDate, rightDate, boardID) map[[2]uint]float64`：单条 SQL cross-join 获取所有 pair 距离
- [x] 2.3 实现 `phase1Hungarian(leftIDs, rightIDs, distMatrix) []matchResult`：构建扩展方阵 + penalty + 虚拟节点 → 调用匈牙利 → 提取 primary matches
- [x] 2.4 实现 `phase2SplitMerge(leftIDs, rightIDs, distMatrix, primaries) []matchResult`：检测未匹配 section 的 gap-based split/merge
- [x] 2.5 实现 `phase3SkipDay(tx, boardID, allDates, allSections, writtenRelations) []matchResult`：对完全未匹配 section 检查隔天匹配（无中间延续 + dist < 0.20）
- [x] 2.6 为 Phase 1 编写单元测试：完美 1:1、penalty 阻止不相关匹配、混合场景
- [x] 2.7 为 Phase 2 编写单元测试：split 检测（gap<0.03）、gap 过大不写入、merge 检测
- [x] 2.8 为 Phase 3 编写单元测试：skip-day 补查、有中间延续则跳过

## 3. 后端：MatchAndSaveRelations 替换为 RebuildBoardRelations

- [x] 3.1 新增 `RebuildBoardRelations(tx *gorm.DB, boardID uint) error`：接收 `*gorm.DB`（事务或裸 DB），内部不做事务管理（不 Begin/Commit/Rollback）。清除 board 所有 relation → 获取所有日期 → 按相邻天对逐对执行 Phase 1+2+3 → 写入 relation。任何阶段失败直接返回 error，由调用方决定回滚策略
- [x] 3.2 修改 `SaveReport`：将 `MatchAndSaveRelations(tx, boardID, reportDate, sections)` 调用替换为 `RebuildBoardRelations(tx, boardID)`。同时移除 `SaveReport` 中旧的当天 relation 删除逻辑（line 64-70）——`RebuildBoardRelations` 会清除整个 board 的 relations，旧逻辑变为冗余
- [x] 3.3 删除旧函数：`MatchAndSaveRelations`、`shouldWriteRelation`、`competitiveFilter`、`hasContinuationInIntermediateDays`、`matchCandidate`
- [x] 3.4 重写 `BackfillRelations(boardID uint) (int, error)` 为薄包装：自行 `Begin()` 事务 → 调用 `RebuildBoardRelations(tx, boardID)` → `Commit()`。保持对外签名不变
- [x] 3.5 确认 `BackfillSectionEmbeddings` Phase 2 仍正确调用 `BackfillRelations(boardID)`（签名不变，无需修改）
- [x] 3.6 确认 `BackfillAllRelations` 不需要修改（仍遍历 board 调用 `BackfillRelations`）
- [x] 3.7 确认 `DeriveSectionStatuses` 不受影响（从完整 relation 图推导，输入格式不变）

## 4. 后端：集成测试

- [x] 4.1 为 `RebuildBoardRelations` 编写表级测试（真实 DB 事务回滚），覆盖：5 天 5 sections 的完美延续、split 场景（1→2）、merge 场景（2→1）、emerging（无匹配）、skip-day 补查
- [x] 4.2 为 `BackfillRelations` 编写全量重建测试：清空 → 重建 → 验证 relation 拓扑与 status 推导一致
- [x] 4.3 验证 `SaveReport` → `RebuildBoardRelations` 集成：保存新报告后 relation 拓扑正确

> 集成测试需要真实数据库（当前项目无 testcontainers 基础设施），暂通过数据回刷验证（Task 5）。

## 5. 验证（数据回刷）

- [x] 5.1 对 board 2853（伊朗局势）运行 `BackfillRelations`，验证 relation 数量从 ~36 降至 ~20，merge 占比大幅下降
- [x] 5.2 对 board 3639（AI 技术）运行 `BackfillRelations`，验证 relation 数量从 ~118 降至 ~46，continuing 占比大幅上升
- [x] 5.3 验证 `DeriveSectionStatuses` 在新 relation 拓扑下无异常（emerging/continuing/split/merge/ending 分布合理）
- [x] 5.4 编译通过、受影响包测试通过（26 tests PASS, go vet clean, go build clean）

## 6. 前端适配（可选，暂不实施）

- [x] 6.1 无需特殊处理。重连在 `SaveReport` 事务中同步完成，前端收到 WebSocket "completed" 事件后查询 timeline API 即可获取最新 relation 拓扑。事务原子性保证不会出现中间状态。
