## Context

日报 thread（事件）是用户实际阅读的最细粒度对象，但它**没有质量信号**。现有 observability 量到两档：

| 维度 | 信号 | change |
|------|------|--------|
| tag↔板块（System 1） | `best_tier`/`quality_breakdown` | `quality-scoring-observability`（进行中）|
| section↔话题（System 2） | `topic_match_distance`/`confidence` | `topic-anchor-match-observability`（完成）|
| **thread↔section（System 3）** | **无** | **本 change** |

数据流现状（链路需新建）：

```
GenerateClusterThreads (Step5) → threads 标题，无 embedding
   ▼
Step6 装配 section（标题已有 embedding）
   ⚠️ 断点：thread 与 section 标题的贴合度从未计算
   ▼
daily_report_threads 落库（无 embedding/贴合距离列）
   ▼
前端 DailyReportTopicSection.vue 渲染 .drm-thread（无离群信号，全量正常展示）
```

真实案例佐证（2026-06-26，board「全球科技巨头动态」section 800）：LLM 在 `ClusterTags`(Step3) 把 tag「华为腾讯百度齐下场争夺机器人大脑生态」误归进"OpenAI 发布管控"簇（9 个 tag 里 1 个异类），`GenerateClusterThreads` 忠实生成 thread「华腾百巨头争夺机器人控制器核心生态控制权」。section 标题向量锚定到 topic「OpenAI 债务危机」= 0.1814（anchor_hit，**锚定本身正确**），但跑题 thread 搭便车进入 OpenAI 话题，用户无任何信号。

约束：
- 跨端 change（Go 后端算距离落库 + Nuxt 前端展示），单用户、无 auth。
- 复用 observability 系列展示分层哲学：**正文极轻（形态/降级，无数字），分数文字只进探究区**。
- 复用 orchestrator 现有 `Embed` 调用链路（section 标题已在 Step6 批量 embed），thread 标题同 provider。

## Goals / Non-Goals

**Goals:**
- thread 生成后计算其标题 embedding 与所属 section 标题 embedding 的余弦距离，落库为贴合度信号。
- 前端对离群 thread（距离超阈值）**软降级**（灰显/折叠），保留信息不删除；附离群标记 + hover 探究区展示贴合度数值与中文标签。
- 信号随 section 叙事广度自适应（紧凑型 section 严判、伞形 section 宽判），不预设"组须紧凑"。
- 历史 thread（无贴合度字段）降级为正常 thread，不报错、不降级。

**Non-Goals:**
- 不治源头：不检测/重组 section 内 tag 归簇（`ClusterTags` 聚类纯度归 `embedding-content-mismatch` 待办 issue）。
- 不做归簇后 embedding 紧凑性剔除（A1 方案，伞形话题误杀——见 D1）。
- 不做反哺回路（事后校验不回流到事前 ClusterTags 约束，留后续 change）。
- 不改 System 1/2 可视化与 section↔topic 锚定算法。
- 不自动剔除跑题 thread（软降级保信息，由用户甄别）。

## Decisions

### D1. 治理点：事后校验 thread↔section 贴合度（B1），非事前归簇紧凑性剔除（A1）

**选 B1，否决 A1。** 两种治理点对比：

| 方案 | 检测量 | 误杀风险 | 对伞形话题 |
|------|--------|---------|-----------|
| A1 归簇后 tag 紧凑性剔除 | tag↔group_name | **高** | ❌ 误杀（见下）|
| B1 thread↔section 标题距离 | thread↔section_title | 低 | ✅ 自适应 |

**A1 误杀的实测证据**（2026-06-26 anchor_hit 话题内 section 距离 spread）：

| 话题 | n | spread | 形态 |
|------|---|--------|------|
| 白宫庆典活动与争议插曲 | 3 | 0.000 | 紧凑型 |
| 俄乌冲突前线战损与战术博弈 | 8 | 0.000 | 紧凑型 |
| **XR 硬件爆发：消费新品到全球军援** | 7 | **0.290** | **伞形** |
| 开发者工具链：本地调试→平台化 | 12 | 0.291 | 伞形 |
| 大模型商业化路径：融资/广告/开源 | 8 | 0.273 | 伞形 |

