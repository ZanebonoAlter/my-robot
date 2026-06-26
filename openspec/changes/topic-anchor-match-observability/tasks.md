## 1. 前端工具函数（TDD 基石 · 纯函数先行）

- [ ] 1.1 新建 `front/app/utils/topicAnchor.ts`，导出双阈值常量 `TOPIC_ANCHOR_TIGHT_THRESHOLD = 0.05`、`TOPIC_ANCHOR_LOOSE_THRESHOLD = 0.15` 与两个纯函数：`topicAnchorTier(distance, confidence)` → `0=极紧/1=稳锚/2=松锚/3=新候选/4=未锚`；`topicAnchorLabel(distance, confidence)` → "极紧锚定"/"稳锚定"/"松锚定"/"新话题候选"/"未锚定"。判定主信号是 `confidence`（unmatched/缺失 → 4），`distance` 仅在 `anchor_hit` 内做三档细分（≤0.05 极紧 / (0.05,0.15] 稳 / (0.15,0.30] 松）。`auto_new` 恒为档 3。验收：纯函数无副作用、无 DOM 依赖
- [ ] 1.2 **TDD 红**：先写 `front/app/utils/topicAnchor.test.ts`，覆盖：五态典型值、双阈值边界值（0.05 归极紧、0.15 归稳锚）、distance 缺失/0 零值、confidence 缺失、auto_new 忽略 distance。验收：`pnpm test:unit topicAnchor` 全红（函数未实现）
- [ ] 1.3 **TDD 绿**：实现函数使测试全通过。验收：`pnpm test:unit topicAnchor` 全绿

## 2. 前端正文锚定紧实度徽章（System 2 · 不破沉浸）

- [ ] 2.1 新建 `front/app/features/tags/components/daily-report/SectionAnchorBadge.vue`，props：`tier: number`（topicAnchorTier 返回值）。渲染 0.4rem 圆点，档 0 实心 / 档 1 半透明(accent 55%) / 档 2 淡半透明(accent 30%) / 档 3 空心 accent / 档 4 空心灰 token。无任何数字/文字。颜色 MUST 由主题语义 token（`--color-*`）派生，跟随双主题。验收：五档视觉符合 design D2 表
- [ ] 2.2 **TDD**：写 `SectionAnchorBadge.test.ts`，断言五档 class/style/data 属性正确、尺寸 ≤ tier 徽章（0.5rem）、无数字文字、双主题 token 派生。先红后绿
- [ ] 2.3 在 `DailyReportTopicSection.vue` 的 `.drm-section-card__head` 挂载：`<SectionTierBadge>`（已有）后并列 `<SectionAnchorBadge :tier="topicAnchorTier(section.topic_match_distance, section.topic_match_confidence)">`，间距 0.25rem。验收：两徽章并列渲染、head 不溢出
- [ ] 2.4 验证 §7 codegraph：`SectionAnchorBadge` 调用点（`DailyReportTopicSection`）无遗漏，`topicAnchorTier` 引用面正确

## 3. 前端探究区话题锚定行（分数进 hover · 复用探针）

- [ ] 3.1 改造 `SectionQualityExplore.vue`：新增三个可选 props `topicLabel?: string`、`topicDistance?: number`、`topicConfidence?: string`（不破坏现有 `breakdown` 调用点）。当 `topicConfidence` 非 unmatched 且 `topicDistance` 有效时，在 per-tag 列表**上方**渲染 header 行：「🔗 话题锚定 · {topicLabel} · 距离 {distance.toFixed(2)} · {topicAnchorLabel}」。topicLabel 缺失降级 cluster_label/"未命名话题"。验收：行在明细上方、历史 section 不渲染该行
- [ ] 3.2 **TDD**：写/扩 `SectionQualityExplore.test.ts`，覆盖：完整锚定行渲染、松/紧标签区分、auto_new 行仍展示 distance、unmatched/缺失不渲染行、topicLabel 缺失降级、历史 section（无 breakdown 无 anchor）显示"无质量明细"且不报错。先红后绿
- [ ] 3.3 在 `DailyReportTopicSection.vue` 调用处把 `section.persistent_topic?.label`、`section.topic_match_distance`、`section.topic_match_confidence` 透传给 `<SectionQualityExplore>`。验收：探针展示真实话题锚定数据

## 4. 架构体检（§7 强制，每子任务后）

- [ ] 4.1 codegraph 代码图：确认 `topicAnchorTier`/`topicAnchorLabel` 调用面（徽章 + 探针）、`SectionAnchorBadge` 挂载点无遗漏、`SectionQualityExplore` props 变更未漏调用点。验收：`codegraph impact topicAnchorTier` / `SectionQualityExplore` 命中预期
- [ ] 4.2 架构合理性：两徽章并列不引入循环依赖；新 utils 落在共享层（`app/utils/`）而非 feature 内，与 `matchQuality.ts` 同层对齐；探针 props 扩展为可选不破坏现有契约。验收：无新增 lint 警告、无循环依赖

## 5. 测试（§5.2 前端双层）

- [ ] 5.1 工具函数单测：`cd front && pnpm test:unit topicAnchor` → PASS
- [ ] 5.2 组件单测：`cd front && pnpm test:unit SectionAnchorBadge SectionQualityExplore` → PASS
- [ ] 5.3 全量回归：`cd front && pnpm test:unit` → PASS（无既有用例回归）

## 6. 文档（§9 产出物 / §12 流转）

- [ ] 6.1 更新 `docs/reference/flow/` 下日报相关业务链路（如有 section 可视化/质量段落），补充 System 2 锚定可观测的展示语义（正文锚定点 + 探究区锚定行），与 System 1 并列说明两套维度
- [ ] 6.2 更新 `docs/reference/architecture/map.md` 索引：日报域新增"话题锚定可观测"入口指向本次组件（若 map 有质量可视化条目则补并列条目）
- [ ] 6.3 检查 `docs/reference/standard/` 前端规范是否需补充"双徽章并列/共享 utils 落位"约定（仅在确有新约定时）

## 7. 验证（§11.2 归档门禁 · 每条可执行 + 期望结果）

- [ ] 7.1 `cd front && pnpm lint` → 零 error（WSL 可跑 lint）
- [ ] 7.2 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"` → 零 error（typecheck 必须 Windows cmd）
- [ ] 7.3 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"` → 构建成功（build 必须 Windows cmd）
- [ ] 7.4 `cd front && pnpm test:unit` → 全绿
- [ ] 7.5 `grep -rn "topic_match_distance\|topic_match_confidence" front/app --include=*.vue --include=*.ts | grep -v "\.test\.ts"` → 命中本次新增消费点（徽章 tier 计算 + 探针 props 透传），证明 System 2 字段不再"零展示"
- [ ] 7.6 `grep -rn "SectionTierBadge\|SectionAnchorBadge" front/app --include=*.vue` → 两徽章并列挂载于 `DailyReportTopicSection.vue`
- [ ] 7.7 `bash scripts/check-standards.sh` → 零失败（L1 规范验收，归档前自检）
