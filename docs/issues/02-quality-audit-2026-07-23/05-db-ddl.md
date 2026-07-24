# 数据库 DDL 与迁移设计专项审计报告

> **审计对象**: `backend-go/internal/platform/database/`（迁移层）+ `backend-go/internal/models/`（模型层）+ 运行时 schema ensurer
> **对照规范**: `docs/reference/database/{DATABASE_FIELDS,ER_DIAGRAM,DATA_LIFECYCLE}.md` + `docs/reference/standard/backend/code-style.md` + `docs/reference/开发执行规范.md`
> **评级**: 迁移层 **B−**（幂等性扎实、文档化优秀，但生产可用性维度有系统性短板）｜ 模型层 **B+**（主键/表名/文档对账干净，tag 与迁移分工有长尾）
> **审计日期**: 2026-07-24
> **定位**: 07-23 三路审计（02 后端 / 03 文档 / 04 前端）的【补充维度】。07-23 的 M2 仅从「GORM tag 违反迁移分工」单一视角触及 DDL，本报告补齐迁移编写规范、索引策略、约束完整性、向量/pgvector、锁表风险等独立维度。模型层 tag 长尾与 02 报告 M2 同源，此处仅作 DDL 视角补充并交叉引用，不重复立案。

## 关键执行上下文（影响多条结论，先读这 3 个事实）

| 事实 | 位置 | 影响 |
| ---- | ---- | ---- |
| **AutoMigrate 先于版本迁移执行**，且每次启动都跑 | `database/db.go:36-44` | GORM tag 的 `not null`/`default:` 会被 AutoMigrate 真正下推到 DB，与「显式迁移为唯一权威」竞争 |
| **`DisableForeignKeyConstraintWhenMigrating: true`** | `database/db.go:21` | AutoMigrate 不创建任何外键；全库参照完整性靠应用层 |
| **每条迁移被包在 `db.Transaction(...)` 里执行** | `database/migrator.go:89` | `CREATE INDEX CONCURRENTLY` 在事务内会直接报错——这是全库索引只能用阻塞式 `CREATE INDEX` 的根因 |

迁移主体：`backend-go/internal/platform/database/postgres_migrations.go`（1561 行，约 40 条迁移）。

---

## 技术债治理进度（2026-07-24）

| 治理项 | 状态 | 说明 |
| ------ | ---- | ---- |
| M2 model tag Top3（106/154 处） | ✅ 已完成（07-23） | 详见 02 报告。迁移 `20260723_0001` 兜底 + 约束断言测试 + check-standards H 段守门 |
| M2 model tag 长尾（48 处） | ⏳ 用户决策跳过（07-23） | 详见 02 报告。本报告 §模型层-1 从 DDL 视角再次确认这 48 处的分布与风险 |
| **D-High-7 破坏性迁移守卫** | ✅ 已修复（07-24，change `db-ddl-hardening-low-risk`） | 3 条 TRUNCATE 迁移（20260706/0712/0718）加 `MIGRATIONS_ALLOW_DESTRUCTIVE` env 守卫，生产默认拒绝，dev/test 设 `=1` |
| **D-Med-5 SET NOT NULL 幂等** | ✅ 已修复（07-24） | `20260704_0001` 改用 `ensureNotNullDefault`（幂等） |
| **M-High-3 tag 剥离（30 处 default）** | ✅ 已修复（07-24） | ai_models/topic_graph/semantic_label 三文件剥离 default/not null（保留 3 个 jsonb serializer 例外）；check-standards H 段守门验证计数 |
| **D-High-6 假注释** | ✅ 已修复（07-24，注释部分） | `embedding.go:449` IVFFlat 假注释改为如实描述；3 套逻辑统一（helper 抽取）留架构 change |
| **DDL 迁移层专项（架构级，D-High-1/3/4/5/6）** | ⏳ 待执行（change `db-ddl-hardening-architecture`） | migrator 事务外迁移、向量维度去硬编码、3 套逻辑统一、大表索引 CONCURRENTLY |

