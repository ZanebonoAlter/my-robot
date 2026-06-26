## ADDED Requirements

### Requirement: Section 质量血缘快照

日报生成管线 SHALL 在 section 组装时，把该 section 所属全部来源 tag 的匹配明细快照为 `quality_breakdown` JSON 列持久化到 `daily_report_sections`。快照 SHALL 在**生成时刻冻结**（不随后续 rematch 漂移），结构为 `[{tag_id, label, match_reason, score, downgraded}]`。

`collectBoardTags` 查询 SHALL 取 `topic_tag_board_labels` 的 `downgraded` 列，并填入 `TagInput.Downgraded` 字段（修复当前 downgraded 进管线即丢失的缺陷）。`filterTagsByQuality` 的截断排序 SHALL 使用各 tag 真实的 `downgraded` 标记调用 `MatchTier`（修复当前硬编码 `MatchTier(reason, false)` 导致降级 max_sim 错误落到 tier=2 的缺陷）。

`MergeSimilarSections` 合并多个 section 后，SHALL 按合并后的 tag 集合重算 `quality_breakdown`（与重算 `avg_score` 同一位置、同一规则）。

历史 section 的 `quality_breakdown` SHALL 保持 NULL（不回刷），前端降级显示"无质量明细"。

#### Scenario: 新日报写入质量明细
- **WHEN** 新日报生成，某 section 聚合自 3 个 tag：AI芯片(direct_hit, 1.00, downgraded=false)、GPT-5发布(max_sim, 0.85, downgraded=false)、AI竞赛(weighted, 0.59, downgraded=false)
- **THEN** 该 section 行的 `quality_breakdown` SHALL 持久化为包含这 3 条明细的 JSON 数组，每条含 tag_id/label/match_reason/score/downgraded

#### Scenario: 降级标记不再丢失
- **WHEN** 某 tag 在 topic_tag_board_labels 中为 max_sim 且 downgraded=true（score=0.82），被收入日报
- **THEN** 其 `TagInput.Downgraded` SHALL 为 true，且 `quality_breakdown` 中对应条目 downgraded=true

#### Scenario: 截断排序使用真实降级标记
- **GIVEN** board 收集到 >30 个 tag 需截断
- **WHEN** filterTagsByQuality 执行截断排序
- **THEN** 降级的 max_sim tag SHALL 按 tier=3 排序（而非硬编码的 tier=2），可能被正确排在截断边界之后

#### Scenario: 合并后重算明细
- **WHEN** MergeSimilarSections 把 section A（来源 tag {1,2}）与 section B（来源 tag {3,4}）合并
- **THEN** 合并后 section 的 `quality_breakdown` SHALL 包含 tag {1,2,3,4} 四条明细，avg_score 与之保持一致

#### Scenario: 历史无明细降级
- **WHEN** 查询变更上线前的历史 section
- **THEN** 其 `quality_breakdown` SHALL 为 NULL

### Requirement: MatchTier 质量等级语义规格化

系统 SHALL 按以下映射将 `match_reason` + `downgraded` 归并为质量等级 tier（越小越好），作为 `best_tier` 排序与人话翻译的唯一依据：

- `direct_hit` → tier 0
- `hit_rate` → tier 1
- `max_sim` 且 `downgraded=false` → tier 2
- `max_sim` 且 `downgraded=true` → tier 3
- `weighted`（或未知）→ tier 3

`MatchTier(matchReason string, downgraded bool) int`（`tagmanagement/handler/board_match_handler.go`）SHALL 实现该映射。`best_tier`（section 级）SHALL 为该 section 全部来源 tag 中最优（最小）的 tier。前端 `sortDailyReportSections` 的 `best_tier` 升序排序 SHALL 因此把 direct_hit 主导的 section 排最前。

#### Scenario: direct_hit 最优
- **WHEN** 某 tag 的 match_reason=direct_hit
- **THEN** MatchTier SHALL 返回 0

#### Scenario: 降级 max_sim 落到最弱
- **WHEN** 某 tag 的 match_reason=max_sim 且 downgraded=true
- **THEN** MatchTier SHALL 返回 3

#### Scenario: section best_tier 取组内最优
- **GIVEN** 某 section 由 3 个 tag 组成：tier 0、tier 2、tier 3
- **WHEN** 组装 section 的 best_tier
- **THEN** best_tier SHALL 为 0（组内最优）

#### Scenario: best_tier 排序含义
- **WHEN** 前端按 best_tier 升序排序 sections
- **THEN** direct_hit 主导（best_tier=0）的 section SHALL 排在 weighted 主导（best_tier=3）的 section 之前

### Requirement: 日报 Section 质量明细 API 暴露

日报 section 相关接口（detail / timeline / lifeline）SHALL 在 section 表示中暴露 `quality_breakdown`（可空）字段，供前端展示匹配血缘明细。

