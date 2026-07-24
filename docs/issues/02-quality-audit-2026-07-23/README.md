# 质量审计报告总索引（2026-07-23 + 2026-07-24 DDL 补充）

> **Status:** High 全部修复 + 技术债批量治理完成（model tag Top3 / 前端机械项 / handler 轻量下沉；高风险大重构按用户决策跳过）。**2026-07-24 新增 DDL 专项审计（05），为纯审计未执行，按 ROI 分层给执行建议。**
> **Scope:** 全项目对照 `docs/reference/` 规范体系，检查文档遗漏 + 代码设计规范合理性 + **数据库 DDL/迁移设计（07-24 补）**
> **审计方法**: 3 路并行子审计（后端代码 / 前端代码 / 文档完整性） + 主线程逐项验证关键发现；**07-24 追加 DDL/迁移 + 模型层双路并行子审计**

## 整改进度（2026-07-23 更新）

### High 问题（全部修复）

| 问题 | 状态 | 修复说明 |
| ---- | ---- | -------- |
| H1 IDOR 漏洞 | ✅ **已修复** | TDD 写 3 个复现测试 → 修 `updateReviewDeviation`/`applyReview`/`sedimentQA` 加 topicId 归属校验 |
| H5b docs/README 死链 | ✅ **已修复** | 删除 summaries.md 死链行（端点已废弃，见 ai-summary.md 说明） |
| H6 watch-keyword 前置声明 | ✅ **已修复** | 改为符合现状：表+spec 已就绪，change 未归档但不阻塞 |
| H2 causal-analysis-agent 文档 | ✅ **已修复** | 补 4 处文档 + 修 tasks.md doc-impact 声明（加 api），doc-impact verify 3 FAIL→0 |
| H3 API 文档缺 QA 端点 | ✅ **已修复** | api/dataenrichment.md 补 QA/sediment 4 端点 |
| H4 DB 文档缺 topic_enrichment_qa 表 | ✅ **已修复** | DATABASE_FIELDS.md 补 §10.6 表 + sectors 语义更新 + 计数 43→44 |

### 技术债治理（Medium/Low，按用户决策执行）

| 治理项 | 状态 | 说明 |
| ------ | ---- | ---- |
| **M2 model tag Top3**（106/154 处） | ✅ 已完成 | 迁移 20260723_0001 兜底 + 约束断言测试 + 删 tag + check-standards H 段守门。3 个 jsonb 字段保留 default（serializer:json 必需例外） |
| M6 ArticleTagList 4 色硬编码 | ✅ 已修复 | 替换为 `var(--color-tag-*)` token |
| M6 BTB statusColorMap 5 色 | ✅ 已修复 | 新增 `--color-thread-status-*` 双主题 token |
| M8 URLSearchParams 手写 3 处 | ✅ 已修复 | 改用 `buildQueryString()` |
| M1 topicgraph handler 越层（2处） | ✅ 已修复 | `CountSectionsByTopic` 下沉 |
| M1 reader content_completion 越层（2处） | ✅ 已修复 | 下沉到 repository |
| M7 BoardThreadBrowser 拆分 | ⏳ **用户决策跳过** | 2456 行大重构，高风险，需测试驱动专项 |
| M1 admin/reader handler 越层（大头） | ⏳ **用户决策跳过** | admin「假三层」补建半个 service 层，工作量大 |
| M2 model tag 长尾（48 处） | ⏳ **用户决策跳过** | job_queue/feed/narrative 等，留给未来 |
| M10 AppButton 58 文件迁移 | ⏳ **用户决策跳过** | 量大需人工逐个判别 |

> **门禁结果**：`check-standards.sh` 从 68 PASS / 2 FAIL → **72 PASS / 0 FAIL 全绿**（新增 H 段 3 个 OK）。后端 lint/vet/build 全过；测试仅 develop 预先存在的 2 包失败（tagmanagement seed duplicate-key，与本次无关）。前端 lint 0 errors + typecheck pass + test:unit 360 passed。

### 评级变化