> **门禁**：本次为只读审计，未改动代码。下方所有优化方案均为建议，执行前需按 `开发执行规范.md §0.6` 走 openspec change 编排。

---

## 正面发现（保持）

| 亮点 | 位置 | 说明 |
| ---- | ---- | ---- |
| 幂等性整体扎实 | `postgres_migrations.go` 全文 | 绝大多数 DDL 用 `IF NOT EXISTS`/`IF EXISTS`/`CREATE OR REPLACE`/`DO $$ ... IF NOT EXISTS ... END $$` 守卫，可重跑 |
| `ensureNotNullDefault` 抽象复用得当 | `postgres_migrations.go:54-65` | 先 `columnIsNullable` 检查再 ALTER，幂等设计典范；可惜未被所有迁移复用（见 D-中-4） |
| 迁移 Description 文档化优秀 | 各迁移 | 每条都有描述，部分带详细背景注释（如 `20260620_0001` 讲清扩宽唯一约束原因、`20260626_0001` 讲语义翻转） |
| 主键/表名/无软删除统一 | `models/` 全部 27 struct | 统一 `ID uint` + 手写 `CreatedAt`/`UpdatedAt`，无 `gorm.Model`/`gorm.DeletedAt`/`deleted_at` 混用 |
| 文档对账干净 | `DATABASE_FIELDS.md` | 本目录 27 struct 字段与文档一一对应，经 2026-07-18 系统性清理后无幽灵字段（仅 1 处悬空，见 M-中-3） |
| ER_DIAGRAM FK 矩阵吻合 | `ER_DIAGRAM.md` Part A/B | 与模型 `foreignKey`/`constraint:OnDelete` 声明完全吻合 |
| 启动期 schema ensurer 有 lock_timeout 先例 | `topicgraph/repository/daily_report_models.go:299` | 设了 5s `lock_timeout`，是全库唯一有此守卫的 schema 变更路径——应推广到所有 ALTER TYPE/ADD CONSTRAINT |

---

## 迁移层问题清单（`postgres_migrations.go`）

### D-High（生产数据安全 / 性能隐患，建议优先处理）

#### [D-High-1] 索引创建全部用阻塞式 `CREATE INDEX`，大表锁表写入
- **位置**: 几乎所有 `CREATE INDEX` 语句（`20260417_0001`/`20260418_0001`/`20260420_0001`/`20260430_0001`/`20260417_0002` GIN/`20260521_0001` 10 个索引/`20260526_0001`/`20260529_0001`/`20260630_0002`/`20260704_0001`/`20260717_0001` 等）
- **问题**: 全文检索 GIN 索引、各业务表二级索引全部阻塞写入。`articles`（FTS GIN）、`ai_call_logs`、`topic_tag_embeddings` 这类大/热表尤甚。
- **根因**: `migrator.go:89` 把迁移包在事务里，而 PG 不允许 `CONCURRENTLY` 在事务内执行（见 D-High-3）。
- **建议**: 先解 D-High-3（migrator 支持事务外迁移），再把大表索引改 `CREATE INDEX CONCURRENTLY`。

#### [D-High-2] 外键约束基本缺失，参照完整性无 DB 兜底
- **位置**: `database/db.go:21` `DisableForeignKeyConstraintWhenMigrating: true` + 迁移全文仅 `20260601_0001`（`:556-557`）给 `topic_tags.merged_into_id` 补了 1 个 FK
- **问题**: `articles.feed_id`、`article_topic_tags.(article_id,topic_tag_id)`、`feeds.category_id`、`topic_tag_embeddings.topic_tag_id`、`board_composition`、`topic_tag_semantic_labels`、`daily_report_*` 的 FK 全靠应用层保证，孤儿行风险高。
- **建议**: 分批补关键 FK。用 `ALTER TABLE ADD CONSTRAINT ... NOT VALID`（不扫已有数据、不长锁）+ 之后 `VALIDATE CONSTRAINT`（可并发）。优先级：`articles→feeds`、`article_topic_tags→articles/topic_tags`、`topic_tag_embeddings→topic_tags`。

