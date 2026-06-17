## MODIFIED Capability: detective-wall-scene

> 本 delta 在既有 `detective-wall-scene`（见 `detective-topic-wall` change 的 scene spec）基础上，叠加"侦探办公桌台灯下"的环境纵深层：前景桌面 + 远景墙 + 台灯道具 + 卷宗堆 + 半球光 + 环境贴图 + 方向性雾 + 尘埃微粒。
>
> **不变项**：卡片工作平面（`z≈0`）、布局算法、相机坐标系、交互、BFS、红线、全局 `FogSystem`、后处理链。卡片材质不注入方向雾（保持 `highlight/dim` 语义干净）。

## MODIFIED API: TopicWallScene

```typescript
class TopicWallScene {
  // 既有成员保留（scene/camera/renderer/css2d/cardGroup/redStrings/fog/
  //   composer/followLight/selectionLight），行为不变。

  /** 新增：程序化暖色环境贴图，影响 PBR 材质反射（图钉等金属）。 */
  readonly ambientEnv: AmbientEnv
  /** 新增：桌面 + 远景墙 + 台灯 + 卷宗堆，场景环境几何。loadBoardData 时重建。 */
  readonly setDressing: SetDressing | null
  /** 新增：台灯光锥内尘埃微粒。loadBoardData 时重建。 */
  readonly dust: DustParticles | null

  // loadBoardData 行为扩展（签名不变）：
  //   重建 setDressing（定位台灯/卷宗到当前 latestDayX/minX）
  //   重建 dust（以台灯位置为采样顶点）
  //   主 spotlight 从台灯灯罩位置射向今天列卡片中心
  //   方向雾 uFogOriginX = latestDayX
  loadBoardData(sections, relations, dateRange, days): void

  // 既有 setSelectionLight / addFrameCallback / startRenderLoop / onResize / dispose
  //   保留；render loop 追加 dust.update(dt)；dispose 追加 ambientEnv/setDressing/dust 清理。
}
```

## ADDED: SetDressing

```typescript
class SetDressing {
  /** 包含桌面、远景墙、台灯组、卷宗堆的 Group。 */
  readonly group: THREE.Group
  /** 台灯灯罩开口的世界坐标（主 spotlight 起点 / 尘埃采样顶点）。 */
  readonly lampPosition: THREE.Vector3
  /** 共享方向雾 uniform（注入到本组所有 MeshStandardMaterial）。 */
  readonly fogUniforms: {
    uFogOriginX: { value: number }
    uDirFogDensity: { value: number }
    uDirFogRange: { value: number }
    uDirFogColor: { value: THREE.Color }
  }

  constructor(opts: { latestDayX: number; minX: number; timelineWidth: number })

  /** 切换时间范围/板块后更新方向雾 origin（保留 density/range）。 */
  setFogOrigin(latestDayX: number): void

  dispose(scene: THREE.Scene): void
}
```

几何构成（均 `MeshStandardMaterial`，方向雾注入）：
- **桌面**：`PlaneGeometry`，`y=STYLE.desk.y`，`rotation.x=-π/2`，深胡桃木纹 `CanvasTexture`，`roughness 0.9`
- **远景墙**：`PlaneGeometry`，`z=STYLE.wall.farZ`，竖直，`STYLE.wall.farColor`，靠雾衰减
- **台灯组**（5 primitive）：底座/立柱（黄铜 metalness 0.85）、灯罩（墨绿玻璃 `STYLE.lamp.glass`）、灯泡（`emissive STYLE.lamp.bulb` × `STYLE.lamp.bulbEmissive` → Bloom 发光）。整组置于 `latestDayX + STYLE.lamp.offset.x`、桌面 `y`、`STYLE.lamp.offset.z`
- **卷宗堆**（2 叠 × 3-4 个错落 `BoxGeometry`）：纸色纹理，微旋转，分布在 `minX-1.5` 与 `latestDayX+4`

## ADDED: AmbientEnv