| 维度 | 审计评级 | 治理后 |
| ---- | -------- | ------ |
| 后端代码设计 | B− | **B**（model tag Top3 治理 + 4 处 handler 下沉 + 守门） |
| 前端代码设计 | B | **B**（机械项已清，大重构未做维持 B） |
| 文档完整性 | B | **B+**（High 全修 + doc-impact 全绿） |
| **数据库 DDL/迁移**（07-24 补） | 迁移层 **B−** / 模型层 **B+** | 迁移层 **B**（07-24 修复破坏性迁移守卫 D-High-7 + SET NOT NULL 幂等 D-Med-5 + tag 剥离 M-High-3 + 假注释 D-High-6；架构级 D-High-1/3/4/5 留 `db-ddl-hardening-architecture` change）/ 模型层 **A−**（tag 长尾 13 文件待收口） |

## 一、审计背景与依据

对照项目规范的三层文档体系与红线，逐一核验：

| 规范层 | 权威文件 | 核验内容 |
| ------ | -------- | -------- |
| 流程层 | `docs/reference/flow/*.md`（五段式活文档） | 8 篇齐全性 + 五段式齐全 + 与代码一致性 |
| 规范层 | `docs/reference/standard/{backend,frontend}/*.md` | 防孤立引用 + 与代码红线一致性 |
| 架构层 | `docs/reference/architecture/*.md` | map.md 索引 + 分层边界 |
| 执行层 | `docs/reference/开发执行规范.md` | openspec change 编排纪律 + 归档门禁 |

**客观校验工具**：`bash scripts/check-standards.sh` 实测结果（A–G 七段），`bash scripts/doc-impact.sh verify`。

## 二、总体评分

| 维度 | 评级 | 一句话结论 |
| ---- | ---- | ---- |
| 文档完整性 | **B** | 结构骨架健康（flow 五段式 100%、standard 防孤立 100%），但进行中 change 的文档零跟进 |
| 后端代码设计 | **B−** | 分层骨架扎实、并发处理专业，但 handler 越层访问 DB 是系统性问题 + 1 处 IDOR 漏洞 |
| 前端代码设计 | **B** | 架构红线守得住，问题集中在 tags 域巨型组件 + 主题硬编码回潮 |
| **数据库 DDL/迁移**（07-24 补） | 迁移层 **B−** / 模型层 **B+** | 幂等性与文档化优秀，但「事务包一切阻断 CONCURRENTLY」「外键基本缺位」「向量维度三方矛盾」「>2000 维无索引」「1 个迁移生产会丢数据」是 5 个高危项 |

**综合结论**：项目工程化程度高，规范体系本身设计精良且被工具强制（check-standards.sh 自动兜底）。当前问题**不是结构性的**，而是**执行层面的局部回潮**——尤其集中在「代码先行、文档后补」的进行中 change（causal-analysis-agent）上。

## 三、High 级问题清单（建议优先处理）

> 完整详情见各分报告。此处按「优先级 + 改动成本」排序，便于排期。

### 🔴 安全类（必须修）

| # | 问题 | 位置 | 证据 | 分报告 |
| - | ---- | ---- | ---- | ------ |
| H1 | **IDOR 漏洞**：review 偏差/应用 handler 只校验 `:id` 不校验 `:topicId` 归属 | `dataenrichment/handler/handler.go:676-715`（`updateReviewDeviation`/`applyReview`） | 路由带 `:topicId`，handler 只 `parseIDParam(c,"id")` 后直接 `UpdateTopicEnrichmentReview/ApplyTopicEnrichmentReview`，从不读 topicId；同文件 `getResult`(:401)/`triggerDebate`(:482) 却正确做了校验 | [后端](./02-backend-code-design.md) |

### 🔴 文档一致性（归档门禁阻塞项）

| # | 问题 | 位置 | 证据 | 分报告 |
| - | ---- | ---- | ---- | ------ |
| H2 | **causal-analysis-agent 文档零跟进**（doc-impact verify 3 FAIL） | 5 个新代码文件（`qa_agent.go`/`exploration.go`/`web_search.go`/`lens_source.go`/`board_listers_impl.go`） | `doc-impact.sh verify` 实测 3 FAIL：api 未声明、DATABASE_FIELDS 未更新、ai-logging.md 未更新 | [文档](./03-doc-completeness.md) |
| H3 | **API 文档缺 4 个 QA 端点** | `docs/reference/api/dataenrichment.md` | 代码已注册 `POST/GET /results/:id/qa`、`POST /qa/:id/sediment`，文档 grep 全空 | [文档](./03-doc-completeness.md) |
| H4 | **DB 文档缺 `topic_enrichment_qa` 表** | `docs/reference/database/DATABASE_FIELDS.md` | 迁移已建表 + `sedimented` 列（`postgres_migrations.go:1113`），文档 §10 无 10.6 | [文档](./03-doc-completeness.md) |
| H5 | **docs/README.md 死链**（check-standards G 段 FAIL） | `docs/README.md:47` → `reference/api/summaries.md` | `ls` 文件不存在；check-standards.sh G 段实测 FAIL | [文档](./03-doc-completeness.md) |

