## Why

标签管理页的话题总览（`BoardThreadBrowser`）当前是 2D SVG 时间线 DAG，信息密度高但缺乏叙事感。用户面对大量节点时难以聚焦于单条线索的演化。Syntopica 的核心价值不是"一网打尽"，而是"抽丝剥茧"——用户需要从今天的热点出发，沿着因果红线追踪一个话题的来龙去脉。

3D 侦探照片墙将话题总览升级为沉浸式信息调查体验：卡片钉在软木墙上，红线连接相关话题，迷雾遮蔽时间窗口之外的信息，相机默认聚焦今天。交互模型从"看所有话题"转变为"追一根线索到底"。

## What Changes

- **新建 `TopicDetectiveWall` 全屏 3D 可视化**：Three.js 场景，卡片钉在墙上 + 红线连接 + 迷雾系统 + 导演相机，从 `BoardThreadBrowser` 旁的"进入侦探墙"按钮进入
- **卡片系统**：话题节点以旧纸卡片形态呈现（BoxGeometry + 纸纹理），顶部红色图钉，微倾斜角度，状态色条
- **红线连接**：风格化直线（不做物理悬垂），`CatmullRomCurve3` → `Line2`，发光材质，选中时脉动动画
- **迷雾系统**：`FogExp2` 遮蔽时间窗口外的区域，切换时间范围时迷雾推进/后退（GSAP 动画）
- **导演相机**：GSAP Timeline 驱动的运镜系统，默认聚焦今天，支持总览/聚焦/飞越三种镜头
- **板块切换转场**：红色 wipe 扫过 → 档案封面浮现（打字机字体、CONFIDENTIAL 盖章）→ 相机飞入新板块
- **话题聚焦模式（BFS 生命线）**：点击卡片后 BFS 沿 relations 扩展，严格受日期窗口约束，只点亮窗口内节点，非相关卡片退入背景
- **完整生命周期视图**：点击"查看完整生命周期"调用 `getSectionLifecycle(id)`（不限天数），迷雾消失，只渲染该话题的完整演化线
- **2D 详情面板叠加**：CSS2DRenderer 在 3D 场景上叠加案件档案风格的详情面板（案件编号、线索链、文章列表）
- **后处理管线**：FilmGrain + Vignette + Bloom（红线发光），使用 `pmndrs/postprocessing`
- **GSAP 动画驱动**：所有 3D 动画（相机、卡片入场、红线绘制、转场）由 GSAP Timeline 编排，与 unify-ui-components 的 GSAP 统一层共享基础设施

## Capabilities

### New Capabilities
- `detective-wall-scene`: Three.js 3D 侦探照片墙场景，含卡片、红线、迷雾、光照、后处理管线
- `detective-wall-camera`: 导演相机系统，GSAP 驱动的运镜（聚焦今天、板块飞越、话题追踪），板块切换的档案封面转场
- `detective-wall-interaction`: 交互层——点击卡片触发 BFS 生命线展开（日期窗口约束）、Raycaster 悬停/点击、2D 面板叠加、时间范围切换

### Modified Capabilities

- `topic-overview`（现有 `BoardThreadBrowser` 2D 总览）：新增"进入侦探墙"入口按钮，不替换现有功能

## Impact

- **新增文件**：
  - `features/tags/components/TopicDetectiveWall.client.vue`（全屏 3D 容器 + Vue 状态管理）
  - `features/tags/components/detective-wall/`（Three.js 场景子模块）
    - `TopicWallScene.ts`（场景管理、渲染循环）
    - `CardGroup.ts`（卡片+图钉创建、布局）
    - `RedString.ts`（红线连接创建、drawProgress 动画）
    - `FogSystem.ts`（迷雾密度管理、时间窗口联动）
    - `DirectorCamera.ts`（相机运镜、Shot 类型定义）
    - `WallPostProcessing.ts`（后处理管线 setup）
    - `InteractionLayer.ts`（Raycaster、hover/click）
    - `ChapterTransition.ts`（板块切换转场、档案封面）
  - `features/tags/components/detective-wall/shaders/`（自定义 shader：卡片纸纹理、迷雾、红线脉动）
- **修改文件**：
  - `features/tags/components/BoardThreadBrowser.vue`：添加"进入侦探墙"按钮
  - `features/tags/components/TagsPage.vue`：集成全屏 3D 视图的显示/隐藏逻辑
- **依赖变更**：
  - `gsap`：需新增安装（也是 unify-ui-components 的共享依赖）
  - `pmndrs/postprocessing`：需新增安装
  - `three`：已安装，直接使用
  - `Line2` / `LineMaterial`（Three.js examples）：用于粗红线
- **无后端影响**：复用现有 API（`getBoardSectionTimeline`、`getSectionLifecycle`、`getDailyReportDetail`）
- **无 API 变更**：不涉及数据层
- **性能考量**：
  - 默认 7 天窗口，卡片数量可控（单板块通常 < 30 个节点）
  - 迷雾自动 cull 远处对象
  - WebGL fallback：需要检测 WebGL 支持，不支持时保持 2D 总览
  - 移动端：不提供 3D 入口，保持 2D 总览
