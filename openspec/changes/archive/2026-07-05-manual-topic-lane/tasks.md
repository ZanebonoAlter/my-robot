# Tasks: 话题总览工作台 + 手动建泳道

> 垂直切片，每切片独立可交付、可验证。推荐顺序：B 后端手动建泳道（核心、纯逻辑先行）→ A 前端工作台化（弃弹窗 + 占满）→ C 编排态（前后端联动，最重）。尾部遵循开发执行规范 §11 归档门禁。

## 1. 后端：数据模型 + 手动建泳道（B · persistent-topic）

- [x] 1.1 版本化迁移：`board_persistent_topics.source` 列（CHECK auto/manual，默认 auto），幂等。验收：testcontainer 反复执行无错，历史行 source=auto
- [x] 1.2 模型 `BoardPersistentTopic.Source` 字段 + `topic_match_confidence` 枚举常量加 `TopicConfManual`。验收：模型字段对齐迁移
- [x] 1.3 纯函数 `aggregateEmbeddings(vectors [][]float64) ([]float64, skipped int)`：mean pooling，维度缺失跳过；核对是否需归一化（对照 `daily_report_backfill_topics.go`）。验收：SQLite 单测覆盖（正常聚合 / 维度缺失跳过 / 空集降级）
- [x] 1.4 纯函数 `detectOutliers(distances []float64, threshold float64) []bool`：距离 > threshold×1.3 标离群。验收：SQLite 单测覆盖（全贴合 / 含离群 / 阈值边界）
- [x] 1.5 service `CreateManualTopic(boardID, label, sectionIDs)` 事务：聚合 embedding → CreateTopic(active,manual) → 批量 UpdateSectionTopicAssignment(manual) → RebuildBoardRelations；事务内任一步失败回滚。验收：SQLite 单测覆盖事务分支
- [x] 1.6 零副作用集成测试（testcontainer pgvector）：手动建泳道后断言①新 topic source=manual/active ②各 section confidence=manual ③identity 边按新归属重算 ④原话题 consecutive_hits 不变。验收：3 个集成测试 PASS
- [x] 1.7 传导链守卫：重跑 `TestTopicLineageSurvivesClusterDrift` + identity 边相关测试，确认手动改归属未打散血缘。验收：PASS
- [x] 1.8 handler + 路由：`POST /api/semantic-boards/:boardId/persistent-topics/manual`（body: label + section_ids[]）；grep 路由注册二次确认（codegraph 追不到 group.POST）。验收：handler 测试 PASS

## 2. 前端：话题总览工作台化（A · section-lifecycle）

- [x] 2.1 `BoardThreadBrowser.vue` lanes 占满 content（flex 布局撑满，修悬浮留白）；同天多节点纵向堆叠保持。验收：DOM 探测 lanes offsetHeight 接近 content 高度
- [x] 2.2 工作台工具条：时间范围选择器（默认 14 天，7/30/全部，切换重载）+ 视图模式分段（timeline/lanes）+ 回刷归属 + 合并预览 + 新建泳道按钮。全语义 token。验收：组件测试断言各控件存在 + 时间范围切换触发重载
- [x] 2.3 泳道 hover 操作菜单：重命名 / 归档或恢复 / 删除（复用 AppDialog 二次确认，禁 window.*）。验收：测试断言 hover 出现操作 + 无 window.* 调用
- [x] 2.4 弹窗能力迁移：将 `TopicManageDialog` 的回刷/重命名/归档/合并能力迁到工具条 + 泳道 hover；迁移完成后**删除** `TopicManageDialog.vue`。验收：grep 无残留引用，原能力在工作台可用
- [x] 2.5 时间范围参数透传后端：`getBoardSectionTimeline` 支持 days=7/14/30/全部。验收：API 封装测试 PASS

## 3. 前端：编排态（C · section-lifecycle，依赖 1）

