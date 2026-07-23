## Why

版块发现/扩展机制在算法与工程上双重失灵，且有实测数据佐证（验证脚本：`verification/upgrade_verify.sql`，结果见 design.md「验证报告」）：

1. **"扩展"象限缺失**：`discover_new` 模式下 LLM 只许答 `create_new / skip`，`merge_into_existing` 被 prompt 与 filter 双重禁止（`semantic_board_upgrade.go`）。实测 135 个未归属候选 aux 标签中，大量（DeepSeek/Agent/GPT-5.6/月之暗面…）的内容今天已通过其他 aux 标签流入既有版块（真值 top 版块占比 50-83%），但没有任何路径能把这些标签并入对应版块——LLM 看到 board_affinity 也只能 skip，且 skip 不落库，下次原样重复。
2. **该功能实际从未运行**：`ai_call_logs` 共 106,624 条，`board_upgrade_suggest` **0 次**。建议即算即弃、无持久化、无调度、无 skip 记忆——发现池在无人观测中膨胀。
3. **aux 标签创建无去重闸**：实测 **184 组 / 380 个**文本变体重复（SK海力士×3、2026世界人工智能大会×3、标普500/标普 500），ref_count 被劈叉、聚类候选被污染；语义级重复极少（top500 内仅 8 对，均为 alias 型），说明只需创建侧归一化查重，不需重型方案。
4. **归属判断信号单一**：现有 board_affinity 仅用 composition aux 标签袋 min-distance（对真值命中率 51%），未利用各版块 active 泳道的近期真实内容（泳道签名命中率 39%，两者并集 61%）——单一签名都不足以自动裁决，但适合做候选名单 + LLM 证据裁决。

## What Changes

四块，围绕"把扩展象限补齐、让建议有生命周期、把数据源洗干净"：

### A. 扩展决策空间补齐（算法主线）

- `discover_new` 模式决策空间由 `{create_new, skip}` 扩展为 `{create_new, merge_into_existing, skip}`，解除 prompt 与 filter 对 merge 的双重禁止。
- merge 目标不由 LLM 自由发挥：后端用**双签名 shortlist**（composition 签名 + 泳道签名，各取 top-2 候选版块），LLM 只在 shortlist 内裁决或判 create_new/skip。
- 裁决证据升级：prompt 注入候选版块的 **active 泳道近期内容摘要**（复用 topic briefs 思路）+ 现有 co-tag 事件；保留 board_affinity 数值。
- 置信度分流：双签名一致且 margin 超过阈值的簇，直接产出高置信 merge 建议（标注 `high_confidence`），低置信进 LLM 裁决。
- 聚类卫生：单标签簇不进 LLM，进"观察池"直接落库为 pending 建议（下次新标签加入成簇后再裁决）；≥2 标签的簇才进 LLM。

### B. 建议持久化与生命周期（让机制真正转起来）

- 新增 `board_upgrade_suggestions` 表：建议落库，状态机 `pending → confirmed / dismissed`，记录决策、目标版块、aux 标签集合、置信度、证据快照。
- `GET /api/semantic-boards/upgrade-suggestions` 改为读表（分页/按状态过滤），`POST /api/semantic-boards/upgrade-suggestions/generate` 生成新一批建议并入表（幂等：同集合同决策的 pending 建议不重复插入；替代旧 `POST upgrade-suggest`，旧路由保留兼容期）。
- dismissed 冷却期：被 dismiss 的（候选集合， 决策） 在冷却窗口（默认 14 天，可配）内不再生成相同建议。
- 新增 scheduler 定期任务（默认每天 06:30 固定时间，可配，松耦合不保证紧随日报），自动跑生成；`POST /upgrade-suggestions/generate` 为手动触发入口。

### C. 补强 aux 标签创建去重 + 历史迁移

- 创建侧：现状 `ResolveAuxiliaryLabel` 已有 L1(Slugify+alias)/L2(merge_embedding 0.95 const) 两级去重，但 `Slugify` 不去空白致"SK 海力士/SK海力士"漏网。本 change 在 L1 增加"去空白+lower"归一化键（与迁移同函数），并把 L2 的 0.95 const 提为 ai_settings 可配。
- 一次性迁移：合并 184 组文本变体重复（归一化键分组，复用现有 `MergeAuxiliaryLabelAlias`，主=ref_count 最大），迁移末尾 invalidate 版块缓存。
- 语义级 alias 对（8 对，如 哈梅内伊/阿里·哈梅内伊）不在自动迁移范围，仅在迁移报告中列出供人工确认。

### D. 前端升级建议面板适配

- 建议列表改读持久化接口，新增决策过滤（全部/merge/create_new）、dismiss 操作、置信度标识、merge 建议的目标版块与证据展示（泳道内容摘要 + co-tag 事件）。
- 保留现有手动合并下拉，不动 `expand_existing` 模式（重新分配语义，与本 change 正交，后续单评）。

## Capabilities

### New Capabilities

- `board-upgrade-suggestions`: 升级建议的持久化存储、状态生命周期（pending/confirmed/dismissed）、dismissed 冷却、scheduler 定期生成、建议查询 API。

### Modified Capabilities

- `board-upgrade`: `discover_new` 决策空间扩展 merge_into_existing（**推翻现有「LLM SHALL NOT 产出 merge_into_existing」需求**）；双签名 shortlist；泳道内容证据注入；单标签簇观察池；置信度分流。
- `auxiliary-label`: 现有创建去重补强（L1 增加"去空白+lower"归一化键、L2 的 0.95 const 提为可配 `auxiliary_label_dedupe_sim`）；一次性文本变体重复合并迁移需求（复用 MergeAuxiliaryLabelAlias）。

## Impact

- **后端**：`internal/tagmanagement/service/board/semantic_board_upgrade.go`（决策空间、shortlist、证据、观察池）、`internal/tagmanagement/service/auxlabel/`（创建闸）、`internal/tagmanagement/handler/`（建议查询/状态 API）、`internal/admin/scheduler/`（新 job 注册）、迁移（`postgres_migrations.go` 新增 suggestions 表 + 一次性 dup 合并迁移）。
- **前端**：`front/app/features/tags/components/SemanticBoardPanel.vue`（建议列表重构）、`front/app/api/`（新接口封装）。
- **API**：统一到 `/upgrade-suggestions` 资源——新增 `GET /api/semantic-boards/upgrade-suggestions`（列表）、`POST .../upgrade-suggestions/generate`（生成入表，替代旧 `POST upgrade-suggest`）、`POST .../upgrade-suggestions/:id/dismiss`；`upgrade-execute` 请求体新增 `suggestion_id` 用于 confirm 联动；`GET upgrade-candidates` 保留。
- **数据**：一次性迁移合并 184 组 aux 重复（影响 topic_tag_semantic_labels、board_composition、semantic_labels）；迁移末尾 invalidate 版块缓存。
- **部署后影响**（用户可见）：升级建议面板从"手动临时算"变为"每天自动攒建议"；merge 建议开始出现；旧数据经迁移后 aux 标签数量减少约 370 个（重复合并），无用户手动操作需求。