### 🔴 流程纪律

| # | 问题 | 位置 | 证据 |
| - | ---- | ---- | ---- |
| H6 | **watch-keyword 前置依赖声明错误** | `openspec/changes/watch-keyword-and-quickadd/tasks.md:5` | 声称「前置 topic-watchlist-observability 已归档」，但该 change 仍在 active 区（32/49） |

## 四、Medium 级问题清单（系统性技术债）

### 后端

| # | 问题 | 范围 | 证据 |
| - | ---- | ---- | ---- |
| M1 | **handler 越层访问 DB**（系统性，非个别） | reader/article_handler(30+处)、admin/ai_handler+preferences_handler、topicgraph/daily_report_handler、tagmanagement 多 handler | 违反 package-layout.md「Handler 不直接访问 DB」红线；admin 域几乎无 service 层 |
| M2 | **models 约 154 处 `not null`/`default` GORM tag** | `internal/models/*.go` 全包 | `grep` 实测 154 处；违反 code-style.md「model tag 不写 not null/default」红线，且有「ai-call-logging-schema」事故教训 |
| M3 | **dataenrichment 根包布局违规** | 9 个非 routes.go/wire.go 文件 | `board_config_impl.go`/`board_listers_impl.go` 是 repository 实现却放根包，违反 package-layout.md |
| M4 | **admin 调度器 9 个 job 零单测** | `admin/scheduler/job_*.go` | 架构文档已自认，写库任务无回归保护 |
| M5 | **3 个超大文件** | `board_crud_handler.go`(1285行)、`semantic_board_upgrade.go`(1196行)、`daily_report_repository.go`(994行)、`orchestrator.go`(1243行) | 超 800 行阈值 |

### 前端

| # | 问题 | 范围 | 证据 |
| - | ---- | ---- | ---- |
| M6 | **主题硬编码回潮** | `ArticleTagList.vue`(4 处)、`BoardThreadBrowser.vue`(statusColorMap 5 色) | 重复造 Layer 2 已有 token；暗色模式失效 |
| M7 | **`BoardThreadBrowser.vue` 2456 行无 composable** | tags 域 | 全项目最大文件，41 函数/27 computed/4 视图模式，未抽任何 composable |
| M8 | **7 处 `as unknown as` 类型逃逸** | stores/api.ts、articles.ts、useArticlePagination 等 | `utils/api-helpers.ts` 已提供 `unwrapResponse<T>()` 专门替代 |
| M9 | **核心 store/composable 无单测** | useEventStream(WS 基建)、stores/articles、useTagMergePreview 等 | testing.md 要求核心 composable 有单测 |
| M10 | **AppButton 统一组件被大面积绕过** | ~50 个 .vue 用原生 `<button>` | theming.md「按钮必须复用 AppButton」 |

## 五、Low 级问题（择机处理）

> 详见各分报告。代表性：多处 best-effort 吞错无日志（`_ = ...`）、`components/dialog/` 混放各域业务对话框、`preferences.ts` computed 内原地 sort、otel-tracing-completion 停滞 18 天等。

## 六、正面发现（值得保持的工程亮点）

审计并非只挑毛病，以下实践**超出平均水平**，应保持：

- **`tool_registry.go` 的 ETF 缓存主动规避 `sync.Once` 永久缓存失败陷阱**（mutex + loaded flag + 失败可重试，注释说明）
- **`tag_merge_preview_handler.go` 合并事务用 `FOR UPDATE` 双锁 + 注释记录死锁事故与修复**（成熟代码标志）
- **前端 API 归一化彻底——零组件内 `fetch`/`$fetch`**，HTTP 边界全收敛在 `api/client.ts`
- **Store 派生架构规范**——apiStore 主源、feedsStore 纯派生、无循环依赖
- **写操作全 API 持久化 + 乐观更新 + 回滚**（articles.ts）
- **`<script setup>` 100% 覆盖**（89/89 组件，无 Options API 残留）
- **规范体系本身设计精良**——`check-standards.sh` 自动兜底（实测 68 通过 / 2 失败，失败项正是本报告 H2/H5）
- **doc-impact 双源注入机制**（flow 业务约束 what + standard 代码 how）

