## Why

大工具输出进上下文这件事，现有防线全部「事前预防、靠自觉」：pi 工具内置截断（bash 2000 行 / read 50KB，**砍掉即丢**）、AGENTS.md 的 ctx 路由纪律（纪律约束，agent 偶尔犯懒漏走）、headroom/context-mode（主动调用，playwright 等 MCP 工具输出不经过它们）。真实漏网场景：playwright `browser_snapshot` 几十 K 的 accessibility 树、read 整读大文件（50KB ≈ 12K token）、未走沙箱的大 grep——单个就把会话推向 compaction。

harness 调研 C 级判定（`docs/research/harness-survey/findings.md` C12）确认了便宜的那半解法：pi 的 `tool_result` 事件是 middleware 链、**can modify result**，替换发生在新结果进任何 LLM 请求之前（缓存尚未建立），**入口 spill 零缓存代价**——对比存量剪枝（C9，不抄）从第一个改动 token 起缓存全灭。dsh spill 模式的「完整内容持久化 + 有界预览 + 取回定位符」三层结构正是这半的成熟设计。

## What Changes

- **spill 扩展（新 `.pi/extensions/` 文件）**：挂 `tool_result` middleware，结果超过阈值（初定 32KB）时无条件 spill——这是保险丝，不依赖 agent 自觉
- **替换格式**：完整内容写入 `.pi/harness/spill/<sessionId>/<seq>-<tool>.txt`（与 events.db 同体系目录），上下文里替换为「头部预览 + 取回路径（read 可取）+ 尾部预览 + 省略字节数」
- **生命周期**：spill 文件 30 天清理，与 events.db TTL 对齐（复用 harness-log 清理机制）
- **记账**：向 events.db 记 `spill.write` 事件（第八类事件词汇；payload 含 tool、字节数、spill 路径），月度 SQL 可查哪些工具最常 spill——反过来量化 ctx 纪律的漏网率
- **图片块不处理**（初判只 spill 文本块，图片块保持原样——design 阶段定案）

## Capabilities

### New Capabilities

- `tool-output-spill`：超大工具结果的强制 spill 行为（阈值判定、预览替换格式、取回路径、文件保留期）

### Modified Capabilities

- `harness-fact-log`: 「事件类型词汇与保留期」需求从七类事件扩为八类（新增 `spill.write`，30 天保留，与 gate.check 同级）

## Impact

- `.pi/extensions/tool-output-spill.ts`（新建，改动主战场）：`tool_result` handler + spill 写文件 + 替换结果
- `.pi/extensions/lib/harness-log.ts`：kind union + RETENTION_DAYS 各加一行；kind 为 TEXT 列，**无 DB schema 迁移**
- `.pi/harness/spill/`（新目录）：会话维度 spill 文件，30 天清理
- 烟测脚本（`tests/run-harness-smoke.sh` 或新增）：超阈值 spill / 不超不动 / 取回路径有效 / TTL 清理用例
- 纯工具链 change（harness 层），不涉及业务域，无 constraint-domains 声明

（设计开放点，design.md 定案：`tool_result` 事件内 sessionId 的获取方式、MCP 工具（playwright 等）结果是否走此链、文本/图片混合 content 块的处理边界、阈值 32KB 的定值依据与是否按工具差异化。）
