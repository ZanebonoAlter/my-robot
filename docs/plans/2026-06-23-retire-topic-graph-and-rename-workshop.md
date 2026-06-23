# 退休旧 Topic Graph + 重命名「叙事工坊」Implementation Plan

> **REQUIRED SUB-SKILL:** Use the executing-plans skill to implement this plan task-by-task.

**Goal:** 移除已名存实亡的旧 topic-graph（按天标签共现图谱页面 + `/api/topic-graph/*` 端点 + 整条管线），把腾出的导航位语义化；将侧栏「标签管理」重命名为「叙事工坊」。

**Architecture:** 这是一次**搬迁 + 手术切除**，不是一键删目录。后端 graph 代码与 daily_report/persistent-topic 零交叉依赖（已验证），可干净切除；前端 topic-graph feature 目录里混着 3 个仍被活跃功能引用的共享件，必须先搬迁再删剩余。所有 DB 模型与表保留不动。

**Tech Stack:** Go (Gin/GORM) backend · Nuxt 4 / Vue 3 / TS frontend · pgvector PostgreSQL

---

## 范围与关键决策（执行前必读）

### ✅ 删除（真正死亡的旧管线）
- **前端**：`pages/topics.vue`、整个 `features/topic-graph/` feature（除 3 个搬迁件）、`api/topicGraph.ts`、`api/index.ts` 中的 re-export、侧栏「主题图谱」入口、onboarding 中 nav-topic-graph 步骤、watched-tags「前往关注」指向 /topics 的跳转。
- **后端**：`internal/topicgraph/handler/graph_handler.go`(+test)、`service/graph_service.go`、`repository/graph_helpers.go`；从 `repository/repository.go` 剥离所有 graph 方法（保留 Repo 单例 + struct + DB()）；`routes.go` 删 `/topic-graph` 路由组；`wire.go` 删 graph handler re-export。
- **文档**：`docs/reference/api/topic-graph.md`、`docs/user-guide/topic-graph/*`、`docs/reference/api/_index.md` 等活文档中的 topic-graph 条目；README / architecture 中对「主题图谱」的现状描述。

### 🔒 保留（共享 / 仍活跃，绝对不动）
- **DB 模型与表全部保留**：`models/topic_graph.go`（实为 tag 核心模型：分类常量、`TopicTagEmbedding`、`TagMergeSuggestion`、`ArticleTopicTag`）、`topic_tag_relation.go`（被 tagmanagement 共用）、`topic_tag_analysis.go`（快照表，仅 migrator 引用，孤儿但无害）。**不写任何 drop 迁移**（§10：表变更走显式迁移，本次不动表）。
- **后端 daily_report / persistent-topic 全套**（`daily_report_*.go`、persistent-topic 新代码）原样保留。
- **tagging 包中的类型**（`tagging.TopicGraphResponse` 等）：作为可选清理（Task 9），仅在零引用时移除。

### 🚚 搬迁（3 个活着的共享件，先搬后删）
| 文件 | 现位置 | 目标位置 | 消费方 |
|------|--------|---------|--------|
| `graphBfsHighlight` | `features/topic-graph/utils/graphBfsHighlight.ts` | `front/app/utils/graphHighlight.ts` | BoardThreadBrowser、detective-wall/InteractionLayer、SectionLifecyclePanel |
| `TagQueuePanel` | `features/topic-graph/components/TagQueuePanel.vue` | `front/app/features/settings/components/TagQueuePanel.vue` | GlobalSettingsDialog、SettingsSectionQueues |
| `TagMergePreview` | `features/topic-graph/components/TagMergePreview.vue` | `front/app/features/tags/components/TagMergePreview.vue` | TagsPage（叙事工坊的合并 UX） |

### 环境约定（来自 AGENTS.md）
- **前端 typecheck / build 必须走 Windows cmd**（WSL 缺 native binding）；lint 可在 WSL 跑。
- **测试只跑本次修改影响的包**，不要跑全量。
- 路径用 Windows 格式 `D:\project\Syntopica\...`（cmd 场景）。

