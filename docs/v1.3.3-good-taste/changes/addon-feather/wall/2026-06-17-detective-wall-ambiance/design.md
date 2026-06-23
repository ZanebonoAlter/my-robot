## Architecture Overview（叠加在既有 detective-wall-scene 之上）

本次改动**不重构既有架构**，只在 `TopicWallScene` 的场景构建阶段叠加一个"环境与纵深"层。Vue 层、交互层、相机层、数据层零改动。

```
TopicWallScene (既有，扩展)
  ├── AmbientEnv        (新增) → scene.environment = PMREM 程序化暖色 env
  ├── SetDressing       (新增) → Group: 桌面 + 远景墙 + 台灯 + 卷宗堆
  │     └── 方向雾 uniform 由 SetDressing 持有，注入到自身所有 StandardMaterial
  ├── CorkBoardWall     (既有，改：退到 z=-0.6)
  ├── CardGroup         (既有，不动) ← 卡片不受方向雾（保持 highlight 语义干净）
  ├── RedString         (既有，不动)
  ├── DustParticles     (新增) → Points，台灯光锥内 ~150 颗飘动
  ├── lighting          (改：半球光 + 台灯定向暖光；保留 follow/selection)
  └── PostProcessing    (既有，不动；Bloom 让台灯灯泡发光)
```

## 核心设计原则

1. **视角/相机/布局/交互零改动**——"面对软木墙钉卡片"的核心不变。所有新增内容都铺在前景与远景，不触碰卡片所在的 `z≈0` 工作平面。
2. **方向雾的边界**：只注入到"环境表面"（桌面、远景墙、主软木墙、卷宗堆、台灯）。**不注入卡片**——卡片的高亮/暗化已由 `CardGroup` 的 `highlight()/dim()/reset()` 管理，叠加方向雾会与生命线语义冲突，且违反"不动 CardGroup"的 surgical 约束。
3. **台灯是光的"动机"**：现有 spotlight 是凭空天光；本 change 让暖光从台灯灯罩射出，朝向墙面卡片中心，光从此"有出处"。

## 空间坐标系（既有 + 扩展）

```
既有（不变）：
  X = 时间（左=早，右=今天/晚），COL_W=3.0
  Y = 同日话题垂直排列（卡片中心在 y≈0 附近）
  Z = 景深（卡片 z≈0，jitter ±0.15）

新增（环境层）：
  桌面平面   y = -1.6, rotation.x = -π/2, 从 z=-2 延伸到 z=+12（前景）
  主软木墙   z = -0.6（从原 -0.16 后退，给卡片更多"浮出"厚度）
  远景墙     z = -4.0，竖直，暗色，靠全局 FogExp2 + 方向雾吃掉
  台灯       x = latestDayX + 2.8, 底座 y=-1.6, z=5.2（桌面右侧前景）
  卷宗堆     x = minX - 1.5 与 latestDayX+4 各一叠，桌面 y≈-1.4
```

`latestDayX` / `minX` 来自既有 `utils.latestDayX()` 与卡片布局结果，SetDressing 在 `loadBoardData` 时接收并定位，时间范围/板块切换时随场景重建自动 reflow。

## Component Design

### SetDressing（桌面 + 远景墙 + 台灯 + 卷宗堆）

```typescript
interface SetDressingOptions {
  latestDayX: number   // 今天列 X（台灯定位 + 方向雾 origin）
  minX: number         // 时间线左端 X（卷宗堆定位）
}

class SetDressing {
  readonly group: THREE.Group
  /** 共享方向雾 uniform（桌面/墙/卷宗/台灯材质引用同一对象） */
  readonly fogUniforms: { uFogOriginX: { value: number }; ... }

  constructor(opts: SetDressingOptions)
  /** 更新方向雾 origin（切换时间范围/板块后调用） */
  setFogOrigin(latestDayX: number): void
  dispose(scene: THREE.Scene): void
}
```

**桌面**（`PlaneGeometry`，大）：木纹 `CanvasTexture`（深胡桃色基底 + 纹理线 + 做旧磨损），`roughness 0.9`，方向雾注入。

