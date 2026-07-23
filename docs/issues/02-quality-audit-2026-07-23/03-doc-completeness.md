# 文档完整性与一致性审计报告

> **审计对象**: `docs/reference/`（flow / standard / architecture / api / database）+ openspec changes 编排纪律
> **对照规范**: `docs/reference/开发执行规范.md` §0.5 文档归属规则 + §11 归档门禁 + §12 文档流转
> **客观校验**: `bash scripts/check-standards.sh` 实测（A–G 七段）+ `bash scripts/doc-impact.sh verify`
> **评级**: **B**（整改后升至 B+：causal-analysis-agent 文档组已补齐，doc-impact verify 0 FAIL，check-standards 69 PASS / 0 FAIL 全绿）
> **审计日期**: 2026-07-23

## 整改进度（2026-07-23）

| 问题 | 状态 | 修复 |
| ---- | ---- | ---- |
| H2 causal-analysis-agent 文档零跟进 | ✅ 已修复 | 补 4 处文档 + 修 tasks.md 声明，doc-impact verify 3 FAIL→0 |
| H3 API 文档缺 QA 端点 | ✅ 已修复 | api/dataenrichment.md 补 QA/sediment 4 端点 |
| H4 DB 文档缺 topic_enrichment_qa 表 | ✅ 已修复 | DATABASE_FIELDS.md 补 §10.6 + sectors 语义 + 计数 44 |
| H5b docs/README 死链 | ✅ 已修复 | 删除 summaries.md 死链行 |
| flow/data-enrichment.md 未跟进 agent loop | ✅ 已修复 | 补报告追问子流程 + 4 探索工具 + 7 Operation 速查 |
| ai-logging.md 缺 qa_tool_use | ✅ 已修复 | 登记 qa_tool_use + session_id 规则 + Capability 映射 |
| H6 watch-keyword 前置声明 | ✅ 已修复 | 改为符合现状 |
| otel-tracing-completion 停滞 18 天 | ⏳ 待排期 | 需用户确认废弃或重启 |

## 客观校验基线

```
scripts/check-standards.sh
==============================
通过 68 / 失败 2
==============================
```

| 段 | 内容 | 结果 |
| -- | ---- | ---- |
| A | flow 五段式齐全 | ✅ 8/8 OK |
| B | domain 白名单 | ✅ OK |
| C | 双主题 | ✅ OK |
| D | standard 防孤立 | ✅ 11/11 OK |
| E | flow 变更溯源链接（archive 区） | ✅ OK |
| F | doc-impact 声明对账 | ❌ **causal-analysis-agent FAIL** |
| G | 导航层文档死链 | ❌ **docs/README.md → summaries.md 死链** |

**结论**：文档**结构层**优秀（A–E 段全过），问题集中在 **F/G 两段**——都是「内容跟进」类问题，非结构缺陷。

## 总体评价

三层文档体系（flow/standard/architecture）骨架完整且被工具强制兜底。flow 五段式 100% 齐全、standard 防孤立 100% 通过、map.md 索引完整、AGENTS.md 深链全部有效——这是文档体系成熟度高的表现。当前问题**全部集中在进行中 change（causal-analysis-agent）的「代码先行、文档后补」**上，以及个别长期停滞 change 的流程纪律问题。

## 正面发现（保持）

| 维度 | 结论 |
| ---- | ---- |
| flow 五段式齐全 | 8 篇 flow 文档全部通过 A 段校验（需求说明/链路设计/业务约束/代码入口/变更溯源） |
| standard 防孤立 | 11 个 standard 文档全部被 AGENTS.md 或 README.md 引用，D 段 11/11 OK |
| map.md 索引完整 | 覆盖全部 5 domain + 全部 8 flow，业务域索引表完整 |
| AGENTS.md 深链有效 | 根/子 AGENTS.md 引用的 12 个 standard 路径 + 执行规范路径**全部存在** |
| doc-impact 双源注入机制 | flow 业务约束 what + standard 代码 how 自动注入，设计精良 |
| 前端小 feature 无独立 flow | feeds/preferences/settings 并入 reading.md/scheduler.md 是合理切分，非遗漏 |

---

## 问题清单

### 一、进行中 change 文档零跟进（归档门禁阻塞）

#### [High] H2 — causal-analysis-agent 文档零跟进（doc-impact verify 3 FAIL）

- **问题**: causal-analysis-agent 的 5 个新代码文件已写入工作区，但文档零跟进
- **涉及新文件**:
  - `backend-go/internal/dataenrichment/service/exploration.go`（list_boards/list_lanes/get_lane_detail 工具）
  - `backend-go/internal/dataenrichment/service/qa_agent.go`（报告追问 agent）
  - `backend-go/internal/dataenrichment/service/web_search.go`（web_search 工具接口）
  - `backend-go/internal/dataenrichment/service/lens_source.go`（分析视角来源接口）
  - `backend-go/internal/dataenrichment/board_listers_impl.go`（DB 实现）
- **证据**: `bash scripts/doc-impact.sh verify openspec/changes/causal-analysis-agent/` 实测 3 FAIL：
  ```
  - 疑似遗漏: 改了 api 相关代码未声明 api
  - 声明了未更新: docs/reference/database/DATABASE_FIELDS.md
  - 声明了未更新: docs/reference/standard/backend/ai-logging.md
  verify: openspec/changes/causal-analysis-agent/ — 3 FAIL
  ```
- **根因**: tasks.md:88 声明 `<!-- doc-impact: flow, database, standard -->` 但实际改了 api 却遗漏声明；且声明的 database/standard 也未实际更新
- **建议**: 按 verify 提示补齐 4 处文档（见 H3/H4 + flow + ai-logging），并把声明改为 `flow, database, standard, api`