---

## Task 0: 基线确认

**Files:** 无修改

**Step 1:** 确认工作区起点干净
Run: `cd /mnt/d/project/Syntopica && git status --short`
Expected: 无未跟踪/未提交的无关变更（若有，先与用户确认）

**Step 2:** 后端基线编译
Run: `cd /mnt/d/project/Syntopica/backend-go && go build ./...`
Expected: 成功

**Step 3:** 前端基线 typecheck（Windows cmd）
Run: `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"`
Expected: TYPECHECK_PASS

**Step 4:** Commit 起点（若需要）
仅在步骤 1 有 staged 变更时处理；否则跳过。

---

## Task 1: 搬迁 `graphBfsHighlight` 工具

**Files:**
- Move: `front/app/features/topic-graph/utils/graphBfsHighlight.ts` → `front/app/utils/graphHighlight.ts`
- Modify imports in:
  - `front/app/features/tags/components/BoardThreadBrowser.vue:6`
  - `front/app/features/tags/components/detective-wall/InteractionLayer.ts:20`
  - `front/app/features/tags/components/SectionLifecyclePanel.vue:6`

**Step 1:** 移动文件（用 git mv 保留历史）
```bash
cd /mnt/d/project/Syntopica/front
git mv app/features/topic-graph/utils/graphBfsHighlight.ts app/utils/graphHighlight.ts
```
注意：若 `app/utils/` 目录不存在则先 `mkdir -p app/utils`。

**Step 2:** 更新 3 处导入路径
- `BoardThreadBrowser.vue`: `~/features/topic-graph/utils/graphBfsHighlight` → `~/utils/graphHighlight`
- `InteractionLayer.ts`: 同上
- `SectionLifecyclePanel.vue`: 同上（该文件还导入了 `type GraphHighlightEdge`，一并改路径）

**Step 3:** 确认没有遗漏的引用
Run: `grep -rn "graphBfsHighlight" app`
Expected: 仅 `app/utils/graphHighlight.ts` 自身（文件名匹配）或无结果

**Step 4:** typecheck（Windows cmd）
Run: `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"`
Expected: TYPECHECK_PASS

**Step 5:** Commit
```bash
git add -A && git commit -m "refactor: relocate graphBfsHighlight to shared utils"
```

---

## Task 2: 搬迁 `TagQueuePanel`

**Files:**
- Move: `front/app/features/topic-graph/components/TagQueuePanel.vue` → `front/app/features/settings/components/TagQueuePanel.vue`
- Modify imports in:
  - `front/app/components/dialog/GlobalSettingsDialog.vue:5`
  - `front/app/features/settings/components/SettingsSectionQueues.vue:4`

**Step 1:** 移动文件
```bash
cd /mnt/d/project/Syntopica/front
git mv app/features/topic-graph/components/TagQueuePanel.vue app/features/settings/components/TagQueuePanel.vue
```

**Step 2:** 更新 2 处导入路径
- `GlobalSettingsDialog.vue`: `~/features/topic-graph/components/TagQueuePanel.vue` → `~/features/settings/components/TagQueuePanel.vue`
- `SettingsSectionQueues.vue`: 同上

**Step 3:** 确认无遗漏
Run: `grep -rn "topic-graph/components/TagQueuePanel" app`
Expected: 无结果

**Step 4:** typecheck（Windows cmd）
Run: `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"`
Expected: TYPECHECK_PASS

**Step 5:** Commit
```bash
git add -A && git commit -m "refactor: relocate TagQueuePanel to settings feature"
```

---

## Task 3: 搬迁 `TagMergePreview`

**Files:**
- Move: `front/app/features/topic-graph/components/TagMergePreview.vue` → `front/app/features/tags/components/TagMergePreview.vue`
- Move + 重写 facade: `front/app/features/topic-graph/public.ts`（仅导出 TagMergePreview）→ 改为 TagsPage 直接导入
- Modify import in `front/app/features/tags/components/TagsPage.vue:13`