## 七、整改建议（按优先级排序）

### 第一优先（安全 + 归档门禁阻塞，本周内）

1. **修 IDOR（H1）**：在 `updateReviewDeviation`/`applyReview` 加 `review.PersistentTopicID != topicID { 404 }`，与 `getResult` 对齐。**成本：~15 分钟，1 个文件。**
2. **补 causal-analysis-agent 文档（H2/H3/H4）**：按 `doc-impact.sh verify` 3 项 FAIL 补 api/dataenrichment.md（4 端点）+ DATABASE_FIELDS.md（§10.6 表）+ standard/backend/ai-logging.md + flow/data-enrichment.md。这是该 change 归档的硬阻塞。
3. **修 docs/README.md 死链（H5）**：删除第 47 行 summaries.md 引用。**成本：1 行。**

### 第二优先（系统性技术债，下个迭代）

4. **handler 越层访问 DB 治理（M1）**：从最严重的 admin 域入手，补 `admin/service` 层。建议作为独立 change `admin-service-layer-extraction`。
5. **models tag 治理（M2）**：批量移除 154 处 `not null`/`default`，DB 约束收敛到 `postgres_migrations.go`。建议分 domain 渐进迁移（先动新增的 dataenrichment 表）。
6. **前端 BoardThreadBrowser 拆分（M7）**：按视图抽 5 个 composable，是前端最大单点债务。
7. **补 admin 调度器单测（M4）**：优先 firecrawl/content_completion/tag_quality_score 写库任务。

### 第三优先（执行规范回潮）

8. 前端主题硬编码治理（M6）、`as unknown as` → unwrapResponse（M8）、补核心 composable 单测（M9）。
9. 流程纪律：修正 watch-keyword 前置声明（H6）、处置 otel-tracing 停滞。

## 八、分报告索引

| 报告 | 内容 | 评级 |
| ---- | ---- | ---- |
| [02-backend-code-design.md](./02-backend-code-design.md) | 后端 Go 各域代码设计规范审计 | **B−** |
| [03-doc-completeness.md](./03-doc-completeness.md) | 文档完整性与一致性审计 | **B** |
| [04-frontend-code-design.md](./04-frontend-code-design.md) | 前端 Nuxt/Vue 代码设计规范审计 | **B** |
| [05-db-ddl.md](./05-db-ddl.md) | **数据库 DDL/迁移/模型层专项审计（07-24 补，纯审计未执行）** | 迁移层 **B−** / 模型层 **B+** |

## 九、客观校验证据

以下命令实测通过，可作为整改后的回归验证基线：

```bash
# 归档门禁（整改前 68 PASS / 2 FAIL → High 修复后 69/0 → 技术债治理后 72 PASS / 0 FAIL 全绿，含新增 H 段守门）
bash scripts/check-standards.sh

# model tag 约束完整性回归（防约束真空闸门，需 Docker testcontainer）
cd backend-go && go test ./internal/platform/database/ -run TestModelTagConstraints_MaterializedInDB

# IDOR 修复回归（H1）
cd backend-go && go test ./internal/dataenrichment/handler/ -run 'IDOR|Review|Sediment'

# causal-analysis-agent 归档前置（声明 flow/database/standard/api 4 域）
bash scripts/doc-impact.sh verify openspec/changes/causal-analysis-agent/

# model tag 守门基线（Top3 文件 not null 应为 0）
grep -cE 'gorm:"[^"]*not null' backend-go/internal/models/{ai_models,topic_graph,semantic_label}.go  # 各为 0
```

---

## Comments

（2026-07-23 记录）本次审计由用户发起，按项目规范要求出具各功能代码块质量分析。所有 High/Medium 发现均经主线程二次验证（grep/ls/脚本实测），非子审计单方面结论。整改建议已按「安全→门禁阻塞→系统性债→执行回潮」分层排期，可逐项落地。
