# AGENTS.md

Agent guide for coding assistants working in `Syntopica` (`D:\project\my-robot`).

## Project Snapshot
- Syntopica: Nuxt 4 frontend + Go backend (Gin/GORM), single-user, no auth.
- Frontend API: `http://localhost:5000/api`; WebSocket: `ws://localhost:5000/ws`.
- PostgreSQL + pgvector for persistence; Redis optional for job queues.
- 和用户沟通使用中文，开发环境 Windows。
- 使用 openspec 编写任务时，tasks.md 必须遵循 `docs/reference/开发执行规范.md` §11 归档门禁：以「测试 / 文档 / 验证」三节收尾，验证节每条附可执行命令；归档前重跑验证节确认零失败。归档后按 §12 文档流转把 change 移入当前里程碑 `docs/v1.x/changes/`，reference 在里程碑收尾时统一更新。数据库更新规范（迁移索引 vs gorm 自动建表）按 §10 处理。

## 开发环境 (Development Environment)

| 项目 | 说明 |
|------|------|
| OS | **Windows**（WSL2 `bash` 可用，但路径使用 Windows 格式如 `D:/project/...`）|
| 数据库 | **Docker**：`docker compose -f docker-compose.pg.yml up -d` 启动 PostgreSQL（pgvector），默认端口 `5432`，用户/密码为 `postgres`，库名为 `syntopica`（对应 `docker-compose.pg.yml` 的 `POSTGRES_DB` 默认值）。数据持久化在 `./data/` 下。`docker compose -f docker-compose.pg.yml down` 停止。|
| Python | **uv**：需要 Python 脚本/工具时使用 `uv`（如 `uv run script.py`、`uv add package`）。Python 集成测试位于 `tests/workflow/`、`tests/firecrawl/`。|
| Node.js | `pnpm`（要求 corepack 启用）。详见 `front/AGENTS.md`。|
| Go | 直接使用系统 Go 工具链。详见 `backend-go/AGENTS.md`。|

## Headroom (Context Compression)