**Step 1:** 移动文件
```bash
cd /mnt/d/project/Syntopica/front
git mv app/features/topic-graph/components/TagMergePreview.vue app/features/tags/components/TagMergePreview.vue
```

**Step 2:** 更新 TagsPage 导入
`TagsPage.vue:13`: `import { TagMergePreview } from '~/features/topic-graph/public'` → `import TagMergePreview from './TagMergePreview.vue'`

**Step 3:** 确认 `public.ts` 现仅剩 TagMergePreview 一条导出（搬迁后它将变空，留待 Task 4 随 feature 目录一起删除）。确认无其他消费方
Run: `grep -rn "features/topic-graph/public" app`
Expected: 无结果

**Step 4:** typecheck（Windows cmd）
Run: `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"`
Expected: TYPECHECK_PASS

**Step 5:** Commit
```bash
git add -A && git commit -m "refactor: relocate TagMergePreview to tags feature"
```

---

## Task 4: 删除剩余 topic-graph 前端（页面 + feature 目录 + API 客户端 + 路由）

**Files:**
- Delete: `front/app/pages/topics.vue`
- Delete: 整个 `front/app/features/topic-graph/` 目录（搬迁后剩余全部文件：TopicGraphPage、Canvas、Header、Sidebar、FooterPanels、EmptyGuide、FeedCategoryFilter、KeywordCloud、TopicTimeline、Timeline*、TopicGraphArticleCard、TagHierarchy*、TagMergeGroup、TagQueuePanel?、public.ts、所有 composables/、所有 utils/ 剩余、public.ts 等）
- Delete: `front/app/api/topicGraph.ts`
- Modify: `front/app/api/index.ts:9`（删除 `export { useTopicGraphApi } from './topicGraph'`）

**Step 1:** 删除页面路由
```bash
cd /mnt/d/project/Syntopica/front
git rm app/pages/topics.vue
```

**Step 2:** 删除整个 feature 目录（搬迁后已不含 3 个共享件）
```bash
git rm -r app/features/topic-graph
```

**Step 3:** 删除 API 客户端
```bash
git rm app/api/topicGraph.ts
```

**Step 4:** 删除 `api/index.ts` 中的 re-export 行
移除: `export { useTopicGraphApi } from './topicGraph'`

**Step 5:** 兜底搜索任何残留引用
Run:
```bash
cd /mnt/d/project/Syntopica/front
grep -rn "features/topic-graph\|api/topicGraph\|useTopicGraphApi\|TopicGraphPage\|/topics" app --include='*.vue' --include='*.ts'
```
Expected: 仅 Task 5 将处理的侧栏/onboarding 中的 `/topics` 跳转（watched-tags-go-btn）；其余应无结果。若发现遗漏引用，逐一清除。

**Step 6:** typecheck（Windows cmd）
Run: `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"`
Expected: TYPECHECK_PASS

**Step 7:** Commit
```bash
git add -A && git commit -m "feat: remove legacy topic-graph frontend (page, feature, api client)"
```

---

## Task 5: 清理导航与 onboarding 中对 /topics 的引用

**Files:**
- Modify: `front/app/features/shell/components/AppSidebarView.vue`（删除「主题图谱」按钮块 ~L174-177；watched-tags-go-btn 跳转改向）
- Modify: `front/app/composables/useOnboarding.ts`（删除 nav-topic-graph tour 步骤 ~L125）

**Step 1:** 删除侧栏「主题图谱」入口按钮块
在 `AppSidebarView.vue` 中删除：
```vue
<button class="sidebar-item" :class="{ active: selectedCategory === 'topic-graph' }" data-onboarding="nav-topic-graph" @click="handleTopicGraphClick">
  <Icon icon="mdi:graph-outline" width="20" height="20" class="text-[var(--color-text-secondary)]" />
  <span v-if="!sidebarCollapsed" class="flex-1 text-left font-medium">主题图谱</span>
</button>
```
同时清理仅被该按钮使用的 `handleTopicGraphClick`（若它定义在 script 中且无其他引用）与 `'topic-graph'` 相关 active 判断。删除前 `grep -n "handleTopicGraphClick\|'topic-graph'" app/features/shell` 确认无其他引用。

