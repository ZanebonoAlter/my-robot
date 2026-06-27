# separate-topic-candidates-from-attention 收尾实施计划

> **REQUIRED SUB-SKILL:** 由主控用 subagent-driven-development 编排执行；每个阶段派独立子进程，阶段间做 code review。

**Goal:** 把 OpenSpec change `separate-topic-candidates-from-attention` 从当前 5/39 推进到 39/39 并通过全部门禁，可归档。

**Architecture:** 叙事身份（PersistentTopic candidate/active/archived）与日报阅读注意力分区解耦。后端已完成快照字段 `topic_status_at_report`、candidate 失活窗口、统一可锚定选择器；剩余主体在前端分区重构、补 2 个后端测试缺口、文档同步和门禁验证。

**Tech Stack:** Go (Gin/GORM) + PostgreSQL/pgvector；Nuxt 4 / Vue 3 / TS / Vitest；openspec spec-driven。

---

## 真实起点状态（2026-06-27 已核实）

- §1（基线）已完成并勾选。
- §2 后端代码**已在工作树实现并通过 `go test ./internal/topicgraph/repository ./internal/topicgraph/service`**，但 tasks.md 未勾选、未提交。
  - 2.8 待办：`ListActiveTopicsByBoard` 命名含混（实际返回 candidate+active），仍被 `cmd/verify-cluster-prompt/main.go:62` 调用。
- §4.1 集成测试已存在（`daily_report_topic_integration_test.go`）。**§4.2（cluster↔assignment 集合一致性测试）缺失**；§4.3 snake_case 契约仅集成测试覆盖，缺独立的“current status ≠ snapshot status”契约断言。
- §3 前端**完全未动**：`dailyReportMagazine.ts` 仍三分区、仍有 `'突发的新话题'`/`'Developing'`；`DailyReportTopicSection.vue:44` 仍有 `candidate: '突发 · 观察中'` 徽章。
- §5/§6/§7 未做。

## 跨阶段硬约束（来自 AGENTS.md / 项目规范）

- 测试只跑影响包，禁止 `go test ./...`（除非门禁阶段）。
- 前端 typecheck/build/test:unit **必须 Windows cmd**：`cmd.exe /C "cd /d D:\project\Syntopica\front && ..."`；lint 可走 WSL。
- Go 用 `go.exe`（Windows），在 WSL 跑需 `cmd.exe /C "cd /d D:\project\Syntopica\backend-go && ..."`。
- 数据库已起（`syntopica-postgres:5432`）。
- 提交身份：`zanebonoalter <380207345@qq.com>`。
- 不新增 linter/formatter/无关改动；改动最小化、贴合现有风格。
- 完成后即时把 tasks.md 对应 `- [ ]` 改 `- [x]`。

---

## Phase 1 — 后端收口与补测试（subagent: `opencode-go/deepseek-v4-pro`）

**Files:**
- Modify/Verify: `backend-go/internal/topicgraph/repository/daily_report_topic_repository.go`、`daily_report_assignment.go`、`daily_report_models.go`
- Modify: `backend-go/cmd/verify-cluster-prompt/main.go`（收敛调用点）
- Test: `backend-go/internal/topicgraph/repository/daily_report_topic_integration_test.go`（新增）、`backend-go/internal/topicgraph/service/*_test.go`（新增 4.2 一致性测试）

**步骤：**

