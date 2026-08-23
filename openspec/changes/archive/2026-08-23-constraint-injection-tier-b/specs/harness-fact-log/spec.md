# harness-fact-log delta（constraint-injection-tier-b）

## MODIFIED Requirements

### Requirement: 事件类型词汇与保留期

事实库 SHALL 支持七类事件：`session.start`（90 天）、`constraint.inject`（30 天）、`pin.write`（永久）、`pin.read`（30 天）、`gate.check`（30 天）、`subagent.dispatch`（30 天）、`mode.set`（30 天）。每条事件 MUST 携带单调递增 id、ISO 8601 UTC 时间戳、session_id、kind，change 列可空。开库时 MUST 按 kind 分保留期清扫过期行；库文件超过 100MB 时 MUST 触发删最老一半的保险丝。除 TTL 清扫与保险丝外 MUST NOT 修改或删除既有事件。

#### Scenario: TTL 分级清扫

- **WHEN** 开库时存在 31 天前的 constraint.inject 行与 91 天前的 session.start 行
- **THEN** 过期的 constraint.inject 被删除，91 天前的 session.start 被删除，pin.write 永久保留

#### Scenario: 事件追加不可变

- **WHEN** 事件已写入后再次开库

- **THEN** 除 TTL/保险丝外既有行的 ts、session_id、kind、change、payload 不被任何 API 改写

## ADDED Requirements

### Requirement: 档位记账（mode.set）

constraint-injection SHALL 在档位激活（input 命令命中或 tool_execution_start 的 skill 路径命中）时自报 `mode.set` 事件，payload 含 mode（`requirements` / `implementation`）与 boundChange（可空）；档位绑定 change 修正（提及 change / 写 change 目录）时 SHALL 追记新事件以反映最新绑定。事件写入失败 MUST 不阻断档位激活本身（记账 fail-loud、注入照常）。`session_start{reason:"resume"}` 的恢复取数 MUST 复用既有查询 API；恢复语义（new/fork/reload 不恢复、change 不存在回落）由 constraint-injection capability 定义。

#### Scenario: 档位激活记账

- **WHEN** 用户输入 `/opsx-apply some-change` 激活 implementation 档
- **THEN** 产生一条 mode.set（payload 含 mode=implementation、boundChange=some-change），档位激活不因记账失败而中断

#### Scenario: 绑定修正追记

- **WHEN** implementation 档会话中 agent 写 `openspec/changes/another-change/tasks.md` 使绑定修正
- **THEN** 追记一条 mode.set（boundChange=another-change），此前事件不改写

#### Scenario: TTL 与既有事件兼容

- **WHEN** 升级后首次开库（既有六类事件已存在）
- **THEN** 开库校验与 DDL 幂等通过，mode.set 按写入正常入账，既有事件不受影响
