# Tasks: 关注标记 + 归属理由可视化 + 画布密度

> 垂直切片，每切片独立可交付、可验证。三块（C 画布密度 / B 理由可视化 / A 关注标记）可并行，B 前端依赖 C 的空间放宽与 B 后端的数据。尾部遵循开发执行规范 §11 归档门禁。

## 1. 画布密度（C · section-lifecycle）

- [ ] 1.1 `front/app/features/tags/components/BoardThreadBrowser.vue`：放宽 `COL_W`（148→约 200）、上调节点标签与正文文字字号至常规可读区间。验收：默认状态下节点标签清晰可读
- [ ] 1.2 同文件：默认 `zoomScale` 由 1 调至约 1.2-1.3，首屏即可读。验收：首次打开无需浏览器缩放
- [ ] 1.3 确认 `btb-zoom-bar`（0.4x-3x）在放宽后仍正常响应。验收：手动缩放行为不变
- [ ] 1.4 lanes 视图布局验证：节点标签变大后不挤压同泳道节点、不溢出泳道背景。验收：多话题 board 的 lanes 视图无重叠
- [ ] 1.5 浏览器视觉验收（cmd）：桌面 1280px 宽下默认可读、横向滚动正常。验收：人工核验

## 2. 归属理由持久化与 API 暴露（B 后端 · topic-assignment-reasoning / daily-report-system）

- [ ] 2.1 新增版本化迁移 `20260624_*`：`daily_report_sections` 加 `matched_topic_id BIGINT NULL`（无外键约束，兼容历史 NULL）。验收：迁移在 testcontainer 幂等执行
- [ ] 2.2 `daily_report_models.go`：`DailyReportSection.MatchedTopicID` 由 `gorm:"-"` 改为持久化 tag（`gorm:"column:matched_topic_id;index"`）。验收：AutoMigrate/迁移增列
- [ ] 2.3 `daily_report_assignment.go`：anchor_hit / auto_new 分支写入持久化 `matched_topic_id`（anchor_hit 写 LLM 指向的 id，auto_new 写 NULL）。验收：归属后列被正确写入
- [ ] 2.4 section / timeline / lifeline 接口的 section 表示暴露 `matched_topic_id`（JSON 字段）。验收：前端能取到三元组
- [ ] 2.5 历史行 matched_topic_id 保持 NULL，前端降级文案（"历史数据，无 AI 判断记录"）。验收：NULL 不报错

## 3. 归属理由画布可视化（B 前端 · topic-assignment-reasoning，依赖 1 + 2）

- [ ] 3.1 `BoardThreadBrowser.vue`：节点按 `topic_match_confidence` + `topic_match_distance` 分层样式（anchor_hit 实心 / 边界命中半实心 / auto_new 空心）。三种样式颜色 MUST 由主题语义 token（`--color-*`）派生，跟随 editorial/dark 双主题，不写死色值。验收：三种样式可区分 + 两主题下均清晰
- [ ] 3.2 边界命中比例可配置常量（默认 `match_threshold × 0.85`），集中定义便于调参。验收：常量单一来源
- [ ] 3.3 节点 hover 气泡：人话理由（“与『X』约 N% 相关，AI 也认同” / “相似接近但 AI 未确认，已开新候选”）+ 原始数值（distance/confidence/matched_topic_id）。气泡样式跟主题 token。验收：hover 显示翻译后理由 + 原始值
- [ ] 3.4 话题详情侧栏：点击话题展示 `getTopicLifeline` 聚合的历史 section + 各自信度。验收：详情列表每条含信度
- [ ] 3.5 用真实 board 的 section distance 分布直方图校准边界比例（0.85），避免边界区间过宽/过窄。验收：边界命中占比合理（不超 30%）

## 4. 关注标记后端（A · topic-watch）

- [ ] 4.1 版本化迁移：新建 `board_topic_watches`（id / semantic_board_id / label / status CHECK(active,paused) / created_at / updated_at）与 `topic_watch_hits`（id / watch_id / section_id / report_id / period_date / reason）。验收：迁移幂等
- [ ] 4.2 模型与 repository：`BoardTopicWatch` / `TopicWatchHit`，CRUD 方法（CreateWatch / ListWatchesByBoard / UpdateWatch / DeleteWatch 含命中级联清理 / ListActiveWatchesByBoard）。验收：CRUD 单测
- [ ] 4.3 `EvaluateWatchHits(boardID, report)`：日报生成流程末尾接入，对该 board 全部 active 关注 + 当期 section 批量提交 AI 判定命中，写 `topic_watch_hits`。SHALL NOT 改 section 归属或 persistent_topic 生命周期。SHALL 失败时记日志跳过、不阻断日报生成。验收：单信号命中、零副作用
- [ ] 4.4 AI 命中判定 prompt schema（一次请求：全部 section + 全部 active 关注 → 输出 watch_id×section_id 命中矩阵 + reason）。验收：批量单次请求
- [ ] 4.5 handler + 路由：`POST/GET /api/semantic-boards/:boardId/topic-watches`、`PATCH/DELETE /api/topic-watches/:id`、`GET /api/daily-reports/:id/watch-hits`（或并入日报详情）。验收：四端点可用
- [ ] 4.6 `paused` 关注跳过判定。验收：paused 不产生命中记录

## 5. 关注标记前端（A · topic-watch，依赖 4）

