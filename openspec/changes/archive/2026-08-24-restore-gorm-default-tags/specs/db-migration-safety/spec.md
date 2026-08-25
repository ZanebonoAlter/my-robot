## MODIFIED Requirements

### Requirement: 版本迁移闭包 SHALL 幂等可重复执行

每条版本迁移的 `Up` 闭包 SHALL 可重复执行而不报错（幂等），即使对应 `schema_migrations` 版本已应用、人工补跑亦不应失败。涉及 `SET NOT NULL` 等非幂等 DDL 时，SHALL 复用 `ensureNotNullDefault` 抽象（先判 `columnIsNullable`、回填、再 `SET NOT NULL`）或 `DO $$ ... IF ... END $$` 守卫。

迁移内约束 helper SHALL 如实尊重其参数声明：`constrain(table, column, defaultLit, notNull)` 在 `notNull=false` 时 SHALL 仅 SET DEFAULT，不得因 defaultLit 非空而绕过 notNull 参数执行 SET NOT NULL（20260723_0001 的架空 bug——context_layers/aliases 两列被意外 NOT NULL）。

#### Scenario: SET NOT NULL 迁移重复执行不报错

- **WHEN** `ai_call_logs.operation` 列已是 NOT NULL 时重复执行 `20260704_0001` 的 SET NOT NULL 逻辑
- **THEN** 不报错（幂等返回成功），而非因 `ALTER COLUMN ... SET NOT NULL` 对已 NOT NULL 列重复操作而失败

#### Scenario: 重复执行 InitDB 不产生迁移错误

- **WHEN** 同一 testcontainer 数据库连续执行两次 `InitDB`（AutoMigrate + 全量版本迁移）
- **THEN** 第二次执行所有迁移闭包均成功（不因非幂等 DDL 报错）

#### Scenario: constrain helper 尊重 notNull=false 声明

- **WHEN** 空库重放 `20260723_0001` 且 cols 表声明某列 `notNull=false`（如 semantic_labels.context_layers / aliases）
- **THEN** 该列仅被 SET DEFAULT + 回填，不被 SET NOT NULL——列保持 nullable，与 cols 声明一致
