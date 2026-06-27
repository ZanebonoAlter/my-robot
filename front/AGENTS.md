# Frontend Agent Guide

遵循根 `AGENTS.md` 的所有规则。以下为前端特有差异。

> **权威源声明（方案 B，防孤立）**：前端代码规范、双主题、lint/测试配置的**唯一权威源**在 `docs/reference/standard/frontend/`。本文件只保留 agent 秒级要看到的**红线速查**，每节末尾给 `→ standard/xxx.md` 深链。规范有出入时以 standard/ 为准。

## Commands

```bash
pnpm install  &&  pnpm dev  &&  pnpm build
pnpm lint  &&  pnpm exec nuxi typecheck
pnpm test:unit  &&  pnpm test:e2e
```

> **⚠️ WSL 注意**：`pnpm lint` 可在 WSL 跑；`pnpm exec nuxi typecheck` 和 `pnpm build` 因缺少 Linux native binding 必须在 Windows cmd 中执行：
> ```bash
> cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"
> cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"
> ```
> 提交前检查全流程见 `docs/reference/standard/shared/commit-pr.md`。

## 红线速查（Anti-Patterns — 硬禁）

- ❌ 组件内直接调 API / 调 WebSocket / EventSource（全局事件走 `useEventStream()`）
- ❌ `any` 类型 / Options API / `@ts-ignore`
- ❌ 不调 API 直接改 Store 状态（写操作必须持久化 + 乐观更新 + 回滚）
- ❌ Store 间循环依赖（articlesStore 不 import apiStore mutation）
- ❌ 跨 feature 深 import（用 `features/*/public.ts` facade 或 `api/normalizers/`）
- ❌ 重复通知（同一失败多层 toast，底层 API 不弹 toast）

→ 完整 Anti-Patterns 与代码示例：`docs/reference/standard/frontend/code-style.md` §10

## 关键约定速查

- 组件 `<script setup lang="ts">`；组件名 PascalCase；composable 名 `use` 前缀 camelCase
- 导入顺序：Vue/Nuxt → 第三方 → 内部 → type-only；`import type` 导入纯类型；`~` alias
- 数字 ID 在 API 边界转 `string`；`snake_case → camelCase` 只在 store/API 层，模板里不出现
- HTTP 走 `ApiClient`（`app/api/client.ts`）；返回 `{ success, data, error, message }`
- 双主题：三层 Token（Primitive → Semantic → Component），`data-theme="editorial|dark"`，`useTheme()`；组件只引用 Layer 2；editorial/magazine 风格不回退 SaaS
- 统一组件：`AppDialog` / `AppButton` / `AppToggle` / `AppInput` / `AppSectionHeader`
- 大组件超 500 行 / ~15KB 拆分

→ 组件/API/Store/事件流/通知/Feature 共享：`docs/reference/standard/frontend/code-style.md`
→ 双主题与设计系统：`docs/reference/standard/frontend/theming.md`

## 目录归属速查

| 代码类型 | 归属 |
|---------|------|
| 业务组件/composable | `features/*/components/`、`features/*/composables/`（私有） |
| 跨 feature 共享 | `features/*/public.ts` facade 或上移 `components/` |
| 全局 composable | `composables/`（`useEventStream`、`useNotify`） |
| HTTP 调用 | `api/`（**唯一边界**，`useXxxApi()`） |
| 数据 normalizer | `api/normalizers/` |
| 状态管理 | `stores/`（仅跨组件共享） |
| 领域类型 / 纯工具 | `types/` / `utils/` |
| 页面 | `pages/`（只挂载） |

→ 完整归属规则 + 新增功能 Checklist：`docs/reference/standard/frontend/code-style.md` §2、§8

## 测试与 Lint

- 单元：Vitest + happy-dom，`*.test.ts` 与源同目录
- E2E：Playwright，`front/tests/e2e/*.spec.ts`，串行执行（只放稳定 smoke）
- 交互验证：DeepSeek v4 Flash 按需验证（断言现写现跑，不堆回归脚本）→ `.agents/skills/playwright-e2e/` + `docs/reference/architecture/ui-navigation.md`
- 交互约定：状态标记左对齐、状态说明不伪装动作 → `docs/reference/standard/frontend/interaction-conventions.md`
- ESLint flat config：`front/eslint.config.js`

→ `docs/reference/standard/frontend/testing.md`、`docs/reference/standard/frontend/lint.md`
