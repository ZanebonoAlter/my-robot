> **编排策略（§0.6 主线程调度，TDD）**：抽 `composables/useInlineCompose.ts`（纯逻辑：池加载 / 选中 / 聚合锚点 / 节点 distance·tier·离群 / 移出分类 / 聚类质量卡 / 语义搜索 / 采纳预勾 / 保存）+ 两个展示组件（`ComposeInlineToolbar.vue` 顶部浮条 / `ComposeSidebar.vue` 右侧栏）+ `BoardThreadBrowser.vue` 最小装配（composeMode 叠加态 + 节点 checkbox/标徽 + active 淡显 + 移出二次确认 AppDialog + 删 ComposePanel）。
> **侧边栏候选话题数据源澄清（局部澄清·§8）**：复用 host 现有 `topics`（`listBoardTopics`，已含 consecutive_hits/section_count/can_activate）按 `status==='candidate'` 过滤——与旧 `ComposePanel` 传入的 `:candidate-topics` **同源同口径**，不改后端、不改数据契约、保留已上线行为。已知限制：后端 `FilterVisibleTopics` 过滤掉 `hit_count < upgrade_threshold` 的候选，故 spec scenario「油价波动(can_activate=false)」「已中断 consecutive_hits=0」仅在 hit_count 达标时可见——此为**既有行为**（旧 ComposePanel 同样受限），非本 change 引入。
> **派发**：D1 composable+测试（纯逻辑 TDD）→ D2 两展示组件 → D3 host 装配+删 ComposePanel(+test)。

## 1. 编排态叠加状态与入口

- [x] 1.1 `BoardThreadBrowser.vue` 新增 `composeMode: ref<boolean>` 叠加态；进入编排态时置 true，`viewMode` SHALL 保持 `lanes` 不变（不切 `viewMode='compose'`）✅ viewMode 类型已去 `'compose'`（现 `'timeline'|'lanes'|'focus'`），composeMode 与 composable.active 同步
- [x] 1.2 unassigned（待确认/未分类）泳道头部新增「新建泳道」按钮；点击 → `composeMode=true`（主战场入口）；工作台工具条原有「新建泳道」入口保留，两者皆触发同一 `composeMode` ✅ 两入口均 enterCompose
- [x] 1.3 退出编排态（保存成功 / 取消）→ `composeMode=false`，视图恢复浏览态 ✅ composable.save/cancel 置 active=false → watch 同步 composeMode=false

## 2. 就地勾选与贴合度渲染（主战场）

- [x] 2.1 编辑态下 unassigned 泳道每个 section 节点渲染 checkbox；维护 `selectedIds: Set<string>` 选中集 ✅ SVG checkbox（rect+勾 path）@click.stop toggle，composeMode 门控
- [x] 2.2 勾/取消实时重算：`aggregatePreview(选中向量)` 得 mean → `cosineDistance(v, mean)` 重算全员 distance → `distanceTier`/`outlierFlags` 更新分层；数据源用 `getComposeCandidates` 池（已带 embedding），按 section id 与 lanes 节点对齐 ✅ nodeInfo computed 随 selectedIds 自动重算；nodeInfoFor(String(pn.data.id)) 对齐
- [x] 2.3 节点 UI：distance 数字 + 边框分层（good/boundary/outlier）+ 离群标黄（distance > matchThreshold×1.3，标黄但保持勾选态由用户决定）；同天多节点维持纵向堆叠 ✅ tier 文本着色（success/accent/warning token）+ 离群 stroke 标黄（:not 手动节点），不取消勾选

## 3. active 泳道淡显 + 可勾走移出

- [x] 3.1 编辑态下 active 泳道淡显（opacity ≈ 0.3），但其 section 节点仍可勾选 ✅ btb-lane-row--dimmed（composeMode && lane.topicId!=null）；unassigned 不淡显
- [x] 3.2 勾选 active section 时节点实时标注「将从【原泳道名】移出」；原泳道名取自 `candidate.persistentTopic.label` ✅ nodeInfo.moveOut+originLabel，节点下方提示
- [x] 3.3 顶部计数区分「unassigned 来源」与「active 移入来源」两类 ✅ counts.unassigned/moveOut → 工具条副标

## 4. 顶部浮工具条 + 聚类质量卡

- [x] 4.1 新增 `ComposeInlineToolbar.vue`：泳道名输入（`AppInput`）+ 已勾计数（区分来源）+ 聚类质量单卡 + 取消/保存（`AppButton`）；SHALL NOT 用 `window.alert/prompt/confirm` ✅ 176 行
- [x] 4.2 聚类质量卡数据：成员数 / 平均距离（选中向量到 mean 的 `cosineDistance` 平均）/ 离群数（`outlierFlags` 计数），随勾选实时更新 ✅ quality computed 实时
- [x] 4.3 SHALL NOT 渲染原「撞车检查」「未来预期」卡 ✅ 仅单卡