#### [D-High-3] migrator 事务包裹一切，阻断 `CONCURRENTLY` 路径（根因）
- **位置**: `database/migrator.go:89` `db.Transaction(func(tx) error { return migration.Up(tx) })`
- **问题**: `Migration` 结构体（`migrator.go:12-16`）只有 `Up func(db *gorm.DB) error`，且强制事务执行。任何 `CREATE INDEX CONCURRENTLY` 都会报错。这是 D-High-1 的根因。
- **建议**: 扩展 `Migration` 加 `RunOutsideTx bool` 字段（或 `Kind: "index"|"schema"|"data"`），让索引类迁移脱离外层事务。改动集中在 migrator 一个文件，收益覆盖 D-High-1 全部索引迁移。

#### [D-High-4] `ALTER COLUMN TYPE vector(N)` 全表重写 + 无 lock_timeout
- **位置**: `20260403_0003`（`postgres_migrations.go:89-94`）`ALTER TABLE topic_tag_embeddings ALTER COLUMN embedding TYPE vector(4096)`；运行时 `ensureVectorDimension`（`tagmanagement/service/core/embedding.go:471-475`、`topicgraph/repository/daily_report_models.go:320-325,372-377`）会再次 ALTER
- **问题**: 每次维度变更都全表重写，长时间 AccessExclusiveLock。迁移文件内的 ALTER 完全没有 `lock_timeout` 保护（只有 daily_report 的 startup ensurer 设了 5s）。
- **建议**: 给所有 `ALTER TABLE ... ALTER COLUMN TYPE` / `ADD CONSTRAINT` 加 `SET LOCAL lock_timeout '5s'` 守卫（复用 daily_report_models.go:299 先例）。

#### [D-High-5] 向量维度三方矛盾（迁移硬编码 4096 / seed 1024 / 运行时 2560）
- **位置**: `20260403_0003`（`:87-97`）硬编码 `vector(4096)` ↔ seed `embedding_dimension=1024`（`:108`）↔ 运行时 2560（`auxiliary_label_service.go:214` 注释）
- **问题**: 迁移建的 4096 维与 seed 的 1024 维直接矛盾，首启即被运行时 ensurer 改写，纯属误导。且模型 tag 写无维度 `type:vector`（见 M-High-1），AutoMigrate 每次启动做不可靠 schema 比较。
- **建议**: 迁移改为建无维度 `vector` 列（与 `daily_report_sections.embedding` 做法 `:677` 一致），维度统一交给运行时 `ensureVectorDimension`。tag 同步改 `type:vector(4096)` 或去掉维度让 ensurer 管。

#### [D-High-6] >2000 维向量无任何索引，退化为顺序扫描
- **位置**: `tagmanagement/service/core/embedding.go:449,478-486`；`topicgraph/repository/daily_report_models.go:329-332,381-384`
- **问题**: 对 >2000 维只 Warn 不建索引，注释里写「uses IVFFlat instead of HNSW」（`:449`）但代码未实现 IVFFlat 分支。2560 维辅助标签 embedding 无向量索引，aux-label 去重/匹配全表扫描。
- **建议**: 实现 IVFFlat 分支（`USING ivfflat (embedding vector_cosine_ops) WITH (lists=...)`，需先 `SET maintenance_work_mem`、且 ivfflat 需先有数据再建），或切 `halfvec`/降维。

#### [D-High-7] `20260706_0001` TRUNCATE 标注 dev-only，生产会丢数据
- **位置**: `postgres_migrations.go:1055-1080`，注释 `:1074` 写「Dev env only — production would need a backfill script」
- **问题**: 该迁移 TRUNCATE `topic_lifeline_context` 全表且无 backfill，生产环境执行会清数据。这是当前最紧迫的数据安全风险。
- **建议**: 补生产 backfill 脚本，或加环境守卫（`SELECT current_setting('server_version')`/应用层 env 判断，dev 才 TRUNCATE）。对所有 TRUNCATE/DROP 迁移统一加 `[DESTRUCTIVE]` 标注。

