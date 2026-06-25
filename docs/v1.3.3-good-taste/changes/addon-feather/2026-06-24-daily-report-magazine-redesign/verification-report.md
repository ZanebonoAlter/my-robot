# 日报杂志化视觉验收

## 验收基准

- 视觉源：`design-demos/daily-report-magazine.html`（报刊拟真 × 纸感极简）、`v4-final.png`，以及用户提供的超宽屏展开态 / mini 时间线截图。
- 真实数据：Board 1974「中东地缘政治与美伊关系」，日报 60。
- 主题：editorial、dark。
- 视口：1440×1000、1000×900、720×900、390×844；另以用户 2048px Chrome 截图检查超宽屏结构。

## 已确认结果

- masthead 已区分板块标题（`boardTitle`）与日报标题（`report.title` 作为 sub），首屏层级与 demo 一致：kicker + 大标题 + sub + accent `.sep` + 刊头 `3px double` 下分隔。
- 话题区改为"左侧目录 + 右侧单列通栏正文"，展开态不再被双列网格锁在半宽卡片内。
- 首个 active 话题默认展开；当前 threads 位于 mini 时间线之前，文章标题按需加载。
- mini 时间线改为通栏七日泳道：日期均匀分布、同日 section 数量角标、identity 贝塞尔连线、当前日原位详情、图例与侦探墙出口处于同一模块。
- mini 时间线连线、节点、今日高亮、hover 光晕、图例统一使用 `--color-accent`（editorial 红烧 / dark 琥珀），与 demo 报刊统一强调色一致；topic 身份识别由边栏色块、status badge 与 lifeline 容器左侧细线承担。
- 跨过一个以上空白的 identity 连线以弱化不透明度呈现，与相邻节点强连线区分（demo 原意还原）。
- thread 改为左竖线 + `::before` 圆点节点（hover 竖线/圆点/背景三层变 accent）；article 卡片以左侧 accent 色条 + hover accent-soft 背景呈现；历史日报边栏改为日历风（serif + 左色条 + 当前项 accent-soft 高亮）。
- masthead/头条/highlights/各 zone/dynamics/colophon 使用 `ink-fade` 进场动画错开延迟；lifeline 内层使用 `ll-slide`；全部遵守 `prefers-reduced-motion`。
- 阅读层底部新增 colophon 页脚（`3px double` 上分隔 + 斜体署名 + accent ornament + 日期）。
- editorial / dark 主题均只使用语义 token；1440 / 1000 / 720 / 390 检查未出现阅读层横向溢出。
- 跨层弹窗修复：阅读层 `z-index:9000`，`ArticlePreviewModal` 经 `AppDialog` 新增的 `zIndex` prop 以 `9100` 渲染，文章预览在阅读层之上可触发。
- lint：0 error（仓库既有 23 warnings，与本次无关）；Nuxt typecheck PASS。
- Vitest：22 files / 132 tests PASS（含 lifeline weak 连线判定 +1）；生产构建 PASS。

## demo 到生产实现的取舍

- demo 的节点和文章来源为静态示例；生产实现严格使用 topic lifeline、report detail 和 article API，不补造节点、关系或文章元数据。
- demo 可把"动态数量"画在节点角标；生产节点角标按同日 section 聚合数量，避免把 thread 数误写成 section 数。
- **纸张噪声不还原**：demo 中 `body::before` 的 SVG `feTurbulence` 噪声纹理未还原到生产。该叠层在 dark 主题下 `mix-blend-mode:screen` 会漂白整个阅读层（本 change 验收期实测确认），"纸感"由阅读层背景叠加的 accent 径向暖光渐变（`color-mix` 透明）承担。
- **lifeline 配色**：demo 全局使用 `--accent`（红），生产最初按 spec 决策 6 改为 `--topic-color`，实测后按用户要求统一回到 `--color-accent`，与 demo 一致；topic 自带色仅保留在边栏色块、status badge 与 lifeline 容器左侧细线。
- **section 生命周期入口移除**：原 demo 与早期 spec 保留 section-card header 打开 `SectionLifecyclePanel` 的入口，生产实现确认该入口与 mini 生命线职责重复、角落 `is-compact` 图标语义不明，遂从阅读层移除该入口及 `BoardDailyReportTimeline` 上的事件链与 `SectionLifecyclePanel` 挂载。`SectionLifecyclePanel.vue` 组件本体及其 BFS 高亮测试保留，侦探墙继续复用其 `statusColorMap`；多 section 时仅以 `<h4>` 标题展示 `cluster_label`。
- **SVG 测量基准**：demo 与生产首轮都把 SVG 坐标基准挂在带 padding 的 scroll 容器，导致线点纵向错位（demo 偏 8px、生产偏约 10px）。生产实现改为以 `.drm-lifeline__track` 本身为基准，并在 `requestAnimationFrame` 后计算，线点贴合。
- **跨层 z-index**：`AppDialog` 由硬编码 `z-index:1000` 改为可配 `zIndex` prop（默认 1000，向后兼容），`ArticlePreviewModal` 传 `9100`。

## 已知数据差异

- Board 1974 日报 60 的部分 `related_article_ids`（如 17181、17187）在当前本地库返回 404。组件已按设计局部显示失败与重试；fixture 可完整验证 article preview，但真实数据烟测需后端数据修复后复跑。
