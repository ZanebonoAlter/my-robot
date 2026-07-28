## 1. 可复用转场 composable 与组件

- [ ] 1.1 新增 `usePeelTransition` composable：封装 GSAP enter/leave，参数化 `dist` / `rot` / `duration` / `ease`，返回 `onEnter` / `onLeave` / `beforeEnter` / `afterEnter` hooks（从 demo 抽取，去掉 demo 专属状态）
- [ ] 1.2 新增 `PeelTransition.vue` 组件：包装 `<Transition :css="false">`，接收 `direction: 'horizontal' | 'vertical'` prop，透传 `:key` 触发切换，内部用 usePeelTransition
- [ ] 1.3 单元测试：`horizontal` 与 `vertical` 两种方向下，enter/leave 的初始与终态 clip-path、transform 值正确

## 2. 日报视图接入转场

- [ ] 2.1 `BoardDailyReportTimeline.vue`：将日报内容区（Masthead + 正文）用 `PeelTransition` 包裹，`:key` 绑定当前日报标识（boardId + dayIndex）
- [ ] 2.2 新增 `direction` 状态：watch `boardId` 变化→`vertical`、`currentDayIndex` 变化→`horizontal`；在触发 key 变更**前**同步写入 direction
- [ ] 2.3 转场容器 `relative` + 子项 `absolute inset-0` + 固定 `min-height`，处理同时模式布局
- [ ] 2.4 验证 `useDailyReportReader` 在 `boardId` 变化时自动 reload（若缺则补 `watch(boardId) → loadReports + 重置 dayIndex`）
- [ ] 2.5 动画进行中加锁，防止越界连点导致状态错乱

## 3. 就近版块切换 UI

- [ ] 3.1 日报区头部新增版块切换条（segmented control），数据来自 `boards`（`useTagsPage`）
- [ ] 3.2 点击版块调用 `handleSelectBoard`，当前版块高亮
- [ ] 3.3 边界态：仅一个版块时切换条优雅降级（隐藏或仅展示当前）

## 4. 清理与验证

- [ ] 4.1 删除临时 demo 页 `app/pages/demo/peel-transition.vue`
- [ ] 4.2 `pnpm lint`（WSL 可跑）
- [ ] 4.3 `pnpm exec nuxi typecheck`（**Windows cmd**）
- [ ] 4.4 `pnpm test:unit`（**Windows cmd**）
- [ ] 4.5 `pnpm build`（**Windows cmd**）

## 5. 文档

<!-- doc-impact: none(纯前端 GSAP 转场组件 + 日报版块就近切换 UI；不改业务链路/数据/API/配置/部署。apply 启动时以 doc-impact.sh suggest 实际预勾选为准；若 suggest 命中 flow/standard，则在此补对应文档更新) -->

- [ ] 5.1 apply 时按 `doc-impact.sh suggest` 预勾选结果同步 `docs/reference/`；若 suggest 无命中则本节空置