### D-Medium（规范一致性 / 锁表中等风险）

#### [D-Med-1] 索引命名前缀混用，UNIQUE 索引叫 `idx_`
- **位置**: `idx_topic_tag_embeddings_tag_type_hash`（`:253`）、`idx_semantic_labels_slug`（`:266`）、`idx_watch_section_report`（`:986`）、`idx_ai_route_provider_link` 是 UNIQUE 却叫 `idx_`；而 `uq_section_relations_pair`、`uq_board_upgrade_suggestions_hash` 用 `uq_`
- **建议**: 统一规则：UNIQUE 一律 `uq_<table>_<cols>`，普通 `idx_`，CHECK 用 `chk_`，FK 用 `fk_`。旧名保留（改名 = DROP+CREATE 锁表），仅约束新增。

#### [D-Med-2] `idx_topic_gran` 被误当约束 DROP（语义错误）
- **位置**: `20260706_0001`（`:1067-1068`）对 `idx_topic_gran` 既 `DROP CONSTRAINT IF EXISTS` 又 `DROP INDEX`
- **问题**: `idx_topic_gran` 是 unique index 不是表约束，`DROP CONSTRAINT` 对索引无效（`IF EXISTS` 不报错但语义错），说明作者对 unique index vs unique constraint 区分不清。
- **建议**: 明确区分 UNIQUE INDEX（`DROP INDEX`）与 UNIQUE CONSTRAINT（`DROP CONSTRAINT`）。

#### [D-Med-3] `merged_into_id ON DELETE CASCADE` 语义存疑
- **位置**: `20260601_0001`（`:496-566`，尤其 `:556-557`）
- **问题**: 自引用 `topic_tags.merged_into_id` 改 CASCADE——合并目标被删时级联删源标签可能丢数据，通常应 `SET NULL`。且该迁移专门删除子表上重复的 `NO ACTION` 约束再补 CASCADE，说明历史上 FK 级联不一致。
- **建议**: 审视 CASCADE 是否真要删源标签；多数自引用「指向」关系更适合 `SET NULL`。建立统一级联策略文档。

#### [D-Med-4] CHECK 约束覆盖不足，枚举字段靠应用层
- **位置**: 仅 `board_persistent_topics.status/source`（`:760-774`）、`board_topic_watches.status`（`:960-971,1034-1045`）有 CHECK；大量枚举字段（`topic_tags.status/category/kind/source`、`semantic_labels.status/label_type/source`、`tag_merge_suggestions.status/source`、`ai_routes.strategy` 等）无 CHECK
- **问题**: 同库 status 字段约束不一致（有的有 CHECK 有的没有，如 `semantic_labels.status` 无 CHECK 而 `board_persistent_topics.status` 有）。
- **建议**: 对高频枚举字段补 `CHECK (... IN (...))`，用与现有 `chk_*` 一致的幂等 `DO $$ ... END $$` 模式。

#### [D-Med-5] `20260704_0001` 的 SET NOT NULL 非幂等
- **位置**: `:1005` `ALTER TABLE ai_call_logs ALTER COLUMN operation SET NOT NULL`（未做 nullable 检查）
- **问题**: PG `SET NOT NULL` 非幂等，人工补跑会因已 NOT NULL 报错。与 `20260723_0001` 的 `constrain()` helper（幂等）思路不一致。
- **建议**: 复用 `ensureNotNullDefault` 模式或在 `DO $$` 里加 nullable 判断。

#### [D-Med-6] 大批量 UPDATE 无分批，长事务阻塞 VACUUM
- **位置**: `20260417_0002` 全表 UPDATE articles search_vector（`:232-234`）；`20260617_0001` 全表 UPDATE feeds icon_source（`:720-735`）；`20260723_0001` 对 ~30 表 × 多列回填（`:54-65`）
- **问题**: 单事务内大表全表 UPDATE，WAL 撑大、长事务阻塞 VACUUM。
- **建议**: 数据回填迁移分批（`WHERE id BETWEEN`/`ctid` 分片）+ 每批 COMMIT，或单独脚本离线跑。