```typescript
class AmbientEnv {
  constructor(scene: THREE.Scene, renderer: THREE.WebGLRenderer)
  // PMREMGenerator.fromScene(程序化暖色场景)：
  //   深棕底 + 台灯位置暖色发光球 + 顶部冷光
  //   → scene.environment = envMap（不替换 background）
  dispose(): void
}
```

## ADDED: DustParticles

```typescript
class DustParticles {
  constructor(origin: THREE.Vector3)   // 台灯灯罩位置；椭球锥采样朝墙面延伸
  readonly points: THREE.Points
  update(dt: number): void             // 正弦漂移 + 越界 wrap
  dispose(scene: THREE.Scene): void
}
```

`PointsMaterial`：小圆点 `CanvasTexture`，`color STYLE.dust.color`，`size STYLE.dust.size`，`transparent`，`AdditiveBlending`，`depthWrite=false`，`count STYLE.dust.count`。

## ADDED: Directional Fog Shader 注入

```typescript
/** 给 MeshStandardMaterial 注入 X 方向雾项；共享 fogUniforms 一处更新全局生效。 */
function injectDirectionalFog(
  material: THREE.MeshStandardMaterial,
  fogUniforms: SetDressing['fogUniforms'],
): void
```

注入语义（见 design.md §Directional Fog）：
- 顶点输出世界 `x`（`vWorldX`）
- 片段在 `fog_fragment` 后叠加：`dx = uFogOriginX - vWorldX`；只对"过去"（`dx<0`）加重，今天/未来不动；`mix(fragColor, uDirFogColor, 1-exp(-density*clamp(-dx/range,0,1)))`

**作用对象**：桌面、远景墙、主软木墙、卷宗堆、台灯。**不含卡片**（保持 `CardGroup.highlight/dim` 语义）。与全局 `FogSystem`（空间纵深）叠加，二者同色 `#0a0f14`。

## MODIFIED: Style Constants 补充（types.ts STYLE）

```
desk:           { y: -1.6, zBack: -2, zFront: 12, color: '#3a2418' }
wall:           { backZ: -0.6, farZ: -4, farColor: '#06090c' }
lamp:           { offset:{x:2.8,z:5.2}, brass:'#b08d3f', glass:'#1f3d2e',
                  bulb:'#ffe9b0', bulbEmissive:1.5, spotColor:'#ffd9a0', spotIntensity:2.0 }
dossier:        { stackColor:'#e8d4a5' }
directionalFog: { density:1.2, range:12, color:'#0a0f14' }
dust:           { count:150, color:'#ffe9c8', size:0.06 }
lighting:       { ..., hemiSky:'#3a2a1a', hemiGround:'#0a0f14', hemiIntensity:0.55 }
```

## MODIFIED: Lighting（lighting.ts）

| 光源 | 变更 |
|------|------|
| 环境光 | `AmbientLight(#fff,0.15)` → `HemisphereLight(hemiSky, hemiGround, hemiIntensity)` |
| 主聚光 | 暖色 `STYLE.lamp.spotColor`，`STYLE.lamp.spotIntensity`，位置=台灯灯罩，`target`=今天列卡片中心，`angle 0.5 penumbra 0.6` |
| 跟随灯 | 色调微暖 `#fff0d0`，其余不变 |
| 选中灯 | 不变 |

## MODIFIED: Scene Construction（rebuildWall）

主软木墙 `z`：`-0.16` → `STYLE.wall.backZ (-0.6)`（卡片更多浮出厚度）。`rebuildWall` 的软木墙材质注入方向雾。

## Constraints（补充）

- 环境层几何总 draw call 增量 ≤ 12（桌面/远景墙/卷宗×6/台灯×5/尘埃×1）
- 方向雾 shader 注入仅增加常数级 ALU，不新增 pass
- `PMREMGenerator` 仅 constructor 一次性生成；不每帧
- `dispose()` 必须清理 env map、SetDressing 全部 geometry/material/texture、dust geometry/material
- 方向雾 `uFogOriginX` 在每次 `loadBoardData` 后更新（随今天列移动）
- 既有 perf 预算不变：7 天 < 30 卡片 ≥ 55fps；环境层增量不破坏该预算
