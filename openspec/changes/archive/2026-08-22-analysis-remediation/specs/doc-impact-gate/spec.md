# doc-impact-gate Delta

## ADDED Requirements

### Requirement: 归档命令硬门禁（spec-gate）

pi 扩展 `.pi/extensions/spec-gate.ts` SHALL 拦截 bash 工具中匹配 `openspec archive` 的命令，在放行前强制执行三项检查：① `scripts/doc-impact.sh verify <change-dir>` 退出码为 0；② `scripts/check-standards.sh` 无失败；③ 该 change 的 tasks.md 含「测试/文档/验证」尾三节及 doc-impact 标记。任一失败 MUST block 并输出中文 reason（列失败项 + 修复指引）。豁免通道：命令显式带 `--force` 或环境变量 `SPEC_GATE_BYPASS=1`（MUST 记 warning，不得静默放行）。开关：`SPEC_GATE_ENABLE`（默认开启）。

#### Scenario: 门禁未过时归档被拦截
- **WHEN** agent 执行 `openspec archive <change>` 且 doc-impact verify 失败
- **THEN** 命令被 block，reason 列出失败项与修复指引

#### Scenario: 三项全过时放行
- **WHEN** 三项检查全部通过
- **THEN** 归档命令正常执行

#### Scenario: 显式豁免留痕
- **WHEN** 命令带 `--force` 或 `SPEC_GATE_BYPASS=1`
- **THEN** 归档放行且记录一条 warning（不静默）

### Requirement: quota-gate fail-open 落盘可观测

quota-gate 在额度查询失败而放行（fail-open）时 MUST 追加一条 custom_message 落盘记录（含失败原因），使"查询失败放行"与"未触发"在会话记录中可区分。

#### Scenario: 查询失败放行留痕
- **WHEN** quota 查询接口失败且按 fail-open 策略放行派发
- **THEN** 会话记录中出现一条 custom_message 说明"quota 查询失败已放行"及原因

### Requirement: 全量测试软守卫（test-scope-guard）

pi 扩展 `.pi/extensions/test-scope-guard.ts` SHALL 在检测到非归档语境下的全量 `go test ./...` 命令时发出软提醒（notify，不 block）。模式开关 `TEST_SCOPE_GUARD=soft|hard|off`，默认 soft。

#### Scenario: 日常误跑全量测试触发提醒
- **WHEN** 会话语境非归档且命令命中全量 `go test ./...`
- **THEN** 收到软提醒（建议只跑影响包），命令不被阻断