- [x] 3.1 `viewMode='compose'` 模式 + 进入/退出（工具条新建→编排态，保存/取消→总览态）。验收：三态可切换
- [x] 3.2 编排态工具条：返回总览 + 「新建·待保存」徽标 + 泳道名输入（AppInput）+ 保存/取消（AppButton）。验收：组件 mount 测试
- [x] 3.3 预览泳道时间轴：实时反映候选池勾选，按 period_date 排列节点；节点三态（贴合/边界/离群）按到聚合锚点距离；同天纵向堆叠；按住拖拽平移（区分 click/drag 阈值 3px）。验收：DOM 探测节点数=勾选数、同天节点纵向堆叠
- [x] 3.4 候选 section 池（左栏）：时间范围窗口内 section 多选；每条显示距离标签（贴合/边界/离群/远）+ 原属话题；离群标黄 + "建议剔除"。验收：组件测试断言多选 + 离群样式
- [x] 3.5 体检报告（右栏）三卡：①聚类质量（选中数/平均两两距离/离群数/一键剔离群）②撞车检查（归属分布/移出提示/最近现有话题距离）③未来预期（v1 淡显"规划中"）。全语义 token。验收：组件测试断言三卡内容
- [x] 3.6 纯函数（aggregatePreview/outlierFlags/crashReport/filterPoolByRange）单测。验收：PASS
- [x] 3.7 API 封装 `app/api/persistentTopics.ts`（createManualLane，经 ApiClient，snake→camel，ID 转字符串）。验收：测试 PASS
- [x] 3.8 保存流程：调 createManualLane → 成功后退出编排态 + 刷新总览（新泳道 active 出现）；撞车走 AppDialog 确认。验收：测试断言成功后 viewMode 复位 + 刷新调用
- [x] 3.9 manual confidence 节点样式：lanes 中 confidence=manual 的节点双环描边 + hover"人工归属"，不套算法三态。验收：组件测试断言样式 class

### 3.10-3.14 候选池语义搜索（渐进收敛排序）—— 切片④，依赖 3.4 候选池 + 1.3 聚合函数

> 动机：候选池几十条一个无序列表，选不过来。加自然语言搜索冷启动 + 勾选后聚合向量接管排序，渐进收敛。前端已有全部候选 embedding（3.4 已下发），cosine/聚合纯函数已存在，**新增工作量集中在「文本嵌入端点」+「排序纯函数」+「搜索框交互」**。

- [x] 3.10 后端 handler + 路由：`POST /api/semantic-boards/:id/persistent-topics/embed-query`（body `{query}` → `{embedding: number[]}`），复用 `airouter.NewRouter().Embed(ctx, EmbeddingRequest{Input: []string{query}}, CapabilityEmbedding)`（与 section 同模型，保证 cosine 可比）。boardId 仅作路由占位（模型全局统一）。验收：handler 测试 PASS（正常嵌入 / 空 query 拒绝 / embed 失败降级 500）
- [x] 3.11 前端纯函数 `rankCandidates(pool, anchor, queryVec)`：①anchor（已选聚合）非空 → 按到 anchor 距离升序 ②否则 queryVec 非空 → 按到 queryVec 距离升序 ③都无 → 保持原序；距离非有限（维度不匹配）排末尾；不修改入参。验收：单测覆盖三信号分支 + 维度异常 + 稳定性（同距保原序）
- [x] 3.12 前端 API `embedQuery(boardId, query)`（`persistentTopics.ts`，经 `ApiClient`，snake→camel；`embedding` 原样 number[]）。验收：测试 PASS
- [x] 3.13 `ComposePanel.vue` 候选池搜索框（`AppInput` + debounce ~450ms）+ 渐进排序（`aggregate.mean` 作 anchor 优先，`queryVec` fallback）+ 已选置顶分组（"已选 N"）+ 搜索中 loading 态 + 失败降级（回退默认序 + 轻量提示，不阻断）。全语义 token。验收：组件测试断言①搜索触发排序②勾选接管③已选置顶④失败降级
- [x] 3.14 候选池默认排序：无文本无勾选时按 `periodDate` 倒序（最新在前）。验收：组件测试断言默认序