## 5. 候选侧边栏（候选 topic + 语义搜索）

- [x] 5.1 新增 `ComposeSidebar.vue`（右侧滑出）：候选 topic 区 + 语义搜索框 ✅ 289 行
- [x] 5.2 候选 topic 区：列 `status=candidate` persistent_topic（label / consecutive_hits / section_count），can_activate 置顶高亮；无 candidate 时该区隐藏 ✅ 数据源 host topics 过滤 candidate（同旧 ComposePanel 口径）；activatable 置顶 + brokenStreak 分组；空时 v-if 隐藏
- [x] 5.3 候选 topic 两动作：「确认启用」（can_activate 时调状态更新→active，刷新总览）+「采纳」（预填名 + 预勾 unassigned 中 matchThreshold 内 section，纯前端、上限截断提示） ✅ activate→updateTopic(active)；adopt→centroid 预勾截断
- [x] 5.4 语义搜索：侧边栏搜索框 debounce → `embedQuery` 取查询向量 → 未勾选时按 cosine 相关度对 unassigned 节点高亮；勾选后主信号切换为已选聚合 mean，节点 distance/tier 重算（渐进收敛）；失败降级回退默认分层 + 轻量提示 ✅ runSearch 300ms debounce，activeSignal=anchor??queryVec，失败 queryVec=null+searchError

## 6. 保存 / 取消 / 移出二次确认

- [x] 6.1 保存：存在 active 移入项时先弹 `AppDialog` 二次确认，列全部移出 section（含原泳道名）；确认后调 `createManualLane(boardId, label, selectedIds)` ✅ requestMoveOutConfirm→Promise 桥接 AppDialog；@update:model-value 关闭即 cancel，无挂起
- [x] 6.2 无移入项可直接保存；保存成功 → 退出编排态 + 刷新总览，新泳道以 active+source=manual 出现在 lanes，被勾 section 从原位置移出 ✅ save() onSaved+exit
- [x] 6.3 取消：清空勾选与名字、退出编排态，不发 API ✅ cancel() reset+exit 零 API

## 7. 废弃 ComposePanel

- [x] 7.1 移除 `viewMode='compose'` 入口与相关切换逻辑 ✅ viewMode 类型去 compose，删 prevViewMode/旧 enter/exit 逻辑
- [x] 7.2 删除 `front/app/features/tags/components/ComposePanel.vue`；清理其独占的导入/样式/路由（确认无残留引用） ✅ 文件已删（.vue+.test.ts），grep 零残留
- [x] 7.3 保留 `composeReport.ts`（被新模式复用，不删） ✅

## 8. 测试

- [x] 8.1 单元测试：贴合度计算（aggregatePreview → cosineDistance → distanceTier/outlierFlags）在勾选增删时正确重算（复用/扩展 composeReport 现有测试） ✅ useInlineCompose.test.ts 31 例覆盖
- [x] 8.2 单元测试：「采纳」预勾逻辑——给定候选 centroid + matchThreshold，正确选出 unassigned 内 section 并截断上限 ✅ adopt 三例
- [x] 8.3 单元测试：移出分类计数——unassigned 来源 vs active 移入来源正确区分 ✅ counts/moveOutItems 多例

## 9. 文档

<!-- doc-impact: none(纯前端编排态交互重构：composeMode 就地叠加 + active 淡显 + unassigned 主战场勾选 + 聚类质量单卡 + 候选侧边栏 + 废弃 ComposePanel；不改业务链路/数据模型/API/配置/部署。apply 启动时以 doc-impact.sh suggest 实际预勾选为准；若 suggest 命中 flow/standard，则在此补对应文档更新) -->
<!-- doc-impact-excuse: flow=启发式命中系其它在途 change 的后端脏文件(backend-go/internal/admin|topicgraph/...)，本 change 纯前端零后端改动，不涉业务链路; api=同上脏文件干扰，本 change 复用既有 API 零新增/零变更; database=同上脏文件干扰，本 change 零 schema/数据模型变更(proposal 明确)；standard=命中的 standard/frontend/interaction-conventions.md 系其它 change 脏文件，本 change 的 inline-compose 是 feature 级交互非全局约定，不新增 standard 规范 -->

