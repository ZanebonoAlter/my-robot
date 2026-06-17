## Context

Syntopica 是一个复杂的信息聚合产品，包含 RSS 订阅管理、AI 自动标签、话题图谱可视化、每日热报、语义版块等多个子系统。目前产品没有任何用户引导机制——首次使用的用户直接面对完整的应用界面，没有引导流程、功能提示或空状态说明。

当前状态：
- 前端没有任何 onboarding 相关代码（无 composable、组件或状态管理）
- 没有 localStorage 中的首次访问标记
- 空状态下无引导性内容（空 feed 列表、空日报、空图谱均只显示空白）
- 没有功能发现提示（tooltip/popover）
- 后端不参与此变更，所有逻辑纯前端

关键页面和入口：
- 首页 `pages/index.vue`：RSS 文章阅读（侧边栏 + 文章列表 + 内容面板）
- 话题图谱 `pages/topics.vue`：标签关系图可视化 + 时间线
- 标签管理 `pages/tags.vue`：标签列表 + 语义版块管理
- 侧边栏 `front/app/features/shell/components/AppSidebarView.vue`（外层 `AppSidebarShell.vue` 仅 `<AppSidebarView v-bind="$attrs" />` 转发）：主导航。导航项均为 `<button class="sidebar-item">`，通过 `emit('topicGraphClick')` / `navigateTo('/tags')` 触发跳转，**非 `<NuxtLink>`**；`data-onboarding` 属性挂在这些 button 上。关注标签区**标题容器始终渲染**（line 190-192），仅内部标签列表 `v-if="watchedTags.length > 0"`

技术栈约束：
- Vue 3 + Nuxt 4，TypeScript
- Tailwind CSS v4
- Pinia 状态管理
- 无后端改动
- **SSR 状态**：`front/nuxt.config.ts` 为 `ssr: false`（纯 SPA / CSR）。因此 `document` / `window` / `localStorage` 在所有执行点（含 composable 顶层、`onMounted`）均可用，**无 server bundle、无水合阶段**。下文凡提及 "SSR-safe" / "客户端守卫" 均指**防御性预留**（便于将来切换 `ssr: true` 与 Vitest 下行为可预测），并非当前存在 SSR 风险。

## Goals / Non-Goals

**Goals:**
- 首次访问时自动启动分步引导流程，带领用户完成核心操作路径
- 各功能区域在无数据时显示引导性空状态
- 引导可随时跳过，并可在设置中重新触发
- 尊重 prefers-reduced-motion 媒体查询

**Non-Goals:**
- 不做用户账号系统或服务端偏好持久化（纯 localStorage）
- 不做交互式教学（用户不需要在引导中实际操作）
- 不做 A/B 测试或多变体引导
- 不做引导步骤的自定义或跳转到特定步骤
- 不做引导完成率的埋点统计
- 不修改后端 API
- **不做功能发现提示（feature tips）**——原计划纳入，审查后推迟到后续 change：现有 `AppTooltip.vue` 是 hover-triggered（`@mouseenter`/`@mouseleave`），而 feature tip 需求是 onMount 自动显示一次，交互模式不匹配，需独立设计，不宜塞进本 change

## Decisions

### D1: 引导库选择 driver.js

