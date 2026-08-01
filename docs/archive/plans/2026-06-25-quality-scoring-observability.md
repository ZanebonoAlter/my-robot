# 日报质量排序可观测 Implementation Plan

> **REQUIRED SUB-SKILL:** Use the executing-plans skill to implement this plan task-by-task.
> **执行总指挥（本会话主 agent）负责：** 派发后端/前端执行子进程、review、opsx-verify、归档门禁收尾。

**Goal:** 把日报生成时刻的匹配血缘（`quality_breakdown`）快照到 section 级，修复 `downgraded` 进管线即丢失的 🔴 缺陷，规格化 MatchTier 语义，前端补探测区 tag 级明细 + 正文 tier 徽章。

**Architecture:** 纯 `internal/topicgraph/` 域闭环（后端），`daily_report_sections` 加 `quality_breakdown JSONB` 列（复刻 `cluster_tag_ids` 的 jsonb 模式，零新表）。前端复刻 `match-score-visualization` 的四色系，工具函数上移共享 utils，徽章/t明细颜色由主题 token 派生。

**Tech Stack:** Go/Gin/GORM/PostgreSQL+pgvector（后端）；Nuxt 4/Vue 3 `<script setup lang="ts">`/Tailwind v4（前端）。

**OpenSpec change:** `openspec/changes/quality-scoring-observability/`（schema: spec-driven，36 tasks）。权威 spec 见 `specs/daily-report-system/spec.md`，design 决策见 `design.md`。

---

## ⚠️ 执行前必读：决策张力点（本指挥已权衡，按此执行）

这些是代码现状与 spec/design 假设有出入、或与项目规范字面冲突的点。本指挥已决策，subagent **按下方决策执行**，不要自行扩大 scope。

### DP-1：`dailyReports.ts` 全链路 snake_case，不强行 camelize（偏离 task 4.4 字面）
- **现状：** `front/app/api/dailyReports.ts` **未走** `camelizeKeys()`，TS interface 与组件全程 snake_case（`best_tier`、`cluster_tag_ids`、`a.best_tier` 见 `dailyReportMagazine.ts:64`）。
- **决策：** 新增的 `quality_breakdown` 字段**与现有链路保持一致用 snake_case**（组件内以 `section.quality_breakdown` 访问），**不**为本次引入全链路 camelize 改造（会波及所有日报消费组件，回归风险 >> 本次 scope）。
- **降级 task 4.4：** `quality_breakdown` 字段命名沿用 snake_case 与 `best_tier` 一致；`tag_id` 保持 number 与 `cluster_tag_ids: number[]` 一致（不转字符串）。
- **本指挥会在交付总结里向用户标记此偏离，请用户确认是否另开 change 做全链路 camelCase 规范化。**

### DP-2：~~timeline / lifeline 接口暴露 scope~~【已纠正：全做】
- **原误判（作废）：** 曾认为三处时间线接口返回轻量 `SectionTimelineNode`，扩展需 JOIN + 改结构体 + 改 SQL，成本高，拟只做 detail。
- **纠正（用户拍板 + 指挥调研）：** 回到 spec 原始要求，detail / timeline / lifecycle / lifeline **四接口全暴露**。调研确认成本极低：三处时间线 SQL（`daily_report_repository.go:388/520/691`）查的都是 `FROM daily_report_sections ds`（quality_breakdown 所在表），只需 SELECT 加 `ds.quality_breakdown` 一列 + `SectionTimelineNode`（:47）加 `QualityBreakdown json.RawMessage` 字段。不是 JOIN，不改结构。详见 Task B7。

### DP-3：`matchReasonColor`/`matchInfoLabel` 当前重复两处，统一抽到共享 utils
- **现状：** 两份重复定义：`composables/useBoardTimeline.ts:122` 与 `components/BoardTimelinePanel.vue:41`（逻辑相同）。调用方**仅** `BoardTimelinePanel.vue`。
- **决策：** 抽到 `front/app/utils/matchQuality.ts`（新建），删除两处重复，`BoardTimelinePanel.vue` 与新日报探测区统一 import。颜色从写死 hex 改为**主题 token**（见 DP-4）。