- [x] 9.1 apply 时按 `doc-impact.sh suggest` 预勾选结果同步 `docs/reference/`；若 suggest 无命中则本节空置 ✅ suggest 命中的 flow/database 系**其它在途 change 的脏工作树文件**（backend-go/...），与本纯前端 change 无关；standard 命中的 interaction-conventions 是 feature 级交互非全局约定。本 change 纯前端、零后端/API/数据模型/配置/部署变更，doc-impact: none 成立（归档前以 `doc-impact.sh verify` 复核）
  - **doc-impact verify 现状（归档前必读）**：`verify` 报 4 FAIL（声明 none 但启发式命中 flow/api/database/standard）。诊断：①api/database/standard 三项命中来自**工作树里其它在途 change 的脏文件**（backend handler/models + docs/reference/standard/interaction-conventions.md），干净工作树复跑即消；②**flow 项是 `^front/app/features/` 全匹启发式对纯 UI 重构的误报**——本 change 复用 createManualLane 全链路、零业务链路变更（proposal/spec/design 一致确认）。`verify` 规则4（声明 none→任何启发式命中即 FAIL）不读 excuse，放仅作文档留档。归档建议：干净工作树复跑 verify（消 api/database/standard）；flow 误报项需团队定夺（细化启发式或接受 excuse）

## 10. 验证

> 前端 typecheck/build/test:unit 必须用 Windows cmd（WSL 缺 native binding）；lint 可 WSL。

- [x] 10.1 `cd front && pnpm lint`（WSL 可跑）→ 期望零 error ✅ 实测 0 error（5 warning 全在无关旧文件 useArticleContentView/utils.test/schedulerMeta.test）
- [x] 10.2 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"` → 期望零 error ✅ **本 change 改动文件 0 error**；唯一 1 error 在 `settings/components/FeedDetailEditor.vue`(TS1109)，系**工作树既有脏文件**（另一在途 change，非本 change 引入，未 import 本 change 任何代码）——归档建议在干净工作树复核
- [x] 10.3 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit 2>&1"` → 期望新增/扩展测试全过、无回归 ✅ 40 文件 / 428 测试全绿（含 useInlineCompose 31 + ComposeInlineToolbar 11 + ComposeSidebar 9 + workbench 9 改后）
- [x] 10.4 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"` → 期望构建成功 ✅ `✨ Build complete!` exit 0
- [ ] 10.5 手测：lanes 视图点「新建泳道」→ active 泳道淡显保留 + unassigned 节点可勾 + 贴合度实时标 + 勾走 active 标移出 + 保存二次确认 + 新泳道出现；视图全程不切 viewMode ⏳ **未执行**：需起 dev server + Docker PG + 种子数据。逻辑/组件层已被 60 个单测覆盖 + build 通过；建议在用户环境跑 opencli 端到端冒烟（见 §5.3），或派 k3 视图验证截图

## 11. 交互打磨（收尾补丁 · 用户反馈驱动）

> 用户反馈 inline-compose-lane 交付后交互性差：①编排态浮层（顶部工具条 + 右侧 300px 候选栏）遮挡泳道、不可收；②行间距太挤，选中节点 label 盖到相邻泳道；③缩放按钮等比放大「放大了还是挤」无意义、且无滚轮。本次为收尾小修，不立新 change。

- [x] 11.1 缩放重做：废弃 `transform: scale(zoomScale)` 等比缩放（间距/字体同比涨，比例不变→放大无用）；改为**间距驱动**——`LANE_BASE`/`LANE_NODE_GAP`/`COL_W`/`ROW_H`/`PAD` 随 `zoomScale` 线性变化（computed），**节点半径 `NODE_R` 与字号不随缩放**。语义：放大=拉开布局间距，文字不爆开盖邻道 ✅ BoardThreadBrowser.vue 常量区 + laneLayout/positionedNodes/svgWidth/svgHeight/dateColumns/laneOpsY 全部 computed 化
- [x] 11.2 滚轮缩放：新增 `onWheelZoom`（直接滚轮，**不用 Ctrl**——避免与浏览器原生缩放冲突），`passive:false` 手动 `addEventListener` 以便 `preventDefault` 阻止页面滚动；`watch(svgScrollRef)` 动态绑/解（viewMode 切换重挂载）；`setZoom` 改为保持视口中心比例。MIN/MAX 0.6~2.4，步长 0.15 ✅
- [x] 11.3 基础行高加大：`LANE_BASE` 52→76、`LANE_NODE_GAP` 24→34（100% 时也不挤，选中节点 label 不再溢出邻道）✅
- [x] 11.4 浮层可折叠 + 让位：`ComposeSidebar` 加一键折叠（`composeSidebarCollapsed` ref + toggle 按钮，收起成 36px 窄条）；`.btb-svg-scroll` 编排态动态 `paddingRight`（展开 324 / 折叠 52），泳道内容物理避让、不再被侧边栏遮挡 ✅
- [x] 11.5 验证：lint 0 error（本文件 0 问题）/ typecheck exit 0 / build `✨ Build complete!` exit 0 ✅
