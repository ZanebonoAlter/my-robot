# 前端代码设计规范审计报告

> **审计对象**: `front/app/`（Nuxt 4 + Vue 3 + TS，89 个 `.vue` + ~90 个 `.ts`）
> **对照规范**: `docs/reference/standard/frontend/{code-style,theming,interaction-conventions,testing}.md`
> **评级**: **B**（技术债治理后部分项已修复，评级维持 B——剩余的 BoardThreadBrowser 拆分、暗色面板 token 化、AppButton 迁移等大重构未做，是升 B+ 的关键）
> **审计日期**: 2026-07-23

## 技术债治理进度（2026-07-23）

| 治理项 | 状态 | 说明 |
| ------ | ---- | ---- |
| ArticleTagList 4 色硬编码（M6） | ✅ 已修复 | 替换为 `var(--color-tag-*)` + `--color-accent*`，删 `--raw-red-700` |
| URLSearchParams 手写 3 处（M8 同类） | ✅ 已修复 | `abstractTags.ts`/`scheduler.ts` 改用 `buildQueryString()` |
| BTB statusColorMap 5 色（M6） | ✅ 已修复 | 新增 `--color-thread-status-*` 双主题 token，statusFill 改用 var() |
| as unknown as 2 处（M8） | ⏳ 跳过 | 单点强转本质是 API 层类型声明与运行时不一致，unwrapResponse 不匹配，强行改有隐患 |
| BoardThreadBrowser 拆 composable（M7） | ⏳ 跳过 | 2456 行大重构，高风险，需测试驱动专项排期 |
| BTB 暗色面板块 token 化 | ⏳ 跳过 | 需重新选 token + 双主题目检，中风险 |
| AppButton 58 文件迁移（M10） | ⏳ 跳过 | 量大需人工逐个判别图标/文本按钮 |

**门禁结果**：lint 0 errors（5 warnings 既有）+ typecheck pass + test:unit 360 passed。

架构红线守得住，问题主要是**执行层面的局部回潮**。API 归一化彻底（零组件内 `fetch`）、Store 派生架构规范（无循环依赖）、`<script setup>` 100% 覆盖、通知分层正确——这些都是高成熟度的标志。技术债集中在 **tags 域巨型组件**（BoardThreadBrowser 2456 行）、**主题硬编码回潮**、**类型逃逸**和**核心 composable 测试盲区**。

## 正面发现（保持）

| 维度 | 结论 | 证据 |
| ---- | ---- | ---- |
| API 归一化彻底 | **零** 组件内 `fetch`/`$fetch`/`useFetch`，全走 `~/api` | HTTP 边界全收敛在 `api/client.ts` |
| Store 派生架构规范 | apiStore 主源、feedsStore 纯派生、articlesStore 经小 Interface 同步 | **无循环依赖**、无 syncToLocalStores 副本 |
| 写操作全持久化 + 乐观更新 | markAsRead/toggleFavorite/markAllAsRead 均 API 持久化 + 回滚 | `stores/articles.ts` |
| 通知分层正确 | `api/` 层**零** `useNotify`（符合 §6），单一责任层通知 | grep 验证 |
| `<script setup>` 100% 覆盖 | 89/89 组件，无 Options API 残留 | 全量扫描 |
| pages 纯挂载 | 业务逻辑全在 feature | pages/ 仅布局 |
| SSE 长任务规范 | scan/evaluate 经 API module 的 named adapter 暴露 | `useTagMergePreview` → `api.createEvaluateEventSource` |
| 主题三层架构完整 | editorial/dark 双主题定义齐全 | main.css |
| tags 域交互规范执行到位 | aria-live/role=alert/状态左侧位 | daily-report 系列质量高 |

---

## 问题清单

### 一、全局 / 跨 feature

#### [High] God 组件 BoardThreadBrowser.vue（2456 行）未拆 composable

- **位置**: `front/app/features/tags/components/BoardThreadBrowser.vue`
- **证据**（已主线程验证）:
  - 2456 行（`<script setup>` 765 行 + template 584 + style 1103）
  - 41 个函数、27 个 computed、12 个 ref
  - 4 种视图模式（timeline/lanes/focus/compose）+ popup/zoom/SVG 渲染全塞一个文件
  - **未使用任何 composable**（tags 域其他大组件都抽了 `useBoardEnrichment`/`useBoardTimeline`）
- **规范依据**: code-style §8（单文件 >500 行/~15KB 应拆；按内聚行为拆 composable）
- **建议**: 按视图拆 `useThreadTimeline` / `useThreadLanes` / `useThreadFocus` / `useThreadCompose` / `useThreadZoom`，style 拆到 `.css`

#### [Medium] 组件内 `as unknown as` 类型逃逸，未用 unwrapResponse