A1 的隐含假设是"同组 = embedding 紧凑"，但伞形话题的全部意义就是**用一个叙事框架统摄 embedding 上分散的子事件**。"XR 硬件"里讲军援的 tag 离 group_name 向量远（0.29），但归在"XR 硬件爆发"框架下完全合理——A1 会误杀它。

B1 量的是 **thread 是否忠于它所在 section 的标题**，而 section 标题是 LLM 在 Step3 给这组的**叙事宣言**。紧凑型 section 标题聚焦（如"俄乌前线战损"），thread 必须贴战损→严判；伞形 section 标题包容（如"XR 硬件爆发：新品到军援"，标题声明了广度），thread 贴新品或军援都行→自然宽容。**信号随 section 自身叙事广度自动伸缩，无需预判话题形态。**

**备选（否决）**：B2 让 LLM 在 thread 加"贴合组主题"自评字段——LLM 自评不可靠（既当运动员又当裁判），且 LLM 聚类本身就在这步犯过错。

### D2. 贴合度计算时机：Step5 之后、Step7 合并之前，批量算

thread 标题 embedding 与 section 标题 embedding 都来自同一 provider。section 标题已在 Step6 批量 embed（`daily_report_orchestrator.go:226`）。thread 标题在 Step5 生成后即可批量 embed。

**计算位置**：Step6 装配 section（拿到 section.Embedding）之后、Step7 `MergeSimilarSections` 之前。原因：Step7 合并可能改 section 归属，需在合并前固定 thread↔section 配对；但 section.Embedding 在 Step6 末尾已就绪，足够算距离。

```
Step5 GenerateClusterThreads → threads[].Title
Step6 装配 section（含 section.Embedding）
  ▼ 新增：批量 embed thread 标题，算 thread.Embedding ↔ section.Embedding 余弦距离
  ▼ 写入 thread.fit_distance
Step7 MergeSimilarSections（thread 随 section 一起合并/搬运）
```

**备选（否决）**：落库后异步补算——引入一致性问题（生成完到补算前前端读不到信号），且多一条异步链路。同步算简单可靠，thread 数量级（每 section 几条）embed 开销可控。

### D3. 软降级阈值：实现期实测标定，候选 0.20 → 实测标定 0.28

阈值**不可在 propose 阶段实测**（thread 表当前无 embedding 列，需先落库）。design 给标定方法 + 候选值，实现期 Task 1 补一个现网分布标定步骤（参照 `topic-anchor-match-observability` D2 的标定先例）。

**候选阈值 `THREAD_FIT_DEMOTE_THRESHOLD = 0.20`**（导出常量，便于调参）：
- 候选理由：section↔topic 锚定阈值 0.30 是"够上"的及格线；thread↔section 是"是否忠于本 section 主题"，应比跨话题锚定严。0.20 < 0.30 给出 0.10 的余量，让"贴 section 但略远"的 thread（如 0.15-0.20）不被误降级。
- 标定方法：落库后查现网 thread.fit_distance 分布，找分布的"自然断点"（贴合聚集 vs 离群聚集的分界），据此微调常量。
- **降级形态**：阈值之上为离群（灰显 + 折叠 + 离群标记），之下为正常——**单阈值二态**（不做三档，thread 粒度信号只需"正常 vs 跑题"二元判断，无需 anchor 那样的紧实度梯度）。

**实测标定结果（2026-06-27，重新生成 2026-06-26 日报后，86 个有信号 thread）**：

| 统计量 | 值 |
|--------|-----|
| min / p50 / avg / p90 / p95 / p99 / max | 0.000006 / 0.156 / 0.159 / 0.274 / 0.287 / 0.306 / 0.306 |

各阈值降级数（共 86）：

| 阈值 | 降级数 | 占比 | 评价 |
|------|--------|------|------|
| 0.20（候选）| 30 | **35%** | ❌ 太宽，1/3 thread 灰显淹没信号（候选推理的“0.20 给余量”假设被实测否定：分布整体偏大，p50 已 0.156）|
| 0.25 | 15 | 17% | 偏宽 |
| **0.28（选定）** | **7** | **8%** | ✅ **选定**——抓住真跑题、不过度；给长尾留余量 |
| 0.30 | 2-3 | 2.3% | 最保守，假阴性风险（放跑跑题）|

