# Tasks: revamp-landscape-charts

> 纯前端 change，后端零改动。TDD：option 纯函数先写测试再实现（happy-dom 无 canvas，只测纯函数，不测 echarts 渲染）。

## 1. 依赖与图表基建

- [x] 1.1 `cd front && pnpm add echarts`（模块化按需引入，不引 vue-echarts）
- [x] 1.2 新建 `front/app/features/tags/components/topic-landscape/useEcharts.ts` composable：`onMounted` init（`echarts/core` + 注册 BarChart/ScatterChart/LineChart + Grid/Tooltip/DataZoom/Legend + CanvasRenderer）、`ResizeObserver` 自动 resize、`onBeforeUnmount` dispose、暴露 `setOption`
- [x] 1.3 新建 `topic-landscape/chart-options.ts`：`ChartPalette` 类型 + `readPalette()`（`getComputedStyle` 读 `--color-accent` 等 CSS 变量 + stance 语义色常量，对齐 main.css 双主题）

## 2. 话题节奏总览气泡图（方案 C）

- [x] 2.1 【测试先行】`app/features/tags/components/topic-landscape/topicRhythmOption.test.ts`：断言 `buildRhythmOption(topics, dates, palette)` 的 y 轴话题排序（stance 分组序 active→stalled→emerging→pending→archived、组内 hit_count DESC）、按 stance 分 5 series、archived series 默认 legend unselected、`section_count=0` 不产数据点、气泡尺寸 sqrt 缩放且 clamp、dataZoom 默认窗口
  - 注：仓库无 `front/tests/unit/`（vitest include 为 `app/**/*.test.ts`），测试与组件同目录放置（对齐 `BoardThreadBrowser.focus.test.ts` 约定）
- [x] 2.2 实现 `chart-options.ts` 的 `buildRhythmOption`（x=日期 category、y=话题 inverse、symbolSize sqrt clamp 4~18、legend、y 轴 dataZoom inside+slider、tooltip 显示话题/日期/N 节）
- [x] 2.3 新建 `TopicRhythmChart.vue`：`<ClientOnly>` 包裹 + useEcharts 挂载；点击气泡 emit `selectTopic(topicId)`；`watch(useTheme().theme)` 重建 option
  - 注：`useTheme()` 实际暴露 `theme`（非 design 写的 `currentTheme`），已用实际 API
- [x] 2.4 `TopicLandscapePanel.vue` 挂载：VitalityBar 之下、StanceCardWall 之上，selectTopic 沿用现有 emit 链路

## 3. 卡片迷你柱图 + emerging 去图（方案 A）

- [x] 3.1 【测试先行】`miniBarOption.test.ts`：断言 `buildMiniBarOption(lifeline, palette)` 的 x 轴取 lifeline 全部日期（空日 0 值占位，日期轴连续）、柱高=section_count、tooltip formatter 输出「M/D：N 节」
- [x] 3.2 实现 `buildMiniBarOption` + 新建 `MiniLifelineChart.vue`（无轴无网格迷你柱图，useEcharts 挂载，主题跟随）
- [x] 3.3 改造 `TopicStanceCard.vue`：`active`/`stalled`/`pending`/`archived` 用 `MiniLifelineChart` 替换 `MiniLifeline`；`emerging` 不渲染节奏图（保留 label/命中数/最近命中）
- [x] 3.4 确认 `StanceCardWall.vue` archived 分组折叠时图表 `v-if` 不挂载（展开才初始化）；删除 `MiniLifeline.vue`

## 4. 活力顶栏升级

- [x] 4.1 【测试先行】`vitalityOption.test.ts`：断言 `buildVitalityOption(trend, palette)` 的 x 轴日期连续、series 为 line+areaStyle、含轻量坐标轴与 axis tooltip
- [x] 4.2 实现 `buildVitalityOption` + 改造 `VitalityBar.vue`：手算 polyline 换 ECharts 面积图，指标数字行（article_count/section_count/active_topic_count）不变

## 5. 测试与静态检查

- [x] 5.1 单测：`cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit 2>&1"` 全绿（44 files / 486 tests，含新增 3 文件 26 tests）
- [x] 5.2 `cd front && pnpm lint` 无新增告警（0 error；5 warning 全为既有文件，新文件零告警）
- [x] 5.3 typecheck：`cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"`（exit 0）
- [x] 5.4 构建：`cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"`，确认 echarts chunk 体积符合预期（独立 chunk ~280KB / ~104KB gzip，模块化引入，无全量打包）

