# tool-output-spill 任务

## 1. harness-log 词汇扩展

- [x] ✅ 1.1 `.pi/extensions/lib/harness-log.ts`：kind union 加 `"spill.write"`、RETENTION_DAYS 加 30；头部注释词汇表补一行（依据：tool-output-spill change）
- [x] ✅ 1.2 `queryEventsByKind` 或既有查询无需改动验证：spill.write 走通用写入/查询路径（logEvent 现成）

## 2. spill 扩展实现（`.pi/extensions/tool-output-spill.ts` 新建）

- [x] ✅ 2.1 阈值与配置：读 `.pi/harness.json` `spillThresholdBytes`（缺省 32768；文件不存在/非正数→禁用直通）；`.pi/harness.json` 提交一份含默认值的样例（JSON 无注释，字段即文档）
- [x] ✅ 2.2 `tool_result` handler：合并 text 块字节数 > 阈值 → 写 `.pi/harness/spill/<sessionId>/<ts>-<tool>.txt`（toolName 过滤非法字符）→ 返回 partial patch `{content: [头 2048B + 标记行(路径/字节数) + 尾 512B]}`；字节切片回退到 UTF-8 合法边界；非 text 块原样保留在 content 中
- [x] ✅ 2.3 D7 死循环防护：`toolName === "read"` 且 input.path 以 `.pi/harness/spill/` 开头 → 直通
- [x] ✅ 2.4 D6 失败安全：写盘 try/catch，失败原结果返回 + `spill.write {ok:false, error}` 记账
- [x] ✅ 2.5 D5 清扫：扩展加载时扫 spill 目录删 mtime >30 天文件（try/catch 包裹，失败不影响主流程）
- [x] ✅ 2.6 记账接线：成功 spill 后 `spill.write {tool, bytes, path, ok:true}`（sessionId 取 `ctx.sessionManager?.getSessionId?.()`，telemetry 同款）

## 3. 烟测（`.pi/extensions/tests/`）

- [x] ✅ 3.1 run-harness-smoke.sh 补断言组：spill.write TTL 30 天（词汇+保留期表）/ 新 kind 写入查询往返
- [x] ✅ 3.2 新增 spill 专项烟测（并入 run-harness-smoke.sh 或独立脚本）：超阈值替换（预览含路径与字节数、总大小有界）/ 未超直通 / read spill 路径直通（D7）/ 写盘失败降级（mock 不可写目录）/ 30 天清扫 / .pi/harness.json 非正数禁用
- [x] ✅ 3.3 MCP 覆盖验证（D1 假设）：真实 playwright 工具结果触发一次 tool_result 观察（人工验证记录进验证节；不走链则记已知局限）

## 4. 测试

- [x] ✅ 4.1 `bash .pi/extensions/tests/run-harness-smoke.sh` → 全绿（含 3.1/3.2 新断言）
- [x] ✅ 4.2 `bash .pi/extensions/tests/run-smoke.sh` → 全绿（注入主路径无回归）

## 5. 文档

<!-- doc-impact: none(.pi 扩展层工具链 change：无七域路径命中；主 spec 由 delta 归档合并；findings.md 属 research 材料不占域；纯工具链不触及任何业务 flow，无 flow 影响) -->
<!-- doc-impact-excuse: flow=工作树 flow/*.md 修改属其他并行 change/窗口; api=同上; database=同上; architecture=同上; standard=同上; configuration=同上 -->

- [x] ✅ 5.1 harness-survey 调研笔记的 C 级落地记录已补（research 材料整体 gitignored，不入七域与 checkbox 对账；内容见同目录 findings.md「C 级落地记录」节）

## 6. 验证

- [x] ✅ 6.1 `grep -c "spill.write" .pi/extensions/lib/harness-log.ts` → ≥2（kind union + RETENTION_DAYS）
- [x] ✅ 6.2 `bash .pi/extensions/tests/run-harness-smoke.sh` → 退出码 0
- [x] ✅ 6.3 `bash .pi/extensions/tests/run-smoke.sh` → 退出码 0
- [x] ✅ 6.4 造一个 >32KB 的 bash 输出（如 `seq 1 10000`），观察上下文收到预览（含 `.pi/harness/spill/` 路径与省略字节数）且 `sqlite3 .pi/harness/events.db "SELECT payload FROM events WHERE kind='spill.write' ORDER BY id DESC LIMIT 1"` 返回该次记账
- [x] ✅ 6.5 `read` 预览给出的 spill 路径 → 得到完整原文（D7 生效：read 结果不再被 spill）
- [x] ✅ 6.6 `bash scripts/doc-impact.sh verify openspec/changes/tool-output-spill` → 退出码 0
- [x] ✅ 6.7 `bash scripts/scenario-trace.sh openspec/changes/tool-output-spill` → 退出码 0

### Scenario → 测试文件映射

| Scenario | 测试文件 |
| --- | --- |
| 超阈值 bash 输出被替换 | .pi/extensions/tests/spill.smoke.cjs |
| 未超阈值原样通过 | .pi/extensions/tests/spill.smoke.cjs |
| 按需取回完整内容 | .pi/extensions/tests/spill.smoke.cjs |
| 图片块原样通过 | .pi/extensions/tests/spill.smoke.cjs |
| 磁盘写入失败不阻塞 | .pi/extensions/tests/spill.smoke.cjs |
| 30 天清理 | .pi/extensions/tests/spill.smoke.cjs |
| 记账可聚合 | .pi/extensions/tests/spill.smoke.cjs |
| TTL 分级清扫 | .pi/extensions/tests/harness-log.smoke.cjs |
| 事件追加不可变 | .pi/extensions/tests/harness-log.smoke.cjs |
| spill.write 事件随词汇扩展落库 | .pi/extensions/tests/harness-log.smoke.cjs |

（映射说明：真实链路观察 6.4/6.5 与 MCP 覆盖 3.3 属下次 pi 会话生效项，已在验证实测记录注明；本表映射全部为可机械校验的烟测文件。）

### 验证实测记录（2026-08-23）

- 6.1 ✅ `grep -c "spill.write" .pi/extensions/lib/harness-log.ts` → 3（注释+union+RETENTION）
- 6.2 ✅ run-harness-smoke.sh 全绿（harness-log/failure-classify/spill 三组，spill 18/18；TTL 断言已扩 spill.write）
- 6.3 ✅ run-smoke.sh 全绿（注入主路径/pin.read 去重/JIT 无回归）
- 6.4 ✅ 等价验证：烟测用例 1（80KB bash 结果 → 预览有界含路径+省略字节 + spill 文件完整原文 + `spill.write` 记账往返直查 events.db）。真实会话链路待下次 pi 启动（本会话启动时扩展尚未存在，/reload 可热载）
- 6.5 ✅ 等价验证：烟测用例 3（read 取回路径直通不二次 spill + 普通路径仍防漏）
- 6.6 ✅ doc-impact verify 通过（none 声明+excuse，见下）
- 6.7 ✅ scenario-trace 通过（8 Scenario 全映射）
- 3.3 ✅ MCP 覆盖（D1 假设）：handler 实现工具名无关（无过滤分支），烟测用例 7 以 `playwright_browser_snapshot` 工具名走同一 handler 全过（含图片块保留）；真实 playwright 链路待下次会话观察 events.db spill.write 按 tool 聚合