**Step 2:** 修正 watched-tags 空状态的「前往关注」跳转
将 `@click="navigateTo('/topics')"` 改为 `@click="navigateTo('/tags')"`（/topics 已不存在，指向叙事工坊）。

**Step 3:** 删除 onboarding driver 中指向已删元素的 nav-topic-graph 步骤

引导用的是 `driver.js`（见 `useOnboarding.ts`）。首页引导 `HOME_STEPS` 数组里有一条步骤的 `element` 选择器是 `[data-onboarding="nav-topic-graph"]` —— 指向 Task 5 Step 1 删除的侧栏按钮。元素没了，这条步骤必须整条删除（虽然 `preFilterSteps` 会过滤掉找不到元素的步骤，但留着死引用是脏的）。

在 `useOnboarding.ts` 的 `HOME_STEPS` 中删除整条：
```ts
{ element: '[data-onboarding="nav-topic-graph"]', popover: { title: '主题图谱', description: '可视化标签之间的关系，发现内容之间的联系。' } },
```

注意：`useOnboarding.test.ts` 里有一条断言 `expect(titles).not.toContain('主题图谱')`，删步后仍成立（标题不在数组里），**无需改这条**。但同文件里 `toContain('标签管理')` / `toBe('语义板块管理')` 会在 Task 6 改名后失效 —— 由 Task 6 处理。

**Step 4:** 兜底搜索
Run: `grep -rn "nav-topic-graph\|主题图谱\|/topics" app --include='*.vue' --include='*.ts'`
Expected: 无结果

**Step 5:** typecheck + lint
Run: `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"`
Run: `cd /mnt/d/project/Syntopica/front && pnpm lint`
Expected: TYPECHECK_PASS / lint 0 error

**Step 6:** Commit
```bash
git add -A && git commit -m "feat: remove topic-graph nav entry and onboarding step"
```

---

## Task 6: 重命名「标签管理」→「叙事工坊」（含 onboarding driver + 测试）

改名涉及三处用户可见文案 + 引导文案 + **测试断言**（测试里有写死标题，不改会红）。

**Files:**
- Modify: `front/app/features/shell/components/AppSidebarView.vue`（侧栏 span 文案）
- Modify: `front/app/features/tags/components/TagsPage.vue`（页面 h1）
- Modify: `front/app/composables/useOnboarding.ts`（home nav-tags 步标题、tags tour 欢迎步标题、文件头/JSDoc 注释）
- Modify: `front/app/composables/useOnboarding.test.ts`（两处写死标题的断言）

**Step 1:** 侧栏文案
`AppSidebarView.vue` 中 `<span ...>标签管理</span>` → `<span ...>叙事工坊</span>`（保留 `data-onboarding="nav-tags"` 不变）。

**Step 2:** 页面标题
`TagsPage.vue` 中 `<h1 class="tags-page-title">语义板块管理</h1>` → `<h1 class="tags-page-title">叙事工坊</h1>`。

**Step 3:** onboarding driver 文案（`useOnboarding.ts`，逐项改，不要用全局替换碰 storage key）
- 文件头注释 `支持多个 tour preset：首页（home）与标签管理页（tags）` → `…与叙事工坊页（tags）`。
- `HOME_STEPS` 里 nav-tags 步：`title: '标签管理'` → `title: '叙事工坊'`（description `'查看和管理 AI 为文章自动生成的标签与语义版块。'` 保持不变，语义一致）。
- `TAGS_STEPS` 欢迎步：`title: '语义板块管理'` → `title: '叙事工坊'`（description 保持不变）。
- `startTagsTour` / `isTagsFirstRun` 的 JSDoc 注释 `标签管理页引导教程` → `叙事工坊页引导教程`。
- 🔒 **绝不动** `STORAGE_KEY_PREFIX` / `TAGS_KEY` 等 localStorage key 字符串及其注释，否则会重置已用过引导的用户的状态。

