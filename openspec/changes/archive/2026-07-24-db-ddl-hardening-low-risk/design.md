# Design: db-ddl-hardening-low-risk

> 切片 1（低风险治理）。架构级改造（migrator 事务外迁移、3 套向量逻辑统一、CONCURRENTLY 索引、维度去硬编码）在独立 change `db-ddl-hardening-architecture`。

## 背景

`05-db-ddl.md` 审计 D-High-7/M-High-3 等低风险项的落地设计。关键约束：项目**无现成环境判断机制**（`IsDev`/`APP_ENV` 零命中），配置用 viper + 5 个固定 env override 范式（`config.go:104-128`）。

## 决策

### D1. 破坏性迁移守卫用「env 开关 + 包级标志」，不引入 APP_ENV 概念
- **选项**：(a) 新增专门 env `MIGRATIONS_ALLOW_DESTRUCTIVE`；(b) 引入通用 `APP_ENV=dev/prod`；(c) 用 `schema_migrations` 记录探测环境；(d) 在 `embedding_config` 表加 environment key。
- **选 (a)**。理由：与项目现有 env override 范式完全一致（认知成本最低、reviewer 最易接受）；默认安全（生产不设即拒绝）；语义专一（只管破坏性迁移，不背负环境判断的歧义）。不选 (b)：引入 APP_ENV 会牵动全项目环境语义，超出本 change 范围。不选 (c)：全新生产部署也从 0 跑迁移，会被误判成 dev，不可靠。不选 (d)：管理员可经 HTTP 改写该 key，且存在 seed 引导顺序耦合，语义错位。
- **实现**：`config.go` `DatabaseConfig` 加 `AllowDestructiveMigrations bool`；`applyEnvOverrides` 用 `os.LookupEnv("MIGRATIONS_ALLOW_DESTRUCTIVE")` 判 `== "1"`；`migrator.go` 在 `RunMigrations` 开头存 `allowDestructive = config.AppConfig.Database.AllowDestructiveMigrations` 为包级 var（迁移闭包读取）。

### D2. 守卫粒度：单条迁移闭包内判断，非 RunMigrations 全局跳过
- 守卫写在每条 TRUNCATE 迁移的 `Up` 闭包开头（`if !allowDestructive { warn; return nil }`），而非在 `RunMigrations` 循环里按版本号跳过。
- **理由**：破坏性意图应在迁移自身声明（自文档化），而非集中在一个 if-else 版本号清单里。未来新增 TRUNCATE 迁移时，作者在闭包里加守卫即可，无需改 migrator。`return nil` 而非 error——跳过是预期行为（生产本就不该跑），不是失败；但 `schema_migrations` 仍记录该 version 为 applied（避免下次启动重复 warn 日志）。

### D3. schema_migrations 记录策略：跳过的迁移仍标记 applied
- 被守卫跳过的迁移，`RunMigrations` 仍照常 `INSERT INTO schema_migrations`。
- **理由**：(1) 避免每次生产启动重复打 warn 日志；(2) 该迁移在生产的"应有状态"就是"不执行"，标记 applied 语义正确；(3) 若生产将来真需要执行（如补了 backfill 脚本），可手动 `DELETE FROM schema_migrations WHERE version=...` 重跑——这是运维操作，不在自动路径。

### D4. tag 剥离边界：保留 3 个 jsonb 字段的 default
- `20260723_0001` 已把 ai_models/topic_graph/semantic_label 的 NOT NULL/DEFAULT 落地 DB。本次从 tag 删除 `default:`/`not null`，但**保留** `metadata`/`aliases`/`context_layers` 三个 jsonb 字段的 `default:`。
- **理由**：这三个字段用 `serializer:json`，GORM 序列化时零值（nil slice/map）会省略，导致写库时 default 不触发→列值 NULL→读取反序列化失败。07-23 治理时已确认此例外（见 `02-backend-code-design.md` M2 治理记录）。

### D5. SET NOT NULL 幂等复用 ensureNotNullDefault
- `20260704_0001:1005` 的裸 `ALTER ... SET NOT NULL` 改调 `ensureNotNullDefault(db, "ai_call_logs", "operation", "''")`（先 SET DEFAULT '' → 回填 NULL 行 → SET NOT NULL，全程 `columnIsNullable` 守卫幂等）。
- **注意**：`operation` 是 varchar(80)，业务上非空，回填用空串 `''` 安全（历史 NULL 行若存在，赋空串不破坏语义）。

### D6. 假注释修正不触碰逻辑
- `embedding.go:449` 只改注释文字，不改 `ensureVectorDimension` 函数逻辑（逻辑统一在架构 change 做）。同步修 `:485` 的 Infof 文案。

## 风险与回滚

- **守卫误判**：若生产误设 `MIGRATIONS_ALLOW_DESTRUCTIVE=1`，3 条迁移会清数据。缓解：deployment.md 明确「生产绝不设」；守卫 warn 日志在启动时可见。
- **tag 剥离漏删**：`constraints_test.go`（07-23 加的约束断言测试）+ check-standards H 段守门兜底，约束真空会立刻测出。
- **回滚**：纯加法/注释/tag 修改，git revert 即可，无数据迁移不可逆风险。

## 不做（留架构 change）

migrator `RunOutsideTx`、向量 `vector_dim.go` 统一 helper、维度去硬编码、CONCURRENTLY 索引——均属架构级，单独 change。