## 6. 文档

<!-- doc-impact: flow,architecture,standard -->
<!-- doc-impact-excuse: api,database=suggest 基于脏工作树中无关后端改动误命中，本 change 纯前端零 API/模型改动 -->

- [x] 6.1 `docs/reference/flow/semantic-board.md`：话题态势版图的可视化描述更新（气泡总览图 + 迷你柱图 + emerging 去图），变更溯源链接本 change
- [x] 6.2 `docs/reference/architecture/frontend.md`：图表库选型记录（echarts 模块化引入 + useEcharts 封装 + option 纯函数约定）
- [x] 6.3 `docs/reference/standard/frontend/`：补图表封装红线（统一走 useEcharts + chart-options 纯函数，禁止组件内手写 echarts init）

## 7. 验证

- [x] 7.1 opencli 端到端：`/tags` 选多话题板块（如「生成式 AI 与大模型厂商」），验证气泡图渲染/排序/legend 过滤/点击跳话题总览聚焦；活跃+停滞卡片迷你柱图 tooltip；emerging 卡片无图；archived 折叠不挂载、展开渲染（卡片点击链路已实测跳话题总览 focus 视图；气泡点击与卡片共用同一 handler，受工具限制未在 canvas 内精确坐标实测）
- [x] 7.2 opencli 截图验证亮/暗主题切换后全部图表配色跟随（派 kimi-coding/k3 做视觉判断）（用户人工确认）
- [x] 7.3 归档门禁：`./scripts/doc-impact.sh verify`（exit 0：声明 flow,architecture,standard / 文件 2 个）与 `./scripts/check-standards.sh` A-D/F/G 段通过（E 段 1 处历史 FAIL 属另一 archive change `2026-08-01-fix-watch-delete-cascade` 未补溯源，与本 change 无关）

## 8. apply 后端到端验证发现并修复（2026-08-01）

> opencli 真实 Chrome 端到端验证时发现三处问题并已修复（task 7.x 勾选前必须先过本清单）：

- [x] 8.1 **echarts 完全不渲染（根因：`<ClientOnly>` 时序死结）**：`<ClientOnly>` 的插槽内容在自身 `onMounted` 后才异步渲染，父组件 `useEcharts()` 的 `onMounted` 执行时 `elRef.value === null` → `if (!elRef.value) return` 静默跳过 init，永不重试（实测 0 canvas、无 JS 报错）。修复：三个图表组件（`TopicRhythmChart`/`VitalityBar`/`MiniLifelineChart`）删除模板 `<ClientOnly>` 包裹（`ssr:false` 下本就多余，`onMounted` 自身保证 client-only）+ `useEcharts` 增加 `watch(elRef)` 兜底（容器晚于 onMounted 挂载时自动 init，防未来 v-if/延迟渲染回归）。
- [x] 8.2 **点击气泡/卡片跳侦探墙而非话题总览**：`TagsPage.handleLandscapeSelectTopic` 原实现切 tab 后 `openTopicOverviewDetectiveWall(topicId)`（弹全屏侦探墙），与 spec「切到话题总览 tab 并聚焦」不符。修复：`BoardThreadBrowser` 新增 `focusTopicId` prop（watch 后 `enterFocus` 进入 focus 专注视图），TagsPage 点击改为切 tab + 传 `focusTopicId`，不再弹侦探墙；同一话题重复点击先置 null 再设值保证 watch 触发。
- [x] 8.3 **气泡图 y 轴空泳道误导**：y 轴含 lifeline 全 0（所选范围内无任何命中）的话题行，无气泡易被误读为「有节奏没画出来」。实测 139 话题中 25 个无命中。修复：`buildRhythmOption` 过滤 lifeline 无 `section_count>0` 的话题（y 轴 labels 与 series 都不含），补单测断言。
- [x] 8.4 **气泡图可读性**：总览图 320px 高、文字 9-10px、气泡 sqrt*4 clamp [4,18] 偏小。调大：容器 520px、y 轴标签 12px、x 轴 11px、legend 12px、气泡 sqrt*5 clamp [6,26]，同步更新 `bubbleSize` 单测。
