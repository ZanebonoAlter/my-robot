## Capability

导演相机系统，负责 Three.js 相机的运镜控制和板块切换转场动画。所有相机运动由 GSAP Timeline 编排。

## API

### DirectorCamera

```typescript
interface DirectorCameraHooks {
  onTransitionStart?: () => void                         // 运镜开始：orbit 禁用
  onTransitionComplete?: (shot: CameraShot) => void      // 运镜完成：orbit 重启用
  onTargetUpdate?: (x: number, y: number, z: number) => void  // 每帧同步 orbit target
}

class DirectorCamera {
  constructor(camera: THREE.PerspectiveCamera)
  hooks: DirectorCameraHooks   // 由 WallCameraControls 注入

  // 当前镜头
  currentShot: CameraShot

  // 预设镜头
  todayFocus(dateX: number): CameraShot
  overview(totalWidth: number): CameraShot
  topicFocus(card: PinCard): CameraShot
  lifecycleFull(totalWidth: number): CameraShot

  // 运镜
  transitionTo(shot: CameraShot): gsap.core.Timeline
  // 返回 Timeline；transitionTo/lookProxy.onUpdate/snapTo 均触发 hooks 以同步 orbit

  // 即时跳转（无动画，初始化用）
  snapTo(shot: CameraShot): void
}

interface CameraShot {
  position: THREE.Vector3
  target: THREE.Vector3
  fov: number
  duration: number    // 秒
  ease: string        // GSAP easing 字符串
}
```

### 预设镜头参数

```
todayFocus(dateX):
  position: (dateX, 5.5, 8)
  target: (dateX, 0, 0)
  fov: 50
  duration: 0.6
  ease: 'power2.inOut'
  → 相机在"今天"正前方，略高，微微俯视

overview(totalWidth):
  position: (totalWidth / 2, 12, 16)
  target: (totalWidth / 2, 0, 0)
  fov: 55
  duration: 0.8
  ease: 'power2.inOut'
  → 居中鸟瞰，看到完整时间跨度

topicFocus(card):
  position: (card.x, card.y + 1, 5)
  target: (card.x, card.y, 0)
  fov: 45
  duration: 0.5
  ease: 'power2.out'
  → 靠近目标卡片，略有偏移角度

lifecycleFull(totalWidth):
  position: (totalWidth / 2, 8, 14)
  target: (totalWidth / 2, 0, 0)
  fov: 60
  duration: 0.8
  ease: 'power2.inOut'
  → 比总览略近，看到完整生命线
```

### 运镜约束

```
规则:
  • 所有相机运动必须通过 transitionTo()，禁止直接修改 camera.position
  • transitionTo 在执行中如果被再次调用，前一个 Timeline 自动 kill
  • camera.lookAt(target) 在 GSAP 的 onUpdate 中调用，跟随 target 插值
  • fov 变化不能超过 35~65 范围
  • 相机 Y 值不能低于 2（不能跑到卡片平面以下）
  • transitionTo/snapTo 触发 hooks：onTransitionStart（orbit 禁用）→
    onTargetUpdate（每帧同步 orbit target）→ onTransitionComplete（orbit 重启用）
```

## Wall Camera Controls（OrbitControls 平移/缩放）

> 侦探墙是 2.5D 软木墙隐喻，相机只需沿墙平移 + 缩放，不需旋转（避免翻到墙后迷失）。

### WallCameraControls

```typescript
class WallCameraControls {
  constructor(
    camera: THREE.PerspectiveCamera,
    domElement: HTMLElement,
    directorCamera: DirectorCamera,
    hooks?: { onInteractStart?: () => void; onInteractEnd?: () => void }
  )
  readonly controls: OrbitControls   // three/examples/jsm/controls/OrbitControls.js
  update(): void                     // 每帧调用（注册到 scene.addFrameCallback）
  dispose(): void
}
```

### 配置

```
OrbitControls 配置:
  enableRotate: false              // 禁旋转，保持轴测"看墙"视角
  enablePan: true                  // 左键拖拽平移
  enableZoom: true                 // 滚轮缩放
  mouseButtons: { LEFT: MOUSE.PAN, MIDDLE: MOUSE.DOLLY, RIGHT: MOUSE.PAN }
  minDistance: 3                   // 缩放下限
  maxDistance: 40                  // 缩放上限
```

### 与 DirectorCamera 的协调（核心难点）

```
问题: DirectorCamera 的 GSAP transition 直接写 camera.position + lookAt；
      OrbitControls 用自己的 target 重算 position。两者会互相打架——
      transitionTo 后第一次 orbit.update() 会把相机拉回 orbit.target 附近。

协调（通过 DirectorCameraHooks）:
  onTransitionStart   → controls.enabled = false（运镜期间禁用 orbit）
  onTargetUpdate(x,y,z) → controls.target.set(x,y,z)（运镜中同步 target）
  onTransitionComplete → controls.enabled = true（重启用）
```

### 拖拽期间暂停 hover

