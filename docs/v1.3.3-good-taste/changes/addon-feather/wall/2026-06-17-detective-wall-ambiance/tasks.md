# Tasks — detective-wall-ambiance

> 实现侦探照片墙的环境纵深与氛围层（"侦探办公桌台灯下"）。
> 原则：surgical——不改相机/布局/交互/数据层；卡片不注入方向雾。
> 归档前重跑 §10 验证节，确认零失败（§11.4）。

## 1. 样式常量扩展（types.ts）

- [x] 1.1 `STYLE` 补 `desk`（y/zBack/zFront/color）、`wall`（backZ=-0.6/farZ=-4/farColor）、`lamp`（offset/brass/glass/bulb/bulbEmissive/spotColor/spotIntensity）、`dossier`（stackColor）、`directionalFog`（density/range/color）、`dust`（count/color/size）常量组。
  - verify: `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"` 0 error。
- [x] 1.2 `STYLE.lighting` 补 `hemiSky/hemiGround/hemiIntensity`，保留既有 `ambient/spotAngleDeg/spotPenumbra`（lighting.ts 会改用半球光，但常量保留不删以免破坏引用）。
  - verify: 同 1.1 typecheck。

## 2. 方向雾 shader 注入（shaders/directionalFog.ts）

- [x] 2.1 新建 `detective-wall/shaders/directionalFog.ts`，导出 `injectDirectionalFog(material, fogUniforms)`：patch `MeshStandardMaterial` 的顶点 `#include <common>`（加 `varying float vWorldX`）+ 片段 `#include <common>`（加 varying + 4 个 uniform 声明）+ `#include <fog_fragment>` 之后追加方向雾 mix（只对 `dx<0` 过去加重）。
  - verify: typecheck。
- [x] 2.2 抽纯函数 `directionalFogFactor(worldX, originX, density, range): number`（与 shader 公式一致：`1 - exp(-density * clamp((originX-worldX)/range,0,1))`），放 `shaders/directionalFog.ts` 导出供单测。
  - verify: 见 §9 单测。

## 3. SetDressing（桌面 + 远景墙 + 台灯 + 卷宗堆）

- [x] 3.1 新建 `detective-wall/SetDressing.ts`：`class SetDressing { group; lampPosition; fogUniforms }`，`constructor({latestDayX, minX, timelineWidth})` 构建桌面（PlaneGeometry 水平 + 木纹 CanvasTexture）、远景墙（z=farZ 竖直暗色）、台灯组（5 primitive：底座/立柱黄铜 + 墨绿灯罩 + emissive 灯泡）、2 叠卷宗堆。台灯定位 = `latestDayX+lamp.offset.x`。
  - verify: typecheck + 浏览器肉眼。
- [x] 3.2 SetDressing 内对所有 `MeshStandardMaterial` 调 `injectDirectionalFog(mat, this.fogUniforms)`；`lampPosition` 灯罩开口世界坐标返回。
  - verify: typecheck。
- [x] 3.3 抽纯函数 `lampPositionFor(latestDayX, offset, deskY): {x,y,z}`（台灯定位逻辑，与 shader 无关），单测覆盖。
  - verify: §9 单测。
- [x] 3.4 `setFogOrigin(latestDayX)` 更新 `fogUniforms.uFogOriginX.value`；`dispose(scene)` 移除 group 并 dispose 全部 geometry/material/texture（含 CanvasTexture）。
  - verify: typecheck。

## 4. AmbientEnv（环境贴图）

- [x] 4.1 新建 `detective-wall/AmbientEnv.ts`：`class AmbientEnv { constructor(scene, renderer) }`，内部 `PMREMGenerator.fromScene(程序化暖色场景)`（深棕底 + 台灯位置暖色发光 Mesh + 顶部冷光），结果赋 `scene.environment`；`dispose()` 释放 PMREM + envMap。
  - verify: typecheck + 浏览器肉眼（图钉有暖色反射高光）。
  - 注意：不替换 `scene.background`（背景保持纯色）。

## 5. DustParticles（光锥浮尘）

- [x] 5.1 新建 `detective-wall/DustParticles.ts`：`class DustParticles { points; constructor(origin) }`，150 个点椭球锥采样（以台灯为顶点朝墙面延伸），`PointsMaterial` 小圆点 CanvasTexture + additive + depthWrite false。
  - verify: typecheck。
- [x] 5.2 `update(dt)`：每点 y 正弦漂移 + 越界 wrap 回起点；`dispose(scene)` 释放 geometry/material/texture。
  - verify: 浏览器肉眼（光中浮尘缓慢飘动）。

## 6. 光照重构（lighting.ts）

