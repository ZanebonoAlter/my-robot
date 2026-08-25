## 1. 共享纯函数库

- [x] 1.1 新增 `lib/test-case-gate.ts`：`parseComplexityDeclaration(text) → "complex"|"simple"|null`（识别 `<!-- complexity: complex|simple -->`，容忍前后空白，多声明取首个，非法值视为未声明）+ `scanComplexityKeywords(tasksMd) → string[]`（任务行 `- [ ]`/`- [x]` 锚定，词表从 spec-gate.ts 迁入：算法/状态机/解析/协议）+ `decideEntryReminder(...) → {remind, reason} | null` 纯决策函数（入参：mode/boundChange/目录文件列表/proposal 文本/tasks 文本/已提醒标记；分支：requirements 档→null、有 test-cases*→null、声明 complex→强提醒、simple/未声明+词表命中→兜底提醒、未声明未命中→null）；验证：`tests/test-case-gate.smoke.cjs` 覆盖上述全分支
- [x] 1.2 `spec-gate.ts`：`COMPLEXITY_KEYWORDS` 与任务行收集核改为从 lib 引入并 re-export（保持 `tests/spec-gate.smoke.cjs` 现有 import 不破）；`scanAcceptanceWording(tasksMd, files, proposalText?)` 加可选第三参实现声明优先（未传参行为与现版一致）；验证：现有 spec-gate.smoke.cjs 全绿（回归）
- [x] 1.3 `tests/run-harness-smoke.sh` 注册 lib/test-case-gate.ts 与 entry-gate.ts 的 esbuild 产物 + 新冒烟文件；验证：`bash .pi/extensions/tests/run-harness-smoke.sh` 退出码 0

## 2. 入口门禁扩展

- [x] 2.1 新增 `.pi/extensions/entry-gate.ts`：挂 turn_end——`ctx.sessionManager?.getSessionId?.()` 取会话，`queryBySession(cwd, sessionId, ["mode.set"])` 取最新档位（无记录→跳过）；mode=implementation 且 boundChange 时 `decideEntryReminder` 判定，命中则 `pi.sendMessage({deliverAs:"steer", triggerTurn:true})` 注入提醒并记 `gate.check`（cmd=entry-gate，payload 含 declaration/kwHits）；去重 Map 每会话每 change 一次，`session_start{reason≠startup}` 清空（quality-gate 同款防御）；全程 try/catch fail-open + console.warn 留痕；验证：冒烟经导出的判定函数覆盖三档分支与去重，`SPEC_GATE_ENABLE=0` 同款开关 ENTRY_GATE_ENABLE=off 生效

## 3. 归档门禁同步

- [x] 3.1 `spec-gate.ts` 检查⑤文案升级：声明 complex + 缺文档 → 「声明复杂档但缺白盒用例文档」强违例；声明 simple + 词表命中 → 反向质询（改声明或补文档）；未声明 + 命中 → 沿现文案；验证：spec-gate.smoke.cjs 新增三条分支断言（传 proposalText 参）

## 4. skill 与规范同步

- [x] 4.1 `docs/reference/开发执行规范.md` §2 增补「复杂度声明制」段：proposal 头 MUST 携带 `<!-- complexity: complex|simple -->`（判定标准=状态机≥3状态/算法/多模块协议任一）、机器执行说明（entry-gate / spec-gate ⑤a）、红旗项加「proposal 缺复杂度声明」（openspec 官方 skill 不植入定制项，义务以规范为准）；验证：grep 开发执行规范含声明格式与 entry-gate 说明
- [x] 4.2 `docs/reference/standard/shared/test-design.md`「验收措辞规范」节：⑤a 行改写为声明制语义（complex 缺文档=强违例、simple+词表=反向质询），同步义务对象改为 `lib/test-case-gate.ts`；JIT 摘要节同步；验证：文档内两处一致且无「改此表必同步 spec-gate.ts 内置常量」陈旧表述残留

## 5. 测试

- 集成自检：本 change 自身声明 simple 且任务行不含兜底关键词 → 档位切入 implementation 后 entry-gate 应保持静默（反向用 watch-materialized-topic 的 tasks.md 文本喂判定函数应产生提醒）
- 回归确认：spec-gate.smoke.cjs 既有断言不改语义全绿；run-harness-smoke.sh 全链退出码 0

## 6. 文档

<!-- doc-impact: standard -->
<!-- doc-impact-excuse: flow=疑似遗漏来自 watch-materialized-topic 等并行 change 的脏文件（本 change 零 backend-go/front 代码改动）; api=同上，本 change 不碰 api; database=同上，本 change 不碰 database -->

- [x] 6.1 `docs/reference/standard/shared/test-design.md` 禁用词表与同步义务更新（见 4.2，含 JIT 摘要节同步；开发执行规范 §2 变更属编排文档不入本表）；验证：`bash scripts/doc-impact.sh verify openspec/changes/test-case-entry-gate` 退出码 0

## 7. 验证

- [x] 7.1 `bash .pi/extensions/tests/run-harness-smoke.sh` — 退出码 0（33 项含真实数据自检）
- [x] 7.2 `cd backend-go && golangci-lint run ./... && go vet ./... && go build ./...` — 零报错（确认零后端波及，实测 0 issues + BUILD-OK）
- [x] 7.3 `openspec validate test-case-entry-gate` — valid；`openspec show test-case-entry-gate --json --deltas-only` 核对 delta 与实现一致

### Scenario → 测试文件映射

| Scenario | 测试文件 |
| --- | --- |
| 简单 CRUD 直接进实现 | 人工：流程义务场景（免白盒文档），由 test-design.md 层选择规则约束，无独立自动化测试 |
| 复杂逻辑先枚举白盒用例 | 人工：流程义务场景，由 entry-gate/spec-gate ⑤a 机器提醒落地（见 .pi/extensions/tests/test-case-gate.smoke.cjs 真实数据自检） |
| 声明 complex 且缺文档，动工时被提醒 | .pi/extensions/tests/test-case-gate.smoke.cjs |
| 已有文档，动工全程静默 | .pi/extensions/tests/test-case-gate.smoke.cjs |
| 声明 simple 且词表未命中，静默放行 | .pi/extensions/tests/test-case-gate.smoke.cjs |
| 声明 simple 但任务行命中关键词，兜底质询 | .pi/extensions/tests/test-case-gate.smoke.cjs |
| 提醒每会话每 change 至多一次 | .pi/extensions/tests/test-case-gate.smoke.cjs |
| requirements 档零触发 | .pi/extensions/tests/test-case-gate.smoke.cjs |
| 归档对账时声明优先复核 | .pi/extensions/tests/spec-gate.smoke.cjs |