**选择**：使用 [driver.js](https://driverjs.com/)（`^1.4.0`，`npm view driver.js version` 实测；注意 driver.js 历史版本为 0.x → 1.x，**无 v5**，早期 design 草稿的 "v5+" 为笔误）作为分步引导引擎。

**理由**：
- 轻量（~8KB gzipped），无 jQuery 依赖，框架无关
- 原生支持 highlight + popover + 键盘导航
- 支持程序化步骤定义，可通过 Vue composable 完全控制
- 活跃维护，TypeScript 类型完善
- 支持 `animate` 配置项，可在 prefers-reduced-motion 下禁用动画

**替代方案**：
- intro.js：功能类似但体积更大（~30KB），商业许可要求
- 自定义实现：开发成本高，需要自行处理 overlay、键盘导航、焦点管理、scroll-into-view 等
- vue-tour：仅支持 Vue 2，不活跃

### D2: 引导步骤定义

**选择**：5 步引导流程，覆盖核心操作路径：

| 步骤 | 目标元素 | 说明 |
|------|----------|------|
| 1 | 欢迎弹窗（无特定元素） | "欢迎使用 Syntopica" 简介 |
| 2 | 侧边栏 Feed 分类区域 | "从这里添加 RSS 源和分类" |
| 3 | 话题图谱导航按钮 | "话题图谱可视化标签关系" |
| 4 | 标签管理导航按钮 | "标签管理查看 AI 自动标签" |
| 5 | 日报/关注标签区域 | "关注标签获取个性化推送" |

步骤使用 CSS 选择器定位元素（`data-onboarding` 属性），不依赖组件内部结构。

**锚点挂点约束（实现必须遵守）**：
- `sidebar-feeds`：必须挂在 `AppSidebarView.vue` 的 `.categories` **容器 div**（line 226，`v-if="!sidebarCollapsed"`）。首次访问用户无 feed 无分类，`feedsStore.categories` 为空数组时该容器仍渲染（v-for 空数组 → 空容器），但内部 category-group / uncategorized 子项均不渲染。**禁止挂到任何 `v-for` 子项上**，否则 `querySelector` 在首次访问时必然落空。
- `nav-topic-graph` / `nav-tags`：挂在 line 177 / 182 的 `<button class="sidebar-item">` 上，这两个 button **无 v-if、始终渲染**（折叠态仅隐藏内部 `<span>`，button 本体在 DOM 中），最稳定。
- `watched-tags`：挂在 line 190-192 的 `.watched-tags-header` **标题容器**（`v-if="!sidebarCollapsed"` 始终成立时渲染），**不要**挂到 `v-if="watchedTags.length > 0"` 内部标签列表上。
- 欢迎弹窗（step 1）：不依赖任何 DOM 元素。

**driver.js v1 不自动跳过缺失元素步骤**：经 [issue #489](https://github.com/kamranahmedse/driver.js/issues/489) / [#279](https://github.com/kamranahmedse/driver.js/issues/279) 确认，driver.js v1 当 step 的 `element` 选择器查不到时，**不会**自动跳到下一步，而是渲染一个屏幕居中的无高亮 popover（观感像 bug）。因此 spec.md "Tour step skips missing DOM element" 需求由 composable 自行实现：`startTour()` 内 `await nextTick()` 后，用 `document.querySelector` 预检每个带 `element` 的 step，过滤掉缺失项，再把过滤后的 steps 数组传给 `driver()`。

**理由**：引导不是交互式教学（Non-Goal），而是让用户知道各功能在哪里。5 步足够覆盖核心入口，不会造成认知过载。

### D3: 首次运行检测与持久化

**选择**：localStorage 存储一个键：

- `syntopica_onboarding_complete`：`"true"` 表示引导已完成（包括主动跳过）

（原计划的 `syntopica_feature_tips` 键随 feature tips 推迟移除，见 D6）

**理由**：
- localStorage 同步读取，不影响首屏加载
- 简单的布尔 key-value，无需序列化复杂对象
- 用户清除浏览器数据可自然重置引导

**替代方案**：
- Pinia + persistedstate 插件：需要引入额外依赖，且 onboarding 状态不需要跨标签页同步
- 后端存储：违反纯前端原则，增加 API 复杂度

### D4: useOnboarding composable 设计

**选择**：创建 `front/app/composables/useOnboarding.ts`，提供：

```typescript
interface UseOnboarding {
  isFirstRun: ComputedRef<boolean>
  isTourActive: ComputedRef<boolean>
  startTour(): void
  dismissTour(): void
  resetOnboarding(): void  // 用于设置页重新触发
}
```

composable 内部管理 driver.js 实例生命周期，提供响应式状态给 Vue 组件消费。

**driver.js 初始化策略（选定方案 A）**：在 `useOnboarding.ts` 顶层 `import { driver } from 'driver.js'`（import 阶段不触碰 `document`，仅引入工厂函数），在 `startTour()` 内 `await nextTick()` + `querySelector` 预检后调用 `driver({...})` 创建实例。当前 `ssr: false` 下顶层 import 即安全；保留 `if (!import.meta.client) return` 守卫作为防御性预留（将来切 `ssr: true` 与 Vitest 行为可预测）。不采用 `.client.ts` plugin 注入（onboarding 非全局服务）或动态 `import('driver.js')`（多一层 async，收益不抵复杂度）。

**理由**：单一 composable 封装所有 onboarding 逻辑，组件只需调用接口。driver.js 实例在 composable 内创建和销毁，避免全局污染。

### D5: 空状态引导组件

**选择**：在各功能区域创建独立的空状态组件，不使用统一组件：

| 组件 | 位置 | 显示条件 | 内容 |
|------|------|----------|------|
| `FeedEmptyGuide.vue` | `features/feeds/components/` | 无 Feed 且无分类 | "添加你的第一个 RSS 源" + 操作按钮 |
| `TopicGraphEmptyGuide.vue` | `features/topic-graph/components/` | 图谱无数据 | "等待标签数据积累" + 说明 |
| （增强 `BoardDailyReportTimeline.vue` 内部 `drt-empty`，不新建独立组件） | `features/tags/components/BoardDailyReportTimeline.vue` line 379-381 | 日报列表为空（`reports.length === 0`，组件已有该判定） | "日报需要积累数据" + 说明（改造现有 `drt-empty` 文案与样式） |

每个组件独立负责自己的空状态判断逻辑和视觉呈现。

**理由**：各功能区域的数据模型和操作差异大，统一组件会增加 props 复杂度。独立组件可复用各自的 composable。

**日报例外**：`BoardDailyReportTimeline.vue` 内部已有 `drt-empty` 空状态（line 379-381），且日报是 board-scoped（`defineProps<{ boardId: number }>()`），页面级独立组件语义不通，故改造现有内部空状态而非新建。

### D6: 功能发现提示（feature tips）推迟到后续 change

**选择**：本 change **不实现** feature tips（功能发现提示）。原计划的 `data-feature-tip` 属性、`syntopica_feature_tips` localStorage 键、自造 tooltip 系统全部移出本 change 范围。

**理由**：审查发现现有 `front/app/components/common/AppTooltip.vue` 是 hover-triggered（`@mouseenter`/`@mouseleave`，基于 `@floating-ui/vue`），与 feature tip 的 "onMount 自动显示一次后记录不再弹" 交互模式根本不匹配，不能直接复用。若保留需自造 onMount-show-once 组件或改造 driver.js popover，是独立的子系统。为保持本 change 聚焦（引导流 + 空状态 + 重播入口），推迟到后续独立 change 评估。`useOnboarding` composable 相应移除 `isFeatureTipShown` / `markFeatureTipShown` 两个方法。

### D7: 重新触发引导的入口

**选择**：在 `front/app/features/settings/components/SettingsSectionPreferences.vue`（偏好设置区）添加"重播引导"按钮，调用 `useOnboarding().resetOnboarding()` 后自动刷新页面。

**理由**：`SettingsSectionPreferences` 语义最贴合（偏好类设置），`pages/settings.vue` 是持久入口（非 `GlobalSettingsDialog` 弹窗、非 header 临时菜单），重置策略简单不增加路由状态管理复杂度。

## Risks / Trade-offs

- **[引导元素未渲染]** → 步骤依赖的 DOM 元素可能未挂载（如侧边栏折叠时 `.categories` / `.watched-tags-section` 被 `v-if="!sidebarCollapsed"` 隐藏，或分类为空时 Feed 子项不存在）。缓解：`startTour()` 内 `await nextTick()` + `document.querySelector` 预检过滤缺失步骤（driver.js v1 原生不跳过缺失元素，见 D2）；欢迎弹窗不依赖任何 DOM 元素；锚点严格按 D2 挂点约束挂到始终渲染的容器。
- **[driver.js v1 不自动跳过缺失步骤]** → 见 D2。若不实现预检过滤，缺失元素步骤会渲染屏幕居中的无高亮 popover（观感异常）。缓解：composable `buildSteps()` 阶段过滤。
- **~~[driver.js 与 Vue SSR 不兼容]~~** → **当前不适用**：`nuxt.config.ts` 为 `ssr: false`（纯 SPA），无 server bundle，driver.js 顶层 import 与 `onMounted` 调用均安全。保留 `import.meta.client` 守卫仅为防御性预留（将来切 SSR 与 Vitest 行为可预测），非当前风险。
- **[localStorage 被清除]** → 用户清除浏览器数据后引导会重新出现。缓解：这是预期行为，不是 bug。
- **[引导步骤与未来 UI 变更不同步]** → 当 UI 布局改变时引导步骤可能失效。缓解：使用 `data-onboarding` 属性而非 CSS 类名选择器，属性名语义明确，重构时不易遗漏。
- **[移动端适配]** → 引导流程主要面向桌面端。缓解：移动端暂不触发引导，后续可扩展。
- **~~[空状态组件 SSR 水合闪烁（FOUC）]~~** → **当前不适用**：`ssr: false` 无水合阶段，空状态组件直接基于客户端 store 状态渲染，无 SSR→CSR 闪烁。无需 `<ClientOnly>` 包裹。
- **[侧边栏折叠态隐藏部分锚点]** → 折叠态下 `sidebarCollapsed=true`，`.categories`（line 226）、`.watched-tags-section`（line 189）被 `v-if` 隐藏，step 2/5 锚点消失；但 `nav-topic-graph`（line 177）/ `nav-tags`（line 182）button 无 v-if 仍在 DOM。缓解：首次访问默认展开（`FeedLayoutShell.vue:39 sidebarCollapsed = ref(false)` 实测确认），正常路径稳定；折叠态下 step 2/5 走预检过滤的 "skip missing DOM" 路径。

## Open Questions

无遗留。审查中浮现的待确认项均已落定：

- 功能发现提示（feature tips）→ 推迟到后续 change（见 D6）
- DailyReportEmptyGuide 定位 → 改造 `BoardDailyReportTimeline` 内部 `drt-empty`（见 D5 日报例外）
- "重播引导"按钮入口 → `SettingsSectionPreferences.vue`（见 D7）
- 侧边栏首次访问默认折叠状态 → 默认展开（`FeedLayoutShell.vue:39` `sidebarCollapsed = ref(false)` 实测确认），导航项 line 167-189 在展开态均渲染，不影响 step 2/3/4 锚点
- 引导文案 i18n → 暂不需要（与现有全中文 UI 一致）
