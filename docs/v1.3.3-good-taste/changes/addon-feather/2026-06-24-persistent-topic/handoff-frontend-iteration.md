# Handoff：前端交互迭代（2026-06-19 第二轮会话）

> **用途**：本文件是 PersistentTopic change 在"前端交互闭环"这一轮迭代的完整交接记录。新会话读这一份即可接续，无需回滚长上下文。
> **范围**：后端管理 API 补齐 + 前端话题管理/视图模式/泳道/缩放/主题适配。
> **前置**：算法核心（§1–§8，归属/状态机/身份边/回刷/真实数据验证）已于前一轮完成，详见 `tasks.md` 与 `verification-report.md` §1–§8。

---

## 0. 一句话现状

后端管理 API + 测试**全部就绪并验证通过**；前端从"入口缺失"迭代到"话题总览页统一管理 + 2D/3D 双视图（时间线/话题泳道）+ 缩放 + 主题适配"。**typecheck/build/lint/test 全绿**。剩余待办见 §6。

**2026-06-20 上线后修复轮（阶段⑨⑩，本次会话）**：针对真实生产库的三个现象（时间线断链 / 话题未持久化 / 泳道未区分）定位为后端数据层两问题，并修复一个 prompt 质量问题 + 一个 reuse bug。后端 build/lint/service 测试全绿，**已用真实生产库 + 真实 LLM 双重验证**（209/209 归属、identity/similarity 边共存、topic 8 不再收纳不相关事件）。代码改动集中在 `daily_report_matching.go` / `daily_report_cluster.go` / `postgres_migrations.go` + 两个新运维工具。**testcontainer 集成测试因环境（Windows go.exe 连不上 WSL Docker）未跑，push 前需补**。

---

## 1. 本轮变更脉络（按发生顺序）

### 阶段 ① 诊断并修复三个原始问题
用户指出 change 有三处问题：(1) 目录未按 OpenSpec 规范放置；(2) tasks 进度未更新；(3) 前端交互入口和相关 API 缺失。
- **目录归位**：`docs/v1.3.3-good-taste/changes/addon-feather/2026-06-19-persistent-topic/` → `openspec/changes/2026-06-19-persistent-topic/`（依据 `开发执行规范.md` §12.1：开发中 change 在 `openspec/changes/`，里程碑归类是归档后的事）。
- **tasks.md 回填**：70 项全 `[ ]` → 据代码现状回填，已完成打 `[x]` 并标注实现偏离（见 §4）。
- **补齐后端管理 API**（merge/split/rename）+ handler + 路由 + 9 个 testcontainer 测试。
- **补齐前端 API 封装** + 3D 侦探墙详情面板的管理入口（早期版本，后被阶段②取代为主入口）。

### 阶段 ② 用户故事重述 → 话题总览页管理面板
用户反馈："没有回刷按钮，也没法管理衍生话题"。核实发现：
- `backfillPersistentTopics` 前端**无任何调用点**（只能 curl）。
- 管理入口藏在「3D 墙 → 点卡片 → 详情面板」，且**仅当 section 有 persistent_topic 才显示**——而显示前提是"已回刷"，形成**死循环**。
- `app/pages/topics.vue` 是另一套话题系统（文章标签层级图 TagHierarchy），与本 change 的 persistent topic 无关。

→ 在 `BoardThreadBrowser.vue`（"话题总览"页本体）顶部加**话题管理面板**：回刷按钮 + 话题清单（活跃/候选/归档统计）+ 行内重命名/归档/合并。话题清单从 section-timeline 的 `persistent_topic` 字段聚合，无需新后端端点。

### 阶段 ③ 边类型诊断 → 视图模式双轨
用户反馈："侦探墙线索看起来叠加，2D 是匈牙利结果"。核实：
- 后端 `getBoardSectionTimeline` / `GetSectionLifecycle` / `GetTopicLifeline` 的 SQL **不过滤 relation_type**，identity + similarity 两类边都返回。
- **3D**（RedString.ts）读 `relation_type`，identity 实线满 opacity、similarity 虚线半透明——**有区分**。
- **2D**（BoardThreadBrowser.edgePaths）只按 distance 上色，**完全没读 relation_type**——identity 边被"溶解"，观感像"只有匈牙利"。

→ 决策（用户拍板）：**默认只显匈牙利**，另加"话题泳道"视图显 identity。两端对齐。

