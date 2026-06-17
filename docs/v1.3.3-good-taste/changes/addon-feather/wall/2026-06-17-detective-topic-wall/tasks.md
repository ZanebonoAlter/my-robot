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

- [x] 6.1 `utils.test.ts` bfsLifeline：日期窗口过滤、孤立焦点、反向遍历 edge key、连通分量（4 测试）+ BFS depth 梯度/孤立焦点 depth=0（2 测试，评审修复追加）。共 15 测试。
- [x] 6.2 `utils.test.ts` layoutCards：同日多话题垂直错开、Z jitter/rotation.z 范围、确定性（3 测试）。
- [x] 6.3 `utils.test.ts` densityForDays：7/14/30/60 → 对应 density + 最近桶回退（含 edgeKey/timelineWidth/latestDayX 共 6 测试）。

## 7. 文档

- [x] 7.1 更新 `docs/reference/architecture/frontend.md`：在 Feature 划分补 detective-wall 子目录条目 + 新增 §3D 侦探墙（子模块结构/动画库分工/BFS 语义/数据层）。
- [x] 7.2 gsap/postprocessing 用途与 3D/2D 动画库分工已写入 frontend.md §3D 侦探墙（configuration.md 是环境变量导向，不适合放依赖说明）。

## 8. 验证

> 归档前重跑本节全部命令，确认零失败（§11.4）。

- [x] 8.1 `cd front && pnpm lint → 0 error`（warning 全是预存 topic-graph/articles，detective-wall 零 warning）
- [x] 8.2 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck" → 0 error`
- [x] 8.3 `cd front && pnpm test:unit -- detective-wall → PASS`（detective-wall/utils.test.ts 15 测试全通过：原 13 + depth 2；topic-graph 的 5 个预存失败与本 change 无关）
- [x] 8.4 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build" → 构建成功（✨ Build complete）`
- [x] 8.5 `Select-String -Path openspec/changes/detective-topic-wall/**/*.md -Pattern 'unify-ui-components' → 0 命中`
- [x] 8.6 `Select-String -Path openspec/changes/detective-topic-wall/**/*.md -Pattern 'addPass(new BloomEffect' → 0 命中`
- [x] 8.7 `Select-String -Path openspec/changes/detective-topic-wall/**/*.md -Pattern 'list = adj.get' → 0 命中（仅 tasks.md 自身验证条目文字匹配，proposal/design/spec 零命中）`

## 9. 评审修复（迭代二）

> 基于「侦探墙评审报告」两份评审 + 3 个 Explore agent 核实，修复所有成立的问题。
> 已剔除 3 条事实性错误（`params.Line2` 确实生效、ChapterTransition 死代码已删、"退出 3D 视图"实为弹窗叠加）。