`quality_breakdown` 为 NULL（历史 section）时，接口 SHALL 正常返回该字段为 null，不报错。

#### Scenario: section 详情含质量明细
- **WHEN** 前端请求日报 section 列表或 section 详情
- **THEN** 每个 section SHALL 返回 `quality_breakdown`（数组或 null），每条含 tag_id/label/match_reason/score/downgraded

#### Scenario: 历史 section 返回 null
- **WHEN** 前端请求变更上线前的历史 section
- **THEN** 其 `quality_breakdown` SHALL 为 null，接口不报错

### Requirement: 日报正文 Tier 徽章展示

日报正文每个 section SHALL 展示一个基于 `best_tier` 的极简徽章，**仅以色彩/形态区分层级、不展示任何分数或百分比文字**，避免破坏沉浸阅读：

- best_tier=0 → 实心点，绿色 token
- best_tier=1 → 实心点，蓝色 token
- best_tier=2 → 实心点，橙色 token
- best_tier=3 → 空心点，灰色 token

徽章颜色 SHALL 由主题语义 token（`--color-*`）派生，跟随 editorial/dark 双主题，不写死色值。历史 section（quality_breakdown 为 null）但 best_tier 仍存在的，SHALL 正常展示徽章（best_tier 是独立冻结字段）。

分数、匹配方式文字、降级标记 SHALL 仅在探究区（hover/详情）展示，不进正文徽章。

#### Scenario: 正文徽章仅色彩无数字
- **WHEN** 渲染 best_tier=0 的 section 徽章
- **THEN** 徽章 SHALL 显示为绿色实心点，且不包含任何分数/百分比/匹配方式文字

#### Scenario: 保底徽章空心
- **WHEN** 渲染 best_tier=3 的 section 徽章
- **THEN** 徽章 SHALL 显示为灰色空心点

#### Scenario: 徽章跟随双主题
- **WHEN** 切换 editorial 与 dark 主题
- **THEN** 徽章四态颜色 SHALL 分别跟随对应主题 token，两主题下均可区分

#### Scenario: 探究区展示明细
- **WHEN** 用户 hover 或展开某 section 的质量探究区
- **THEN** SHALL 展示 quality_breakdown 中每条 tag 的 match_reason 色彩 + score + 降级标记（降级用 50% 不透明边框 + "↓" 后缀）

#### Scenario: 历史 section 徽章仍展示
- **WHEN** 历史 section 的 quality_breakdown 为 null 但 best_tier=1
- **THEN** 正文 SHALL 正常展示 best_tier=1 的蓝色实心徽章，探究区显示“无质量明细”

### Requirement: 持久话题锚定的双重确认容忍聚类漂移

日报 section 锚定到持久话题（persistent topic）的双重确认机制 SHALL 容忍 LLM 聚类的轻微漂移：当 section 的 embedding 在某 active topic 的匹配阈值（`MatchThreshold`）内，且 LLM 的 `matched_topic_id` 指向**阈值内任一** topic（不要求是 embedding 最近邻）时，SHALL 判定为锚定成功（`anchor_hit`），锚定到 LLM 指向的那个 topic。

先前要求 `matched_topic_id` 必须等于 embedding 单一最近邻的严格规则 SHALL 被放宽——该规则使任何 LLM 聚类变化（如质量排序截断改动导致进聚类的 tag 集合变化）都会令 `matched_topic_id` 落到第 2 近的 topic，从而大面积打断话题血缘、令 section 误判为全新突发话题。

两道闸门仍同时生效：embedding 距离 ≤ 阈值 AND LLM 指向同一 topic。此放宽 **不是** 纯 embedding 匹配，也 **不是** 纯 LLM 匹配。

#### Scenario: LLM 选了阈值内第 2 近的 topic 仍锚定
- **GIVEN** 两个 active topic：T1（embedding 最近）、T2（embedding 第 2 近，仍在 MatchThreshold 内）
- **WHEN** 某 section 的 LLM matched_topic_id 指向 T2（因聚类漂移）
- **THEN** 该 section SHALL 锚定到 T2（anchor_hit），使用 T2 的 embedding 距离，而非开新 candidate

#### Scenario: LLM 选的 topic 超出阈值仍开新 candidate
- **GIVEN** active topic T1 在阈值内、T2 在阈值外
- **WHEN** section 的 LLM matched_topic_id 指向 T2
- **THEN** embedding 闸门拒绝 T2，该 section SHALL 开新 candidate（auto_new），不锚定

#### Scenario: LLM 选了最近邻仍锚定（不回归）
- **GIVEN** active topic T1 为 embedding 最近邻且在阈值内
- **WHEN** section 的 LLM matched_topic_id 指向 T1
- **THEN** 该 section SHALL 锚定到 T1（anchor_hit）