### DP-4：四色 + 徽章颜色改为主题 token（不写死 hex）
- **现状：** `matchReasonColor` 写死 `#22c55e/#3b82f6/#f59e0b/#94a3b8`；token 定义在 `front/app/assets/css/main.css`（editorial `:85` / dark `:150`）。
- **决策：** 在 `main.css` 两个主题块各新增 4 个 token（`--color-match-direct-hit` / `-hit-rate` / `-max-sim` / `-weighted`）+ 徽章复用前三个 + 灰。`matchReasonColor` 返回 `var(--color-match-...)`。降级半透明仍用 `color + '80'`（hex alpha，token 值是 hex 时可行）或改用 opacity 工具——执行者按现有半透明手法保持一致。

### DP-5：section 组装处 `bestTier` 的 `MatchTier` 调用也要修（task 2.1 只提了 filterTagsByQuality）
- **现状：** `orchestrator.go:~170` 算 `bestTier` 时 `tagging.MatchTier(t.MatchReason, false)` 硬编码 false；`filterTagsByQuality:~400` 两处同样硬编码。
- **决策：** **三处全改**为 `MatchTier(t.MatchReason, t.Downgraded)`（依赖 task B2 的 `Downgraded` 字段）。否则 `best_tier` 聚合仍失真（降级 max_sim 被算成 tier 2）。`MergeSimilarSections` 的 `bestTier` 取预存 `min`（merge.go:~204）不需调 MatchTier，但要重算 `quality_breakdown`（task B5）。

### DP-6：迁移是代码闭包，非独立 SQL 文件
- **现状：** 所有版本化迁移是 `internal/platform/database/postgres_migrations.go` 的 `postgresMigrations()` 切片里的闭包；最新版本 `20260620_0001`；版本字符串字典序排序，新版本必须 > 它。纯加列由 AutoMigrate 兜底，但本 change 仍**显式注册一个迁移闭包**（幂等 `ADD COLUMN IF NOT EXISTS`，便于回滚 DROP COLUMN，并满足 task 1.1 的 testcontainer 幂等验收）。

---

## 阶段一：后端切片（派发后端执行子进程 · 模型 Deepseek V4 Pro · TDD）

> 所有改动限定在 `backend-go/internal/topicgraph/` + `backend-go/internal/tagmanagement/handler/board_match_handler.go`（只读复用 MatchTier，不改）+ `backend-go/internal/platform/database/postgres_migrations.go`（迁移）+ 测试。
> **测试只跑影响包**：`go test ./internal/topicgraph/... ./internal/tagmanagement/...`（不跑全量）。
> **TDD 顺序：每个 task 先写失败测试 → 验证失败 → 实现 → 验证通过 → commit。**

### Task B1: 迁移闭包 + GORM 模型字段（tasks 1.1, 1.2）

**Files:**
- Modify: `backend-go/internal/platform/database/postgres_migrations.go`（`postgresMigrations()` 末尾追加）
- Modify: `backend-go/internal/topicgraph/repository/daily_report_models.go:96`（`DailyReportSection`）、`:196`（`TagInput`）
- Test: `backend-go/internal/platform/database/db_unit_test.go`（用 `mustFindMigration` 断言注册，参考 `:129`）

**Step 1：迁移闭包**（追加到 `postgresMigrations()` 切片，Version 必须 > `20260620_0001`）：
```go
{
    Version:     "20260625_0001",
    Description: "Add quality_breakdown JSONB column to daily_report_sections.",
    Up: func(db *gorm.DB) error {
        if !tableExists(db, "daily_report_sections") {
            return nil
        }
        return db.Exec("ALTER TABLE daily_report_sections ADD COLUMN IF NOT EXISTS quality_breakdown JSONB NULL").Error
    },
},
```
幂等（IF NOT EXISTS）；回滚：`ALTER TABLE daily_report_sections DROP COLUMN IF EXISTS quality_breakdown`。

**Step 2：模型字段。** 参考现有 `ClusterTagIDs JSON \`gorm:"type:jsonb" json:"cluster_tag_ids"\``（仓库自定义 `JSON []byte` 类型，见 models 同文件）：
- `DailyReportSection` 加：`QualityBreakdown JSON \`gorm:"type:jsonb" json:"quality_breakdown"\``
- `TagInput` 加：`Downgraded bool \`json:"downgraded"\``

**Step 3：测试。** 在 `db_unit_test.go` 加：`mustFindMigration(t, postgresMigrations(), "20260625_0001")` 断言已注册。

**Step 4：验证迁移在 testcontainer 幂等。** 写/复用一个集成测试（见 Task B8），重复 RunMigrations 两次无错。先此步可先标 TODO，B8 覆盖。