1. **审计现有未提交后端 diff**：对照 `specs/persistent-topic/spec.md` 逐条核对 `selectAnchorableTopics`/`ListAnchorableTopicsByBoard`/`planLifecycle`/`SaveReport` 快照写入是否符合规格（排序键 `last_seen_date DESC, hit_count DESC, id ASC`、窗口边界 `gap > window` 归档、active 仍 30 天、同事务写入快照）。发现偏差则修正。
2. **2.8 命名收敛**：`ListActiveTopicsByBoard` 改名为更准确的 `ListNonArchivedTopicsByBoard`（或保留但修正注释/调用点），`cmd/verify-cluster-prompt/main.go` 同步更新。确认结构化日志（assignment.go:379）含 active/候选总数/窗口过滤/上限截断/auto_new，不输出正文/prompt/embedding。
3. **4.2 RED→GREEN 集合一致性测试**：新增 service 层或 repository 层测试，断言同一 reportDate 下，ClusterTags 注入的 topic id 集合 == assignment 双重确认接受集合（窗口外/被截断 candidate 不单边参与）。先红后绿。
4. **4.3 API 契约测试**：补 snake_case（`topic_status_at_report` JSON tag）+ current status 与 snapshot 不一致的断言（历史 snapshot=active 但 topic 当前 archived，API 仍返回 snapshot）。
5. **4.1 核验**：确认集成测试覆盖迁移幂等、历史 NULL 可查、candidate 自动归档后退出可锚定集合、active 不受 candidate 上限影响；缺则补。
6. **5.1 / 5.3 跑测试**：
   - `cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go test ./internal/topicgraph/repository ./internal/topicgraph/service -count=1"` → 全 PASS。
   - 集成子集：`go test ./internal/topicgraph/repository -run "Test.*(PersistentTopic|TopicAssignment|Candidate|SaveReport|Backfill)" -count=1`。
7. **勾选** tasks.md 的 2.3-2.8、4.1-4.3、5.1、5.3。
8. **不提交**（留待 Phase 1 code review 后由主控决定）。

**验收：** 后端影响包全绿；4.2/4.3 新测试存在且通过；2.8 命名收敛；tasks 对应项已勾选。

---

## Phase 1.5 — 后端 Code Review（subagent: `zai-coding-cn/glm-5.2`）

对 Phase 1 产出的后端 diff（含未提交的预存实现 + 本次新增）做严格审查：
- 是否完全符合 `specs/persistent-topic/spec.md` 与 design.md Decision 1-6；
- 选择器在 ClusterTags 与 assignment 两侧是否真正共享同一集合（无单边加载全部 candidate 的回潮）；
- 快照写入是否在同一事务、是否回填历史（应不回填）；
- 日志是否泄露正文/prompt/embedding；
- 有无过度设计、无关改动。
输出：PASS 或具体修改清单。主控据结果让 Phase 1 子进程返工或提交。

---

## Phase 2 — 前端 TDD 分区解耦（subagent: `opencode-go/deepseek-v4-pro`）

**Files:**
- Modify: `front/app/features/tags/components/daily-report/dailyReportMagazine.ts`（类型 + normalizer + buildQualityZones）
- Modify: `front/app/features/tags/components/daily-report/DailyReportTopicSection.vue`（状态文案）
- Modify: `front/app/features/tags/components/daily-report/DailyReportSidebar.vue`、`BoardDailyReportTimeline.vue`（分区渲染/侧栏）
- Modify: `front/app/api/dailyReports.ts`（`DailyReportSection.topic_status_at_report` 可空字段）
- Test: `dailyReportMagazine.test.ts`、`DailyReportTopicSection.test.ts`、相关 fixture

**步骤（TDD，先红后绿）：**

1. **3.1 RED**：扩展 `dailyReportMagazine.test.ts`，覆盖：snapshot active → “关心的话题”；candidate/null → “其他动态”；candidate 不获排序加权；当前 `topic.status` 变化不改历史分区。旧三分区实现下应红。
2. **3.2 RED**：扩展 `DailyReportTopicSection.test.ts` / BoardDailyReportTimeline 相关测试，断言页面不再出现“突发的新话题”“Developing”或 candidate 状态徽章，candidate 仍能在“其他动态”正常阅读。旧文案/zone 下应红。
3. **3.3 GREEN**：`DailyReportSection` 类型与 normalizer 接收可空 `topic_status_at_report`（API 字段 snake_case → TS camelCase），缺失保持兼容。
4. **3.4 GREEN**：`QualityZoneKey` 收敛为 `'active' | 'briefs'`，按报告时快照分区；candidate/archived/旧 NULL 统一进“其他动态”，保持 `(best_tier ASC, avg_score DESC)` 排序，candidate 无额外加权。
5. **3.5 GREEN**：更新侧栏、正文话题组、状态文案；仅 active snapshot 话题进目录与自动展开；其他动态不自动请求 topic lifeline；移除 candidate “突发·观察中”徽章文案。
6. **3.6 REFACTOR**：删除废弃 candidate zone 分支/类型/样式，不动相邻主题/文章展开/section 生命周期交互。
7. **5.2 跑测试**：`cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit -- app/features/tags/components/daily-report/dailyReportMagazine.test.ts app/features/tags/components/daily-report/DailyReportTopicSection.test.ts"` → PASS。
8. **勾选** 3.1-3.6、5.2、4.4（若 fixture/断言更新到位）。

