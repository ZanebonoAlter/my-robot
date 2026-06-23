# 日报视觉重构 + "突发新闻"修复 · Handoff

> 本会话（2026-06-22）的完整成果交接。新会话可直接基于本文档接手落地。
> 设计 demo 已完成验证，后端问题1修复已完成，问题2待落地到 Vue 组件。

---

## 一、本次任务背景

用户报告两个问题（Board 1974「中东地缘政治与美伊关系」为例）：

1. **日报全是"突发新闻"**：日报页的「关心的话题」分区永远空，归属了持久话题的 section 全堆在「突发的新话题」
2. **日报样式问题**：①样式硬编码不适配 dark 主题；②宽屏留白多；③"弹窗"展示不够沉浸；④需要杂志拟真化重设计

---

## 二、问题1：日报全是"突发新闻" ✅ 已修复（待最终验证）

### 根因（已用数据库 + 代码双重确认）

数据完全正常（Board 1974：11 active / 3 candidate 话题，40/43 section 归属 active 话题）。
问题在**后端取数漏了一环**：

- 前端 `qualityZones` 用 `s.persistent_topic?.status === 'active'` 分类（`BoardDailyReportTimeline.vue:59-64`）
- 但日报详情 API（`GetReportByID`）只 `Preload("Sections")`，**没加载嵌套的 `PersistentTopic`**
- `DailyReportSection` model 原本只有 `PersistentTopicID *uint`，无 `PersistentTopic` 关系字段
- 所以前端拿到的 section JSON 里 `persistent_topic` 永远是空 → `?.status` 是 `undefined` ≠ `'active'` → 全归「突发」

> 对比：section-timeline API（BoardThreadBrowser 用的）有 `attachTopicBriefs` 正常 —— **只有日报主体页这条链路坏了**。

### 后端修复（4 处，已 build + vet 通过）

| 文件 | 改动 |
|---|---|
| `repository/daily_report_models.go:118` | `DailyReportSection` 加 transient 字段 `PersistentTopic *PersistentTopicBrief \`gorm:"-"\`` |
| `repository/daily_report_repository.go:597` | 新增 `loadTopicBriefMap(db, ids)` —— 抽取通用加载逻辑，section-timeline 复用 |
| `repository/daily_report_repository.go:622` | 新增 `AttachTopicBriefsToReport(db, report)` + `attachBriefsToSections` |
| `repository/daily_report_repository.go:265` | **`GetReportByID` 调用 `AttachTopicBriefsToReport`**（关键！日报详情 API 入口）|
| `repository/daily_report_repository.go:332` | `ListReportsForAllBoards` 也调用 |

> ⚠️ **handoff 前发现 `GetReportByID` 的调用曾遗漏，已在 handoff 时补上并验证（build+vet OK）**。新会话务必确认此处未被回退。

### 前端

**无需改动**。`DailyReportSection` 接口已有 `persistent_topic?: PersistentTopicBrief`（`dailyReports.ts:94`）。后端修复后，前端 `qualityZones` 自动正确分类。

### 待办

- [ ] 重启后端后，浏览器实测日报页「关心的话题」分区正常显示（用户已重启过后端，但 `GetReportByID` 修复在后，需再重启一次）
- [ ] 补 repo 单测：`TestGetReportByID_AttachesTopicBriefs`

---

## 三、问题2：日报视觉重构 · 设计完成（demo 已验证）

### 设计决策链（全部经用户确认）

1. **方向**：报刊拟真(A) × 纸感极简(C) 混搭 —— 基于项目已有的「Editorial Magazine」设计系统 DNA
2. **字体**：Noto Serif SC（思源宋体，已在 `nuxt.config.ts` 配置）—— 中文报刊经典衬线
3. **交互架构**：升级当前全屏 overlay（A 方案），**不**做独立路由页 —— 仿侦探墙 `position:fixed` 全屏模式
4. **宽屏布局**：2 栏（左 sticky 边栏 + 正文），头条/速递通栏跨满，正文多栏
5. **左侧边栏内容**：本期目录 + 话题泳道（色块）+ 历史日报日期导航 —— 统计去掉
6. **话题泳道交互**：方案 C（inline 原位展开）—— **不跳转**
7. **泳道形态**：横向 mini 泳道（E 方案），只画 identity 线，参考现有 `BoardThreadBrowser.edgePaths` 逻辑