#### [D-Med-7] 完全无 down/回滚迁移
- **位置**: `migrator.go:12-16` `Migration` 结构体只有 `Up`；多处「Rollback:」注释（如 `:985`）只是注释不可执行
- **问题**: 破坏性操作（DROP COLUMN/TABLE/TRUNCATE）一旦执行无法回退。
- **建议**: 至少为破坏性迁移补 down 迁移或明确「不可逆」标注 + 备份要求。扩展 `Migration` 加 `Down` 字段（与 D-High-3 的结构体改造合并做）。

#### [D-Med-8] `ADD CONSTRAINT UNIQUE` 扫全表锁表
- **位置**: `:595-599`、`:867-872`
- **问题**: 大表上 ADD CONSTRAINT UNIQUE 会扫全表验证并锁表，无 lock_timeout。
- **建议**: 用 `ALTER TABLE ADD CONSTRAINT ... NOT VALID` + 之后 `VALIDATE CONSTRAINT`（可并发，锁轻）。

#### [D-Med-9] HNSW 索引未指定构建参数
- **位置**: `embedding.go:480`、`daily_report_models.go:335,387`
- **问题**: 三个 HNSW 索引全用默认值，对召回质量敏感场景通常次优。
- **建议**: 按数据规模调参 `WITH (m = 16, ef_construction = 64)`，并把 `ef_search` 写入会话。

#### [D-Med-10] 版本号字母后缀破坏纯数字约定
- **位置**: `20260601_0001b`（`:570`）
- **问题**: 字典序排序下 `_0001b` 与 `_0002` 的顺序极易出错（当前 `'1'<'2'` 恰好正确，但脆弱）。
- **建议**: 弃用字母后缀，改纯数字 `20260601_0011`。

#### [D-Med-11] `fmt.Sprintf` 拼 DDL 标识符（模式危险，当前无注入）
- **位置**: `ensureNotNullDefault`（`:58,61`）、`20260723_0001` 的 `constrain` 闭包（`:1195`）
- **问题**: 当前调用方全是硬编码常量，无实际注入风险，但模式不安全——若日后接受外部输入则危险。
- **建议**: 标识符无法参数化，建议加白名单校验（`^[a-zA-Z_][a-zA-Z0-9_]*$`）；值字面量尽量参数化。加 `//nosec G201` 注释说明调用方为硬编码常量。

#### [D-Med-12] 索引双重声明（tag + 迁移），单一真相源缺失
- **位置**: `idx_topic_tag_embeddings_tag_type_hash`（模型 `topic_graph.go:121-126` + 迁移 `20260514_0001:253`）、`idx_semantic_labels_*`（`semantic_label.go:8,14,20` + `20260521_0001:266-268`）、`idx_narrative_boards_semantic_board_id`（`narrative_board.go:15` + `:275`）
- **问题**: 靠 `IF NOT EXISTS` 兜底未冲突，但职责重复。
- **建议**: 明确单一真相源——要么完全由 tag 管（AutoMigrate 幂等建），要么完全由迁移管（删 tag）。

### D-Low（次要）

- **D-Low-1**: 种子数据用「先 First 再 Create」，Create 失败仅 `logging.Warnf` 吞错（`:114,313,825,1128`），可能丢配置项。建议返回 error。
- **D-Low-2**: `idx_topic_tag_embeddings_tag_type`（2列，已删）遗留风险——若环境停在 `20260418_0001` 之后、`20260514_0001` 之前会残留。靠版本顺序保证，可加幂等清理。
- **D-Low-3**: `20260603_0001` 5 步（加约束+迁数据+DROP 多列）揉在一个迁移，粒度过大失败难定位。建议拆分。

---

## 模型层问题清单（`backend-go/internal/models/`）

> 与 02 报告 M2 同源的项此处仅作 DDL 视角补充，不重复立案。

### M-High（与 DDL 直接相关）