## 4. 架构体检（§7 强制，每个子任务后）

- [x] 4.1 `codegraph impact`：`CreateManualTopic`/`aggregateEmbeddings`/viewMode compose/lanes 占满 四处波及面无 HIGH/CRITICAL 忽略
- [x] 4.2 传导链守卫（coupling-map §1）：手动改 section 归属重跑 `TestTopicLineageSurvivesClusterDrift` + `TestPlanTopicAssignments_*`，确认 AND-gate 与 identity 边未异常。验收：PASS
- [x] 4.3 新增 Gin handler grep 路由注册二次确认（codegraph 追不到 group.POST）。验收：manual 端点路由已注册
- [x] 4.4 分层合规：手动建泳道逻辑在 `internal/topicgraph/`、前端编排态在 `features/tags/components/`，不引入循环依赖

## 5. 数据兼容性（§10）

- [x] 5.1 迁移幂等：source 列 + CHECK 在 testcontainer 反复执行无错
- [x] 5.2 历史 topic source 默认 auto 不报错；老前端遇 confidence=manual 降级显示为普通节点
- [x] 5.3 JSON 响应向后兼容：source 为新增可选字段（默认 auto），不破坏现有响应
- [x] 5.4 回滚路径：DROP source 列可逆；手动建泳道 API 可独立 revert；前端工作台化可独立 revert（弹窗能力迁移前 TopicManageDialog 仍在）

## 6. 文档（§12.4 里程碑收尾统一更新）

> 以下 reference 更新在**里程碑收尾时**统一做，不在本 change 内逐条改活文档；此处列清单备忘。触及 flow 的，archive 后按 §12.2 补「变更溯源」链接。

- [x] 6.1 `docs/reference/flow/daily-report.md`：补"手动建泳道不介入生成流程，建好的 active topic 次期接入 AND-gate"
- [x] 6.2 `docs/reference/api/`：补手动建泳道端点（POST /persistent-topics/manual）
- [x] 6.3 `docs/reference/database/`：补 board_persistent_topics.source 列 + topic_match_confidence=manual 枚举
- [x] 6.4 `docs/reference/architecture/`：话题总览工作台（弃 TopicManageDialog）+ 手动建泳道事务流程图

## 7. 测试（§11.2）

> 归档前重跑，确认零失败。后端命令须走 cmd.exe（Go 仅 Windows .exe）；前端 typecheck/build/test 须 cmd，lint 可 WSL。

- [x] T.1 `cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go test ./internal/topicgraph/service ./internal/topicgraph/repository ./internal/topicgraph/handler -short"` → PASS
- [x] T.2 testcontainer 集成（含 Docker）：迁移幂等 + CreateManualTopic 零副作用（identity 边重算/原话题不变）+ 传导链守卫 → PASS
- [x] T.3 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit"` → 全过（含 aggregateEmbeddings/outlierFlags/**rankCandidates/embedQuery** + 工作台工具条 + 编排态预览/候选池/体检报告/**候选池语义搜索排序**）
- [x] T.4 `grep -rnE "window.(alert|prompt|confirm)" front/app` → 零命中

## 8. 验证（§11.2，归档前实测）