**Step 5：commit。** `feat(topicgraph): add quality_breakdown column migration and model fields`

### Task B2: collectBoardTags 补 downgraded（task 1.3，治 🔴 事实1）

**Files:**
- Modify: `backend-go/internal/topicgraph/service/daily_report_orchestrator.go:248`（`collectBoardTags`）

**Step 1：测试先行（纯逻辑单测，内存 SQLite 不适用——collectBoardTags 走 DB）。** 此函数依赖 DB，单测成本高。改为在 Task B6/B8 用集成测试覆盖（写入带 downgraded 的 `topic_tag_board_labels` 行，断言 `TagInput.Downgraded` 透传）。本 task 先实现，测试归 B8。

**Step 2：实现。** 在 `collectBoardTags`（`:248`）：
1. 本地 `tagRow` 结构体加 `Downgraded bool \`json:"downgraded"\``。
2. `Select(...)` 子句加 `topic_tag_board_labels.downgraded AS downgraded,`（与现有 `match_reason`/`score` 同位置）。
3. 构造 `repository.TagInput{...}` 时填 `Downgraded: row.Downgraded`。
4. **fallback 路径**：函数内若有 `boardMatch` 兜底扫描（执行者 grep `boardMatch`/fallback 定位），同样从匹配结果取 `downgraded` 填入。

**Step 3：验证编译。** `cd backend-go && go build ./internal/topicgraph/...`

**Step 4：commit。** `fix(topicgraph): collect downgraded flag in collectBoardTags (red fact 1)`

### Task B3: section 组装填 quality_breakdown + 修 bestTier MatchTier（task 1.4 + DP-5）

**Files:**
- Modify: `backend-go/internal/topicgraph/service/daily_report_orchestrator.go:~166-194`（section 组装）

**Step 1：测试先行（纯逻辑，可内存 SQLite 或纯函数测试）。** 见 Task B6（best_tier 聚合 + breakdown 组装单测）。先写失败测试：给定 tags（含降级）+ cluster.TagIDs，断言 breakdown 含正确明细且 bestTier 用真实 downgraded。

**Step 2：实现。** 在 section 组装循环（`tagIDSet` 构建后、`sections append` 前）：
```go
// DP-5: 修 bestTier 硬编码 false
for _, t := range tags {
    if tagIDSet[t.ID] {
        tier := tagging.MatchTier(t.MatchReason, t.Downgraded) // 原为 false
        if tier < bestTier { bestTier = tier }
        totalScore += t.Score
        matchCount++
    }
}
// 新增：组装 quality_breakdown（参考 tagIDsJSON 的序列化手法）
type qualityEntry struct {
    TagID       uint    `json:"tag_id"`
    Label       string  `json:"label"`
    MatchReason string  `json:"match_reason"`
    Score       float64 `json:"score"`
    Downgraded  bool    `json:"downgraded"`
}
breakdown := make([]qualityEntry, 0, len(cluster.TagIDs))
for _, t := range tags {
    if tagIDSet[t.ID] {
        breakdown = append(breakdown, qualityEntry{t.ID, t.Label, t.MatchReason, t.Score, t.Downgraded})
    }
}
breakdownJSON, _ := json.Marshal(breakdown) // 用仓库 JSON 类型（[]byte），参考 tagIDsJSON
```
`DailyReportSection{...}` 构造加 `QualityBreakdown: breakdownJSON`（字段类型是仓库 `JSON` 即 `[]byte`）。

**Step 3：验证。** B6 单测通过。

**Step 4：commit。** `feat(topicgraph): snapshot quality_breakdown at section assembly, fix bestTier downgrade`

### Task B4: filterTagsByQuality 截断修复（task 2.1 + DP-5）

**Files:**
- Modify: `backend-go/internal/topicgraph/service/daily_report_orchestrator.go:~400`（`filterTagsByQuality`，两处 MatchTier 调用）

**Step 1：实现。** 两处 `tagging.MatchTier(kept[i].MatchReason, false)` → `tagging.MatchTier(kept[i].MatchReason, kept[i].Downgraded)`（`kept[j]` 同理）。

**Step 2：测试（B6 覆盖）。** 截断排序单测：>30 tag 含降级 max_sim，断言降级 tag 按 tier=3 排序、可能落截断边界外。

**Step 3：commit。** `fix(topicgraph): use real downgraded in filterTagsByQuality truncation sort`

