# AGENTS.md

Agent guide for coding assistants working in `Syntopica` (`D:\project\my-robot`).

## Project Snapshot

- Syntopica: Nuxt 4 frontend + Go backend (Gin/GORM), single-user, no auth.
- Frontend API: `http://localhost:5000/api`; WebSocket: `ws://localhost:5000/ws`.
- PostgreSQL + pgvector for persistence; Redis optional for job queues.
- 和用户沟通使用中文，开发环境 Windows, 返回的回答尽量用大白话，接地气，能让用户理解。
- **所有改动默认走 openspec**（代码/功能/接口/数据模型必须先开 change）；豁免清单与编排见 `docs/reference/开发执行规范.md` §0.6「准入总则」

## 开发环境 (Development Environment)

| 项目 | 说明 |
| ------ | ------ |
| OS | **Windows**（WSL2 `bash` 可用，但路径使用 Windows 格式如 `D:/project/...`） |
| 数据库 | **Docker**：`docker compose -f docker-compose.pg.yml up -d` 启动 PostgreSQL（pgvector），默认端口 `5432`，用户/密码为 `postgres`，库名为 `syntopica`（对应 `docker-compose.pg.yml` 的 `POSTGRES_DB` 默认值）。数据持久化在 `./data/` 下。`docker compose -f docker-compose.pg.yml down` 停止。 |
| Python | **uv**：需要 Python 脚本/工具时使用 `uv`（如 `uv run script.py`、`uv add package`）。Python 集成测试位于 `tests/workflow/`、`tests/firecrawl/`。 |
| Node.js | `pnpm`（要求 corepack 启用）。详见 `front/AGENTS.md`。 |
| Go | 直接使用系统 Go 工具链。详见 `backend-go/AGENTS.md`。 |

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

- **Code Standards**: `docs/reference/standard/` — 代码规范/项目约束/lint/测试配置的**唯一权威源**（前后端分文件夹）
- **Business Flow**: `docs/reference/flow/` — 五位一体活文档（需求说明 / 链路设计 / 业务约束与不变量 / 代码入口 / 变更溯源），替代原 user-guide；「业务约束」节是 `doc-impact.sh context` 的数据源
- **Architecture**: `docs/reference/architecture/` — 架构定位与骨架；`architecture/map.md` 是业务域→流程→代码入口的索引地图
- **API**: `docs/reference/api/`
- **Database**: `docs/reference/database/`
- **Configuration**: `docs/reference/configuration.md`
- **Deployment**: `docs/reference/deployment.md`
- **执行规范**: `docs/reference/开发执行规范.md` — 任务拆解/TDD/门禁/归档纪律
- Subdirectory guides: `front/AGENTS.md`, `backend-go/AGENTS.md`.

> `development.md` / `testing.md` 的规范内容已迁入 `standard/`，仅保留构建/运行参考。

## Repo Layout

- `front/`: Nuxt 4, Vue 3, TypeScript, Pinia, Tailwind CSS v4.
- `backend-go/`: Gin, GORM, PostgreSQL + pgvector.
- `docs/`: reference/ (活文档，含 flow 变更溯源) + v1.x/ (里程碑，可选) + experience/.
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
- git提交使用 zanebonoalter <380207345@qq.com>
- **测试只跑本次修改影响的包**，不要跑全量 `go test ./...`。例如改了 `daily_report` 和 `ws`，就只跑 `go test ./internal/domain/daily_report ./internal/platform/ws`。
- **前端 pnpm 编译/测试类命令（typecheck / build / test:unit）必须通过 Windows cmd 执行**，WSL 环境缺少 native binding（typecheck 如 `@oxc-parser/binding-linux-x64-gnu`；test:unit 经 Vite→rollup 缺 `@rollup/rollup-linux-x64-gnu`）会失败。lint 可在 WSL 跑。权威定义见 [`standard/frontend/testing.md`](docs/reference/standard/frontend/testing.md) §跨平台运行 + §常见陷阱。示例：

  ```bash
  # lint — WSL 可用
  cd front && pnpm lint
  # typecheck / build / test:unit — 必须用 cmd
  cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"
  cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"
  cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit 2>&1"
  ```

