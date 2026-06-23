## Why

侦探照片墙（`detective-wall-scene`）的卡片 dossier 纹理、后处理链、光照接线已相当扎实，但**整体三维场景纵深与环境设计仍粗糙**：当前本质上是一块"竖在黑屋子里的软木广告牌"——只有一面 `z≈0` 的墙 + 卡片，无地板/桌面、无远景层次、无金属环境反射、光是平的、雾是均匀的。缺乏沉浸感与空间参考系。

本次把它升级为**"侦探办公桌台灯下"**的沉浸场景：台灯照亮"今天"的卷宗，往左是越翻越旧、越没入迷雾的旧案档案。这与 Syntopica"抽丝剥茧"的产品理念完全契合——时间纵深=叙事纵深。

## What Changes

核心原则：**保持"面对软木墙钉卡片"的视角、相机坐标系、布局算法、交互逻辑、数据层完全不变**（surgical），只在场景视觉层增加纵深与环境。相机/布局/交互/BFS/红线/迷雾系统行为不受影响。

- **前景桌面**：水平大平面铺到卡片底部以下（`y≈-1.6`），深色木纹纹理，从墙面延伸到相机前方，建立空间纵深的参考系
- **远景墙**：现有软木公告板后退到 `z=-0.6`，在其后方 `z≈-4` 补一面更暗的远景墙，靠全局迷雾自然吃掉，形成层次
- **台灯道具**（banker's lamp：底座 + 黄铜/墨绿灯罩 + 灯泡）：放在桌面右侧前景，成为 spotlight 的**视觉来源**（光有"动机"），暖色光锥照亮墙面卡片；灯泡 emissive 让 Bloom 自然发光
- **卷宗堆**：桌面左/远处 1-2 叠案卷（BoxGeometry + 纸纹理），强化"办公桌"语义并作远景遮挡
- **半球光替换平光**：`AmbientLight(白,0.15)` → `HemisphereLight(暖天/冷地)`，上下色温差异带出空气感
- **环境贴图**：`PMREMGenerator` 从程序化暖色场景生成 env map 赋给 `scene.environment`，图钉金属（metalness 0.7）从此反射出暖色高光，从"塑料球"变"金属球"
- **方向性雾（过去浓/今天清）**：保留全局 `FogExp2` 作基底氛围，叠加用 `material.onBeforeCompile` 注入的 X 方向雾项——越往左（越早的日期）越浓，"今天"列（`uFogOriginX = latestDayX`）附近最清。注入到所有 `MeshStandardMaterial`（桌面/墙/卡片/台灯统一）
- **光锥中尘埃微粒**：~150 个 sprite 在台灯光锥范围内缓慢飘动，配合 Vignette + Bloom 形成"档案室浮尘"的厚重感

## Capabilities

### Modified Capabilities

- `detective-wall-scene`：场景几何（桌面/远景墙/台灯道具/卷宗堆）、光照（半球光 + 台灯定向暖光）、环境贴图、方向性雾 shader 注入、尘埃微粒系统。API 扩展与样式常量补充见 specs delta。

## Impact

- **新增文件**：
  - `features/tags/components/detective-wall/SetDressing.ts`（桌面 + 远景墙 + 台灯 + 卷宗堆的创建与销毁，返回 Group 引用供场景管理）
  - `features/tags/components/detective-wall/AmbientEnv.ts`（PMREM 环境贴图生成 + `scene.environment` 设置 + dispose）
  - `features/tags/components/detective-wall/DustParticles.ts`（尘埃 Points 系统：生成/每帧漂移/销毁）
  - `features/tags/components/detective-wall/shaders/directionalFog.ts`（`onBeforeCompile` 注入函数 + uniform 管理，给 MeshStandardMaterial 加 X 方向雾项）
- **修改文件**：
  - `detective-wall/TopicWallScene.ts`：constructor 设置环境贴图；`loadBoardData` 构建 SetDressing；软木墙退到 `z=-0.6`；渲染循环更新尘埃 + 上传方向雾 uniform（`uFogOriginX` 随时间范围/今天列更新）；`dispose` 清理新资源
  - `detective-wall/lighting.ts`：`AmbientLight` → `HemisphereLight`；overhead `SpotLight` 改为从台灯灯罩位置发出的暖色定向光；保留 `followLight`（调暖）/`selectionLight`
  - `detective-wall/types.ts`：`STYLE` 补 `desk` / `lamp` / `directionalFog` 常量组
- **不改动**：`CardGroup.ts`、`RedString.ts`、`FogSystem.ts`（全局基底雾保留）、`DirectorCamera.ts`、`WallCameraControls.ts`、`InteractionLayer.ts`、`utils.ts`（布局/BFS）、`ChapterTransition.ts`、Vue 容器、后端、API、数据层
- **无依赖变更**：`PMREMGenerator` / `HemisphereLight` / `Points` 均属已安装的 `three`
- **性能考量**：
  - 新增几何多为静态大平面 + 少量道具 primitive，draw call 增加可控（桌面/远景墙/卷宗各 1，台灯 ~5）
  - 尘埃 150 个 Points 单次 draw call
  - `onBeforeCompile` 注入不增加 pass、不增加 draw call，仅每材质多几条 ALU
  - 守住既有 perf 预算：默认 7 天 < 30 卡片 ≥ 55fps；方向性雾统一注入不破坏该预算
