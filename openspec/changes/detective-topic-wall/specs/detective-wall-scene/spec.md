## Capability

Three.js 3D 侦探照片墙场景，负责卡片、红线、迷雾、光照和后处理的创建与管理。接收 Vue 层传入的 section/relation 数据，映射为 3D 对象并渲染。

## API

### TopicWallScene

```typescript
class TopicWallScene {
  constructor(canvas: HTMLCanvasElement)

  // 数据加载
  loadBoardData(
    sections: SectionTimelineNode[],
    relations: SectionRelation[],
    dateRange: { start: string; end: string }
  ): void

  // 清空场景（切换板块前调用）
  clearScene(): void

  // 渲染控制
  startRenderLoop(): void
  stopRenderLoop(): void
  onResize(width: number, height: number): void

  // 生命周期
  dispose(): void

  // 暴露给 InteractionLayer 和 DirectorCamera
  readonly scene: THREE.Scene
  readonly camera: THREE.PerspectiveCamera
  readonly renderer: THREE.WebGLRenderer
  readonly composer: EffectComposer
  readonly cardGroup: CardGroup
  readonly redStrings: RedString[]
}
```

### CardGroup

```typescript
class CardGroup {
  // 创建所有卡片（调用 layout 算法）
  buildCards(
    sections: SectionTimelineNode[],
    relations: SectionRelation[],
    dateRange: { start: string; end: string }
  ): void

  // 获取卡片
  getCardById(sectionId: number): PinCard | undefined
  readonly cards: PinCard[]

  // 批量动画
  staggerEntrance(intervalSec: number): gsap.core.Timeline
  staggerExit(intervalSec: number): gsap.core.Timeline

  // 高亮控制
  highlightSet(ids: Set<number>): void
  dimAll(): void
  resetAll(): void
}
```

### PinCard

```typescript
interface PinCard {
  readonly data: SectionTimelineNode
  readonly group: THREE.Group       // 包含 paper + pin + text
  readonly position: THREE.Vector3  // 世界坐标

  // 动画
  elevate(): void                   // 悬停：沿 Z 抬起
  settle(): void                    // 取消悬停：回到原位
  highlight(): void                 // 生命线点亮：emissive 增强
  dim(): void                       // 退入背景：opacity 降低
  reset(): void                     // 恢复正常状态
}
```

### RedString

```typescript
interface RedString {
  readonly fromId: number
  readonly toId: number
  readonly distance: number

  // 动画
  draw(progress: number): void      // drawProgress 0→1，红线逐渐出现
  highlight(): void                 // 高亮：线宽增大 + 发光
  dim(): void                       // 退入背景
  reset(): void
}
```

### FogSystem

```typescript
class FogSystem {
  constructor(scene: THREE.Scene)

  setDensityForDays(days: number): void
  // 7 → 0.08, 14 → 0.05, 30 → 0.03, 60 → 0.02

  animateToDensity(density: number, durationSec: number): gsap.core.Tween
  disable(): void                   // 完整生命周期模式
  enable(days: number): void        // 恢复
}
```

## Layout Algorithm

```
输入: sections[], relations[]
输出: Map<sectionId, { x, y, z }>

1. dates = unique sorted dates from sections
2. dateX(date) = indexOf(date) * COL_W  (COL_W = 3.0)
3. 同一日期内:
   a. 按 article_count 降序
   b. 分配 Y 位置: row * ROW_H (ROW_H = 2.2)
   c. 有 relation 连接的相邻日期的节点，尽量 Y 靠近
      (简化：暂不优化，直接按序排列，后续可加入力导向微调)
4. Z = random(-0.15, 0.15)
5. 每张卡片 rotation.z = random(-3°, 3°)
```

## Post Processing

```
管线:
  RenderPass → BloomEffect → VignetteEffect → FilmGrainPass

BloomEffect:
  intensity: 0.6
  luminanceThreshold: 0.8
  luminanceSmoothing: 0.3
  → 只让红线（emissive red）发光，卡片不发光

VignetteEffect:
  darkness: 0.5
  → 暗角聚焦中央信息

FilmGrainPass:
  intensity: 0.04
  → 轻微胶片颗粒，增强氛围
  → 自定义 ShaderPass (simplex noise)
```

## Style Constants

```
色板:
  CARD_PAPER = #FFFBEB
  CARD_BORDER = rgba(26, 26, 26, 0.12)
  PIN_COLOR = #DC2626
  PIN_METALNESS = 0.7
  PIN_ROUGHNESS = 0.3
  STRING_COLOR = #DC2626
  STRING_BASE_OPACITY = 0.4
  STRING_HIGHLIGHT_OPACITY = 1.0
  BG_COLOR = #0a0f14
  FOG_COLOR = #0a0f14

状态色 (复用现有):
  STATUS_COLORS = {
    emerging: '#16a34a',
    continuing: '#2563eb',
    split: '#ea580c',
    merge: '#9333ea',
    ending: '#9ca3af',
  }

光照:
  AMBIENT_INTENSITY = 0.15
  SPOT_ANGLE = 45°
  SPOT_PENUMBRA = 0.5

尺寸:
  CARD_WIDTH = 2.0 (世界单位)
  CARD_HEIGHT = 1.4
  CARD_DEPTH = 0.05
  PIN_RADIUS = 0.08
  COL_W = 3.0 (日间距)
  ROW_H = 2.2 (同行间距)
```

## Constraints

- 单板块卡片数 < 30 时帧率 ≥ 55fps (默认 7 天)
- 单板块卡片数 < 100 时帧率 ≥ 30fps (60 天)
- dispose() 必须清理所有 geometry、material、texture，避免内存泄漏
- 卡片创建后位置固定（不跑物理模拟），只有 hover 时 Z 轴微动
- 红线是直线（CatmullRomCurve3 with 2 points），不做贝塞尔弯曲