### 关键设计约束（落地时必须遵守）

- **泳道连线**：用 SVG `<path>` 贝塞尔曲线 `M x1,y1 C midX,y1 midX,y2 x2,y2`，连接相邻有数据节点，跨过空白天仍连线（同话题延续性）。**绝对不要**画横贯直线 + 假断开
- **双主题**：全部走项目 `--color-*` token（`main.css` 已定义 `[data-theme="editorial"]` / `[data-theme="dark"]`），**消除所有硬编码** `rgba(0,0,0,*)`
- **标题撑满宽屏**：`clamp(40px, 8vw, 128px)` + `white-space:nowrap`，去掉 `<br>`
- **横向滚动条**：6px 细滚动条（专属 `.lane-strip::-webkit-scrollbar` 样式 + Firefox `scrollbar-width:thin`）

### Demo 位置 & 运行

```
design-demos/daily-report-magazine.html   (936 行，单文件，双主题可切换)
```

```bash
# 运行 demo（handoff 时已在 8899 端口运行，新会话可能需重启）
cd design-demos && python3 -m http.server 8899 --bind 127.0.0.1
# 访问 http://localhost:8899/daily-report-magazine.html
# 右上角"夜刊/Dark"按钮切换主题
```

### Demo 包含的完整交互（playwright 全部实测通过）

| 交互 | 实现 |
|---|---|
| 主题切换 | `data-theme` 属性切换 editorial/dark |
| 左侧目录锚点 | `data-scroll` → `scrollIntoView` + 高亮 |
| 历史日期切换 | `.cal-day` 单选高亮 |
| 话题卡片展开泳道 | 点击 `.topic-card-head` → `toggleTopic` → `renderLaneLines` |
| 泳道节点展开当日详情 | 点击 `.lane-day.has-data` → `toggleLaneDay`（单选切换）|
| **泳道 SVG 连线** | `renderLaneLines()` 用 `getBoundingClientRect` 算坐标，展开/resize 重绘 |
| thread 展开关联文章 | `.thread.has-articles` 点击 → inline 展开文章列表 |
| lane-detail thread 展开文章 | `.ld-thread.has-articles` 点击 → 紧凑文章列表 |
| 文章 preview | 点击文章项 → 高亮（落地时 `emit('openArticle', id)`）|

### 数据流（落地时的映射）

demo 是静态数据。落地到 Vue 时：

| demo 元素 | Vue 数据来源 |
|---|---|
| masthead 统计 | `selectedDetail.article_count` / `cluster_count` |
| 头条 | `selectedDetail.highlights[0]` 或最高分 section |
| Highlights 三栏 | `selectedDetail.highlights` |
| 话题卡片 | `qualityZones` computed（已修复，按 active/candidate/unassigned 分类）|
| thread 列表 | `section.threads`（DailyReportThread，含 `related_article_ids`）|
| 关联文章 | `thread.related_article_ids` → 调 `getArticle(id)` 加载标题 |
| **泳道历史节点** | `getTopicLifeline(topicId)` API（返回 sections + relations，**已有**）|
| 泳道连线 | `lifeline.relations` 过滤 `relation_type==='identity'` → 贝塞尔路径 |
| 历史日报日期 | `reports` 列表（`getBoardDailyReports`）|

---

## 四、落地工作清单（问题2）

### 阶段 1：重构 BoardDailyReportTimeline.vue 容器

- [ ] 当前是 `np-overlay` + `np-paper(width:1100px)` 的弹窗形态 → 改为 `position:fixed; inset:0` 全屏铺开
- [ ] 去掉 `np-paper` 的 `width: min(1100px, 92vw)` 约束
- [ ] 内容区改用 demo 的 2 栏 grid（`.paper` grid-template-areas）
- [ ] 把现有硬编码 `rgba(0,0,0,*)` 全部替换为 `--color-*` token

### 阶段 2：masthead + 头条 + highlights