### Task B5: MergeSimilarSections 重算 quality_breakdown（task 1.5）

**Files:**
- Modify: `backend-go/internal/topicgraph/service/daily_report_merge.go:~203-242`（`MergeSimilarSections` 合并重算块）

**Step 1：实现。** 在重算 `avgScore` 同处（`for _, id := range mergedTagIDs` 循环附近），按合并后的 `mergedTagIDs` 从 `tags` 切片重建 `quality_breakdown`（与 B3 同一组装逻辑，建议抽小工具函数 `buildQualityBreakdown(tagIDSet, tags) []byte` 复用，放 orchestrator 或 merge 文件）。赋值 `primary.QualityBreakdown = breakdownJSON`。

**Step 2：测试（B8 集成测试覆盖）。** 合并 section A{1,2} + B{3,4} → breakdown 含 4 条，avgScore 一致。

**Step 3：commit。** `feat(topicgraph): recompute quality_breakdown on section merge`

### Task B6: MatchTier + best_tier + breakdown 纯逻辑单测（tasks 2.2, 2.3, 7.1）

**Files:**
- Create/Modify: `backend-go/internal/tagmanagement/handler/board_match_handler_test.go`（MatchTier 五分支）
- Create/Modify: `backend-go/internal/topicgraph/service/daily_report_orchestrator_test.go`（best_tier 聚合 + breakdown 组装 + filterTagsByQuality 截断）

**Step 1：MatchTier 五分支测试**（覆盖 design D3 映射表）：
```go
func TestMatchTier(t *testing.T) {
    cases := []struct{ reason string; downgraded bool; want int }{
        {"direct_hit", false, 0},
        {"direct_hit", true, 0},   // downgraded 对 direct_hit 无影响
        {"hit_rate", false, 1},
        {"max_sim", false, 2},
        {"max_sim", true, 3},
        {"weighted", false, 3},
        {"unknown", false, 3},     // 未知 reason → 保底 tier 3
    }
    for _, c := range cases {
        if got := MatchTier(c.reason, c.downgraded); got != c.want {
            t.Errorf("MatchTier(%q,%v)=%d want %d", c.reason, c.downgraded, got, c.want)
        }
    }
}
```

**Step 2：best_tier 聚合 + breakdown 组装测试。** 把 section 组装的核心逻辑抽成可测纯函数（如 `computeSectionQuality(tagIDSet, tags) (bestTier int, avgScore float64, breakdown []byte)`），单测覆盖：
- `{0,2,3} → best_tier=0`（组内 min）
- 降级 max_sim 落 tier 3
- breakdown 含每 tag 的 `{tag_id,label,match_reason,score,downgraded}`，JSON 可反序列化

**Step 3：filterTagsByQuality 截断测试。** 构造 >30 个 tag（含降级 max_sim），断言截断后保留 30 个、降级 tag 按 tier=3 排序。

**Step 4：验证。** `cd backend-go && go test ./internal/topicgraph/... ./internal/tagmanagement/...`

**Step 5：commit。** `test(topicgraph): cover MatchTier tiers, best_tier aggregation, breakdown assembly`

### Task B7: API 序列化 — detail + 三个时间线接口全暴露（tasks 3.1, 3.2）【scope 已纠正】

**【scope 纠正（指挥裁定）：回到 spec 原始要求，timeline/lifecycle/lifeline 三接口也要暴露 quality_breakdown。指挥已调研确认成本极低——三处 SQL 都查 `daily_report_sections` 表（quality_breakdown 所在表），只需 SELECT 加一列，不是 JOIN、不改结构。】**

**Files:**
- Modify: `backend-go/internal/topicgraph/repository/daily_report_models.go`（DailyReportSection json tag，B1 已加）
- Modify: `backend-go/internal/topicgraph/repository/daily_report_repository.go:47`（SectionTimelineNode 加字段）、`:388`/`:520`/`:691`（三处 SQL SELECT 加列）

**Step 1：detail 接口（零成本，B1 已完成）。** `DailyReportSection` 加了 `json:"quality_breakdown"` 后，`getDailyReport`→`GetReportByID`（返回 `BoardDailyReport.Sections`）自动序列化。

