# change-scope-gate Specification（delta）

## ADDED Requirements

### Requirement: 改动范围→验证命令判定（change-scope 脚本）

系统 SHALL 提供 `scripts/change-scope.sh`，对给定 base ref（默认 `HEAD`）收集改动文件集合（committed/staged/unstaged/untracked，与 doc-impact.sh `changed_files()` 同口径），并按路径映射表输出建议执行的最小验证命令清单。脚本 SHALL 在 WSL bash 环境可运行且不触发任何编译命令的执行（只输出命令文本，不代替执行）。

映射表 SHALL 按目录结构自动发现 domain 档位，无需硬编码白名单维护：

| 改动路径 | 输出 |
|---|---|
| `backend-go/internal/<domain>/**`（domain = internal/ 下除 app/models/platform 外的目录，运行时发现） | `go test -short ./internal/<domain>/...`（递归；DB 集成测试在 -short 下自动 skip，无需 Docker） |
| `backend-go/internal/platform/<pkg>/**` | `go test ./internal/platform/<pkg>` + 升级提示（`go vet ./...`） |
| `backend-go/internal/{app,models}/**` 或 `backend-go/cmd/**` | `go build ./...` + `go vet ./...`（不自动 test） |
| `backend-go/go.mod` 或 `go.sum` | 提示全量 `go test ./...` |
| `front/**`（非 .md） | `pnpm lint`；typecheck/test:unit/build 标注「需 cmd.exe」 |
| `docs/**`、`*.md`、`scripts/**`、`.pi/**` | 无测试命令，提示文档/规范一致性检查 |

#### Scenario: domain 包改动

- **WHEN** 工作区改动含 `backend-go/internal/daily_report/service/foo.go`
- **THEN** 脚本输出含 `go test ./internal/daily_report`

#### Scenario: 新增 domain 零维护

- **WHEN** `backend-go/internal/` 下新增目录 `newdomain` 且改动含 `backend-go/internal/newdomain/x.go`
- **THEN** 脚本无需修改即输出 `go test ./internal/newdomain`

#### Scenario: 骨架变更不自动 test

- **WHEN** 改动仅含 `backend-go/internal/app/router.go`
- **THEN** 脚本输出 `go build ./...` 与 `go vet ./...`，且不输出任何 `go test` 命令

#### Scenario: 前端命令标注跨平台限制

- **WHEN** 改动含 `front/app/components/Foo.vue`
- **THEN** 脚本输出的 `pnpm lint` 可在 WSL 执行，typecheck/test:unit/build 命令附带「需 cmd.exe」标注

#### Scenario: 未命中路径不猜测

- **WHEN** 改动含映射表未覆盖的路径（如仓库根新建 `tool.exe`）
- **THEN** 脚本输出「无法判定，请手动选择验证命令」，不输出任何猜测性测试命令

### Requirement: 机器可读输出（--json）

`scripts/change-scope.sh --json` SHALL 输出合法 JSON（单行或多行均可），包含字段：

- `base`：实际使用的 base ref
- `paths`：改动文件路径数组
- `testTargets`：判定出的可执行测试目标数组（每项含 `cmd` 命令文本与 `tier` 档位：`domain` / `platform` / `skeleton` / `frontend`）
- `notices`：提示文本数组（升级提示、需 cmd.exe 标注、无法判定警告）

#### Scenario: quality-gate 消费

- **WHEN** 以 `--json` 运行且改动含 `backend-go/internal/topicgraph/`
- **THEN** 输出可被 `JSON.parse` 解析，且 `testTargets` 含 `{"cmd":"go test ./internal/topicgraph","tier":"domain"}`

### Requirement: quality-gate turn_end 跑影响包 go test

`.pi/extensions/quality-gate.ts` 的 turn_end 门禁 SHALL 在本轮改动命中后端时调用 change-scope（`--json`）判定影响包，并对 `tier=domain` 的目标自动执行对应 `go test`；执行 SHALL 满足：

- 每个 go test 目标有单条超时保护（约 120s），总预算保护（约 5min）；超时输出 warn 不计失败。
- go test 统一带 `-short` 标志：DB 集成测试自动 skip（无 Docker 不挂、有 Docker 不耗时），完整集成测试由 agent 手动或归档门禁执行。
- `tier=platform` 在门禁中 SHALL 只执行 `go vet`；`tier=skeleton` 只执行 `go build` + `go vet`。
- 测试失败与 lint 失败同机制处理（steer 消息回喂，agent 必须修复或显式说明豁免理由）。
- 前端改动维持仅 `pnpm lint`（cmd.exe 限制不变）。

#### Scenario: domain 改动触发自动测试

- **WHEN** 本轮编辑修改了 `backend-go/internal/reader/handler/list.go` 且 turn_end 门禁运行
- **THEN** 门禁执行 `go test ./internal/reader`，失败时以 steer 消息回喂失败输出

#### Scenario: DB 依赖包跳过

- **WHEN** 改动命中 `DB_DEPENDENT_PKGS` 清单内的包
- **THEN** 门禁跳过该包 go test，输出「跳过（需 Docker DB）：<pkg>」

#### Scenario: 前端改动行为不变

- **WHEN** 本轮仅修改 `front/` 下文件
- **THEN** 门禁仅执行 `pnpm lint`，不尝试执行任何需 cmd.exe 的命令

### Requirement: 规则文本与判定命令一致

AGENTS.md「测试只跑本次修改影响的包」段落及 `docs/reference/开发执行规范.md` 相应段落 SHALL 指向 `bash scripts/change-scope.sh` 作为权威判定方式，替代纯文字描述的自觉执行。

#### Scenario: 文档指向脚本

- **WHEN** 开发者/agent 需确定本次改动应跑哪些测试
- **THEN** AGENTS.md 与开发执行规范的相关段落可检索到 `scripts/change-scope.sh` 命令引用