- [x] 6.1 `setupLighting`：`AmbientLight` → `HemisphereLight(hemiSky, hemiGround, hemiIntensity)`；主 `SpotLight` 改暖色 `lamp.spotColor` + `lamp.spotIntensity`（位置/target 由 TopicWallScene 在 loadBoardData 接线到台灯，初始仍设合理默认）；`followLight` 色调改 `#fff0d0`；`selectionLight` 不变。返回 `{ spot, followLight, selectionLight }`（spot 需外部重定位，故返回）。
  - verify: typecheck + 浏览器肉眼（暖光从台灯方向打来，上下色温差异）。

## 7. TopicWallScene 集成

- [x] 7.1 constructor：新增 `this.ambientEnv = new AmbientEnv(scene, renderer)`；`setupLighting` 返回值取 `spot`（新）+ followLight + selectionLight；`setDressing/dust` 初始 null。
  - verify: typecheck。
- [x] 7.2 `rebuildWall`：软木墙 `z` 由 `-0.16` 改 `STYLE.wall.backZ`，其材质调 `injectDirectionalFog`。
  - verify: typecheck + 浏览器（卡片浮出厚度增加）。
- [x] 7.3 `loadBoardData`：在 `rebuildWall`/redStrings 之后，按当前数据 `latestDayX`/`minX`/`timelineWidth` 重建 `setDressing`（dispose 旧的）与 `dust`（dispose 旧的）；主 `spot.position` 设到 `setDressing.lampPosition`、`spot.target.position` 设到今天列卡片中心 `(latestDayX, 0, 0)`、`spot.target.updateMatrixWorld()`。
  - verify: typecheck + 浏览器（切时间范围/板块，台灯与光锥随今天列移动）。
- [x] 7.4 渲染循环：`tick` 内追加 `this.dust?.update(dt)`（在 composer.render 之前）。
  - verify: typecheck。
- [x] 7.5 `dispose`：追加 `ambientEnv.dispose()`、`setDressing?.dispose(scene)`、`dust?.dispose(scene)`。
  - verify: typecheck + 多次进出侦探墙无内存泄漏（肉眼/控制台无 WebGL 警告堆积）。

## 8. 配置/接入确认

- [x] 8.1 确认 `PMREMGenerator`/`HemisphereLight`/`Points`/`PointsMaterial` 均属已装 `three`，无新增依赖；`pnpm-lock.yaml` 不变。
  - verify: `cd front && git diff pnpm-lock.yaml package.json` → 空。

---

## 9. 测试

- [x] 9.1 `detective-wall/utils.test.ts` 不受影响（本 change 不改 utils.ts 纯函数），回归确认 15 测试仍全过。
  - verify: `cd front && pnpm test:unit -- detective-wall`。
- [x] 9.2 新建 `detective-wall/ambiance.test.ts`：
  - `directionalFogFactor`：origin 处 factor=0；worldX 远小于 origin → 接近 1-exp(-density)；worldX>origin（未来）→ 0；range 边界 clamp 正确（4 测试）。
  - `lampPositionFor`：latestDayX+offset.x 正确、deskY 传入、z=offset.z（2 测试）。
  - verify: `cd front && pnpm test:unit -- detective-wall` → 全过（原 15 + 新 6 = 21）。

## 10. 文档

- [x] 10.1 更新 `docs/reference/architecture/frontend.md` §3D 侦探墙：补"环境纵深层"子节（桌面/远景墙/台灯/环境贴图/方向雾/尘埃 + 方向雾边界说明：不注入卡片）。
  - verify: 文档内 grep "方向雾"/"台灯" 有命中。
- [x] 10.2 proposal/design/specs delta 与实现一致（无遗留占位文字、无 `TODO`）。
  - verify: `grep -rn "TODO\|占位" openspec/changes/detective-wall-ambiance/` → 仅本 tasks 验证条目自身。

---

## 11. 验证（归档门禁，重跑确认零失败）

> 前端编译类命令必须走 Windows cmd（WSL 缺 native-binding）；lint 可 WSL。

- [x] 11.1 `cd front && pnpm lint` → 0 error（detective-wall 零 warning）。
- [x] 11.2 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"` → 0 error。
- [x] 11.3 `cd front && pnpm test:unit -- detective-wall` → PASS（21 测试）。
- [x] 11.4 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"` → 构建成功。
- [ ] 11.5 浏览器人工验证：进入侦探墙 → 看到前景桌面 + 台灯暖光锥 + 图钉金属反光 + 光中浮尘 + 往左越早越浓的方向雾；切 7/14/30 天与板块，台灯/光锥/雾 origin 随今天列移动；hover/BFS/详情面板/ESC/完整生命周期等既有交互无回归。
- [x] 11.6 `git diff --stat` 确认仅触碰本 change 列出文件（detective-wall/ 新增 4 + 改 3；docs/frontend.md；ambiance.test.ts），不触碰 CardGroup/RedString/FogSystem/DirectorCamera/WallCameraControls/InteractionLayer/utils/ChapterTransition/后端。
