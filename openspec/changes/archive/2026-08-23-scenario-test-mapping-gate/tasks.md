# Tasks — scenario-test-mapping-gate

> 顺序：先写 smoke（1.1，验收基准）再实现脚本（1.2）最后集成（组 2）——脚本与 smoke 均为纯 bash，同 change 落地即符合 case-first-testing 顺序解绑。组 4 尾节映射表为 dogfooding：本 change 归档时须过自己的门禁。

## 1. scenario-trace.sh 对账脚本

- [x] 1.1 写 `scripts/scenario-trace.smoke.sh`：fixture 驱动自测（临时目录拼装 change 目录，旁置被测脚本），七类 case 逐个断言退出码与关键输出——①映射齐全+多文件单元格过 ②缺映射 FAIL 且输出列全部待映射标题 ③映射文件不存在 FAIL ④人工映射过 ⑤无 delta specs 直接过 ⑥验证节/映射表缺失 FAIL ⑦REMOVED 节 Scenario 不计入对账。验证：case 清单与 scenario-trace-gate spec 的 Scenario 一一对应
- [x] 1.2 实现 `scripts/scenario-trace.sh <change-dir>`（design D1-D5）：delta spec 解析（ADDED/MODIFIED/RENAMED 节下 `#### Scenario:` 计入、REMOVED 忽略）→ tasks.md `^## [0-9]+\. 验证` 节内表头 `| Scenario | 测试文件 |` 的表格提取 → 标题逐字匹配（裁首尾空白）→ 测试文件仓库根相对路径存在性（多路径逐一校验）→ `人工` 前缀放行；中文输出逐条列失败原因、退出码 0/1；风格对齐 doc-impact.sh（`set -u` / REPO_ROOT / 头部注释写用法与退出码）。验证：`bash scripts/scenario-trace.smoke.sh` 退出 0（实测 9/9 全过）

## 2. spec-gate 检查④集成

- [x] 2.1 `.pi/extensions/spec-gate.ts` 新增检查④：`runScript(pi, ctx, ["scripts/scenario-trace.sh", changeDir])`，与前三项独立判定、失败 detail 并入 reason，修复指引追加「验证节补 `| Scenario | 测试文件 |` 映射表」条目，头部注释三处同步（检查清单、设计决策、失败指引）。验证：esbuild 打包零报错（51ms）+ 替换点逐个 grep 命中（实测 10/10）

## 3. 文档

<!-- doc-impact: none(工具链门禁：scripts/scenario-trace*.sh 与 .pi/extensions/spec-gate.ts 均无七域路径命中；docs/reference/开发执行规范.md 属流程规范文档不在七域，改动为 3.1 的门禁条目同步；主 spec 由 delta 归档合并) -->
<!-- doc-impact-excuse: flow=另一线程 backend-go 脏文件命中，非本 change 改动; api=同上; database=同上; architecture=同上; standard=同上; configuration=同上 -->

- [x] 3.1 `docs/reference/开发执行规范.md` 五处：§11.1 归档条件补第④项（含标题三→四）与 Scenario 映射对账细则、§4.1 门禁分层表 spec-gate 行补「④ Scenario 映射对账」、§0.6 步骤 2 的「映射表雏形」表述改为指向 scenario-trace-gate 的格式约定、§11.2 验证节结构补映射表要求。验证：`grep -n "scenario-trace" docs/reference/开发执行规范.md` 多节命中（实测 §0.6/§4.1/§11.1/§11.2 四节）
- [x] 3.2 （2026-08-23 归档后补记）纯工具链 change，无 flow 影响（E 段豁免声明：本 change 仅触及 scripts/·spec-gate·执行规范门禁条目，不涉任何业务 flow 文档；漏声明致 check-standards E 段 FAIL 卡后续归档，由 doc-authoring-guide change 顺手补手续）

## 4. 测试

- 影响包（change-scope）：`scripts/**` + `.pi/**` → 无编译测试命令，提示文档一致性检查；本 change 的测试资产即 1.1 的 smoke
- 冒烟：`bash scripts/scenario-trace.smoke.sh`（七类 fixture 全断言）
- spec-gate 打包检查：`cd .pi/extensions/tests && npx -y esbuild ../spec-gate.ts --bundle --platform=node --format=cjs --outfile=/dev/null`

## 5. 验证

- [x] 5.1 `bash scripts/scenario-trace.smoke.sh` → 期望退出 0（实测 9/9 全过）
- [x] 5.2 `bash scripts/scenario-trace.sh openspec/changes/scenario-test-mapping-gate` → 期望退出 0（实测：✓ 13 个 Scenario 映射齐全，自动 9 / 人工 4；曾逮住两处自犯错误——映射表误放测试节、行标题与 delta 标题不一致——均已修）
- [x] 5.3 `cd .pi/extensions/tests && npx -y esbuild ../spec-gate.ts --bundle --platform=node --format=cjs --outfile=/dev/null` → 期望零报错（实测 22ms Done）
- [x] 5.4 `bash scripts/check-standards.sh` → 期望退出 0（实测 109 过 / 1 败，唯一 FAIL 为并行线程新 change fix-mode-recovery-cross-session 的 doc-impact 声明缺失，与本 change 无关；本 change 相关项全过且 F 段单独 [OK]）
- [x] 5.5 `bash scripts/doc-impact.sh verify openspec/changes/scenario-test-mapping-gate` → 期望退出 0（实测通过）
- [x] 5.6 `grep -n "scenario-trace" docs/reference/开发执行规范.md` → 期望多节命中（实测 §0.6/§4.1/§11.1/§11.2 四处命中）
- [x] 5.7 归档实战端到端：`openspec archive scenario-test-mapping-gate` 被 spec-gate 四项检查放行（映射对账在 5.2 已预演）→ 期望归档成功（实测：四项检查全过零阻断，归档为 2026-08-23-scenario-test-mapping-gate，主 specs 同步成功：3 added + 1 modified，scenario-trace-gate/spec.md 新建、case-first-testing 归档对账 requirement 升级为机器强制）

Scenario → 测试文件映射（scenario-trace.sh 归档对账，13 个 delta Scenario；case 编号对应 smoke 七类：①齐全 ②缺映射 ③文件不存在 ④人工 ⑤无 delta ⑥验证节缺失 ⑦REMOVED）：

| Scenario | 测试文件 |
| --- | --- |
| 映射齐全通过 | scripts/scenario-trace.smoke.sh |
| 缺映射阻断 | scripts/scenario-trace.smoke.sh |
| 映射文件不存在阻断 | scripts/scenario-trace.smoke.sh |
| 人工映射合法 | scripts/scenario-trace.smoke.sh |
| 无 delta Scenario 直接过 | scripts/scenario-trace.smoke.sh |
| 定位不到验证节 | scripts/scenario-trace.smoke.sh |
| 多文件单元格 | scripts/scenario-trace.smoke.sh |
| 归档时自动对账 | 人工（spec-gate ④为 runScript 直调一行，与既有三项同模式；本 change 归档实战即为端到端验证） |
| 逃生口留痕 | 人工（--force/SPEC_GATE_BYPASS 为既有代码路径，本 change 未改） |
| REMOVED 节不计入 | scripts/scenario-trace.smoke.sh |
| 顺序解绑 | 人工（流程约定 scenario，无对应代码行为） |
| bug 修复先复现 | 人工（流程约定 scenario，无对应代码行为） |
| 归档对账 | scripts/scenario-trace.smoke.sh |
| 人工验证映射留痕 | scripts/scenario-trace.smoke.sh |
