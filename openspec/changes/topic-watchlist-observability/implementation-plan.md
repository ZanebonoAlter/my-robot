# 实现计划：关注标记 + 话题专注视图 + 泳道上下文注入

> 本计划是 OpenSpec change `topic-watchlist-observability` 的施工编排，遵循 `docs/reference/开发执行规范.md` §0.6。三切片按依赖与风险排序，独立可派发。每个子线程任务块自包含（含验收命令），可并行/串行。

## 全局环境约束（每个子线程必读）

1. **Go 命令必须走 cmd.exe**：本机 Go 仅有 Windows `.exe`（`D:\tool\Go\bin\go.exe`），WSL bash 无法执行。后端 build/vet/test/lint 一律：
   ```bash
   cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go build ./... && echo BUILD_OK"
   cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go vet ./internal/topicgraph/..."
   cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go test ./internal/topicgraph/... -short"
   ```
2. **前端 typecheck/build 必须走 cmd**（AGENTS.md 已知，缺 native binding）；**lint 可 WSL**。
3. **测试只跑本次修改影响的包**，不跑全量 `go test ./...`（AGENTS.md）。
4. **TDD 铁律**（§2）：每个切片先写失败测试，再写最小实现。
5. **codegraph 已知局限**：Gin `group.POST(..., fn)` 追不到，新增 handler 删前必 grep 路由注册二次确认。

## 关键代码事实（已调研）

- `buildClusterSystemPrompt`（`daily_report_cluster.go:15`）：注入循环用 `t.ID/Label/statusLabel/LastSeenDate/HitCount`，纯 label。D 在 active 分支补近期内容。
- `ClusterTags`（`daily_report_cluster.go:74`）：调用方，签名 `(ctx, tags, existingTopics)`。existingTopics 来自 `ListAnchorableTopicsByBoard`（orchestrator:48）。
- `GetTopicLifeline(topicID)`（`daily_report_repository.go:693`）：按 `persistent_topic_id` 聚合 section（含 cluster_label/period_date/thread_count）。D 复用此数据源，**额外补拉 thread 标题**（按 fit_distance 升序，每 section 取 1-2 条）。
- 日报生成两入口：`handler/daily_report_handler.go:117/164`（手动）+ `admin/scheduler/job_daily_report.go:61`（定时），均 `GenerateDailyReport → SaveReport`。归属 `assignAndUpdateTopics` 跑在 `SaveReport` 事务内（`daily_report_repository.go:230`）。
- `BoardThreadBrowser.vue`：`viewMode` 现 `'timeline'|'lanes'`；节点点击 `selectNode`→`section.threads` 加载→全屏 popup；`svgScrollRef` 做横向 scroll pan。
- `DailyReportMasthead.vue` 是日报顶部报头；A 的关注栏插在 masthead 之下、正文分区之上。
- 组件库：`AppButton/AppDialog/AppInput/AppToggle/AppTooltip`（`components/ui/`、`components/common/`）。双主题 token 见 `assets/css/main.css`。
- 迁移文件：`platform/database/postgres_migrations.go`，命名 `YYYYMMDD_NNNN`（最新 `20260529_0002`）。

## 传导链提醒（§7.1 已知局限2）

D（改 ClusterTags 注入）处于 coupling-map §1 传导链上（聚类输入→topic 锚定）。但方向是**增强输入信息量**（label→label+内容），预期**减少误吸、强化血缘**，与 §1 记录的"截断打散血缘"相反。兜底守卫测试：`TestPlanTopicAssignments_AnchorHit_MatchedWithinThresholdNotNearest` + `TestTopicLineageSurvivesClusterDrift`。子线程改完 D 必须重跑这两个测试确认未打散血缘。

---

## 切片 D：泳道上下文注入（后端，核心，优先）

> capability: `persistent-topic`（MODIFIED `ClusterTags 注入历史叙事框架`）。tasks.md §1。

### D-T1 新增"话题近期 brief"查询（RED→GREEN）
- 新增 repository 方法 `ListTopicRecentBriefs(boardID, days, perTopicLimit)`：按 active 话题拉最近 `days`(=7) 天的 section（cluster_label, period_date）+ 每条 section 代表性 thread 标题（join `daily_report_threads`，按 `fit_distance` 升序，每 section 至多 2 条）。返回 `map[topicID][]TopicRecentItem`。按 period_date DESC + 截断到 `perTopicLimit`。
- 测试（内存 SQLite 或 testcontainer）：构造 active 话题 + 历史 section + thread，断言返回结构正确、7 天窗口、perTopic 截断、fit_distance 排序。
- **验收**：`go test ./internal/topicgraph/repository -run TopicRecentBrief -short`（cmd）

