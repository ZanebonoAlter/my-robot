# 前端架构

> 最后更新：2026-06-11（v1.3.3 架构深化：Store 拆分、事件流统一、错误通知、API 归一化、Feature Facade、大组件拆分）

## 技术栈

- Nuxt 4.2.2
- Vue 3.5
- TypeScript
- Pinia
- Tailwind CSS v4
- Iconify
- Day.js
- marked
- motion-v
- Vitest

## 入口与路由

- 应用壳入口：`front/app/app.vue`
- 主阅读页：`front/app/pages/index.vue`
- 标签管理：`front/app/pages/tags.vue`
- Topic Graph：`front/app/pages/topics.vue`

`app.vue` 只做一件事：启动时调用 `apiStore.initialize()`，先拉分类、订阅源和文章，再渲染页面。

## 目录结构

```text
front/
├─ app/
│  ├─ api/                 # 唯一 HTTP 边界
│  │  ├─ client.ts         # ApiClient 封装（fetch、错误处理、query 构建）
│  │  ├─ index.ts          # 统一 re-export 所有 API composable
│  │  ├─ normalizers/      # 共享数据 normalizer（snake_case → camelCase）
│  │  │  └─ article.ts     # 文章 DTO 转换（唯一权威 normalizer）
│  │  ├─ categories.ts
│  │  ├─ feeds.ts
│  │  ├─ articles.ts
│  │  ├─ summaries.ts
│  │  ├─ ...               # 各领域 API 模块
│  │  └─ createQueueApi.ts # 泛型队列 API 工厂
│  ├─ assets/css/          # 全局主题与基础样式
│  ├─ components/          # 通用组件与对话框（NotifyContainer、dialog、feed 等）
│  ├─ composables/         # 跨 feature 的全局能力
│  │  ├─ useEventStream.ts # 统一实时事件流（WebSocket 单例 + 类型化订阅）
│  │  ├─ useNotify.ts      # 全局通知管道（toast 队列）
│  │  ├─ useGlobalSettings.ts
│  │  ├─ useSchedulerStatus.ts
│  │  ├─ useDailyReportProgress.ts
│  │  ├─ useAI.ts
│  │  ├─ useFirecrawlConfig.ts
│  │  └─ useReadingPreferences.ts
│  ├─ features/            # 业务实现主体（见下文详细说明）
│  ├─ pages/               # Nuxt 路由入口
│  ├─ plugins/             # Nuxt 插件
│  ├─ stores/              # Pinia store（见 Store 层说明）
│  ├─ types/               # 领域类型（API 响应、Store 状态、业务模型）
│  ├─ utils/               # 常量和纯工具函数
│  │  ├─ api-helpers.ts    # 唯一查询构建/解包/camelCase 转换工具
│  │  ├─ eventTypes.ts     # SSE/WS 事件类型常量集中定义
│  │  ├─ constants.ts
│  │  ├─ date.ts
│  │  └─ text.ts
│  └─ app.vue
├─ nuxt.config.ts
└─ package.json
```

## Feature 划分

每个 feature 是一个自包含的业务模块，包含自己的组件、composable、工具和公共 facade。

