# harness-fact-log Delta

## MODIFIED Requirements

### Requirement: 事件类型词汇与保留期

事实库 SHALL 支持九类事件：`session.start`（90 天）、`constraint.inject`（30 天）、`pin.write`（永久）、`pin.read`（30 天）、`gate.check`（30 天）、`subagent.dispatch`（30 天）、`subagent.complete`（30 天）、`mode.set`（30 天）、`spill.write`（30 天）。每条事件 MUST 携带单调递增 id、ISO 8601 UTC 时间戳、session_id、kind，change 列可空。开库时 MUST 按 kind 分保留期清扫过期行；库文件超过 100MB 时 MUST 触发删最老一半的保险丝。除 TTL 清扫与保险丝外 MUST NOT 修改或删除既有事件（完成回填以追加新事件表达，MUST NOT 改写既有 dispatch 行）。

#### Scenario: TTL 分级清扫

- **WHEN** 开库时存在 31 天前的 constraint.inject 行与 91 天前的 session.start 行
- **THEN** 过期的 constraint.inject 被删除，91 天前的 session.start 被删除，pin.write 永久保留

#### Scenario: 事件追加不可变

- **WHEN** 事件已写入后再次开库
- **THEN** 除 TTL/保险丝外既有行的 ts、session_id、kind、change、payload 不被任何 API 改写

#### Scenario: spill.write 事件随词汇扩展落库

- **WHEN** spill 扩展成功 spill 一次工具结果
- **THEN** events.db 新增一条 kind 为 `spill.write` 的事件行，payload 含工具名与字节数；31 天后被 TTL 清扫（无 DB schema 迁移，kind 为 TEXT 列）

#### Scenario: subagent.complete 随词汇扩展落库

- **WHEN** 后台子线程完成，harness-telemetry 追加完成事件
- **THEN** events.db 新增一条 kind 为 `subagent.complete` 的事件行（既有 `subagent.dispatch` 行内容不变）；31 天后被 TTL 清扫

### Requirement: 门禁记账（gate.check）

quality-gate SHALL 对每条实际执行的门禁命令（golangci-lint / go vet / go build / change-scope 判定的 domain go test / pnpm lint）各记一条 `gate.check` 事件，payload 含 cmd、phase、ok、ms；ok=false 时附带截断摘要 diag（≤512B、单行）。ok=true MUST 同样记账（失败率统计需要分母）。事件 MUST 绑定 `detectActiveChange` 检测到的活跃 change（无则 null），该检测 MUST 与 constraint-injection 共享同一实现。纯文档/纯对话回合（未触发门禁命令）MUST 零记录。修复回合不单独记账——失败→通过由相邻 gate.check 的 ok 翻转表达。

diag 提取 SHALL 失败特征优先：从命令输出（stdout+stderr 拼接）中优先取首个含失败特征关键词（如 `FAIL`、`error`、`# <pkg>`、`exit` 等关键词表）的行；无任何命中时回退首个非空行。截断规范不变（单行、剥控制字符、≤512 字节）。提取 MUST 确定性（相同输出字节产生相同 diag）。该优先规则仅适用于 gate.check 记账路径；子线程失败白名单的 diag 规范（首个非空错误行）不受影响。

#### Scenario: 门禁命令失败记账

- **WHEN** 某回合 golangci-lint exit 1、go vet exit 0
- **THEN** 产生两条 gate.check：一条 ok=false 含 512B 内截断 diag，一条 ok=true 无 diag，两条 change 列均绑当前活跃 change

#### Scenario: 未运行不记账

- **WHEN** 回合只改了 docs/ 下 markdown（门禁放行，未执行任何命令）
- **THEN** 该回合不产生任何 gate.check 事件

#### Scenario: diag 失败特征优先（stdout 噪声行不掩盖真实错误）

- **WHEN** golangci-lint exit 1，stdout 首行为 `0 issues.`，stderr 后续行含编译错误（如 `# syntopica-backend/internal/topicgraph/service`）
- **THEN** ok=false 的 gate.check 事件 diag 含编译错误特征行（`# syntopica-backend/...`），而非 `0 issues.`

#### Scenario: go test 记账不丢 FAIL 行

- **WHEN** `go test -short` exit 1，输出首行为父包正常行 `? pkg [no test files]`，后续行含 `FAIL pkg [build failed]`
- **THEN** diag 含 `FAIL` 特征行，可事后审计还原真实失败原因

## ADDED Requirements

### Requirement: 子线程完成回填（subagent.complete）

harness-telemetry SHALL 在后台派发的子线程真实结束时追加一条 `subagent.complete` 事件，payload 至少含 agentId、status、ms、tokens、toolUses、turnCount、isError，经 agentId 与派发时的 `subagent.dispatch` 事件关联。同一 agentId 的完成事件至多一条（重复完成信号幂等去重）。既有 `subagent.dispatch` 事件 MUST NOT 改写（append-only）。完成信号涵盖成功 / 失败 / 用户取消；取消 SHALL 记 status=cancelled。未观察到完成信号（如进程退出断链）MUST NOT 伪造完成事件——断链本身可经「有 dispatch 无 complete」的查询发现。记账失败 MUST 不影响子线程结果装配（fail-safe）。change 归属 SHALL 复用派发时的绑定（完成回填不重检测）。

#### Scenario: 后台完成回填

- **WHEN** 后台子线程（派发时记 status=background 的 dispatch 事件）成功结束
- **THEN** 追加一条 subagent.complete（payload 含 agentId、ms、tokens、toolUses），原 dispatch 行不变，两事件经 agentId 可关联审计

#### Scenario: 取消记 cancelled

- **WHEN** 用户在后台子线程结束前取消派发
- **THEN** 追加一条 subagent.complete，status=cancelled，isError=false

#### Scenario: 断链不伪造

- **WHEN** 子线程派发后进程退出，完成信号缺失
- **THEN** 不产生 subagent.complete 事件；查询侧按 agentId 无 complete 对应可发现断链，不阻塞不报错
