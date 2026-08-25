<!-- constraint-domains: daily-report -->

## Why

candidate 话题与 active 话题在 L1 泳道享受完全相同的零成本直挂：同域 tag 质心距离 < 0.18 即直接归属、刷新 last_seen，导致观察期话题"每苟活一次再活 7 天"永不出窗。实例：candidate #1032（label 为 8-01 一次性新闻标题"伊朗议长卡利巴夫透露哈梅内伊殉难后的凌晨应急决断"）此后 24 天靠同域 tag 直挂续命 10 次，section 标题被冻结 label 覆盖，天天顶着过期新闻标题出现在日报"其他动态"区，且 candidate 对用户不可归档——人工主权够不着的地方在自动苟活。

同时 L2 裁决的"近期内容"注入存在空转 bug：L1/L2 命中 section 的 cluster_label 被话题 label 硬覆盖，`ListTopicRecentBriefs` 拉回来的"近期 section 框架"是候选话题 label 的复读（零信息量），Slice D 设计意图完全落空。

## What Changes

- **candidate 取消 L1 直挂资格**：`BucketTagsByCentroid` 的 L1 分支增加 `status=active` 条件；最近话题为 candidate 时（即使距离 < lane_l1_threshold、非 vacuum）降级进入 L2 band 走 LLM 裁决。active 话题保持直挂不变。语义补完："用户确认过的框架信任直挂；系统猜想的框架每次挂载都要过审"。
- **一次性事件自然退场**：candidate 的 tag 被判 new → 转 L3 后，candidate 失去当日命中 → 7 天锚定窗口（candidate_decay_window）自然滑出 → 不再进入注入/锚定集合。不引入任何自动归档，不改变全人工 archive 主权。
- **briefs 事实化修复**：`ListTopicRecentBriefs` 注入内容从 `cluster_label`（冻结 label 复读）改为 section 当天实际 tag 标签（`cluster_tag_ids` join `topic_tags.label`，事实指纹），并将覆盖范围从仅 active 扩展到 candidate（candidate 现在流经 L2，需要内容供裁决）。查询失败仍降级 label-only（既有降级契约不变）。
- **L2 prompt 观察期从严指令**：候选话题标注"观察中"时，prompt 明确要求 LLM 依据近期实际 tag 内容从严判断 keep；无近期内容支撑的一次性标题话题不应仅凭域相近 keep。
- **keep 解析尊重显式 target（08-25 卡里巴夫复发修复）**：`parseL2Response` 对 keep+显式 target 指向候选集内另一话题的（小模型常态混用 keep/switch，实测 ~10-20%），尊重该指定归属而非静默改写回最近候选——旧实现把 LLM 判的 keep→1151 改写回 keep→1032，致候选话题被续命，门禁形同虚设。空/集外 target 维持回最近候选安全网。
- **briefs 排除当日 section（同日重跑防自证）**：`ListTopicRecentBriefs` 注入窗口加 `period_date < today` 上界——同日重跑时当日早前运行挂错的 tag 会作为"近期内容"证据洗白错挂；次日运行昨日 section 才作为证据。
- **运行时开关**：`persistent_topic_candidate_l1_gate_enabled`（ai_settings KV，默认 true=启用门禁），可在线回滚到旧行为，无需发版。
- **不改**：L1/L2 阈值本身、vacuum 机制、L3 规则、话题生命周期状态机、归档主权、API、前端、DB schema。

## Capabilities

### New Capabilities

（无）

### Modified Capabilities

- `persistent-topic`: 「DailyReportSection 强制归属 PersistentTopic」需求变更——L1 直挂增加 active-only 条件，candidate 近距离降级 L2；「ClusterTags 注入历史叙事框架」需求变更——近期内容注入改为 section 实际 tag 标签并覆盖 candidate。

## Impact

- **后端代码**：
  - `backend-go/internal/topicgraph/service/daily_report_lane.go`（`BucketTagsByCentroid` L1 分支条件 + `parseL2Response` keep 尊重显式 target + `buildL2Prompt` 观察期从严指令）
  - `backend-go/internal/topicgraph/repository/daily_report_repository.go`（`ListTopicRecentBriefs` SQL 改造：tag labels 提取 + candidate 覆盖 + 当日排除）
  - `backend-go/internal/topicgraph/repository/daily_report_topic_repository.go`（`PersistentTopicConfig` 增加开关键 + 默认值 + 加载）
- **测试**：`daily_report_lane_test.go`（分桶单测：candidate 近距离进 L2 / active 保持 L1 / 开关关闭回退旧行为）、`daily_report_repository_test.go`（briefs：tag labels 注入、candidate 覆盖、label-only 降级）
- **配置**：`ai_settings` 新增 `persistent_topic_candidate_l1_gate_enabled`（默认 true）
- **可观测**：沿用现有 `lane cluster %d tags → l1=%d l2=%d l3=%d` 日志行；上线后观察 l2 占比上升与新 candidate 出生率为预期形态
- **无影响**：API / 前端 / DB schema / 部署