**验收：** `rg -n -F "突发的新话题" front/app` 零命中；前端单测全绿；types 兼容旧数据。

---

## Phase 2.5 — 前端 Code Review（subagent: `zai-coding-cn/glm-5.2`）

审查 Phase 2 diff：
- 分区是否严格按 snapshot 而非 current status；
- 旧数据 NULL 是否保守降级到“其他动态”；
- 有无残留 candidate zone/徽章/排序加权；
- 无关视觉改动；
- a11y 与主题 token 未被破坏。
输出 PASS 或修改清单。

---

## Phase 3 — 文档同步（subagent: `opencode-go/deepseek-v4-flash`）

**Files:** `docs/reference/flow/`、`architecture/`、`database/`、`api/`、`configuration.md`，及本 change 的 design/spec/tasks 校准。

**步骤（6.1-6.4）：**
1. flow：日报生成/阅读流程改为“candidate 是内部叙事观察态，日报仅关心的话题/其他动态两类”，补报告时快照节点。
2. architecture：说明 ClusterTags 与 assignment 共享可锚定选择器；明确 PersistentTopic 身份系统 vs topic-watch 注意力系统边界。
3. database/api/configuration：记录 `topic_status_at_report`、两个 candidate 配置、NULL 兼容、排序与窗口边界。
4. 对照实现勾选 tasks，检查 reference 不再把 candidate 描述为“突发的新话题”。

**验收：** `rg -n -F "突发的新话题" docs/reference` 仅允许在明确标注“旧语义已废弃”的迁移说明中出现。

---

## Phase 4 — 门禁与质量验证（subagent: `opencode-go/deepseek-v4-flash`）

按 tasks.md §7 全量跑，**逐条贴真实输出证据**，任一失败停下报告：

1. `cmd.exe /C "cd /d D:\project\Syntopica\backend-go && golangci-lint run ./..."` → 退出码 0。
2. `... && go vet ./...` → 0。
3. `... && go test ./internal/topicgraph/repository ./internal/topicgraph/service` → PASS。
4. `... && go build ./...` → 0。
5. `cd front && pnpm lint` → 0（WSL 可用）。
6. `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"` → 0。
7. `... && pnpm test:unit` → 全 PASS。
8. `... && pnpm build` → 构建成功。
9. `rg -n -F "突发的新话题" front/app docs/reference` → 零命中或仅废弃说明。
10. `openspec validate separate-topic-candidates-from-attention --type change --strict` → valid。
11. `git diff --check` → 0；确认 proposal/design/specs/tasks 与 reference 已同步、任务全勾。

**验收：** 11 项全绿并附证据；归档前置条件满足。

---

## 主控（我）的编排节奏

- 每个 Phase 派一个子进程，前台等结果（工作有强先后/共享文件依赖，不并行）。
- Phase 1、2 各配一次 Code Review 子进程；据其结论决定返工或进入下一阶段。
- 提交节点：Phase1 review 通过 → 提交后端；Phase2 review 通过 → 提交前端；Phase3 → 提交文档；Phase4 全绿后由主控做最终 `openspec validate` 复核并提示可归档。
- 任何子进程报告阻塞/规格偏差，停下与用户确认，不擅自改规格。
