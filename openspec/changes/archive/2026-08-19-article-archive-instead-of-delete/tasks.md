# Tasks: article-archive-instead-of-delete

## 1. 数据模型与 Migration

- [x] 1.1 `internal/models/article.go`：Article 增加 `Archived bool` 字段（`gorm:"default:false" json:"archived"`）
- [x] 1.2 `internal/platform/database/postgres_migrations.go`：新增 migration——`ALTER TABLE articles ADD COLUMN archived boolean NOT NULL DEFAULT false`（无回填、无索引）

## 2. CleanupOldArticles 重写（TDD：先写 2.1 测试再实现）

- [x] 2.1 测试先行：`internal/reader/service` 包内为 CleanupOldArticles 写单测，覆盖——超限归档且原文字段保留、favorite 免死、0/9999 跳过、归档幂等（已归档不进候选集）、count/候选仅统计 archived=false（窗口侵蚀防护）、衍生数据清除（tags 边删 + behaviors 删 + search_vector 置 NULL + CleanupOrphanedTags 调用）
- [x] 2.2 实现：DELETE 路径改为 UPDATE（`archived=true, search_vector=NULL`），DELETE `reading_behaviors`、DELETE `article_topic_tags` + 复用 `CleanupOrphanedTags`；count 与排序查询加 `WHERE archived = false`；favorite/上限跳过/排序语义保持不变

## 3. 查询过滤（按 design D3 清单）

- [x] 3.1 `article_handler.go GetArticles`：base query 加 `archived = false` 默认过滤；支持 `archived=true` 查询参数显式返回归档集
- [x] 3.2 统计口径：`GetArticlesStats`（handler）与 `GetArticleStats`（repository）加 `archived = false`
- [x] 3.3 feed 统计：`feed_handler.go` feed 列表附带统计（:92 附近）与单 feed 统计（:185 附近）加 `archived = false`
- [x] 3.4 一致性对账：grep 全仓 articles 查询点，确认 D3 清单外无遗漏的"列表/统计型"查询（按 ID 单点、队列、dedupe、日报反查豁免不过滤）

## 4. 前端文案

- [x] 4.1 feed 设置最大文章数控件辅助说明：表述改为"超出后归档保留原文"，不出现"删除"措辞（`front/` 内定位具体组件）

## 5. 测试

- [x] 5.1 `go test ./internal/reader/...`（service + handler 相关包）通过，含 2.1 新增用例
- [ ] 5.2 手动冒烟（可选，Docker 环境）：向测试 feed 灌超量文章触发刷新 → psql 确认 `archived=true` 行存在、`search_vector IS NULL`、tags/behaviors 已清、日报引用反查可读

## 6. 文档

<!-- doc-impact: flow api database -->
<!-- doc-impact-excuse: architecture=app/runtime.go 属并行 change ai-health-reprobe 的脏工作区改动（本 change 未动 app/）; configuration=configs/config.yaml 属并行 change data-enrichment-structural-depth 的博查配置（本 change 未动 config） -->

- [x] 6.1 `docs/reference/flow/`：reader/feeds 文章链路补"超限归档"行为与业务约束（archived 语义、窗口计数排除归档、dedupe 含归档 title）
- [x] 6.2 `docs/reference/flow/`：daily-report 线索链路补"引用文章永久可读（含归档豁免）"约束
- [x] 6.3 `docs/reference/database/`：articles 表补 `archived` 字段说明与保留策略

## 7. 验证

- [x] 7.1 `cd backend-go && go test ./internal/reader/...` → 全部 PASS
- [x] 7.2 `cd backend-go && golangci-lint run ./... && go vet ./... && go build ./...` → 0 issues、编译通过
- [x] 7.3 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"` → 无类型错误
- [x] 7.4 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit"` → 全部 PASS（51 文件 518 用例）
- [x] 7.5 `grep -rn "删除" front/app --include="*.vue" | grep -i "文章\|article\|max"` → 唯一命中为 EditFeedDialog 的 feed 删除确认文案（本 change 未改该行为，非 max_articles 控件文案）；最大文章数控件无删除措辞
- [x] 7.6 `openspec validate article-archive-instead-of-delete` → 通过
