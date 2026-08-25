## Context

现行 lane 分桶（`BucketTagsByCentroid`，`daily_report_lane.go`）的 L1 分支条件为 `nearestDist < LaneL1Threshold && !nearestTopic.IsVacuum`，对 active 与 candidate 一视同仁；candidate 由此获得零成本直挂续命（实例 #1032，见 proposal）。L2 裁决的 briefs 注入（`ListTopicRecentBriefs`，Slice D）查询仅取 active 话题，且取的是 `daily_report_sections.cluster_label`——而 L1/L2 命中 section 的该列被 orchestrator 用 topic label 硬覆盖，注入内容退化为 label 复读。

约束背景：
- 2026-08-19 prompt-hygiene 已确立「L2 prompt 历史隔离」红线——不注入历史 thread 文案，防"昨日幻觉→今日延续"闭环。本设计的 briefs 修复必须遵守：注入的是**当天事实性 tag 标签**，非叙事文案。
- 全人工归档主权：算法 SHALL NOT 自动置 archived。观察期门禁只改变"挂载路径"，退场依赖既有 candidate_decay_window 自然滑出。
- 打标/裁决模型为小参数 LLM（qwen3.5-9b 级），结构化抽取能力有限——这是否决 SVO 方案、选择"降级 L2 由 LLM 整体裁决"路线的原因。

## Goals / Non-Goals

**Goals:**

- candidate 近距离 tag 从 L1 直挂改道 L2 LLM 裁决，且可用运行时开关一键回滚
- L2 裁决基于事实性近期内容（section 当天 tag 标签）而非冻结 label 复读
- L2 batch prompt 增量可控（单次调用裁决全部 L2 band tags 的既有结构不变）

**Non-Goals:**

- 不改 lane_l1_threshold / lane_l2_threshold 数值、vacuum 机制、L3 规则
- 不改话题生命周期状态机（consecutive_hits/last_seen 语义、升级门槛、归档规则）
- 不做 SVO/主谓宾抽取（已否决：小模型结构化抽取错误率高，脏数据会把门变成随机门）
- 不做存量 candidate 清理（800 存量中 52 个"隐藏苟活"依赖门禁后自然失血，另观察）
- 不动 active 话题的直挂资格与 briefs 注入语义（active 泳道保持确定性）

## Decisions

### D1 门禁实现点：BucketTagsByCentroid 单点改条件，而非 planTopicAssignments

L1/L2 分桶在 `BucketTagsByCentroid`（纯函数、无 DB/LLM，单测覆盖充分）。在此加 `nearestTopic.Status == active` 条件，candidate 近距离自然落入已有的 `case nearestDist <= LaneL2Threshold` L2 分支，复用 top-K 候选构造，无新代码路径。
备选（否决）：在 `planTopicAssignments`（归属落库层）拦截——分桶与落库会不一致（lane 报 l1、落库改 l2），破坏 section.lane_tier 与 confidence 的一致性契约。

### D2 开关：persistent_topic_candidate_l1_gate_enabled（默认 true）

挂进既有 `PersistentTopicConfig` KV 加载体系（`LoadPersistentTopicConfig` + `DefaultPersistentTopicConfig`，存 ai_settings）。语义为"门禁启用"（true=新行为），关闭即回退旧行为。备选（否决）：无开关直改——观察期话题分流形态变化需要在线回滚能力，出问题时不该等发版。
命名对齐既有 `persistent_topic_*` 前缀风格。

### D3 briefs 事实化：tag labels 取代 cluster_label，SQL 层一次到位

`ListTopicRecentBriefs` 的 LATERAL/JOIN 改为从 `daily_report_sections.cluster_tag_ids`（JSON 数组）join `topic_tags.label` 聚合出每 section 的标签清单（上限截断，如 5 个/section），替换 `section_label` 字段内容。同时查询范围从 `status=active` 扩到 `status IN (active, candidate)`——candidate 现在流经 L2 裁决，需要内容供判断。
注入格式（prompt 侧）：`- section (日期): tag1 / tag2 / tag3`。事实指纹无叙事诱导面，不触 hygiene 红线（红线禁的是 thread title/summary 类生成文案）。
备选（否决）：注入 section title 原文——LLM 生成文案，与被删的 thread title 同性质，重开 hygiene 决定。

### D4 candidate briefs 注入 cap：与 active 相同（5 section/话题）

不引入独立 cap 配置。candidate 现存命中数普遍低（#1032 为 10 次/24 天），7 天窗口内 section 数自然少，实际注入量由窗口天然限制。避免配置面膨胀。

