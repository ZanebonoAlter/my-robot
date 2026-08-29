# 白盒用例 · board-level-deep-analysis（修订版）

> 复杂档白盒用例文档（开发执行规范 §2）。2026-08-28 用户验收将主线修订为“版块简报→用户选题→多假设调查”。黑盒行为见 specs；本文只补内部边界、失败分支与兼容矩阵。

## M1 态势卡与补全门（保留当前实现回归）

| # | 条件 | 期望 | 判据 |
|---|---|---|---|
| M1.1 | 有 week lifeline | 卡片优先使用 week 摘要 | facts_source=lifeline_week，不透传全文 |
| M1.2 | week 缺失但有 month lifeline | 卡片使用 month 摘要 | facts_source=lifeline_month，补全成果被消费 |
| M1.3 | 无 lifeline 但有近期 section 正文/摘要 | 降级实质事实指纹 | 不得只输出“泳道名×篇数” |
| M1.4 | 无 lifeline 且无可读 section | 再降级 description/none | 不报错、不伪造内容 |
| M1.5 | 多泳道质量排序/≥10 泳道 | 高质量在前且输入预算受控 | 质量只影响排序，不进入关系依据 |
| M1.6 | month/year 有数据周期缺行（含首份） | 串行补建后装配 | missing_period 被刷新；week 不检查 |
| M1.7 | month/year 已有行最后写入 >72h | 重算 | stale_row 被刷新，as_of 不晚于 now |
| M1.8 | 粒度无 section 数据 | 跳过 | action=skip_no_data，不建空档 |
| M1.9 | 补全需求 >40 | 超额降级 | action=budget_exhausted，分析继续 |
| M1.10 | 补全失败 | 用旧卡继续 | 日志含 topic/granularity/period/error |
| M1.11 | 同日二次触发 | 新鲜档不重复补 | 幂等 |

## M2 board_brief 生成

| # | 条件 | 期望 | 判据 |
|---|---|---|---|
| M2.1 | 正常多泳道 | summary + observations + relationships + uncertainties + research_questions | 无 thesis/argument/depth 必填 |
| M2.2 | 泳道均有内容但无关系 | 正常 brief | relationships 可空或 unclear/context_only，不走 sparse |
| M2.3 | 多个方向分化 | 产 divergent/并行观察 | 不压成单一命题 |
| M2.4 | 全部无观察 | 素材不足 brief | research_questions 为空 |
| M2.5 | LLM 非法 JSON | 重试一次 | 第二次仍错走机械观察降级 |
| M2.6 | 机械降级 | 只列高质量观察 | 不造关系、不造研究问题 |
| M2.7 | observations >5 / relationships >6 / questions >4 | parser 截断或拒绝重试 | 上限稳定可测 |
| M2.8 | prompt snapshot | 无工具说明、无方法卡全文、无“X不是A而是B”要求 | 默认触发工具调用次数=0 |
| M2.9 | 旧 applied review 存在 | 只作为偏差提醒 | 不把旧 thesis 当本次事实 |

## M3 关系与引用校验

| # | 输入 | 期望 |
|---|---|---|
| M3.1 | 合法 common_driver/possible_causal/divergent/context_only/unclear | 保留 |
| M3.2 | 未知 relation type | 降级 unclear 或拒绝该条并留痕 |
| M3.3 | lane_ids 含非活跃 lane | 拒绝整条关系或清理幽灵 id，不污染其他条目 |
| M3.4 | evidence_refs 指向不存在 observation | 拒绝悬空 ref |
| M3.5 | possible_causal 无解释/低证据 | 降级 unclear，不能提升置信 |
| M3.6 | 质量分高但无关系证据 | 不建立关系 | 质量信号不得充当依据 |

## M4 假设生成

| # | 条件 | 期望 | 判据 |
|---|---|---|---|
| M4.1 | generated question | 2-4 hypotheses | 含 question source=generated |
| M4.2 | custom question | 同一链路 | 保存原始文本，source=custom |
| M4.3 | 无零假设 | 重试；仍缺则机械补 H0 | 进入研究前 `is_null=true` 至少 1 个 |
| M4.4 | 所有假设只是宏大叙事同义改写 | 判无竞争性并重试 | H0/普通解释必须独立 |
| M4.5 | 假设缺 support_needed/disconfirm_needed/scope | 解析失败 | 不进入研究循环 |
| M4.6 | 5 个以上假设 | 截断或重试至 2-4 | 控制研究预算 |

## M5 共享研究循环

| # | 条件 | 期望 | 判据 |
|---|---|---|---|
| M5.1 | 4 个假设 | 只启动一个共享 tool loop | 非每假设一套 loop |
| M5.2 | 研究计划 | 含基础事实、中性查询、支持与反证任务 | 不得所有查询都含某个结论词 |
| M5.3 | 非零假设 | 至少尝试一次反证/替代核查 | 无结果记 gap，不伪造 counter evidence |
| M5.4 | 同 tool+args 重复 | 拦截 | 既有三防御回归 |
| M5.5 | web_search 未配置 | 继续内部研究 | result 记录 external gap |
| M5.6 | fetch_page 失败 | 不把 snippet 当原文 quote | 证据质量降级 |
| M5.7 | 历史类比与问题无关 | 不检索或不纳入结论 | 无证据类型配额驱动 |

## M6 调查综合与持久化