### 阶段 ④ 三点确认
1. 泳道默认**只留 identity**（filter 掉 similarity），方向 A。
2. 3D 同步加视图切换。
3. 归档话题**不进泳道**（话题清单沉底，泳道不渲染）。

### 阶段 ⑤ 2D 泳道布局 bug
- **间隔不够**：固定 `LANE_H=58`，同天多节点溢出 → 改**自适应高度**（`laneLayout` computed，每泳道高 = 基础行高 + 单天最大节点数 × 间距）。
- **节点没对齐泳道**：节点 cy 漏加 `PAD`，与背景 rect 基准不一致 → 补 `PAD` 对齐。

### 阶段 ⑥ 缩放功能（几经反复）
- v1：Ctrl+滚轮 → **用户否决**（与浏览器网页缩放冲突）。
- v2（定稿）：**+/− 按钮 + 百分比显示**，范围 40%–300%，步进 0.2，点百分比重置。
- bug：缩放条 `position:absolute` 但父级无 `position` → 修；`transform:scale` 不改布局尺寸导致画布"没放大"→ zoom 容器 width/height 按 `svgWidth*scale` 算。

### 阶段 ⑦ 泳道标签 + tooltip + 主题
- 标签截断看不见 → `LANE_LABEL_W` 116→160，新增 `truncateLaneLabel`（阈值 12）。
- hover tooltip：泳道标签 + 节点 `<g>` 各加 SVG `<title>`（完整名/日期/话题/篇数）。
- 主题适配：SVG 颜色改 `theme.value` computed（`svgLaneLabelColor`/`svgLaneStripeColor`）。

### 阶段 ⑧ 缩放条主题 + 3D 切换主题 + 3D 泳道真分赛道
- 缩放条硬编码 → CSS 变量（`--color-bg-overlay` 等）。
- **3D view-toggle 原本根本没写主样式**（裸按钮）→ 补全（深色浮层，跟 days-toggle 同款，active 蓝）。
- **3D 泳道模式原本只切边、不重布局** → `layoutCards(sections, mode)` 加 lanes 分支（Y=话题索引分赛道），`setViewMode` 切换时**重建场景**。

### 阶段 ⑨ 上线后三大问题修复（2026-06-20，后端数据层）
用户用真实生产库报告三个现象：①时间线断开但泳道连通（状态错）；②日报没持久化话题 + 误分类；③侦探墙泳道没区分。根因调研后定为后端数据层两个问题，前端无需改动：
- **根因 1（Issue 1/3）**：唯一约束 `(from,to)` + identity 的 `ON CONFLICT DO UPDATE SET relation_type` → identity 覆盖 similarity。强匈牙利匹配被吞，similarity-only 时间线丢边断链；泳道 layoutKey 依赖 topic.id，NULL 全挤单泳道。
- **根因 2（Issue 2）**：feature 上线只回刷了 board 1974，其余 board 历史从未回刷 → 154/209 section 的 `persistent_topic_id` 为 NULL。
- **修复 A（边共存）**：迁移 `20260620_0001` 拓宽约束为 `(from,to,relation_type)`；`daily_report_matching.go` 三处 INSERT 的 ON CONFLICT 目标同步为三元组。identity 与 similarity 两行独立共存。
- **修复 B（全量回刷）**：新增运维工具 `cmd/rebuild-topics`：InitDB(跑迁移) → 逐 board `RebuildBoardRelations` → `BackfillAllPersistentTopics`。
- **验证（真实库）**：209/209 归属（0 NULL）；identity 95 + similarity 103；33 对 section 同时含两类边（修复前为 0）；section 297 时间线恢复连通。详见 verification-report §10。

