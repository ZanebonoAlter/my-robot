## 1. 契约冻结与组件边界

- [x] 1.1 重启/更新本地后端并确认 `GET /api/daily-reports/:id` 的已归属 section 全部包含 `persistent_topic`；若缺失则停止视觉实施，先修复前置依赖（日报 60：4/4 已归属 section 均返回 brief）
- [x] 1.2 为现有关闭、Esc、日期切换、SectionLifecyclePanel、BoardThreadBrowser 和 `openArticle` 事件补行为特征测试，冻结重构前能力（3 个组件特征测试通过）
- [x] 1.3 从 `BoardDailyReportTimeline.vue` 向 `components/daily-report/` 提取 masthead、sticky 边栏、topic section/card、mini lifeline 四类职责明确的组件，容器保留列表/详情/日期/缓存编排

## 2. 全屏阅读层与编辑排版

- [x] 2.1 将 `np-paper` 定宽弹窗改为 `position: fixed; inset: 0` 全屏阅读层，保留关闭、Esc、背景滚动锁和焦点恢复
- [x] 2.2 实现产品化 masthead、头条和最多三项 highlights；头条按 highlight-first、section-fallback 映射真实数据，移除固定“号外”文案
- [x] 2.3 实现本期目录、active 话题索引和历史日报日期 sticky 边栏，日期点击复用现有 detail cache，目录锚点跟随当前阅读区
- [x] 2.4 保持 active/candidate/unassigned 的 `qualityZones` 分区和分区内质量排序；多 section 以 `<h4>` 标题展示 `cluster_label`，移除 section-card header 打开 SectionLifecyclePanel 的入口

## 3. 主题与响应式

- [x] 3.1 将日报相关 surface、文字、边框、阴影和交互状态迁移到现有 `--color-*` / `--shadow-*` token，清除固定浅色背景假设（生产组件固定浅色扫描零命中）
- [x] 3.2 实现 >1100px 双栏、721–1100px 单栏顶部控制、<=720px 窄屏布局；宽屏标题可 nowrap，窄屏必须恢复换行且无页面级横向溢出
- [x] 3.3 为横向 lifeline 增加细滚动条、触控滚动和清晰 focus 状态；所有 loading/error/empty 状态不得只靠颜色表达

## 4. Topic mini 生命线

- [x] 4.1 实现按 topic id 缓存的 lifeline loader，覆盖 idle/loading/success/error/retry，重复展开不重复请求
- [x] 4.2 将 lifeline 数据裁剪为当前日报向前七个自然日，并把同日多个 section 聚合为带数量角标的单节点
- [x] 4.3 仅从 `relation_type=identity` 的真实关系生成贝塞尔路径，支持跨空白日期连接；不得臆造节点/边或混入 similarity relation；跨过一个以上空白的连线以弱化不透明度区分
- [x] 4.4 在展开/resize/节点详情高度变化后安全重算 SVG 坐标（以 track 本身为测量基准，避免 padding 偏移），`requestAnimationFrame` 后计算；组件卸载时清理 observer/listener
- [x] 4.5 节点点击原位展开当日 threads；active topic 在设备能力允许时提供进入侦探墙完整生命线的出口

## 5. Thread 文章与旧能力回归

- [x] 5.1 实现按 article id 去重的标题缓存，只请求未缓存 ID；单篇失败局部展示并可重试
- [x] 5.2 将 thread 文章从 floating popup 改为原位列表，点击继续 `emit('openArticle', articleId)`，移除由本次改造产生的废弃 popup 状态和样式
- [x] 5.3 切换 board 时重置 days、topic/day/thread 展开状态和 board 相关缓存；“加载更早”继续以 7 天递增并替换列表响应
- [ ] 5.4 复测并保留话题总览、侦探墙生命线入口、文章 preview、日报日期导航和空日报状态（阅读层不再提供 SectionLifecyclePanel 入口，组件本体保留）

## 6. 报刊装饰 DNA 与 demo 对齐

