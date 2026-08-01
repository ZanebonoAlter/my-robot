## Why

日报视图切换日期或版块时目前是硬切，没有过渡，阅读连续感被打断；更关键的是想换一个版块看它的日报，得先退出日报视图、回左侧栏选版块、再进日报——心智消耗大。给日报加一个轻量的**方向化翻页转场**（切日期横向翻、切版块纵向翻），并支持**在日报视图内就近切换版块**，能显著提升操作流畅度与阅读连续性。

## What Changes

- 新增可复用的方向化翻页转场能力（composable + 组件），基于项目已装的 GSAP，全浏览器兼容、零实验性 API
- 改造日报视图：日报内容切换时走 peel 翻页转场，方向由触发动作决定（切日期→横向、切版块→纵向）
- 在日报视图头部新增就近的版块切换条（segmented control），复用现有 `selectedBoardId` / `handleSelectBoard` 数据层
- 删除验证用的临时 demo 页 `/demo/peel-transition`
- **不改**后端、数据模型、API、配置、部署

## Capabilities

### New Capabilities

- `peel-transition`: 可复用的方向化 GSAP 翻页转场（composable + 组件），支持 `horizontal` / `vertical` 两种语义方向，参数化距离 / 旋转角度 / 时长 / 缓动

### Modified Capabilities

> 无。本 change 是前端视觉 / 交互增强，不改变任何现有业务 spec 的 requirement（业务不变量、数据契约、链路规则均不动）。

## Impact

- **前端**：`front/app` 下新增 composable 与组件；改造 `BoardDailyReportTimeline.vue`；删除临时 demo 页
- **后端 / 数据库 / API / 配置 / 部署**：无影响
- **依赖**：复用已装的 `gsap ^3.15.0`，不引入新依赖
