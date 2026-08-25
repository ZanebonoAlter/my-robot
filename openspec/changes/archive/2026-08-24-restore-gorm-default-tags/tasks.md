# Tasks — restore-gorm-default-tags

## 1. 修复

<!-- doc-impact: database -->
<!-- doc-impact-excuse: api=工作树中 tagmanagement/handler 等为 retire-narrative-legacy change 脏文件，非本 change 改动; flow=工作树中 tagmanagement/service 与 front/app/features 为 retire-narrative-legacy change 脏文件，非本 change 改动; standard=工作树中 docs/reference/standard/shared/test-design.md 为 test-case-design-standard change 脏文件，非本 change 改动 -->

- [x] 1.1 `backend-go/internal/models/topic_graph.go`：`TopicTag.Status` 恢复 `default:active`、`TagMergeSuggestion.Status` 恢复 `default:pending`（仅这两个状态机字段，其余剥除保持）→ `go build ./...` 绿
- [x] 1.2 `backend-go/internal/models/semantic_label.go`：`ContextLayers` tag 转义修复 `default:'[\"week\",\"month\",\"year\",\"all\"]'` → `default:'["week","month","year","all"]'` → `go build ./...` 绿
- [x] 1.3 `backend-go/internal/platform/database/postgres_migrations.go`：`20260723_0001` 的 `constrain` helper 修复——defaultLit 非空时仅 `notNull=true` 才调 `ensureNotNullDefault`（修 notNull 参数架空，见 design D3）

## 2. 测试

- 影响包：`internal/tagmanagement/repository`（status 测试）、`internal/tagmanagement/service/auxlabel`（×3）、`internal/platform/database`（迁移重放）、`internal/models`
- [x] 2.1 `go test ./internal/tagmanagement/repository/ -run TestTopicTagCreate_StatusDefaultsToActive`（Docker DB）→ 转绿
- [x] 2.2 `go test ./internal/tagmanagement/service/auxlabel/`（Docker DB）→ 全绿（3 个 23502 失败转绿）
- [x] 2.3 `go test ./internal/platform/database/ ./internal/models/...` → 全绿（constrain 修复后 golden schema 重放正常）
- [x] 2.4 `golangci-lint run ./... && go vet ./... && go build ./...` → 全绿

## 3. 文档

- [x] 3.0 `docs/reference/database/DATABASE_FIELDS.md` 核对结论：:337 `topic_tags.status` `NOT NULL DEFAULT 'active'`、:483 `context_layers` `DEFAULT '["week","month","year","all"]'`（未标 NOT NULL）——文档描述的即修复后行为（文档写的是设计本意，代码 constrain bug 才是偏差方），无需修改；存量库 context_layers 已 NOT NULL 不回滚（无害，见 design D3）

## 4. 验证

| Scenario | 测试文件 |
| --- | --- |
| 新建 TopicTag 未显式设 status 默认 active | backend-go/internal/tagmanagement/repository/topic_tag_default_test.go |
| 新建 SemanticLabel 未显式设 context_layers 由 DB DEFAULT 填充 | backend-go/internal/tagmanagement/service/auxlabel/auxiliary_label_service_test.go |
| jsonb default tag 转义正确性 | 人工（struct tag 走查 semantic_label.go:27，tag 已剥除改 BeforeCreate，源注释留据） |
| constrain helper 尊重 notNull=false 声明 | 人工（golden schema 重放走查 postgres_migrations.go constrain + db_unit_test.go 迁移链） |
| SET NOT NULL 迁移重复执行不报错 | backend-go/internal/platform/database/db_unit_test.go |
| 重复执行 InitDB 不产生迁移错误 | backend-go/internal/platform/database/db_unit_test.go |

- [x] 3.1 `bash scripts/scenario-trace.sh openspec/changes/restore-gorm-default-tags` → 退出码 0
- [x] 3.2 归档门禁：`bash scripts/doc-impact.sh verify openspec/changes/restore-gorm-default-tags` + `bash scripts/check-standards.sh` → 通过
- [x] 3.3 部署提示（完工汇报必含）：无迁移无操作，重启即生效；存量库 context_layers/aliases 列保持已 NOT NULL 状态不回滚（无害）；新建 TopicTag 未显式设 status 恢复默认 active
