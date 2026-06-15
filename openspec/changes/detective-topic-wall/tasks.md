# Tasks — detective-topic-wall

> 实现顺序按依赖自底向上：依赖安装 → 基础设施 → 视觉对象 → 控制层 → Vue 集成。
> 每个任务可独立验证。动画分工遵循 design.md §Animation Library Split：3D 用 gsap，2D overlay 用 motion-v。

## 1. 依赖与脚手架

- [x] 1.1 安装依赖：`pnpm add gsap` 与 `pnpm add postprocessing`（three 已安装）+ `pnpm add -D @types/three`（three 0.183 不带 .d.ts）。删除残留的空 `front/three.d.ts`（`declare module 'three'` 会遮蔽 @types/three 导致所有 three 导入变 any）。
- [x] 1.2 创建目录骨架 `front/app/features/tags/components/detective-wall/` 及空文件占位（types.ts、TopicWallScene.ts、CardGroup.ts、RedString.ts、FogSystem.ts、DirectorCamera.ts、WallPostProcessing.ts、InteractionLayer.ts、ChapterTransition.ts）。注：不创建空占位文件（避免 lint/typecheck 噪音），目录随各实现任务自然形成。
- [x] 1.3 编写 `types.ts` + `utils.ts`：共享类型（`CameraShot`、`InteractionMode`、`InteractionState`、`PinCard`/`RedString` 接口、`LayoutResult`、`DateRange`、`STYLE`/`SUPPORTED_DAYS` 常量）+ 可测纯函数（`bfsLifeline`、`layoutCards`、`densityForDays`、`edgeKey`、`timelineWidth`、`latestDayX`）。

## 2. 场景基础设施（TopicWallScene）

- [x] 2.1 实现 `TopicWallScene.ts` 构造函数：WebGLRenderer（antialias）、PerspectiveCamera、Scene（背景 `#0a0f14`）、CSS2DRenderer（tooltip 用）、resize、渲染循环（requestAnimationFrame）、dispose（清 geometry/material/renderer）。
- [x] 2.2 实现 `loadBoardData()`：接收 sections/relations/dateRange/days，委托 CardGroup + RedString 构建；`clearScene()` 清空卡片与红线；暴露 readonly scene/camera/renderer/cardGroup/redStrings。

## 3. 视觉对象层

- [x] 3.1 实现 `CardGroup.ts` + `PinCardImpl`：BoxGeometry 纸卡片（`#FFFBEB`，roughness 0.85，emissive 高亮机制）+ 图钉（Cylinder+Sphere，`#DC2626`，metalness 0.7）+ SpriteText（话题标题/元信息）+ 状态色条。layoutCards 定位。`elevate/settle/highlight/dim/reset` 用 gsap。`staggerEntrance/staggerExit` 返回 gsap Timeline。
- [x] 3.2 实现 `RedString.ts`：Line2/LineMaterial（three examples）粗红线，距离衰减 opacity，`draw/highlight/dim/reset` + setResolution。
- [x] 3.3 实现 `FogSystem.ts`：FogExp2（`#0a0f14`），密度映射 7→0.08/14→0.05/30→0.03/60→0.02，`setDensityForDays/animateToDensity(gsap)/disable/enable`。
- [x] 3.4 实现 `lighting.ts`：AmbientLight(0.15) + SpotLight(45°, penumbra 0.5) + 跟随相机 PointLight（探险灯）+ 选中卡片红色 PointLight（返回引用供 per-frame 更新）。
- [x] 3.5 实现 `WallPostProcessing.ts`：pmndrs EffectComposer + RenderPass + Bloom(0.6) + Vignette(0.5)（EffectPass 包装）+ three/examples FilmPass(0.04)（无新增依赖）。

## 4. 控制层

- [x] 4.1 实现 `DirectorCamera.ts`：`CameraShot` 预设 `todayFocus/overview/topicFocus/lifecycleFull`。`transitionTo(shot)` 返回 gsap Timeline（kill 前一个，lookAt 跟随，fov 35~65，y≥2）。`snapTo` 无动画跳转。
- [x] 4.2 实现 `InteractionLayer.ts` 的 BFS 生命线算法 `bfsLifeline`（复用 utils.bfsLifeline：Map<number,Set<number>> 无向邻接表 + 日期窗口约束 + 规范化 minId-maxId edge key）。单测见 6.1。
- [x] 4.3 实现 `InteractionLayer.ts` 的 Raycaster：pointermove（rAF 节流）检测 PinCard 命中 → elevate + 邻居红线高亮；pointerdown/up 区分 click vs drag（<5px）。状态机 idle/focusing/lifecycle。
- [x] 4.4 实现 BFS 动画序列（gsap Timeline）：非相关卡片 dim stagger → 相机 topicFocus → BFS 节点逐个 highlight → 红线绘制。`setTimeRange`/`resetToOverview`。
- [x] 4.5 实现 `ChapterTransition.ts`：Phase1 红色 wipe（gsap xPercent）→ Phase2 档案封面 + 打字机（substring+onUpdate，无 TextPlugin）→ Phase3 onReload hook + 淡出。Vue 渲染 DOM，本类只动画。