```text
features/
├─ shell/              # 主壳、顶部栏、侧栏、文章列表栏
│  └─ components/      # FeedLayoutShell、AppHeader、AppSidebar、ArticleListPanel
├─ articles/           # 正文阅读、内容补全、Firecrawl 全文、AI 整理稿
│  ├─ components/      # ArticleContentView、ArticleCardView、ArticleTagList
│  ├─ composables/     # useArticleContentView、useArticlePagination、useContentCompletion、useTagWebSocket
│  ├─ utils/           # normalizeArticle（feature 私有）
│  └─ public.ts        # 跨 feature 共享的稳定 facade
├─ feeds/              # 自动刷新和刷新轮询
│  ├─ composables/     # useAutoRefresh、useRefreshPolling
│  └─ public.ts        # 跨 feature 共享 facade
├─ summaries/          # AI 总结列表、队列进度（通过 useEventStream 实时更新）
├─ digest/             # 日报、周报、详情页、配置抽屉
├─ preferences/        # 阅读行为埋点与偏好
│  ├─ composables/     # useReadingTracker
│  └─ public.ts        # 跨 feature 共享 facade
├─ tags/               # 标签管理、合并、质量评分
│  ├─ components/      # TagsPage、BoardCRUD、Timeline、Merge 等
│  ├─ components/detective-wall/  # 3D 侦探照片墙（Three.js 子模块，见 §3D 侦探墙）
│  └─ composables/     # useTagsPage、useBoardCRUD、useBoardTimeline、useAuxiliaryLabels
├─ topic-graph/        # 主题图谱、热点标签、话题详情、analysis、timeline、标签层级、合并预览
│  ├─ components/      # TopicGraphPage、Canvas、Sidebar、Timeline、TagHierarchy、TagMergePreview 等
│  ├─ composables/     # useTopicGraph、useTopicTimeline、useArticlePreview、useFloatingPanelDrag 等
│  ├─ utils/           # buildDisplayedTopicGraph、buildTopicGraphViewModel、normalizeTopicCategory
│  └─ public.ts        # 跨 feature 共享 facade（TagMergePreview）
├─ ai/                 # AI Router 设置面板、Embedding 队列
│  ├─ components/      # AIRouterSettingsPanel、EmbeddingQueuePanel 等
│  └─ composables/     # useAIRouterSettings
└─ hierarchy-config/   # 层级配置管理
```

### Feature Facade 约定

每个 feature 如需暴露跨 feature 共享能力，**必须**通过 `public.ts` facade：

- `features/articles/public.ts` → `ArticleContentView`、`ArticleCardView`、`useArticlePagination`
- `features/feeds/public.ts` → `useGlobalAutoRefresh`、`useRefreshPolling`
- `features/topic-graph/public.ts` → `TagMergePreview`
- `features/preferences/public.ts` → `useReadingTracker`、`useScrollDepthTracker`

**禁止跨 feature 深 import 对方内部实现**（组件、composable、工具函数）。共享 normalizer 上移到 `api/normalizers/`，共享 UI 上移到 `components/` 或通过 feature facade 暴露。

## 数据层约定

### API 层（唯一 HTTP 边界）

`front/app/api/*` 是前端唯一 HTTP 边界。所有后端调用必须经此。

**核心约定：**

- `client.ts` 统一封装 `fetch`，返回 `{ success, data, error, message }`
- 各领域模块通过 `useXxxApi()` composable 形式使用
- query 参数统一走 `buildQueryString()`（`utils/api-helpers.ts`）
- 响应解包使用 `mapApiResponse<T>()` / `unwrapResponse<T>()`
- snake_case → camelCase 转换使用 `camelizeKeys()`（`utils/api-helpers.ts`）
- `api/index.ts` 统一 re-export 所有 API composable

**唯一权威原则（D19）：**

| 职责 | 唯一权威 Module |
|------|-----------------|
| 查询参数构建 | `utils/api-helpers.ts` → `buildQueryString()` |
| 响应解包 | `utils/api-helpers.ts` → `mapApiResponse()` / `unwrapResponse()` |
| camelCase 转换 | `utils/api-helpers.ts` → `camelizeKeys()` |
| 文章 DTO 转换 | `api/normalizers/article.ts` → `normalizeArticle()` |

**已落地的 API 模块：**

`categories` · `feeds` · `articles` · `summaries` · `digest` · `opml` · `reading_behavior` · `firecrawl` · `scheduler` · `aiAdmin` · `topicGraph` · `abstractTags` · `semanticBoards` · `auxiliaryLabels` · `embeddingConfig` · `embeddingQueue` · `mergeReembeddingQueue` · `tagMergePreview` · `tagQueue` · `watchedTags` · `dailyReports`

### Store 层

**Store 职责分离（D13/D16）：**