### D-T2 buildClusterSystemPrompt 增强注入（RED→GREEN）
- 修改 `buildClusterSystemPrompt` 签名增加 `briefs map[uint][]TopicRecentItem` 参数（或封装结构体）。
- active 话题：注入行追加"近期内容"段（section 标题 + 代表 thread 标题）；candidate：维持现状（仅 label）。
- prompt 文案补指示："依据框架近期实际内容判断归属，而非仅凭标题字面沾边"。
- 测试（纯逻辑，内存 SQLite 或直接字符串断言）：active 含内容 / candidate label-only / 截断 / 空 briefs 降级四分支的 prompt 字符串。
- **验收**：`go test ./internal/topicgraph/service -run BuildClusterSystemPrompt -short`（cmd）

### D-T3 ClusterTags 接线 + 降级（RED→GREEN）
- orchestrator 在 `ClusterTags` 调用前查 briefs 传入；查询失败 `log.Warnf` 降级为空 briefs（= label-only），不阻断。
- 测试：briefs 查询失败时 ClusterTags 仍返回（降级路径）。
- **验收**：`go test ./internal/topicgraph/service -run ClusterTags -short`（cmd）

### D-T4 传导链守卫回归
- 重跑 `TestPlanTopicAssignments_AnchorHit_MatchedWithinThresholdNotNearest` + `TestTopicLineageSurvivesClusterDrift` 确认血缘未打散。
- **验收**：上述两测试 PASS（cmd）

### D-T5 质量门禁（D 切片）
- `cmd.exe /C "cd /d D:\project\Syntopica\backend-go && golangci-lint run ./internal/topicgraph/... && go vet ./internal/topicgraph/... && go test ./internal/topicgraph/... -short && go build ./..."`

---

## 切片 C′：话题专注视图（前端，独立）

> capability: `section-lifecycle`（ADD `话题专注视图模式`）。tasks.md §2。依赖现有 lanes 数据，无后端依赖。

### C-T1 viewMode focus + 进入/退出（RED→GREEN）
- `BoardThreadBrowser.vue`：`viewMode` 加 `'focus'`；`focusedTopicId` ref。
- lanes 视图泳道标签/背景挂 click → `enterFocus(topicId)`；focus 视图顶部「← 返回总览」→ 退回 lanes。
- 测试（Vitest）：点泳道进 focus、返回退回 lanes、viewMode 状态正确。

### C-T2 sticky 标题栏（GREEN）
- focus 视图顶部 sticky 栏：话题名 + 状态徽章 + 元信息（动态数/跨度/最近日期）。颜色全语义 token。
- 测试：sticky 渲染、双主题（可只断言 class/token，视觉人工验）。

### C-T3 横向时间轴 + 拖拽 + 就地展开（RED→GREEN）
- focus 主体：仅渲染 `focusedTopicId` 的 section 节点（复用 lanes 数据过滤 by persistent_topic_id），贯穿竖线主线 + 放大节点 + 日期列。
- 拖拽平移：复用 svgScrollRef scroll，pointer 事件区分 click/drag（位移 > 阈值吞 click）。
- 节点 click → 下方 accordion 就地展开 thread（复用 selectNode/section.threads），focus 模式 SHALL NOT 弹 popup overlay。
- 测试：节点过滤、click/drag 区分、就地展开不弹 popup。

### C-T4 空话题降级（GREEN）
- 话题最近无 section → 显示历史最后一条 + "最近无新动态"提示。测试：空态不报错。

### C-T5 质量门禁（C′ 切片）
- `cd front && pnpm lint`（WSL）
- `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck && pnpm test:unit"`
- 对照 `mockups/topic-focus-view.html` 浏览器视觉验收（cmd 起前后端）

---

## 切片 A：关注标记（前后端，最重）

> capability: `topic-watch`（ADD 全新）。tasks.md §3、§4。

