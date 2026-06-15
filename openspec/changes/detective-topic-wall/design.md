## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│ Vue 层                                                       │
│  TopicDetectiveWall.client.vue                               │
│  ├── <canvas> (Three.js)                                    │
│  ├── BoardSelectorOverlay (2D HTML)                         │
│  ├── DaysRangeControl (2D HTML)                             │
│  ├── TopicDetailPanel (2D HTML, position: fixed 叠加)        │
│  ├── CardTooltip (CSS2DRenderer, 跟随 3D 卡片)              │
│  └── ChapterTitleOverlay (2D HTML, 转场时显示)              │
│                          │ props / events                    │
│  Three.js 层 (TopicWallScene)  ◄────────┘                   │
│  ├── CorkBoardWall                                          │
│  ├── CardGroup → PinCard[] + RedString[]                    │
│  ├── DirectorCamera (GSAP 驱动)                             │
│  ├── FogSystem                                              │
│  ├── InteractionLayer (Raycaster)                           │
│  └── PostProcessing (Bloom + Vignette + FilmGrain)         │
└─────────────────────────────────────────────────────────────┘
```

## Data Flow

### 数据源

复用现有 API，不新增后端接口。

| API | 用途 | 参数 |
|-----|------|------|
| `getBoardSectionTimeline(boardId, days)` | 总览模式：获取时间窗口内的话题节点和关系 | boardId, days (7/14/30/60) |
| `getSectionLifecycle(sectionId)` | 完整生命周期：单个话题的完整演化图 | sectionId（不限天数） |
| `getDailyReportDetail(id)` | 话题详情：线索列表和文章 | reportId |

### 数据到 3D 映射

```
SectionTimelineNode → PinCard (3D 卡片)
  .id           → 卡片唯一标识，用于交互和高亮
  .period_date  → X 轴位置（时间）
  .cluster_label → 卡片上的标题文字 (TextSprite)
  .status       → 状态色条颜色 + 图钉样式
  .article_count → 卡片右下角数字
  .thread_count  → 卡片右下角数字

SectionRelation → RedString (红线)
  .from_id      → 起始卡片
  .to_id        → 目标卡片
  .distance     → 红线透明度权重（近=实，远=虚）

Y 轴位置：同一日期内多个话题的垂直错开（避免重叠）
Z 轴位置：轻微随机偏移（0~0.3），增加空间感
```

## Scene Layout

### 空间坐标系

```
X 轴 = 时间（从左到右，单位：天）
Y 轴 = 同日话题的垂直排列（避免重叠）
Z 轴 = 轻微景深（卡片不完全贴平墙壁）
```

卡片定位算法：

```typescript
// 伪代码
function layoutCards(sections: SectionTimelineNode[], relations: SectionRelation[]) {
  const COL_W = 3.0   // 列宽（天间距，世界单位）
  const ROW_H = 2.2   // 行高（同日话题间距）
  const Z_JITTER = 0.3 // Z 轴随机偏移范围

  // 1. 按日期分列
  const dates = unique(sections.map(s => s.period_date.slice(0, 10))).sort()
  const dateX = new Map(dates.map((d, i) => [d, i * COL_W]))

  // 2. 每列内按 relation 分组排布 Y
  //    有关联的话题尽量靠近，无关联的拉开
  //    （简化：按文章数降序排列即可）

  // 3. Z 轴轻微随机
  //    z = Math.random() * Z_JITTER
}
```

### 默认相机位

相机定位到"今天"正前方，不是远距离鸟瞰。

```typescript
// 相机参数
const CAMERA_DEFAULT = {
  // 定位在今天列的正前方
  fov: 50,
  height: 6,        // 略高于卡片平面
  distance: 8,      // 距卡片平面的距离
  lookDownAngle: 25, // 度，微微俯视
}
```

## Component Design

### PinCard（话题卡片）

```
┌────────────────────────────┐
│  🔴 ← 图钉 (红色球+金属柱) │
│                            │
│  量子计算                   │ ← TextSprite (标题)
│  ────────                  │
│  ● 持续                    │ ← 状态色条
│  5篇 · 3线索               │ ← 元信息
│                            │
└────────────────────────────┘