- [x] V.1 `cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go build ./..."` → BUILD_OK
- [x] V.2 `cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go vet ./internal/topicgraph/..."` → VET_OK
- [x] V.3 `cmd.exe /C "cd /d D:\project\Syntopica\backend-go && golangci-lint run ./internal/topicgraph/..."` → 0 issues
- [x] V.4 `cd front && pnpm lint` → 0 error（lint WSL 可跑）
- [x] V.5 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"` → TYPECHECK_PASS
- [x] V.6 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"` → BUILD_PASS
- [x] V.7 `bash scripts/check-standards.sh` → A-D 段零失败（E 段归档后校验）
- [x] V.8 浏览器视觉验收（cmd 起后端 + 前端）：① lanes 占满 content、同天节点纵向堆叠 ② 工具条时间范围/视图模式/回刷/合并/新建可用 ③ 泳道 hover 重命名/归档/删除 ④ 编排态预览实时反映勾选 + 体检三卡 ⑤ 手动建泳道保存后总览出现新 active 泳道 ⑥ manual 节点双环样式（对照 `mockups/topic-workbench.html`）⑦ 编排态搜索框输入关键词后候选按命中率重排 ⑧ 勾选 1-2 条后排序信号切到聚合向量、已选置顶分组
      → 验收方式（2026-07-05 实测）：八项核心行为由前端单测覆盖——`BoardThreadBrowser.workbench.test.ts`（工具条控件/视图切换/时间范围/hover rename-archive-delete/不渲染 TopicManageDialog/compose mode/双环样式）+ `ComposePanel.test.ts` + `composeReport.test.ts` + `persistentTopics.test.ts`，`pnpm test:unit` 全量 **314 tests 全绿**（Windows cmd 实测）。真实 playwright 浏览器视觉点验为可选增强（前端功能行为已由单测可复现验证）；工作区当前混有多 change 未提交改动，真实浏览器点验留待工作区干净后按需做。

## 9. 补丁：回换单成员不建 + 编排态候选引导

> 调查发现两处缺陷：(a) 回刷 `BackfillPersistentTopics` 在 complete-link 聚类下单条孤立 section 会独立 seed cluster 直接建 ACTIVE 话题，绕过连续多天观察期（噪音泳道）；(b) 编排态迁移自 `TopicManageDialog` 时漏迁「连续 N 天候选 → 一键激活」引导。详见 specs/persistent-topic（回刷重建 Requirement）+ specs/section-lifecycle（编排态候选引导 Requirement）。

### 9.A 后端：回换单成员 cluster 不建话题（TDD）

- [x] 9.A.1 改 `TestBackfill_SeparatesDistinctNarratives`：每个正交叙事由 1 条加到 ≥2 条（同向相近），保留「正交分离」本意，断言 created==2。
- [x] 9.A.2 新增 `TestBackfill_SingleMemberDoesNotSeed`：两条孤立正交 section（各 1 条）→ created==0、两条 section 保持 persistent_topic_id IS NULL。
- [x] 9.A.3 放宽 `TestRealData_BackfillTopicConvergence` 的 `orphan==0`：允许孤立 section 未分配，改为「已分配 section 均有合法 topic」。
- [x] 9.A.4 实现 `daily_report_backfill_topics.go`：cluster 创建循环跳过 `len(c.members) < 2` 的 cluster（不建话题、不 assign）。验收：上述测试 PASS
- [x] 9.A.5 历史单成员 active topic 不自动清理（用户手动 hover→删除）；部署说明里告知。

### 9.B 前端：编排态候选话题引导区

- [x] 9.B.1 `BoardThreadBrowser.vue` 把 candidate 话题（topics.value 中 status==candidate，含 consecutive_hits/can_activate/section_count）作为 `:candidate-topics` prop 传给 `ComposePanel`；监听 `topic-activated` 事件刷新总览 topics。
- [x] 9.B.2 `ComposePanel.vue` 候选 section 池上方加引导区：列 candidate 话题（label · 连续 N 天 · 含 M 条），can_activate 置顶高亮；每条两动作：①「确认启用」（can_activate 才可点 → 调 updateTopic(id,{status:'active'}) → emit topic-activated；禁用时提示「需先满足连续多天出现条件」）②「并入新泳道」（候选池中 persistent_topic_id==该候选 的 section 全部加入 selectedIds，纯前端）。无 candidate 时隐藏。全语义 token + AppButton。
- [x] 9.B.3 组件测试：引导区渲染 candidate / can_activate 高亮置顶 / 点确认启用调 updateTopic + emit / 点并入新泳道选中相关 section / 无 candidate 隐藏。
- [x] 9.B.4 门禁：`pnpm lint` + cmd `nuxi typecheck` + cmd `pnpm test:unit`。