- **位置**: `stores/api.ts:129-130`、`stores/articles.ts:34`、`features/articles/composables/useArticlePagination.ts:65-66`、`features/shell/components/FeedLayoutShell.vue:161`、`features/tags/composables/useTagsPage.ts:70`
- **证据**: 共 7 处 `response.data as unknown as XxxPayload[]`
- **规范依据**: code-style §3（"API 模块中的 `as unknown as ApiResponse<T>` 应替换为 `mapApiResponse<T>()`"）+ §10 禁用 `any`/类型断言滥用
- **建议**: 统一改用 `utils/api-helpers.ts` 已提供的 `unwrapResponse<T>()` 解包

#### [Medium] AppButton 统一组件被大面积绕过

- **位置**: 约 50 个 `.vue` 用原生 `<button>`，仅 20 个用 `AppButton`
- **证据**: BoardThreadBrowser=16 个原生 button、ComposePanel=5、TopicManageDialog=3、DailyReportTopicSection=8
- **规范依据**: theming.md「按钮 → AppButton（必须复用，禁止各写一套）」
- **建议**: 非图标按钮迁移到 AppButton；纯图标/特例保留原生但加注释

---

### 二、tags 域（最大 feature — 整改重点）

#### [High] ArticleTagList.vue 重复实现已存在的 Layer 2 token

- **位置**: `front/app/features/articles/components/ArticleTagList.vue:124-144`
- **证据**（已主线程验证）:
  ```css
  /* 硬编码三色，而 main.css:136-144 已定义对应 token（含 dark 适配） */
  color: #9a5c00;   /* 应为 var(--color-tag-event) */
  color: #0d7a56;   /* 应为 var(--color-tag-person) */
  color: #234d66;   /* 应为 var(--color-tag-keyword) */
  color: var(--raw-red-700);  /* ← 直接用 Layer 1 原始色，违反三层架构 */
  ```
- **规范依据**: theming.md「组件只引用 Layer 2 语义 token，不直接使用原始色值」+「禁止各写一套」
- **建议**: 改为 `var(--color-tag-*)`；删除 `--raw-red-700` 改用 `--color-error`/`--color-accent`

#### [High] BoardThreadBrowser.vue 状态色硬编码，暗色模式失效

- **位置**: `features/tags/components/BoardThreadBrowser.vue:55-60`（statusColorMap）+ 多处 CSS
- **证据**（已主线程验证）:
  ```ts
  const statusColorMap: Record<string, string> = {
    emerging: '#34d399',
    continuing: '#60a5fa',
    ...
  }
  ```
  - CSS 用 `#fff/#e2e8f0/#93c5fd/#6ee7b7` 等浅色文字（1439/1470/1536/1788 行）
  - 这些浅色文字色在 editorial（亮）主题下几乎不可见；状态色未走主题 token
- **规范依据**: theming.md（颜色走主题变量，暗/亮都适配）
- **建议**: 状态色上提到 main.css 作 `--color-thread-status-*`（双主题），文字色全换 `--color-text-*`

#### [Medium] as unknown as + 大文件集中在 tags 数据流

- **位置**: `features/tags/composables/useTagsPage.ts:70`；`useBoardEnrichment.ts`(592行) / `api/boardEnrichment.ts`(658行)
- **建议**: useBoardEnrichment 接近 500 行红线，按 table（lifeline/context/data-source/debate）拆分

#### [Low] TopicDetectiveWall.client.vue 1536 行

- **位置**: `features/tags/components/TopicDetectiveWall.client.vue`
- **建议**: 场景装配逻辑下沉到 `detective-wall/` 子模块（已部分拆出 CardGroup/RedString，继续收敛）

---

### 三、ai 域

#### [Medium] inject context 接口大面积用 any，丢失已存在的类型

- **位置**: `features/ai/components/AIRouterBackupProviders.vue:7-18`、`AIRouterCapabilityRoutes.vue:11`
- **证据**: `AIRouterCtx` 中 `backupProviders: any[]`、`newProviderForm: any`、`editProviderForm: any`、`startEditingProvider: (p: any)=>void`、`deleteBackupProvider: (p: any)=>void`
- **对比**: `useAIRouterSettings.ts:39` 实际定义为 `reactive<AIProviderUpsertRequest>`
- **规范依据**: code-style §10（禁 any）
- **建议**: ctx 接口字段改成 `AIProviderUpsertRequest` / `AIProvider[]`，从 composable 导出共享类型

---

### 四、shell 域

#### [Medium] FeedLayoutShell.vue（569行）壳内做数据 hydrate

- **位置**: `features/shell/components/FeedLayoutShell.vue`
- **证据**: 直接 import `useArticlesApi`、`useWatchedTagsApi`、`normalizeArticle`、`useApiStore`，在壳里做数据 hydrate（line 161 `normalizeArticle(response.data as unknown as ArticlePayload)`）
- **规范依据**: code-style §2（shell 壳定位）+ §3（组件内不做数据映射）
- **建议**: 文章/订阅源 hydrate 逻辑移入 `useArticlePagination`，壳只负责布局