| Store | 职责 | 自有状态 |
|-------|------|---------|
| `useApiStore` | 应用初始化、categories/feeds 的加载与基本 CRUD | `categories`、`feeds`、全局 loading/error |
| `useArticlesStore` | 文章状态管理，所有文章 mutation 直接调用 API | `articles`、`totalArticles`、`filters`、`currentArticle`、`loading` |
| `useFeedsStore` | Feed 派生视图层（computed from apiStore） | 无独立状态 |
| `usePreferencesStore` | 阅读偏好和统计 | 独立偏好状态 |

**Store 核心规则（store-integrity）：**

1. **状态变更必须经 API 持久化**：Store 内的 mutation 必须调用对应 API 方法，禁止仅修改本地状态
2. **乐观更新 + 回滚**：写操作先本地更新状态，API 失败时回滚并通知用户
3. **Store 之间不得形成循环知识**：`articlesStore` 不 import `apiStore` 的文章 mutation，`apiStore` 不动态 import `articlesStore`
4. **Feed unread count 同步**通过 `useFeedsStore` 暴露的小 Interface（`adjustUnreadCount`）完成

**一次性的 API 调用**（如对话框中的 CRUD）直接走 `api/` 层，不经过 Store。Store 只管理需要跨组件共享的状态。

## 实时事件流（useEventStream）

**唯一全局实时事件 Seam**（D15）。所有全局实时事件消费者必须通过 `useEventStream()` 订阅。

**核心特性：**

- **单例连接**：全局维护唯一 WebSocket 连接（`/ws`），生命周期由订阅者引用计数自动管理
- **类型化事件**：事件类型常量集中在 `utils/eventTypes.ts`
- **自动重连**：指数退避（1s → 30s）
- **自动清理**：最后一个订阅者退订时关闭连接并释放全局实例

**使用模式：**

```typescript
const stream = useEventStream()

// 订阅
const off = stream.on(EVENT_TYPES.TAG_COMPLETED, (data) => { ... })

// 组件卸载时清理（onUnmounted 中）
off()
```

**禁止：**

- Feature/Component 中直接 `new WebSocket` 或 `new EventSource` 订阅全局事件
- 组件卸载后不退订（必须在 `onUnmounted` 执行返回的 unsubscribe 函数）

**特例：** 专用长任务 stream（如 tag merge preview 的 scan/evaluate SSE）在对应 API module 中作为命名 Adapter 暴露。

## 错误通知（useNotify）

**全局通知管道**（D20），所有 UI 错误/成功反馈统一经此。

**核心特性：**

- 使用 Nuxt `useState` 管理 toast 队列（SSR 兼容）
- 提供 `notify.success()`、`notify.error()`、`notify.warn()` 方法
- 全局共享实例，所有 `useNotify()` 调用返回同一队列
- `components/common/NotifyContainer.vue` 渲染 toast 列表

**通知责任层（唯一责任原则）：**

| 层级 | 责任 |
|------|------|
| Store / Composable | 执行写操作失败时调用 `notify.error()` |
| 底层 API module | **不直接弹 toast**，只返回错误 |
| View 组件 | 可保留局部 `error` 展示状态，全局错误走 `useNotify()` |

**禁止同一次失败由多个层同时通知**（如 `apiStore` 和 feature store 同时弹 toast）。

## 数据映射规则

- 后端字段以 `snake_case` 为主
- 前端 store 和组件内部统一用 `camelCase`
- 转换只发生在 API 边界（`api/normalizers/` 和 `utils/api-helpers.ts`）
- ID 在前端统一存成 `string`；数字 ID 与字符串 ID 的转换只在 API 边界
- 后端 snake_case DTO 进入 Store/Feature 前必须经 normalizer 转为前端领域类型

## 页面骨架

主阅读页由 `FeedLayoutShell.vue` 组织为三栏：

- 左栏：分类、订阅源、快捷入口
- 中栏：文章列表或 AI 总结列表
- 右栏：文章正文或 AI 总结详情

Digest 页面走独立路由和独立视觉壳，不复用主阅读页的三栏壳。

### Topics 页面架构

#### 组件结构

