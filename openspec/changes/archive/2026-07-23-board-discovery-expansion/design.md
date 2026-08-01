## Context

现状（代码已核实）：

- `SemanticBoardUpgradeService.GenerateSuggestions` 支持两模式：`discover_new`（候选=未归属 aux 标签，决策={create_new, skip}）与 `expand_existing`（候选=已归属标签，决策={create_new, merge_into_existing, skip}）。**"未归属标签 → 并入已有版块"这一扩展象限不存在**：`filterSemanticBoardUpgradeSuggestions` 在非 expand 模式直接丢弃 merge 决策，prompt 也不提供该选项。
- 建议即算即弃：无持久化、无状态机、无 skip 记忆、无调度，仅前端手动触发 `POST upgrade-suggest`。
- Board affinity = 候选 embedding 对版块 composition aux 标签的 min-distance，未归一化（大版块补获面大、信号糊）。
- aux 标签创建已有两级去重（`ResolveAuxiliaryLabel`）：L1 按 `core.Slugify` slug + alias 精确匹配；L2 按 merge_embedding cosine（阈值 `auxiliaryLabelMergeThreshold = 0.95`，const 硬编码不可配）。漏网根因：`Slugify` 把空白压缩成单空格而非删除，"SK 海力士"与"SK海力士"slug 不同，L1 漏；L2 merge_embedding 对字符级变体 cosine 不到 0.95 也漏。

验证报告（脚本 `verification/upgrade_verify.sql`，2026-07-17 对生产库实测）：

| 验证 | 结果 | 结论 |
| --- | --- | --- |
| 发现候选规模 | 135 个（ref≥5 且无版块），其中 130 个近 14 天活跃 | 池子新鲜，问题不在新鲜度 |
| 真值（候选内容实际流向） | 57 个有真值的候选，top 版块占比 50-83% | 未归属 ≠ 无家可归，内容已在"投票" |
| 泳道签名命中率 | 22/57 = 39%（top-1） | 单签名不足以自动裁决 |
| composition 签名命中率 | 29/57 = 51%（top-1，有主场优势） | 同上 |
| 双签名并集 | 35/57 = 61%（各取 top-1） | shortlist + LLM 裁决是正确架构 |
| 泳道签名 margin 分布 | margin>0.05 时基本全对；约 30% 样本 margin<0.01 | margin 可作置信度闸门 |
| `board_upgrade_suggest` 调用 | 106,624 条 AI 日志中 **0 次** | 功能实际从未运行，必须调度化+持久化 |
| aux 文本变体重复 | 184 组 / 380 个标签（含 ×3：SK海力士、2026世界人工智能大会） | Slugify 不去空白致 L1 漏；需补"去空白+lower"归一化键 + 一次性迁移 |
| aux embedding 语义重复 | top500 内仅 8 对（均为 alias 型） | 不需重型语义去重，转 alias 即可 |

## Goals / Non-Goals

**Goals:**

- 补齐扩展象限：`discover_new` 决策空间 = {create_new, merge_into_existing, skip}，merge 目标来自双签名 shortlist，LLM 凭泳道内容证据 + co-tag 事件裁决。
- 建议有生命周期：持久化 + pending/confirmed/dismissed + dismissed 冷却 + scheduler 每日自动生成。
- 数据源卫生：aux 创建归一化查重 + 近重复转 alias；184 组历史文本变体一次性合并。
- 聚类卫生：单标签簇不进 LLM（进观察池落库），≥2 才裁决。

**Non-Goals:**

- 不动 `expand_existing` 模式（重新分配/多挂语义，与本 change 正交；若后续证明冗余再单独废弃）。
- 不动日报话题聚类（candidate 膨胀、consecutive 清零等话题层问题）——那是 topicgraph 域的另一笔账，本 change 只动版块发现/扩展。
- 不做 embedding 级语义去重的自动合并（仅列报告供人工）。
- 不做"标签移出原版块"的 move 操作（merge 仍是"挂入目标版块"的添加语义）。

## Decisions

### D1. 扩展象限落在 `discover_new`，而非改 `expand_existing` 候选集

备选：(a) discover_new 决策空间加 merge；(b) expand_existing 候选集改为"已归属+未归属"混合；(c) 新增第三模式。
选 (a)。理由：(b) 让 expand 模式同时背两种语义（重新分配 + 吸收新血），prompt 复杂度翻倍；(c) 增加前端模式切换成本。discover_new 本来就是用户看"未归属池"的入口，merge 是 missing option 而非新模式。`expand_existing` 保持原样、不在本 change 范围内。

### D2. merge 目标 = 双签名 shortlist，LLM 不自由选版块

