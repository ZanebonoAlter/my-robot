# Tagging Domain (Delta)

## Purpose

Delta spec for `tagging-domain`: remove SQLite fallback code that exists solely for test compatibility, ensuring all production code paths use pgvector exclusively.

## Requirements

### Requirement: 移除 auxiliary_label_service 中的 SQLite fallback代码

系统 SHALL 从 `auxiliary_label_service.go` 中移除仅为 SQLite 测试兼容而存在的代码路径。`sqlMergeMatcher` 的 Go 侧余弦相似度计算 SHALL 被评估：如果 pgvector SQL 等价方案性能足够，替换为 SQL 查询；如果 Go 侧计算确为性能优化（非 SQLite fallback），保留但移除注释中的 "SQLite tests" 字样。

#### Scenario: auxiliary_label_service.go 不包含 SQLite 相关注释

- **WHEN** 检查 `auxiliary_label_service.go` 源码
- **THEN** 不存在 "SQLite" 或 "sqlite" 字样（注释或条件分支）

#### Scenario: sqlMergeMatcher 使用 pgvector 或有明确的性能优化理由

- **WHEN** 检查 `sqlMergeMatcher` 实现
- **THEN** 要么使用 pgvector SQL 距离操作符进行相似度匹配，要么保留 Go 侧计算但注释说明为 "performance optimization for high-dim vectors" 而非 "SQLite fallback"

### Requirement: 不引入新的 SQLite 依赖路径

重构 SHALL NOT 在 tagging 域任何生产代码中引入新的 SQLite 兼容逻辑。所有向量操作统一通过 pgvector 处理。

#### Scenario: 新代码不包含 SQLite 条件分支

- **WHEN** 检查 `internal/domain/tagging/` 下所有非 `_test.go` 文件
- **THEN** 不存在 `db.Name() == "sqlite"` 或类似的数据库类型条件判断