**Step 2：SectionTimelineNode 加字段（daily_report_repository.go:47）。** 三个时间线接口（getBoardSectionTimeline / getSectionLifecycle / getTopicLifeline）都返回 `SectionTimelineNode`。加字段：
```go
QualityBreakdown json.RawMessage `json:"quality_breakdown"`
```
**为什么用 `json.RawMessage` 而非 `[]byte`：** `[]byte` 在 encoding/json 里会被 base64 编码（错）；`json.RawMessage` 实现 MarshalJSON 原样输出 JSON，且历史行 NULL 扫描成 nil → 序列化为 JSON `null`（spec scenario「历史 section 返回 null」要求），非空行原样输出 JSON 数组。DailyReportSection 那边用仓库自定义 `JSON` 类型，这里时间线轻量结构用标准库 json.RawMessage 即可（两条路径各自合理）。

**Step 3：三处 SQL SELECT 加列。** 三处查询都是 `FROM daily_report_sections ds JOIN board_daily_reports bdr ON ...`，查的就是 quality_breakdown 所在表。在各 SELECT 子句加 `ds.quality_breakdown`（与 `ds.cluster_label` 同位置，照搬现有列名风格）。GORM `Raw(...).Scan(&nodes)` 靠列名映射到结构体字段（`quality_breakdown` → `QualityBreakdown`）。

**Step 4：集成测试（B8 覆盖）。** detail + 三个时间线接口各补断言：新 section 返回 quality_breakdown 数组、历史行（NULL）返回 null、不报错。

**Step 5：commit。** `feat(topicgraph): expose quality_breakdown across detail and timeline APIs`

### Task B8: 后端集成测试（testcontainer pgvector）（tasks 7.2, 8.1-8.4）

**Files:**
- Create/Modify: `backend-go/internal/topicgraph/service/daily_report_integration_test.go`（或现有集成测试文件）

**Step 1：** 用 `testutil.SetupTestDB(t)`（参考 `testutil.go:135`），覆盖：
1. **迁移幂等**（8.2）：连续两次 `RunMigrations` 无错。
2. **新日报写入明细**（spec scenario）：造 3 个 tag（direct_hit/max_sim/weighted）→ 生成日报 → 断言 section `quality_breakdown` 含 3 条明细，字段正确。
3. **降级标记透传**（🔴 事实1）：造 max_sim+downgraded=true 的 tag → 断言 `TagInput.Downgraded=true` 且 breakdown 中 downgraded=true。
4. **合并重算**（1.5）：合并两个 section → breakdown = 并集，avgScore 一致。
5. **历史行 NULL 兼容**（8.1, 3.2）：手动 INSERT 一行 `quality_breakdown=NULL` → 查询不报错、返回 null。
6. **API 序列化**（3.1）：detail 接口返回 `quality_breakdown`（数组或 null）。
7. **回滚可逆**（8.4）：`DROP COLUMN quality_breakdown` 成功（单独断言或文档化）。

**Step 2：验证。** `cd backend-go && go test ./internal/topicgraph/...`（testcontainer，需 Docker）

**Step 3：commit。** `test(topicgraph): integration tests for quality_breakdown lifecycle`

### Task B9: 架构体检 + 后端门禁（tasks 6.1, 6.2, 10.1）

**Step 1：架构体检（§7）。** 跑：
- `codegraph impact filterTagsByQuality` / `codegraph impact MatchTier`：波及面无 HIGH/CRITICAL 被忽略。
- `codegraph affected daily_report_orchestrator.go` / `daily_report_merge.go` / `daily_report_models.go`：受影响测试范围符合预期。
- 分层合规：改动全在 `internal/topicgraph/` + 迁移文件，无循环依赖。

**Step 2：后端门禁（task 10.1）。**
```bash
cd backend-go && golangci-lint run ./... && go vet ./... && go test ./internal/topicgraph/... ./internal/tagmanagement/... && go build ./...
```
测试只跑影响包（AGENTS.md）。全部通过。

**Step 3：报告。** 向本指挥汇报门禁结果 + 架构体检结果。

---

## 阶段二：前端切片（派发前端执行子进程 · 模型 glm5.2 · 审美/双主题）

> 依赖阶段一的 detail API 契约（`quality_breakdown` 字段）。前端改动限定在 `front/app/`。
> **门禁：lint（WSL）+ typecheck/build（Windows cmd）+ test:unit。**

### Task F1: 四色 + 徽章主题 token 定义（DP-4）

**Files:**
- Modify: `front/app/assets/css/main.css`（editorial `:85` 块 + dark `:150` 块各加）

