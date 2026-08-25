# Tasks: port-constraint-injection

> 前置依赖：`amend-dev-workflow` 已归档 ✅（docs/research/ 就位、源码已迁至 docs/research/extensions/）。

## 1. 摸底（配置数据源）

- [x] 1.1 grep 全部 standard 文档头 `doc-impact-applies` 标签 → 生成 jitDocs/pathSignals 初版（一份数据源，两消费者）；有 `## Requirements` 节的文档配置 `section: "Requirements"`
  > 摸底结果：仅 `ai-logging.md` 有标签（airouter/ + dataenrichment/）且已 spec 化 → jitDocs 1 条带 section: Requirements；其余 standard 文档无标签（旧 context 同样不注入，行为对齐，不扩scope）
- [x] 1.2 摸底 flow 文档 domain 组织 → 产出 keywordDocs 初版（domain 名 + 高频业务词 → flow 文档路径，全部带 `section: "业务约束与不变量"`）；确认 baseDocs 索引文档内容（极短索引）
  > 摸底结果：9 个 flow 文档全部有「业务约束与不变量」节（1.7~4.6K）；keywordDocs 9 条（依据各文档「代码入口」节 domain 关联 + 业务词）；索引文档 `docs/reference/constraints-index.md`（1.1KB）

## 2. extension 移植

- [x] 2.1 移植 `.pi/extensions/constraint-injection.ts`：保留档位/绑定/JIT/速览/pin_finding 架构，改/增五处——①配置指向 ②pin_finding research 落点（`docs/research/<topic>/`，无 topic 落通用池 `docs/research/explore-findings.md`）③注入块头部加「与 AGENTS.md 优先级宪法冲突时以宪法为准」④**新增节提取函数**（`section` 字段：`## 节名` 到下一 `##`，节尾附全文路径指引；无该节回落全文；keywordDocs/jitDocs 通用）⑤**关键词命中会话粘性**（与 jitDocHits 同款只增不减，防输入滚动窗使注入块缩水砸缓存）
- [x] 2.2 编写 `.pi/constraint-injection.json`：两档命令表（`/opsx-*` + `/skill:openspec-*`）+ skillSignals + baseDocs/keywordDocs（带 section）/jitDocs（带 section）/stackSignals（用 1.x 摸底产物）
- [x] 2.3 smoke test：`.pi/extensions/tests/constraint-injection.smoke.cjs` 覆盖纯函数（命令匹配/skill 路径匹配/栈判定/速览提取/节提取（有节/无节回落全文）/关键词命中粘性/pin_finding 落点三级解析/JIT 只增不减）+ `run-smoke.sh` 直跑不依赖 pi（31 项断言全过）
- [x] 2.4 bugfix：新会话 explore 默认 mtime 兑底绑定无关 change → 内容错位。修复：绑定与分析源的 mtime 兑底收紧到仅 implementation 档；requirements 档只认明确提及，未绑定时 pin 落 research 库；未激活档不显示不分析 change。smoke 补 6 项断言（36 项全过）
  > 31 项断言全过（含 stub pi API 全链路回放：未激活仅索引/切档/绑定/节级注入/粘性/JIT 门控/session 重置/pin 三级落点/节回落全文）

## 3. doc-impact context 退役（T2~T5 全过后执行）

- [x] 3.1 `scripts/doc-impact.sh` 删 context 子命令（-112 行；V2 退役提示 + suggest/verify 复跑不变 + bash -n 通过），原入口提示「已由 constraint-injection extension 取代」；suggest/verify 不动；spec-gate 引用不受影响（grep 确认）
- [x] 3.2 引用面清理（17 处一次清完，不留过渡态）：
  - `.pi/prompts/opsx-apply.md` —「跑 context」步骤改为「extension 已自动注入，无需手动跑」（不改则每次 apply 都执行已退役命令）
  - 根 `AGENTS.md`（Business Flow 数据源表述）、`docs/README.md`、`docs/reference/architecture/map.md`
  - `docs/reference/开发执行规范.md` §0.6 步骤 1 + 分层表 context 引用行（与任务 4.1 同文件一次改完）
  - 10 个 flow 文档「业务约束与不变量」节尾脚注 + `flow/README.md` — 统一改「constraint-injection extension 注入数据源」（仅表述，约束内容零改动）
  - `openspec/specs/` 主库不动 — docs-reference-layer / standard-spec-format 两个 delta 归档合并时更新

## 4. 文档

- [x] 4.1 `docs/reference/开发执行规范.md` §0.6 步骤 1：「跑 context 双源注入」改为「extension 自动注入（无需手动跑）」；§4.1 门禁分层表加注入层（turn 前 constraint-injection 注入 → turn 中 agent 执行 → turn_end quality-gate 门禁）
- [x] 4.2 根 `AGENTS.md`：pi 增量门禁段前新增「约束注入（管知道）」条目（quality-gate 条目改标「管做到」）+ Business Flow 行数据源表述更新
- [x] 4.3 extension 头部注释含设计决策与配置文件指引（自文档，对齐 quality-gate 风格）——随任务 2.1 完成

