## Context

- 日报在 `TagsPage.vue` 渲染，核心组件 `BoardDailyReportTimeline` 接收 `boardId`，内部用 `useDailyReportReader(boardId)` 管理 `currentDayIndex` / `selectedDetail`
- 版块切换基础设施**已存在**：`useTagsPage()` 暴露 `boards` / `selectedBoardId` / `handleSelectBoard`；`getBoards()` API 可取版块列表；切版块时 `selectedBoardId` 变化，`BoardDailyReportTimeline` 已绑定 `:board-id`
- 切日期 = 改 `currentDayIndex`（已有）；切版块 = 改 `selectedBoardId`（已有，但需在日报视图内就近触发）
- `gsap ^3.15.0` 已是项目依赖；已用临时 `/demo/peel-transition` 验证 peel 翻页效果可行、手感达标
- 现有日报为杂志风纯 CSS 排版（Masthead + TopicSection + Highlights），无 canvas 特效，转场仅作用于切换瞬间

## Goals / Non-Goals

**Goals:**

- 切日期 → 横向 peel 翻页；切版块 → 纵向 peel 翻页（语义可区分，用户凭方向即可感知"翻日子"还是"换频道"）
- 日报视图内可就近切换版块，无需退出回侧栏
- 转场组件可复用（与日报解耦，其他视图能用）
- 全浏览器兼容，无实验性 API 依赖
- 仅在切换瞬间触发，不持续干扰阅读

**Non-Goals:**

- 不做 html-in-canvas / Canvas UI 那类实验性特效（兼容性差，已否决）
- 不改日报的数据层、排版结构、后端
- 不在内容区加鼠标驱动的覆盖型特效（破坏阅读）
- 不做整页 3D 翻转（过度）

## Decisions

1. **技术栈**：GSAP + Vue `<Transition :css="false">` 的 JS hooks（`enter` / `leave`），禁用 CSS transition 避免与 GSAP 冲突
2. **peel 实现**：`clip-path inset` + `transform: translate + rotateX/Y` + 容器 `perspective`；`transform-origin` 在卷起边；进出方向相反；距离 / 角度 / 时长参数化（demo 验证值：dist=64、rot=16°、enter 0.55s `power3.out` / leave 0.5s `power2.in`）
3. **方向判定**：在 `BoardDailyReportTimeline` 内 watch `boardId` 变化→`vertical`、`currentDayIndex` 变化→`horizontal`；在触发 key 变更**之前**同步写入 direction，保证 Transition 拿到正确方向
4. **布局**：转场容器 `position: relative` + 子项 `absolute inset-0` + 固定 `min-height`，缓解 `<Transition>` 同时模式（非 `out-in`）下新旧页叠放的塌陷
5. **版块切换 UI**：日报区头部 segmented control（按钮条），数据来自 `boards`（`useTagsPage`），点击调用 `handleSelectBoard`，当前版块高亮
6. **复用**：不改 `useDailyReportReader` / `useTagsPage` 的数据层接口

## Risks / Trade-offs

- **同时模式布局塌陷** → absolute + min-height 缓解（demo 已验证可行）
- **切版块时 reader 需随 boardId 变化自动 reload** → 依赖现有行为；apply 时验证，若缺则补 `watch(boardId) → loadReports + reset dayIndex`（现有侧栏切版块功能可用，推断已具备）
- **方向判定依赖时序** → 用同步赋值（先设 direction 再改 key）确保正确，并在 animating 锁期间防越界连点
- **性能** → 全 GPU 友好属性（transform / opacity / clip-path），无 layout thrash；日报内容体量小，无压力
- **降级** → 即便浏览器对 clip-path/3d transform 支持不全，最差退化为无动画直接切换，不报错、不影响可读性
