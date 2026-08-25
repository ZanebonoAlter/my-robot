<!-- constraint-domains: semantic-board, data-enrichment -->

## Why

主线 2 个预存测试失败（retire-narrative-legacy 期间 stash 验证坐实与本档无关）：

1. `TestTopicTagCreate_StatusDefaultsToActive`（tagmanagement/repository）：新建 TopicTag 不设 Status 落库为 `""`——`getBoardArticles` 按 `status='active'` 过滤，`""` 使 tag 及其文章不可见。
2. `TestAuxiliaryLabelService*`（tagmanagement/service/auxlabel ×3）：创建 SemanticLabel 报 `null value in column "context_layers" violates not-null constraint`。

两处根因均出自 `a0b03bdc`（db-ddl-hardening-low-risk「tag 剥离」切片）：

- **根因 A（status）**：该 commit 剥掉 `TopicTag.Status` 的 `default:active` tag（`tag_merge_suggestions.Status` 的 `default:pending` 同批剥掉），赌 DB 层 DEFAULT 兜底。但 GORM 对无 `default:` tag 的零值字段会**显式 INSERT `""`**，DB DEFAULT 无机会生效——该测试注释与迁移 `20260724_0001` 描述（"producing empty/NULL status rows that broke strict status filters"）都记录过此坑，病根只有 default tag 能治。主路径 `tagger.go:164` 显式设 Status 故线上无症状，但回归守卫失效中。
- **根因 B（context_layers）**：`SemanticLabel.ContextLayers` 的 tag 写作 `default:'[\"week\",\"month\",\"year\",\"all\"]'`——Go 反引号字符串中 `\"` 是字面反斜杠+引号，GORM 解析 default 串失败，插入 nil slice 时显式写 SQL NULL，撞上 NOT NULL（23502）。对照组 `Aliases`（`default:'[]'` 无嵌套引号）一直正常。
- **附带（constrain 架空）**：`20260723_0001` 的 `constrain` helper 在 defaultLit 非空时无条件调 `ensureNotNullDefault`（SET NOT NULL），cols 表中 `notNull: false` 声明形同虚设——context_layers/aliases 两列被意外 NOT NULL。

## What Changes

- `models/topic_graph.go`：还 `TopicTag.Status` 的 `default:active`、`TagMergeSuggestion.Status` 的 `default:pending` GORM tag（其余剥除保持不变——仅恢复「零值显式 INSERT 会破坏语义」的状态机字段）
- `models/semantic_label.go`：`ContextLayers` tag 转义修复——`default:'[\"week\",…]'` → `default:'["week","month","year","all"]'`（Go 反引号内双引号无需转义）
- `postgres_migrations.go`：`20260723_0001` 的 `constrain` helper 尊重 `notNull` 参数——defaultLit 非空时仅在 `notNull=true` 才调 `ensureNotNullDefault`（修声明架空）
- 无新迁移：DB 层 DEFAULT/NOT NULL 由 20260723/20260724_0001 已落地，本档修 Go 侧写入行为与迁移代码 bug；存量库已 NOT NULL 的列不回滚（无害——tag 修复后不再有 NULL 写入），新库重放起 cols 声明如实生效

## Capabilities

### New Capabilities

（无）

### Modified Capabilities

- `tagging-domain` — 新增 Requirement：状态机字段的 GORM default tag 保留（TopicTag.Status / TagMergeSuggestion.Status）
- `db-migration-safety` — 幂等 Requirement 补 Scenario：`constrain` helper 须尊重 notNull 参数

### Removed Capabilities

（无）

## Impact

- **后端**：`internal/models/`（topic_graph.go、semantic_label.go）、`internal/platform/database/postgres_migrations.go`（历史迁移 helper 修复）
- **行为恢复**：新建 TopicTag/TagMergeSuggestion 未显式设 status 时默认 active/pending（回归守卫转绿）；新建 SemanticLabel 未显式设 ContextLayers 时由 DB DEFAULT 填充
- **部署**：无迁移、无操作、无数据变化；纯代码修复，重启即生效
- **风险边界**：不动既有剥除决策的其余部分（a0b03bdc 的 DDL 治理方向保留）；不动 DB 存量约束