- [ ] masthead：标题用 `clamp(40px,8vw,128px)` + `nowrap`，统计并入 meta-row
- [ ] 头条：取最高分 section，双栏分栏正文，首字母 dropcap
- [ ] highlights 三栏：`selectedDetail.highlights` 渲染

### 阶段 3：左侧 sticky 边栏

- [ ] 三块：本期目录（`data-scroll` 锚点）+ 话题泳道（色块，对应 `laneRows`）+ 历史日报（`reports` 列表，点击切换 `currentDayIndex`）
- [ ] 1100px 以下隐藏边栏，降为单栏

### 阶段 4：话题卡片 + inline 横向泳道（核心）

- [ ] 话题卡片标题区可点击展开/收起
- [ ] 展开后调 `getTopicLifeline(topicId)` 异步加载历史 sections
- [ ] 渲染横向 mini 泳道：7 天节点，identity 线 SVG 连线（复用 demo 的 `renderLaneLines` 逻辑）
- [ ] 同天多 section → 单节点 + 数字角标
- [ ] 节点点击 → 展开当日 threads

### 阶段 5：thread 关联文章

- [ ] thread 加 `.has-articles`（`related_article_ids.length > 0`）
- [ ] 点击展开 → inline 文章列表卡片（`getArticle` 加载标题）
- [ ] 文章点击 → `emit('openArticle', id)`（复用现有 TagsPage 的 preview）

### 阶段 6：验证

- [ ] 前端：`pnpm lint` + `pnpm exec nuxi typecheck`（Windows cmd）+ `pnpm test:unit` + `pnpm build`
- [ ] 双主题视觉检查（editorial 晨报 + dark 夜刊）
- [ ] 宽屏（1440+）/ 中屏（1100-）/ 窄屏（720-）响应式
- [ ] openspec change 归档（建议归入现有 `2026-06-19-persistent-topic` 或新建）

---

## 五、本次会话已改动的文件清单

### 后端（问题1修复，已完成）
- `backend-go/internal/topicgraph/repository/daily_report_models.go` — 加 transient 字段
- `backend-go/internal/topicgraph/repository/daily_report_repository.go` — `loadTopicBriefMap` + `AttachTopicBriefsToReport` + `GetReportByID`/`ListReportsForAllBoards` 调用

### 前端 API（本会话早些时候的话题管理任务，已完成）
- `front/app/api/dailyReports.ts` — `listBoardTopics` / `deleteTopic` / `BoardTopicListItem`（已含 `persistent_topic`）

### 设计 demo（问题2，已完成）
- `design-demos/daily-report-magazine.html` — 单文件完整 demo

### 截图（playwright 生成，仅供参考，可删）
- `v4-final.png` 等（项目根目录，可清理）

---

## 六、关键风险 & 注意事项

1. **`GetReportByID` 的 `AttachTopicBriefsToReport` 调用**：handoff 时刚补上，新会话第一步务必确认 `daily_report_repository.go:265` 这行存在且未被回退（这是问题1修复的命门）
2. **泳道连线不能臆造**：务必参考 `BoardThreadBrowser.vue:311-326` 的 `edgePaths`，或直接复用 demo 的 `renderLaneLines()` 逻辑（贝塞尔，跳空白）
3. **前端编译必须 Windows cmd**：`pnpm exec nuxi typecheck` / `pnpm build` 在 WSL 会缺 native binding 失败
4. **测试只跑影响包**：改了 daily_report 就只跑 `go test ./internal/topicgraph/...`，不要跑全量
5. **demo 用真实数据**：Board 1974（中东地缘政治），6.21 日报 id=60，6.20 id=52

---

## 七、未决问题（需新会话确认）

- [ ] 落地时是否要保留 demo 的"号外 · Extra Edition"刊头风格，还是改为更朴素的产品 header？
- [ ] 侦探墙出口（"在侦探墙打开完整生命线 ↗"）是否保留？demo 里画了，落地时需对接 `TopicDetectiveWall` 的 `enterTopicLifeline`
- [ ] 是否新建 openspec change 还是归入 `2026-06-19-persistent-topic`？（建议新建，视觉重构和话题管理是两件事）
