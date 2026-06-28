## 1. 后端数据模型与存储（贴合度列落库）

- [x] 1.1 在 `internal/topicgraph/repository/daily_report_models.go` 的 `DailyReportThread` 结构体新增两字段：`Embedding string`（`gorm:"type:vector" json:"-"`，绝不外泄）、`FitDistance float64`（`json:"fit_distance,omitempty"`）。验收：AutoMigrate 后 `daily_report_threads` 表含 `embedding`/`fit_distance` 两列
- [x] 1.2 确认 detail/timeline/lifeline/topic-lifeline 四接口通过 section.threads GORM Preload 自动带出 `fit_distance`（Preload 加载全列）；验证 `embedding` 因 `json:"-"` 不出现在 API 响应。验收：detail 接口返回 thread 含 fit_distance、不含 embedding

## 2. 后端贴合度计算（Step6 后同步批量算）

- [x] 2.1 在 `internal/topicgraph/service/daily_report_orchestrator.go` 的 Step6 section 装配之后、Step7 `MergeSimilarSections` 之前，新增贴合度计算：批量收集当批 thread 标题（`sections[i].Threads[].Title`），调 `airouter.Embed`（`CapabilityEmbedding`，与 section 标题 embedding 同 provider）算向量
- [x] 2.2 计算每个 thread 标题向量与所属 section 标题向量的余弦距离（复用 `repository` 现有 `cosineDistance` 或 service 层等价实现），写入 `thread.Embedding`（`FloatsToPgVector`）与 `thread.FitDistance`
- [x] 2.3 embedding 调用失败 SHALL 非致命：记 warn 日志，该批 thread 的 Embedding/FitDistance 留零值，生成继续。验收：mock embedding 失败时日报生成不中断
- [x] 2.4 **TDD**：写 orchestrator 贴合度计算的单测（`daily_report_orchestrator_test.go`），覆盖：贴合 thread 算小距离、跑题 thread 算大距离（用真实案例标题）、embedding 失败留空不中断、thread↔section 配对在 Step7 前固定。先红后绿。验收：`go test ./internal/topicgraph/service` PASS

## 3. 前端工具函数（TDD 基石 · 纯函数先行）

- [x] 3.1 **现网阈值标定**（落库后）：查现网 thread.fit_distance 分布找自然断点（贴合聚集 vs 离群聚集分界），用 section 800 真实案例（thread 1817 机器人 vs 同 section 其他 OpenAI thread）做真阳验证，据此确定 `THREAD_FIT_DEMOTE_THRESHOLD` 最终值（候选默认 0.20）
- [x] 3.2 新建 `front/app/utils/threadFit.ts`，导出常量 `THREAD_FIT_DEMOTE_THRESHOLD`（标定后的值）与纯函数：`isThreadFitDemoted(fitDistance?: number)` → boolean（缺省/零值 → false，超阈值 → true）、`threadFitLabel(fitDistance?: number)` → "贴合"/"可能跑题"/"无贴合信号"。验收：纯函数无副作用、无 DOM 依赖
- [x] 3.3 **TDD 红**：先写 `front/app/utils/threadFit.test.ts`，覆盖：贴合值（低距离）、离群值（超阈值）、阈值边界值（0.20 本身不降级）、fit_distance 缺失/零值（历史 thread 不降级）。验收：`pnpm test:unit threadFit` 全红
- [x] 3.4 **TDD 绿**：实现函数使测试全通过。验收：`pnpm test:unit threadFit` 全绿

## 4. 前端数据契约

- [x] 4.1 在 `front/app/api/dailyReports.ts` 的 `DailyReportThread` 类型新增可选字段 `fit_distance?: number`（embedding 不声明，后端不外泄）。验收：typecheck 通过，消费点无破坏

## 5. 前端 thread 软降级渲染