**远景墙**（`PlaneGeometry`，竖直 `z=-4`）：极暗（`#06090c`）+ 轻微噪点纹理，方向雾注入，靠雾自然衰减。

**台灯**（banker's lamp，5 个 primitive 组合）：
```
底座   CylinderGeometry(r=0.45, h=0.12)    黄铜 metalness 0.85 roughness 0.35
立柱   CylinderGeometry(r=0.05, h=1.1)     黄铜
灯罩   CylinderGeometry(rTop=0.15,rBottom=0.45,h=0.45, openEnded) + 半圆切割
       墨绿玻璃 #1f3d2e emissive 弱，或黄铜；开口朝墙面（-z 侧削平）
灯泡   SphereGeometry(r=0.12)              emissive #ffe9b0 强度 1.5 → Bloom 发光
```
台灯整组 `group` 放在桌面，`position.set(latestDayX+2.8, -1.6, 5.2)`，轻微朝墙面倾斜。

**卷宗堆**（2 叠，各 3-4 个错落 BoxGeometry）：纸色 `CanvasTexture`（做旧 + 标签占位），微旋转 ±2°，方向雾注入。一叠在左侧 `x=minX-1.5`，一叠在右侧远景 `x=latestDayX+4, z=3`。

### AmbientEnv（环境贴图）

```typescript
class AmbientEnv {
  constructor(scene: THREE.Scene, renderer: THREE.WebGLRenderer)
  // 内部用 PMREMGenerator.fromScene(程序化暖色场景) 生成
  //   程序化场景：深棕底 + 台灯位置一个暖色发光球 + 顶部冷光
  //   → 图钉 metalness 0.7 反射出暖色高光
  dispose(): void
}
```

`scene.environment = envMap`（不替换 `scene.background`，背景仍是纯色 `#0a0f14`，由后处理 Vignette 收边）。env map 只影响 PBR 材质反射，不改变光照基底。

### DustParticles（光锥浮尘）

```typescript
class DustParticles {
  constructor(origin: THREE.Vector3)  // 台灯灯罩位置，椭球采样区域
  update(dt: number): void            // 正弦漂移 + 区域 wrap
  dispose(scene: THREE.Scene): void
}
```
- `BufferGeometry` 150 个点，初始位置在以台灯为顶点、朝墙面延伸的椭球锥内
- `PointsMaterial`：`map` = 小圆点 `CanvasTexture`，`color #ffe9c8`，`size 0.06`，`transparent`，`blending Additive`，`depthWrite false`
- 每帧：`y += sin(t*ω+phase)*amp*dt`，超出区域 wrap 回起点
- 配合 Bloom + Vignette，光中浮尘厚重感

### Directional Fog（shader 注入）

通过 `material.onBeforeCompile` 给 `MeshStandardMaterial` 注入 X 方向雾项。共享 `fogUniforms` 对象，所有注入材质引用同一份，更新 `uFogOriginX` 一次即全局生效。

**注入点**：
```glsl
// vertex — #include <common> 后追加
varying float vWorldX;
// project_vertex 之后
vWorldX = (modelMatrix * vec4(transformed, 1.0)).x;

// fragment — #include <common> 后追加
varying float vWorldX;
uniform float uFogOriginX;    // 今天列 X
uniform float uDirFogDensity; // 方向雾强度（~1.2）
uniform float uDirFogRange;   // 衰减距离（timelineWidth 或固定 12）
uniform vec3  uDirFogColor;   // #0a0f14（与全局雾同色）

// fragment — #include <fog_fragment> 之后追加（只对"过去"加重，今天/未来不动）
float dx = uFogOriginX - vWorldX;            // 左(过去)<0，右(今天/未来)>0
float past = clamp(-dx / uDirFogRange, 0.0, 1.0);
float dirFactor = 1.0 - exp(-uDirFogDensity * past);
gl_FragColor.rgb = mix(gl_FragColor.rgb, uDirFogColor, dirFactor);
```

**uniform 更新**：`loadBoardData` / 时间范围切换后，`setDressing.setFogOrigin(latestDayX)`。生命周期模式（`fog.disable()`）不影响方向雾——方向雾是"时间叙事"，不是"遮蔽窗口外"，二者语义不同，lifecycle 模式全局雾关掉但方向雾保留可强化"沿时间线追溯"。

