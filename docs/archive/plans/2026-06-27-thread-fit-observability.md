# Thread 贴合度可观测 (thread-fit-observability) 实现计划

> **REQUIRED SUB-SKILL:** 用 subagent-driven-development 派子线程执行；controller（本会话）负责进度把控、两阶段 review、门禁。

## Goal

给日报 thread（事件粒度）补 System 3 observability 信号：thread 标题 ↔ 所属 section 标题的余弦贴合距离。离群 thread 前端软降级（灰显+折叠+标记），保信息不删除。

## Architecture

后端在日报生成管线 Step6（section embedding 就绪）后、Step7（MergeSimilarSections）前，批量 embed thread 标题、算与所属 section 标题向量的余弦距离，落库到 `daily_report_threads.fit_distance`。前端纯函数判定离群 → thread 行软降级 + 探究区贴合度数值。复用 observability 系列展示分层（正文极轻、分数进探究区）。

## Tech Stack

Go (Gin/GORM) + pgvector 后端；Nuxt 4 + Vue 3 + TS 前端。

## 用户决策（已拍板）

1. **分支**：沿用当前 `feature/quality-scoring-observability`（可观测系列统一分支），不开新分支、不用 worktree。
2. **阈值标定（Task 3.1）= 完整标定**：后端落地后重新生成 2026-06-26 日报，连库查真实 `fit_distance` 分布找自然断点，用 section 800（机器人 thread）案例做真阳验证，据此定 `THREAD_FIT_DEMOTE_THRESHOLD` 最终值（候选 0.20）。

## 关键事实（子线程须知的精确上下文）

### 后端注入点（已精确定位）

文件：`backend-go/internal/topicgraph/service/daily_report_orchestrator.go`

- **section embedding 生成块**：行 215-242（`embedTexts`/`embedIndices` → `airouter.NewRouter().Embed(..., airouter.CapabilityEmbedding)` → 写入 `sections[idx].Embedding`）
- **MergeSimilarSections 调用**：行 243
- **新代码注入点**：行 242 `}` 之后、行 243 之前

### ⚠️ 数据结构事实（决定实现写法）

此时 thread **不在** `sections[i].Threads`（该字段此时尚未填充），而在 `threadBatches [][]repository.DailyReportThread`，**`threadBatches[i]` 与 `sections[i]` 同索引配对**（见行 198-213 的 thread batch 构建循环）。

→ 贴合度计算遍历 `threadBatches[i]`，配对 `sections[i].Embedding`，写回 `threadBatches[i][k].Embedding`/`.FitDistance`。

### 复用点（不要重复造）

| 用途 | 位置 | 签名 |
|------|------|------|
| 余弦距离（pgvector string→string） | `service/daily_report_merge.go:48` | `cosineDistance(vec1, vec2 string) (float64, error)` — **同包，直接调用**；返回 1.0 + error 表示解析/维度问题 |
| float→pgvector 编码 | `repository/daily_report_models.go:188` | `FloatsToPgVector(v []float64) string` |
| 批量 embed 调用模式 | orchestrator 行 226-242 | `airouter.NewRouter().Embed(ctx, airouter.EmbeddingRequest{Input, Metadata}, airouter.CapabilityEmbedding)`，失败 `logging.Warnf` 非致命 |
| section embedding 字段 | `repository.DailyReportSection.Embedding string` (`gorm:"type:vector"`) | 照搬到 thread |

### TDD 可测性方案（核心，必做）

现有 orchestrator 的 `airouter.Embed` 是**硬编码直接调用**，无法 mock。为满足 Task 2.4 TDD 要求，**新增一个可测纯函数**：

```go
// daily_report_thread_fit.go（新文件，同 service 包）
type embedFunc func(ctx context.Context, req airouter.EmbeddingRequest, cap airouter.Capability) (*airouter.EmbeddingResult, error)

// 计算并原地填充所有 thread 的 Embedding + FitDistance；embedding 失败/section 无 embedding 时留零值，非致命。
func computeThreadFitDistances(ctx context.Context, sections []repository.DailyReportSection, threadBatches [][]repository.DailyReportThread, boardID uint, embed embedFunc) {
    // 1. 收集所有 (secIdx, threadIdx) 标题非空 且 对应 section 有 embedding 的 thread
    // 2. 批量 embed（operation: "daily_report_thread_embedding"）
    // 3. 失败 → Warnf + return（留零值）
    // 4. 成功 → FloatsToPgVector + cosineDistance(threadEmb, sections[secIdx].Embedding)，写回
    //    距离计算失败 → Warnf + 跳过该 thread（留零值）
}
```