- Frontend edits → `pnpm lint` / `pnpm exec nuxi typecheck` / `pnpm test:unit` / `pnpm build`。
- Backend edits → `golangci-lint run ./...` / targeted `go test` first, then `go test ./...` / `go build ./...`。
- Docs-only edits: consistency check unless behavior changed.
- **pi 增量门禁（自动）**：`.pi/extensions/quality-gate.ts` 已挂 `turn_end`，每回合结束若改了代码，自动跑后端 `golangci-lint`+`go vet`+`go build` / 前端 `pnpm lint`，失败以 `steer` 消息喂回。agent 见到失败消息**必须修**，不得忽略。不跑 `go test`/typecheck/build（影响包不可自动判定/需 cmd.exe），这些仍由 agent 手动跑 + §11 归档门禁兜底。门禁分层见 `docs/reference/开发执行规范.md` §4.1。
- Keep code changes minimal and scoped. Match existing code style.
- 完成任务后更新维护 `./docs/reference/` 知识库；openspec change 执行走 `开发执行规范.md` §0.6 标准编排流程（**apply 启动跑 `doc-impact.sh suggest`+`context`，归档前跑 `doc-impact.sh verify`+`check-standards.sh`**），归档前满足 §11 门禁，归档后按 §12 补 flow 变更溯源链接（archive 即永久家，v1.x 里程碑可选）。
- **开工前/完工后必须汇报"部署后影响 + 需要的操作"**：每个 change 完工汇报必须包含一节明确告诉用户——(a) 部署/合并后用户可见行为会发生什么变化；(b) 需要用户手动执行的操作（如重新生成数据、清理、配置）；(c) 旧数据如何降级。避免用户打开界面才发现行为变了产生误会。涉及数据迁移、状态机变更、UI 分区变更时尤其强制。

子线程派发参考（pi 的 subagent 派发如何选供应商与模型）:

> ⚠️ **硬规则：Agent 的 `model` 参数必须用 `provider/modelId` 全称**（如 `zai-coding-cn/glm-5.2`），**禁止用 fuzzy 名**（如 `glm-5.2`）。
>
> 原因：pi 里有 10 个 model id 跨多个 provider 重复注册（`glm-5.2`/`glm-5.1`/`glm-4.7`/`glm-5-turbo`/`glm-4.5-air`/`glm-5v-turbo`/`deepseek-v4-pro`/`deepseek-v4-flash`/`mimo-v2.5`/`mimo-v2.5-pro`）。fuzzy 名会按字母序解析到**非预期**的供应商——实测传 `glm-5.2` 会落到 `opencode-go`（字母序最先），而不是默认供应商 `zai-coding-cn`。想用默认供应商（`zai-coding-cn/glm-5.2`）时，**省略 `model` 参数即可**；一旦显式传 fuzzy 名反而绕过默认、落到错误供应商。实时清单查 `pi --list-models`。

**任务→模型全称对照**（`model` 参数照填下表全称）：

| 任务类型 | model 全称 |
| --------- | ------------ |
| 简单/重复劳动、编译修复脏活累活 | `deepseek/deepseek-v4-flash` |
| 实现后端功能较复杂、TDD | `zai-coding-cn/glm-5.2` |
| 核心逻辑实现、架构级选型、疑难杂症 | `zai-coding-cn/glm-5.2` |
| 代码审查、审美有要求的前端任务 | `kimi-coding/k3` 或 `zai-coding-cn/glm-5.2` |
| E2E脚本执行和验证 | `deepseek/deepseek-v4-flash` |

> glm 系列统一走当前默认供应商 `zai-coding-cn`（国内 coding 专用），优于 `zai`（国际站）/ `opencode-go`（聚合网关）。
>
> change 执行的完整编排（主线程调度 + 子线程派发六步）见 `docs/reference/开发执行规范.md` §0.6。

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
| ------ | ------ |
| `ctx stats` | 调 `stats` MCP 工具，原样展示完整输出 |
| `ctx doctor` | 调 `doctor` MCP 工具，跑返回的 shell 命令，按 checklist 展示 |
| `ctx upgrade` | 调 `upgrade` MCP 工具，跑返回的 shell 命令，按 checklist 展示 |
| `ctx purge` | 调 `purge` MCP 工具（confirm: true）。清知识库前会警告 |

`/clear` 或 `/compact` 后：知识库和会话统计保留。想重开用 `ctx purge`。