Three.js 结构:
- Group
  ├── PaperMesh (BoxGeometry, depth=0.05, 纸纹理)
  │   └── MeshStandardMaterial(map: paperTexture, color: #FFFBEB)
  ├── PinMesh (CylinderGeometry + SphereGeometry, 红色金属)
  │   └── MeshStandardMaterial(color: #DC2626, metalness: 0.7)
  ├── LabelSprite (TextSprite, 话题标题)
  ├── StatusBar (小色条, 颜色由 status 决定)
  └── MetaText (TextSprite, "5篇 · 3线索")
```

卡片微倾斜：每张卡片绕 Z 轴旋转 -3° ~ +3°（随机固定值，创建时确定）。

### RedString（红线）

```typescript
// 风格化直线，不做物理悬垂
// 用 Line2 (fat lines) 实现可控线宽

class RedString {
  // 从卡片 A 的右侧边缘到卡片 B 的左侧边缘
  // 直线连接，不做贝塞尔弯曲

  material: LineMaterial
  line: Line2

  // 状态
  highlighted: boolean   // 生命线模式下高亮
  drawProgress: number   // 0~1, 红线绘制动画进度

  // 高亮时: 线宽增大 + emissive 脉动
  // 非高亮: 半透明 (#DC2626, opacity 0.4)
  // 距离越远 opacity 越低
}
```

### FogSystem（迷雾）

```typescript
class FogSystem {
  fog: FogExp2
  currentDensity: number  // 由时间范围决定

  // 密度映射:
  // 7天  → density: 0.08 (迷雾近，聚焦)
  // 14天 → density: 0.05
  // 30天 → density: 0.03 (迷雾远，视野宽)
  // 60天 → density: 0.02

  // 切换时间范围时:
  // gsap.to(this, { currentDensity, duration: 0.8, ease: 'power2.inOut' })

  // 完整生命周期模式:
  // 暂时关闭迷雾 (density: 0)
}
```

迷雾颜色：接近背景色（深灰黑 `#0a0f14`），不是白色。

### DirectorCamera（导演相机）

```typescript
interface CameraShot {
  position: Vector3    // 相机位置
  target: Vector3      // 看向的目标点
  fov: number          // 视场角
  duration: number     // 过渡时长(秒)
  ease: string         // GSAP easing
}

class DirectorCamera {
  camera: PerspectiveCamera
  currentShot: CameraShot

  // 预设镜头
  shots: {
    // 聚焦今天（默认）
    todayFocus(): CameraShot

    // 板块总览（鸟瞰）
    overview(): CameraShot

    // 话题聚焦（飞到某张卡片附近）
    topicFocus(cardId: number): CameraShot

    // 生命周期视图（后退到能看到完整时间跨度）
    lifecycleFull(span: number): CameraShot
  }

  // 运镜
  transitionTo(shot: CameraShot): gsap.core.Timeline

  // 镜头语言:
  // todayFocus → topicFocus: "推进" (dolly in + 轻微偏转)
  // topicFocus → todayFocus: "回退" (dolly out)
  // 板块切换: 转场动画接管，不走普通 transitionTo
}
```

## Interaction Design

### 交互状态机

```
                    ┌──────────┐
                    │  IDLE    │ ← 默认，聚焦今天
                    │ 聚焦今天  │
                    └────┬─────┘
                         │
              点击卡片    │    点击空白
              ┌──────────▼──────────┐
              │                     │
              ▼                     │
     ┌────────────────┐             │
     │ FOCUSING        │             │
     │ BFS 生命线展开  │─────────────┘
     └───┬────────┬───┘
         │        │
  点"深挖"│        │点"完整生命周期"
  (扩大   │        │
  时间窗口)│        ▼
         │   ┌────────────────┐
         │   │ LIFECYCLE       │
         │   │ 完整演化视图    │
         │   │ 迷雾消失        │
         │   └────────────────┘
         │
         ▼
   ┌────────────────┐
   │ 时间窗口扩大    │
   │ 迷雾后退        │
   │ BFS 重新计算    │
   └────────────────┘
```

### BFS 生命线算法

```typescript
function bfsLifeline(
  startNode: SectionTimelineNode,
  relations: SectionRelation[],
  allNodes: SectionTimelineNode[],
  dateRange: { start: string, end: string }
): {
  nodes: SectionTimelineNode[],
  edges: SectionRelation[]
} {
  const visited = new Set<number>()
  const queue = [startNode.id]
  visited.add(startNode.id)

  // 预建无向邻接表（Map<number, Set<number>>，实现见 interaction spec §BFS Lifeline Algorithm）
  const adj = buildUndirectedAdjacencyMap(relations)

  // 日期索引
  const nodeMap = new Map(allNodes.map(n => [n.id, n]))

  while (queue.length > 0) {
    const current = queue.shift()!
    const neighbors = adj.get(current) || []

    for (const { nodeId } of neighbors) {
      if (visited.has(nodeId)) continue

      const node = nodeMap.get(nodeId)!

      // 关键约束：日期窗口检查
      const date = node.period_date.slice(0, 10)
      if (date < dateRange.start || date > dateRange.end) continue

      visited.add(nodeId)
      queue.push(nodeId)
    }
  }

  // 收集结果
  const nodes = allNodes.filter(n => visited.has(n.id))
  const edges = relations.filter(r =>
    visited.has(r.from_id) && visited.has(r.to_id)
  )

  return { nodes, edges }
}
```

### 点击与悬停

```
悬停卡片:
  - 卡片沿 Z 轴微微抬起 (translateZ += 0.2)
  - 图钉发光增强
  - 直接邻居的红线变亮
  - CSS2D 小 tooltip 显示话题名和状态

点击卡片:
  - 触发 BFS 生命线
  - 非相关卡片: opacity 降低 + 略后退
  - 生命线红线逐条画出 (drawProgress stagger)
  - 生命线节点逐个点亮 (emissive stagger)
  - 相机 dolly in 到聚焦位置
  - 2D 详情面板滑入

点击红线:
  - 沿该线追踪，以对端节点为起点重新 BFS
  - 相机平移到新焦点

点击空白:
  - 所有卡片恢复正常
  - 相机回到 todayFocus
  - 详情面板滑出
```

## Chapter Transition（板块切换转场）

### 动画序列

```
总时长: ~1.0s

Phase 1: 红色 wipe 扫过 (0~0.15s)
  - 全屏红色条带从左到右横扫
  - 实现: CSS overlay div, gsap.to transform: translateX
  - 颜色: #DC2626

Phase 2: 档案封面 (0.15~0.65s)
  - 红色条带消失，档案封面浮现
  - 标题逐字出现（打字机效果）
  - 样式:
    - 打字机字体 (JetBrains Mono / monospace)
    - 暖黄纸底 (#FFFBEB)
    - 左上 "CONFIDENTIAL" 红色盖章
    - 右下 "CLASSIFIED" 黑色印章 + 日期
    - 板块名、时间窗口、话题统计
    - 整体倾斜 2°
  - 实现: Vue 组件 + GSAP TextPlugin stagger

Phase 3: 封面淡出 + 新场景入场 (0.65~1.0s)
  - 档案封面淡出
  - 相机飞到新板块的 todayFocus 位
  - 新卡片从底部弹上来钉到墙上 (stagger 0.03s/张)
  - 红线随后画出 (stagger 0.02s/条)
```

### GSAP Timeline 编排

```typescript
function playChapterTransition(
  overlay: HTMLElement,    // wipe + 档案封面的 DOM
  scene: TopicWallScene,
  newBoardData: { name: string, dateRange: string, topicCount: number }
): gsap.core.Timeline {
  const tl = gsap.timeline()

  tl
    // Phase 1: wipe
    .to(wipeElement, { x: '100%', duration: 0.15, ease: 'power2.in' })

    // Phase 2: 档案封面
    .set(coverElement, { opacity: 1, scale: 0.95 })
    .to(coverElement, { scale: 1, duration: 0.15, ease: 'back.out(1.2)' })
    .add(() => typewriterReveal(coverText, newBoardData))
    .to({}, { duration: 0.2 }) // 停顿让用户看清

    // Phase 3: 退场
    .to(coverElement, { opacity: 0, scale: 1.02, duration: 0.15 })
    .add(() => scene.swapBoardData(newBoardData))
    .to(scene.camera, { /* fly to todayFocus */ duration: 0.25 }, '<')
    .add(() => scene.staggerCardEntrance(0.03), '<')

  return tl
}
```

## Post Processing

> ⚠️ 下例为**意图伪代码**，表达效果链组合与参数意图，不是可直接编译的 API。
> 实现时按 `pmndrs/postprocessing` 当前版本 API 落地：`BloomEffect`/`VignetteEffect` 是 effect，
> 需用 `composer.addPass(new EffectPass(camera, effect))` 包装，而非 `addPass(effect)`。
> 具体写法以库版本为准。

效果链（意图）:

```
RenderPass(scene, camera)
  → BloomEffect   { intensity: 0.6, luminanceThreshold: 0.8, luminanceSmoothing: 0.3 }  // 只让红线发光
  → VignetteEffect{ darkness: 0.5 }                                                       // 暗角聚焦中央
  → 颗粒感效果     { intensity: 0.04 }                                                     // 轻微胶片颗粒，增强氛围
```

### FilmGrain 选型（按优先级降序，任选其一即可）

1. **优先**：`three/examples` 自带的 `FilmPass`（含颗粒 + 扫描线，参数可调），无新增依赖（three 已装）。
2. 次选：`pmndrs/postprocessing` 的 `NoiseEffect`（配合 `EffectPass` 包装）。
3. 兜底：自定义 GLSL simplex noise 的 `ShaderPass`（仅在上述方案效果不达标时采用，**不必一开始就从头写 shader**）。

设计目标是"很轻微的颗粒感"，不必为它单独引入或手写复杂 shader——优先复用现成 pass。

## Color & Style Reference

```
色板:
  #DC2626  证据红    — 红线、图钉、wipe、高亮状态
  #0a0f14  深夜底    — 场景背景、迷雾色
  #FFFBEB  旧纸黄    — 卡片底色、档案封面
  #1A1A1A  墨黑      — 卡片文字、印章
  #D97706  琥珀金    — 关键数字、计数器
  #4B5563  灰铅      — 日期标注、次要信息

状态色 (与现有 2D 保持一致):
  emerging  → #16a34a (绿)
  continuing → #2563eb (蓝)
  split     → #ea580c (橙)
  merge     → #9333ea (紫)
  ending    → #9ca3af (灰)

材质:
  卡片 → MeshStandardMaterial, roughness: 0.85, metalness: 0.0
  图钉 → MeshStandardMaterial, roughness: 0.3, metalness: 0.7
  红线 → LineMaterial, dashed: false, linewidth: 2-4px
  背景 → 大平面 + procedural noise (cork/暗房纹理)

光照:
  AmbientLight → intensity 0.15 (暗房感)
  SpotLight    → 从上方打, angle: 45°, penumbra: 0.5 (手电筒)
  PointLight   → 跟随相机 (探险灯)
  PointLight   → 红色, 位于被选中话题卡片上方 (选中高亮)

字体:
  档案封面 → 'JetBrains Mono', 'Courier New', monospace
  卡片标题 → 系统默认 sans-serif (通过 TextSprite 渲染)
```

## Animation Library Split

本 change 同时使用两个动画库，分工严格不重叠：

| 库 | 归属 | 职责 | 引入方式 |
|----|------|------|----------|
| `gsap` | 3D 场景 | 对 `THREE.Object3D` / 材质属性做逐帧补间：DirectorCamera 运镜、卡片 stagger 入场、红线 drawProgress、BFS 逐节点点亮、章节转场 Timeline 编排 | **本 change 自行新增安装**（项目内无前置 change 提供） |
| `motion-v` | 2D overlay | Vue 组件 enter/leave/过渡：BoardSelector、DaysRange、DetailPanel 的进入/退出动画 | 已安装，直接复用 |

GSAP 选型理由：对**非 DOM 对象**（`camera.position`、`material.opacity`、`group.rotation`）的属性补间和 `timeline()` 多轨编排是 motion-v 的短板，而这正是 3D 运镜的核心需求。motion-v 继续管它擅长的 Vue 组件声明式过渡，二者无需互相替代。

```
detective-topic-wall
  ├── gsap (本 change 自行安装)
  │   ├── DirectorCamera 运镜
  │   ├── 卡片 stagger 入场
  │   ├── 红线 drawProgress
  │   ├── BFS 逐节点点亮
  │   └── 章节转场 Timeline
  └── motion-v (已安装，复用)
      ├── BoardSelector / DaysRange 进入退出
      └── DetailPanel slide-in / slide-out
```

## Fallback & Platform

```
WebGL 检测:
  const gl = canvas.getContext('webgl2') || canvas.getContext('webgl')
  if (!gl) → 不显示"进入侦探墙"按钮，保持 2D 总览

移动端:
  屏幕宽度 < 768px → 不显示入口按钮
  3D 照片墙是大屏沉浸体验，不适合小屏操作

性能:
  默认 7 天 → 通常 < 30 个卡片，完全流畅
  60 天 → 可能 100+ 卡片，迷雾 cull 远处，可接受
  完整生命周期 → 单条线，节点数有限，无压力
```

## File Structure

```
features/tags/components/
├── BoardThreadBrowser.vue              (修改: 添加入口按钮)
├── TopicDetectiveWall.client.vue       (新建: 全屏容器)
└── detective-wall/                     (新建: 场景子模块)
    ├── TopicWallScene.ts               (场景管理、渲染循环、数据→3D 同步)
    ├── CardGroup.ts                    (PinCard 创建、布局、动画)
    ├── RedString.ts                    (红线创建、drawProgress、高亮)
    ├── FogSystem.ts                    (迷雾密度、时间范围联动)
    ├── DirectorCamera.ts               (相机运镜、Shot 预设)
    ├── WallPostProcessing.ts           (后处理管线)
    ├── InteractionLayer.ts             (Raycaster、hover/click 处理)
    ├── ChapterTransition.ts            (板块转场、档案封面)
    ├── shaders/
    │   ├── paper.vert / paper.frag     (卡片纸纹理)
    │   └── grain.vert / grain.frag     (胶片颗粒)
    └── types.ts                        (共享类型定义)
```