orchestrator 在注入点调用：`computeThreadFitDistances(ctx, sections, threadBatches, boardID, airouter.NewRouter().Embed)`。测试传 mock embedFunc（贴合→返回近向量、跑题→远向量、失败→返回 error）。

> 备选（否决）：把 embed 做成 orchestrator struct 字段——侵入面大，且 section embedding 同样硬编码，没必要只改 thread。抽函数更聚焦、改动面最小。

### 前端注入点（已精确定位）

文件：`front/app/features/tags/components/daily-report/DailyReportTopicSection.vue`

- 两个 thread 渲染循环（结构几乎相同，都要改）：
  - **current**：行 181 `<article v-for="thread in section.threads" class="drm-thread">`
  - **history**：行 262 同款
- 状态：`expandedThreads = ref(new Set<string>())`（行 38）、`toggleThread(prefix, thread)`（行 69）、`threadKey(prefix, thread)`
- 软降级要加：`<article>` 上动态 class（离群→`drm-thread--demoted`）、默认折叠（离群 thread 不进 expandedThreads 初始态）、离群标记图标、section 底部提示行「另有 N 条可能跑题的线索」、thread 展开时探究行显示 `fitDistance.toFixed(2)` + `threadFitLabel`

### 前端复用先例（同层 utils）

`front/app/utils/topicAnchor.ts`（System 2 阈值 + tier + label 纯函数）、`matchQuality.ts`（theme token 派生）。新 `threadFit.ts` 照此结构，落 `app/utils/` 同层。

---

## 执行块（按顺序依赖组织；每块派一个 implementer 子线程 + 两阶段 review）

### Block A：后端数据层 + 贴合度计算（Task 1 + 2）— 必须先于标定

**TDD 核心，派 Deepseek V4 Pro。覆盖 tasks 1.1 / 1.2 / 2.1 / 2.2 / 2.3 / 2.4。**

#### A.1（Task 1.1）DailyReportThread 加字段

文件：`backend-go/internal/topicgraph/repository/daily_report_models.go`，`DailyReportThread` 结构体（行 131-142）。

新增两字段（放在 `RelatedArticleIDs` 后、`CreatedAt` 前，对齐 section 字段风格）：
```go
Embedding   string  `gorm:"type:vector" json:"-"`
FitDistance float64 `gorm:"default:0" json:"fit_distance,omitempty"`
```
> `json:"-"` 确保 embedding 绝不外泄；`fit_distance` 用 omitempty（零值=无信号，历史 thread 同样不返回数值，前端按正常渲染）。
> AutoMigrate 自动加列（`embedding` vector / `fit_distance` numeric），历史行 NULL/0。

#### A.2（Task 2.4 TDD 红→绿）贴合度计算

新文件：`backend-go/internal/topicgraph/service/daily_report_thread_fit.go` + `_test.go`

**红**：`daily_report_thread_fit_test.go` 覆盖：
1. 贴合 thread 算小距离（mock embed 返回与 section 向量近似的向量）
2. 跑题 thread 算大距离（mock embed 返回正交向量 → distance≈1.0；或对照 OpenAI section label vs 机器人 thread 标题）
3. embedding 调用失败 → 不 panic、不中断、Embedding/FitDistance 留零值
4. section 无 embedding（`sections[i].Embedding==""`）→ 该 section 的 threads 被跳过（留零值）
5. thread↔section 同索引配对正确（多 section 多 thread，验证 distance 写回正确的 thread）

**绿**：实现 `computeThreadFitDistances`（见上方 TDD 可测性方案伪代码）。

**验收**：`cd backend-go && go test ./internal/topicgraph/service` PASS。

#### A.3（Task 2.1/2.2/2.3）orchestrator 接线

`daily_report_orchestrator.go` 行 242 后注入：
```go
computeThreadFitDistances(ctx, sections, threadBatches, boardID, airouter.NewRouter().Embed)
```
（在 `MergeSimilarSections` 行 243 之前）

**验收**：`go build ./...` 成功；既有 orchestrator 测试不回归。

#### A.4（Task 1.2）验证四接口 fit_distance 透出 / embedding 不外泄

四接口（detail/timeline/lifeline/topic-lifeline）经 section.Threads GORM Preload 自动带全列。验证：grep 确认 Preload "Threads" 存在；`json:"-"` 生效无需改代码。

**验收**：repository 层无新代码（仅靠 Preload + tag），`go test ./internal/topicgraph/repository` PASS。

#### A 门禁（执行前确认、执行后验证）

