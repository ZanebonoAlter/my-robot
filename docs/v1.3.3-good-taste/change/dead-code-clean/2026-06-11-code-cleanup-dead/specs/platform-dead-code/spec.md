# Delta Spec: platform-dead-code (code-cleanup-dead)

## REMOVED Requirements

### Requirement: datamigrate 包整体删除

移除 `platform/database/datamigrate/` 整个目录。此包为 SQLite→PostgreSQL 一次性迁移工具，包含 8 个导出函数（`NewSQLiteReader`, `ReadRows`, `DefaultTableSpecs`, `WithDB`, `TruncateTables`, `ImportTable`, `ResetSequences`, `Verify`），全代码库零导入。

#### Scenario: datamigrate 目录不存在
- **WHEN** 检查 `internal/platform/database/` 目录
- **THEN** 不存在 `datamigrate/` 子目录

#### Scenario: 无 datamigrate 引用残留
- **WHEN** 在 `backend-go/` 中 grep `datamigrate`
- **THEN** 无任何匹配结果

### Requirement: CleanupBudget 类型删除

移除 `jobs/cleanup_budget.go` 整个文件及其测试 `jobs/cleanup_budget_test.go`。`CleanupBudget` 类型（5 个方法：`NewCleanupBudget`, `SetPhaseQuota`, `Consume`, `ConsumeForPhase`, `IsTimedOut`）零生产调用者，仅在其自身测试中使用。

#### Scenario: cleanup_budget.go 文件不存在
- **WHEN** 检查 `internal/jobs/` 目录
- **THEN** 不存在 `cleanup_budget.go` 文件

#### Scenario: cleanup_budget_test.go 文件不存在
- **WHEN** 检查 `internal/jobs/` 目录
- **THEN** 不存在 `cleanup_budget_test.go` 文件

#### Scenario: 无 CleanupBudget 引用残留
- **WHEN** 在 `backend-go/` 中 grep `CleanupBudget`
- **THEN** 无任何匹配结果（排除 vendor 目录）

### Requirement: autoMigrateModels 废弃函数删除

移除 `database/migrator.go` 中 `autoMigrateModels` 函数。此 deprecated wrapper 仅调用 `RunAutoMigrate(db)`，零调用者。

#### Scenario: autoMigrateModels 不存在
- **WHEN** 检查 `internal/platform/database/migrator.go`
- **THEN** 不存在 `autoMigrateModels` 函数