- [x] 5.1 在 `DailyReportTopicSection.vue` 的 `.drm-section-card__threads` thread 渲染处（`<article class="drm-thread">`）接入贴合度：对 `isThreadFitDemoted(thread.fit_distance)` 为真的 thread 加降级样式（灰 token + **左对齐离群标记图标**，置于标题文字前，带 aria-label）。**【设计 A 修订 · 2026-06-27】** 初版图标埋在右侧 meta「根本看不见」，改为标题左侧；同时取消「默认折叠」（跑题 thread 行保持可见）。
- [x] 5.2 section 内有 ≥1 离群 thread 时，在 thread 列表底部渲染状态说明行「可能跑题的线索 N 条」（纯 `<p>` 说明，**不可点**）。**【设计 A 修订 · 2026-06-27】** 初版做成可点 button 批量展开关联文章，但「展开文章对提示跑题无意义」，改为纯状态说明，删除 toggleAllDemoted。
- [x] 5.3 thread hover/展开时（探究区）展示贴合度数值（`fitDistance.toFixed(2)`）与中文标签（`threadFitLabel`），正文 thread 标题不出现任何数字。验收：正文极轻、分数仅进探究区
- [x] 5.4 历史 thread（无 fit_distance）按正常 thread 渲染，不降级、不折叠、不报错。验收：缺失字段的 thread 样式与正常 thread 一致
- [x] 5.5 **TDD**：写/扩 `DailyReportTopicSection` 或 thread 渲染相关单测，覆盖：离群 thread 灰显折叠、贴合 thread 正常、阈值边界、提示行 N 计数、探究行贴合度数值、历史 thread 不降级。先红后绿

## 6. 架构体检（§7 强制，每子任务后）

- [x] 6.1 codegraph 代码图：确认 `isThreadFitDemoted`/`threadFitLabel` 调用面（thread 渲染处）、贴合度字段消费点无遗漏、`DailyReportThread` 类型变更未漏 API 调用点。验收：`codegraph impact isThreadFitDemoted` 命中预期
- [x] 6.2 架构合理性：thread 软降级样式用主题语义 token 派生（灰 token），跟随双主题；新 utils 落在共享层（`app/utils/`）与 `topicAnchor.ts`/`matchQuality.ts` 同层；embedding 字段绝不外泄（json:"-"）。验收：无新增 lint 警告、无循环依赖

## 7. 测试（§5.2 前端双层 + 后端 targeted）

- [x] 7.1 后端 targeted：`cd backend-go && go test ./internal/topicgraph/service ./internal/topicgraph/repository` → PASS（只跑本次影响包，按 AGENTS.md §测试规范）
- [x] 7.2 后端单测：`cd backend-go && go test ./internal/topicgraph/...` → PASS（无既有用例回归）
- [x] 7.3 前端工具函数单测：`cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit threadFit"` → PASS（test:unit 必须 Windows cmd）
- [x] 7.4 前端组件单测：`cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit"` → PASS（全量回归）

## 8. 文档（§9 产出物 / §12 流转）

- [x] 8.1 更新 `docs/reference/flow/daily-report.md`：§0 概念字典 thread 行补 fit_distance 字段说明；两套匹配血缘表新增 System 3（thread↔section）一行；§2 新增「Thread 贴合度信号」小节（B1 治理点 + 伞形话题为何不用紧凑性剔除的结论，衔接本次 design D1）
- [x] 8.2 更新 `docs/reference/architecture/map.md` 索引：日报域新增"thread 贴合度可观测"入口（与 System 1/2 并列）
- [x] 8.3 检查 `docs/reference/standard/` 前端规范是否需补充"thread 级降级样式/共享 utils 落位"约定（仅在确有新约定时）

## 9. 验证（§11.2 归档门禁 · 每条可执行 + 期望结果）

- [x] 9.1 `cd backend-go && go vet ./internal/topicgraph/...` → 零 error
- [x] 9.2 `cd backend-go && go build ./...` → 构建成功
- [x] 9.3 `cd backend-go && go test ./internal/topicgraph/service ./internal/topicgraph/repository` → PASS
- [x] 9.4 `cd front && pnpm lint` → 零 error（WSL 可跑 lint）
- [x] 9.5 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"` → 零 error（typecheck 必须 Windows cmd）
- [x] 9.6 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"` → 构建成功（build 必须 Windows cmd）
- [x] 9.7 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit"` → 全绿
- [x] 9.8 `grep -rn "fit_distance" front/app --include=*.vue --include=*.ts | grep -v "\.test\.ts"` → 命中本次新增消费点（api 类型声明 + thread 渲染降级判断），证明 System 3 字段已接入
- [x] 9.9 `grep -rn "embedding" front/app/api/dailyReports.ts` → 零命中（embedding 字段绝不外泄到前端契约）
- [x] 9.10 `grep -rn "THREAD_FIT_DEMOTE_THRESHOLD\|isThreadFitDemoted" front/app --include=*.vue --include=*.ts | grep -v "\.test\.ts"` → 命中 utils 定义 + thread 渲染消费点
- [x] 9.11 `bash scripts/check-standards.sh` → 零失败（L1 规范验收，归档前自检）
