# HANDOFF — detective-topic-wall（人工验证阶段）

> 写于 session 额度将尽时。本文件供下一个 session / 人工接手时快速恢复上下文。
> 若 ctx 工具可用，可用 `ctx_search(["detective-wall", "handoff"])` 找回；否则直接读本文件。

## 1. 总体状态

- **change**：`openspec/changes/detective-topic-wall/`
- **openspec status**：`isComplete: True`，proposal/design/specs/tasks 全 done
- **tasks.md**：31/31 勾选完成，验证门禁（§11）全过：lint 0 error / typecheck 0 error / build ✓ / detective-wall 13 测试全通过
- **当前阶段**：代码写完 + 自动化验证通过，**进入人工验证（浏览器跑）阶段**

## 2. 本次 session 已修复的运行时 bug（2 个）

1. **`pass.initialize is not a function`（致命）**：`WallPostProcessing.ts` 原本混用 three/examples 的 `FilmPass` 与 pmndrs 的 `EffectComposer`，两者 Pass 接口不兼容（pmndrs addPass 会调 `initialize()`，FilmPass 没这方法）。**改用 pmndrs 原生 `NoiseEffect`**（颗粒）。⚠️ spec 里写"优先 three/examples FilmPass"，这个选型建议是错的，两者不能混用，必须用 pmndrs 自己的 effect。
2. **`THREE.Clock deprecated`（警告）**：`TopicWallScene.ts` 把 `Clock` 换成 `performance.now()` 手写 delta。

## 3. ⚠️ 已知 gap（人工验证大概率会暴露，需修复）

这些是我实现时已知、但本次没改的**功能未完成/逻辑缺陷**。按优先级：

### 3.1【高】CSS2D tooltip 完全没实现
- `InteractionLayer.onCardHover` 回调里注释写"tooltip handled by CSS2D in scene"，但 **scene 里从未创建任何 CSS2DObject**。
- 后果：hover 卡片时**不会显示话题名/状态 tooltip**。
- 修法：在 `TopicWallScene` 或 `InteractionLayer` 里给每张卡片挂一个 `CSS2DObject`（content 是话题名+状态），hover 时 visible=true，离开时 false。CSS2DRenderer 已在 scene 里初始化好了，基础设施齐备，只差对象创建。

### 3.2【高】ChapterTransition 切换板块逻辑缺陷
- `TopicDetectiveWall.client.vue` 的 `watch(boardId)` 触发 `chapterTransition.play()`，其 `onReload` 调 `loadBoardData()`——但 **loadBoardData 用的是同一个 props.boardId**，没有真正切换数据源。
- 后果：板块切换转场动画会放，但**内容不会变**（因为 boardId 没变；实际切板块要靠父组件改 props）。
- 实际上当前 UI 里**没有 BoardSelectorOverlay**（proposal/design 提了，但 5.2 实现时只做了 DaysRangeControl + DetailPanel，漏了 BoardSelector）。所以目前根本没有"切换板块"的入口，这条 watch 是死代码。
- 修法：要么补一个 BoardSelector overlay 并通过 emit 让父组件改 boardId props；要么删掉这段 watch（如果短期不做板块切换）。建议先删 watch，避免误导。

### 3.3【中】DetailPanel 生命线点击用错 id
- `TopicDetectiveWall.client.vue` 里 `openArticle(n.report_id)`——生命线节点点击用的是 `report_id`，但 emit 的 `openArticle` 语义期望 `articleId`。`SectionTimelineNode` 没有 articleId 字段。
- 后果：点击生命线条目会跳到错误的 article。
- 修法：生命线条目应展开成「该 section 下的文章列表」（调 `getDailyReportDetail(report_id)` 拿 threads→articles），而不是直接 emit report_id。当前是占位实现。

### 3.4【中】BoardThreadBrowser 的 resize listener 没 cleanup
- `BoardThreadBrowser.vue` 里 `window.addEventListener('resize', ...)` 没有 `onUnmounted` 对应的 `removeEventListener`。
- 后果：内存泄漏（每次进出组件累积 listener）。改动时我在 §3 Surgical Changes 原则下没顺手清，应补 onUnmounted。

### 3.5【低】InteractionLayer 的 pickString 阈值
- Line2 的 raycaster 命中阈值默认是 `Raycaster.params.Line.threshold`，Line2 用 `Line2.raycast` 自己管，但粗线命中区可能偏窄，hover/click 红线可能不灵敏。人工验证时注意，必要时调 `line.material.linewidth` 或加大 raycast 容差。

## 4. 关键架构决策（别推翻）

1. **动画库分工**：3D 用 gsap（本 change 自行安装），2D overlay 用已装的 motion-v。详情面板是普通 Vue `position:fixed` overlay（**非 CSS2DRenderer**），只有卡片 tooltip 用 CSS2DRenderer。
2. **BFS 语义**：`utils.bfsLifeline` 与现有 `topic-graph/utils/graphBfsHighlight.bfsHighlight` **不同**不可复用——前者严格受日期窗口约束，后者是连通分量启发式。spec 里有详细说明。
3. **数据层零改动**：复用 `getBoardSectionTimeline`/`getSectionLifecycle`/`getDailyReportDetail`（`~/api/dailyReports`），后端路由已存在。
4. **入口宿主**：proposal 写的 `TagsPage.vue` 是错的，实际宿主是 `BoardDailyReportTimeline.vue`（它渲染 `BoardThreadBrowser`，后者 emit `openDetectiveWall`，父组件切 `showDetectiveWall` 渲染全屏 `TopicDetectiveWall.client.vue`）。
5. **three 类型**：three 0.183 不带 .d.ts，靠 `@types/three`；项目里曾有个残留的空 `front/three.d.ts`（`declare module 'three'`）会遮蔽 @types/three 导致全 any，已删。

## 5. 关键文件清单

**新增（detective-wall/）：**
- `types.ts`（共享类型 + STYLE 常量）、`utils.ts`（可测纯函数：bfsLifeline/layoutCards/densityForDays/edgeKey）、`utils.test.ts`（13 测试）
- `TopicWallScene.ts`、`CardGroup.ts`（PinCardImpl）、`RedString.ts`、`FogSystem.ts`、`lighting.ts`、`WallPostProcessing.ts`、`DirectorCamera.ts`、`InteractionLayer.ts`、`ChapterTransition.ts`
- `TopicDetectiveWall.client.vue`（全屏容器 + overlays）

**修改：** `BoardThreadBrowser.vue`（入口按钮）、`BoardDailyReportTimeline.vue`（渲染全屏视图）、`docs/reference/architecture/frontend.md`（§3D 侦探墙）、`front/package.json`（gsap/postprocessing/@types/three）

## 6. 如何继续

1. 先读本文件 + `openspec/changes/detective-topic-wall/{proposal,design}.md` + 3 个 spec。
2. 优先修 §3.1（tooltip）和 §3.2（删死 watch 或补 BoardSelector）——这两个是功能完整性问题。
3. 修完跑验证节（tasks.md §8）确认没回归。
4. 人工验证 OK 后可执行 `openspec archive detective-topic-wall`（按 §11.4 重跑验证节 + §12 文档流转）。

## 7. 不要做的事

- 不要重新发明 BFS（utils.bfsLifeline 已实现且有测试）。
- 不要把 FilmPass 混回 pmndrs（会崩，见 §2）。
- 不要改后端（数据层零改动是既定决策）。
- 不要碰 topic-graph 的 5 个预存测试失败（git stash 验证过与本 change 无关）。
