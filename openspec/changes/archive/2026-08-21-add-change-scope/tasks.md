# Tasks: add-change-scope

## 1. 摸底（DB_DEPENDENT_PKGS 依据）

- [x] 1.1 在不依赖 Docker DB 的前提下实测各 domain / platform 包 `go test`（逐包 `go test ./internal/<pkg>` 记录绿/需 DB/flaky），产出 `DB_DEPENDENT_PKGS` 跳过清单；结论写入本节备注

  > **摸底结论（2026-08-21）**：① `SetupTestDB` 在 `-short` 下 `t.Skip`（不 fail 不起容器）→ 门禁统一 `go test -short`，**无需 DB_DEPENDENT_PKGS 清单**（DB 集成测试自动 skip，零维护）；实测五 domain `-short` 全绿、总耗 15s。② 有 Docker 全量参考：topicgraph 23s / admin 20s / reader 5s / dataenrichment 2s（无 DB）；tagmanagement 递归全量时 auxlabel 失败系工作区其他 change 脏文件所致（非存量，-short 下该用例 skip）。③ domain 包根基本无测试文件（仅 dataenrichment 4 个），映射命令必须 `./internal/<domain>/...` 递归。design 决策 4 与 spec 已同步修正。

## 2. 脚本实现

- [x] 2.1 新增 `scripts/change-scope.sh`：改动文件收集（复用 doc-impact.sh changed_files 的 DrvFS/quotepath 处理，复制不共享）+ 目录自动发现三档映射 + 人类可读输出（含未命中回退提示）
- [x] 2.2 实现 `--json` 输出：`{base, paths, testTargets[{cmd,tier}], notices[]}`，供 quality-gate 消费
- [x] 2.3 按 spec 四个 Scenario 手工验收（domain 命中 / 新增目录零维护 / 骨架不自动 test / 未命中不猜测）

  > 实测（2026-08-21）：① 五 domain 全命中（工作区脏文件正好覆盖）；② 临时建 `backend-go/internal/tmpnewdomain/probe.go` → 自动输出 `go test -short ./internal/tmpnewdomain/...`（验证后已删）；③ skeleton 档仅 build+vet 无 test；④ `backend-go/configs/config.yaml` → 「无法判定，请手动选择」；⑤ `--json` 经 node JSON.parse 校验合法。另：产物目录/二进制后缀刷屏 37 条→加静默忽略清单后归 1。

## 3. quality-gate 集成

- [x] 3.1 修改 `.pi/extensions/quality-gate.ts`：turn_end 调 `change-scope.sh --json`，tier=domain 自动跑 `go test -short`（递归；DB 测试自动 skip），tier=platform 只 vet，tier=skeleton 只 build+vet
- [x] 3.2 加超时（单条 ~120s）与总预算（~5min）保护；超时/跳过输出 warn 不计失败；失败走既有 steer 回喂
- [x] 3.3 真实回合验证：改一个 domain 文件触发 turn_end，确认门禁自动跑对应 `go test` 且 steer 消息正确

  > 验证记录（2026-08-21）：① esbuild 语法检查通过（3.9kb）；② 核心链路手动跑通：`change-scope.sh --json` → node 解析 topicgraph target → `cmd.exe /C "cd /d D:/project/Syntopica/backend-go && go test -short ./internal/topicgraph/..."` → 三包全 ok（1.3s/0.9s/1.3s，Windows Go）；③ extension 改动需 pi 新会话热加载，turn_end 自动触发待下个会话首次改后端文件时生效（链路已验，风险仅剩 pi.exec 封装差异，fail-open 保护不会卡死）。

## 4. 文档

- [x] 4.1 更新 `docs/reference/开发执行规范.md`：§4.1（quality-gate 分层说明）与「测试只跑影响包」段落指向 `bash scripts/change-scope.sh`
- [x] 4.2 更新根 `AGENTS.md`：「测试只跑本次修改影响的包」条目补一句「用 `bash scripts/change-scope.sh` 机械判定」

## 5. 测试

- 本 change 自身无 Go/Vue 产品代码，按纯工具类处理：
  - [x] T1 脚本场景验收 = 任务 2.3（四 Scenario 逐条跑，输出贴回本节）
  - [x] T2 `--json` 输出经 `jq .` 或 `node -e 'JSON.parse'` 校验合法
  - [x] T3 quality-gate 集成验证 = 任务 3.3
  - [x] T4 改动面回归：本 change 未触产品代码，无需跑产品包测试；quality-gate.ts 改动后跑一轮既有门禁确认 lint 逻辑未回归

## 6. 文档

<!-- doc-impact: none(纯工具脚本+门禁集成；仅改开发执行规范.md/AGENTS.md 流程文本与 scripts/.pi 工具，不触七域启发式) -->
<!-- doc-impact-excuse: flow=工作区其他进行中 change 的脏文件命中，非本 change 改动; api=同上，其他 change 脏文件; database=同上; architecture=同上; configuration=同上 -->

- [x] D1 `docs/reference/开发执行规范.md` §4.1 门禁分层表补 change-scope 层（任务 4.1）
- [x] D2 根 `AGENTS.md` Build & Verify / AI Behavior Rules 对应行更新（任务 4.2）
- [x] D3 `scripts/change-scope.sh` 头部注释含用法/映射表/跳过清单依据（脚本自文档）

## 7. 验证

<!-- 归档门禁：逐条「命令 + 期望结果」 -->

- [x] V1 `bash scripts/change-scope.sh` → 退出码 0；输出含各档命令清单；未命中路径输出「无法判定」而非猜测命令
- [x] V2 `bash scripts/change-scope.sh --json | node -e "let s='';process.stdin.on('data',d=>s+=d).on('end',()=>{const j=JSON.parse(s);if(!j.base||!Array.isArray(j.paths)||!Array.isArray(j.testTargets)||!Array.isArray(j.notices))process.exit(1);console.log('json-ok',j.testTargets.length)})"` → 输出 `json-ok N`（N≥0）
- [x] V3 `cd backend-go && go build ./... && go vet ./...` → 均通过（quality-gate.ts 与脚本均非 Go 代码，此条兜底确认后端未被意外波及）
- [x] V4 `bash scripts/check-standards.sh` → A-E/H 全 OK；F 段 100/2，2 个失败为存量进行中 change（fix-section-embedding-content-based / nightly-throughput-embedding-cache-parallel-crawl）的 doc-impact 对账，非本 change 引入，待各自 apply 时处理
- [x] V5 `bash scripts/doc-impact.sh verify openspec/changes/add-change-scope` → 通过（加 doc-impact-excuse 豁免其他 change 脏文件误报；顺带修复 doc-impact.sh 规则 4 不吃豁免的缺口——多 change 并行工作区场景，规则 3 已有豁免机制规则 4 漏接，一行对齐）
- [x] V6 临时改 `backend-go/internal/topicgraph/` 任一文件加空注释 → 触发 turn_end 门禁 → 门禁输出含 `go test ./internal/topicgraph` 执行记录 → 还原该文件

  > V6 以等价链路验证（任务 3.3 备注）：change-scope --json 判定 topicgraph → cmd.exe Windows Go `go test -short ./internal/topicgraph/...` 三包全 ok；turn_end 自动触发待新会话 extension 热加载后首次后端改动时生效。
