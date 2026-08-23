# Design: port-constraint-injection

## Context

- 源方案：用户另一项目的 `.pi/extensions/constraint-injection.ts`（~600 行，实战运行），核心机制：`input` 事件识别阶段命令设档位 → `before_agent_start` 每 turn 重建 system prompt 时追加约束块（幂等 + 前缀缓存友好 + 抗 compaction）→ `pin_finding` 工具持久化探索发现并在实现档自动注入。设计文档与配置样例随调研资料已入库 `docs/research/extensions/`。
- 本仓现状：4 个 extension（quality-gate/quota-gate/spec-gate/test-scope-guard）均为门禁类，无注入类，无职责重叠；`doc-impact.sh context` 提供 apply 启动时一次性双源注入（flow 业务约束 + standard `doc-impact-applies` 标签命中）。
- Syntopica 文档树：`docs/reference/standard/`（前后端规范，文档头有 `doc-impact-applies` 路径标签）、`docs/reference/flow/`（业务约束与不变量，按 domain 组织）、`docs/reference/开发执行规范.md`。

## Goals / Non-Goals

**Goals:**
- 约束注入从"apply 一次 + agent 自觉"升级为"harness 层每 turn 强制在场"。
- 复用源 extension 的成熟能动性（档位/绑定/JIT/速览/pin_finding），适配成本最小化。
- `pin_finding` 落点与 research-retention 规则对齐。

**Non-Goals:**
- 不做关键词表的自动生成（配置手工维护，grep 摸底产出初版）。
- 不改 standard 文档内容（MVP 不要求「## 硬规则速览」小节；文档超阈值时回落全文注入，见决策 4）。
- 不动 quality-gate / spec-gate / test-scope-guard（注入管"知道"，门禁管"做到"，互补）。

## Decisions

### 1. 移植而非重写

源 extension 经过实战迭代（防档位粘性、JIT 只增不减保前缀缓存、pin_finding 防 research 落错库等教训都在注释里）。移植时保留架构与状态机，改/增五处：①配置指向与命令/skill 映射 ②pin_finding research 落点（`docs/report/research/` → `docs/research/<topic>/`，无 topic 落通用池单文件 `docs/research/explore-findings.md`）③注入块头部宪法优先标注（决策 6）④节提取函数（决策 3）⑤关键词命中粘性（决策 3）。均为局部改动，不动状态机与事件流。**替代方案**（否决）：按 dsh 思路重新设计——重新踩一遍源项目已踩过的坑。

### 2. 档位与命令映射（Syntopica 版）

| 档位 | 触发命令（斜杠） | skill 路径 signal（agent 自动 read skill 时激活） |
|---|---|---|
| requirements | `/opsx-explore` `/opsx-new` `/opsx-propose` `/opsx-continue` 及 `/skill:openspec-*` 对应项 | `openspec-explore` `openspec-new-change` `openspec-propose` `openspec-continue-change` |
| implementation | `/opsx-apply` `/opsx-verify` `/opsx-archive` 及 `/skill:openspec-*` 对应项 | `openspec-apply-change` `openspec-verify-change` `openspec-archive-change` |

change 绑定在源机制基础上分档位收紧（@bugfix）：输入提及优先；mtime 最新兜底仅 implementation 档（/opsx-apply 无参语境正确）；requirements 档不兜底——explore/propose 是新想法语境，兜底会把新会话绑到无关 change（实测：新会话 explore 默认绑 mtime 最新 change，pin 落错库，explore 内容错位）；未激活档不显示不分析任何 change。归档自动回落；写 `openspec/changes/<name>/` 修正绑定。

### 3. 文档命中策略：复用 `doc-impact-applies` 标签 + 节级提取，不发明第二套映射

- **standard how 层（JIT）**：standard 文档头已有 `doc-impact-applies: backend-go/internal/platform/airouter/` 式路径标签（doc-impact.sh context 现用）。extension 的 jitDocs 配置**直接从这些标签生成**：摸底脚本 grep 全部标签 → 生成 `pathSignals` 初版。一份数据两个消费者，避免映射漂移。
- **flow what 层（关键词 + 节提取）**：keywordDocs 按 domain 业务词命中后**只注「业务约束与不变量」节**（新增 `section` 字段），不注全文——实测 flow 全文 8~20K、约束节仅 1.7~4.6K（占全文 11~21%），其余四节（需求说明/链路设计/代码入口/变更溯源）对写代码的 agent 是噪音。节名是 flow 文档既有统一结构（旧 context 同样提取该节），仍是单一真相源；节尾附「全文见 <路径>」指引，需要细节时 agent 自行 read。初版关键词表由摸底产出（grep flow 文档标题与 domain 名）。
- **standard 节提取**：jitDocs 同样支持 `section`——已 spec 化文档（有 `## Requirements`）注 Requirements 节；未 spec 化文档回落全文（standard 定位即约束清单，全文安全；替代原 context 对未 spec 化文档的"静默跳过"语义，见 standard-spec-format delta）。
- **命中粘性（新增）**：关键词命中与 JIT 同样**会话内只增不减**——命中源含最近 5 条输入滚动窗，不粘性则词条滚出窗口 → 注入块缩水 → system prompt 字节变化 → **全部历史缓存报废**（缓存按前缀算，系统提示变一字全重算）。粘性后话题浮出过一次即保留其约束，尾部追加不动前缀。
- **baseDocs 极小化**：两档 baseDocs 只放超短索引文档（未激活档也仅注索引）——常驻部分越小前缀缓存越稳。开发执行规范全文**不进** baseDocs（太长；其关键约束已摘要进 AGENTS.md）。