[Headroom](https://headroom-docs.vercel.app/) 通过 MCP 集成，用于压缩大量工具输出（日志、grep、JSON 等）以节省 token。详细用法见 `/skill:headroom`。配置文件：`~/.pi/agent/mcp.json`。

**快速开始本地开发：**

```bash
# 1. 启动数据库（Docker）
docker compose -f docker-compose.pg.yml up -d

# 2. 启动后端（backend-go/）
cd backend-go && go run cmd/server/main.go

# 3. 启动前端（新终端，front/）
cd front && pnpm dev
```

## Reference Docs (authoritative source)
- **Architecture**: `docs/reference/architecture/`
- **API**: `docs/reference/api/`
- **Database**: `docs/reference/database/`
- **Development**: `docs/reference/development.md`
- **Configuration**: `docs/reference/configuration.md`
- **Deployment**: `docs/reference/deployment.md`
- **Testing**: `docs/reference/testing.md`
- Subdirectory guides: `front/AGENTS.md`, `backend-go/AGENTS.md`.

## Repo Layout
- `front/`: Nuxt 4, Vue 3, TypeScript, Pinia, Tailwind CSS v4.
- `backend-go/`: Gin, GORM, PostgreSQL + pgvector.
- `docs/`: reference/ (活文档) + v1.x/ (里程碑) + experience/.
- `tests/workflow/`, `tests/firecrawl/`: Python integration tests.

## Key Entry Points
- `README.md`, `front/app/app.vue`, `front/app/api/client.ts`, `front/app/stores/api.ts`
- `backend-go/cmd/server/main.go`, `backend-go/internal/app/router.go`, `backend-go/internal/app/runtime.go`

## Build & Verify

**Frontend** (`front/`): `pnpm install` / `pnpm dev` / `pnpm build` / `pnpm lint` / `pnpm exec nuxi typecheck` / `pnpm test:unit` / `pnpm test:e2e`

**Backend** (`backend-go/`): `go mod tidy` / `go run cmd/server/main.go` / `golangci-lint run ./...` / `go vet ./...` / `go test ./...` / `go build ./...`

**Pre-push check**: `cd backend-go && golangci-lint run ./... && go vet ./... && go test ./... && go build ./...` && `cd front && pnpm lint && pnpm exec nuxi typecheck && pnpm test:unit && pnpm build`

## AI Behavior Rules
- Do not add linters, formatters, or tooling unless asked.
- Do not assume Python backend; the product backend is Go.
- Ignore unrelated dirty-worktree changes. Verify smallest relevant command after edits.
- **测试只跑本次修改影响的包**，不要跑全量 `go test ./...`。例如改了 `daily_report` 和 `ws`，就只跑 `go test ./internal/domain/daily_report ./internal/platform/ws`。
- **前端 pnpm 编译类命令（typecheck / build）必须通过 Windows cmd 执行**，WSL 环境缺少 native binding（如 `@oxc-parser/binding-linux-x64-gnu`）会失败。lint 可在 WSL 跑。示例：
  ```bash
  # lint — WSL 可用
  cd front && pnpm lint
  # typecheck / build — 必须用 cmd
  cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"
  cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"
  ```
- Frontend edits → `pnpm lint` / `pnpm exec nuxi typecheck` / `pnpm test:unit` / `pnpm build`。
- Backend edits → `golangci-lint run ./...` / targeted `go test` first, then `go test ./...` / `go build ./...`。
- Docs-only edits: consistency check unless behavior changed.
- Keep code changes minimal and scoped. Match existing code style.
- 完成任务后更新维护 `./docs/reference/` 知识库；openspec change 归档前满足 `开发执行规范.md` §11 归档门禁，归档后按 §12 流转归类到里程碑。

## Browser Automation
Use `agent-browser`: `open <url>` → `snapshot -i` → `click @eX` / `fill @eX "text"` → re-snapshot.

---
Behavioral guidelines to reduce common LLM coding mistakes. Merge with project-specific instructions as needed.

**Tradeoff:** These guidelines bias toward caution over speed. For trivial tasks, use judgment.

## 1. Think Before Coding

**Don't assume. Don't hide confusion. Surface tradeoffs.**

Before implementing:
- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them - don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

## 2. Simplicity First

**Minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.

Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

## 3. Surgical Changes

**Touch only what you must. Clean up only your own mess.**

When editing existing code:
- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it - don't delete it.

When your changes create orphans:
- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

The test: Every changed line should trace directly to the user's request.

## 4. Goal-Driven Execution

**Define success criteria. Loop until verified.**

Transform tasks into verifiable goals:
- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:
```
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
```

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.

---

**These guidelines are working if:** fewer unnecessary changes in diffs, fewer rewrites due to overcomplication, and clarifying questions come before implementation rather than after mistakes.

---

## context-mode — 上下文窗口路由规则（按需使用）

> 本节由 context-mode plugin（`~/.config/opencode/opencode.json` → `plugin: ["context-mode"]`）提供。目的：把大块工具输出（日志/grep/JSON）沙箱化、索引进 FTS5，避免一次性灌爆上下文窗口。**与上面的核心开发规范并行生效；当本节规则与上面冲突时，上面的 Syntopica 项目规范优先**（例如前端编译必须走 Windows cmd、测试只跑影响包、和用户用中文沟通等不变）。

context-mode 提供 11 个 `ctx_*` 工具。下列规则保护上下文窗口，一条未路由的命令可能直接灌入 56 KB。

### Think in Code（强烈建议）

分析/计数/过滤/比较/搜索/解析/变换数据时：用 `context-mode_ctx_execute(language, code)` 写代码，**只 `console.log()` 最终答案**，不要把原始数据读进上下文。PROGRAM 分析，而不是 COMPUTE。一次脚本顶十次工具调用。

> 项目适配：本仓库后端是 Go，日常代码分析优先用 Go/项目自带工具；ctx_execute 主要用于跨工具批量取数、日志聚合、JSON 解析这类“会产出大块输出”的场景。

### BLOCKED — 会被拦截（不要重试）

- **curl / wget**：Shell 里的 `curl`/`wget` 会被拦截。改用 `context-mode_ctx_fetch_and_index(url, source)` 或 `context-mode_ctx_execute(language: "javascript", code: "const r = await fetch(...)")`。
- **内联 HTTP**：`fetch('http`、`requests.get(`、`http.get(`、`http.request(` 会被拦截。改用 `context-mode_ctx_execute`，只有 stdout 进上下文。
- **直接抓网页**：用 `context-mode_ctx_fetch_and_index(url, source)` 然后 `context-mode_ctx_search(queries)`。

### REDIRECTED — 走沙箱

- **Shell（输出 >20 行）**：Shell 只用于 `git`、`mkdir`、`rm`、`mv`、`cd`、`ls`、`npm install`、`pnpm install`、`go mod tidy` 等小输出命令。其余用 `context-mode_ctx_batch_execute(commands, queries)` 或 `context-mode_ctx_execute(language: "shell", code: "...")`。
- **读文件（为了分析）**：读文件**为了编辑** → 正常读；读文件**为了分析/探索/总结** → `context-mode_ctx_execute_file(path, language, code)`。
- **grep / 搜索（大结果）**：用 `context-mode_ctx_execute(language: "shell", code: "grep ...")` 在沙箱里跑。

### 工具选择速查

0. **MEMORY**：`context-mode_ctx_search(sort: "timeline")` — resume 后先查历史，别直接问用户。
1. **GATHER**：`context-mode_ctx_batch_execute(commands, queries)` — 一次跑多条命令并自动索引、返回搜索结果，一次调用顶 30+。
2. **FOLLOW-UP**：`context-mode_ctx_search(queries: ["q1", "q2", ...])` — 多个问题合并成数组一次调用。
3. **PROCESSING**：`context-mode_ctx_execute(language, code)` | `context-mode_ctx_execute_file(path, language, code)` — 沙箱，只有 stdout 进上下文。
4. **WEB**：`context-mode_ctx_fetch_and_index(url, source)` 然后 `context-mode_ctx_search(queries)`。
5. **INDEX**：`context-mode_ctx_index(content, source)` — 存进 FTS5 供后续搜索。

### 并发 I/O 批处理

多 URL 抓取或多 API 调用时，带上 `concurrency: N`（1-8）：
- 网络类（`gh`、`curl`、多区域云查询）用 `concurrency: 4-8`。
- CPU 密集或共享状态（`npm test`、`build`、`lint`、同仓库写）保持 `concurrency: 1`。
- GitHub API 限流：`gh` 调用上限 4。

### 产物输出

大产物写到**文件**，不要内联进上下文。返回：文件路径 + 一行描述。`search(source: "label")` 用有意义的 source 标签。

### 会话连续性 & 记忆

- 整个会话内 skills/roles/决策保持有效，不要随对话变长就丢弃。
- 会话历史持久且可搜索。resume 时**先搜后问**：

| 需求 | 命令 |
|------|------|
| 我们决定了什么？ | `context-mode_ctx_search(queries: ["decision"], source: "decision", sort: "timeline")` |
| 有哪些约束？ | `context-mode_ctx_search(queries: ["constraint"], source: "constraint")` |

不要问“我们刚才在做什么？”——**先搜**。搜出 0 条结果，就按全新会话处理。

### ctx 命令

| 命令 | 动作 |
|------|------|
| `ctx stats` | 调 `stats` MCP 工具，原样展示完整输出 |
| `ctx doctor` | 调 `doctor` MCP 工具，跑返回的 shell 命令，按 checklist 展示 |
| `ctx upgrade` | 调 `upgrade` MCP 工具，跑返回的 shell 命令，按 checklist 展示 |
| `ctx purge` | 调 `purge` MCP 工具（confirm: true）。清知识库前会警告 |

`/clear` 或 `/compact` 后：知识库和会话统计保留。想重开用 `ctx purge`。
