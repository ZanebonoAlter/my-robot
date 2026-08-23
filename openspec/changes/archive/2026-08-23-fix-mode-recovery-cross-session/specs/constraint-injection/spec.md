# constraint-injection delta — fix-mode-recovery-cross-session

## MODIFIED Requirements

### Requirement: 档位识别与 change 绑定

extension SHALL 通过 `input` 事件识别阶段命令设置会话内档位（`requirements` / `implementation`），命令集覆盖本仓 openspec 斜杠命令（`/opsx-*`、`/skill:openspec-*`）与对应 skill 文件读取（`tool_execution_start` 的 read 路径命中 skill 目录）。

档位 SHALL 绑定活跃 change，绑定规则分档位：**输入提及优先**（命令参数/输入文本/写 change 目录文件时修正）；**mtime 最新兜底仅 implementation 档**（apply/verify/archive 无参语境）；requirements 档（explore/propose 新想法语境）SHALL NOT mtime 兜底——未明确提及时不绑定（关键词命中源不含无关 change 文本，pin_finding 落 research 库）。未激活档 SHALL 不显示、不分析任何 change。绑定 change 归档或删除后 SHALL 自动回落未激活档；agent 写 `openspec/changes/<name>/` 下文件时 SHALL 修正绑定。

档位状态 SHALL 持久化：每次档位激活（命令命中或 skill 路径命中）时 MUST 向事实库记 `mode.set` 事件（payload 含 mode 与绑定 change）。会话边界的恢复按 reason 区分，**三条恢复路径（resume / reload / startup）均 SHALL 仅按同 sessionId 的最近一条 `mode.set` 恢复，MUST NOT 全局兜底**（多 pi 窗口并行下，全局最新 mode.set 几乎必然属于其他会话——实测 2026-08-23 探索窗口 reload 被灌入他窗口 implementation 档）：`resume`（/resume 切换目标会话，sessionId 即目标会话 id）与 `reload`（同会话扩展重载）SHALL 清零后按同 sessionId 恢复；`startup` SHALL NOT 清零（pi-subagents 派发共用模块实例），但档位为空时 SHALL 按同 sessionId 恢复（pi 进程重启恢复会话的真实路径）；`new`/`fork` SHALL 维持清零语义（不恢复）。无本会话 mode.set 记录、或无法恢复（记录过期、绑定 change 目录已不存在）MUST 回落未激活档。恢复的档位与绑定遵循既有回落规则（change 归档后自动回落、写 change 目录修正绑定）。

#### Scenario: 斜杠命令激活档位

- **WHEN** 用户输入 `/opsx-apply add-change-scope`
- **THEN** 档位切换为 implementation 并绑定 change `add-change-scope`

#### Scenario: skill 读取激活档位

- **WHEN** agent 自动 read `.agents/skills/openspec-propose/SKILL.md`
- **THEN** 档位切换为 requirements（skill 路径 signal 命中）

#### Scenario: change 归档后回落

- **WHEN** 档位绑定的 change 目录已不存在（已归档）
- **THEN** 下一次 `before_agent_start` 前档位回落未激活，不再注入该 change 相关约束

#### Scenario: explore 新会话不绑定无关 change

- **WHEN** 新会话读 openspec-explore skill 激活 requirements 档，输入未提及任何 change，且存在 mtime 更新的其他 change 目录
- **THEN** 注入块活跃变更显示「无」，关键词命中源不含无关 change 文本，pin_finding 落 research 库而非 mtime 最新 change

#### Scenario: requirements 档提及后正常绑定

- **WHEN** requirements 档下用户输入提及某 change 名（如 `/opsx-continue <name>`）
- **THEN** 档位绑定该 change，其探索发现与约束照常注入

#### Scenario: apply 中断后 resume 恢复档位

- **WHEN** implementation 档会话（绑定 change X）中断后以 `session_start{reason:"resume"}` 恢复，事实库存在该会话线最近的 mode.set 且 change X 目录仍存在
- **THEN** 档位恢复为 implementation 并绑定 X，约束注入与 explore-findings 注入不丢失，无需重新触发 `/opsx-apply`

#### Scenario: pi 重启恢复会话后档位恢复（真实路径）

- **WHEN** pi 进程退出后重启并恢复原会话（`session_start{reason:"startup"}`，sessionId 与此前 mode.set 记录一致，模块档位为空）
- **THEN** 档位按同 sessionId 的最近一条 mode.set 恢复（含 change 绑定），约束注入不丢失

#### Scenario: 全新 pi 会话启动不继承其他会话档位

- **WHEN** pi 冷启动打开全新会话（reason=startup，该 sessionId 无任何 mode.set 记录），而事实库存在其他会话的 mode.set
- **THEN** 档位为未激活（startup 恢复无全局兜底）

#### Scenario: 无自身档位历史的会话 reload/resume 不继承他窗口档位

- **WHEN** 某会话自身无任何 mode.set 记录（新窗口探索会话），执行 `/reload` 或 `/resume`（reason=reload/resume），而事实库全局最新一条 mode.set 属于其他并行窗口（如另一窗口正在 apply 某 change）
- **THEN** 该会话档位为未激活（三条恢复路径均仅同 sessionId 取数，MUST NOT 全局兜底）

#### Scenario: 子线程派发不清零主会话档位

- **WHEN** 主会话 implementation 档激活，pi-subagents 派发子线程触发 `session_start{reason:"startup"}`（共用模块实例）
- **THEN** 主会话档位与命中保持不变（不清零、不重复恢复）

#### Scenario: reload 后档位恢复

- **WHEN** 档位激活的会话执行 `/reload`（reason=reload，sessionId 不变且有 mode.set 记录）
- **THEN** 清零后按同 sessionId 恢复档位与 change 绑定，无需重新触发阶段命令

#### Scenario: resume 无法恢复回落未激活

- **WHEN** resume 时事实库无可用 mode.set（无本会话记录/过期），或记录绑定的 change 目录已不存在
- **THEN** 档位回落未激活（仅注入索引），不报错

#### Scenario: 新会话不继承档位

- **WHEN** 以 `session_start{reason:"new"|"fork"}` 开始会话，此前另一会话刚记录过 mode.set
- **THEN** 档位为未激活，不从事实库恢复