**Step 1：** 在 `[data-theme="editorial"]` 与 `[data-theme="dark"]` 各新增：
```css
--color-match-direct-hit: #22c55e; /* editorial 可微调为暖色调适配，dark 提亮 */
--color-match-hit-rate:    #3b82f6;
--color-match-max-sim:     #f59e0b;
--color-match-weighted:    #94a3b8;
```
两主题下色值可分别调（editorial 暖白底、dark 深底），保证对比度可区分。

**Step 2：commit。** `style(front): add match-quality theme tokens (editorial/dark)`

### Task F2: matchReasonColor/matchInfoLabel 上移共享 utils（task 4.1, DP-3）

**Files:**
- Create: `front/app/utils/matchQuality.ts`
- Modify: `front/app/features/tags/composables/useBoardTimeline.ts:122`（删除本地定义，改 import）
- Modify: `front/app/features/tags/components/BoardTimelinePanel.vue:41`（删除本地副本，改 import）

**Step 1：** 新建 `utils/matchQuality.ts`，把 `matchReasonColor`/`matchInfoLabel` 搬入。`matchReasonColor` 颜色改用 token（DP-4）：
```ts
import type { BoardArticleTag } from '...' // 确认类型来源

export function matchReasonColor(reason: string, downgraded?: boolean): string {
  const colors: Record<string, string> = {
    direct_hit: 'var(--color-match-direct-hit)',
    hit_rate: 'var(--color-match-hit-rate)',
    max_sim: 'var(--color-match-max-sim)',
    weighted: 'var(--color-match-weighted)',
  }
  return colors[reason] || 'var(--color-match-weighted)'
}
export function matchInfoLabel(tag: { match_reason: string; score: number; downgraded?: boolean }): string {
  const labels: Record<string,string> = { direct_hit:'直接命中', hit_rate:'命中率', max_sim:'相似度', weighted:'综合' }
  return `${labels[tag.match_reason]||tag.match_reason} ${tag.score.toFixed(2)}${tag.downgraded?'↓':''}`
}
```
**注意降级半透明：** 现有写法 `color + '80'`（hex alpha）。token 化后 `var(...)` 不能拼 alpha。执行者改为：降级时给元素加 `opacity: 0.5` 或 Tailwind `opacity-50` class，或在调用处用 `style="opacity:0.5"`。**保持降级视觉一致（50% 不透明 + ↓）**。

**Step 2：** `useBoardTimeline.ts` 与 `BoardTimelinePanel.vue` 删除本地定义，改为 `import { matchReasonColor, matchInfoLabel } from '~/utils/matchQuality'`。`codegraph impact matchReasonColor` 确认无遗漏调用点。

**Step 3：验证。** `cd front && pnpm lint`（WSL）。原 TagsPage 行为不变（人工/快照核验）。

**Step 4：commit。** `refactor(front): hoist match-quality utils to shared, use theme tokens`

### Task F3: 正文 Tier 徽章组件（tasks 5.1, 5.2, 5.4）

**Files:**
- Create: `front/app/features/tags/components/daily-report/SectionTierBadge.vue`
- Modify: section 渲染组件 `front/app/features/tags/components/daily-report/DailyReportTopicSection.vue`（接入徽章）

**Step 1：** 新建 `SectionTierBadge.vue`（`<script setup lang="ts">`），props `{ bestTier: number }`：
- bestTier=0 → 实心点，`var(--color-match-direct-hit)`
- bestTier=1 → 实心点，`var(--color-match-hit-rate)`
- bestTier=2 → 实心点，`var(--color-match-max-sim)`
- bestTier=3 → **空心点**（border），`var(--color-match-weighted)`
- **零文字/数字/百分比**（spec 强制）。形态仅"实心 vs 空心 + 四色"。
- 用内联 `style` 或 Tailwind + `style` 绑定 token；不写死 hex。

**Step 2：** `DailyReportTopicSection.vue` 在 section 标题/区块头接入 `<SectionTierBadge :best-tier="section.best_tier" />`（snake_case，DP-1）。

**Step 3：历史兼容（5.2）。** 徽章只接 `best_tier`（独立冻结字段），不依赖 `quality_breakdown`。历史 section（quality_breakdown=null 但 best_tier 存在）正常显示徽章。

**Step 4：组件库合规（5.4）。** 徽章无交互则纯展示组件；若加 hover 说明，用 `AppDialog`/`AppButton`（`components/ui/`），禁原生 button 样式类、禁 `window.*`。

