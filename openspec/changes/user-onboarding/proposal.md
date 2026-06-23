## Why

Syntopica 目前没有任何用户指引机制——首次使用的用户直接面对完整的应用界面，没有引导流程、功能提示或空状态说明。对于信息聚合 + AI 标签系统这种复杂产品，缺少用户引导会显著增加学习成本和流失率。

## What Changes

- 首次使用引导流程（First-run experience）：
  - 检测用户是否首次访问（基于 localStorage 标记）
  - 分步引导用户完成核心操作：添加 RSS 源 → 等待内容抓取 → 查看标签/话题图 → 查看日报
  - 使用 overlay + 高亮步骤式引导（参考 driver.js 或类似方案）
- 空状态引导：
  - 当用户尚未添加 RSS 源时，显示引导卡片
  - 当日报为空时，显示"等待数据积累"的说明（改造 `BoardDailyReportTimeline` 内部 `drt-empty`）
  - 当话题图为空时，提示可能的原因和操作建议

> **Scope 收窄说明**：功能发现提示（feature tips）原计划纳入本 change，审查后发现与现有 `AppTooltip`（hover 模式）交互不匹配，推迟到后续独立 change。详见 design.md D6。

## Capabilities

### New Capabilities

- `user-onboarding`: 首次使用引导流程，包括分步教程和空状态引导

### Modified Capabilities

（无现有能力的需求变更）

## Impact

- `front/app/composables/`：新增 `useOnboarding` composable（纯前端状态 + localStorage）
- `front/app/features/shell/components/AppSidebarView.vue`：新增 4 个 `data-onboarding` 锚点属性（`sidebar-feeds` / `nav-topic-graph` / `nav-tags` / `watched-tags`）
- `front/app/features/feeds/`：Feed 空状态引导卡片
- `front/app/features/topic-graph/`：Topic graph 空状态引导
- `front/app/features/tags/components/BoardDailyReportTimeline.vue`：增强内部 `drt-empty` 日报空状态
- `front/app/features/settings/components/SettingsSectionPreferences.vue`：新增"重播引导"按钮
- 新增 npm 依赖 driver.js `^1.4.0`
- 后端无需改动（纯前端 change）

## Engineering Standards

本 change 遵循 `@docs/reference/开发执行规范.md`，适用条款：

- **§5.0 前置检查**：apply 启动时确认 `pnpm lint` / `pnpm exec nuxi typecheck` / `pnpm test:unit` / `pnpm build` 就绪
- **§5.1 前端质量门禁**：提交前必须通过 `pnpm lint && pnpm exec nuxi typecheck && pnpm test:unit && pnpm build`（typecheck / build 必须走 Windows cmd，见 AGENTS.md）
- **§5.2 测试规范**：Vitest + happy-dom，测试文件与源文件同目录命名 `*.test.ts`（如 `app/composables/useOnboarding.test.ts`）
- **§11 归档门禁**：tasks.md 以「测试 / 文档 / 验证」三节收尾，验证节每条为可执行命令 + 期望结果；归档前重跑验证节确认零失败；归档后按 §12 流转归类到里程碑
- **豁免**：本 change 纯前端，无后端改动，豁免 §4.1 后端门禁与 §6 集成测试（testcontainer）