#### [Low] ArticleContentPreviewPanel.vue 33 个 props（prop-drilling）

- **位置**: `features/articles/components/ArticleContentPreviewPanel.vue:12-46`
- **证据**: defineProps 33 个字段（含 6 个 handler、多个 `*Loading`/`*Error`/`*Label`/`*show*` 布尔）
- **规范依据**: code-style §8（props 过多职责过重）
- **建议**: 文章处理状态聚合成 `provide/inject` 的 `useArticleProcessing` composable

---

### 五、settings / preferences / feeds 域

#### [Low] 多个全局 composable 实为单域私有，放错位置

- **位置**:
  - `composables/useDailyReportProgress.ts`（仅 tags 的 NarrativeGenerateDialog 用）
  - `composables/useFirecrawlConfig.ts`（仅 settings 用）
  - `composables/useSchedulerStatus.ts`（仅 settings 用）
  - `composables/useReadingPreferences.ts`（仅 preferences 用）
- **证据**: grep 显示各 composable 仅 1 个消费者
- **规范依据**: code-style §2（业务 composable 归 `features/*/composables/`，全局 composable 才放 `composables/`）
- **建议**: 迁到对应 feature；`useAI` 跨域共享可留全局

#### [Low] components/dialog/ 混放各域业务对话框

- **位置**: `components/dialog/`
- **证据**: TopicManageDialog→tags、FeedSettingsPanel/AddFeedDialog→feeds、FirecrawlConfigPanel/SchedulerStatusPanel→settings、ReadingPreferencesPanel→preferences、GlobalSettingsDialog→shell 混放
- **规范依据**: code-style §2（业务组件归 `features/*/components/`）
- **建议**: 域私有对话框迁回各 feature；`components/dialog/` 只留真正跨域的

---

### 六、API 层

#### [Medium] 手写 URLSearchParams，未用 buildQueryString

- **位置**: `api/abstractTags.ts:46,77`、`api/scheduler.ts:6`
- **证据**: `const params = new URLSearchParams(); params.set(...)`
- **对比**: `utils/api-helpers.ts:34` 已有 `buildQueryString()`，`api/client.ts`/`createQueueApi.ts` 已正确使用
- **规范依据**: code-style §3（禁止手写 URLSearchParams）
- **建议**: 替换为 `buildQueryString({ category, feedId, ... })`

---

### 七、测试覆盖

#### [Medium] 核心 store / composable 无单测

- **位置**:
  - `stores/articles.ts`、`stores/feeds.ts`、`stores/preferences.ts`（仅 `stores/api.test.ts` 有覆盖）
  - `useEventStream`（核心 WS 基建）、`useNotify`、`useTheme`、`useArticleContentView`、`useTagMergePreview`、`useBoardEnrichment`、`useBoardCRUD`、`useAutoRefresh` 均无 `*.test.ts`
- **规范依据**: testing.md（核心 composable/util/api 应有单测）
- **建议**: 优先补 `useEventStream`（重连/订阅清理）、`articles.ts`（乐观更新回滚）、`useTagMergePreview`（SSE 状态机）

---

### 八、交互 / 可访问性

#### [Medium] 多组件有 loading 无 error 状态

- **位置**: ArticlePreviewModal.vue、AuxiliaryLabelPicker.vue、MatchingConfigDialog.vue、FeedSettingsPanel.vue、BoardTimelinePanel.vue、BackfillProgress.vue 等 11 个
- **规范依据**: interaction-conventions（loading/error/empty 三态齐全）
- **建议**: 补 error 分支 + `role="alert"`（tags 域 daily-report 系列已有良好实践可作模板）

#### [Low] preferences.ts computed 内原地 sort 污染源数组

- **位置**: `stores/preferences.ts:22-32`（`topFeeds`/`topCategories`）
- **证据**: `feedPreferences.value.sort(...)` 直接变异派生源
- **建议**: `[...feedPreferences.value].sort(...)`

---

## 前端总评：B（良好，有明确技术债但架构骨架健康）

**主要技术债（拉低评级）**：
1. **tags 域巨型组件**：BoardThreadBrowser.vue 2456 行无 composable，是全项目最大债务
2. **主题硬编码回潮**：多处绕过 Layer 2 token（ArticleTagList 重复造色、BoardThreadBrowser 状态色硬编码且暗色失效、3 处 `--raw-*` 直用）
3. **类型逃逸**：7 处 `as unknown as` 未用现成的 unwrapResponse；ai ctx 接口 6 个 any
4. **测试盲区**：核心 store/composable（尤其 useEventStream 基建）无单测
5. **AppButton 统一组件被大面积绕过**

**结论**：架构红线（API 边界、Store 派生、事件流、通知分层）守得住，问题主要是执行层面的局部回潮。**tags 域是整改重点**——BoardThreadBrowser 拆分 + 主题 token 治理 + useBoardEnrichment 拆分三项做完，前端可升至 B+。