- `TopicGraphPage`: 主页面容器
  - `TopicGraphHeader`: 头部控制区（返回首页、刷新图谱）
  - `TopicGraphCanvas`: 3D 拓扑图
  - `TopicGraphFooterPanels`: 底部面板区（含 TagQueuePanel）
  - `TopicGraphSidebar`: 右侧详情栏
  - `TopicTimeline`: digest timeline
  - `TagHierarchy`: 标签层级树
  - `KeywordCloud`: 关键词云

#### Composable 深化拆分（D17）

Topic Graph 页面的复杂度已按内聚行为拆分为多个深 Module：

| Module | 职责 | 外部 Interface |
|--------|------|----------------|
| `useTopicGraph` | 页面级状态编排 | `filters`、`viewModel`、`loadGraph`、`loading/error` |
| `useTopicTimeline` | digest、pending articles、聚合分组 | `items`、`groups`、`selectDigest`、`loadForTopic` |
| `useArticlePreview` | 文章预览、收藏、局部更新 | `selectedArticle`、`open`、`close`、`toggleFavorite` |
| `useFloatingPanelDrag` | 浮层拖拽通用逻辑 | `panelRef`、`position`、`startDrag`、`reset` |
| `useHotspotTopics` | 热点标签加载 | `hotspotTopics`、`loading` |

同样规则适用于 `useTagsPage()` 等其他大型页面 composable。

#### 数据流

1. 用户点击分类入口（热点分类按钮）后更新 `selectedCategory`
2. 页面基于 `selectedCategory` 计算 `highlightedNodeIds`
3. `highlightedNodeIds` 传给 `TopicGraphCanvas` 并驱动图谱高亮
4. 用户切换分析 Tabs 或选中标签后，底部分析面板按类型加载对应分析数据

#### 标签展示约定

- `ArticleContentView` 现在会直接展示标准 article tags
- digest 列表、digest 详情、topic graph timeline 使用 digest 的 `aggregated_tags`
- `/topics` 中当前选中的 topic slug 会在 digest/article 标签上做高亮

## 大组件拆分阈值（D14）

- 单文件超过 **500 行 / ~15KB** 时应考虑拆分
- 拆分目标不是文件行数下降，而是形成多个**深 Module**，每个以小 Interface 隐藏一组高内聚行为
- 不能只把复杂度搬进单个巨型 composable（D17）
- 组件通过 composable 获取状态和方法，子组件通过 props 接收数据

## 3D 侦探墙（detective-wall）

`features/tags/components/detective-wall/` 是项目内首个直接使用 Three.js 的特性，将话题总览（`BoardThreadBrowser` 2D SVG DAG）升级为沉浸式 3D 侦探照片墙。入口在 `BoardThreadBrowser` 的"侦探墙"按钮（仅 WebGL 可用且屏幕宽 ≥768px 显示），全屏视图 `TopicDetectiveWall.client.vue` 由 `BoardDailyReportTimeline` 渲染。

### 子模块结构

```text
detective-wall/
├─ types.ts              # 共享类型 + STYLE 常量 + SUPPORTED_DAYS
├─ utils.ts              # 可测纯函数：bfsLifeline / layoutCards / densityForDays / edgeKey
├─ TopicWallScene.ts     # 场景管理（renderer/camera/scene/CSS2D/软木墙面/render loop/dispose）
├─ CardGroup.ts          # PinCardImpl（档案袋 CanvasTexture + 可选文章配图 + 图钉/胶带）+ CardGroup
├─ RedString.ts          # Line2 红线绳（边缘锚点折线 + 距离衰减 opacity）+ RedStringCollection
├─ FogSystem.ts          # FogExp2（密度映射天数窗口，gsap 动画）
├─ lighting.ts           # HemisphereLight（暖天/冷地）+ 台灯定向暖 SpotLight + 跟随相机 PointLight + 选中红色 PointLight
├─ WallPostProcessing.ts # pmndrs EffectComposer + Bloom + Vignette + three/examples FilmPass
├─ DirectorCamera.ts     # gsap 运镜（todayFocus/overview/topicFocus/lifecycleFull）
├─ InteractionLayer.ts   # Raycaster hover/click + BFS 生命线编排 + 状态机
├─ ChapterTransition.ts  # 板块切换 wipe + 档案封面打字机（gsap Timeline）
├─ SetDressing.ts        # 桌面/远景墙/台灯/卷宗堆（侦探办公桌环境层，随 loadBoardData 重建）
├─ AmbientEnv.ts         # PMREM 程序化暖色环境贴图（图钉/黄铜金属反射）
├─ DustParticles.ts      # 台灯光锥尘埃微粒（additive Points）
└─ shaders/directionalFog.ts # 方向雾 onBeforeCompile 注入（过去浓/今天清，不注入卡片）
```

