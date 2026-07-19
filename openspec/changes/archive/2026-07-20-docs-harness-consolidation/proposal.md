# Proposal — docs-harness-consolidation

## Why

文档 harness 的规范方向（§12 archive 即家、flow 反向溯源）已定，但物理结构和验证机制没跟上，探索发现多处实锤：

1. **版本号目录淤积**：`docs/v1.1` ~ `docs/v1.3.4` 共 7 个目录 398 个 md 文件，§12.1 已降级为可选，但 README 仍在显著位置介绍，每次维护范围被动扩大；其中 `v1.3.1`/`v1.3.2`/`v1.3.4` 三个目录连 `SUMMARY.md` 都缺。
2. **user-guide 定位错误且已腐烂**：10 个文件中 6 个是 API 类文件（`reference/api/` 的拷贝/子集）且已漂移（如 `feeds/api.md` 的 `icon` 字段描述与 `reference/api/feeds.md` 不一致；`feeds/categories-api.md`、`feeds/opml-api.md` 同属此类）；`getting-started.md`、`tagging-flow.md` **带着未解决的 git 合并冲突标记被提交**，后者还引用早已删除的 `internal/domain/topicanalysis/` 包。
3. **文档更新靠自觉、无验证**：tasks.md 文档节自由书写，`docs/README.md` 已积累 9 个死链（`getting-started.md`、`userguide/*`×5、`reference/architecture/data-flow.md`、`experience/ENCODING_SAFETY.md`、`experience/LESSONS_LEARNED.md`），无任何脚本检查。
4. **执行规范过重**：`开发执行规范.md` 490 行，TDD 全文背诵等段落每个 change 重复消耗 token，而强制力并未增加。
5. **死重目录**：`docs/agents/` 引用不存在的 `CONTEXT.md`/`docs/adr/`（模板抄来的）；`docs/plans/` 44 个散装 plan 是 openspec 之外的平行宇宙。

## What Changes

1. **物理清淤**
   - 删除 `docs/user-guide/`：`getting-started` 有效内容并入 `docs/reference/development.md`；`tagging-flow`/`content-processing`/`reading-preferences` 仍有效的内容并入对应 flow 文档后整目录删除；API 拷贝直接删（唯一权威源 = `reference/api/`）。
   - 删除 `docs/agents/`（死模板）。
   - `docs/plans/` 44 个文件 `git mv` 至 `docs/archive/plans/`（保留历史，退出活跃视野）。
   - `docs/v1.x/` 物理保留不动（历史快照），`docs/README.md` 重写将其降级为"历史里程碑索引"，不再作为活跃维护面。
   - 修复 `docs/reference/development.md` 指向 `../getting-started.md` 的死链。
2. **选项式文档门禁（新增 `scripts/doc-impact.sh`）**
   - `menu` / `suggest` / `verify <change>` / `context` 四子命令；8 个固定文档域选项（flow/api/database/architecture/standard/configuration/deployment/none）。
   - apply 启动时跑 `suggest` 按 git diff 启发式预勾选，agent 逐项确认后以机器可读注释 `<!-- doc-impact: ... -->` 写入 tasks.md 文档节。
   - apply 同时跑 `context`：按 git diff 命中的代码 domain，dump 对应 flow 文档「业务约束与不变量」节全文，作为改代码前的约束上下文注入（前置必读，非归档断言）。
   - `verify` 归档前对账：未声明 FAIL；声明了未更新 FAIL；代码改了 handler 未声明 api（反向启发式）FAIL；声明 none 但启发式命中 FAIL。
   - `check-standards.sh` 新增 F 段（对每个 active change 跑 doc-impact verify）与 G 段（`docs/README.md` + `docs/reference/` 一级文档的 markdown 相对链接死链检查）。
3. **flow 定位升级（替代 user-guide）**
   - `flow/README.md` 改写：flow = 需求说明 + 链路设计 + 业务约束 + 代码索引 + 变更溯源。
   - 7 个 flow 文档统一五段式模板：`需求说明 → 链路设计(mermaid) → 业务约束与不变量 → 代码入口 → 变更溯源`；`check-standards.sh` A 段加五段式结构校验。
   - 业务约束归属规则写入 `开发执行规范.md` §0.5：业务不变量→flow 固定节；跨功能传导→coupling-map.md；代码写法→standard/；数据红线→testing.md。
   - 「业务约束与不变量」节双重身份：归档时 A 段校验其存在 + apply 时 `doc-impact.sh context` 按命中 domain dump 其全文作为改代码约束上下文——业务不变量从「被动写在 flow 里」升级为「主动注入 apply」。
4. **执行规范精简**：`开发执行规范.md` 490 → 约 200 行。铁律/门禁/归档纪律全保留；TDD 等完整工作流指到技能；重复描述删除。§0.6 第 2 步编入 doc-impact declare，§11.4 编入 doc-impact verify + 死链检查。
5. **配套同步**：`openspec/config.yaml` context、`.pi/prompts/opsx-apply.md`（declare 步骤）、`opsx-archive.md`（verify 步骤）、根/前后端 `AGENTS.md`（删 user-guide 引用、更新文档地图）。

## Capabilities

### New Capabilities

- `doc-impact-gate`：选项式文档影响门禁——apply 启动时 declare（8 选项 + 启发式预勾选 + 机器可读声明）并跑 `context` 注入业务约束上下文，归档前 verify（声明↔git diff 对账 + 反向启发式），进 check-standards.sh F/G 段。

### Modified Capabilities

- `docs-reference-layer`：flow/ 定位从"业务链路概要设计"升级为"需求说明 + 链路设计 + 业务约束 + 索引 + 溯源"，五段式模板强制；user-guide 目录删除。
- `userguide-directory`：整个 capability 移除（定位由 flow 五段式的「需求说明」节承接）。
- `docs-milestone-structure`：`user-guide/` 子目录 legacy 化（v1.x 历史保留，新里程碑不要求）；`docs/plans` 处置从「删除」改为「移至 docs/archive/plans」。
- `development-docs`：`开发执行规范.md` 精简至约 200 行，doc-impact 步骤编入 §0.6/§11。

## Impact

- **纯文档 + 脚本 change**：新增 `scripts/doc-impact.sh`；修改 `scripts/check-standards.sh`；docs/ 下大量删除/改写/移动；不改任何产品代码。
- **工作流影响**：之后每个 change 的 apply 启动多两步——跑 `suggest` 做文档域 declare、跑 `context` 读相关 flow 业务约束；归档门禁多出 F/G 段校验。历史存量 archive change 不追溯（同 E 段 cutoff 策略，F 段只校验本 change 归档之后新建的 change）。
- **spec 一致性**：同步清理 main spec `docs-milestone-structure` 与本 change 冲突的 requirement（user-guide 强制、plans 删除），归档后无遗留矛盾。
- **删除面**：`docs/user-guide/`（11 文件）、`docs/agents/`（3 文件）、`docs/plans/`（44 文件移走）。其中 user-guide 两个文件带合并冲突标记，属腐烂内容，删除即修复。
- **风险**：user-guide 仍有零散有效内容（getting-started 安装步骤）——tasks 中安排"先比对迁移、后删除"的顺序，避免误删。