- [x] 9.1 **光照接线**：`TopicWallScene` 保存 `followLight`/`selectionLight` 引用，渲染循环每帧 `followLight.position.copy(camera)`（探险灯跟随相机）；新增 `setSelectionLight(card)`，`InteractionLayer` 在 triggerLifeline/resetToOverview 接线。修复"跟随光创建后不动、选中灯从未启用"。
- [x] 9.2 **BFS depth 改造**：`LifelineResult` 加 `depth: Map<number,number>`，`bfsLifeline` 维护 hop 计数；`playLifelineAnimation` 按 depth 计算 stagger delay（`0.1 + depth*0.08`），对齐 spec §BFS 动画序列。新增 2 个 depth 测试（梯度链 + 孤立焦点）。修复"动画按数组索引 stagger、顺序随机"。
- [x] 9.3 **OrbitControls 相机拖拽**：新增 `WallCameraControls.ts`，仅 pan+zoom（`enableRotate:false`，`mouseButtons.LEFT=PAN`），`minDistance 3 / maxDistance 40`。`DirectorCamera` 加 `hooks`（onTransitionStart→orbit 禁用 / onTargetUpdate→同步 target / onTransitionComplete→重启用），解决"主动 GSAP 动画 vs 被动 orbit"的 target/position 之争。`TopicWallScene` 加 `frameCallbacks`/`addFrameCallback`，渲染循环调 `controls.update()`。orbit start/end 事件 `setHoverSuspended`（拖拽时暂停 hover）。确认 `@types/three` 已安装（jsm 类型就绪）。
- [x] 9.4 **完整生命周期模式**：`InteractionLayer` 加 `enterLifecycle`（loadBoardData + fog.disable + lifecycleFull 镜头）/`exitLifecycle`（fog.enable）。修隐藏 bug：lifecycle 模式背景点击不再误触发 resetToOverview（改为 onBackgroundClick 交 Vue）；lifecycle 模式卡片点击不触发 BFS。Vue 加「查看完整生命周期」按钮 + `enterLifecycle`/`exitLifecycle` 编排（fetch getSectionLifecycle → 派生 dateRange → 场景重建）。
- [x] 9.5 **接线类小修**：(a) tooltip `pointer-events:auto` + `data-card-id`，InteractionLayer 监听 css2d click 转发卡片点击（解决"点文字落空"），innerHTML 改 textContent 防注入；(b) 卡片标题超 14 字符加 `…`；(c) status 中文化（emerging→新兴 等）；(d) 键盘 ESC（lifecycle→exit / focusing→closePanel / idle→close）；(e) 详情面板补汇总（总文章/线索数 + 状态分布彩点）、生命线节点改滚动容器（去 slice(0,10) 硬限制）、拆分「关闭面板」/「返回时间线」/「返回」语义。
- [x] 9.6 **线索→文章二级展开**：线索点击不再默认取首篇 `related_article_ids[0]`，改为 `toggleThreadArticles` 展开该线索的文章列表（批量 getArticle 取标题，最多 10 篇，超出显示"还有 N 篇"），点击具体文章才 `openArticle`。对齐 BoardThreadBrowser 的 2D 下钻模式。
- [x] 9.7 **卡片高亮光晕**：`highlight()` 的 `emissiveIntensity` 0.5 → 0.2，修复"光晕过曝盖住卡片表面文字"。

## 10. 文档同步（评审修复回写）

- [x] 10.1 interaction spec：bfsLifeline 返回值加 depth、BFS 动画序列按 depth、点击语义加 lifecycle 分支、面板操作改二级展开、API 加 enterLifecycle/exitLifecycle/setHoverSuspended + tooltip 转发/ESC 说明、Constraints 加 lifecycle guard/depth/ChapterTransition 状态。
- [x] 10.2 scene spec：TopicWallScene API 加 followLight/selectionLight/setSelectionLight/addFrameCallback/css2d/fog、PinCard 加 tooltip、highlight emissive 0.2、光照区加跟随灯/选中灯每帧更新。
- [x] 10.3 camera spec：DirectorCamera 加 hooks、新增 Wall Camera Controls 整节（配置/协调/hover 暂停/渲染循环/依赖）、Chapter Transition 标注当前无入口状态。
- [x] 10.4 proposal：What Changes 加 OrbitControls 拖拽 + BFS depth、camera capability 加 OrbitControls、Impact 文件清单加 WallCameraControls.ts/lighting.ts。
- [x] 10.5 tasks.md：§6.1 单测数 13→15、§8 验证节更新、追加 §9 评审修复任务 + §10 文档同步。

## 11. 评审修复（迭代三：氛围与导航）

- [x] 11.1 **台灯光锥修正**：降低半球/跟随环境光占比，提高台灯 SpotLight 强度；主光目标改为照片墙中心；实际光源与 DustParticles 锥顶移到灯罩下沿并向墙面探出，避免被灯罩遮挡；桌面后沿补暗边，拉开书桌与远背景层次。
- [x] 11.2 **右侧案件抽屉**：详情面板改为右侧全高抽屉，生命线/线索/文章标题改为可换行，避免 overflow 截断；案件编号区新增上一个/下一个快捷导航。
- [x] 11.3 **分化节点导航与摄像头聚焦**：新增 `InteractionLayer.focusNode(nodeId)`，抽屉导航选中节点后同步选中灯与 `DirectorCamera.topicFocus`；“下一个”若存在多个同时间分化候选，显示可选择的案件编号。
- [x] 11.4 **版块切换与生命周期回退**：顶部新增 `主墙 / 生命线 / 生命周期` 分段切换；完整生命周期进入后保持右侧抽屉可见，并提供返回主墙入口。

