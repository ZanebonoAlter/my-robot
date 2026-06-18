## 0. 前置条件检查（§5.0）

apply 启动时先确认前端基础设施就绪（缺失则作为首个子任务补齐）：

- [x] 0.1 `cd front && pnpm lint` 可执行（`front/eslint.config.js` 已存在）
- [x] 0.2 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"` 可正常执行（0 错误）
- [x] 0.3 `cd front && pnpm test:unit` 可正常执行（Vitest + happy-dom）
- [x] 0.4 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"` 可正常编译

## 1. Install driver.js

- [x] 1.1 Run `pnpm add driver.js` in `front/` to add the guided-tour library（实测安装 `1.4.0`，package.json 记 `^1.4.0`）

## 2. Create useOnboarding composable

- [x] 2.1 Create `front/app/composables/useOnboarding.ts` with composable interface: `isFirstRun`, `isTourActive`, `startTour()`, `dismissTour()`, `resetOnboarding()`（feature-tip 方法已随 D6 推迟移除）
- [x] 2.2 Implement first-run detection: check `localStorage.getItem('syntopica_onboarding_complete')` to determine `isFirstRun`
- [x] 2.3 Implement tour initialization: `startTour()` 内 `if (!import.meta.client) return` 守卫 → `await nextTick()` → `document.querySelector` 预检过滤掉 `element` 选择器查不到的 step（driver.js v1 原生不跳过缺失元素，见 design.md D2）→ 调用 `driver({...})` 创建实例并 `.drive()`；5 个 step 用 `data-onboarding` 选择器定义：welcome overlay / `sidebar-feeds` / `nav-topic-graph` / `nav-tags` / `watched-tags`（与 spec.md "Guided tour steps" 一致，见 §10.5）
- [x] 2.4 Implement `dismissTour()`: set `syntopica_onboarding_complete` to `"true"`, destroy driver instance
- [x] 2.6 Implement `resetOnboarding()`: clear `syntopica_onboarding_complete` key, call `location.reload()` to re-trigger tour

### 2b. 多 tour 重构与标签引导（见 design.md D8）

- [x] 2b.1 Refactor `useOnboarding.ts` 为多 tour 架构：抽出 `preFilterSteps(steps)` 与 `runTour(id, steps)` 通用逻辑，driver 实例管理 / `prefers-reduced-motion` 检测 / 缺失元素预检过滤复用
- [x] 2b.2 新增 `startTagsTour()` + `isTagsFirstRun`，5 个 tags step 用 `data-onboarding` 选择器定义：welcome overlay / `tags-board-list` / `tags-content-tabs` / `tags-board-actions` / `tags-add-board`（与 spec.md "Tags page guided tour" 一致）
- [x] 2b.4 新增 `startSettingsTour()` + `isSettingsFirstRun`，5 个 settings step：welcome overlay / `settings-nav` / `settings-nav-feeds` / `settings-nav-ai-providers` / `settings-nav-schedulers`（与 spec.md "Settings page guided tour" 一致）
- [x] 2b.3 独立完成标记键：`syntopica_onboarding_complete`（home）/ `syntopica_onboarding_tags_complete`（tags）/ `syntopica_onboarding_settings_complete`（settings），互不影响

## 3. Add data-onboarding attributes to tour targets

- [x] 3.1 Add `data-onboarding="sidebar-feeds"` to the `.categories` **container div**（`AppSidebarView.vue` line 226，`v-if="!sidebarCollapsed"`）— 首次访问无 feed 无分类时该容器仍渲染（v-for 空数组），但内部 category-group / uncategorized 子项不渲染，**禁止挂子项**（否则首次访问 querySelector 落空，见 design.md D2）
- [x] 3.2 Add `data-onboarding="nav-topic-graph"` to the topic graph `<button class="sidebar-item">`（line 177, `handleTopicGraphClick`，无 v-if 始终渲染）in `front/app/features/shell/components/AppSidebarView.vue`
- [x] 3.3 Add `data-onboarding="nav-tags"` to the tag management `<button class="sidebar-item">`（line 182, `navigateTo('/tags')`，无 v-if 始终渲染）in `front/app/features/shell/components/AppSidebarView.vue`
- [x] 3.4 Add `data-onboarding="watched-tags"` to the `.watched-tags-header` **title container**（`AppSidebarView.vue` line 190-192 附近，`v-if="!sidebarCollapsed"` 下始终渲染；勿挂到 `v-if="watchedTags.length > 0"` 内部标签列表）in `front/app/features/shell/components/AppSidebarView.vue`
- [x] 3.5 确认侧边栏首次访问默认展开状态（`FeedLayoutShell.vue:39 sidebarCollapsed = ref(false)` 实测为默认展开）；当前 `ssr: false`（SPA）无 SSR 风险。折叠态下 `.categories`(L226) / `.watched-tags-section`(L189) 被 `v-if` 隐藏 → step 2/5 锚点消失，但 line 177/182 导航 button 无 v-if 仍在 DOM；折叠态 step 2/5 走 §2.3 预检过滤的 "skip missing DOM" 路径（见 design.md Risks）