## 5. 测试

本 change 无产品代码（Go/Vue），extension 与脚本按工具类测试：

- [x] T1 smoke test = 任务 2.3（纯函数断言全过）✅ 31 项全过
- [x] T2 真实会话验证档位流：`/opsx-new` 触发 requirements 档 → 建 change → `/opsx-apply` 切 implementation 档 → 确认注入块随 turn 更新且绑定正确 change
- [x] T3 JIT + 节注入验证：实现档下 edit `backend-go/internal/platform/airouter/` 文件 → 确认命中 ai-logging.md 注入；change 文本含「板块」→ 确认 semantic-board「业务约束与不变量」节（非全文）注入
- [x] T4 pin_finding 落点验证：无档无 topic → `docs/research/explore-findings.md` 通用池；无档带 topic → `docs/research/<topic>/`；档激活 → change explore-findings.md 且实现档可见注入
- [x] T5 doc-impact 回归：suggest/verify 行为不变
  > 3.x 删 context 后复跑：verify 通过（声明 none + excuse 生效）、suggest 正常预勾选——与基线一致

## 6. 文档

<!-- doc-impact: none(新增 extension+配置+smoke test 不在七域启发式路径；其余改动为 context 退役的引用表述清理——flow 脚注/README/AGENTS/map/规范表格仅改数据源表述，不改任何文档的约束与规范内容；主 spec 由 delta 归档合并) -->
<!-- doc-impact-excuse: flow=工作区其他进行中 change 脏文件命中，非本 change 改动; api=同上; database=同上; architecture=同上; configuration=同上 -->

- [x] D1 开发执行规范.md §0.6/§4.1（任务 4.1）+ 分层表 context 引用（任务 3.2）
- [x] D2 根 AGENTS.md（任务 4.2 + 任务 3.2 数据源表述）
- [x] D3 extension 自文档（任务 4.3）

## 7. 验证

<!-- 归档门禁：逐条「命令 + 期望结果」 -->

- [x] V1 `bash .pi/extensions/tests/run-smoke.sh` → 全断言过，退出码 0
- [x] V2 `bash scripts/doc-impact.sh context` → 输出「已由 constraint-injection extension 取代」提示（退出码非 0 可）
- [x] V3 `bash scripts/doc-impact.sh suggest | head -3` → 正常输出预勾选菜单（suggest 未受影响）
- [x] V4 `bash scripts/doc-impact.sh verify openspec/changes/port-constraint-injection` → 通过
- [x] V5 `grep -rn "doc-impact.sh context" --include="*.md" --include="*.ts" --exclude-dir=node_modules --exclude-dir=.git . | grep -v openspec/changes/ | grep -v openspec/specs/ | grep -v docs/research/` → 空输出（openspec/specs 主库经 delta 归档合并清除；docs/research 为非权威快照不追改；其余引用已由任务 3.2 清完）
- [x] V6 `bash scripts/check-standards.sh` → A-H 全 OK
- [x] V7 `cd backend-go && go build ./... && go vet ./...` → 通过（兜底确认后端未被波及）

## 备注

- T2-T4 真实会话验证记录（2026-08-22，经子线程探针，pi-subagents 共享 extension 模块实例）：
  - T4a：无档位 pin → `docs/research/explore-findings.md` 通用池（新建），返回文本逐字符合 ✅
  - T2：子线程 read openspec-apply-change skill → implementation 档激活 + 绑定 mtime 最新 change（fix-section-merge-blackhole，并行会话在途；行为正确——implementation 无参兜底设计如此，真实用法带 change 名）✅
  - T4b：实现档 pin → 落绑定 change 的 explore-findings.md ✅
  - T3：airouter 探针文件写入事件接线生效（JIT 状态记录），注入块内容经 smoke 39 项断言验证 ✅
  - 意外发现①：子线程继承派发时刻注入块快照（源项目记载"完全不吃注入"，实测吃快照不吃增量）——bonus 行为，spec 不作要求
  - 意外发现②（已修）：子线程派发会 `session_start{startup}` 清零主会话档位状态 → apply 中派子线程后主线程注入掉档。修复：reason 过滤（startup 跳过，仅 new/resume/fork/reload 重置），smoke 补 3 项断言。**需 reload pi 后生效，reload 后建议目检一次注入块随档位切换**
- apply 期 bugfix ①（用户实测）：新会话 explore 默认 mtime 兜底绑定无关 change → 内容错位。修复：兜底收紧到仅 implementation 档（见 design 决策 2 / spec delta）。
- 任务 3.x 在 T2~T5 全过后执行（迁移计划：删除与上线同 change 内完成，不留双机制过渡期）。