## 12. 测试

- [x] 12.1 前端 lint：`cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm lint"` → exit 0（仍有 23 个预存 warning，0 errors）。
- [x] 12.2 前端类型检查：`cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"` → exit 0。
- [x] 12.3 前端单元测试：`cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit"` → exit 0（17 files / 90 tests passed；happy-dom teardown 有预存 AbortError stderr，但测试通过）。
- [x] 12.4 前端构建：`cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"` → exit 0（构建成功；存在预存 CSS nesting warning 与 Nuxt 依赖 deprecation warning）。

## 13. 文档

- [x] 13.1 更新 `docs/reference/architecture/frontend.md`：记录右侧案件抽屉、分段切换、生命周期回主墙、分化候选导航，以及台灯出光口/照片墙中心照明语义。

## 14. 验证

> 归档前重跑本节全部命令，确认零失败（§11.4）。

- [x] 14.1 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm lint"` → exit 0（0 errors）。
- [x] 14.2 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"` → exit 0。
- [x] 14.3 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit"` → exit 0（17 files / 90 tests passed）。
- [x] 14.4 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"` → exit 0。

## 15. 评审修复（迭代四：主题与材质）

- [x] 15.1 **抽屉主题 token 化**：右侧案件抽屉改用 `--color-dialog-bg`、`--color-text-*`、`--color-border-*`、`--color-accent*`、`--shadow-*`，避免硬编码米黄色/深色按钮破坏 editorial/dark 主题适配。
- [x] 15.2 **书桌/背景层次**：调整 `STYLE` 的桌面、远景墙、软木墙、全局背景色，桌面偏暖木色、远景墙偏冷灰绿低明度，避免读成两个黑色集合体。
- [x] 15.3 **灰尘光锥收敛**：`DustParticles` 改为接收 origin + target，沿“灯罩出光口 → 照片墙中心”的真实光束采样；最终将 `STYLE.dust.count` 设为 0，先移除右下角割裂的独立尘埃团。
- [x] 15.4 **台灯重新收束**：台灯灯罩改回不透明实体材质，只让实际 SpotLight 从灯罩下沿外侧发出，避免“透出灯泡”的廉价透明感。
- [x] 15.5 **真实材质贴图**：下载并压缩 Poly Haven CC0 纹理到 `front/public/textures/detective-wall/`，桌面使用 `oak_veneer_01`，主墙使用 `beige_wall_001` plaster；`SetDressing` 与 `TopicWallScene.rebuildWall()` 通过 `TextureLoader` 加载本地 1K JPEG 贴图，删除 4K 原始大图。

## 16. 测试

- [x] 16.1 前端 lint：`cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm lint"` → exit 0（0 errors，预存 23 warnings）。
- [x] 16.2 前端类型检查：`cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"` → exit 0。

## 17. 文档

- [x] 17.1 更新 `docs/reference/architecture/frontend.md`：补充抽屉 token 规范、半透明灯罩、书桌/远景墙材质区分、真实光锥尘埃采样。
- [x] 17.2 新增 `front/public/textures/detective-wall/README.md`：记录贴图来源与 CC0 授权。

## 18. 验证

> 归档前重跑本节全部命令，确认零失败（§11.4）。

- [x] 18.1 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm lint"` → exit 0（0 errors）。
- [x] 18.2 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"` → exit 0。

## 19. 评审修复（迭代五：布景边界与立体感）

