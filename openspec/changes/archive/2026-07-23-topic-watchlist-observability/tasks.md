# Tasks: 关注标记 + 话题专注视图 + 泳道上下文注入

> 垂直切片，每切片独立可交付、可验证。推荐顺序：D 泳道注入（核心、后端先行）→ C′ 专注视图（前端，独立）→ A 关注标记（前后端，最重）。尾部遵循开发执行规范 §11 归档门禁。

## 1. 泳道上下文注入（D · persistent-topic / daily-report-system）

- [x] 1.1 新增查询 `ListTopicRecentBriefs(boardID, sinceDays, perTopicLimit)`：按 active 话题拉最近 7 天 section brief（标题）+ 每条 section 代表性 thread 标题（按 fit_distance 升序，每 section 至多 2 条）。验收：返回 `map[topicID][]TopicRecentBrief`，按 last_seen/period_date 倒序截断
- [x] 1.2 token 截断保护：每话题最多 N 条 section（N=5），超则截断。验收：注入内容有上限
- [x] 1.3 `buildClusterSystemPrompt` 增强：active 话题注入"近期实际内容"（section + thread），candidate 维持 label-only；补指示"依据近期内容判断归属而非字面沾边"。验收：prompt 含 active 内容、candidate 仅 label
- [x] 1.4 注入查询失败非致命：拉取失败降级 label-only，不阻断 ClusterTags。验收：降级路径有效
- [x] 1.5 单测：注入 prompt 构造四分支（active 含内容 / candidate label-only / 截断 / 降级）。验收：7 个 repository 测试 + 5 个 service 测试 PASS

## 2. 话题专注视图（C′ · section-lifecycle，前端）

- [x] 2.1 `BoardThreadBrowser.vue` 新增 `viewMode='focus'` + `focusedTopicId`；lanes 点泳道进 focus、返回总览。验收：三模式可切换
- [x] 2.2 focus sticky 标题栏（话题名 + 状态 + 动态数/跨度/最近日期），全语义 token。验收：sticky 生效、双主题清晰
- [x] 2.3 横向时间轴（仅 focusedTopicId 节点，复用 lanes 数据过滤）+ 按住拖拽平移（区分 click/drag 阈值 3px）。验收：拖拽正常、click 不误触
- [x] 2.4 节点点击就地展开 thread（复用 selectNode 加载，focus 模式 SHALL NOT 弹 popup overlay）。验收：就地展开、无弹窗
- [x] 2.5 空话题降级（窗口内无节点 → 提示 + 返回，不报错）。验收：空态不崩
- [ ] 2.6 浏览器视觉验收（cmd 起前后端）：点泳道进 focus、sticky、拖拽、就地展开（对照 `mockups/topic-focus-view.html`）。验收：人工核验（待里程碑验收）
- [x] 2.7 前端单测：纯函数（filterFocusNodes/isDragMove/buildFocusMeta）11 个 + 组件 mount 8 个。验收：PASS

## 3. 关注标记后端（A · topic-watch）

- [x] 3.1 版本化迁移 `20260630_0001`：`board_topic_watches.status` CHECK(active,paused)，幂等。验收：testcontainer 反复执行无错
- [x] 3.2 版本化迁移 `20260630_0002`：`topic_watch_hits(watch_id, section_id, report_id)` 复合唯一索引（防重复命中行），幂等。验收：testcontainer 幂等 + 去重测试 PASS
- [x] 3.3 模型 `BoardTopicWatch`/`TopicWatchHit` + repository CRUD（CreateWatch/ListWatchesByBoard/UpdateWatch/DeleteWatch 含命中级联/ListActiveWatchesByBoard/GetWatchHitsByReport）。验收：8 个 SQLite 单测 PASS
- [x] 3.4 service 编排统一入口 `GenerateAndSaveReport` = Generate→Save→EvaluateWatchHits（事务外，失败 log.Warnf 吞）。两入口（handler+scheduler）改调它。验收：统一入口生效
- [x] 3.5 `EvaluateWatchHits`/`evaluateWatchHitsWithChat`：active 关注 × 当期 section 批量 AI 单信号命中判定，输出 JSON `{hits:[{watch_id,section_id,reason}]}`，合法 id 集防幻觉。插入 upsert（OnConflict DoNothing，防重复）。验收：单信号、防幻觉、去重
- [x] 3.6 handler + 路由（5 端点）：POST/GET `/semantic-boards/:boardId/topic-watches`、PATCH/DELETE `/topic-watches/:id`、GET `/daily-reports/:id/watch-hits`。grep 路由注册二次确认（codegraph 追不到 group.POST）。验收：7 个 handler 测试 PASS
- [x] 3.7 paused 关注跳过判定。验收：测试覆盖
- [x] 3.8 零副作用集成测试（testcontainer）：断言 `section.persistent_topic_id` 与 `topic.consecutive_hits` 命中后不变（2 个硬约束 Scenario）+ 防幻觉过滤。验收：3 个集成测试 PASS

## 4. 关注标记前端（A · topic-watch，依赖 3）

