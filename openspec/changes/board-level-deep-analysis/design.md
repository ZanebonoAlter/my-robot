# Design: board-level-deep-analysis

## Context

数据增强现有三角色编排（解读员→探索 agent→分析员）、三表存储（背景记忆/结果快照/review）、5 个探索工具均以**单泳道（persistent topic）**为粒度运转。探索阶段（见 `docs/research/board-analysis-reference-role/`）确认的三个断点：触发粒度无版块入口、跨泳道探索无引用槽位（evidence 仅 `news|web|page`）、分析无方法论锚。参考角色文档的实物原型（《内部看美国·方法论画像》）已由 opencli + yt-dlp + faster-whisper 管线在 research 验证产出。

## Goals / Non-Goals

**Goals:**

- 版块级分析最小可用：手动触发、命题生成、跨泳道论据织入、lane 引用下钻、review 复用
- 参考角色库最小可用：手工 CRUD + 三角色注入 + 用量控制
- 单泳道分析无行为破坏，仅增下钻入口

**Non-Goals:**

- 定时/事件驱动的版块分析节奏（后置）
- 泳道治理建议（冗余检测/合并/归档状态机，后置）
- 参考角色自动生产线（opencli/ASR 管线产品化，后置）
- FinGenius 遗留清理（另行处理）

## Decisions

### D1 版块级报告复用 `topic_enrichment_result` 表（加 scope 列），不建新表

- `topic_enrichment_result` 加 `semantic_board_id *uint`（NULL=单泳道档）+ `analysis_scope string`（`topic|board`，默认 `topic`）。`persistent_topic_id` 改可空。
- 理由：review judge（对比两份快照）、QA 追问、append-only 不可变语义全部直接复用，API 形态一致；新表则要复制整套对比/追问链路。
- 产物 JSON：`sectors` jsonb 继续作为载体存 `{scope:'board', thesis, candidates[], argument, depth, lane_refs[]}`——沿用「sectors 实为单对象」的现状形状，前端按 scope 分支渲染。
- 备选否决：新表 `board_enrichment_result`——对比逻辑重复实现，收益仅语义清晰。

### D2 态势卡：机械拼装，不加 LLM pass

- 态势卡 = 循环 A lifeline week 粒度（缺失时降级近期 section 事实指纹）+ 命中统计（hit_count/consecutive_hits/last_seen）+ 质量信号。每卡 ~100 字，纯机械拼装零 LLM 成本。
- agent 需要论据细节时经既有 `get_lane_detail` 下钻——该工具已复用 `RenderLifelineForAgent` 渲染，无需新建。
- 备选否决：先跑一轮 LLM「泳道态势压缩」pass——多一次 LLM 调用且产物即 lifeline 已有内容，重复投资。

### D3 命题生成：新增 `data_enrichment.board_interpret` Operation

- 输入：全板块态势卡集合 + 参考角色文档（注入方法锚）+ 历史 applied review（板块级）。
- 输出：`{candidates: [{thesis, hook, angle}], chosen_index, reason}`——「钩子 × 切角」公式（钩子=板块近期异常/新涌现素材，切角=参考角色的命题生成模式）。
- 探索/分析阶段**复用**既有 `tool_use` / `analyze` Operation（prompt 按版块形态分支：论证骨架=层级递进机制层，织入 lane 论据），不另造 Operation。
- SessionID：`data_enrichment_board_{board_id}_{uuid8}`。

### D4 evidence `source_type` 扩 `lane`

- `lane` 证据携带 `{lane_id, note}`；`web`/`page`/`news` 行为不变。校验：lane 证据的 lane_id MUST 属于本板块活跃泳道集合（防跨板块幽灵引用）。

### D5 参考角色库：独立小表 + 装配处统一注入

- 新表 `reference_roles(id, name unique, content text, enabled bool default true, created_at, updated_at)`。
- 注入点：三角色 prompt 装配的公共出口（orchestrator 内单点函数），注入形态为 markdown 附录段「分析方法参考」。上限：注入总量 ~4k 字符，超出按 `updated_at DESC` 截断，截断记录写 ai_call_logs（input_snapshot 已留痕，补一条结构化日志）。
- 证据链不出现参考角色类型（spec 已锁：角色给方法不给事实）。

### D6 泳道质量信号：查询期计算，不落库

- 质量信号（活跃度/密度/sparse 历史）在装配态势卡时现查（hit 统计 + lifeline 存在性 + 历史结果 form 统计），不建质量分表——本期仅用于注入排序与卡片详略，无治理消费方，落库是过度设计。

### D7 前端：版块分析为主视图，单泳道入口收拢

- `BoardEnrichmentPanel` 重排：顶部为「版块分析」（最新报告 + 触发 + 历史列表），单泳道选择下拉移入「聚焦分析」折叠区；版块报告组件新增论证段泳道引用点击（→ 聚焦分析预填 lens 触发）。
- 参考角色管理放设置页（对齐博查配置的 section 模式）。

### D8 API 增量

- `POST /semantic-boards/:id/enrichment/analysis/trigger`（版块级触发）、`GET /semantic-boards/:id/enrichment/analysis/results`（列表/详情，复用 result 序列化）。
- 参考角色：`/reference-roles` 标准 CRUD。
- 单泳道 trigger 接口加可选 body `{prefill_lens}`（下钻入口用）。

## Risks / Trade-offs

- [版块分析 token 成本高于单泳道] → 态势卡压缩 + 质量加权排序 + max_loops 沿用；analyze 版块形态分支控制论证长度上限
- [`sectors` jsonb 复用导致 schema 双形并存] → `analysis_scope` 字段显式分派，前端按 scope 渲染分支；旧数据不受影响（默认 topic）
- [命题质量早期不稳] → 候选清单全量透出（不黑箱选定）；参考角色文档可迭代替换；首份画像已实物产出
- [result 表迁移风险] → 全部新列可空/带默认，向后兼容；无数据回填需求

## Migration Plan

1. 迁移：`topic_enrichment_result` 加列（可空/默认值）+ 新表 `reference_roles`——非破坏性，部署即生效。
2. 旧单泳道报告：`analysis_scope` 默认 `topic`，前端渲染路径不变。
3. 回滚：列可空无回填，回滚迁移安全；参考角色库独立，停用（enabled=false）即等效功能关闭。

## Open Questions

- 版块分析的 `enrichment_enabled` 是否需要独立于单泳道的开关粒度（board vs 全局）——实现时看现有开关位置再定，不阻塞。
- 候选命题数量上限（2-3）是否需要按板块规模自适应——首版固定 3。