### 阶段 ⑩ 聚类 prompt 质量调优（2026-06-20）
用户进一步报告「topic 8『特朗普在 G7 峰会期间的盟友关系紧张』强行归属了不相关事件」。核雪数据发现：report 52 中 LLM 把 3 个语义不相关事件（美以关系/空军一号交付/美伊通牡）塞进同一人名框架。根因是聚类 prompt 的规则 2/4 + 复用规则过于宽松，鼓励「解释性命名」+「优先复用」，在宽泛框架名下吸收沾边事件。
- **修复（方案 A）**：`daily_report_cluster.go` 的 `buildClusterSystemPrompt` 收紧规则 2（单标签独立成组组名必须用原文）、规则 4（标题禁止脑补未提及的外部语境）、复用规则（仅当核心议题延续才复用），加反面教材。
- **验证（真实 LLM）**：新增运维工具 `cmd/verify-cluster-prompt` 跳真实 `ClusterTags`。修复后 topic 8 只收 2 个真正相关事件（美以关系 + 美情报警告内塔尼亚胡），「空军一号」「美伊通牡」移出；组名从「特朗普在 G7...」改为「特朗普盟友关系紧张与外交施压」（去掉脑补的 G7）。详见 verification-report §11。
- **后续**：prompt 只影响新生成日报，已污染的 topic 8 标题仍会被注入——新复用限定已降低风险，观察后再决定是否手动归档。
- **附带修复**：验证中发现「同 topic 被 reuse 两次」的逻辑 bug（`parseClusterResponse` 未查占用），已顺手修：加 `usedTopicIDs` 占用检测 + 单测 `TestParseClusterResponse_DuplicateTopicID_SecondClaimDegrades`，真实 LLM 复验通过。

---

## 2. 文件变更清单（精确）

### 后端（本轮新增的管理 API；算法核心文件见 tasks.md §1–§8）
| 文件 | 性质 | 内容 |
|---|---|---|
| `repository/daily_report_topic_repository.go` | 新增 | `UpdateTopic` / `MergeTopics` / `SplitTopic`（事务 + RebuildBoardRelations 重建身份边） |
| `repository/daily_report_topic_management_test.go` | 新增 | 9 个 testcontainer 用例（rename/archive/reactivate/invalid-status + merge 正常/跨板/target∈source + split 正常/carve-all/foreign） |
| `handler/daily_report_handler.go` | 修改 | `updateTopic`/`mergeTopic`/`splitTopic` handler + `parseTopicID` + 路由注册（PATCH/POST merge/POST split） |

### 后端（2026-06-20 上线后修复轮，阶段⑨⑩）
| 文件 | 性质 | 内容 |
|---|---|---|
| `repository/daily_report_matching.go` | 修改 | 3 处 INSERT 的 `ON CONFLICT` 目标从 `(from,to)` 改为 `(from,to,relation_type)`，identity 与 similarity 边共存；`writeIdentityEdges`/`RebuildBoardRelations` 注释更新为共存语义 |
| `platform/database/postgres_migrations.go` | 修改 | 新增迁移 `20260620_0001`：唯一约束拓宽为 `(from_section_id, to_section_id, relation_type)`。幂等 ||
| `service/daily_report_cluster.go` | 修改 | `buildClusterSystemPrompt` 收紧规则 2/4 + 复用限定 + 反面教材（prompt 质量调优）；`parseClusterResponse` 加 `usedTopicIDs` 占用检测（修同 topic 被 reuse 两次 bug） |
| `service/daily_report_cluster_test.go` | 修改 | 新增 `TestParseClusterResponse_DuplicateTopicID_SecondClaimDegrades`（TDD）|
| `cmd/rebuild-topics/main.go` | **新增** | 运维工具：InitDB(跑迁移) → 逐 board `RebuildBoardRelations` → `BackfillAllPersistentTopics` → 打印验证统计 |
| `cmd/verify-cluster-prompt/main.go` | **新增** | 诊断工具：跳真实 `ClusterTags`，核查聚类 prompt 质量（topic 8 场景） |

### 前端
| 文件 | 性质 | 内容 |
|---|---|---|
| `api/client.ts` | 修改 | 新增 `patch()` 方法 |
| `api/dailyReports.ts` | 修改 | `updateTopic`/`mergeTopics`/`splitTopic` + `PersistentTopic` 类型 |
| `features/tags/components/BoardThreadBrowser.vue` | **重头修改** | 话题管理面板 + 2D 视图切换（timeline/lanes）+ 泳道自适应布局 + 缩放（+/−）+ tooltip + 主题适配 |
| `features/tags/components/TopicDetectiveWall.client.vue` | 修改 | 3D 详情面板话题操作区（早期入口，保留）+ 3D 视图切换 + `setViewMode` 重建场景 + `applyEdgeFilter` |
| `features/tags/components/detective-wall/RedString.ts` | 修改 | `setVisibleByRelationType(type, visible)`（line.visible 开关，零重建） |
| `features/tags/components/detective-wall/CardGroup.ts` | 修改 | `buildCards` 透传 `viewMode` |
| `features/tags/components/detective-wall/TopicWallScene.ts` | 修改 | `loadBoardData` 透传 `viewMode` |
| `features/tags/components/detective-wall/utils.ts` | 修改 | `layoutCards(sections, mode)`：lanes 模式按话题分赛道（Y=话题索引） |