### D5 L2 prompt 观察期从严指令：system prompt 追加一段

在 `buildL2Prompt` 的 system 部分追加：「状态为观察中的候选话题尚未经用户确认，请依据其近期实际内容从严判断 keep；若其近期内容与待判标签分属不同事件，应判 new」。既有 per-candidate 渲染已含 `状态:观察中` 标注，无需改 user prompt 结构。默认 keep 兜底（解析失败/漏答）保持不变——这是安全网，不是主路径。

### D6 生效范围：仅改当日及以后生成的报告

无迁移、无回刷。存量 section 的 lane_tier 不重算；存量 candidate（含 52 个隐藏苟活）依赖后续每日生成自然滑出窗口。部署后当日日报即生效（scheduler 下轮触发）。

### D7 keep 解析尊重显式 target（08-25 卡里巴夫复发根因修复）

上线首日观察发现：门禁已生效（相关 tag 全走 l2_llm），LLM 也做出了语义正确裁决（keep→1151「美伊博弈」），但 `parseL2Response` 的 `case "keep"` 无条件 `targetTopicID = nearest`（Candidates[0]），丢弃 LLM 填写的 target——tag 被改写吸附回 embedding 最近处（僵尸 candidate #1032）。小模型常态混用 keep/switch（实测两轮 3/16 与 1/20+），keep+非最近 target 恰是"LLM 有不同意见"的场合。
修复：keep 且 target 非空、≠nearest、在候选集内（复用 `inCandidateSet`）→ 尊重该指定；空/集外 target 维持回 nearest 安全网（keep 不携带换轨强意图，不改 off-shortlist switch 的降级 new 规则）。
备选（否决）：keep+集外 target 也降级 new——会放大碎片化风险，安全网回 nearest 更保守。

### D8 briefs 排除当日 section（同日重跑自证回路）

`ListTopicRecentBriefs` 原只有下界（`period_date >= now-7d`），同日重跑时把当日早前运行（旧版本行为）挂错的 tag 当作"近期实际内容"注入，LLM 据此"有内容支撑"合理 keep——错挂被洗白。修复：加 `period_date < today` 上界（`NormalizeReportDate(now)`），注入仅含今日之前的事实；次日运行昨日 section 才作为证据。与"7 天窗口"语义兼容（昨日仍在窗口内）。
备选（否决）：按 report id 排除"正在重算的报告"——需要传递重跑上下文到 repo 层，侵入性大；按日排除在重跑与首次生成两种场景下都成立。

## Risks / Trade-offs

- [L2 band 流量增大，单次 batch prompt 变长] → 调用次数不变（1 次/板块/天），input token 增量预计 3~5 倍内；qwen 级模型长上下文裁决质量若劣化，观察日志后可先关 D2 开关回滚。
- [小模型误判 new → 碎片化：同叙事 tag 流落 L3 开新 candidate] → 双保险：候选集注入事实性 briefs（D3）+ 从严指令只针对"无内容支撑"场景（D5）；默认 keep 兜底仍在。上线后观察新 candidate 出生率，异常升高即回滚。
- [真延续的 candidate 每天过 L2，某天被判 new 断流] → 该 tag 转 L3 后新 candidate 会重新聚起，损失的是历史命中链而非内容；且 upgrade_threshold 门槛要求本就需连续命中，人工确认前的 candidate 断链成本可接受。
- [briefs 扩到 candidate 后 prompt 总长上升] → per-topic cap 5 不变，candidate 数量受 7 天窗口自然约束；D4 拒绝了独立 cap，若实测过长再补配置。
- [cluster_tag_ids 含已合并/禁用 tag] → join 时过滤 `topic_tags.status='active'`，标签缺失的 section 自然少注入，不影响其余。

## Migration Plan

1. 合并部署（后端单侧，无 DB 变更；开关默认 true 即生效）。
2. 部署后首日观察：`daily-report: lane cluster %d tags → l1=%d l2=%d l3=%d` 日志行——l2 占比上升为预期形态；`ai_call_logs` 中 `daily_report.decide_l2_tags` 的 prompt 长度与裁决结果抽样。
3. 回滚：`ai_settings` 置 `persistent_topic_candidate_l1_gate_enabled=false`，无需发版；briefs 事实化为纯增益修复，不随开关回退（若需同时回退，单独 revert）。

## Open Questions

- 观察 1~2 周后是否将「观察期门禁」推广到 active（甲方案：全取消 L1）——数据说话（新 candidate 出生率、双胞胎话题率），另行立项。
