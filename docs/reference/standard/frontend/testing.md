# 前端测试（Vitest + Playwright）

> **权威源**：本文件是前端测试约定的唯一权威。运行门禁见《开发执行规范》§5.1。

## 套件总览

| 套件 | 框架 | 位置 | 环境 |
|------|------|------|------|
| 单元测试 | Vitest + Vue Test Utils | `front/app/**/*.test.ts` | happy-dom |
| E2E 测试 | Playwright | `front/tests/e2e/*.spec.ts` | Chromium（真实浏览器） |

## 单元测试（Vitest）

- 配置文件：`front/vitest.config.ts`（使用 `happy-dom`，**不需要真实浏览器**）
- 测试文件与源文件**同目录**，命名 `*.test.ts`（如 `app/utils/foo.test.ts`）
- `front/tests/e2e/` 通过 `vitest.config.ts` 排除在 Vitest 之外
- 使用 `describe`/`it` 块，描述行为而非实现：

```typescript
import { describe, expect, it } from 'vitest'
import { myFunction } from './myFunction'

describe('myFunction', () => {
  it('does the expected thing', () => {
    expect(myFunction('input')).toBe('expected')
  })
})
```

运行：

```bash
cd front
pnpm test:unit                                            # 全部
pnpm test:unit -- app/utils/articleContentSource.test.ts  # 单文件
pnpm test:unit -- app/utils/articleContentSource.test.ts -t "prefers firecrawl"  # 按名称
```

## E2E 测试（Playwright）

- 配置文件：`front/playwright.config.ts`
- spec 文件放 `front/tests/e2e/*.spec.ts`
- 在 `http://localhost:3000` 自动启动 Nuxt 开发服务器后跑浏览器测试
- **串行执行**（`fullyParallel: false`、`workers: 1`）保证启动稳定

```typescript
import { test, expect } from '@playwright/test'

test('page loads', async ({ page }) => {
  await page.goto('/some-page')
  await expect(page.locator('body')).toBeVisible()
})
```

运行：

```bash
cd front
pnpm test:e2e           # 全部（自动启动 dev server）
pnpm test:e2e:ui        # Playwright UI
pnpm test:e2e:list      # 只列出
pnpm test:e2e:args -- --grep "topic-graph"  # 传参
```

## 覆盖率

未配置最低覆盖率阈值；Vitest `vitest.config.ts` 无覆盖率设置。

## 资料来源

收敛自原 `testing.md`（§前端单元 / §前端 E2E / §编写新测试）与《开发执行规范》§5.2。