### OpenSpec 文档
| 文件 | 内容 |
|---|---|
| `openspec/changes/2026-06-19-persistent-topic/` | 整个目录从 `docs/v1.3.3-.../addon-feather/` 归位过来（untracked） |
| `tasks.md` | 回填勾选 + 标注实现偏离 |
| `verification-report.md` | 追加 §9 管理闭环 |
| `handoff-frontend-iteration.md` | 本文件 |

---

## 3. 关键决策记录（带理由）

| 决策 | 理由 |
|---|---|
| **目录放 `openspec/changes/`** | §12.1：开发中 change 在此；`docs/v1.x` 是归档后里程碑归类位 |
| **迁移用 GORM AutoMigrate** 而非 `migrations/` 文件 | 现有代码惯例；testcontainer 每次启动幂等建表 |
| **配置用代码默认值** 而非 seed 行 | `DefaultPersistentTopicConfig()`（cluster_threshold=0.28）；`LoadPersistentTopicConfig` 仍读 ai_settings，缺失回落 |
| **聚类算法 complete-link** 而非贪心 | 真实数据证伪贪心（链式合并）；详见 verification-report §3 |
| **管理入口放话题总览页**（BoardThreadBrowser）而非 3D 墙深处 | 3D 入口依赖"已回刷"形成死循环；话题总览是用户找话题的自然位置 |
| **话题清单从 section-timeline 聚合**，不加新后端端点 | 零后端改动；代价：只看当前时间窗内的话题 |
| **视图双轨**：默认时间线（只匈牙利）/ 泳道（只 identity） | 修复 identity 被溶解；泳道显话题连续性；用户拍板方向 A（泳道不留 similarity） |
| **3D 泳道真分赛道**（layoutCards 加 mode） | 原 3D 泳道只切边不重布局，看不出赛道；切 lanes 时 setViewMode 重建场景 |
| **缩放用显式 +/− 按钮**不用滚轮 | Ctrl+滚轮撞浏览器网页缩放，拦不住 |
| **缩放用 CSS transform** 不改 SVG 坐标 | 命中检测不受影响；zoom 容器尺寸按 scale 算以撑开滚动 |
| **3D 保持深色浮层**，不套亮色主题变量 | 3D 是全屏沉浸深色场景（钉板墙暗色调是设计意图）；亮色变量会在深底变黑字黑底 |
| **2D 主题走 CSS 变量 / `theme.value` computed** | 与现有 svgGridColor 等同款模式，切主题跟随 |

---

## 4. 当前实现状态

### 2D 话题总览（BoardThreadBrowser）
| 模式 | 时间线（默认） | 话题泳道 |
|---|---|---|
| 边 | 只匈牙利相似度 | 只 identity（话题色实线） |
| 布局 | 按天分列、同天堆叠 | 按话题分横向泳道（自适应高度） |
| hover | 沿相似度传导 | 沿 identity 传导（同话题） |
| 归档话题 | — | 不进泳道 |
| 缩放 | +/− 按钮条（右下角，40%–300%） | 同 |
| tooltip | 节点/泳道标签 `<title>` | 同 |
| 主题 | CSS 变量 + computed | 同 |

顶部还有：话题管理开关（回刷 + 话题清单 + 重命名/归档/合并）。

### 3D 侦探墙（TopicDetectiveWall）
- 顶部「时间线/话题泳道」切换；lanes 模式**卡片重布局分赛道**（Y=话题索引）。
- 边过滤：timeline 隐 identity、lanes 隐 similarity（`setVisibleByRelationType`）。
- 详情面板保留话题操作区（话题生命线/重命名/归档/合并）——作为沉浸式查看的补充入口。
- **lifecycle 模式禁用视图切换**（避免与单话题生命线场景冲突）。

### 后端 API
| 方法 | 路由 |
|---|---|
| GET | `/api/daily-reports/topics/:id/lifeline` |
| PATCH | `/api/daily-reports/topics/:id`（label / status=active\|archived） |
| POST | `/api/daily-reports/topics/:id/merge`（body `source_topic_ids`） |
| POST | `/api/daily-reports/topics/:id/split`（body `section_ids`, `label`） |
| POST | `/api/daily-reports/backfill-topics`（?board_id） |