## 5. Vue 集成层

- [x] 5.1 实现 `TopicDetectiveWall.client.vue`：全屏 `<canvas>` + WebGL 检测 + 初始化 TopicWallScene/DirectorCamera/InteractionLayer/ChapterTransition。接收 boardId props，调 `getBoardSectionTimeline` 加载。管理 loading/days/focusedNode/showDetailPanel 响应式状态。onBeforeUnmount dispose + ResizeObserver 清理。
- [x] 5.2 实现 2D overlays（集成在 TopicDetectiveWall.client.vue 内）：DaysRangeControl（7/14/30/60 触发 setTimeRange）、TopicDetailPanel（**普通 Vue `position:fixed` overlay + `<Transition>` 过渡**，非 CSS2DRenderer；案件档案风格：案件编号/生命线列表）、ChapterTransition DOM（wipe+cover，由 ChapterTransition.ts 动画）。CardTooltip 由 CSS2DRenderer 在 scene 层处理。
- [x] 5.3 修改 `BoardThreadBrowser.vue`：添加"侦探墙"按钮（WebGL + ≥768px 才显示），emit `openDetectiveWall`。接入 `BoardDailyReportTimeline.vue`（实际宿主，proposal 误写 TagsPage）渲染 `TopicDetectiveWall` 全屏视图。
- [x] 5.4 移动端门禁：`showDetectiveEntry` computed 检查 viewportWidth ≥ 768，不满足不渲染按钮。

## 6. 测试

> 前端单元测试（vitest），重点覆盖纯逻辑：BFS 算法、layout 算法、edge key 规范化。Three.js 对象渲染类（需要 WebGL 上下文）用 happy-dom 不便测，转为可测的纯函数抽取。

- [x] 6.1 `utils.test.ts` bfsLifeline：日期窗口过滤、孤立焦点、反向遍历 edge key、连通分量（4 测试）。
- [x] 6.2 `utils.test.ts` layoutCards：同日多话题垂直错开、Z jitter/rotation.z 范围、确定性（3 测试）。
- [x] 6.3 `utils.test.ts` densityForDays：7/14/30/60 → 对应 density + 最近桶回退（含 edgeKey/timelineWidth/latestDayX 共 6 测试）。

## 7. 文档

- [x] 7.1 更新 `docs/reference/architecture/frontend.md`：在 Feature 划分补 detective-wall 子目录条目 + 新增 §3D 侦探墙（子模块结构/动画库分工/BFS 语义/数据层）。
- [x] 7.2 gsap/postprocessing 用途与 3D/2D 动画库分工已写入 frontend.md §3D 侦探墙（configuration.md 是环境变量导向，不适合放依赖说明）。

## 8. 验证

> 归档前重跑本节全部命令，确认零失败（§11.4）。

- [x] 8.1 `cd front && pnpm lint → 0 error`（23 warning 全是预存 topic-graph 代码，detective-wall 零 warning）
- [x] 8.2 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck" → 0 error`
- [x] 8.3 `cd front && pnpm test:unit -- detective-wall → PASS`（detective-wall/utils.test.ts 13 测试全通过；topic-graph 的 5 个预存失败经 git stash 验证与本 change 无关）
- [x] 8.4 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build" → 构建成功（client+server+Nitro，✓ built in 6.69s）`
- [x] 8.5 `Select-String -Path openspec/changes/detective-topic-wall/**/*.md -Pattern 'unify-ui-components' → 0 命中`
- [x] 8.6 `Select-String -Path openspec/changes/detective-topic-wall/**/*.md -Pattern 'addPass(new BloomEffect' → 0 命中`
- [x] 8.7 `Select-String -Path openspec/changes/detective-topic-wall/**/*.md -Pattern 'list = adj.get' → 0 命中（仅 tasks.md 自身验证条目文字匹配，proposal/design/spec 零命中）`