> 注：方向雾与全局 `FogExp2` 叠加（一个管空间纵深、一个管时间纵深），两者都用 `#0a0f14`，视觉上统一。

## Lighting 重构（`lighting.ts`）

| 光源 | 既有 | 本 change |
|------|------|-----------|
| 环境光 | `AmbientLight(#fff, 0.15)` 纯白平光 | `HemisphereLight(sky #3a2a1a 暖, ground #0a0f14 冷, 0.55)` |
| 主聚光 | `SpotLight(#fff, 1.2)` 在 `(0,12,6)` 凭空天光 | `SpotLight(#ffd9a0 暖, 2.0)` 在**台灯灯罩位置**，`target` 指向今天列卡片中心，`angle 0.5 penumbra 0.6` |
| 跟随灯 | `PointLight(#fff4e6, 0.6)` 跟相机 | 保留，色调微调暖 `#fff0d0` |
| 选中灯 | 红色 `PointLight` 聚焦卡片上方 | 不动 |

主 spotlight 的位置/target 由 SetDressing 提供台灯世界坐标，`loadBoardData` 后接线。

## TopicWallScene 集成改动

```typescript
// constructor
this.ambientEnv = new AmbientEnv(this.scene, this.renderer)
this.dust = null  // 随数据建

// loadBoardData（在现有 rebuildWall 之后）
this.setDressing?.dispose(this.scene)
this.setDressing = new SetDressing({ latestDayX, minX })
this.scene.add(this.setDressing.group)
this.dust?.dispose(this.scene)
this.dust = new DustParticles(this.setDressing.lampPosition)
this.scene.add(this.dust.points)
// 接线主 spotlight 到台灯
this.repositionMainSpot(this.setDressing.lampPosition, latestDayX)

// 渲染循环
this.dust?.update(dt)

// dispose
this.ambientEnv.dispose(); this.setDressing?.dispose(this.scene); this.dust?.dispose(this.scene)
```

软木墙 `rebuildWall` 的 `z` 从 `-0.16` 改为 `-0.6`（给卡片更多浮出厚度；CSS2D/交互不受影响，因卡片仍按布局位置渲染）。

## STYLE 常量扩展（`types.ts`）

```typescript
desk:    { color: '#3a2418', y: -1.6, zBack: -2, zFront: 12 }
wall:    { backZ: -0.6, farZ: -4, farColor: '#06090c' }
lamp:    { offset: { x: 2.8, z: 5.2 }, brass: '#b08d3f', glass: '#1f3d2e',
           bulb: '#ffe9b0', bulbEmissive: 1.5, spotColor: '#ffd9a0', spotIntensity: 2.0 }
dossier: { stackColor: '#e8d4a5' }
directionalFog: { density: 1.2, range: 12, color: '#0a0f14' }
dust:    { count: 150, color: '#ffe9c8', size: 0.06 }
lighting:{ ...existing, hemiSky: '#3a2a1a', hemiGround: '#0a0f14', hemiIntensity: 0.55 }
```

## Performance 预算

| 项 | 增量 | 说明 |
|----|------|------|
| Draw call | +~10 | 桌面/远景墙/卷宗×6/台灯×5/尘埃×1，全部静态 |
| Shader ALU | +~6 ops/frag | 方向雾注入，仅环境表面材质 |
| PMREM | 一次性 | constructor 生成，不每帧 |
| 尘埃 | 150 点 1 draw | 每帧 CPU 更新 150×3 float，可忽略 |
| 内存 | env map (~1MB) + 几张 CanvasTexture | dispose 清理 |

守住既有预算：默认 7 天 < 30 卡片 ≥ 55fps。

## 边界（明确不做）

- 不改相机坐标系、DirectorCamera/WallCameraControls 行为
- 不改布局算法、BFS、红线、迷雾系统全局行为
- 不改 CardGroup（卡片无方向雾，保持 highlight 语义）
- 不改后端/API/数据层
- 不加阴影（`castShadow` 默认关，避免 perf 与配置复杂度；光锥感由 spotlight penumbra + 尘埃表达）
- DoF（景深）属 Tier 3，本 change 不含
