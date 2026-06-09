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
- 侧边栏 `AppSidebarView.vue`：主导航（全部文章、收藏夹、主题图谱、标签管理、关注标签、分类/Feed 列表）

技术栈约束：
- Vue 3 + Nuxt 4，TypeScript
- Tailwind CSS v4
- Pinia 状态管理
- 无后端改动

## Goals / Non-Goals

**Goals:**
- 首次访问时自动启动分步引导流程，带领用户完成核心操作路径
- 各功能区域在无数据时显示引导性空状态
- 关键功能入口提供首次交互提示
- 引导可随时跳过，并可在设置中重新触发
- 尊重 prefers-reduced-motion 媒体查询

**Non-Goals:**
- 不做用户账号系统或服务端偏好持久化（纯 localStorage）
- 不做交互式教学（用户不需要在引导中实际操作）
- 不做 A/B 测试或多变体引导
- 不做引导步骤的自定义或跳转到特定步骤
- 不做引导完成率的埋点统计
- 不修改后端 API

## Decisions

### D1: 引导库选择 driver.js

**选择**：使用 [driver.js](https://driverjs.com/)（v5+）作为分步引导引擎。

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

**理由**：引导不是交互式教学（Non-Goal），而是让用户知道各功能在哪里。5 步足够覆盖核心入口，不会造成认知过载。

### D3: 首次运行检测与持久化

**选择**：localStorage 存储两个键：

- `syntopica_onboarding_complete`：`"true"` 表示引导已完成（包括主动跳过）
- `syntopica_feature_tips`：JSON 对象记录已展示过的功能发现提示 `{ "tipId": true }`

**理由**：
- localStorage 同步读取，不影响首屏加载
- 简单的 key-value 结构，无需序列化复杂对象
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
  isFeatureTipShown(tipId: string): boolean
  markFeatureTipShown(tipId: string): void
  resetOnboarding(): void  // 用于设置页重新触发
}
```

composable 内部管理 driver.js 实例生命周期，提供响应式状态给 Vue 组件消费。

**理由**：单一 composable 封装所有 onboarding 逻辑，组件只需调用接口。driver.js 实例在 composable 内创建和销毁，避免全局污染。

### D5: 空状态引导组件

**选择**：在各功能区域创建独立的空状态组件，不使用统一组件：

| 组件 | 位置 | 显示条件 | 内容 |
|------|------|----------|------|
| `FeedEmptyGuide.vue` | `features/feeds/components/` | 无 Feed 且无分类 | "添加你的第一个 RSS 源" + 操作按钮 |
| `TopicGraphEmptyGuide.vue` | `features/topic-graph/components/` | 图谱无数据 | "等待标签数据积累" + 说明 |
| `DailyReportEmptyGuide.vue` | `features/tags/components/` | 无日报 | "日报需要积累数据" + 说明 |

每个组件独立负责自己的空状态判断逻辑和视觉呈现。

**理由**：各功能区域的数据模型和操作差异大，统一组件会增加 props 复杂度。独立组件可复用各自的 composable。

### D6: 功能发现提示机制

**选择**：基于 `data-feature-tip` 属性的轻量提示系统：

- 关键功能入口标记 `data-feature-tip="<tipId>"`
- 首次进入对应页面时，检查 `syntopica_feature_tips` 中是否已展示
- 使用 Tailwind + CSS transition 实现简单的 tooltip 动画（非 driver.js）
- 动画在 `prefers-reduced-motion: reduce` 下降级为即时显示

初始功能发现提示列表：
- 话题图谱筛选控件（`topic-graph-filter`）
- 标签合并建议（`tag-merge-suggestions`）
- 日报时间线查看（`daily-report-timeline`）

**理由**：功能发现提示是轻量级的"看一眼就知道"，不需要 driver.js 的完整 overlay 能力。CSS 实现更轻量，不增加运行时开销。

### D7: 重新触发引导的入口

**选择**：在设置面板（偏好设置页面或 header 菜单）添加"重新播放引导"按钮，调用 `useOnboarding().resetOnboarding()` 后自动刷新页面。

**理由**：简单的重置策略，不增加路由状态管理复杂度。

## Risks / Trade-offs

- **[引导元素未渲染]** → 步骤依赖的 DOM 元素可能未挂载（如侧边栏折叠、分类为空时 Feed 区域不存在）。缓解：使用 `beforeNext` 钩子检查元素存在性，不存在则跳过该步骤；欢迎弹窗不依赖任何 DOM 元素。
- **[driver.js 与 Vue SSR 不兼容]** → driver.js 引用 `document` 对象，Nuxt SSR 环境下会报错。缓解：composable 仅在 `onMounted` 中初始化 driver.js 实例，tour 步骤定义为客户端数据。
- **[localStorage 被清除]** → 用户清除浏览器数据后引导会重新出现。缓解：这是预期行为，不是 bug。
- **[引导步骤与未来 UI 变更不同步]** → 当 UI 布局改变时引导步骤可能失效。缓解：使用 `data-onboarding` 属性而非 CSS 类名选择器，属性名语义明确，重构时不易遗漏。
- **[移动端适配]** → 引导流程主要面向桌面端。缓解：移动端暂不触发引导，后续可扩展。

## Open Questions

- 侧边栏折叠状态下如何处理侧边栏相关步骤（当前建议跳过）
- 功能发现提示的具体样式（跟随现有 AppTooltip 组件还是独立设计）
- 引导的语言——当前 UI 全中文，引导文案是否需要 i18n 支持（暂不需要，与现有 UI 保持一致）
