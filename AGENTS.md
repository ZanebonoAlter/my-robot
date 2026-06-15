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