### 动画库分工

3D 场景用 **gsap**（对 `THREE.Object3D`/材质属性的逐帧补间 + `timeline()` 多轨编排），2D overlay 用项目已装的 **motion-v**（Vue 组件声明式过渡）。详情面板是普通 Vue `position:fixed` overlay（非 CSS2DRenderer）；仅卡片悬停 tooltip 用 CSS2DRenderer（跟随 3D 坐标）。

### 视觉对象语义

侦探墙的 3D 对象应优先服务“证据墙”隐喻，而不是普通关系图：

- `CardGroup` 使用 canvas procedural 档案袋作为 `CanvasTexture`，在同一纹理内绘制固定边界的标题、`CASE #id`、状态、文章/线索计数，并优先渲染 section 关联文章中的首张 `image_url`；无真实图片时绘制案件路线式默认缩略图。卡片点击热区仍由 3D mesh + CSS2D `data-card-id` tooltip 双路承担。
- `RedString` 不从卡片中心连线，而是根据相对方向锚定在卡片边缘附近；路径使用轻微折线并抬到卡片前方，普通状态为暗红低透明，生命线高亮时变为证据红且线宽增加。
- `TopicWallScene` 在卡片范围后方动态铺一块暗软木墙面，使用 procedural texture 提供磨砂颗粒、暗格和关卡路线式装饰点；选中红色 PointLight 绑定卡片当前 world position 并沿 z 轴打到卡片前方，随 `loadBoardData()` 重建并在 `clearScene()` dispose。

### 环境纵深层（detective-wall-ambiance）

在“面对软木墙钉卡片”的核心视角之外，叠加一层“侦探办公桌台灯下”的环境纵深，强化空间感与叙事氛围（change `detective-wall-ambiance`）：

- **桌面 + 远景墙 + 台灯 + 卷宗堆**（`SetDressing.ts`）：前景水平桌面建立空间纵深参考系；台灯（banker's lamp：黄铜底座/立柱 + 墨绿灯罩 + emissive 灯泡）成为主 SpotLight 的视觉来源——暖光锥从灯罩射向今天列卡片。主软木墙退到 `z=-0.6`（卡片更多浮出厚度），后方 `z=-4` 一面暗远景墙靠雾衰减。随 `loadBoardData()` 按 `latestDayX`/`minX` 重建。
- **环境贴图**（`AmbientEnv.ts`）：PMREMGenerator 从程序化暖色场景（深棕底 + 台灯位置暖色发光球 + 顶部冷光）烘焙 env map 赋给 `scene.environment`，图钉/黄铜金属从此反射出暖色高光（不替换背景）。
- **方向性雾**（`shaders/directionalFog.ts`）：通过 `material.onBeforeCompile` 给环境表面（桌面/墙/卷宗/台灯/软木墙）的 MeshStandardMaterial 注入 X 方向雾项——越往左（越早的日期）越浓，今天列附近最清，契合“抽丝剥茧”叙事。**不注入卡片**（保持 CardGroup 的 highlight/dim 语义干净）。共享 `fogUniforms` 一处更新全局生效。
- **半球光**（`lighting.ts`）：`AmbientLight` → `HemisphereLight(暖天/冷地)`，上下色温差异带出空气感。
- **光锥尘埃**（`DustParticles.ts`）：~150 个 additive Points 在台灯光锥内正弦漂移，配合 Bloom + Vignette 形成“档案室浮尘”厚重感。

