
## test-case-entry-gate 实现落点与验证记录

入口门禁实现事实（2026-08-25 实装，全绿）：

**新增文件**：
- `.pi/extensions/lib/test-case-gate.ts`——共享纯函数：`parseComplexityDeclaration`（认 `<!-- complexity: complex|simple -->`，多声明取首个，非法值 null）、`hasTestCaseDoc`（test-cases*.md 前缀 / *-test-cases.md 后缀）、`scanComplexityKeywords`（4 词表扫任务行）、`decideEntryReminder`（入口决策：requirements 档/未绑定/已提醒/有文档→null；complex 缺文档→strong；simple|未声明+词表命中→兜底）、`buildEntryReminderMessage`（三档文案）。COMPLEXITY_KEYWORDS 常量的家在此（spec-gate.ts re-export 兼容旧 import）。
- `.pi/extensions/entry-gate.ts`——挂 turn_end：`queryBySession(cwd, sessionId, ["mode.set"])` 取最新档位（本会话查询，不做全局兜底），implementation+boundChange 时 decideEntryReminder 判定，命中 steer（`deliverAs:"steer", triggerTurn:true`）+ 记 gate.check（cmd=entry-gate, payload 含 declaration/kwHits）。去重 Set 每 change 一次，session_start{非startup} 清空。开关 ENTRY_GATE_ENABLE。
- `.pi/extensions/tests/test-case-gate.smoke.cjs`——33 项，含真实数据双断言（archive/2026-08-25-watch-materialized-topic 的 tasks 文本+无文档→兜底提醒；真实目录含 test-cases.md→静默）。

**修改**：spec-gate.ts `scanAcceptanceWording(tasksMd, files, proposalText?)` 加可选第三参声明优先（complex 缺文档→强违例一条；simple+命中→质询一条；未声明+命中→旧逐词文案；两参调用兼容）；warnAcceptanceWording 读 proposal.md 传入。run-harness-smoke.sh 注册 .tcg.cjs/.egate.cjs 产物。test-design.md 禁用词表 4 行声明制语义 + 同步义务对象改 lib/test-case-gate.ts。开发执行规范 §2 加「复杂度声明制」段 + 红旗项「proposal 缺复杂度声明」（openspec 官方 skill 按用户指示不植入定制项）。

**验证**：冒烟 33/33；golangci-lint 0 issues + go build OK（零后端波及）；openspec validate valid；doc-impact verify 通过（声明 standard 文件 1 个，flow/api/database 用 excuse 豁免并行脏文件误报）。

**部署生效条件**：pi 重载扩展（新会话/reload）。本 change 自身声明 simple，任务行已避开 4 个兜底词，不会被自己的门禁 nag。

<!-- pinned 2026-08-25T15:52:20Z -->
