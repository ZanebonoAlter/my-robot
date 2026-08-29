
## 内部看美国方法论画像 v2（7期全文验证完成）

7 期全文转写（yt-dlp -f 30280 音频 + faster-whisper medium GPU，脚本 yt-dlp/asr/batch_transcribe.py 支持断点续跑，转写稿在 yt-dlp/transcripts/*.txt，ASR 系繁体+错字）经 3 路子代理精读交叉验证，产出两份文档更新：(1) 内部看美国-分析基因.md 补「六期全文验证」节——7 基因中 ①②④⑦ 全稳（①深化为认知归因批判、④编号化、⑦数字 delta 化），③历史机制呈两副面孔（立自己框架=纵深背书 / 拆别人框架=证伪弹药），⑤边界限定不对称（只护选定轴），⑥范式转折分化（产业类弱化/辩论类变形为转折审计）；(2) inside-america-methodology-profile.md 升 v2——新增辩论流水线（钢人先行→概念考古→举证责任门→自然实验）、思想背叛叙事、机制保质期、比喻对决、物理隐喻系统、双向排异、金句工程、情绪阀门等模式，并给出跨议题稳定/变形清单。该文档即 tasks.md 2.3「手工录入首份参考角色」的内容源，v2 可直接录入（约 3.1KB，远低于注入上限 4k 字符）。

**引用**：docs/research/board-analysis-reference-role/inside-america-methodology-profile.md、docs/research/yt-dlp/transcripts/、docs/research/内部看美国-分析基因.md

<!-- pinned 2026-08-25T16:36:02Z -->

## 新鲜度门3天阈值+证据多样性纪律落档

用户新增两项需求已落 change（2026-08-26 讨论定案）：(1) D9 素材新鲜度门——EnrichBoard/EnrichTopic 装配前对本板块活跃泳道 week/month/year 三档 lifeline 查 as_of_date 滞后 >3 天者同步补齐（复用循环 A 汇总函数，从 job 抽出为服务方法）；关键坑：month 档月中滞后是常态，补齐必须把 as_of_date 推进到「本次增量处理位置」而非周期边界，否则每次分析重复补齐死循环；all 档不查；补齐串行限流、失败降级不阻塞+结构化日志；落 tasks 3.6 + spec delta 3 场景（滞后补齐/幂等/失败降级）。(2) D10 证据多样性纪律——analyze prompt 约束证据链覆盖≥3 类（数据序列/报告文献/历史对照/新闻网页）+ 一手源检索引导，source_type 枚举不动；落 tasks 3.7 + spec delta 2 场景。后置讨论：专业数据源结构化集成（API 序列/PDF 文献库）另开 change，本期不做。

<!-- pinned 2026-08-26T02:37:04Z -->

## 版块分析刻板文风的 prompt 与调用链根因

真实链路是：态势卡 → boardInterpret 先生成并选定命题 → boardResearchDirections 围绕已选命题生成“命题机制/钩子核实/历史对照”方向 → runBoardAgentLoop 做支持性检索 → boardAnalyze 以“命题已定”写论文式长文。boardInterpretPrompt 明文要求 thesis 必须是「X 不是 A，而是 B」；启用的《内部看美国·方法论画像》又把「不是X而是Y」、概念重命名、钩子升维、底层规律、块尾金句列为稳定硬核，并完整注入 interpret 与 final analyze。最终 prompt 强制 3-5 层递进机制、system_reframe、历史类比、depth；UI 又连续渲染 argument.layers 与内容高度重叠的 depth.mechanism_layers。当前没有“普通解释已足够/无结构性反转”的非 sparse 出口，也没有反证检索、竞争假设或研究后改写/推翻 thesis 的阶段，因此形成先立戏剧性命题、再搜证据支撑的确认偏误闭环。生产库 7 份 board 报告合计出现「不是」55 次、「而是」73 次，平均句长约 67 个中文字符，59 句超过 80 字；说明用户感受是系统性产物，不是单次模型漂移。建议后续设计讨论区分“方法卡”与“作者文风”，改为证据先行、多假设（含朴素/零假设）、反证检索、研究后可撤回 thesis，并让历史类比/system_reframe/深层机制按素材适配而非一律强制。

**引用**：backend-go/internal/dataenrichment/service/board_interpret.go:boardInterpretPrompt、backend-go/internal/dataenrichment/service/enrich_board.go:EnrichBoard、backend-go/internal/dataenrichment/service/enrich_board.go:boardResearchDirections、backend-go/internal/dataenrichment/service/board_analysis.go:boardAnalyzePrompt、backend-go/internal/dataenrichment/service/board_analysis.go:boardAgentLoopSystemPrompt、backend-go/internal/platform/database/seeddata/inside-america-methodology-profile.md、front/app/features/tags/components/BoardAnalysisReport.vue:BoardAnalysisReport

<!-- pinned 2026-08-28T15:12:43Z -->

## 重新定位：语义版块不是天然因果系统

重新对照 flow 后发现更上游的产品建模问题：SemanticBoard 的原始职责是把散装 event 标签组织成持久概念分区，section 还能同时挂 1-3 个版块；同板块泳道仅表示语义相关，不保证共同驱动、因果关系或能被一个命题串联。当前 board-level 分析却预设“这个板块作为一个系统怎么了”，强制把最多 12 条泳道织成单一传导链，这是把分类容器误当因果系统。更合理的产品分层是：①默认版块简报/态势扫描：事实变化、重要泳道、关系类型（共同驱动/因果/分化/仅相关/暂无关系）、未知项；②只对选中的可研究问题做深度调查：多假设含零假设，支持+反证检索，研究后允许撤回/拆分命题；③单泳道聚焦下钻。深度应由证据质量、替代解释和边界处理体现，而不是固定层数、宏大系统重定位或历史类比。参考角色应降级为按问题选择的方法卡，不能参与事实抽取，也不应把作者修辞全局注入。

**引用**：docs/reference/flow/semantic-board.md、docs/reference/flow/data-enrichment.md、backend-go/internal/dataenrichment/service/enrich_board.go:EnrichBoard、backend-go/internal/dataenrichment/service/board_interpret.go:boardInterpretPrompt、backend-go/internal/dataenrichment/service/board_analysis.go:boardAnalyzePrompt

<!-- pinned 2026-08-28T15:17:45Z -->

## 重设计必须兼容现有补全门与异步 runner

交叉审查确认两个既有集成点不能按旧 board-level 设计回退：① `fix-board-analysis-material` 已把 freshness gate 改成 month/year 有数据周期补全（缺行含首份先建、UpdatedAt>72h 重算、week 退出检查、单次 40 调用上限、失败降级），态势卡事实源为 week→month→实质 section 指纹→description→none；新版简报/调查必须直接复用。② 当前分析 trigger 已经由 analysisRunner 异步执行，但 key 仅 `(scope,targetID)`、状态无 job id/kind。拆成 brief/investigation 后需扩展唯一 job_id/job_kind；同一 board 两类任务仍串行，409 返回当前任务身份，前端按 job_id 轮询，避免调查完成被误当成新简报。另以规范化问题文本 hash 的 question_key 识别同父简报同题重跑；方法卡必须先按问题+父简报选择，再参与假设生成，避免选择循环。

**引用**：backend-go/internal/dataenrichment/service/freshness_gate.go:ensureLaneFreshness、backend-go/internal/dataenrichment/handler/analysis_runner.go:analysisRunner、backend-go/internal/dataenrichment/handler/board_enrichment_handler.go:triggerBoardEnrichment、front/app/features/tags/composables/useBoardEnrichment.ts:startBoardPoll、openspec/changes/fix-board-analysis-material/specs/data-enrichment/spec.md、openspec/changes/fix-board-analysis-material/specs/board-level-analysis/spec.md

<!-- pinned 2026-08-28T15:59:58Z -->