#### [M-High-1] `topic_graph.go:123` vector 维度 tag/迁移不一致
- **位置**: `topic_graph.go:123` `EmbeddingVec string gorm:"type:vector;column:embedding"`
- **问题**: tag 声明无维度 `type:vector`，但迁移 `20260403_0003` 固定为 `vector(4096)`。AutoMigrate 每次启动看到 tag 写 `vector`（无维度）与 DB 现有 `vector(4096)` 不一致，GORM 对 vector 类型的 ALTER 行为不可靠。
- **建议**: tag 改为 `type:vector(4096)` 与迁移对齐，或配合 D-High-5 一起去掉维度让 ensurer 管。
- **关联**: D-High-5（维度三方矛盾的模型侧）。

#### [M-High-2] 13 文件未纳入 NOT NULL/DEFAULT 收敛迁移（M2 长尾，DDL 视角）
- **位置**: `job_queue.go`/`feed.go`/`narrative.go`/`narrative_board.go`/`embedding_queue.go`/`article.go`/`category.go`/`reading_behavior.go`/`topic_tag_relation.go`/`board_upgrade_suggestion.go`/`embedding_config.go`/`user_preference.go`/`merge_reembedding_queue.go`
- **问题**: 这些表的 NOT NULL+DEFAULT 完全由 GORM tag 经 AutoMigrate 下推，无显式迁移固化，与 ai-call-logging 事故同型风险。07-23 已标记「用户决策跳过」。
- **建议**: 下一轮 `2026072x` 迁移补齐（与 D-High-3 migrator 改造同期做，一次性收口 tag/迁移分工）。
- **关联**: 02 报告 M2。

#### [M-High-3] `20260723_0001` 已落地但 tag 未剥离（30 处 default:）
- **位置**: `ai_models.go:12,15,18-21,68,72-73,79,93-95,97,111-112,130`；`topic_graph.go:55-58,60,62,69,122,147-148,160-161`；`semantic_label.go:16,18-21,23,25,51,53,54`
- **问题**: 「落地未剥离」使 tag 与迁移仍双源竞争（AutoMigrate 仍会下推 tag 的 default）。
- **建议**: 从 tag 删除 `default:`/`not null`（3 个 jsonb 字段 metadata/aliases/context_layers 保留 default，serializer:json 零值必需，07-23 已确认例外）。
- **关联**: 02 报告 M2 Top3 治理的收尾步。

### M-Medium

#### [M-Med-1] 索引双重声明 + 反向遗漏（规范双向都不齐）
- **双重声明**（删 tag）: 见 D-Med-12。
- **反向遗漏**（补迁移）: `idx_tag_analysis_date`（`topic_tag_analysis.go:8-11`）、`idx_cursor_tag_type_window`（`:23-25`）、`idx_tag_relation_pair`（`topic_tag_relation.go:9-10`）、`idx_tag_merge_suggestion_pair`/`idx_tag_merge_suggestion_status_sim`（`topic_graph.go:141-147`）、`idx_ai_routes_capability_name`（`ai_models.go:91-92`）、`idx_ai_route_provider_link`（`:109-110`）——这些 unique/复合索引仅 tag 声明无显式迁移，只能靠 AutoMigrate 建。
- **建议**: 补显式迁移（与 D-High-3 migrator 改造同期）。

#### [M-Med-2] `article.go:10` `CategoryID gorm:"-"` 悬空字段
- **位置**: `article.go:10` `CategoryID *uint gorm:"-"`，`ToDict()`（`:45`）仍输出
- **问题**: 物理无列、文档（DATABASE_FIELDS.md articles 表）未登记、但 API 仍返回 `category_id`。
- **建议**: 明确标注为「仅入参」并从 `ToDict()` 剥离，或登记到文档。

#### [M-Med-3] `narrative.go:21` 值类型关联 + 可空外键
- **位置**: `narrative.go:21` `Board NarrativeBoard gorm:"foreignKey:BoardID"`（值类型），`BoardID *uint`（可空）
- **问题**: `board_id` 为 NULL 时 GORM 返回零值 `NarrativeBoard{}`（非 nil），`json:"board"` 序列化出空对象；无 `omitempty`。其它关联（`article.go:34 Feed` 值类型、`feed.go:30 Category *Category` 指针）混用，风格不统一，N+1 preload 隐患。
- **建议**: 统一改指针 + `omitempty`。

