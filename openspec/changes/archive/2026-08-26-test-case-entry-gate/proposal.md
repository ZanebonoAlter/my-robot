<!-- complexity: simple -->

## Why

watch-materialized-topic（2026-08-25）作为复杂档动工至收尾全程无 test-cases.md：spec-gate 检查⑤a（白盒用例缺失扫描）唯一触发点是 `openspec archive` 命令，失败发现在成本曲线最右端；且 ⑤a 为 warn 级留痕，不进 agent 上下文。事件库证实该 change 在 implementation 档跑门禁 278 次全绿、零子线程用例枚举派发——义务全靠 agent 自觉。需要把「缺白盒用例文档」的反馈从归档瞬间提前到动工瞬间（mode 切入 implementation 档），并把复杂度判定从事后词法猜测改为设计阶段自声明。

## What Changes

- **新增入口门禁 extension（`.pi/extensions/entry-gate.ts`）**：挂 turn_end（quality-gate 同款 steer 软提示模式），从 harness 事实库读本会话最新 `mode.set`——mode=implementation 且绑定 change 时核对该 change 目录：
  - 已有 test-cases*.md → 静默通过；
  - proposal 头声明 `<!-- complexity: complex -->` 且缺文档 → steer 提醒补文档或改声明（确定性核对，零误报）；
  - 声明 simple / 未声明 → 沿用 4 词关键词表扫 tasks.md 任务行作兜底信号，命中 → steer 提醒（提示补声明或补文档）。
  - 每会话每 change 只提醒一次（文件补齐后不再重复）；纯 requirements 档零触发。
- **复杂度声明制**：proposal.md 头部新增机器可读声明 `<!-- complexity: complex | simple -->`（与 constraint-domains 同款注释惯例）；声明义务写入《开发执行规范》§2（openspec 官方 skill 不植入仓库定制项）。词法扫描降级为未声明/声明 simple 时的兜底质询信号，**词表不扩容**（全量校准：4 词 fire 33%、19 词 fire 69%，扩容只稀释警报）。
- **spec-gate 归档检查⑤同步**：⑤a 文案与判定改为声明优先——声明 complex 缺文档为最强违例；声明 simple 但词表命中 → 反向质询（改声明或补文档）；未声明 → 沿用现行为。关键词表与 test-design.md 禁用词表的双向同步义务保持不变，同步项增加声明格式说明。
- **共享纯函数落 `lib/`**：复杂度声明解析（parseComplexityDeclaration）+ ⑤a 词法兜底扫描收敛到共享模块，entry-gate 与 spec-gate 同源调用，冒烟测试挂 run-harness-smoke.sh。

## Capabilities

### New Capabilities

（无——入口门禁是 harness 扩展行为，归 case-first-testing 既有义务的执行机制）

### Modified Capabilities

- `case-first-testing`: 「用例设计先行」Requirement 增补复杂度声明制（proposal 头 `<!-- complexity: -->` 声明义务与语义）与入口门禁行为（implementation 档动工时对复杂档缺白盒用例文档的 steer 级提醒、去重与静默条件）。

## Impact

- 代码：`.pi/extensions/entry-gate.ts`（新增）、`.pi/extensions/lib/test-case-gate.ts`（新增共享纯函数）、`.pi/extensions/spec-gate.ts`（⑤a 判定改声明优先）、`.pi/extensions/tests/`（新冒烟 + runner 注册）。
- 文档：`docs/reference/standard/shared/test-design.md`（禁用词表 ⑤a 行同步声明制）、`docs/reference/开发执行规范.md` §2（复杂度声明制段与红旗项）。
- 事实库：entry-gate 触发时记 `gate.check`（phase=turn_end，cmd=entry-gate），复用现有 schema。
- 用户可见行为：implementation 档动工首回合后多一条 steer 提醒（仅缺文档的复杂档）；归档门禁 ⑤a 文案变化。无 API/数据模型/前端影响。
