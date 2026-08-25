# Design: fix-section-merge-blackhole

## Context

8-22 日报全线塌缩（51→19 sections，单板 6+→1，当日 0 个新话题）的根因链见 proposal 与 explore findings（`explore-findings.md`「日报黑洞根因」）：`fix-section-embedding-content-based` 换了 section embedding 的文本构成后，新几何下无关叙事间距压缩到 0.11~0.25，`MergeSimilarSections` 的 0.20/0.25 阈值（7-29 按旧标题几何标定）失去判别力，union-find 传递闭包把跨叙事 section 链式熔断成 mega-section，primary 的 `persistent_topic_id`/`lane_tier` 全盘保留导致归因伪装（乌克兰线索顶着 `l1_direct` 挂美伊 topic）。

关键事实：实证无关叙事可贴近 0.11，**距离维度在新几何下不存在可用判别阈值**；lane 管线（L1/L2/L3 分桶 + LLM keep/switch/new 裁决）本身工作正常，且其裁决是系统记录（`MatchedTopicID` + `LaneTier` 已在 section 上）。合并是纯展示层操作（消除聚类过碎），却拥有推翻归因的能力——这是权限错位。

现状代码锚点：

- `daily_report_merge.go` `MergeSimilarSections`：两两 cosine → `dist<0.20` 建边 → union-find 闭包 → `[0.20,0.25)` 灰区 LLM 仲裁（`llmArbitrateMerges`）→ 按 article_count 选 primary 合并
- `daily_report_orchestrator.go:51` 已加载 `topicCfg := LoadPersistentTopicConfig(...)`，`:337` 调用 merge
- `LoadPersistentTopicConfig`（`daily_report_topic_repository.go`）从 `ai_settings` 读 `persistent_topic_*` 系列键——它已是日报管线的实际配置袋

## Goals / Non-Goals

**Goals:**

- 阻断黑洞：同日合并不得跨越 lane 归因边界（不同 `MatchedTopicID` / 新叙事↔锚定 拒绝合并）
- 提供零代码回退开关，默认关闭合并，观察 lane 原始粒度后再决定是否重启
- Stage 1 确定性合并补审计日志（当前只有灰区对进 LLM 有痕）

**Non-Goals:**

- 不重标定 0.20/0.25 阈值（0.11 数据点证伪）
- 不改 lane 分桶 / L2 裁决 / 质心 / 跨日关系重建
- 不自动重跑 8-22 日报（用户手动 `runNow`，幂等覆盖）
- 不改合并后 primary 保留策略（同 topic 合并场景下该策略无害）

## Decisions

### D1: 锚定边界 = 建边前过滤，不是事后拆分

在 `MergeSimilarSections` 收集 `deterministicPairs` / `grayZonePairs` **之前**对每个 pair 校验锚定一致性：

```
可以合并 ⇔ (a.MatchedTopicID == b.MatchedTopicID != NULL)   // 同话题当日分组
       ∨  (a.MatchedTopicID == NULL ∧ b.MatchedTopicID == NULL) // 同新叙事池
```

不合法 pair 直接跳过：不建边、不进灰区 LLM、不产生日志噪音以外的副作用。

**为什么不是事后拆分**：union-find 闭包后按锚定拆分连通分量是 NP 难的图割问题；建边前过滤则数学上保证每个连通分量锚定必然一致（同 topic 边构成的分量必然同 topic），零成本。

**为什么 NULL↔NULL 放行**：L3 新叙事池本就该允许 LLM 把多个新组合并成一个叙事组（8-22 board 1974 的「中东地缘」组就是 3 个 L3 tag 合成）。边界只挡「系统已明确判为不同归属」的跨界。

**为什么不用「给 LLM 仲裁 prompt 加锚定上下文」**：8-22 实证 board 1980 的 LLM 仲裁全对（`merge_pairs:[]`）但照样塌——塌缩发生在确定性 <0.20 区，LLM 根本看不到那些 pair。修补 LLM 看不到的地方是无效的。

### D2: 开关走 `PersistentTopicConfig`，键名 `daily_report_section_merge_enabled`，默认 false

`PersistentTopicConfig` 加 `SectionMergeEnabled bool` 字段，`LoadPersistentTopicConfig` 的 keys 列表与 switch 加一条（`strconv.ParseBool`，解析失败记 warning 用默认）。orchestrator `:337` 调用点改为传 `topicCfg.SectionMergeEnabled`。

**为什么塞进这个结构体**：它已是日报管线事实上的配置袋（lane 阈值、窗口都在这），orchestrator 已加载，加一个字段是最小 diff。键名不用 `persistent_topic_` 前缀——合并是日报 section 层行为，语义上独立命名更清晰，keys 列表本身就是混装的字符串列表。

**为什么默认 false 而非 true（带边界）**：边界的正确性没有线上数据背书（0.11~0.25 的同域分布下，同 topic 的当日分组是否真需要合并、合并阈值是否仍合适，都未知）。默认关 = 明天 21:00 日报回到 lane 原始粒度，先观察几天拿到真实分布，再评估重启合并与阈值。开关存在本身就是保险丝：无需发版即可切换。

### D3: 审计日志用 `logging.Infof`，不建新表、不加 LLM 调用

Stage 1 每个候选对一条：双方 `cluster_label`、`MatchedTopicID`、`lane_tier`、距离、结果（merged / rejected-by-boundary / kept-dist）。灰区对本就经 `ai_call_logs` 留痕。这是可观测性补洞，不是审计系统——够用即止。

### D4: `MergeSimilarSections` 签名增加 `mergeEnabled bool` 参数

`func MergeSimilarSections(ctx, sections, threadBatches, tags, mergeEnabled bool)`。开关判断在函数内第一行短路返回。比在 orchestrator 调用点包 if 更可单测（现有测试直接构造调用）。

## Risks / Trade-offs

- **[合并关闭 → section 偏碎]** 同 topic 当日多分组将原样落库（lane 的 fallback 路径下可能出现）。→ 接受的回归：碎比黑洞好；观察后若需要，开 `daily_report_section_merge_enabled=true` 即获得「带锚定边界」的合并（本 change 交付的边界逻辑随开关启用生效）。
- **[边界过严 → 同 topic 合法合并不了]** 同 topic 的 L1/L2 分组当日距离若 >0.25，开关开启下也不会合并。→ 这正是设计意图（距离在新几何下无判别力），合并决策未来应交给 lane 层而非展示层。
- **[8-22 污染数据滞留]** 11 份 mega-section 日报保持污染直到重跑。→ 用户可选：各 board `runNow` 重跑 8-22（幂等覆盖重建）；topic 1151 的 `consecutive_hits=3` 重跑后会被当日结果自然修正，但若修复部署晚于下一次 planLifecycle 且 topic 1151 已激活，需评估是否人工降级——留给重跑时观察。
- **[跨日关系几何未标定]** `RebuildBoardRelations` 消费同一新几何 embedding，其匹配阈值是否失准未排查。→ 明确 out of scope；合并关闭不影响跨日关系逻辑运行，其正确性单独观察。

## Migration Plan

1. 合并部署 → `daily_report_section_merge_enabled` 默认 false 生效，下次 21:00 日报自动回到无合并行为
2. （可选）部署后立即手动重跑 8-22 各 board 日报复原当日数据
3. 观察数日 section 粒度与 `daily_report.merge` 审计日志 → 后续 change 决策是否以边界模式重启合并及阈值

## Open Questions

（无——边界规则与开关默认值已由 8-22 数据与用户决策收敛；重启合并的阈值问题留给观察期后的新 change）