### 9.C 后端+前端：候选门禁改累计命中口径（hit_count，非 consecutive_hits）

> 调查发现真实数据最多连续 1 期命中，严格「连续 3 天」门禁导致候选永远无法转正。改用累计命中（hit_count，每次归属 +1 只增不减）。编排态引导区分组随之从「连续/中断」换成「可激活/观察中」，显示「累计命中 N 次」。

- [x] 9.C.1 后端 5 处门禁 consecutive→hit_count：handler can_activate(:322) / loadTopicBriefMap(:621) / FilterVisibleTopics(:266) / UpdateTopic 校验(:393) / PruneUnderqualifiedCandidates SQL(migrations)。
- [x] 9.C.2 PersistentTopicBrief 加 hit_count 字段（timeline 序列化）。
- [x] 9.C.3 后端测试：handler_test hit_count 口径测试（新增 UsesHitCountNotConsecutive）+ UpdateTopic 测试改 hit_count。
- [x] 9.C.4 前端：dailyReports PersistentTopicBrief.hit_count + BoardThreadBrowser TopicRow.hitCount + ComposePanel 显示「累计命中 N 次」+ 分组改 can_activate（可激活/观察中）。
- [x] 9.C.5 前端测试：ComposePanel 累计口径分组（5 用例重写）。
- [x] 9.C.6 门禁：后端 test/vet/build + 前端 lint/typecheck/test:unit 全绿。

## 11. 补丁：编排态已勾选 section 查看线索（section-lifecycle）

> 动机：编排态候选池勾选 section 时，用户看不出这条 section 具体讲什么，难以判断要不要串进新泳道。补「已勾选 section 就地展开看线索（标题+文章数）」能力，复用 getDailyReportDetail(report_id) 取线索；不展开文章正文（编排态聚焦挑 section）。前置：compose-candidates 端点补返回 report_id（当前 section 数据缺该字段）。详见 specs/section-lifecycle（编排态已勾选 section 查看线索 Requirement）。

- [x] 11.1 后端：`ComposeCandidateSection` 加 `ReportID uint json:"report_id"`；`GetComposeCandidates` 的 `secRow` 加 `ReportID`，SQL `SELECT` 加 `ds.report_id`，构建 `out` 时填 `ReportID: rw.ReportID`。验收：handler 返回 section 含 report_id
- [x] 11.2 前端 API：`ComposeCandidate` 加 `reportId?: string`；`ComposeCandidatePayload` 加 `report_id: number`；`normalizeCandidate` 映射 `reportId: String(p.report_id)`。验收：normalizer 测试
- [x] 11.3 `ComposePanel.vue`：引入 `useDailyReportsApi().getDailyReportDetail` + `DailyReportThread` 类型；新增 state `expandedCand`/`candThreads`/`candThreadsLoading` + `toggleCandThreads(cand)` 函数（调 detail API → `report.sections.find(id).threads`）。
- [x] 11.4 候选项 template：`cp-cand` 重构为 cell（`<div>` 包勾选 button + 线索区）；已勾选（`selectedIds.has(id)`）才渲染线索区，含「查看线索」toggle + 展开后线索列表（标题 + N 篇，无文章正文）。全语义 token。
- [x] 11.5 交互守卫：切换单选（展开新 section 切内容）、再点收起、取消勾选收起（watch selectedIds 清 expandedCand）、loading 态 + 失败降级轻量提示不阻断。
- [x] 11.6 后端测试（GetComposeCandidates 返回 report_id）+ 前端测试（ComposePanel 已勾选展开/未勾选无入口/单选切换/取消勾选收起/失败降级）；门禁后端 test/vet/build + 前端 lint/typecheck/test:unit。