- [x] 6.1 按 demo 基线拉齐 masthead 装饰：kicker + sub 副标题 + accent `.sep` + 刊头 `3px double` 下分隔；宽屏标题 `clamp()` + `nowrap` + ink-bleed `text-shadow`，窄屏恢复换行
- [x] 6.2 头条 dateline 以破折号包裹，正文 `column-count:2` 双栏 + `column-rule`，首字母 dropcap（SHALL NOT 与 `text-indent` 同时使用）
- [x] 6.3 highlights 下分隔为 `4px double`；每项以 `N°NN` 斜体序号起头
- [x] 6.4 thread 改为左竖线 + `::before` 圆点节点（移除 border-top 横向分隔）；article 卡片改为左侧 accent 色条 + hover accent-soft 背景；history 面板左线改 accent
- [x] 6.5 历史日报边栏改为日历风：serif 字体 + 左色条 + 当前项 accent-soft 高亮
- [x] 6.6 mini 生命线连线/节点/今日高亮/hover 光晕/图例统一使用 `--color-accent`；topic 身份色仅保留在边栏色块、status badge 和 lifeline 容器左侧细线
- [x] 6.7 新增 colophon 页脚（`3px double` 上分隔 + 斜体署名 + accent ornament + 日期）
- [x] 6.8 为各区块新增 `ink-fade` 进场动画（masthead/头条/highlights/zone/dynamics/colophon 错开 delay，lifeline 内层 `ll-slide`），遵守 `prefers-reduced-motion`
- [x] 6.9 阅读层背景叠加 accent 径向暖光渐变（`color-mix` 透明）；SHALL NOT 还原 `feTurbulence` 纸张噪声叠层（dark 主题整层发白）
- [x] 6.10 修复跨层弹窗 z-index：`AppDialog` 暴露 `zIndex` prop，`ArticlePreviewModal` 以 `9100` 渲染（阅读层为 `9000`）

## 测试 / Test

- [x] T.1 为 headline fallback、quality zone 排序、七日窗口、同日节点聚合、identity edge 过滤与贝塞尔路径增加 Vitest 单元测试（7 tests PASS）
- [x] T.2 为 lifeline/article 缓存增加 Vitest 测试，覆盖请求去重、失败隔离、重试和 board 切换清理（3 tests PASS）
- [x] T.3 为全屏 shell 增加 Vue 组件测试，覆盖关闭/Esc/焦点恢复、日期切换、话题总览入口和 `openArticle` emit（3 tests PASS）
- [x] T.4 新增 `front/tests/e2e/daily-report-magazine.spec.ts`，使用可控 fixture 覆盖 editorial/dark、1440/1000/720 viewport、话题/日期/thread 展开及无横向溢出
- [ ] T.5 使用真实 Board 1974 日报做浏览器烟测，确认 active/candidate 分区、identity 连线和文章 preview 与 fixture 一致

- [x] T.6 为 lifeline weak 连线增加 Vitest 单元测试，覆盖相邻强连线与跨空白弱连线判定（+1 test，共 8 tests PASS）

## 文档 / Docs

- [x] D.1 更新 `docs/reference/architecture/frontend.md`：日报全屏阅读层、组件职责、lifeline/article 缓存和事件流
- [x] D.2 更新 `docs/reference/api/daily-reports.md`：说明前端如何消费 topic brief/lifeline/identity relation；API 本身无新增端点
- [x] D.3 在 change 内记录双主题与三档 viewport 的视觉验收结果、已知差异和 demo 到生产实现的取舍

## 验证 / Verify

> 归档前必须按顺序重跑本节全部命令并确认零失败。

- [x] V.1 前端 lint：`cd front && pnpm lint`；期望：0 error
- [x] V.2 前端 typecheck：`cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"`；期望：退出码 0
- [x] V.3 前端单元测试：`cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit"`；期望：全部测试通过
- [ ] V.4 日报 E2E：`cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec playwright test tests/e2e/daily-report-magazine.spec.ts"`；期望：全部场景通过
- [x] V.5 前端生产构建：`cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"`；期望：退出码 0
- [x] V.6 固定浅色回归扫描：`cd front && rg -n "rgba\(0,\s*0,\s*0|#[0-9a-fA-F]{6}" app/features/tags/components/BoardDailyReportTimeline.vue app/features/tags/components/daily-report`；期望：除 topic 数据色与明确注释豁免外零命中
- [x] V.8 跨层弹窗修复扫描：`cd front && rg -n "zIndex\|:z-index" app/components/ui/AppDialog.vue app/features/tags/components/ArticlePreviewModal.vue`；期望：AppDialog 暴露 zIndex prop、ArticlePreviewModal 传 `9100`
- [x] V.7 OpenSpec 严格校验：`openspec validate daily-report-magazine-redesign --type change --strict`；期望：valid
