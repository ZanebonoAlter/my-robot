# change-scope-gate Delta

## MODIFIED Requirements

### Requirement: quality-gate turn_end 跑影响包 go test

`.pi/extensions/quality-gate.ts` 的 turn_end 门禁 SHALL 按「本回合新增变化」增量路由两侧门禁，并在本轮改动命中后端时调用 change-scope（`--json`）判定影响包，对 `tier=domain` 的目标自动执行对应 `go test`。

**增量路由**：门禁 SHALL 维护会话内改动快照——会话边界（session_start）以当时 git 状态（tracked diff + untracked）初始化，每回合 turn_end 门禁判定后更新为当前 git 状态。本回合触发集 = 当前 git 改动中相对快照新增或内容变化的路径。后端门禁（golangci-lint / go vet / go build / domain go test）仅当触发集命中后端路径（`backend-go/**.go` 等）时执行；前端门禁（仅 `pnpm lint`）仅当触发集命中前端路径（`front/` 非 .md）时执行。会话开始前已存在的工作区残留改动 SHALL NOT 触发任何门禁（非本会话 agent 所为，全量验证由归档门禁兜底）；被跳过的一侧 MUST 不产生 gate.check 事件（与「未运行不记账」一致）。

**执行约束**（既有语义保留）：

- 每个 go test 目标有单条超时保护（约 120s），总预算保护（约 5min）；超时输出 warn 不计失败。
- go test 统一带 `-short` 标志：DB 集成测试自动 skip（无 Docker 不挂、有 Docker 不耗时），完整集成测试由 agent 手动或归档门禁执行。
- `tier=platform` 在门禁中 SHALL 只执行 `go vet`；`tier=skeleton` 只执行 `go build` + `go vet`。
- 测试失败与 lint 失败同机制处理（steer 消息回喂，agent 必须修复或显式说明豁免理由）。
- 前端门禁维持仅 `pnpm lint`（cmd.exe 限制不变）。

#### Scenario: domain 改动触发自动测试

- **WHEN** 本轮编辑修改了 `backend-go/internal/reader/handler/list.go` 且 turn_end 门禁运行
- **THEN** 门禁执行 `go test ./internal/reader`，失败时以 steer 消息回喂失败输出

#### Scenario: DB 依赖包跳过

- **WHEN** 改动命中 `DB_DEPENDENT_PKGS` 清单内的包
- **THEN** 门禁跳过该包 go test，输出「跳过（需 Docker DB）：<pkg>」

#### Scenario: 前端改动行为不变

- **WHEN** 本轮有前端新变化（修改 `front/` 下非 .md 文件）
- **THEN** 门禁仅执行 `pnpm lint`，不尝试执行任何需 cmd.exe 的命令

#### Scenario: 残留前端脏文件不触发前端门禁

- **WHEN** 会话开始前工作区已存在前端脏文件（残留 diff/untracked），本回合 agent 仅编辑后端 `.go` 文件
- **THEN** 门禁执行后端全套但不执行 `pnpm lint`，且无 pnpm lint 的 gate.check 事件

#### Scenario: 会话前残留改动不触发首轮门禁

- **WHEN** 会话启动时工作区含上个会话遗留的后端+前端脏文件，本回合 agent 未编辑任何代码（纯对话）
- **THEN** 触发集为空，不执行任何门禁命令，本回合零 gate.check 事件

#### Scenario: 修复循环每轮按新变化触发对应侧

- **WHEN** 前一回合后端编译失败（steer 回喂），本回合 agent 修复该 `.go` 文件
- **THEN** 该文件相对快照有新变化，后端门禁重新执行；若本回合未触碰前端，前端门禁继续跳过
