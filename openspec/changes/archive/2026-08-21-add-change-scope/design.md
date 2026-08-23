# Design: add-change-scope

## Context

- 「测试只跑影响包」（AGENTS.md / 开发执行规范 §4.1）目前靠 agent 读规则自觉执行；`.pi/extensions/quality-gate.ts` 因「影响包无法自动判定（codegraph affected 实测误报）」在 turn_end 只跑 lint 不跑 go test。
- 现有同类基础设施：`scripts/doc-impact.sh` 已解决「改动→文档域」映射（`changed_files()` 收集 committed/staged/unstaged/untracked，含 DrvFS `core.checkStat=minimal` 优化），证明「路径正则白名单 + 保守回退」在本仓库 WSL 环境下可靠且够快。
- 参考方案 deepseek-harness `change-scope.ts` 的核心思想：**不猜语义，只报事实**（diff 路径集合）+ 消费方按显式映射选最小验证集，「永不反射性跑全量」。

## Goals / Non-Goals

**Goals:**

- 一条命令回答「这次改动该跑哪些验证」：`bash scripts/change-scope.sh [--base <ref>]`。
- quality-gate turn_end 从「后端只 lint」升级为「lint + 影响包 `go test`」，让 AGENTS.md 的规则变成机械执行。
- 新增 domain 包时**零维护**：映射自动发现，不需要往脚本里加白名单。

**Non-Goals:**

- 不做 import 链传播分析（改 `platform/ws` 波及 `daily_report` 这类语义影响不推断——共享包按「升级提示」处理，见决策 3）。
- 不自动化前端 cmd.exe 编译命令（跨平台限制维持现状，脚本只提示该跑什么）。
- 不替换/重构 doc-impact.sh（两脚本文件收集逻辑各自独立，见决策 5）。
- 不跑需要 Docker DB 的 integration 测试（只提示，见决策 4）。

## Decisions

### 1. bash 而非 TS/Node（同 dsh 的形态差异）

dsh 的 change-scope 是 TS 因为它是纯 Node monorepo；本仓库 `scripts/` 现状全是 bash（doc-impact.sh / check-standards.sh），WSL bash 可跑、无构建步骤。**替代方案**（否决）：TS + node 运行——引入 tsx/构建依赖，与现有脚本栈割裂。

### 2. 双输出：默认人类可读，`--json` 供 quality-gate 消费

quality-gate.ts 是 TS，解析自由文本脆弱。`--json` 输出 `{base, paths:[...], testTargets:[...], notices:[...]}`；默认输出带分组的可读清单 + 每条命令可复制执行。两个消费者（人 / extension）各有稳定接口。

### 3. 映射规则：目录结构自动发现 + 三档分类

对 `backend-go/internal/` 下的目录**运行时发现**（`ls` 而非硬编码白名单），归三档：

| 改动路径 | 档位 | 输出命令 |
|---|---|---|
| `internal/<domain>/**`（domain ∈ admin/dataenrichment/reader/tagmanagement/topicgraph，**自动发现**：internal 下非 app/models/platform 的目录） | domain 包 | `go test ./internal/<domain>` |
| `internal/platform/<pkg>/**` | platform 子包 | `go test ./internal/platform/<pkg>` + 升级提示（platform 被谁 import 不可知，提示 `go vet ./...`） |
| `internal/{app,models}/**`、`cmd/**` | 骨架/共享 | `go build ./...` + `go vet ./...`（不自动 test） |
| `go.mod` / `go.sum` | 依赖 | 提示全量 `go test ./...` |
| `front/**`（非 .md） | 前端 | `pnpm lint`（WSL 安全）；typecheck/test:unit/build 标注「需 cmd.exe」 |
| `docs/**` / `*.md` / `scripts/**` / `.pi/**` | 非代码 | 无测试命令，提示文档/规范一致性检查 |
| 其他未命中 | 回退 | **不猜**，打印「无法判定，请手动选择」（吸取 codegraph affected 误报教训） |

**替代方案**（否决）：显式白名单数组（同 check-standards.sh B 段 DOMAIN_WHITELIST）——新增 domain 忘更新会静默漏测；自动发现 + 显式档位判断零维护。

### 4. quality-gate 集成：统一 `go test -short`，不维护 DB 跳过清单

- 摸底实测（2026-08-21，有 Docker 环境）推翻「需要 DB_DEPENDENT_PKGS 清单」预设：`SetupTestDB` 在 `-short` 下 `t.Skip`（不 fail、不起容器）→ 门禁统一 `go test -short`：无 Docker 时 DB 测试自动 skip，有 Docker 时也不浪费时间起容器（完整集成测试留给 agent 手动/归档门禁不带 -short 跑）。实测五 domain `-short` 全绿、总耗 15s（含编译）；dataenrichment 全量无 DB 仅 2s。
- 映射命令必须用 `./internal/<domain>/...` 递归：domain 包根基本无测试文件（仅 dataenrichment 有 4 个），非递归等于空跑。
- turn_end 流程：`bash scripts/change-scope.sh --json` → 解析 testTargets → 对 domain 档逐条 `go test -short`（每条超时 ~120s，总预算 ~5min，超时只 warn 不算失败）。
- platform 档在门禁里只跑 vet 不跑 test（platform 测试面广、易被 DB 依赖污染）；骨架档只 build+vet。
- 失败仍走现有 steer 回喂机制。

### 5. 与 doc-impact.sh 的关系：复制而非复用

`changed_files()` ~15 行是两脚本唯一重叠。抽公共库（source 共享 .sh）为省 15 行引入耦合 + 加载路径复杂度，不值。复制时保留 DrvFS/quotepath 注释出处。**替代方案**（否决）：`source scripts/lib/changed-files.sh`——两个门禁独立演进，doc-impact 有自己的豁免机制，强行共享反而锁死。

### 6. base ref 语义

默认 `HEAD`（工作区改动视角，匹配 turn_end「本轮编辑还没提交」场景）；`--base <ref>` 供手动审阅用（如 `--base main` 看 change 全量）。与 doc-impact.sh `--base` 语义一致，心智模型统一。

## Risks / Trade-offs

- [映射漏报：新增目录结构（如 backend-go/internal 下新分层）未覆盖] → 回退档显式提示「无法判定」，永远不静默绿。check-standards.sh 后续可加「档位规则与实际目录结构一致性」校验（本 change 不做，留验证节备注）。
- [go test 慢导致 turn_end 卡顿] → 超时 + 总预算双保护；跳过清单可随摸底扩充；失败不阻塞对话（steer 提示后由 agent 决定修还是申请豁免）。
- [quality-gate 每轮跑重复测试] → 幂等可接受（正确性优先于速度）；未来可加「本轮 diff 未触及该包则跳过」优化，非首版目标。
- [自动发现把误建目录当 domain] → 目录档位判断同时被 check-standards.sh B 段 DOMAIN_WHITELIST 结构约束兜底（结构检查在归档门禁跑）。

## Migration Plan

纯增量：新脚本 + extension 行为升级，无数据/接口迁移。回滚 = 还原 quality-gate.ts 两处调用 + 删脚本。

## Open Questions

（无——映射档位与跳过清单依赖任务 1 摸底实测，属执行期确定的事实，不阻塞设计。）