选定理由：(1) 分布为平滑长尾无干净双峰，取 p95≈0.287 附近作为"正常尾巴 vs 真离群"的过渡带，0.28 落在 p95 之上、贴近上尾起点；(2) 真阳验证——最大离群 `0.3062`（「Anthropic CEO 因沟通冲突被换下」挂「科技股领跌」板块）确属真跑题（人事变动 vs 股市），与原 design 设想的机器人案例同类；(3) 软降级保信息不删，8% 降级面可接受。

**最终常量 `THREAD_FIT_DEMOTE_THRESHOLD = 0.28`**（`front/app/utils/threadFit.ts` 导出，边界测试用常量引用，后续调参改一处）。

**备选（否决）**：双阈值三档（0.10/0.20 切"紧贴/一般/离群"）——thread 粒度无需细粒度梯度，三档徒增视觉噪声，违反"正文极轻"原则。

### D4. 软降级展示：灰显 + 折叠（默认折叠），保信息不删

离群 thread 的处置选**软降级**而非剔除/重组：

| 处置 | 机制 | 选/否 |
|------|------|-------|
| 纯标记（D1 路线） | 仅 hover 信号 | ❌ 太轻，正文看不出 |
| **软降级** | 灰显 + 默认折叠 + 离群标记 | ✅ **选** |
| 软降级 + 探究行 | 上者 + hover 贴合度数值 | ✅ **选** |
| 剔除 | 不渲染 | ❌ 删信息，单用户产品损失叙事完整性 |
| 重组 section | tag 回流拆分 | ❌ 治源头，scope 跳档（Non-Goal）|

软降级保证：跑题 thread 不刺眼（默认折叠，不污染 section 叙事），但点击/展开仍可读（信息不丢，用户可甄别）。默认折叠的 thread 数 ≥1 时，section 底部显示"另有 N 条可能跑题的线索"提示行。

**展示分层**：正文 thread 标题照常显示但**降级样式**（灰 token + 折叠收起）；贴合度数值 + 中文标签（"贴合"/"可能跑题"）只在 hover/展开时呈现——与 anchor 徽章一致。

### D5. 与 observability 系列的边界（粒度互补）

本 change 是 observability 三系列的**第三层**，三者正交、展示面互补：

| 系列 | 粒度 | 信号 | 展示面 |
|------|------|------|--------|
| quality-scoring | tag↔板块 | tier/breakdown | section 头部色点 + 探针 per-tag |
| topic-anchor | section↔topic | distance/confidence | section 头部锚定点 + 探针顶部行 |
| **thread-fit（本）** | **thread↔section** | **fit_distance** | **thread 行降级样式 + 探究行** |

三者挂点不同、数据血缘不同，不互相改 spec。本 change 新增 thread 行级渲染逻辑，不动 section 头部已有的两套徽章。

## Risks / Trade-offs

- **[贴合度阈值 0.20 是候选值，实现期实测可能调整]** → 做成 `threadFit.ts` 导出常量 `THREAD_FIT_DEMOTE_THRESHOLD`，单测覆盖边界；Task 1 含现网分布标定步骤，实测后微调一处常量即可。
- **[伞形 section 的跑题 thread 可能误判为正常（假阴性）]** → B1 天然对伞形宽容（标题声明广度），这是特性非缺陷；伞形 section 内真正跑题的 thread（如案例的人事变动混入股市/算力板块）距离仍会显著超阈值（实测真阳 0.3062 远大于选定阈值 0.28），假阴性风险低。Task 3.1 标定用实测最大离群做真阳验证。
- **[批量 embed thread 标题增加生成耗时]** → thread 数量级小（每 section 几条，单 board 单日通常 <30），与 section 标题 embed 同 provider 同批次，增量耗时可控。失败非致命（thread 无 embedding → 无 fit_distance → 前端按正常渲染，与历史 thread 同降级路径）。
- **[历史 thread 无 fit_distance 被误降级]** → fit_distance 为空/零值判为"正常 thread"（不降级），只在 fit_distance 有效且超阈值时降级。降级是"有信号才触发"，默认状态是正常。
- **[默认折叠可能让用户错过信息]** → 折叠不等于隐藏：section 底部显示"另有 N 条可能跑题的线索"提示，点击展开可见全量内容 + 贴合度数值。信息完整，只是视觉降级。
