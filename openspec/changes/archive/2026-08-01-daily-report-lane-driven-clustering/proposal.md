## Why

日报聚类当前是「LLM 自由聚类成 section → 再用 section 标题 embedding 匹配 PersistentTopic」两段式，两层主观判断叠加放大偏差。实测见 `docs/experience/cluster-bias-investigation.md`（2026-07-26 三批调研）：

1. **LLM 自聚类 5 形态偏差**：跑题 thread 搭便车（`fit_distance>0.28` 占 14%，338/2395）、万能包装标题强行打包不相关 tag（「开发者工具链：视频弹幕与 BFF」）、脑补标题（「存储芯片暴涨」tag 只有「CXL 技术」）、section↔topic 错锚、跨 board 重复发散（同 tag 挂多 board 各聚一遍）。
2. **事后匹配放大偏差**：section 标题是 LLM 万能包装产物，topic 标题也是，二者向量天然近，0.30 阈值把大量「沾边但不相关」错锚。最松 anchor_hit 抽样：「巴林银行合并」↔「卡塔尔赠送星链空军一号」(0.300)、「中年三无生活」↔「年轻人一人公司」(0.299)。
3. **现有 embedding 锚点太弱**：topic 用「首义向量」（首条 section 标题继承），对当天 event tag 强挂（dist<0.20）仅 14.3%。

## What Changes

核心反转：**先以 PersistentTopic 泳道为框架，用 embedding 把当天 tag 分桶，LLM 退化为弱区裁决 + 新叙事兜底**，而非 LLM 自由聚类后匹配。

### A. topic 泳道表示升级（embedding 锚点）
- topic 的匹配锚点由「首义向量」改为「历史 section embedding 质心」（近期窗口加权）。
- 实测（最近 7 天 718 event tag）：强挂（<0.20）从首义向量 14% 提升到质心 **62%**，<0.25 覆盖 **88%**，挂不上（>0.30）从 19% 降到 **1.3%**。

### B. 三层分桶聚类
- **L1 强挂（质心 dist<0.18）**：tag 直接归属对应 topic，高置信。约 47% tag。
- **L2 弱区（0.18–0.30）**：不硬挂，LLM 在「embedding 预筛的候选 topic（top-K）+ tag」上做「留/换/新」三选一。约 51%。
- **L3 兜底（>0.30）**：开新 cluster（LLM 起新叙事标题）或单 tag / 过滤噪声。约 1.3%。
- section 天生挂 topic（L1 直挂 / L2 LLM 挂 / L3 新 topic），**消除事后 section↔title↔topic 匹配**（形态4错锚根源）。

### C. 吸尘器 topic 检测
- 部分 topic 是历史 LLM 聚类产出的万能包装标题，质心过宽，把沾边 tag 都吸成最近邻（实测「中国中央银行相关新闻」strong=0/全 mid+weak、「开发者工具链从本地调试走向平台化」strong3/mid31、「XR 硬件生态爆发」strong2/mid21）。
- 检测规则：topic 的 `strong/(strong+mid) < vacuum_ratio（默认 0.20）` 判为过宽，挂到它的 tag 降级 L2 让 LLM 裁决。

### D. LLM 角色退化
- 从「自由聚类全部当天 tag」退化为「只处理 L2 弱区子集（在预筛候选上三选一）+ L3 新叙事起标题」。输入聚焦，万能包装/强行打包诱因大幅减少。

## Capabilities

### Modified Capabilities
- `daily-report-system`：聚类流程由「LLM 自由聚类 → 事后 embedding 匹配」改为「embedding 质心分桶（L1/L2/L3）→ LLM 弱区裁决/兜底」；`ClusterTags` 的 LLM 职责收窄到 L2+L3。
- `persistent-topic`：topic 匹配锚点由首义向量改为历史 section 质心；新增吸尘器 topic 检测；section 归属判定由「双重确认 AND-gate」改为「L1 直挂 / L2 LLM / L3 新建」。

## Impact
- **后端**：`daily_report_cluster.go`（分桶 + L2/L3 LLM 调用收窄）、`daily_report_assignment.go`（`planTopicAssignments` 重构为 L1/L2/L3）、`daily_report_orchestrator.go`（管线顺序：质心计算→分桶→LLM 弱区→组装）、`daily_report_topic_repository.go`（质心计算/缓存、吸尘器检测）。
- **数据**：topic 质心物化（`board_persistent_topics` 增 `centroid` 向量列 + 增量更新，或质心视图）；`daily_report_sections` 新增 lane 归属标记（`lane_tier`: l1_direct/l2_llm/l3_new）。
- **flow**：`docs/reference/flow/daily-report.md` 聚类节重写。
- **configuration**：阈值 `lane_l1_threshold`(0.18) / `lane_l2_threshold`(0.30) / `vacuum_ratio`(0.20) / `centroid_window`（质心近期窗口）。
- **部署后影响**（用户可见）：日报 section 不再出现「万能包装标题强行打包不相关 tag」；section↔topic 错锚大幅减少；topic 泳道跨天连续性增强。
- **旧数据降级**：历史 section 归属不回刷（保持原 `topic_match_confidence`），仅新日报走新流程；topic 质心首次按历史 section embedding 离线构建一次。
- **本 change 不做**（留后续 change）：tag 碎片化去重（实测 33% tag 有 <0.10 近重复邻居）、跨 board 重复聚类。