- [ ] 5.1 API 封装进 `app/api/`（新 `topicWatches.ts` 或并入 `dailyReports.ts`）：createWatch / listWatches / updateWatch / deleteWatch，经 `ApiClient`、snake_case→camelCase 在 normalizer 层。验收：类型完整、不直接 fetch
- [ ] 5.2 日报顶部独立栏组件（新）：按关注分组展示命中 section 标题 + 一句话理由；空态显示“今天无你关注的动态”或隐藏；每组限展条数 + 折叠。颜色跟主题 token。验收：分组渲染、空态正确、双主题清晰
- [ ] 5.3 关注管理入口（新建关注 / 暂停 / 删除），MUST 复用项目组件库（AppDialog/AppButton/AppInput，禁原生 button/input 样式类、禁 window.*）。验收：CRUD 可用、零原生弹窗
- [ ] 5.4 顶部栏与正文 persistent_topic 分区在 UI 上语义区分（关注命中 vs 话题归属）。验收：用户可区分两者

## 6. 架构体检（§7 强制，每个子任务后）

- [ ] 6.1 `codegraph impact <修改的符号>`：波及面无 HIGH/CRITICAL 被忽略；EvaluateWatchHits / matched_topic_id 持久化 / 画布分层三处重点核
- [ ] 6.2 `codegraph affected <改动文件>`：受影响测试范围符合预期
- [ ] 6.3 新增 Gin handler（关注四端点）grep 路由注册二次确认（codegraph 追不到 group.POST 注册，会误报“无调用者”）
- [ ] 6.4 分层合规：关注逻辑在 `internal/topicgraph/` 内、不引入循环依赖

## 7. 测试

- [ ] 7.1 后端纯单元（内存 SQLite，`glebarez/sqlite` mode=memory，参考 `feed_service_test.go`）：关注命中 prompt 构造、边界命中比例计算、matched_topic_id 写入分支、状态机纯逻辑
- [ ] 7.2 后端集成（testcontainer pgvector `testutil.SetupTestDB`）：迁移幂等、关注 CRUD、命中级联清理、EvaluateWatchHits 零副作用（断言 section.persistent_topic_id 与 topic.consecutive_hits 不变）、matched_topic_id 持久化与历史 NULL 兼容
- [ ] 7.3 后端 CORS：确认新 POST/PATCH/DELETE 端点 preflight 通过（cors.methods 已含 PATCH，§13 已修）
- [ ] 7.4 前端单测（Vitest + happy-dom）：节点分层样式映射、hover 理由人话翻译、顶部栏分组与空态、关注管理无 window.*

## 8. 数据兼容性（§10）

- [ ] 8.1 迁移幂等验证：matched_topic_id 列、两表在 testcontainer 反复执行无错
- [ ] 8.2 历史数据兼容：历史 section matched_topic_id 为 NULL，前端/API 不报错
- [ ] 8.3 JSON 响应向后兼容：新增字段为可空，不破坏现有 section/timeline/lifeline 响应格式
- [ ] 8.4 回滚路径：DROP COLUMN / DROP TABLE 可逆，记录于迁移注释

## 9. 文档（§12.4 里程碑收尾统一更新）

> 以下 reference 更新在**里程碑收尾时**统一做，不在本 change 内逐条改活文档；此处列清单备忘。

- [ ] 9.1 `docs/reference/api/`：补关注标记四端点、section 理由三元组字段（matched_topic_id / topic_match_distance / topic_match_confidence）
- [ ] 9.2 `docs/reference/database/`：补 board_topic_watches / topic_watch_hits 表、daily_report_sections.matched_topic_id 列
- [ ] 9.3 `docs/reference/architecture/`：日报生成流程图补 EvaluateWatchHits 接入点；说明关注标记与 persistent_topic 的隔离边界

## 10. 验证

> 归档前重跑本节，确认零失败。

- [ ] V.1 `cd backend-go && go build ./internal/topicgraph/... ./internal/platform/...` → BUILD_OK
- [ ] V.2 `cd backend-go && golangci-lint run ./internal/topicgraph/...` → 0 issues
- [ ] V.3 `cd backend-go && go vet ./internal/topicgraph/...` → VET_OK
- [ ] V.4 `cd backend-go && go test ./internal/topicgraph/repository ./internal/topicgraph/handler ./internal/topicgraph/service -count=1` → PASS（关注 CRUD / 命中零副作用 / matched_topic_id 持久化用例全过）
- [ ] V.5 testcontainer pgvector 集成测试（迁移幂等 + EvaluateWatchHits 零副作用）→ PASS
- [ ] V.6 `cd front && pnpm lint` → 0 error（lint 可在 WSL 跑）
- [ ] V.7 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"` → TYPECHECK_PASS
- [ ] V.8 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit"` → 全过（节点分层 / hover 翻译 / 顶部栏 / 关注管理无 window.*）
- [ ] V.8a `grep -rn "window.\(alert\|prompt\|confirm\)" front/app` → 零命中（§13 零原生弹窗不回退）
- [ ] V.9 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"` → BUILD_PASS
- [ ] V.10 浏览器视觉验收（cmd 起后端 + 前端）：① 话题画布默认可读无需放大 ② hover 节点出理由气泡 ③ 日报顶部关注栏分组渲染 ④ 关注 CRUD 可用且无原生弹窗
- [ ] V.11 真实 board 数据核验：边界命中占比 ≤ 30%（直方图校准后）；matched_topic_id 对新日报非空、对历史行 NULL