| # | 条件 | 期望 | 判据 |
|---|---|---|---|
| M6.1 | 某非零假设证据充分且反证弱 | supported/plausible | confidence 与证据质量匹配 |
| M6.2 | 支持和反证相当 | plausible/insufficient | 两边均保留 |
| M6.3 | 反证直接否定 | weakened/refuted | 不因初始排序保留赢家地位 |
| M6.4 | 所有非零假设不足 | H0 可最可信或 conclusion=insufficient | 不强选宏大结论 |
| M6.5 | 初始假设需拆分 | 最终可新增/拆分说明 | 保留初始与最终差异可追溯 |
| M6.6 | synthesize 中途失败 | 不落半成品 investigation | 父 brief 不受影响 |
| M6.7 | 正常落库 | parent 指向同 board brief 且 question_key 为规范化问题 hash | tool_calls/method_refs/input_snapshot 齐备 |
| M6.8 | parent 是别版块或 legacy | 拒绝 | DB/service 双层校验 |
| M6.9 | 一 brief 两问题 | 两 child rows | 互不覆盖 |

## M7 方法卡选择与隔离

| # | 条件 | 期望 | 判据 |
|---|---|---|---|
| M7.1 | 空方法库 | brief/investigation 正常 | method_refs=[] |
| M7.2 | enabled 卡均 avoid | 选择 0 张 | 不强选最接近项 |
| M7.3 | 3 张适配 | 最多 2 张 | 记录理由 |
| M7.4 | 两张全文超预算 | 整卡舍弃低相关项 | 不截断卡片内部结构 |
| M7.5 | 生成 brief | 0 方法正文注入 | 即使库中有 enabled 卡 |
| M7.6 | 调查选中历史对照但无历史材料 | 标记方法不适用/gap | 不编造案例 |
| M7.7 | 方法含固定金句/模仿人格要求 | 该修辞不进入 prompt | 方法内容校验/清洗可追溯 |
| M7.8 | 旧 reference role 迁移 | 原文 hash 不变、disabled | 旧表仍可回滚读取 |
| M7.9 | 方法选择顺序 | 仅按问题+父简报选卡，再生成假设 | 不读取未生成的候选假设 |
| M7.10 | 已引用方法被删除 | 软删除且历史可回放 | method_refs 有 title/content_hash，input_snapshot 有注入正文 |

## M8 数据迁移与 kind 隔离

| # | 条件 | 期望 |
|---|---|---|
| M8.1 | 旧 analysis_scope=topic | result_kind=topic_analysis |
| M8.2 | 旧 analysis_scope=board 且 thesis/argument 形状 | result_kind=legacy_board_analysis |
| M8.3 | 新 brief | scope=board + kind=board_brief + parent null |
| M8.4 | 新 investigation | scope=board + kind=board_investigation + parent/question_key non-null |
| M8.5 | GET kind=board_brief | 不混入 investigation/legacy/topic |
| M8.6 | 简报 review | 只找上一 brief |
| M8.7 | 两个不同 question | 不互相 review |
| M8.8 | 同 parent+question_key 重跑 | generated/custom 同一规范化文本可比较 hypothesis assessment 变化 |

## M9 API 与前端

| # | 条件 | 期望 |
|---|---|---|
| M9.1 | POST 现有 analysis trigger | 立即返回唯一 job_id + kind=board_brief，不自动调查 |
| M9.2 | POST investigation generated question | 校验 brief/question id，返回 kind=board_investigation job |
| M9.3 | POST investigation custom question | 空白拒绝，合法文本保存并生成 question_key |
| M9.4 | 同 board 已有任一任务运行 | 409 携带当前 job_id/job_kind，不启动第二任务 |
| M9.5 | 客户端断连/重进 | 后台继续，按 job_id 或 board 当前任务恢复轮询 |
| M9.6 | investigation 完成 | UI 刷新 child result，不误报/选中新 brief |
| M9.7 | 简报无 research_questions | UI 正常，无“深入调查”空壳按钮 |
| M9.8 | 点击候选问题 | payload 含 briefing_result_id+question_id |
| M9.9 | 自填问题 | payload 含 briefing_result_id+question |
| M9.10 | 调查报告 | 首屏 conclusion；hypotheses/support/counter/gaps 折叠 |
| M9.11 | legacy result | 旧组件渲染+“旧版分析”标记 |
| M9.12 | lane 下钻 | prefill 为观察/问题，可编辑 |
| M9.13 | loading/error | job_kind 分派正确，brief 与 investigation 视觉状态隔离 |

## M10 退场链与可观测性

| # | 条件 | 期望 |
|---|---|---|
| M10.1 | 新版默认 trigger | `board_interpret`/旧 board analyze 调用次数=0 |
| M10.2 | 简报 session | 只含 board_brief（及必要 review）Operation |
| M10.3 | 调查 session | hypothesize + tool_use + synthesize，可按 session 回放 |
| M10.4 | 方法选择 | input_snapshot 可查候选、选中、舍弃原因 |
| M10.5 | 旧报告 QA | 继续可用，不触发旧报告改写 |

## 测试落点

| 模块 | 计划测试文件 | 层 |
|---|---|---|
| M1-M3 | `service/situation_cards_test.go`, `freshness_gate_test.go`, `board_brief_test.go` | 单测/DB集成 |
| M4-M6 | `service/board_investigation_test.go`, `board_investigation_research_test.go`, `enrich_board_test.go` | 单测/集成 |
| M7 | `service/analysis_methods_test.go`, `handler/analysis_method_handler_test.go`, database migration test | 单测/DB集成 |
| M8 | repository + migration + review tests | DB集成 |
| M9 | handler tests + Vue component tests | API/unit |
| M10 | orchestrator prompt snapshots + ai log integration | 单测/集成 |