- [x] 4.1 API 封装 `app/api/topicWatches.ts`（5 方法，经 ApiClient，snake→camel normalizer，ID 转字符串）。验收：10 个测试 PASS
- [x] 4.2 日报顶部独立栏 `DailyReportWatchBar.vue`（masthead 下、正文上）：按关注分组展示命中 section 标题 + 理由；空态；每组折叠（阈值 2）。全语义 token。对照 `mockups/topic-watch-bar.html`。验收：11 个组件测试 PASS
- [x] 4.3 关注管理（新建/暂停/恢复/删除）复用 AppDialog/AppButton/AppInput，删除走 AppDialog 二次确认，禁 window.*。验收：测试断言无 window.* 调用
- [x] 4.4 顶部栏与正文 persistent_topic 分区语义区分（eyebrow「你在追踪·Watchlist」+ accent 竖条）。验收：测试断言标识存在
- [x] 4.5 纯函数（groupHitsByWatch/partitionByStatus/formatMoreLabel）单测。验收：9 个 PASS

## 5. 架构体检（§7 强制，每个子任务后）

- [x] 5.1 `codegraph impact`：`buildClusterSystemPrompt`/`EvaluateWatchHits`/viewMode focus 三处波及面无 HIGH/CRITICAL 忽略
- [x] 5.2 传导链守卫（coupling-map §1）：D 改聚类输入，重跑 `TestPlanTopicAssignments_AnchorHit_MatchedWithinThresholdNotNearest` + `TestTopicLineageSurvivesClusterDrift` 确认血缘未打散。验收：两测试 PASS
- [x] 5.3 新增 Gin handler grep 路由注册二次确认（codegraph 追不到 group.POST）。验收：5 端点路由均已注册
- [x] 5.4 分层合规：关注逻辑在 `internal/topicgraph/`、注入增强在 `service/`、不引入循环依赖

## 6. 数据兼容性（§10）

- [x] 6.1 迁移幂等：两表 + CHECK + 复合唯一索引在 testcontainer 反复执行无错
- [x] 6.2 D 注入对历史日报无副作用（注入是生成期行为，不回刷）
- [x] 6.3 JSON 响应向后兼容：关注 API 为全新端点，不破坏现有响应
- [x] 6.4 回滚路径：DROP TABLE / DROP INDEX 可逆，记录于迁移注释；D/C′ 可独立 revert

## 7. 文档（§12.4 里程碑收尾统一更新）

> 以下 reference 更新在**里程碑收尾时**统一做，不在本 change 内逐条改活文档；此处列清单备忘。触及 flow 的，archive 后按 §12.2 补「变更溯源」链接。

- [ ] 7.1 `docs/reference/flow/daily-report.md` §1 Step3：补"注入 active 话题最近 7 天 section/thread 内容（不止 label）"
- [ ] 7.2 `docs/reference/api/`：补关注标记五端点
- [ ] 7.3 `docs/reference/database/`：补 board_topic_watches / topic_watch_hits 表 + 复合唯一索引
- [ ] 7.4 `docs/reference/architecture/`：日报生成流程图补 EvaluateWatchHits 接入点 + ClusterTags 注入增强；说明关注标记与 persistent_topic 隔离边界

## 8. 测试（§11.2）

> 归档前重跑，确认零失败。后端命令须走 cmd.exe（Go 仅 Windows .exe）；前端 typecheck/build/test 须 cmd，lint 可 WSL。

- [ ] T.1 `cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go test ./internal/topicgraph/service ./internal/topicgraph/repository ./internal/topicgraph/handler -short"` → PASS
- [ ] T.2 testcontainer 集成（含 Docker）：迁移幂等 + EvaluateWatchHits 零副作用（3 个）+ 去重（1 个）+ 传导链守卫（`TestTopicLineageSurvivesClusterDrift`）→ PASS
- [ ] T.3 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit"` → 全过（含 topicFocus 11 + BoardThreadBrowser.focus 8 + topicWatches 10 + DailyReportWatchBar 11 + topicWatchGrouping 9）
- [ ] T.4 `grep -rnE "window.(alert|prompt|confirm)" front/app` → 零命中

## 9. 验证（§11.2，归档前实测）

- [ ] V.1 `cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go build ./..."` → BUILD_OK
- [ ] V.2 `cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go vet ./internal/topicgraph/..."` → VET_OK
- [ ] V.3 `cmd.exe /C "cd /d D:\project\Syntopica\backend-go && golangci-lint run ./internal/topicgraph/..."` → 0 issues（已知预存在 gofmt 2 处非本次引入）
- [ ] V.4 `cd front && pnpm lint` → 0 error（lint WSL 可跑，5 warnings 全 pre-existing）
- [ ] V.5 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"` → TYPECHECK_PASS
- [ ] V.6 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"` → BUILD_PASS
- [ ] V.7 `bash scripts/check-standards.sh` → A-D 段零失败（E 段归档后校验）
- [ ] V.8 浏览器视觉验收（cmd 起后端 + 前端）：① 点泳道进 focus、sticky、拖拽、就地展开（对照 topic-focus-view.html）② 日报顶部关注栏分组（对照 topic-watch-bar.html）③ 关注 CRUD 无原生弹窗 ④ D 注入后新生成日报归属质量人工抽查
