# Frontend Agent Guide

遵循根 `AGENTS.md` 的所有规则。以下为前端特有差异。

## Frontend-Specific Conventions

- Always `<script setup lang="ts">` (Composition API).
- Components: PascalCase. Composables: camelCase with `use` prefix.
- Import order: Vue/Nuxt → third-party → internal → type-only.
- Use `import type` for type-only deps. Use `~` alias for app-root paths.
- Convert numeric IDs to strings at API boundary. `snake_case → camelCase` in stores/API, never in templates.
- HTTP via `ApiClient` in `app/api/client.ts`. Return `{ success, data, error, message }`.
- Files must be UTF-8. Preserve existing semicolon style per file.
- UI: editorial/magazine feel, avoid generic SaaS look.

## Anti-Patterns

- No API calls in components. No `any` types. No Options API. No `@ts-ignore`.
- **No direct WebSocket/EventSource** for global events — use `useEventStream()`.
- **No Store mutation without API call** — all state changes must persist to backend.
- **No circular Store dependencies** — articlesStore does not import apiStore's mutation methods.
- **No cross-feature deep imports** — use `features/*/public.ts` facade or `api/normalizers/` for shared code.
- **No duplicate notification** — only one responsible layer calls `notify.error()` per failure.

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

---

## 架构约定（Architecture Conventions）

> 以下约定源自 v1.3.3 架构深化（deepen-architecture change）。新增代码必须遵守。

### 1. 目录归属规则

| 代码类型 | 归属位置 | 说明 |
|---------|---------|------|
| 业务组件 | `features/*/components/` | Feature 私有，其他 feature 不得直接 import |
| 业务 composable | `features/*/composables/` | Feature 私有 |
| 跨 feature 共享组件 | `features/*/public.ts` facade | 或上移到 `components/` |
| 全局 composable | `composables/` | 如 `useEventStream`、`useNotify` |
| HTTP 调用 | `api/` | 唯一 HTTP 边界，通过 `useXxxApi()` 使用 |
| 数据 normalizer | `api/normalizers/` | 共享 DTO 转换，feature 内的仅限内部使用 |
| 状态管理 | `stores/` | 需跨组件共享的状态才放 Store |
| 领域类型 | `types/` | API 响应类型、Store 状态类型 |
| 纯工具函数 | `utils/` | 常量、格式化、通用逻辑 |

### 2. API 层归一化

**查询构建、响应解包、camelCase 转换各只有一个权威 Module：**

```typescript
// ✅ 正确：使用统一工具
import { buildQueryString, camelizeKeys, mapApiResponse, unwrapResponse } from '~/utils/api-helpers'

// ❌ 错误：手写 URLSearchParams 或重复实现
const params = new URLSearchParams()
params.set('page', '1')
```

**API 模块中的 `as unknown as ApiResponse<T>` 应替换为 `mapApiResponse<T>()`。**

### 3. Store 规则

**写操作必须持久化到后端（store-integrity）：**

```typescript
// ✅ 正确：调用 API + 乐观更新 + 回滚
async function toggleFavorite(articleId: string) {
  const prev = article.favorite
  article.favorite = !prev  // 乐观更新
  const { success } = await articlesApi.toggleFavorite(articleId)
  if (!success) {
    article.favorite = prev  // 回滚
    notify.error('收藏失败')
  }
}

// ❌ 错误：只改本地状态
article.favorite = !article.favorite
```

**Store 间数据同步通过小 Interface：**

```typescript
// ✅ 正确：通过 feedsStore 的方法同步 unread count
feedsStore.adjustUnreadCount(feedId, -1)

// ❌ 错误：articlesStore 直接遍历 apiStore.feeds
apiStore.feeds.find(f => f.id === feedId).unread_count--
```

### 4. 事件流使用

**所有全局实时事件通过 `useEventStream()` 订阅：**

```typescript
// ✅ 正确
import { useEventStream } from '~/composables/useEventStream'
import { EVENT_TYPES } from '~/utils/eventTypes'

const stream = useEventStream()
onMounted(() => {
  off = stream.on(EVENT_TYPES.TAG_COMPLETED, (data) => { ... })
})
onUnmounted(() => { off?.() })

// ❌ 错误：自建 WebSocket
const ws = new WebSocket(`${apiBase}/ws`)
```

- 事件类型常量使用 `utils/eventTypes.ts` 中的定义
- **必须在 `onUnmounted` 中调用 unsubscribe**
- 专用 SSE 长任务（如 scan/evaluate）在对应 API module 中作为命名 Adapter 暴露

### 5. 错误通知

**写操作失败时由执行方通知，底层 API 不弹 toast：**

```typescript
// ✅ 正确：Store/Composable 通知
async function deleteBoard(id: string) {
  const { success } = await boardsApi.deleteBoard(id)
  if (!success) {
    notify.error('删除失败')  // Store/Composable 层通知
  }
}

// ❌ 错误：API 模块内弹 toast
// api/semanticBoards.ts 中不应调用 notify.error()
```

- 使用 `useNotify()` 的 `success/error/warn` 方法
- 同一次失败只由一个责任层通知，避免重复 toast

### 6. Feature 间共享

**跨 feature 引用必须通过 facade 或共享层：**

```typescript
// ✅ 正确：通过 feature facade
import { ArticleContentView } from '~/features/articles/public'

// ✅ 正确：通过共享 normalizer
import { normalizeArticle } from '~/api/normalizers/article'

// ❌ 错误：深 import 另一 feature 内部
import ArticleContentView from '~/features/articles/components/ArticleContentView.vue'
import { normalizeArticle } from '~/features/articles/utils/normalizeArticle'
```

**新建 feature 需要对外暴露能力时，创建 `public.ts`：**

```typescript
// features/my-feature/public.ts
export { default as MySharedComponent } from './components/MySharedComponent.vue'
export { useMyComposable } from './composables/useMyComposable'
```

### 7. 大组件拆分

单文件超过 **500 行 / ~15KB** 时应拆分。拆分原则：

1. **按内聚行为拆 composable**，不是按文件行数机械拆分
2. **每个 composable 以小 Interface 暴露一组高内聚行为**
3. **不能形成巨型 composable**（>500 行的 composable 需要继续拆分）
4. 子组件通过 props 接收数据，不直接依赖父级 composable 的内部状态

### 8. 新增功能 Checklist

添加新功能时，确认以下归属正确：

- [ ] API 调用 → `api/xxx.ts`（通过 `useXxxApi()` composable）
- [ ] 需跨页面共享的状态 → `stores/xxx.ts`（mutation 必须调 API）
- [ ] 只在单页面使用的状态 → feature composable 内部管理
- [ ] 实时事件订阅 → `useEventStream()` + `EVENT_TYPES`
- [ ] 错误反馈 → `useNotify()`
- [ ] 跨 feature 引用 → `features/*/public.ts` facade
- [ ] 数据转换 → `api/normalizers/` 或 `utils/api-helpers.ts`
- [ ] snake_case → camelCase 转换只在 API 边界