### 4. 大文档处理：节提取解决大头，速览格式后置

实测（08-22）：flow 文档 8~20K **全部超过**源默认 6144 阈值，且本仓 flow 无「## 硬规则速览」小节 → 源两级注入机制在本仓 flow 上必然回落全文，"单文档基本在阈值内"不成立。对策是决策 3 的**节提取**：flow 走「业务约束与不变量」节后注入量降到 1.7~4.6K 级；standard 最大 testing.md 16.6K，spec 化节提取或全文硬扛均可接受。源机制的「## 硬规则速览 + 目录」代码路径保留，后续某文档膨胀再启用（那时是纯文档 change，命中 doc-impact standard 域，正常走声明）。**替代方案**（否决）：现在就给所有 flow/standard 文档加速览节——动 10+ 文档换一个已被节提取解决大半的问题。

### 5. doc-impact.sh context 退役方式：删子命令，保留脚本

- `context` 子命令删除（职责被 extension 取代；留着会有两个真相源）。
- `suggest`（apply 预勾选）保留——它是**声明生成器**不是注入器；`verify`（归档对账）保留——机械门禁。
- spec-gate.ts 的归档检查引用的是 verify，不受影响。

### 6. 注入块与优先级宪法的关系

注入块头部固定一行：`与 AGENTS.md 优先级宪法冲突时，以宪法为准`。原因：宪法是用户当场指令 > 项目文档 > skill 的仲裁规则，extension 注入的内容属于"项目文档"层，不能悄悄升权。

### 7. context 引用面清理：一次清完，不留过渡态

全仓 grep `doc-impact.sh context` 共 17 处文字引用，本 change 内全部处理：

| 类别 | 位置 | 处理 |
|---|---|---|
| 指令模板 | `.pi/prompts/opsx-apply.md` | 「跑 context」步骤改为「extension 已自动注入」——不改则每次 apply 都执行已退役命令 |
| 叙述文档 | 根 `AGENTS.md`、`docs/README.md`、`architecture/map.md`、`开发执行规范.md`（§0.6 + 分层表） | 数据源表述改指 extension |
| flow 脚注 | 10 个 flow 文档约束节尾注 + `flow/README.md` | 统一改「constraint-injection extension 注入数据源」，仅表述、约束内容零改动 |
| 主 spec | `docs-reference-layer` / `standard-spec-format` | 出 delta，归档合并时更新主库（不直改 openspec/specs/） |
| 调研快照 | `docs/research/` | 不动（快照非权威，README 已注明按路径重验） |

V5 验证对应放宽：grep 排除 `openspec/changes/`、`openspec/specs/`（归档 delta 合并清除）、`docs/research/`（快照）。

## Risks / Trade-offs

- [配置关键词表维护漂移] → jitDocs 从 doc-impact-applies 标签生成（单一真相源）；keywordDocs 初版摸底 + 后续按需补（漏命中=少注入，不注入错误内容，fail-safe）。
- [每 turn 注入增 token 成本] → 未激活档仅索引；flow 节提取（单域 1.7~4.6K）；命中粘性 + JIT 只增不减保 system prompt 字节稳定——缓存全命中时增量成本 ≈ 0，缓存抖动 = 全历史重算（粘性即为此设）；多域叠加由关键词表粒度控制。
- [子线程不吃注入] → 源方案同款兜底：子线程靠 plan 内嵌约束（§0.6 编排既有纪律），extension 只保主线程。
- [pin_finding 落点错乱] → 复用源项目的三级解析（显式 change > 档绑定 > research 池，池分 topic 目录与通用单文件），08-19 教训已在源码注释。

## Migration Plan

1. ~~先 apply `amend-dev-workflow`~~ ✅ 已归档（docs/research/ 与源码快照就位）。
2. 本 change apply 期内：extension + 配置 + smoke test 上线，完成 T2~T5 会话验证（档位流 / JIT+节注入 / pin_finding 落点 / doc-impact 回归）。
3. **T2~T5 全过后即删 context 子命令 + 完成引用面清理（决策 7）**——删除与上线同 change 内完成，不留双机制过渡期；后续 change 的日常使用即观察期（若命中系统性失准，另开 change 调配置关键词表，context 不复活）。
4. 回滚：删 extension + 配置，恢复 doc-impact.sh context（git revert 单 commit）。

## Open Questions

（无——配置具体关键词表在 apply 摸底阶段产出，属执行期事实。）