---

## 5. 验证状态（全绿）

| 项 | 结果 |
|---|---|
| 后端 build / vet | OK |
| golangci-lint `./internal/topicgraph/...` | 0 issues |
| 后端 test repository + handler | ok（repository ~41s + handler 0.6s，9 个管理测试全过） |
| 前端 `nuxi typecheck` | TYPECHECK_PASS |
| 前端 `pnpm build` | BUILD_PASS |
| 前端 `pnpm test:unit` | 19 files / 116 tests 全过 |
| 前端 eslint（改动文件） | 0 error |

**注意**：前端编译类命令必须走 Windows `cmd.exe`（WSL 缺 native binding）。lint 可 WSL。

---

## 6. 待办与开放问题

### 已确认未做（不阻塞）
- **split 前端入口**：后端 API 就绪，前端无选择器（tasks §9.6 只要合并/重命名/归档）。需做"拆分"按钮 → 勾选 section → 调 splitTopic。
- **§8.5 调参脚本**：以 verification-report §8 手动指引替代。
- **reference 文档 D.2–D.5**：已于 2026-06-21 同步数据库字段、API、架构数据流与配置说明。
- **V.12 全量门禁**：pre-push 动作，本轮跑影响包；push 前补全量。

### tasks.md §9 描述与实际实现的偏差（重要）
tasks.md §9.4/9.6 勾选时描述的是**早期版本**（"3D 详情面板入口"）。本轮已演进出更完整的形态：
- §9.6 管理入口**主入口已移到话题总览页**（BoardThreadBrowser），3D 详情面板入口保留为补充。
- §9.4 "生命周期双模式" 现已实现为 **2D/3D 双视图（时间线/话题泳道）**，比原 task 描述的"话题生命线 vs section 图"更宽。

**建议新会话**：决定是否把这些演进回写进 tasks.md（更新 §9.4/9.6 描述），或在 §9 加注指针指向本 handoff。勿改动已勾选状态以免破坏归档门禁记录。

### 可能的打磨方向（用户尚未确认）
- 泳道里 identity 边若交织过乱：可按 BFS 深度渐变 / 话题色相近时加大泳道间距。
- 缩放步进 0.2 若太粗/太细：调 `ZOOM_STEP`。
- 3D 赛道间距：`laneSpacing = rowHeight * 2.6`（utils.ts layoutCards lanes 分支）。
- 合并用 prompt 序号选择（最小版），可升级为可视化选择器。

---

## 7. 新会话快速接入

1. **读本文件** + `tasks.md`（看勾选状态 + 偏离标注）+ `verification-report.md` §9。
2. **关键入口**：话题总览页 = `BoardDailyReportTimeline.vue` 的「话题总览」按钮 → `BoardThreadBrowser.vue`。
3. **改动最集中的文件**：`BoardThreadBrowser.vue`（2D 视图/泳道/缩放/管理全在这）。
4. **3D 布局**：`detective-wall/utils.ts` 的 `layoutCards(sections, mode)`。
5. **验证命令**：见 §5（前端走 cmd.exe）。
6. **继续工作**：从 §6 待办挑一项，或等用户新需求。

### 本次会话的原始三个诉求（已全部闭合）
1. ✅ 目录按 OpenSpec 规范放置（归位到 openspec/changes/）
2. ✅ 进度更新到 task（tasks.md 回填）
3. ✅ 前端交互入口 + 相关 API（管理 API + 话题总览页入口 + 双视图）

---

## 8. 2026-06-21 回归修复补充

- candidate 连续命中达到阈值后仍保持 candidate，只有管理面板人工确认才转 active；未达阈值的 PATCH active 被后端拒绝。
- candidate 中断命中会持久化清零，确保门禁是“连续多天”而非累计多天。
- 时间线状态只使用 similarity 边；identity 仅服务话题泳道连续性。
- 2D/3D hover 改为完整连通分量；7/14/30/60 天窗口精确生效。
- 2D 工具组整体靠右，节点节距/文字描边、日期标尺与缩放容器已重构。
- 验证：后端 lint/vet/repository test/build 全绿；前端 lint 0 error、typecheck、19 files / 117 tests、build 全绿。