**Step 5：commit。** `feat(front): add SectionTierBadge (color-only, no numbers)`

### Task F4: 探测区 tag 级明细（tasks 4.2, 4.3, 4.4）

**Files:**
- Modify/扩展 section 渲染组件的 hover/详情探测区（`DailyReportTopicSection.vue` 或专门的探测区子组件）

**Step 1：** 在 section 探测区（hover 或展开详情）渲染 `section.quality_breakdown`（数组）：每条 tag chip：
- 颜色：`matchReasonColor(entry.match_reason, entry.downgraded)`（复用 F2）
- 文字：`matchInfoLabel(entry)`（含 score + 降级 ↓）
- 降级：50% 不透明（F2 降级表现统一）

**Step 2：历史降级（4.3）。** `quality_breakdown === null`（或 undefined）→ 显示占位文案"无质量明细"。

**Step 3：命名（4.4，按 DP-1 降级）。** 组件内以 `quality_breakdown`（snake_case）访问，与 `best_tier`/`cluster_tag_ids` 一致；`tag_id` 保持 number。**不**引入全链路 camelize。

**Step 4：commit。** `feat(front): render quality_breakdown in section explore panel`

### Task F5: 前端单测（task 7.3）

**Files:**
- Create: `front/app/features/tags/components/daily-report/__tests__/SectionTierBadge.spec.ts`（或项目现有测试目录约定）
- Create: 探测区明细渲染 spec

**Step 1：** Vitest + happy-dom 覆盖：
1. `SectionTierBadge` 四态（0 绿实心/1 蓝实心/2 橙实心/3 灰空心）样式映射，零文字。
2. 探测区渲染 `quality_breakdown`：每条 chip 颜色 + score + 降级标记。
3. 历史 null → "无质量明细"占位。
4. `matchReasonColor`/`matchInfoLabel` 复用正确（token + 标签）。

**Step 2：验证。** `cd front && pnpm test:unit`

**Step 3：commit。** `test(front): cover SectionTierBadge states and explore panel`

### Task F6: 架构体检 + 前端门禁（tasks 6.3, 6.4, 10.2）

**Step 1：架构体检。**
- `codegraph impact matchReasonColor` / `codegraph impact matchInfoLabel`：上移后调用面 import 全更新（6.3）。
- 分层合规：改动在 tags feature + utils + css，无循环依赖（6.4）。

**Step 2：前端门禁（task 10.2）。**
```bash
# lint — WSL 可用
cd front && pnpm lint
# typecheck / build — 必须 Windows cmd（AGENTS.md）
cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"
cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"
cd front && pnpm test:unit
```
全部通过。

**Step 3：报告。** 向本指挥汇报门禁结果。

---

## 阶段三：归档门禁收尾（本指挥亲自执行）

### Task G1: 浏览器视觉验收（task 5.3）
桌面 1280px，editorial + dark 双主题，核验徽章不破沉浸、探测区可读、降级清晰。

### Task G2: openspec validate（task 10.3）
`openspec validate quality-scoring-observability` 通过。

### Task G3: issue 状态（task 10.4）
`docs/issues/01-quality-sort-blackbox.md` 标记 resolved（归档时）。

### Task G4: 文档流转（task 9，里程碑收尾延后）
`docs/reference/`（database/api/architecture）在里程碑收尾统一更新，不在本 change 内逐条改活文档（§12）。本 change 仅登记待更新清单。

---

## 执行顺序与派发策略（本指挥执行）

1. **阶段一**：派发**后端执行子进程**（Deepseek V4 Pro，TDD），执行 B1-B9。background，完成后 review 实际改动 + 门禁输出。
2. **阶段二**：后端 review 通过后，派发**前端执行子进程**（glm5.2），执行 F1-F6。background，完成后 review。
3. **验收**：派发 **opsx-verify 子进程**（glm5.2），按 openspec-verify-change skill 验收完整度/正确性/一致性。
4. **收尾**：本指挥执行 G1-G4（浏览器验收 + openspec validate + issue 状态）。

## 验收标准（每个 task 必须满足）
- TDD：先失败测试 → 实现 → 通过（适用处）。
- 测试只跑影响包（后端 `topicgraph`/`tagmanagement`）。
- 前端 typecheck/build 经 Windows cmd。
- 每个 task 完成更新 `tasks.md` 对应 `- [ ]` → `- [x]`。
- 改动最小、scoped，匹配现有代码风格（§3）。