### A-BE-T1 迁移：board_topic_watches + topic_watch_hits（GREEN）
- 版本化迁移 `20260630_0001`（postgres_migrations.go），两表 DDL，幂等。watches: status CHECK(active,paused)。
- 测试（testcontainer）：迁移幂等（反复执行无错）、CHECK 约束拒绝非法 status。
- **验收**：testcontainer 集成测试 PASS

### A-BE-T2 模型 + repository CRUD（RED→GREEN）
- `BoardTopicWatch` / `TopicWatchHit` 模型；CRUD：CreateWatch/ListWatchesByBoard/UpdateWatch/DeleteWatch(含命中级联)/ListActiveWatchesByBoard。
- 测试（内存 SQLite）：CRUD + 级联清理 + active 过滤。
- **验收**：`go test ./internal/topicgraph/repository -run Watch -short`（cmd）

### A-BE-T3 service 编排统一入口 + EvaluateWatchHits（RED→GREEN）
- 抽 `GenerateAndSaveReport(boardID, date)` = Generate→Save→EvaluateWatchHits（事务外，失败 log.Warnf 吞）。handler + scheduler 两入口改调它。
- `EvaluateWatchHits`：active 关注 × 当期 section 批量 AI 判定，写 topic_watch_hits。一次请求 JSON schema `{hits:[{watch_id,section_id,reason}]}`，合法 id 集防幻觉。
- 测试：零副作用（断言 section.persistent_topic_id 与 topic.consecutive_hits 不变，testcontainer）、paused 跳过、prompt 构造（内存 SQLite 纯逻辑）。
- **验收**：testcontainer 零副作用断言 PASS + `go test -run EvaluateWatchHits -short`

### A-BE-T4 handler + 路由（GREEN）
- `POST/GET /api/semantic-boards/:boardId/topic-watches`、`PATCH/DELETE /api/topic-watches/:id`、`GET /api/daily-reports/:id/watch-hits`。
- grep 路由注册二次确认（codegraph 追不到 group.POST）。
- **验收**：四端点 handler 测试 PASS

### A-BE-T5 质量门禁（A 后端切片）
- `cmd.exe /C "cd /d D:\project\Syntopica\backend-go && golangci-lint run ./internal/topicgraph/... && go vet ./internal/topicgraph/... && go test ./internal/topicgraph/... -short && go build ./..."`

### A-FE-T1 API 封装（GREEN）
- `app/api/topicWatches.ts`：createWatch/listWatches/updateWatch/deleteWatch/getWatchHits，经 ApiClient，snake→camel 在 normalizer。
- 测试：类型完整、normalize 正确。

### A-FE-T2 顶部关注栏组件（RED→GREEN）
- 新组件（日报 masthead 下、正文上）：按关注分组展示命中 section 标题 + 理由；空态；每组折叠。对照 `mockups/topic-watch-bar.html`。全语义 token、双主题。
- 测试：分组渲染、空态、与正文分区语义区分。

### A-FE-T3 关注管理（新建/暂停/删除）（GREEN）
- 复用 AppDialog/AppButton/AppInput，禁 window.*。测试：CRUD 可用、零原生弹窗（grep window.alert|prompt|confirm 零命中）。

### A-FE-T4 质量门禁（A 前端切片）
- `cd front && pnpm lint`
- `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck && pnpm test:unit && pnpm build"`
- 浏览器视觉验收对照 `mockups/topic-watch-bar.html`

---

## 派发顺序与并行策略

1. **D 先行**（后端、核心、传导链敏感，需主线程盯回归）——串行派发一个子线程做完 D-T1~T5。
2. **C′ 与 A 后端并行**（D 完成后，两者无依赖）：C′ 前端 + A-BE 两个子线程并行。
3. **A 前端**（依赖 A-BE 的 API 契约）：A-BE 完成后派发。
4. 每个子线程返回后，主线程**核验实际改动**（读 diff/跑门禁），不轻信汇报。
5. 全部完成后统一 review（ocr）+ 归档门禁（步骤4/5）。

## 边界声明（不可越）

- apply 阶段禁止改 proposal 需求范围（§8）。发现需求缺口走 delta，不就地扩。
- D 严禁碰 embedding AND-gate / 双重确认 / 生命周期算法。
- A 关注标记严禁并入 persistent_topic（独立实体、单信号、无生命周期）。
- C′ focus 模式严禁引入新数据源（复用 lanes 数据模型）。
