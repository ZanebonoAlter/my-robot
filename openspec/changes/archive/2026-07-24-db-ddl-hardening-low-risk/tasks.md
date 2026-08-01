# Tasks: db-ddl-hardening-low-risk（切片1 低风险治理）

> 来源：`docs/issues/02-quality-audit-2026-07-23/05-db-ddl.md` D-High-7 / D-Med-5 / M-High-3 + D-High-6 假注释。架构级改造（migrator 事务外迁移、3 套向量逻辑统一、CONCURRENTLY 索引、维度去硬编码）拆到独立 change `db-ddl-hardening-architecture`。
>
> 垂直切片，每切片独立可交付、可验证。推荐顺序：A 配置+守卫（堵生产丢数据）→ B 测试适配 → C 注释/幂等/tag 机械修复。尾部遵循《开发执行规范》§11 归档门禁。

<!-- doc-impact: configuration, deployment, database -->
<!--
  database 域：启发式命中（改了 internal/models/），但 tag 剥离不改 DB schema
    （NOT NULL/DEFAULT 约束由 20260723_0001 兜底），DATABASE_FIELDS.md 无需更新。
  flow 域：启发式命中（改了 tagmanagement/service/core/ 注释），但仅删假 IVFFlat
    注释、不改业务流程，flow/ 无需更新——无 flow 影响，按《开发执行规范》§12.2 豁免溯源。
  实际更新的文档：configuration.md（env 表）、deployment.md（破坏性迁移开关节）。
-->

## 1. 后端：配置层 + 破坏性迁移守卫（A · db-migration-safety）

- [x] 1.1 `internal/platform/config/config.go`：`DatabaseConfig` 加 `AllowDestructiveMigrations bool` 字段；`applyEnvOverrides` 加 `MIGRATIONS_ALLOW_DESTRUCTIVE`（值 `"1"` → true）处理，遵循既有 5 个 env override 范式。验收：`go test ./internal/platform/config` PASS；新增单测覆盖 env=`1`/未设/其他值三种情况
- [x] 1.2 `internal/platform/database/migrator.go`：`RunMigrations` 入口读取 `config.AppConfig.Database.AllowDestructiveMigrations`，存为包级 `allowDestructive` 变量；包级 helper `isDestructiveAllowed() bool` 暴露给迁移闭包读取。验收：单测覆盖标志读写
- [x] 1.3 `postgres_migrations.go` 守卫 3 条 TRUNCATE 迁移：`20260706_0001`（:1075 topic_lifeline_context）、`20260712_0001`（:1096 三表 CASCADE）、`20260718_0001`（:1157 两表）—— 每条 `Up` 开头加 `if !isDestructiveAllowed() { logging.Warnf("skipping destructive migration %s (set MIGRATIONS_ALLOW_DESTRUCTIVE=1 to enable)", "<version>"); return nil }`。验收：grep 三条迁移 Up 闭包含守卫
- [x] 1.4 集成测试（testcontainer）：未设 `MIGRATIONS_ALLOW_DESTRUCTIVE` 时，跑 InitDB 后断言 `topic_lifeline_context`/`topic_enrichment_result` 等表 schema 存在但未被 TRUNCATE（可插入测试数据后跑迁移断言数据保留）；设 `=1` 时断言 TRUNCATE 执行（数据被清）。验收：两个集成测试 PASS
- [x] 1.5 修正 `20260706_0001:1074` 注释：把「Dev env only — production would need a backfill script」改为指向守卫机制的实际说明。验收：注释与代码行为一致

## 2. 后端：testcontainer 测试路径适配（B · test-infrastructure）

- [x] 2.1 `internal/platform/testutil/testutil.go`：`runTestMigrations`（:433）函数体开头加 `t.Setenv("MIGRATIONS_ALLOW_DESTRUCTIVE","1")`，保证测试跑全量迁移含破坏性。验收：现有 testcontainer 集成测试全 PASS（维持「测试库跑全量生产迁移」不变量）
- [x] 2.2 检查 `ReimportTestDB`（回归测试逃生口）路径是否同样经过 `runTestMigrations`，若经则自动继承 t.Setenv；若不经则同样补。验收：grep 确认逃生口路径开启破坏性开关

## 3. 后端：删假注释 + SET NOT NULL 幂等 + tag 剥离（C · 内部质量）

