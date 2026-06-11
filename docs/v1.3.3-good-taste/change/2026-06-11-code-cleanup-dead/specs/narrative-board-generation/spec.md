# Delta Spec: narrative-board-generation (code-cleanup-dead)

## REMOVED Requirements

### Requirement: 全局叙事生成方法 GenerateAndSaveGlobal

移除 `narrative/service.go` 中 `GenerateAndSaveGlobal` 方法。此方法生成全局级别叙事摘要，已被 `daily-report-system` 替代，无调用者。

#### Scenario: GenerateAndSaveGlobal 不存在
- **WHEN** 检查 `narrative/service.go` 中的导出方法
- **THEN** 不存在 `GenerateAndSaveGlobal` 方法

#### Scenario: GenerateAndSaveGlobal 相关测试已删除
- **WHEN** 运行 `go test ./internal/domain/narrative/...`
- **THEN** 不存在以 `TestGenerateAndSaveGlobal` 或 `GenerateAndSaveGlobal` 命名的测试函数

### Requirement: 全分类叙事生成方法 GenerateAndSaveForAllCategories

移除 `narrative/service.go` 中 `GenerateAndSaveForAllCategories` 方法。此方法按 feed category 生成叙事摘要，已被 board 级别叙事替代，无调用者。

#### Scenario: GenerateAndSaveForAllCategories 不存在
- **WHEN** 检查 `narrative/service.go` 中的导出方法
- **THEN** 不存在 `GenerateAndSaveForAllCategories` 方法

### Requirement: Watched tag 叙事生成函数

移除 `narrative/watched_narrative.go` 整个文件。`GenerateWatchedTagNarratives`、`generateSingleWatchedNarrative` 和 `WatchedTagNarrativeOutput` 均无调用者。

#### Scenario: watched_narrative.go 文件不存在
- **WHEN** 检查 `internal/domain/narrative/` 目录
- **THEN** 不存在 `watched_narrative.go` 文件

### Requirement: Board 叙事批量保存函数 SaveNarrativesForBoard

移除 `narrative/board_narrative_generator.go` 中 `SaveNarrativesForBoard` 函数。此函数为 board 批量保存叙事，无调用者。

#### Scenario: SaveNarrativesForBoard 不存在
- **WHEN** 检查 `narrative/board_narrative_generator.go` 中的导出函数
- **THEN** 不存在 `SaveNarrativesForBoard` 函数

### Requirement: Board 事件标签加载函数 LoadBoardEventTags

移除 `narrative/board_narrative_generator.go` 中 `LoadBoardEventTags` 函数。此函数加载 board 事件标签，无调用者。

#### Scenario: LoadBoardEventTags 不存在
- **WHEN** 检查 `narrative/board_narrative_generator.go` 中的导出函数
- **THEN** 不存在 `LoadBoardEventTags` 函数

### Requirement: Board 叙事收集函数 (board_collector.go)

移除 `narrative/board_collector.go` 整个文件。`CollectPreviousDayBoards` 和 `CollectPreviousBoardNarratives` 均无调用者。关联类型 `PreviousBoardBrief` 和 `BoardNarrativeBrief` 仅被这些函数使用。

#### Scenario: board_collector.go 文件不存在
- **WHEN** 检查 `internal/domain/narrative/` 目录
- **THEN** 不存在 `board_collector.go` 文件

### Requirement: 分类叙事摘要收集函数 CollectCategoryNarrativeSummaries

移除 `narrative/collector.go` 中 `CollectCategoryNarrativeSummaries` 函数。分类级别叙事已被 `daily-report-system` 替代，无调用者。

#### Scenario: CollectCategoryNarrativeSummaries 不存在
- **WHEN** 检查 `narrative/collector.go` 中的导出函数
- **THEN** 不存在 `CollectCategoryNarrativeSummaries` 函数

### Requirement: 叙事生成函数 GenerateNarratives

移除 `narrative/generator.go` 中 `GenerateNarratives` 函数。此函数零生产调用者（仅有 1 个测试调用者）。

#### Scenario: GenerateNarratives 不存在
- **WHEN** 检查 `narrative/generator.go` 中的导出函数
- **THEN** 不存在 `GenerateNarratives` 函数

### Requirement: NarrativeSummaryScheduler 定时任务

移除 `jobs/narrative_summary.go` 整个文件及其在 `runtime.go`、`handler.go`、`runtimeinfo/schedulers.go` 中的注册。此定时任务每日生成全局叙事摘要，已被 `daily-report-system` 替代。

#### Scenario: narrative_summary.go 文件不存在
- **WHEN** 检查 `internal/jobs/` 目录
- **THEN** 不存在 `narrative_summary.go` 文件

#### Scenario: runtime.go 不启动 NarrativeSummaryScheduler
- **WHEN** 检查 `runtime.go` 中的 scheduler 启动代码
- **THEN** 不存在 `NarrativeSummaryScheduler` 的实例化和启动调用

#### Scenario: handler.go 不包含 narrative_summary 描述符
- **WHEN** 检查 `handler.go` 中的 schedulerDescriptors 列表
- **THEN** 不包含 `narrative_summary` 相关条目

#### Scenario: runtimeinfo/schedulers.go 不包含 NarrativeSummarySchedulerInterface
- **WHEN** 检查 `runtimeinfo/schedulers.go`
- **THEN** 不存在 `NarrativeSummarySchedulerInterface` 变量

### Requirement: 废弃叙事测试用例

移除 `narrative/service_test.go` 中约 13 个测试已删除方法的测试用例。

#### Scenario: 废弃测试用例已删除
- **WHEN** 运行 `go test ./internal/domain/narrative/...`
- **THEN** 测试编译通过且所有测试通过，不包含已删除方法的测试