验证显示单签名 top-1 命中率 39%/51%，都不配自动裁判；并集 61%。设计：

```
每个簇 → composition 签名 top-2 版块 + 泳道签名 top-2 版块（去重，≤4 个候选版块）
       → 若双签名 top-1 一致 且 margin ≥ merge_confidence_margin（默认 0.05，ai_settings 可配）
         → 直接产出高置信 merge 建议（skip LLM，confidence=high）
       → 否则 → LLM 裁决：prompt 提供 ≤4 个候选版块（标签+描述+active 泳道近期内容摘要）
         + 簇内 aux 标签 + co-tag 事件，决策 ∈ {merge_into_existing(指定 target), create_new, skip}
```

泳道签名实现：版块 active topic 近 30 天 section embedding（现有 `daily_report_sections.embedding`，无需新 embedding 调用），候选 aux embedding 对其取 min-distance。LLM 输出 `target_board_id` 校验必须在 shortlist 内（防幻觉，同现有 matched_topic_id 校验思路）。

备选"让 LLM 自由选任意版块"被否：60+ 版块全量注入 prompt 不可行，且自由发挥无护栏。

### D3. 建议持久化：新表 + 幂等生成 + 冷却

```
board_upgrade_suggestions
  id, batch_id, mode, decision, board_label, description,
  target_board_id NULL, auxiliary_label_ids jsonb, confidence (high|llm),
  evidence jsonb            -- {shortlist, margins, cotag_events, lane_briefs} 快照
  status (pending|confirmed|dismissed), dismiss_reason NULL,
  created_at, resolved_at NULL, resolved_by NULL
  唯一约束: (status='pending') 时 hash(mode, decision, target_board_id, sorted_aux_ids) 唯一
```

- 生成幂等：同集合同决策的 pending 建议已存在则跳过（靠唯一约束 + ON CONFLICT DO NOTHING）。
- dismissed 冷却：同 hash 的 dismissed 建议在 `semantic_board_upgrade_suggestion_dismiss_cooldown_days`（默认 14 天）内不再生成；冷却过后允许再次生成（新证据可能改变判断）。

> 冷却方案选定：表内 status=dismissed 兼任冷却记录，dismissed 行的 resolved_at 即冷却起点，不单建 suggestion_dismissals 表。

- API 统一到 `/upgrade-suggestions` 资源（避免与旧 `upgrade-suggest` 差一个字母混淆）：`GET /upgrade-suggestions?status=&decision=` 读表；`POST /upgrade-suggestions/generate` = 同步跑一轮生成并入表，返回本批新增数（替代旧 `POST upgrade-suggest`，旧路由保留兼容期）；`POST /upgrade-suggestions/:id/dismiss`。confirm 复用现有 `upgrade-execute`，请求体新增 `suggestion_id`，事务内按 id 把对应 pending 建议置 confirmed（失败回滚状态不变）。

### D4. scheduler 每日生成（固定时间，松耦合不保证紧随日报）

新 job `job_board_upgrade_suggest`（admin/scheduler registry 注册，默认每天 06:30 固定时间触发），调用与手动触发生成相同的 service 方法。默认只跑 `discover_new`。失败仅记日志，不阻塞其他 job。理由：验证 V2 证明"无调度=机制不存在"。

时序决策：现有 `scheduler.Registry` 各 job 独立按 wall-clock 触发，无 job 依赖编排能力，无法做到"日报完成即触发本 job"。采用松耦合方案——本 job 固定 06:30（日报默认 21:00，跨夜后必然在其后），不保证紧随日报；日报被 trigger 提前/跨天重跑时，本 job 按自身节奏运行，不强依赖。（Non-Goal：不做日报→本 job 的显式 trigger 链，避免侵入日报 job。）

### D5. 补强现有 aux 创建去重（加归一化键 + L2 阈值可配）

现状 `ResolveAuxiliaryLabel` 已有两级去重（核实代码）：L1 按 `core.Slugify` slug + alias 精确匹配；L2 按 merge_embedding cosine，阈值 `auxiliaryLabelMergeThreshold = 0.95`（const 硬编码）。漏网根因是 `Slugify` 把空白压缩成单空格而非删除，"SK 海力士"/"SK海力士" slug 不同致 L1 漏；L2 merge_embedding 对字符级变体 cosine 不到 0.95 也漏。本 change 做两件小事（非新建闸）：

1. **L1 加第三种匹配键**：在现有 slug / alias 匹配之外，增加"去全部空白 + lower"归一化键匹配（与一次性迁移用同一个归一化函数，避免迁移后新标签又产生变体）。命中既有 active aux → 复用既有标签，不新建。
2. **L2 阈值提为可配**：把 `auxiliaryLabelMergeThreshold` const 提升为 ai_settings `auxiliary_label_dedupe_sim`（默认 0.95，复用现有 merge_embedding 列，不新增 embedding 计算）。

