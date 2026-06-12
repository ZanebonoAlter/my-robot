## Capability

导演相机系统，负责 Three.js 相机的运镜控制和板块切换转场动画。所有相机运动由 GSAP Timeline 编排。

## API

### DirectorCamera

```typescript
class DirectorCamera {
  constructor(camera: THREE.PerspectiveCamera, scene: TopicWallScene)

  // 当前镜头
  currentShot: CameraShot

  // 预设镜头
  todayFocus(dateX: number): CameraShot
  overview(totalWidth: number): CameraShot
  topicFocus(card: PinCard): CameraShot
  lifecycleFull(totalWidth: number): CameraShot

  // 运镜
  transitionTo(shot: CameraShot): gsap.core.Timeline
  // 返回 Timeline，外部可以 .then() 或组合

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
```

## Chapter Transition

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
