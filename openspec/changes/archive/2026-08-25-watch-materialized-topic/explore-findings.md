
## watch-materialized-topic 缺 test-cases.md 的事实库考古结论

事件考古结论（.pi/harness/events.db，2026-08-25）：

**缺失事实**：change 目录内无 test-cases.md（白盒用例文档），tasks.md §6「测试」节只有三行无 checkbox 的普通 bullet（集成测试/白盒用例锚点/回归确认），§8 验证节也只有命令清单、无 §0.6 步骤 2 要求的 Scenario→测试文件映射表。对比：同族已归档 change `2026-08-24-watch-keyword-and-quickadd` 有完整 test-cases.md（故事总纲/主链路表/变体走查表），其 tasks.md 头部明确引用「测试设计锚 test-cases.md」。

**事实库证据**：
- 08-25 02:33 UTC 两条探索 pin 落 docs/research/watch-materialized-topic/explore-findings.md（当时无激活档，change 字段为空）——探索发现有，但非测试用例。
- 06:27 reload → 06:28 建目录 → 06:31 proposal+mode.set(requirements) → 06:34-35 specs → 06:40 design，一气呵成 ~12 分钟；该 change 全天 0 次 subagent.dispatch（唯一两次派发 07:43 Explore×2 属 board-level-deep-analysis），即 §0.6 步骤 3 前置「复杂档先派子线程枚举白盒用例」从未执行。
- 12:43 mode.set(implementation)（session 01a03676）→ 12:47-14:31 gate.check ~278 次，tasks 1.1-5.3 勾完（tasks.md 北京 22:26 最后修改）。
- 约束注入正常：12:41:57 三条（mode-base + declaration×2：daily-report/topic-graph，proposal 头 constraint-domains 声明生效）。

**根因**：①编制计划（步骤 2）时未把「出白盒用例文档」写成带 checkbox 的任务，后续无人执行；②harness 无「复杂档缺 test-cases.md」的动工前自动门禁（quality-gate 只管 lint/build/test，scenario-test-mapping 只在归档时查映射），§2 红旗靠 agent 自觉。

**补救（未归档前做）**：①按 §2 派子线程机械枚举白盒用例补 test-cases.md（断言判据主线程定），锚点=DNF 匹配语义复用/SaveReport 排除放行边界/提示轨互斥双向/embedding 检索阈值与 top-K；②tasks.md §6 改 checkbox 任务并引用 test-cases.md；③§8 补 Scenario→测试文件映射表（归档门禁 scenario-trace-gate 必查）。

<!-- pinned 2026-08-25T14:37:02Z -->