- [x] 19.1 **移除黑色几何体**：删除 `SetDressing` 中远景暗墙、黑色桌面后沿与额外桌面前沿，避免与主墙/桌面重叠形成穿帮块；主墙盒体侧边改为独立暖灰边缘材质，不注入方向雾，避免俯视角出现黑色横条。
- [x] 19.2 **墙面与桌面实体化**：主墙由 `PlaneGeometry` 改为超宽超高 `BoxGeometry` 实体墙板；桌面由水平平面改为超宽实体桌板，并增加暖棕前沿表达厚度。
- [x] 19.3 **扩大布景覆盖**：桌面中心改为 `minX + timelineWidth / 2`，宽度/深度按墙面安全范围外扩；主墙尺寸增加到足以覆盖相机活动区，并强制向下延伸到桌面后方以下，避免卡片墙与桌面之间露出场景背景。
- [x] 19.4 **相机边界限制**：`TopicWallScene.getCameraBounds()` 暴露墙内安全范围，`WallCameraControls.setBounds()` 在 pan/zoom 与运镜 target 更新时限制目标点，并收紧最大缩放距离，避免看到墙面/桌面边缘。

## 20. 测试

- [x] 20.1 前端 lint：`cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm lint"` → exit 0（0 errors，预存 warnings）。
- [x] 20.2 前端类型检查：`cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"` → exit 0。
- [x] 20.3 前端单元测试：`cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit"` → exit 0。
- [x] 20.4 前端构建：`cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"` → exit 0。

## 21. 文档

- [x] 21.1 更新 `docs/reference/architecture/frontend.md`：记录实体墙板/桌板、移除远景黑墙、相机安全边界与材质来源。

## 22. 验证

> 归档前重跑本节全部命令，确认零失败（§11.4）。

- [x] 22.1 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm lint"` → exit 0（0 errors）。
- [x] 22.2 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"` → exit 0。
- [x] 22.3 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit"` → exit 0。
- [x] 22.4 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"` → exit 0。
- [x] 22.5 `cmd.exe /C "cd /d D:\project\Syntopica && git diff --check"` → exit 0（仅 CRLF warning）。
- [x] 22.6 `openspec instructions apply --change detective-topic-wall --json` → all_done。

## 23. 待排查（迭代六：黑色横条残留）

> 2026-06-16 用户截图确认：即使删除远景暗墙、额外桌面前沿，并扩大主墙/桌面覆盖范围后，画面中仍存在大面积黑色横条。先记录，次日继续排查。

- [ ] 23.1 **定位黑色横条真实来源**：不要继续凭截图猜测；需要在本地浏览器/Three.js inspector 或临时 debug material 中逐个确认是 scene background、wall mesh、desk mesh、postprocessing vignette/film、directional fog，还是 CSS overlay。
- [ ] 23.2 **加临时材质诊断开关**：将墙、桌、背景、后处理分别换成高对比纯色（例如墙红/桌绿/background 蓝），截图确认黑条归属后再做正式修复。
- [ ] 23.3 **复查相机裁切与坐标**：确认 `DirectorCamera.todayFocus/lifecycleFull`、`WallCameraControls` pan/zoom、`wallBottom` 与 `desk.zBack/zFront` 在实际数据布局下没有让相机看到未覆盖区域。
- [ ] 23.4 **复查后处理**：临时禁用 Vignette/FilmPass/方向雾，确认黑条不是后处理或雾 shader 按 world position 造成的分层。

## 24. 测试

- [ ] 24.1 前端类型检查：`cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"`。
- [ ] 24.2 前端 lint：`cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm lint"`。
- [ ] 24.3 前端单元测试：`cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit"`。
- [ ] 24.4 前端构建：`cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"`。

## 25. 文档

- [x] 25.1 在本任务清单记录黑色横条仍未解决、已尝试方向与下一步诊断计划。
- [ ] 25.2 若最终修复涉及场景结构/材质/后处理，更新 `docs/reference/architecture/frontend.md` 对应 3D 侦探墙说明。

## 26. 验证

> 归档前重跑本节全部命令，确认零失败（§11.4）。

- [ ] 26.1 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm lint"`。
- [ ] 26.2 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"`。
- [ ] 26.3 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit"`。
- [ ] 26.4 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"`。
- [ ] 26.5 `cmd.exe /C "cd /d D:\project\Syntopica && git diff --check"`。
- [ ] 26.6 `openspec instructions apply --change detective-topic-wall --json`。