**Step 4:** 修复测试断言（`useOnboarding.test.ts`，改名后必红的两处）
- 首页 tour 测试 "keeps a step whose data-onboarding element is present in the DOM"：
  `expect(titles).toContain('标签管理')` → `expect(titles).toContain('叙事工坊')`。
  （同测 `expect(titles).not.toContain('主题图谱')` 保持不变 —— Task 5 删步后仍成立。）
- tags tour 测试 "runs the tags steps and marks the tags key complete on destroy"：
  `expect(config?.steps?.[0]?.popover?.title).toBe('语义板块管理')` → `toBe('叙事工坊')`。

**Step 5:** 兜底搜索其他「标签管理」/「语义板块管理」入口文案
Run: `grep -rn "标签管理\|语义板块管理" app --include='*.vue' --include='*.ts'`
Expected: 无结果（或仅 storage key 注释等非用户可见处；若还有用户可见的，一并改为叙事工坊）

**Step 6:** typecheck + lint + 引导单元测试
Run: `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"`
Run: `cd /mnt/d/project/Syntopica/front && pnpm lint`
Run: `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm vitest run app/composables/useOnboarding.test.ts"`
Expected: TYPECHECK_PASS / lint 0 error / 测试全过

**Step 7:** Commit
```bash
git add -A && git commit -m "feat: rename 标签管理 to 叙事工坊 (incl. onboarding driver + tests)"
```

---

## Task 7: 后端手术切除 graph 管线

**Files:**
- Delete: `backend-go/internal/topicgraph/handler/graph_handler.go`
- Delete: `backend-go/internal/topicgraph/handler/graph_handler_test.go`
- Delete: `backend-go/internal/topicgraph/service/graph_service.go`
- Delete: `backend-go/internal/topicgraph/repository/graph_helpers.go`
- Modify: `backend-go/internal/topicgraph/repository/repository.go`（剥离 graph 方法，保留 Repo 单例）
- Modify: `backend-go/internal/topicgraph/routes.go`（删 `/topic-graph` 路由组）
- Modify: `backend-go/internal/topicgraph/wire.go`（删 graph handler re-export）

**关键约束：** `daily_report_*.go` 全套原样保留。已验证 graph 代码与 daily_report 零交叉依赖。

**Step 1:** 删除纯 graph 文件
```bash
cd /mnt/d/project/Syntopica/backend-go
git rm internal/topicgraph/handler/graph_handler.go
git rm internal/topicgraph/handler/graph_handler_test.go
git rm internal/topicgraph/service/graph_service.go
git rm internal/topicgraph/repository/graph_helpers.go
```

**Step 2:** 精简 `repository/repository.go`
**保留**：包声明、imports、`var Repo *TopicGraphRepository`、`InitRepository`、`NewTopicGraphRepository`、`TopicGraphRepository` struct 定义、`func (r *TopicGraphRepository) DB() *gorm.DB`。
**删除**以下全部方法（L46–L608，均为 graph 专用）：
- `BuildTopicGraph` / `BuildTopicDetail` / `FetchTopicArticles` / `BuildTopicsByCategory`
- `GetPendingArticlesByTag` / `GetDigestsByArticleTag`
- `collectAllChildTagIDs` / `fetchArticleTagsData` / `getTopicArticles` / `getRelatedTags` / `buildTopicHistory`
- 以及文件内定义的仅被 graph 使用的辅助类型（如 `HotspotDigestCard`、`ArticleTagData`，**若**它们仅被已删方法引用）。
删除后清理因方法移除而不再使用的 imports（如 `tagging`、`models`、`net/url`、`strings` 等）——以 `go build` 报错为准逐个修。

**Step 3:** 精简 `routes.go`
删除 `/topic-graph` 路由组整块（含 6 条 GET 路由），**保留**末尾 `RegisterDailyReportRoutes(rg)`。删除后 `GetTopicGraph` 等 handler 变量不再被引用（它们在 wire.go，Step 4 一并删）。