### 3b. 标签管理页引导锚点（见 design.md D8）

- [x] 3b.1 Add `data-onboarding="tags-board-list"` to the `.sb-list` container in `front/app/features/tags/components/BoardListSidebar.vue`（语义板块列表容器，始终渲染）
- [x] 3b.2 Add `data-onboarding="tags-board-actions"` to the `.sb-actions` container in `BoardListSidebar.vue`（核心操作按钮组）
- [x] 3b.3 Add `data-onboarding="tags-add-board"` to the `.sb-action-btn--primary` button in `BoardListSidebar.vue`（添加板块主按钮）
- [x] 3b.4 Add `data-onboarding="tags-content-tabs"` to the `.tags-content-tabs` container in `front/app/features/tags/components/TagsPage.vue`（板块内容/日报/文章三 Tab）

### 3c. 设置页引导锚点（见 design.md D9）

- [x] 3c.1 Add `data-onboarding="settings-nav"` to the `.settings-sidebar__nav` container in `front/app/features/settings/components/SettingsSidebar.vue`（桌面侧栏导航容器）
- [x] 3c.2 Add `:data-onboarding="\`settings-nav-${section.key}\`"` to the `v-for` `settings-sidebar__item` button in `SettingsSidebar.vue`（动态属性，每个 section 生成 `settings-nav-feeds` / `settings-nav-ai-providers` / `settings-nav-schedulers` 等锚点）

## 4. Empty state components

- [x] 4.1 Create `front/app/features/feeds/components/FeedEmptyGuide.vue` — shown when no feeds and no categories exist, with "添加你的第一个 RSS 源" message and action button（已挂载到 `FeedLayoutShell.vue` `.content-panel`，`!hasAnyFeedsOrCategories` 时显示，`@add` 打开添加订阅源对话框）
- [x] 4.2 Create `front/app/features/topic-graph/components/TopicGraphEmptyGuide.vue` — shown when graph has no data, with "等待标签数据积累" explanation（已挂载到 `TopicGraphPage.vue` canvas 区，`!loadingGraph && graphPayload && !graphHasData`（请求成功但无标签）时显示，取代 canvas）
- [x] 4.3 Enhance the internal `drt-empty` empty state in `front/app/features/tags/components/BoardDailyReportTimeline.vue` (line 379-381, `reports.length === 0` 分支): 改造现有"暂无日报"文案为引导性内容（"日报需要积累数据" + 说明），**不新建独立组件**（见 design.md D5 日报例外）

## 5. Feature discovery tooltips（推迟到后续 change，本 change 无任务）

> 见 design.md D6。功能发现提示（feature tips）与现有 `AppTooltip` hover 模式不匹配，推迟到后续独立 change 评估。本节无实现任务，编号保留以维持后续节号稳定。

## 6. Settings replay button

- [x] 6.1 Add a "新手引导" icon button to `front/app/features/shell/components/AppHeaderView.vue` top toolbar (`.header-right`)，与主题切换同一层级，调用 `useOnboarding().startTour()` 启动引导（决策反转：原 D7 草案放 `SettingsSectionPreferences`，审查后改为顶部工具栏高可发现入口，见 design.md D7）

### 6b. 标签管理页引导入口与自动启动（见 design.md D8）

- [x] 6b.1 Add a guide icon button (`mdi:compass-outline`) to `TagsPage.vue` top toolbar（`ThemeToggle` 旁，同一层级），调用 `useOnboarding().startTagsTour()` 手动唤起标签引导
- [x] 6b.2 `TagsPage.vue` `onMounted` 检测 `isTagsFirstRun` 自动启动 `startTagsTour()`（首次访问自动弹，与首页一致）

