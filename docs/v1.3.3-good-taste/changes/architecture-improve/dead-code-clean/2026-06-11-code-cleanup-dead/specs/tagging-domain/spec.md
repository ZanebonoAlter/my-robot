# Delta Spec: tagging-domain (code-cleanup-dead)

## REMOVED Requirements

### Requirement: TopicTag.Kind 写入

移除 `TopicTag.Kind` 的所有写入点。`Kind` 字段已标记为 deprecated，所有逻辑应使用 `Category` 字段代替。`Kind` 字段本身保留在 struct 中（需要后续 DB migration 移除），但不再被主动赋值。

#### Scenario: topicgraph/service.go 不写入 Kind
- **WHEN** 检查 `internal/domain/topicgraph/service.go` 中所有对 TopicTag 的赋值
- **THEN** 不存在 `.Kind =` 赋值语句

#### Scenario: tagging/tagger.go 不写入 Kind
- **WHEN** 检查 `internal/domain/tagging/tagger.go` 中所有对 TopicTag 的赋值
- **THEN** 不存在 `.Kind =` 赋值语句

#### Scenario: 全代码库无 Kind 写入
- **WHEN** 在 `backend-go/` 目录 grep `\.Kind\s*=`
- **THEN** 不存在对 TopicTag 或相关 struct 的 `.Kind` 赋值（非 TopicTag struct 的 Kind 字段不受影响）

### Requirement: database/db.go 废弃函数 Migrate 和 EnsureTables

移除 `database/db.go` 中 `Migrate()` 和 `EnsureTables()` 函数。这些函数已被 GORM AutoMigrate 替代，无调用者。

#### Scenario: Migrate 函数不存在
- **WHEN** 检查 `internal/platform/database/db.go` 中的导出函数
- **THEN** 不存在 `Migrate` 函数

#### Scenario: EnsureTables 函数不存在
- **WHEN** 检查 `internal/platform/database/db.go` 中的导出函数
- **THEN** 不存在 `EnsureTables` 函数

## MODIFIED Requirements

### Requirement: CST timezone 统一到 models/utils.go

将所有 CST timezone 定义收敛到 `models/utils.go` 中的单一常量。其他文件中的重复定义（`database/db.go`、`tagging/services.go`、以及 11+ 处内联 `time.FixedZone("CST", 8*3600)` 调用）全部替换为从 `models/utils.go` 导入。

#### Scenario: database/db.go 不再定义 CST timezone
- **WHEN** 检查 `internal/platform/database/db.go`
- **THEN** 不存在 `CST` 变量定义或 `time.FixedZone("CST", 8*3600)` 调用，改为从 `models/utils.go` 导入

#### Scenario: tagging/services.go 不再定义 CST timezone
- **WHEN** 检查 `internal/domain/tagging/services.go`
- **THEN** 不存在 `CST` 变量定义或 `time.FixedZone("CST", 8*3600)` 调用，改为从 `models/utils.go` 导入

#### Scenario: 全代码库无重复 CST 定义
- **WHEN** 在 `backend-go/` 目录 grep `FixedZone\("CST"`
- **THEN** 仅在 `internal/domain/models/utils.go` 中存在一处定义