**Step 4:** 精简 `wire.go`
删除 graph handler re-export 块：
```go
GetTopicGraph                  = handler.GetTopicGraph
GetTopicDetail                 = handler.GetTopicDetail
GetTopicsByCategory            = handler.GetTopicsByCategory
GetTopicArticles               = handler.GetTopicArticles
GetDigestsByArticleTagHandler  = handler.GetDigestsByArticleTagHandler
GetPendingArticlesByTagHandler = handler.GetPendingArticlesByTagHandler
```
**保留** service 层 re-export（`CollectBoardIDsForDate` / `GenerateDailyReport` / `SaveReport`，被 admin scheduler 使用）。清理 handler import（若 wire.go 不再引用 handler 包）。

**Step 5:** 编译验证（受影响包）
Run: `cd /mnt/d/project/Syntopica/backend-go && go build ./...`
Expected: 成功。若有 unused import / undefined 报错，按报错清理（这是手术切除的正常反馈）。

**Step 6:** vet + lint + test（受影响包）
Run: `cd /mnt/d/project/Syntopica/backend-go && go vet ./internal/topicgraph/...`
Run: `cd /mnt/d/project/Syntopica/backend-go && golangci-lint run ./internal/topicgraph/...`
Run: `cd /mnt/d/project/Syntopica/backend-go && go test ./internal/topicgraph/...`
Expected: vet 0 / lint 0 issues / test 全过（daily_report 测试应仍全绿）

**Step 7:** Commit
```bash
git add -A && git commit -m "feat: remove legacy topic-graph backend pipeline (handlers/service/repo methods/routes)"
```

---

## Task 8: 文档清理（活文档）

**Files:**
- Delete: `docs/reference/api/topic-graph.md`
- Delete: `docs/user-guide/topic-graph/` 整个目录（api.md、guide.md）
- Modify: `docs/reference/api/_index.md`（删除 topic-graph 条目）
- Modify: `docs/reference/architecture/backend.md`、`frontend.md`、`overview.md`、`runtime.md`（删除/改写对 topic-graph / 主题图谱 的现状描述）
- Modify: `docs/reference/database/` 相关（若有 topic-graph 表描述；DB 表保留，仅删「图谱端点」相关描述）
- Modify: `README.md`（删除「主题图谱」「topic-graph」相关 journey/feature 描述）
- Modify: `docs/README.md`（若有 topic-graph 链接）

**🔒 不动：** `docs/v1.*/` 历史里程碑文档（含 `docs/v1.1-bugfixes/changes/topic-graph-*.md`、`docs/v1.2-tag-intelligence/...` 等）——这些是历史归档，保留原样。`docs/plans/*` 历史计划亦保留。

**Step 1:** 删除 topic-graph 专属文档
```bash
cd /mnt/d/project/Syntopica
git rm docs/reference/api/topic-graph.md
git rm -r docs/user-guide/topic-graph
```

**Step 2:** 更新 `_index.md`，移除 topic-graph 条目

**Step 3:** 逐文件清理 architecture 文档与 README 中的「主题图谱 / topic-graph」现状描述。原则：删除对已移除端点/页面的描述；不改写历史。对每处先读上下文再改，避免误伤仍在用的「话题演进 / persistent-topic」描述。

**Step 4:** 兜底确认活文档中无残留端点引用
Run: `cd /mnt/d/project/Syntopica && grep -rn "api/topic-graph\|/topic-graph/" docs/reference README.md`
Expected: 无结果（历史里程碑 docs/v1.* 可忽略）

**Step 5:** Commit
```bash
git add -A && git commit -m "docs: remove retired topic-graph references from active docs"
```

---

## Task 9（可选清理）: 移除 tagging 包中已孤儿的 TopicGraph* 类型

**仅在 Task 7 完成后、且零引用时执行。** 若 grep 发现仍有引用则跳过本任务。

