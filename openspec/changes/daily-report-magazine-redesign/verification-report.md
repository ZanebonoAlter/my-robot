# 日报杂志化视觉验收

## 验收基准

- 视觉源：`design-demos/daily-report-magazine.html`、`v4-final.png`，以及用户提供的超宽屏展开态 / mini 时间线截图。
- 真实数据：Board 1974「中东地缘政治与美伊关系」，日报 60。
- 主题：editorial、dark。
- 视口：1440×1000、1000×900、720×900、390×844；另以用户 2048px Chrome 截图检查超宽屏结构。

## 已确认结果

- masthead 已区分板块标题与日报头条，首屏层级与 demo 一致。
- 话题区改为“左侧目录 + 右侧单列通栏正文”，展开态不再被双列网格锁在半宽卡片内。
- 首个 active 话题默认展开；当前 threads 位于 mini 时间线之前，文章标题按需加载。
- mini 时间线改为通栏七日泳道：日期均匀分布、同日 section 数量角标、identity 贝塞尔连线、当前日原位详情、图例与侦探墙出口处于同一模块。
- editorial / dark 主题均只使用语义 token；1440 / 1000 / 720 / 390 检查未出现阅读层横向溢出。
- lint：0 error（仓库既有 23 warnings）；Nuxt typecheck PASS。
- Vitest：22 files / 130 tests PASS；生产构建 PASS；OpenSpec strict validate PASS。

## demo 到生产实现的取舍

- demo 的节点和文章来源为静态示例；生产实现严格使用 topic lifeline、report detail 和 article API，不补造节点、关系或文章元数据。
- demo 可把“动态数量”画在节点角标；生产节点角标按同日 section 聚合数量，避免把 thread 数误写成 section 数。
- 纸张纹理伪元素在真实浏览器截图中会造成整层发白，生产实现移除该叠层，保留暖白/深色语义 surface 与编辑排版层级。

## 已知数据差异

- Board 1974 日报 60 的部分 `related_article_ids`（如 17181、17187）在当前本地库返回 404。组件已按设计局部显示失败与重试；fixture 可完整验证 article preview，但真实数据烟测需后端数据修复后复跑。