- 执行前：`cd backend-go && go vet ./... && go build ./...`（基线绿）
- 执行后：`go test ./internal/topicgraph/service ./internal/topicgraph/repository` PASS（只跑影响包）+ `golangci-lint run ./internal/topicgraph/...`

---

### Block B：现网阈值标定（Task 3.1）— controller 主导，不派子线程

**前置**：Block A 合入、后端可跑、`syntopica-postgres` 健康（已确认）。

步骤：
1. 重新生成 2026-06-26 日报（让新代码算出 fit_distance）：
   - 起后端 `cd backend-go && go run cmd/server/main.go`
   - 触发对应 board 的日报重新生成（确认触发入口：API 或定时；标定阶段查）
2. 连库查分布：
   ```bash
   docker exec syntopica-postgres psql -U postgres -d syntopica -tAc \
     "SELECT fit_distance, title FROM daily_report_threads WHERE fit_distance > 0 ORDER BY fit_distance;"
   ```
   找「贴合聚集 vs 离群聚集」的自然断点。
3. 真阳验证：section 800 机器人 thread（标题含「机器人」「华腾百」）的 fit_distance 应显著大于同 section OpenAI thread。
4. 据断点定 `THREAD_FIT_DEMOTE_THRESHOLD` 最终值（候选 0.20），写入 Block C 的 `threadFit.ts`。

**产出**：阈值数值 + 标定依据（记入 tasks.md 3.1 备注 + design D3 补充）。

> 若分布无明显断点或与候选 0.20 偏差大 → 暂停，向用户报告分布。

---

### Block C：前端工具函数 + 类型契约（Task 3.2 / 3.3 / 3.4 / 4）— TDD，派 Deepseek V4 Pro

#### C.1（Task 4）类型契约

`front/app/api/dailyReports.ts` 的 `DailyReportThread`（行 10-20）新增：
```ts
fit_distance?: number  // embedding 不声明（后端 json:"-" 不外泄）
```
**验收**：`pnpm exec nuxi typecheck`（Windows cmd）通过。

#### C.2（Task 3.3 红 / 3.4 绿）threadFit.ts TDD

**红**：`front/app/utils/threadFit.test.ts` 覆盖：
- 贴合值（`fit_distance < THRESHOLD`）→ `isThreadFitDemoted` = false
- 离群值（`> THRESHOLD`）→ true
- **阈值边界**：`fit_distance === 0.20`（候选值本身）→ **false**（阈值之上才降级，0.20 本身不降级，用 `>` 严格大于）
- `fit_distance` 缺失/`undefined`/`0`/负数/NaN → `isThreadFitDemoted` = false（历史 thread 不降级）
- `threadFitLabel`：贴合→「贴合」、离群→「可能跑题」、无信号（缺省/0）→「无贴合信号」

**绿**：`front/app/utils/threadFit.ts`：
```ts
export const THREAD_FIT_DEMOTE_THRESHOLD = <Block B 标定值>  // 候选 0.20
export function isThreadFitDemoted(fitDistance?: number): boolean { ... }
export function threadFitLabel(fitDistance?: number): string { ... }
```
纯函数、无副作用、无 DOM 依赖。

**验收**：`cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit threadFit"`（必须 Windows cmd）全绿。

> ⚠️ `pnpm test:unit` / typecheck / build **必须 Windows cmd 执行**（WSL 缺 native binding，见 AGENTS.md）。lint 可 WSL。

---

### Block D：前端 thread 软降级渲染（Task 5.1-5.5）— 审美 + 交互，派 glm5.2

文件：`DailyReportTopicSection.vue`，两个循环（current 行 181 / history 行 262）都改。

需求映射：
1. **5.1 降级样式**：`<article :class="['drm-thread', { 'drm-thread--demoted': isThreadFitDemoted(thread.fit_distance) }]">`；离群 thread 标题区加灰 token（theme 语义派生，跟随双主题）+ 离群标记图标（如 `mdi:alert-circle-outline`）。
2. **5.1 默认折叠**：离群 thread 初始不在 `expandedThreads`（现状本就不预填，满足；仅需保证展开逻辑对离群 thread 仍可用，点击提示行/标题可展开）。
3. **5.2 提示行**：每个 thread 列表（current + history）底部，当 section 内离群 thread 数 ≥1，渲染「另有 N 条可能跑题的线索」，点击展开这些离群 thread（复用 toggleThread 机制批量加入 expandedThreads）。
4. **5.3 探究区**：thread 展开时（`expandedThreads.has(...)` 分支内）显示贴合度数值（`thread.fit_distance?.toFixed(2)`）+ `threadFitLabel`；**正文 thread 标题绝不出现数字**。
5. **5.4 历史 thread**：`fit_distance` 缺失 → `isThreadFitDemoted` 返回 false → 正常渲染，零额外样式。
6. **5.5 TDD**：扩 `DailyReportTopicSection` 相关单测（或 thread 渲染单测），覆盖：离群灰显折叠、贴合正常、边界、提示行 N 计数、探究行数值、历史不降级。先红后绿。