- [x] 3.1 `internal/tagmanagement/service/core/embedding.go:449`：注释 `// For dimensions > 2000, uses IVFFlat instead of HNSW (HNSW limit is 2000).` 改为如实描述：`// For dimensions > 2000, skips index creation (pgvector HNSW limit is 2000); aux-label dedup falls back to Go-side cosine via process cache (see sqlMergeMatcher).`。同步 `:485` Infof 文案对齐（去掉含糊 "skipping"，明确"无索引，靠缓存计算"）。验收：`grep -rn "IVFFlat instead of\|uses IVFFlat" backend-go/` 为 0
- [x] 3.2 `postgres_migrations.go:1005`（`20260704_0001`）：裸 `ALTER TABLE ai_call_logs ALTER COLUMN operation SET NOT NULL` 改调 `ensureNotNullDefault(db, "ai_call_logs", "operation", "''")`。验收：testcontainer 连续两次 InitDB 无 error
- [x] 3.3 `internal/models/ai_models.go`：剥离 `default:`/`not null` tag（保留 metadata 类 jsonb 字段，本文件无此类例外，全剥）。验收：`grep -cE 'gorm:"[^"]*(not null|default:)' backend-go/internal/models/ai_models.go` 为 0
- [x] 3.4 `internal/models/topic_graph.go`：剥离 `default:`/`not null` tag，**保留** `Metadata` 字段的 `default:`（serializer:json 例外）。验收：grep 计数为 1（仅 Metadata）
- [x] 3.5 `internal/models/semantic_label.go`：剥离 tag，**保留** `Aliases` 字段的 `default:`（serializer:json 例外）。验收：grep 计数为 1（仅 Aliases）
- [x] 3.6 跑约束断言回归：`go test ./internal/platform/database -run TestModelTagConstraints` PASS（验证 tag 剥离后 DB 约束仍由 `20260723_0001` 兜底，无约束真空）

## 4. 文档（doc-impact: configuration, deployment）

- [x] 4.1 `docs/reference/deployment.md`：新增 `MIGRATIONS_ALLOW_DESTRUCTIVE` env 说明节——dev/本地启动设 `=1`（清理历史数据），生产**绝不设**（默认拒绝破坏性迁移）。验收：grep 文档含该 env 名 + 明确的生产禁用说明
- [x] 4.2 `docs/reference/configuration.md`：`DatabaseConfig` 字段表补 `AllowDestructiveMigrations`（env `MIGRATIONS_ALLOW_DESTRUCTIVE`，默认 false，说明用途）。验收：文档字段表与 config.go 结构体一致
- [x] 4.3 `configs/config.yaml`：加注释说明 `MIGRATIONS_ALLOW_DESTRUCTIVE`（不在 yaml 设值，仅 env 覆盖，避免误开）。验收：yaml 含注释说明
- [x] 4.4 `docs/issues/02-quality-audit-2026-07-23/05-db-ddl.md` + `README.md`：标记 D-High-7/D-Med-5/M-High-3/D-High-6(注释部分) 为 ✅ 已修复（本 change）。验收：issue 状态更新

## 5. 归档门禁（开发执行规范 §11）

### 测试
- [x] 5.1 `cd backend-go && go test ./internal/platform/config ./internal/platform/database ./internal/platform/testutil` — 全 PASS（含 testcontainer 集成测试，需 Docker）
- [x] 5.2 `cd backend-go && go test ./internal/tagmanagement/service/core` — PASS（embedding.go 注释改动不影响逻辑，跑现有测试确认无回归）
- [x] 5.3 `cd backend-go && go test ./internal/platform/database -run TestModelTagConstraints` — PASS（tag 剥离后约束兜底验证）

### 文档
- [x] 5.4 `bash scripts/doc-impact.sh verify openspec/changes/db-ddl-hardening-low-risk/` — 0 FAIL（configuration/deployment 域已更新）
- [x] 5.5 `bash scripts/check-standards.sh` — 全 PASS（含 H 段 model tag 守门，验证 tag 剥离未回潮）

### 验证
- [x] 5.6 `cd backend-go && golangci-lint run ./...` — 0 issues
- [x] 5.7 `cd backend-go && go vet ./... && go build ./...` — 无错误
- [x] 5.8 `grep -rn "IVFFlat instead of\|uses IVFFlat" backend-go/` — 输出为空（假注释已清）
- [x] 5.9 `grep -cE 'gorm:"[^"]*(not null|default:)' backend-go/internal/models/{ai_models,topic_graph,semantic_label}.go` — ai_models=0, topic_graph=1(Metadata), semantic_label=1(Aliases)