**Files:**
- `backend-go/internal/tagmanagement/service/core/types.go`（`TopicGraphResponse` 等类型，L176 起）
- 可能波及 `tagmanagement/service/service.go`、`wire.go` 的 re-export

**Step 1:** 确认零引用
Run: `cd /mnt/d/project/Syntopica/backend-go && grep -rn "TopicGraphResponse\|TopicDetail\|GraphNode\|GraphEdge\|TopicArticleCard\|TopicsByCategoryResult\|PendingArticlesResponse\|RelatedTag\|TopicHistoryPoint" internal --include='*.go' | grep -v '_test.go'`
Expected: 仅出现在 tagmanagement 自身定义/ re-export 处。若被任何非 tagmanagement 代码引用 → **跳过本任务**（保留类型）。

**Step 2:** 若零外部引用，移除这些类型定义及对应 re-export

**Step 3:** 编译验证
Run: `cd /mnt/d/project/Syntopica/backend-go && go build ./... && go vet ./internal/tagmanagement/... && golangci-lint run ./internal/tagmanagement/...`
Expected: 成功

**Step 4:** Commit（若执行）
```bash
git add -A && git commit -m "refactor: drop orphaned TopicGraph* types from tagging package"
```

---

## Task 10: 最终全量验证

**Step 1:** 后端全量
Run: `cd /mnt/d/project/Syntopica/backend-go && golangci-lint run ./... && go vet ./... && go test ./... && go build ./...`
Expected: 全绿

**Step 2:** 前端全量（Windows cmd）
Run: `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm lint && pnpm exec nuxi typecheck && pnpm test:unit && pnpm build"`
Expected: 全绿

**Step 3:** 人工确认产品行为
- 侧栏不再有「主题图谱」入口；「标签管理」已变为「叙事工坊」。
- 进入叙事工坊，验证板块管理、日报、侦探墙、话题合并（TagMergePreview）均正常。
- 全局设置中的标签队列面板（TagQueuePanel）正常显示。

**Step 4:** 汇总验证报告，报告完成。

---

## 执行说明（给 subagent）

### 脏树保护（重要）
工作区存在用户的在途 WIP（`backend-go/cmd/dump-sanitizer/*`、`TopicDetectiveWall.client.vue`、`detective-wall/*`、`useDailyReportReader*`、`BoardDailyReportTimeline.vue`、`README.md` 等），**与本重构无关，严禁纳入本计划的 commit、严禁 stash/丢弃**。因此：
- **改用精确路径暂存**：每个任务只 `git add <本任务实际改动的具体文件>`，**禁止 `git add -A` / `git add .`**。提交前用 `git status` 确认 staged 列表里只有本任务文件。
- **Task 8（文档）的 README.md 跳过**：README.md 有用户在途改动，不要在本计划里碰它；Task 8 只清理 `docs/reference/*`、`docs/user-guide/*` 等无重叠的文档。README 里「主题图谱」描述的清理**留待用户自行合并**（在验证报告里标注）。
- 本计划涉及文件与用户 WIP 文件**无重叠**（已核对），故精确暂存即可干净隔离。

### 通用规则
- 按 executing-plans skill：加载本计划 → 逐任务执行 → 每任务跑指定 verify → 每任务 commit → 遇阻停下询问。
- **不要跳过任何 verify 命令。** 前端 typecheck/build/test 必须走 Windows cmd（WSL 缺 native binding）。
- 后端 Go 工具链：WSL 内若无 `go`，可用 `go.exe`（`/mnt/d/tool/Go/bin/go.exe`，已验证可直跑）；`golangci-lint` 走 `cmd.exe`。
- 删除前先 grep 确认引用已清，避免编译断裂后难定位。
- 后端 Task 7 切除 repository.go 方法时，以 `go build` 报错为导向清理 import 与孤立类型，这是正常的手术反馈。
- 测试只跑受影响包（topicgraph / tagmanagement / onboarding），不要随意跑全量直到 Task 10。