surgical 边界：不改相机坐标系/布局算法/交互/BFS/红线/全局迷雾/后处理；卡片材质不注入方向雾。无新增依赖（PMREMGenerator/HemisphereLight/Points 均属已装 three）。

### BFS 生命线

`utils.bfsLifeline` 与现有 `topic-graph/utils/graphBfsHighlight.bfsHighlight` **语义不同**：后者返回连通分量（带启发式截断、无日期约束），前者严格受日期窗口约束。两者不互相复用。

### 数据层

复用现有 API，无新增后端接口：`getBoardSectionTimeline(boardId, days)`、`getSectionLifecycle(sectionId)`、`getDailyReportDetail(id)`（均来自 `~/api/dailyReports`）。其中 timeline/lifecycle section node 扩展 `image_url` 字符串字段，由后端优先从该 section 的线程关联文章选择第一张非空图片，找不到时再从 cluster tags 当天文章里选择第一张非空图片；仍无图时返回空字符串，由侦探墙卡面渲染默认缩略图。

## 设计系统

### 主题系统

前端采用三层 Token 架构和双主题系统，确保视觉一致性：

**三层 Token 架构：**
1. **Layer 1: Primitive Tokens (`--raw-*`)** - 原始色值，项目色板的唯一来源，不直接在组件中使用
2. **Layer 2: Semantic Tokens (`--color-*`)** - 跟主题走，组件直接引用这一层
3. **Layer 3: Component Tokens (`--dialog-*`, `--button-*` 等)** - 可选，复杂组件的局部 token

**双主题：**
- **Editorial Theme (`data-theme="editorial"`)** - 暖白印刷厂风格，主色 Print Red，背景 Paper Warmth
- **Dark Theme (`data-theme="dark"`)** - 深色调查风格，强调色 Amber，背景深蓝灰

**主题切换：**
- 通过 `<html data-theme="editorial|dark">` 属性切换
- 使用 `useTheme()` composable 管理主题状态，支持 localStorage 记忆
- 首次绘制前 `<html>` 必须已有有效 `data-theme`，避免主题闪烁

**Token 使用规则：**
- 组件只引用 Layer 2 语义 token（`--color-*`），不直接使用原始色值
- `--color-bg-overlay` 仅用于模态遮罩，不得作为普通表面背景
- 页面表面按层级使用 `base → elevated → sunken`
- SVG、Canvas 和 CSS gradient 颜色必须由当前主题 token 派生

### 统一组件库

统一组件位于 `components/ui/`，提供一致的交互和视觉体验：

- **AppDialog** - 统一对话框外壳，含 Teleport、Transition、overlay、关闭行为
- **AppButton** - 统一按钮组件，支持 primary/secondary/ghost/danger 变体和 sm/md/lg 尺寸
- **AppToggle** - 统一开关组件，响应主题 token
- **AppInput** - 统一输入框组件，响应主题 token，支持 type="number"
- **AppSectionHeader** - 统一 section 标题组件，可选 icon box + 标题 + 描述

**使用约束：**
- 所有对话框必须使用 AppDialog，禁止自建对话框模式
- 所有按钮必须使用 AppButton，禁止使用原生 button 样式类
- 所有开关必须使用 AppToggle，禁止使用原生 checkbox 或手写 toggle
- 所有输入框必须使用 AppInput，禁止使用原生 input 样式类

### 设计约束

- 不用紫色 / 靛蓝色方案
- 不用纯平背景
- 不用默认 shadcn / Material 风格
- 不做对称、平均分栏的模板布局
- 保持 editorial / magazine 风格，避免通用 SaaS 外观

## 运行与环境

- 前端开发端口：`http://localhost:3000`
- 后端 API：`http://localhost:5000/api`
- 全局实时事件：`ws://localhost:5000/ws`（通过 `useEventStream()` 统一管理）
- 专用 SSE 端点：tag merge preview scan/evaluate 等

## 相关文档

- `docs/reference/architecture/backend.md`
- `docs/reference/architecture/data-flow.md`
- `docs/reference/architecture/overview.md`
- `front/AGENTS.md`