#### [M-Med-4] jsonb 字段处理方式不统一
- **位置**: `ai_models.go:137` `TokenUsage string gorm:"type:jsonb"`（手写 `ToJSONValue` 序列化）vs `topic_graph.go:63` `Metadata MetadataMap`、`semantic_label.go:15 Aliases []string`、`board_upgrade_suggestion.go:18,20`（用 `serializer:json` 自动序列化）
- **问题**: 同库 jsonb 两种处理并存，`TokenUsage` 易漏序列化。
- **建议**: 统一为 `serializer:json`，或在注释写明「必须用原生 SQL/手写序列化」。

#### [M-Med-5] `semantic_label.go:59` 关联命名误导
- **位置**: `semantic_label.go:59` `TopicTagBoardLabel.SemanticBoard *SemanticLabel`（字段名 SemanticBoard 指向 SemanticLabel 表的 board 行）
- **问题**: 命名易误导但功能正确（ER_DIAGRAM.md B-1 确认指向 `semantic_labels.id`）。
- **建议**: 改字段名为 `SemanticBoardLabel` 或加注释。

### M-Low（次要）

- **M-Low-1**: `topic_graph.go:73` Deprecated 的 `Embedding` 关联与 `Embeddings` 并存，preload 误用风险。择期移除 `Embedding`。
- **M-Low-2**: `ai_models.go` `SchedulerTask`/`AISettings` 缺 `TableName()`，依赖默认复数「碰巧对」。补齐。
- **M-Low-3**: `topic_graph.go:124` `Dimension int gorm:""` 空 tag 无意义冗余。
- **M-Low-4**: 主键宽度声明不一致——`TopicTagAnalysis`/`TopicAnalysisCursor` 用 `uint64`，队列表（文档标 BIGSERIAL）用 `uint`。统一。
- **M-Low-5**: `ai_models.go:124` `Operation` 用 `type:varchar(80)` 而其它字段用 `size:N`，风格不统一。
- **M-Low-6**: `topic_graph.go:69` `TopicTag.Kind` 文档已标废弃，择期迁移移除。

---

## Top 7 优先处理（按 ROI：风险降低/性能提升 vs 改动成本）

| # | 问题 | ROI | 改动成本 | 风险 | 建议执行方式 |
| --- | --- | --- | --- | --- | --- |
| **1** | **D-High-7** `20260706_0001` TRUNCATE 生产丢数据 | 🔴 极高 | 小（加环境守卫/backfill 脚本） | 数据安全 | **立即**，无需等批量改造 |
| **2** | **D-High-3** migrator 支持事务外迁移（根因） | 🟠 高 | 中（改 migrator + Migration 结构体） | 低（向后兼容，老迁移仍走事务） | 作为 DDL 改造的**第一步**，解锁 D-High-1/D-Med-8 |
| **3** | **D-High-5 + M-High-1** 向量维度三方矛盾 | 🟠 高 | 小（迁移改无维度列 + tag 对齐） | 低（首启即被 ensurer 改写，改对齐即可） | 与 #2 同期，一次性消除维度误导 |
| **4** | **D-High-6** >2000 维无向量索引 | 🟠 高 | 中（实现 IVFFlat 分支或 halfvec） | 中（需测试召回） | 独立 change，需 A/B 验证 aux-label 匹配质量 |
| **5** | **D-High-1** 大表索引改 CONCURRENTLY | 🟡 中高 | 小（依赖 #2 完成） | 低（CONCURRENTLY 更安全） | #2 完成后批量改，优先 articles/ai_call_logs/topic_tag_embeddings |
| **6** | **D-High-2** 补关键外键 | 🟡 中 | 中（分批 + NOT VALID/VALIDATE） | 中（需先清理现有孤儿数据） | 独立 change，先查孤儿数据量再决定 |
| **7** | **M-High-3 + M-High-2** tag 收口（剥 30 处 + 补 13 文件迁移） | 🟡 中 | 中（机械但量大） | 低（有约束断言测试兜底） | 与 02 报告 M2 长尾合并做，check-standards H 段已守门 |