### 6c. 设置页引导入口与自动启动（见 design.md D9）

- [x] 6c.1 Add a guide icon button (`mdi:compass-outline`) to `SettingsWorkspace.vue` header（主题切换按钮旁，同一层级），调用 `useOnboarding().startSettingsTour()` 手动唤起设置引导
- [x] 6c.2 `SettingsWorkspace.vue` `onMounted` 检测 `isSettingsFirstRun` 自动启动 `startSettingsTour()`（首次访问自动弹，与首页/标签管理一致）

## 7. prefers-reduced-motion support

- [x] 7.1 In `useOnboarding.ts`, check `window.matchMedia('(prefers-reduced-motion: reduce)')` and pass `animate: false` to driver.js config when the user prefers reduced motion

## 8. 测试

- [x] 8.1 `useOnboarding` composable 单元测试（`front/app/composables/useOnboarding.test.ts`，与源文件同目录）：覆盖 first-run 检测（`syntopica_onboarding_complete` 缺失/存在）、`dismissTour()` 写 localStorage、`resetOnboarding()` 清键、client-guard（mock `import.meta.client = false` 时 `startTour()` 不访问 localStorage / 不创建 driver 实例，`isFirstRun`/`isTourActive` 默认 false；注意当前 `ssr: false`，此用例为防御性验证守卫生效）；并覆盖缺失元素预检过滤（某 `data-onboarding` 选择器 querySelector 返回 null 时，该 step 不进入 driver steps 数组）
- [x] 8.1b 标签引导测试：`startTagsTour()` 运行 tags steps、`isTagsFirstRun` 检测、tags 独立键 `syntopica_onboarding_tags_complete`（tags tour onDestroyed 不写 home 键）、缺失元素预检过滤
- [x] 8.1c 设置引导测试：`startSettingsTour()` 运行 settings steps、`isSettingsFirstRun` 检测、settings 独立键 `syntopica_onboarding_settings_complete`（settings tour onDestroyed 不写 home/tags 键）、缺失元素预检过滤
- [x] 8.2 运行：`cd front && pnpm test:unit` → PASS（112/112，含 17 个 useOnboarding 用例）

## 9. 文档

- [x] 9.1 本 change 为纯前端（无后端改动），`docs/reference/` 若有前端组件清单 / onboarding 条目则同步更新；否则在 milestone changes 归档说明中注明无 reference 结构性变更
- [x] 9.2 `openspec show user-onboarding` 确认 spec.md 与 tasks.md 选择器一致（见 §10.5）

## 10. 验证（归档前重跑，每条必须实测零失败）

- [x] 10.1 `cd front && pnpm lint` → 0 error / 0 new warning
- [x] 10.2 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"` → 0 error
- [x] 10.3 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit"` → PASS
- [x] 10.4 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"` → 构建成功
- [x] 10.5 选择器一致性校验：`grep -rn "data-onboarding" front/app/` → 命中 11 个值：home tour `sidebar-feeds` / `nav-topic-graph` / `nav-tags` / `watched-tags`；tags tour `tags-board-list` / `tags-content-tabs` / `tags-board-actions` / `tags-add-board`；settings tour `settings-nav` / `settings-nav-feeds` / `settings-nav-ai-providers` / `settings-nav-schedulers`（`settings-nav-${key}` 动态属性含其他 section 值），与 spec.md 三个 tour requirement 完全一致
- [x] 10.6 路径一致性校验：`grep -rn "data-onboarding" front/app/features/shell/components/AppSidebarView.vue` → 4 个挂载点均在此文件，无遗留 `components/app/sidebar/` 幽灵路径
- [x] 10.7 依赖校验：`grep '"driver.js"' front/package.json` → 版本为 `^1.4.0`（非 "v5+"）
- [ ] 10.8 手动验证（不计入归档门禁，仅记录）：清 localStorage `syntopica_onboarding_complete` → 刷新 → 5 步引导启动并完成 → settings 点"重播引导" → 引导重启（需人工浏览器验证，本次 apply 未执行）
- [x] 10.9 feature-tip 清除校验：`grep -rn 'data-feature-tip\|syntopica_feature_tips\|isFeatureTipShown\|markFeatureTipShown' front/app/` → 零命中（feature tips 已推迟到后续 change，前端代码无残留）
