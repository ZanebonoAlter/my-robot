# Design: scenario-test-mapping-gate

## Context

- case-first-testing 的 SHALL（验证节列映射表）落地率 0：spec-gate.ts（219 行）只查尾三节存在性，`scripts/` 无追溯工具。主 specs Scenario 已 1335 个且持续增长。
- 既有可复用形态：spec-gate 已有 `runScript` 调用模式、三项检查独立判定、fail-open（扩展自身异常放行留痕）、`--force` / `SPEC_GATE_BYPASS=1` 逃生口；scripts/ 门禁族（doc-impact.sh、change-scope.sh）均为 bash + `set -u` + REPO_ROOT 推导 + 中文输出 + 头部注释写明用法与退出码，定位「只判不跑」。
- smoke 基建：`.pi/extensions/tests/run-smoke.sh` 是 TS extension 专用（esbuild 打包 + node 事件回放），不适配纯 bash 脚本；spec-gate.ts 自身无 smoke 先例。

## Goals / Non-Goals

**Goals:**

- 归档对账机械化：映射齐全 + 测试文件存在，两件可机械判定的事交给机器
- 映射表格式极简，手写无负担（人在验证节自然就会写的那种表）
- 与既有门禁族形态完全一致（bash、只判不跑、退出码约定、逃生口）

**Non-Goals:**

- 不执行测试（quality-gate / 归档手动验证的职责，不混）
- 不做覆盖率度量或阈值（审计报告 ③ 覆盖率基线是独立 change）
- 不追溯主 specs 存量 1335 个 Scenario（只对 change 的 delta 对账）
- 不校验映射的语义相关性（映射到不相关文件属 review 范畴）
- 不改 in-flight change 内容（watch-keyword / constraint-injection-tier-b 归档时自补映射表）

## Decisions

**D1：标题级匹配，不做 capability 限定复合键。**
Scenario 标题在单个 change 的 delta 内天然接近唯一；跨 spec 重名时一行覆盖全部同名实例。备选 `capability/标题` 复合键被拒：表格啰嗦，与人在验证节的自然写法不符，且防重名收益极小。

**D2：只判不跑。**
存在性判定秒级完成、无 Docker/编译依赖，与 doc-impact verify / change-scope.sh 同款定位。备选「顺带跑映射到的测试」被拒：超时预算、职责、失败归因全部变糊。

**D3：对账范围 = ADDED / MODIFIED / RENAMED，REMOVED 忽略。**
删除的场景不再需要测试保障；RENAMED 语义是"需求改名、行为仍在"，其 Scenario 照常计入。

**D4：映射表锚定与单元格规则。**
- 验证节定位锚 `^## [0-9]+\. 验证`（与 spec-gate 检查③同锚点，③已拦截节缺失，④报错时给出更具体指引）
- 表头必须为 `| Scenario | 测试文件 |`：验证节常有其他表格（命令+期望结果），固定表头防误读
- 单元格：首尾空白裁剪；测试文件为仓库根相对路径，多个以空白或逗号分隔；`人工` 前缀（如 `人工（UI 目视）`）视为合法映射放行
- 标题含 `|` 等 markdown 表格保留字符不支持（delta spec 标题本就不该含管道符，遇到就改标题）

**D5：bash 实现，风格对齐 doc-impact.sh。**
`set -u`、REPO_ROOT 推导、中文输出、头部注释写明用法与退出码（0 过 / 1 失败）。备选 node 实现被拒：scripts/ 门禁族全 bash，维护一致性优先。

**D6：spec-gate 集成复用既有机制。**
`runScript(pi, ctx, ["scripts/scenario-trace.sh", changeDir])`，与 doc-impact verify / check-standards 共用既有 TIMEOUT_MS 预算（本地文件读取，秒级，预算压力为零）；失败输出 tail 并入 reason，修复指引追加「补映射表」条目。脚本退出非 0 = 检查失败（block）；扩展自身异常仍 fail-open 放行留痕（既有语义不变）。

**D7：smoke 用 fixture 驱动 bash 自测。**
`scripts/scenario-trace.smoke.sh`：临时目录拼装四类 fixture change（映射全过 / 缺映射 FAIL / 人工映射过 / 无 delta spec 直接过 + 多文件单元格与文件不存在两个附加 case），逐个断言退出码与关键输出，自包含零依赖、旁置被测脚本。spec-gate ④ 本体是 runScript 直调一行，不单独建 smoke（与既有三项检查同待遇）。

**D8：无豁免机制、不豁免 in-flight。**
新检查对包括 watch-keyword-and-quickadd、constraint-injection-tier-b 在内的所有后续归档一视同仁——映射表可在实现完成前任何时候补（十分钟级），豁免名单反而制造永久后门。逃生口沿用 `--force` / `SPEC_GATE_BYPASS=1`。

**D9：文档落点。**
`docs/reference/开发执行规范.md` §11.1 归档条件补第④项、§4.1 门禁分层表 spec-gate 行补「④ Scenario 映射对账」、§0.6 步骤 2 的「映射表雏形」表述改为指向本格式约定。归档时随 delta 同步主 specs。

## Risks / Trade-offs

- [映射表形式合规但语义不相关（映射到不相干测试文件）] → 脚本只验存在性；语义相关性归 review，与 doc-impact verify「机械查可机械查的」同哲学
- [「人工」前缀滥用（全写人工绕过门禁）] → 留痕可见，归档 review 与 tasks.md 文档节可查；不引入配额类过度设计
- [标题逐字匹配对大小写/全半角漂移零容忍] → 有意为之（防"差不多就过"）；报错输出全部待映射标题，复制即用
- [验证节标题变体（「验证：」等）导致锚定失败] → 检查③同锚点先拦一道；④报错信息直接给出期望格式
- [未来 delta spec 出现 REMOVED 之外的忽略语义需求] → 脚本对账范围枚举在头部注释，改一处即可

## Migration Plan

纯增量工具链 change：脚本 + smoke + spec-gate 一段 + 文档三处。合入即对所有后续 `openspec archive` 生效。回滚 = revert 单个 commit，无数据、无状态。

## Open Questions

无——映射表格式、对账范围、smoke 形态均已定，属实现细节的留给 tasks。