> 次优先（D-Med 批量）：D-Med-5（SET NOT NULL 幂等）、D-Med-4（CHECK 补齐）、D-Med-12/M-Med-1（索引单一真相源）、D-Med-9（HNSW 调参）——可在 #2/#7 的 change 里顺手做。

---

## 执行建议（按最小风险/性价比分层）

### 🟢 可立即执行（低风险、小改动、高收益）
- **D-High-7**: 给 `20260706_0001` 加生产环境守卫（读取 app env，非 dev 跳过 TRUNCATE）。**这是唯一建议不依赖批量改造、可单独提 change 的高危项。**
- **D-Med-5**: `20260704_0001` 的 SET NOT NULL 改幂等（复用 ensureNotNullDefault）。
- **M-High-3**: 从 ai_models/topic_graph/semantic_label 三文件 tag 剥离 30 处 `default:`（jsonb 3 字段保留）。有 constraints_test.go 兜底。

### 🟡 建议作为一个 openspec change 批量做（中等风险、需要编排）
以 **D-High-3（migrator 改造）为锚点**，一个 change 内收口：
- D-High-3：Migration 加 `RunOutsideTx` 字段
- D-High-5 + M-High-1：向量维度去硬编码（迁移改无维度列 + tag 对齐）
- D-High-1：大表索引改 CONCURRENTLY（依赖 D-High-3）
- D-High-4：给 ALTER TYPE/ADD CONSTRAINT 加 lock_timeout
- D-Med-7：Migration 加 `Down` 字段（与 D-High-3 结构体改造同期）
- M-High-2：补 13 文件 NOT NULL/DEFAULT 收敛迁移（与 02 报告 M2 长尾合并）

### 🔴 需用户决策（高风险/大改动/需验证）
- **D-High-6**: >2000 维向量索引方案选型（IVFFlat vs halfvec vs 降维）——影响召回质量，需 A/B 验证。
- **D-High-2**: 是否补外键——补 FK 需先清理现有孤儿数据，且 `DisableForeignKeyConstraintWhenMigrating` 的设计初衷（避免迁移环依赖）需重新评估。
- **D-Med-3**: `merged_into_id ON DELETE CASCADE` 改 SET NULL——涉及合并语义，需确认业务无依赖。

### ⚪ 暂不建议动（收益低/改动大）
- D-Med-1（索引重命名）：旧索引改名 = DROP+CREATE 锁表，收益仅美观。仅约束新增索引命名。
- D-Low-1/2/3、M-Low 全部：收益低，留待自然迭代。

---

## 关键文件路径（整改入口）

| 文件 | 说明 |
| ---- | ---- |
| `backend-go/internal/platform/database/migrator.go` | 迁移执行器（事务包裹根因 `:89`、结构体 `:12-16`） |
| `backend-go/internal/platform/database/db.go` | FK 关闭开关 `:21`、AutoMigrate 先于迁移 `:36-44` |
| `backend-go/internal/platform/database/postgres_migrations.go` | 迁移主体（1561 行） |
| `backend-go/internal/tagmanagement/service/core/embedding.go` | 向量索引/维度 ensurer `:449-489` |
| `backend-go/internal/topicgraph/repository/daily_report_models.go` | lock_timeout 先例 `:299`、HNSW 索引 `:335,387` |
| `backend-go/internal/models/topic_graph.go` | vector tag 维度 `:123`、索引双重声明 `:121-126` |
| `backend-go/internal/models/ai_models.go` | tag 未剥离 `:12-130`、jsonb 处理 `:137` |
| `backend-go/internal/models/semantic_label.go` | 索引双重声明 `:8,14,20`、命名误导 `:59` |
| `backend-go/internal/models/article.go` | 悬空字段 `:10` |
