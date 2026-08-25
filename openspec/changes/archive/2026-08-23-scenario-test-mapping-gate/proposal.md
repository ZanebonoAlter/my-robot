# Proposal: scenario-test-mapping-gate

## Why

`case-first-testing` 主 spec 已成文 SHALL：「tasks.md 验证节列 Scenario→测试文件映射表，归档时未映射的 Scenario 视为未覆盖」——但落地率为 0：spec-gate.ts 只查尾三节**存在性**，`scripts/` 下无任何追溯工具，归档对账全靠自觉（抽查近期 3 个归档档，验证节均为「跑哪些包 + 手动查验」，无一有映射表）。主 specs 的 Scenario 已达 1335 个且持续增长，「以测试用例作为 change 验收基准」是用户明确的工程目标，缺的就是把这根线变成机械校验的牙齿。

## What Changes

- **新增 `scripts/scenario-trace.sh`**：归档前对账脚本（与 doc-impact verify / change-scope.sh 同为「只判不跑」）：
  1. 解析 change delta specs（`openspec/changes/<name>/specs/**/*.md`）的 `#### Scenario:` 标题（ADDED/MODIFIED 节计入，REMOVED 节忽略——删除的场景不再需要测试保障）
  2. 校验 tasks.md「验证」节存在机器可读映射表，逐 Scenario 覆盖（标题逐字匹配）
  3. 检查映射的测试文件路径真实存在于工作区（不执行测试）
  4. 无自动化测试的 Scenario 允许显式标 `人工`（映射表中留痕，与 case-first-testing 的「视为未覆盖」语义对齐：人工验证是合法映射值，缺失才是未覆盖）
- **spec-gate.ts 加检查④**：`bash scripts/scenario-trace.sh <changeDir>` 退出码 0，复用现有 runScript 模式、超时预算与失败输出格式；任一失败仍走既有 block + 修复指引；逃生口沿用既有 `--force` / `SPEC_GATE_BYPASS=1`，不新增豁免机制
- **映射表格式约定**（机器可读）：验证节内 markdown 表格 `| Scenario | 测试文件 |`，一行一 Scenario；测试文件为相对仓库根路径（`backend-go/..._test.go`、`front/...*.test.ts`、e2e `*.spec.ts`）
- **in-flight change 不豁免**：watch-keyword-and-quickadd（0/43）、constraint-injection-tier-b（0/19）归档时同样受检，届时补映射表即可（早于实现完成，成本≈0）
- **文档**：`docs/reference/开发执行规范.md` §11.1 归档条件补第④项、§4.1 门禁分层表同步；§0.6 步骤 2 的「映射表雏形」表述与格式约定对齐
- **smoke test**：`scripts/scenario-trace.sh` 用 fixture change 目录（有映射全过 / 缺映射 FAIL / 标人工通过 / 无 delta spec 直接过 四种）+ spec-gate ④ 冒烟，仿 `.pi/extensions/tests/*.smoke.cjs` 模式接入 run-smoke.sh

## Capabilities

### New Capabilities

- `scenario-trace-gate`: Scenario→测试文件映射的归档机器校验（scenario-trace.sh 行为契约、映射表格式、人工映射边界、与 spec-gate 的集成）

### Modified Capabilities

- `case-first-testing`: 「测试代码顺序解绑与归档对账」requirement 的归档对账 scenario 升级——从「tasks.md 验证节存在映射」（自觉遵守）改为「spec-gate 检查④机器强制：scenario-trace.sh 退出码 0，未映射的 Scenario 阻断归档」

## Impact

- 代码：`.pi/extensions/spec-gate.ts`（+检查④，约 20 行）、`scripts/scenario-trace.sh`（新增，纯 bash）、`.pi/extensions/tests/`（smoke fixtures）
- 文档：`docs/reference/开发执行规范.md` §0.6 / §4.1 / §11.1；`openspec/specs/case-first-testing/spec.md`（随归档同步）
- 无业务代码、无 DB、无前端改动
- 部署影响：归档门禁行为变严——所有后续 `openspec archive` 均要求映射表对账通过（含两个 in-flight change）；逃生口不变