**主题 token**：参考 `matchQuality.ts` 的 `color-mix(in srgb, token 50%, transparent)` 派生灰 token；勿硬编码 hex。

**验收**：
- `pnpm lint`（WSL）零 error
- `cmd.exe /C "... && pnpm exec nuxi typecheck"` 零 error
- `cmd.exe /C "... && pnpm test:unit"` 全绿（含新增 thread 渲染单测）
- `cmd.exe /C "... && pnpm build"` 成功

---

### Block E：架构体检 + 测试 + 文档 + 门禁（Task 6-9）— controller 主导

#### E.1（Task 6）架构体检
- `codegraph impact isThreadFitDemoted` / `codegraph impact computeThreadFitDistances`：确认调用面命中预期、无 HIGH/CRITICAL 风险遗漏
- `codegraph impact DailyReportThread`：确认类型变更未漏消费点
- 架构合理性：utils 落 `app/utils/` 同层、无循环依赖、embedding `json:"-"` 不外泄、降级样式 theme token 派生

#### E.2（Task 7）测试
- `go test ./internal/topicgraph/service ./internal/topicgraph/repository` PASS
- `go test ./internal/topicgraph/...` PASS（无回归）
- `cmd.exe /C "... && pnpm test:unit threadFit"` PASS
- `cmd.exe /C "... && pnpm test:unit"` PASS（全量回归）

#### E.3（Task 8）文档
- `docs/reference/flow/daily-report.md`：§0 概念字典 thread 行补 fit_distance；匹配血缘表加 System 3 行；§2 加「Thread 贴合度信号」小节（B1 治理点 + 伞形话题为何不用紧凑性剔除，衔接 design D1）
- `docs/reference/architecture/map.md`：日报域加「thread 贴合度可观测」入口（System 1/2/3 并列）
- `docs/reference/standard/` 前端规范：仅在确有新约定时补（thread 级降级样式 / 共享 utils 落位）

#### E.4（Task 9）验证门禁（§11 归档门禁，逐条实测）
- `go vet ./internal/topicgraph/...` 零 error
- `go build ./...` 成功
- `go test ./internal/topicgraph/service ./internal/topicgraph/repository` PASS
- `pnpm lint` 零 error
- `cmd.exe nuxi typecheck` 零 error
- `cmd.exe pnpm build` 成功
- `cmd.exe pnpm test:unit` 全绿
- `grep -rn "fit_distance" front/app --include=*.vue --include=*.ts | grep -v "\.test\.ts"` → 命中新增消费点
- `grep -rn "embedding" front/app/api/dailyReports.ts` → **零命中**（绝不外泄）
- `grep -rn "THREAD_FIT_DEMOTE_THRESHOLD\|isThreadFitDemoted" front/app --include=*.vue --include=*.ts | grep -v "\.test\.ts"` → 命中 utils 定义 + 消费点
- `bash scripts/check-standards.sh` 零失败

#### 收尾
- tasks.md 所有 `[ ]` → `[x]`（边做边勾）
- 全部绿后：`/skill:finishing-a-development-branch`（当前分支，由用户决定 merge 时机）

---

## Review 策略（controller 职责，每块后）

按 subagent-driven-development：
1. **spec reviewer**：代码是否精确匹配 tasks.md/specs（不多建、不少做）—— 派 glm5.2
2. **code quality reviewer**：TDD 是否真红绿、命名、错误处理、theme token、无循环依赖 —— 派 glm5.2
3. 有 issue → 同一 implementer 子线程修 → 再 review，直到双通过才标记块完成
4. 门禁命令由 controller 亲自跑（验证-before-completion，不信任子线程的自述）

## 风险提示

- **Block B 标定依赖真实数据**：若重新生成 2026-06-26 失败或分布无断点 → 暂停报告用户。
- **TDD 红**：Block A/C 必须先看到测试失败、确认失败原因正确，再写实现。
- **Windows cmd 红线**：前端 test:unit/typecheck/build 必须走 `cmd.exe /C`，WSL 会失败。
- **embedding 不外泄**：`json:"-"` 是硬约束，前端契约不得声明 embedding 字段（Task 9.9 门禁）。
- **测试只跑影响包**：后端只跑 `./internal/topicgraph/...`，不跑全量（AGENTS.md）。
