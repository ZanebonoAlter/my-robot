## Why

`docs/issues/02-quality-audit-2026-07-23/05-db-ddl.md` DDL 专项审计发现 3 类低风险但高收益的迁移层隐患：(1) **3 条 TRUNCATE 迁移无环境守卫**（`20260706_0001`/`20260712_0001`/`20260718_0001`），其中 `20260706_0001:1074` 注释自承「Dev env only — production would need a backfill script」，但代码在生产启动时一样会无条件执行，清空 `topic_lifeline_context` 等业务表——**这是当前最紧迫的生产数据丢失风险**；(2) **`embedding.go:449` 注释谎称「uses IVFFlat instead of HNSW」**，实际 >2000 维只是 skip 索引（IVFFlat 分支从未实现），属反复出现的误导，曾让多人误以为高维向量有索引；(3) **`20260704_0001` 的 SET NOT NULL 非幂等**、**07-23 治理遗留的 30 处 GORM tag default 未剥离**（`20260723_0001` 已落地 DB 但 tag 仍在，双源竞争）。这些都是机械性修复，风险低、有现成模式可循，且能立即止血。架构级改造（migrator 事务外迁移、3 套向量逻辑统一、CONCURRENTLY 索引）拆到独立 change `db-ddl-hardening-architecture`，避免一个 change 改动面过大。

## What Changes

### A. 破坏性迁移环境守卫 —— 新增 `db-migration-safety` capability
- 配置层 `DatabaseConfig` 新增 `AllowDestructiveMigrations bool` 字段，env `MIGRATIONS_ALLOW_DESTRUCTIVE=1` 覆盖（复用现有 5 个 env override 范式 `config.go:104-128`）。
- `RunMigrations` 入口读取该配置存为包级标志；3 条 TRUNCATE 迁移（`20260706_0001`/`20260712_0001`/`20260718_0001`）的 `Up` 闭包开头加守卫，未开启时 `logging.Warnf` 并 `return nil`（跳过，不报错、不中止启动）。
- **BREAKING（部署行为）**：生产环境默认不执行破坏性迁移；dev/测试环境需显式设 `MIGRATIONS_ALLOW_DESTRUCTIVE=1` 才执行历史数据清理。这是有意的安全收紧——生产本就不该被 TRUNCATE。

### B. testcontainer 测试路径适配 —— 修改 `test-infrastructure` capability
- `testutil.go:433 runTestMigrations` 前置 `t.Setenv("MIGRATIONS_ALLOW_DESTRUCTIVE","1")`，保证测试仍跑全量迁移（含破坏性），维持「测试库复刻生产迁移路径」不变量。

### C. 删假注释 + 幂等修复 + tag 剥离（内部质量，无 spec 行为变化）
- `embedding.go:449` 假 IVFFlat 注释改为如实描述（>2000 维 skip 索引，aux-label 去重走 Go 缓存 cosine）。
- `20260704_0001:1005` 的非幂等 `SET NOT NULL` 改用 `ensureNotNullDefault`（`postgres_migrations.go:54-65`）。
- ai_models/topic_graph/semantic_label 三文件剥离 ~30 处 `default:`/`not null` tag（3 个 jsonb 字段 metadata/aliases/context_layers 保留 default，`serializer:json` 零值必需例外，07-23 已确认）。

## Capabilities

### New Capabilities
- `db-migration-safety`: 数据库迁移层的安全性行为约束——破坏性迁移（TRUNCATE/DROP）须由显式配置开关控制，生产默认拒绝，测试/dev 显式开启；幂等性要求（同迁移闭包可重复执行不报错）。

### Modified Capabilities
- `test-infrastructure`: 测试迁移路径需显式开启破坏性迁移开关，维持「测试库跑全量生产迁移」不变量。

## Impact

- **代码**：`internal/platform/config/config.go`（加字段+env）、`internal/platform/database/migrator.go`（读配置+包级标志）、`internal/platform/database/postgres_migrations.go`（3 条 TRUNCATE 守卫 + SET NOT NULL 幂等）、`internal/platform/testutil/testutil.go`（t.Setenv）、`internal/tagmanagement/service/core/embedding.go`（注释）、`internal/models/{ai_models,topic_graph,semantic_label}.go`（tag 剥离）。
- **部署**：`docs/reference/deployment.md` + `configs/config.yaml` 注释需说明 `MIGRATIONS_ALLOW_DESTRUCTIVE` 的用法（dev 设 1、生产不设）。
- **无 API/DB schema 变化**：tag 剥离有 `20260723_0001` 已落地的 DB 约束兜底，不改变实际 schema；守卫只影响迁移是否执行，不改表结构。
- **风险**：低。守卫逻辑简单（if 标志），tag 剥离有 `constraints_test.go` 兜底，幂等修复复用现成抽象。