覆盖范围：`ResolveAuxiliaryLabel` 是新建 aux 标签的唯一入口（唯一调用方 `AttachAuxiliaryLabels`，即 LLM 提取入库；系统无 keyword 入库、无手动创建 aux 路径），改动此处即覆盖全量创建路径。

验证 V3：文本变体 184 组 / 380 个是大头，语义重复仅 8 对——L1 归一化键补强后可拦住绝大多数；L2 维持 0.95 兜底语义近重复。

### D6. 一次性迁移：文本变体组合并（复用现有 merge 逻辑，不碰语义对）

迁移 `2026MMDD_000N`（幂等，可重跑）。分组归一化键与 D5 创建闸为**同一函数**（去全部空白 + lower）：

```
按 normalize_key(label) 分组 active aux，HAVING count>1：
  主标签 = ref_count 最大（并列取 id 最小）；其余为从标签
  每个从标签 → 复用现有 MergeAuxiliaryLabelAlias(sourceID=从, targetID=主)：
    该 service 方法已实现：从 label 入主 aliases、topic_tag_semantic_labels 与
    board_composition 引用改指主、从 status=disabled、主 ref_count 重算为 DISTINCT 引用数
  → 迁移只需按分组循环调用，不重写合并逻辑
迁移末尾：packageBoardCache.InvalidateBoardData()（board_composition 已变，避免首轮 shortlist 用旧 composition）
产出迁移报告（log）：每组 主/从/ref 变化；另列 8 对语义 alias 对供人工，不自动处理。
```

从标签不物理删（disabled 保留审计），与现有 merge API 语义一致。注：现有 `MergeAuxiliaryLabelAlias` 主/从由调用方指定，迁移按 ref_count 最大者定主，策略差异在调用层处理。

### D7. 单标签簇观察池

聚类后 size=1 的簇不进 LLM，直接落库为 pending 建议（decision=`watch`，不进前端默认列表，仅 `?status=pending&decision=watch` 可见）；下次生成时若该标签与其他新候选成簇，正常参与裁决并关闭对应 watch 建议。理由：验证显示单标签簇占多数，逐个喂 LLM 是浪费 + 噪声（与日报 singleton section 同病）。

watch GC：watch 建议创建满 `semantic_board_upgrade_watch_gc_days`（默认 30 天，ai_settings 可配）仍未成簇的，自动置 dismissed，避免观察池单调膨胀。

## Risks / Trade-offs

- [泳道签名引入新噪声：版块 active 泳道仅 3-9 条，签名可能偏窄] → 只做 shortlist 不做自动裁判；margin 置信度闸门；泳道无 section 的版块自动降级为仅 composition 签名。
- [merge 建议变多可能打扰用户] → dismissed 冷却 + 高置信才跳过 LLM；前端默认列表按置信度排序。
- [一次性迁移改 board_composition 影响匹配结果] → 迁移只合并"归一化后完全同名"的组（语义等价，安全）；迁移前后各跑一次 `verify`：受影响版块 composition 数量变化纳入迁移日志；迁移可重跑但不可回滚 → 执行前 `pg_dump` 相关表（运维手册注明）。
- [创建闸 embedding 检查拖慢打标管线] → 第 1 级文本查重命中绝大多数；第 2 级仅在文本未命中时触发（新实体量小）；加计时日志，超 50ms 告警。
- [watch 决策增加前端复杂度] → 默认列表过滤掉 watch，几乎无 UI 成本。

## Migration Plan

1. 先上 schema 迁移（suggestions 表）+ D6 dup 合并迁移（同事务不可行则先表后数据，两步均幂等）。
2. 后端：创建闸 → 建议持久化/生成/裁决 → scheduler job → API。创建闸先于生成上线，防止新 dup 继续产生。
3. 前端：建议面板改造。
4. 回滚：suggestions 表可 DROP；dup 迁移不可自动回滚（aliases 保留信息可人工还原）；代码回滚不影响已合并数据（语义等价）。
5. 灰度：单用户系统，直接全量；scheduler job 首日观察日志。

## Open Questions

- `semantic_board_upgrade_merge_confidence_margin` 默认 0.05 是否需要按版块分别调？先全局单值，上线后按建议准确率回看。（口径已定：双签名 top-1 一致，且两签名各自 margin 均 ≥ 阈值，见 spec board-upgrade。）
- `expand_existing` 未来是否废弃并入统一模式？留到下一次评估，本 change 不动。