```
OrbitControls 'start' 事件 → interaction.setHoverSuspended(true)（拖拽时不 raycast）
OrbitControls 'end' 事件   → interaction.setHoverSuspended(false)
→ 避免拖拽平移时卡片 hover 动画乱跳
```

### 渲染循环

```
scene.addFrameCallback(() => controls.update())  // 每帧 update 才能应用 pan
→ TopicWallScene.tick 在 composer.render 前调用所有 frameCallbacks
```

### 依赖

- `@types/three`（devDep，已安装）：补充 OrbitControls/CSS2DRenderer/Line2 的 jsm 类型。
  注：早期存在的 `front/three.d.ts`（`declare module 'three'`）会遮蔽 @types/three 使
  全部 three 导入变 any，已在 1.1 任务删除；@types/three 是补充类型不遮蔽。

## Chapter Transition

> **当前状态**：本 change 无 BoardSelector 入口，板块切换转场暂未启用。
> `ChapterTransition.ts` 类实现保留（供后续补 BoardSelector 时复用），但 Vue 层的
> `watch(boardId)`、wipe/cover DOM、ChapterTransition 实例化均已移除（避免死代码）。
> 以下设计文档保留，待 BoardSelector 落地时按此实现。

### ChapterTransition

```typescript
class ChapterTransition {
  constructor(
    overlayEl: HTMLElement,  // Vue 渲染的转场 DOM 容器
    directorCamera: DirectorCamera,
    scene: TopicWallScene
  )

  play(boardData: {
    name: string
    dateRange: string
    topicCount: number
    categoryBreakdown?: { label: string; count: number }[]
  }): gsap.core.Timeline

  // 播放中检查
  readonly isPlaying: boolean
  kill(): void
}
```

### 转场 Phase 细节

```
Phase 1: 红色 wipe (0~0.15s)
  DOM: <div class="chapter-wipe"> 红色条带
  GSAP: gsap.fromTo(wipeEl, { x: '-100%' }, { x: '100%', duration: 0.15 })
  颜色: #DC2626

Phase 2: 档案封面 (0.15~0.65s)
  DOM: <div class="chapter-cover">
    ├── <span class="cover-stamp">CONFIDENTIAL</span>
    ├── <div class="cover-title">{{ boardName }}</div>
    ├── <div class="cover-meta">
    │     时间窗口: {{ dateRange }}
    │     活跃话题: {{ topicCount }}
    │     分类统计: ...
    │   </div>
    └── <span class="cover-classified">CLASSIFIED {{ date }}</span>

  动画:
    - 封面整体: fromTo opacity 0→1, scale 0.95→1
    - 标题: 打字机效果 (逐字出现, GSAP stagger per char, 0.03s/字)
    - meta: fadeIn stagger
    - CONFIDENTIAL 印章: 从 scale 1.5 → 1 with rotation (盖章动效)

  样式:
    font-family: 'JetBrains Mono', 'Courier New', monospace
    background: #FFFBEB
    border: 1px solid rgba(26, 26, 26, 0.15)
    transform: rotate(2deg)
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.4)

Phase 3: 退场 + 新场景 (0.65~1.0s)
  DOM: 封面 fadeOut
  3D:
    - 清空旧卡片
    - 加载新板块数据
    - 相机 transitionTo(todayFocus)
    - 卡片 staggerEntrance(0.03)
    - 红线 stagger draw(0.02)
```

### GSAP Timeline 编排

```typescript
play(boardData): gsap.core.Timeline {
  const tl = gsap.timeline()

  tl
    // Phase 1
    .fromTo(this.wipeEl,
      { xPercent: -100 },
      { xPercent: 100, duration: 0.15, ease: 'power2.in' }
    )

    // Phase 2
    .set(this.coverEl, { display: 'flex', opacity: 0, scale: 0.95 })
    .to(this.coverEl, { opacity: 1, scale: 1, duration: 0.15, ease: 'back.out(1.2)' })
    .add(() => this.typewriterReveal(boardData))
    .to({}, { duration: 0.2 }) // 停顿

    // Phase 3
    .to(this.coverEl, { opacity: 0, scale: 1.02, duration: 0.15 })
    .call(() => this.scene.clearScene())
    .call(() => this.scene.loadBoardData(newData))
    .add(() => {
      const shot = this.directorCamera.todayFocus(todayX)
      this.directorCamera.transitionTo(shot)
    }, '<')
    .add(() => this.scene.cardGroup.staggerEntrance(0.03), '<0.05')
    .add(() => this.scene.staggerDrawStrings(0.02), '<0.1')

  return tl
}
```

## Constraints

- 转场期间禁止用户交互（Raycaster disable）
- 转场 Timeline 被 kill 时必须清理中间状态（如果 coverEl 半透明，reset 到 hidden）
- 打字机效果的文字不使用 GSAP TextPlugin（避免额外依赖），用 `substring` + `onUpdate` 手动实现
- 转场 DOM 由 Vue 渲染，ChapterTransition 只操作样式和动画，不操作 DOM 结构
