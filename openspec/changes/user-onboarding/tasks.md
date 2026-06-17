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

## 3. Add data-onboarding attributes to tour targets

- [x] 3.1 Add `data-onboarding="sidebar-feeds"` to the `.categories` **container div**（`AppSidebarView.vue` line 226，`v-if="!sidebarCollapsed"`）— 首次访问无 feed 无分类时该容器仍渲染（v-for 空数组），但内部 category-group / uncategorized 子项不渲染，**禁止挂子项**（否则首次访问 querySelector 落空，见 design.md D2）
- [x] 3.2 Add `data-onboarding="nav-topic-graph"` to the topic graph `<button class="sidebar-item">`（line 177, `handleTopicGraphClick`，无 v-if 始终渲染）in `front/app/features/shell/components/AppSidebarView.vue`
- [x] 3.3 Add `data-onboarding="nav-tags"` to the tag management `<button class="sidebar-item">`（line 182, `navigateTo('/tags')`，无 v-if 始终渲染）in `front/app/features/shell/components/AppSidebarView.vue`
- [x] 3.4 Add `data-onboarding="watched-tags"` to the `.watched-tags-header` **title container**（`AppSidebarView.vue` line 190-192 附近，`v-if="!sidebarCollapsed"` 下始终渲染；勿挂到 `v-if="watchedTags.length > 0"` 内部标签列表）in `front/app/features/shell/components/AppSidebarView.vue`
- [x] 3.5 确认侧边栏首次访问默认展开状态（`FeedLayoutShell.vue:39 sidebarCollapsed = ref(false)` 实测为默认展开）；当前 `ssr: false`（SPA）无 SSR 风险。折叠态下 `.categories`(L226) / `.watched-tags-section`(L189) 被 `v-if` 隐藏 → step 2/5 锚点消失，但 line 177/182 导航 button 无 v-if 仍在 DOM；折叠态 step 2/5 走 §2.3 预检过滤的 "skip missing DOM" 路径（见 design.md Risks）

## 4. Empty state components

- [x] 4.1 Create `front/app/features/feeds/components/FeedEmptyGuide.vue` — shown when no feeds and no categories exist, with "添加你的第一个 RSS 源" message and action button
- [x] 4.2 Create `front/app/features/topic-graph/components/TopicGraphEmptyGuide.vue` — shown when graph has no data, with "等待标签数据积累" explanation
- [x] 4.3 Enhance the internal `drt-empty` empty state in `front/app/features/tags/components/BoardDailyReportTimeline.vue` (line 379-381, `reports.length === 0` 分支): 改造现有"暂无日报"文案为引导性内容（"日报需要积累数据" + 说明），**不新建独立组件**（见 design.md D5 日报例外）

## 5. Feature discovery tooltips（推迟到后续 change，本 change 无任务）

> 见 design.md D6。功能发现提示（feature tips）与现有 `AppTooltip` hover 模式不匹配，推迟到后续独立 change 评估。本节无实现任务，编号保留以维持后续节号稳定。

## 6. Settings replay button

- [x] 6.1 Add a "重播引导" button to `front/app/features/settings/components/SettingsSectionPreferences.vue` that calls `useOnboarding().resetOnboarding()` to re-trigger the onboarding tour

## 7. prefers-reduced-motion support

- [x] 7.1 In `useOnboarding.ts`, check `window.matchMedia('(prefers-reduced-motion: reduce)')` and pass `animate: false` to driver.js config when the user prefers reduced motion

## 8. 测试

- [x] 8.1 `useOnboarding` composable 单元测试（`front/app/composables/useOnboarding.test.ts`，与源文件同目录）：覆盖 first-run 检测（`syntopica_onboarding_complete` 缺失/存在）、`dismissTour()` 写 localStorage、`resetOnboarding()` 清键、client-guard（mock `import.meta.client = false` 时 `startTour()` 不访问 localStorage / 不创建 driver 实例，`isFirstRun`/`isTourActive` 默认 false；注意当前 `ssr: false`，此用例为防御性验证守卫生效）；并覆盖缺失元素预检过滤（某 `data-onboarding` 选择器 querySelector 返回 null 时，该 step 不进入 driver steps 数组）
- [x] 8.2 运行：`cd front && pnpm test:unit -- useOnboarding` → PASS

## 9. 文档

- [x] 9.1 本 change 为纯前端（无后端改动），`docs/reference/` 若有前端组件清单 / onboarding 条目则同步更新；否则在 milestone changes 归档说明中注明无 reference 结构性变更
- [x] 9.2 `openspec show user-onboarding` 确认 spec.md 与 tasks.md 选择器一致（见 §10.5）

## 10. 验证（归档前重跑，每条必须实测零失败）

- [x] 10.1 `cd front && pnpm lint` → 0 error / 0 new warning
- [x] 10.2 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"` → 0 error
- [x] 10.3 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit"` → PASS
- [x] 10.4 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"` → 构建成功
- [x] 10.5 选择器一致性校验：`grep -rn "data-onboarding" front/app/` → 命中 4 个值 `sidebar-feeds` / `nav-topic-graph` / `nav-tags` / `watched-tags`，与 spec.md "Guided tour steps" requirement 完全一致（无 `topic-graph-nav` / `tag-management-nav` 旧名残留）
- [x] 10.6 路径一致性校验：`grep -rn "data-onboarding" front/app/features/shell/components/AppSidebarView.vue` → 4 个挂载点均在此文件，无遗留 `components/app/sidebar/` 幽灵路径
- [x] 10.7 依赖校验：`grep '"driver.js"' front/package.json` → 版本为 `^1.4.0`（非 "v5+"）
- [ ] 10.8 手动验证（不计入归档门禁，仅记录）：清 localStorage `syntopica_onboarding_complete` → 刷新 → 5 步引导启动并完成 → settings 点"重播引导" → 引导重启（需人工浏览器验证，本次 apply 未执行）
- [x] 10.9 feature-tip 清除校验：`grep -rn 'data-feature-tip\|syntopica_feature_tips\|isFeatureTipShown\|markFeatureTipShown' front/app/` → 零命中（feature tips 已推迟到后续 change，前端代码无残留）
