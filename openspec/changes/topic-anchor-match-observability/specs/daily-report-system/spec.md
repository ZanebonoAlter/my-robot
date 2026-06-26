## ADDED Requirements

### Requirement: 日报 Section 正文话题锚定紧实度徽章

日报正文每个 section 卡片头部 SHALL 在现有 tier 徽章（System 1，标签↔板块）旁并列展示一个**话题锚定紧实度徽章**（System 2，section↔持久话题），**仅以形态/色彩区分紧实度、不展示任何距离数值或百分比文字**，避免破坏沉浸阅读。两徽章须视觉可区分（锚定徽章尺寸 SHALL 不大于 tier 徽章）。

锚定徽章形态 SHALL 由 `topic_match_confidence`（主判据）与 `topic_match_distance`（紧实度细分）共同决定，分档如下（双阈值 0.05/0.15，对齐 2026-06-26 实测三段聚集）：

- `confidence=anchor_hit` 且 `distance ≤ 0.05` → 实心点，accent 强调 token（极紧锚定）
- `confidence=anchor_hit` 且 `distance ∈ (0.05, 0.15]` → 半透明点，accent 强调 token（稳锚定）
- `confidence=anchor_hit` 且 `distance ∈ (0.15, 0.30]` → 淡半透明点，accent 强调 token（松锚定）
- `confidence=auto_new` → 空心点，accent 强调 token（新话题候选）
- `confidence=unmatched`，或无 `topic_match_distance`，或 `topic_match_distance` 为零值缺省 → 空心点，灰 token（未锚定）

徽章颜色 SHALL 由主题语义 token 派生，跟随 editorial/dark 双主题，不写死色值。距离数值、话题名、中文标签 SHALL 仅在探究区展示，不进正文徽章。

#### Scenario: 极紧锚定 section 渲染实心 accent 点
- **GIVEN** 某 section `topic_match_confidence=anchor_hit`，`topic_match_distance=0.02`
- **THEN** 正文锚定徽章 SHALL 为实心点、accent token 色、尺寸 ≤ tier 徽章，且不出现任何数字

#### Scenario: 稳锚定 section 渲染半透明点
- **GIVEN** 某 section `topic_match_confidence=anchor_hit`，`topic_match_distance=0.10`
- **THEN** 正文锚定徽章 SHALL 为半透明点（accent token 55% 不透明）、与极紧实心点视觉可区分

#### Scenario: 松锚定 section 渲染淡半透明点
- **GIVEN** 某 section `topic_match_confidence=anchor_hit`，`topic_match_distance=0.27`
- **THEN** 正文锚定徽章 SHALL 为淡半透明点（accent token 30% 不透明）、与稳锚半透明点视觉可区分

#### Scenario: 新话题候选渲染空心 accent 点
- **GIVEN** 某 section `topic_match_confidence=auto_new`
- **THEN** 正文锚定徽章 SHALL 为空心点、accent token 色，形态区别于 anchor_hit 的实心/半透明

#### Scenario: 未锚定或历史 section 渲染空心灰点
- **GIVEN** 某 section `topic_match_confidence=unmatched`，或缺失 `topic_match_distance`
- **THEN** 正文锚定徽章 SHALL 为空心点、灰 token 色，且不报错

#### Scenario: 锚定徽章与 tier 徽章并列不混淆
- **WHEN** 渲染 section 卡片头部
- **THEN** tier 徽章（System 1）与锚定徽章（System 2）SHALL 同时并列呈现、各自独立可辨，锚定徽章尺寸 SHALL 不大于 tier 徽章

#### Scenario: 紧实度分档双阈值边界值
- **WHEN** `topic_match_confidence=anchor_hit` 且 `topic_match_distance` 恰为 0.05
- **THEN** 该 section SHALL 归入极紧锚定档（实心点）
- **AND WHEN** `topic_match_distance` 恰为 0.15
- **THEN** 该 section SHALL 归入稳锚定档（半透明点）

### Requirement: 日报 Section 探究区话题锚定行

日报 section 的 hover 探究区（`SectionQualityExplore` 探针）SHALL 在 per-tag 质量明细列表**上方**展示一行话题锚定信息，内容为：话题名 + 余弦距离数值 + 中文紧实度标签。该行 SHALL 仅在存在话题锚定数据（`topic_match_confidence` 非 unmatched 且 `topic_match_distance` 有效）时渲染；否则不渲染该行（探针的 per-tag 明细或"无质量明细"占位行为不受影响）。

中文标签 SHALL 与正文徽章分档语义一致：`anchor_hit 且 distance≤0.05`→"极紧锚定"、`anchor_hit 且 0.05<distance≤0.15`→"稳锚定"、`anchor_hit 且 0.15<distance≤0.30`→"松锚定"、`auto_new`→"新话题候选"、`unmatched`/缺失→不渲染行。距离数值 SHALL 保留两位小数。

话题名 SHALL 取自 `persistent_topic.label`（经 detail/timeline/lifeline 接口已附带的 `PersistentTopicBrief`）；缺失时降级显示 cluster_label 或"未命名话题"，不报错。

#### Scenario: 探究区展示完整锚定行
- **GIVEN** 某 section 锚定到话题"霍尔木兹海峡"，`topic_match_distance=0.03`，`confidence=anchor_hit`
- **WHEN** 用户 hover/focus 展开探究区
- **THEN** 探针顶部 SHALL 渲染一行，含话题名"霍尔木兹海峡"、距离"0.03"、标签"极紧锚定"

#### Scenario: 稳锚与松锚行标签区分
- **GIVEN** 某 section `topic_match_distance=0.10`，`confidence=anchor_hit`
- **THEN** 探究区锚定行标签 SHALL 为"稳锚定"
- **AND GIVEN** 某 section `topic_match_distance=0.27`，`confidence=anchor_hit`
- **THEN** 探究区锚定行标签 SHALL 为"松锚定"，与"稳锚定""极紧锚定"三档可区分

#### Scenario: 新候选行标签
- **GIVEN** 某 section `confidence=auto_new`
- **THEN** 探究区锚定行标签 SHALL 为"新话题候选"，且 distance 数值 SHALL 仍展示（为到最近邻话题的距离）

#### Scenario: 未锚定 section 不渲染锚定行
- **GIVEN** 某 section `confidence=unmatched` 或缺失 `topic_match_distance`
- **THEN** 探究区 SHALL 不渲染话题锚定行，且不报错；per-tag 明细或"无质量明细"占位行为不受影响

#### Scenario: 话题名缺失降级
- **GIVEN** 某 section 有有效 distance 但 `persistent_topic.label` 缺失
- **THEN** 锚定行话题名 SHALL 降级为 cluster_label 或"未命名话题"，整行仍正常渲染