#### [High] H3 — API 文档缺 4 个 QA 端点

- **位置**: `docs/reference/api/dataenrichment.md`
- **证据**: 代码 `handler.go:132-141` 已注册端点，文档 grep 全空：
  ```
  POST   /results/:id/qa          (handler.go:132)
  GET    /results/:id/qa          (handler.go:136)
  POST   /qa/:id/sediment         (handler.go:139)
  ```
- **建议**: 在 dataenrichment.md 补这 3-4 个端点（trigger/result qa、sediment）

#### [High] H4 — DB 文档缺 topic_enrichment_qa 表 + sedimented 列

- **位置**: `docs/reference/database/DATABASE_FIELDS.md` §10
- **证据**: §10 数据增强域列了 10.1-10.5（board_data_sources/lifeline_context/result/review/stock_debate），**缺 10.6 `topic_enrichment_qa` 表**；而迁移已建表 + 加 `sedimented` 列（`postgres_migrations.go:1113,1132-1143`）
- **建议**: 补 §10.6 `topic_enrichment_qa` 表结构（result_id/sedimented/question/answer/created_at 等），并核对表总数

#### [High] H5 — flow/data-enrichment.md 未跟进 agent loop 新工具

- **位置**: `docs/reference/flow/data-enrichment.md`
- **证据**: 新代码 exploration/qa_agent/web_search/lens_source 在该 flow 文档及 architecture/map.md、backend.md 中**零提及**（grep 全空）
- **建议**: 在「链路设计」「代码入口」节补 causal-analysis-agent 阶段 2b/2c 的 agent loop（exploration + lens + qa + web_search 工具集）

### 二、导航层死链（check-standards G 段 FAIL）

#### [High] H5b — docs/README.md 死链

- **位置**: `docs/README.md:47` → `reference/api/summaries.md`
- **证据**（已主线程验证）: `ls docs/reference/api/summaries.md` → No such file；`api/_index.md` 也无 summaries 条目
- **影响**: check-standards.sh G 段实测 FAIL
- **建议**: 删除第 47 行该条目（summaries 相关 API 已并入 articles.md），或补建 api/summaries.md

### 三、流程纪律问题

#### [High] H6 — watch-keyword-and-quickadd 前置依赖声明错误

- **位置**: `openspec/changes/watch-keyword-and-quickadd/tasks.md:5`
- **证据**: 声称「前置 topic-watchlist-observability 已归档」，但 `ls openspec/changes/archive/ | grep topic-watch` = **空**，topic-watchlist-observability 仍在 active 区（32/49）
- **建议**: 要么先推进 topic-watchlist-observability 归档，要么修正前置声明

#### [Medium] — otel-tracing-completion 停滞 18 天 + 0/12

- **位置**: `openspec/changes/otel-tracing-completion/`
- **证据**: 最后提交 2026-07-05，至 2026-07-23 已停滞 18 天，tasks 0/12；且依赖前置 `ai-call-logging-schema` 已于 2026-07-05 归档（前置已满足），可恢复
- **建议**: 确认是否废弃，或排期重启

### 四、openspec changes 进行中状态总览

进行中 change 实为 **6 个**：

| Change | tasks 进度 | 最后提交 | 状态 |
|--------|-----------|---------|------|
| data-enrichment-orchestration | 51/69 (74%) | 2026-07-18 | 活跃 |
| topic-watchlist-observability | 32/49 (65%) | 2026-07-04 | 活跃 |
| board-discovery-expansion | 20/46 (43%) | 2026-07-19 | 活跃 |
| causal-analysis-agent | 0/63 (0%) | 2026-07-23（最新） | ⚠️ **代码先行，task 未勾 + 文档未补** |
| watch-keyword-and-quickadd | 0/43 (0%) | 2026-07-04 | ⚠️ **未启动 + 前置声明错误** |
| otel-tracing-completion | 0/12 (0%) | 2026-07-05 | ⚠️ **停滞 18 天** |

**[Low]** 大部分 active change 未声明 doc-impact（仅 causal-analysis-agent 声明）——属过渡期正常，check-standards F 段判定 OK。

---

## 文档完整性总评：B

**理由**：
- **结构层优秀**：三层文档体系骨架完整，flow 五段式 100%、standard 防孤立 100%、map.md 索引完整、AGENTS.md 深链全有效。体系成熟度高。
- **内容层有明显遗漏**：集中在 causal-analysis-agent 这个进行中 change——代码已写（5 个新文件）但文档零跟进，导致 API 文档、DB 文档、flow 文档三方不一致，`doc-impact.sh verify` 直接 FAIL 3 项。这是该 change 归档的硬阻塞。
- **流程纪律有破绽**：watch-keyword 声称前置已满足实际未满足；otel-tracing 停滞 18 天无处置；docs/README 死链未修。
- **整改后预期**：补齐 causal-analysis-agent 文档（H2/H3/H4/H5）+ 修死链（H5b）后，check-standards.sh 将从 68/2 变为 70/0 全绿。

**关键文件路径**（整改入口）：
- `D:\project\Syntopica\docs\reference\api\dataenrichment.md`（缺 QA 端点）
- `D:\project\Syntopica\docs\reference\database\DATABASE_FIELDS.md`（缺 §10.6）
- `D:\project\Syntopica\docs\reference\flow\data-enrichment.md`（缺 agent loop）
- `D:\project\Syntopica\docs\reference\standard\backend\ai-logging.md`（待更新）
- `D:\project\Syntopica\docs\README.md:47`（死链）
- `D:\project\Syntopica\openspec\changes\causal-analysis-agent\tasks.md:88`（doc-impact 声明缺 api）
- `D:\project\Syntopica\openspec\changes\watch-keyword-and-quickadd\tasks.md:5`（前置声明错误）
