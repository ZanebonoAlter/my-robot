import { apiClient } from "./client";
import type { ApiResponse } from "~/types";

/**
 * 数据增强（data-enrichment）API client。
 *
 * 三表认知闭环 + 板块数据源绑定 + 个股辩论（FinGenius），对应后端已实现的 16 条 REST endpoint。
 * 数据契约设计为侦探墙（TopicDetectiveWall）后续重构可直接复用：
 *  - ContextRow / ResultSummaryRow / ReviewRow / DataSourceRow 顶层行类型稳定；
 *  - sectors / tool_calls / input_snapshot 这类 LLM 产物保留宽口子（unknown / 联合），
 *    侦探墙渲染时再按需 narrow。
 *
 * 路由分两个维度：
 *  - Topic 维度（表 1/2/3）：/persistent-topics/:topicId/enrichment/...
 *  - Board 维度（数据源绑定）：/semantic-boards/:id/data-sources
 *
 * 注意：后端 getResult / applyReview / updateReviewDeviation 虽然实现里只用 :id，
 * 但路由前缀含 :topicId，URL 仍须带上（后端解析忽略），所以函数签名是 (topicId, id)。
 */

// ── Shared scalars ──────────────────────────────────────────────────────────

export type ContextGranularity = "week" | "month" | "year" | "all";

export type EnrichmentSource = "manual" | "llm_assisted" | string;

// ── Table 1: topic_lifeline_context ─────────────────────────────────────────

/** 单条分层新闻汇总上下文（period-archival：granularity+period 唯一）。 */
export interface ContextRow {
	id: number;
	persistent_topic_id?: number;
	granularity: ContextGranularity;
	/** 周期键：week=2026-W27 / month=2026-06 / year=2026 / all=all。 */
	period: string;
	content: string;
	/** 汇总截止日（时效判断 + 检查自愈依据）。 */
	as_of_date: string;
	source: EnrichmentSource;
	created_at?: string;
	updated_at?: string;
}

// ── Table 2: topic_enrichment_result ────────────────────────────────────────

/** 方向（涨/跌/横盘）。debate.verdict / ResultSector.direction 复用。 */
export type Direction = "up" | "down" | "flat";

/** 板块下代表标的（design §3.4 symbols）。 */
export interface ResultSectorSymbol {
	code: string;
	name?: string;
	/** etf=ETF基金，leader_stock=龙头股。 */
	kind?: "etf" | "leader_stock" | string;
	[key: string]: unknown;
}

/** 板块证据条目（design §3.4 evidence）。 */
export interface ResultSectorEvidence {
	context_id?: number | string;
	period?: string;
	text?: string;
	[key: string]: unknown;
}

/**
 * 分析员结论中的一个产业切片（design §3.3/§3.4 新 schema）。
 *
 * 板块名以后端 handler.extractSymbolsFromSectors 读取的字段为准（sector），
 * 同时兼容 design schema 的 name；渲染时取 sector || name。
 * LLM 产物，字段保留宽口子。
 */
export interface ResultSector {
	/** 后端辩论提取读此字段作为 sector 名（权威）。 */
	sector?: string;
	/** design schema 的板块名字段（兼容回退）。 */
	name?: string;
	direction?: Direction | string;
	confidence?: number;
	horizon?: "short" | "mid" | "long" | string;
	reasoning?: string;
	evidence?: ResultSectorEvidence[];
	trigger_up?: string;
	trigger_down?: string;
	symbols?: ResultSectorSymbol[];
	[key: string]: unknown;
}

// ── （causal-analysis-agent 阶段2/3b：result.sectors 已迁移为 AnalyzeOutput
//     {form,lens,analysis} 按形态多态；review.verdict 迁移为 ReviewVerdict。
//     旧的 EvolutionAnalysis / EvolutionPosition / EvolutionSignal /
//     EvolutionEvidence / FinancialView / PositionChange 类型已随消费端
//     EvolutionReport → CausalAnalysisReport 重构一并移除。ResultSector 仍保留
//     给 DebateSection 的金融辩论板块切片用。）───────────────────────────────

/** 表2 列表项（slim 版，后端 listResults 返回）。 */
export interface ResultSummaryRow {
	id: number;
	evolution_assessment: string;
	/** 字段名不变；内容为 AnalyzeOutput（按 form 多态）。 */
	sectors: AnalyzeOutput | null;
	tool_calls_count: number;
	session_id: string;
	created_at: string;
}

/** 表2 详情（后端 getResult 返回，含 tool_calls / input_snapshot / causal_chain）。 */
export interface ResultDetailRow {
	id: number;
	persistent_topic_id?: number;
	evolution_assessment: string;
	/** 字段名不变；内容为 AnalyzeOutput（按 form 多态）。 */
	sectors: AnalyzeOutput | null;
	causal_chain: string | null;
	/** 工具调用记录（名/参数/返回摘要/耗时），LLM 产物，结构未冻结。 */
	tool_calls: unknown;
	/** 编排元数据（读了哪些 context 层 + as_of + section 范围 + 引用 review id）。 */
	input_snapshot: unknown;
	session_id: string;
	created_at: string;
}

/** POST .../results/trigger 的响应。 */
export interface TriggerEnrichmentResponse {
	result: {
		id: number;
		evolution_assessment: string;
		/** 字段名不变；内容为 AnalyzeOutput（按 form 多态）。 */
		sectors: AnalyzeOutput | null;
		causal_chain: string | null;
		tool_calls_count: number;
		session_id: string;
		created_at: string;
	};
	review_generated: boolean;
}

// ── Table 3: topic_enrichment_review ────────────────────────────────────────

/** review verdict 兑现度结算的单条（jsonb 数组元素，review judge 输出）。 */
export type VerdictMark = "hit" | "part" | "miss";

export interface ReviewVerdictItem {
	sector?: string;
	predicted_dir?: Direction | string;
	actual?: Direction | string;
	mark?: VerdictMark | string;
	[key: string]: unknown;
}

/** 两次 result 之间的认知增量（反思）。 */
export interface ReviewRow {
	id: number;
	persistent_topic_id?: number;
	prev_result_id: number | null;
	curr_result_id: number;
	/** 反思结算（causal-analysis-agent：should_review + new_findings + 确定性漂移）。 */
	verdict?: ReviewVerdict | null;
	deviation_summary: string;
	affected_context: ContextGranularity | null;
	confidence: number | null;
	applied: boolean;
	source: EnrichmentSource;
	created_at: string;
	updated_at?: string;
}

// ── Board data sources ──────────────────────────────────────────────────────

/** 板块与数据源的绑定行。 */
export interface DataSourceRow {
	id: number;
	semantic_board_id: number;
	source_type: string;
	/** 板块级参数，schema 由 source_type 决定。 */
	config: Record<string, unknown>;
	enabled: boolean;
	created_at?: string;
	updated_at?: string;
}

// ── Stock debate (FinGenius) ────────────────────────────────────────────────

/** 个股多角色辩论结果（design §4.2b，append-only by result+sector+code）。 */
export interface StockDebateResult {
	id: number;
	topic_enrichment_result_id: number;
	persistent_topic_id?: number;
	sector: string;
	code: string;
	name?: string;
	/** 多角色辩论最终结论方向。 */
	verdict?: Direction | string;
	/** 共识强度（如 majority/unanimous/split）。 */
	consensus?: string;
	/** 参与方（jsonb，结构未冻结）。 */
	agents?: unknown;
	/** 投票（jsonb，结构未冻结）。 */
	votes?: unknown;
	/** FinGenius 原始研究报告（jsonb）。 */
	fingenius_research?: unknown;
	/** FinGenius 原始辩论记录（jsonb）。 */
	fingenius_battle?: unknown;
	fingenius_task_id?: string;
	/** done=已提炼，failed=提炼失败（降级展示原文），running=进行中。 */
	distill_status?: "done" | "failed" | "running" | string;
	html_content?: string;
	created_at?: string;
	[key: string]: unknown;
}

// ── Causal analysis schema（causal-analysis-agent 阶段2 新 result.sectors）──
// 后端 result.sectors 形状为 {form,lens,analysis}（按 form 多态）。本节是该形状的
// TS 镜像；3b 已把 ResultDetailRow/ResultSummaryRow/TriggerEnrichmentResponse.sectors
// 与 ReviewRow.verdict 迁移到 AnalyzeOutput / ReviewVerdict，旧 Evolution* 类型已移除。

/** 因果叙事形态：event_chain=事件链 / theme_vein=主题脉络 / single_point=单点深入 / structural=结构演化 / sparse=稀疏。 */
export type AnalyzeForm =
	| "event_chain"
	| "theme_vein"
	| "single_point"
	| "structural"
	| "sparse";

/** 确定性档位：high=高 / medium=中 / low=低 / question=存疑。 */
export type AnalyzeCert = "high" | "medium" | "low" | "question";

/**
 * 引用来源：news=订阅源新闻报道 / tool=agent 工具查证 / web=web_search 网页 / page=fetch_page 正文。
 * （web/page 为深度层 evidence_chain 的可核查外部证据；既有事实层仍只用 news/tool。）
 * 加 Analyze 前缀避免与 Vue 的 Ref<T>（本文件大量使用）混淆。
 */
export interface AnalyzeRef {
	source_type: "news" | "tool" | "web" | "page";
	ref: string;
	quote?: string;
	[key: string]: unknown;
}

/** 洞察条目（确定性 + 标题 + 逻辑链 + 证据，可选 web 核验引用）。 */
export interface AnalyzeInsight {
	cert: AnalyzeCert | string;
	title: string;
	logic: string;
	evidence: AnalyzeRef[];
	web_verified?: AnalyzeRef[];
	[key: string]: unknown;
}

// ── 深度层 depth（data-enrichment-structural-depth：非 sparse 形态强制产出）──
/** 多层机制拆解的单层（子机制名 + 深层逻辑 + 依据）。 */
export interface MechanismLayer {
	layer: string;
	deep_logic: string;
	basis: string;
	[key: string]: unknown;
}

/** 历史类比（案例 + 机制类比 + 何处不同）。 */
export interface HistoricalAnalogy {
	case: string;
	mechanism: string;
	diff: string;
	[key: string]: unknown;
}

/** 范式转折判断（无迹象则 depth.regime_shift 为 null）。 */
export interface RegimeShift {
	judgment: string;
	evidence: string;
	[key: string]: unknown;
}

/** 可核查证据链条目：news=分层新闻 / web=web_search 网页 / page=fetch_page 正文。web/page 带 url+quote+institution+date。 */
export interface EvidenceChainItem {
	source_type: "news" | "web" | "page" | string;
	ref?: string;
	url?: string;
	/** 原文摘录（非 AI 转述）。 */
	quote?: string;
	institution?: string;
	date?: string;
	[key: string]: unknown;
}

/**
 * 分析深度层（"内部看美国"分析基因映射）。
 * 非 sparse 形态由后端强制产出；前端按可选消费——旧结果无 depth 则降级不渲染、不报错。
 */
export interface AnalyzeDepth {
	/** 系统重定位：一句话把话题放进哪个大系统讲。 */
	system_reframe: string;
	mechanism_layers: MechanismLayer[];
	historical_analogy: HistoricalAnalogy[];
	regime_shift?: RegimeShift | null;
	/** 反过度解读边界：明确"目前还不能下结论"的范围（后端校验非空）。 */
	boundary: string;
	evidence_chain: EvidenceChainItem[];
	[key: string]: unknown;
}

// ── analysis body（按 form 多态：每个 variant 形状不同）────────────────────
/** event_chain 的 analysis 体：事实层 + 时间线 + 洞察层（+ 可选深度层）。 */
export interface AnalysisEventChain {
	fact_layer: Array<{
		claim: string;
		evidence: AnalyzeRef[];
		verified: boolean;
	}>;
	timeline: Array<{
		date: string;
		event: string;
		ref?: AnalyzeRef;
	}>;
	insight_layer: AnalyzeInsight[];
	/** 深度层（可选：旧结果无此字段，前端降级不渲染）。 */
	depth?: AnalyzeDepth;
}

/** theme_vein 的 analysis 体：脉络切片 + 跨脉络洞察（+ 可选深度层）。 */
export interface AnalysisThemeVein {
	veins: Array<{
		name: string;
		desc: string;
		evidence: AnalyzeRef[];
	}>;
	cross_insight: AnalyzeInsight[];
	/** 深度层（可选：旧结果无此字段，前端降级不渲染）。 */
	depth?: AnalyzeDepth;
}

/** single_point 的 analysis 体：单点深入（影响 + 证据，+ 可选深度层）。 */
export interface AnalysisSinglePoint {
	impact: {
		implication: string;
		ripple: string;
		benchmark: string;
	};
	evidence: AnalyzeRef[];
	/** 深度层（可选：旧结果无此字段，前端降级不渲染）。 */
	depth?: AnalyzeDepth;
}

/** structural 的 analysis 体：结构演化叙述 + 关键阶段（+ 可选深度层）。 */
export interface AnalysisStructural {
	evolution_narrative: string;
	phases: Array<{
		period: string;
		event: string;
		ref?: AnalyzeRef;
	}>;
	/** 深度层（可选：旧结果无此字段，前端降级不渲染）。 */
	depth?: AnalyzeDepth;
}

/** sparse 的 analysis 体：稀疏（提示 + 摘要）。 */
export interface AnalysisSparse {
	notice: string;
	summary: string;
}

/**
 * analysis 内容体联合。各 variant 无独立判别字段，窄化须依赖父 AnalyzeOutput.form
 * （switch output.form 后即可安全访问对应 analysis 专属结构）。
 */
export type AnalysisBody =
	| AnalysisEventChain
	| AnalysisThemeVein
	| AnalysisSinglePoint
	| AnalysisStructural
	| AnalysisSparse;

/**
 * result.sectors 新形状 {form,lens,analysis}（判别联合）。
 * form 为判别字段：switch (out.form) 后 TS 自动窄化到对应 variant，
 * 进而安全读取 out.analysis 的专属结构。lens 为分析视角标签（LLM 产物）。
 */
export interface AnalyzeEventChainOutput {
	form: "event_chain";
	lens: string;
	analysis: AnalysisEventChain;
}

export interface AnalyzeThemeVeinOutput {
	form: "theme_vein";
	lens: string;
	analysis: AnalysisThemeVein;
}

export interface AnalyzeSinglePointOutput {
	form: "single_point";
	lens: string;
	analysis: AnalysisSinglePoint;
}

export interface AnalyzeStructuralOutput {
	form: "structural";
	lens: string;
	analysis: AnalysisStructural;
}

export interface AnalyzeSparseOutput {
	form: "sparse";
	lens: string;
	analysis: AnalysisSparse;
}

export type AnalyzeOutput =
	| AnalyzeEventChainOutput
	| AnalyzeThemeVeinOutput
	| AnalyzeSinglePointOutput
	| AnalyzeStructuralOutput
	| AnalyzeSparseOutput;

/**
 * review.verdict 新形状（causal-analysis-agent 阶段2）。
 * 反思结算：是否需要重审 + 原因 + 新发现/被推翻的洞察 + 确定性漂移。
 * ReviewRow.verdict 已在 3b 迁移为本类型。
 */
export interface ReviewVerdict {
	should_review: boolean;
	reason?: string;
	new_findings?: string[];
	overturned?: string[];
	confidence_shift?: Array<{
		insight: string;
		from: AnalyzeCert | string;
		to: AnalyzeCert | string;
	}>;
	affected_context?: string | null;
	confidence?: number | null;
	[key: string]: unknown;
}

/**
 * 报告追问 QA 条目（append-only，归属一个不可变 result 快照；可 sediment 沉淀）。
 * 对齐后端 TopicEnrichmentQA（table topic_enrichment_qa）。
 */
export interface TopicEnrichmentQA {
	id: number;
	topic_enrichment_result_id: number;
	question: string;
	answer: string;
	/** 工具调用记录（jsonb，结构未冻结）。 */
	tool_calls: unknown;
	source: EnrichmentSource;
	sedimented: boolean;
	created_at: string;
}

// ── Request body types ──────────────────────────────────────────────────────

export interface CreateReviewBody {
	curr_result_id: number;
	deviation_summary: string;
	prev_result_id?: number;
}

export interface UpsertDataSourceBody {
	source_type: string;
	config?: Record<string, unknown>;
	enabled?: boolean;
}

/** POST .../results/:id/qa 的响应（对齐后端 service.QAAnswer）。 */
export interface AskQAResponse {
	answer: string;
	/** 工具调用记录（结构未冻结，LLM 产物）。 */
	tool_calls: unknown;
	/** 双类引用：news=报告上下文 / tool=工具查证结果。 */
	refs: AnalyzeRef[];
}

// ── API factory ─────────────────────────────────────────────────────────────

export function useBoardEnrichmentApi() {
	// ── Table 1: contexts (period-archival: keyed by granularity+period) ────
	/** 拉所有 contexts；传 granularity 则按粒度过滤（用于周期筛选器翻历史）。 */
	async function listContexts(
		topicId: number,
		granularity?: ContextGranularity,
	): Promise<ApiResponse<ContextRow[]>> {
		const qs = granularity ? `?granularity=${granularity}` : "";
		return apiClient.get(
			`/persistent-topics/${topicId}/enrichment/contexts${qs}`,
		);
	}

	/** 取具体周期：GET /contexts/:granularity/:period（period 如 2026-W27）。 */
	async function getContext(
		topicId: number,
		granularity: ContextGranularity,
		period: string,
	): Promise<ApiResponse<ContextRow>> {
		return apiClient.get(
			`/persistent-topics/${topicId}/enrichment/contexts/${granularity}/${period}`,
		);
	}

	/** 编辑具体周期：PUT /contexts/:granularity/:period。 */
	async function updateContext(
		topicId: number,
		granularity: ContextGranularity,
		period: string,
		body: { content: string },
	): Promise<ApiResponse<ContextRow>> {
		return apiClient.put(
			`/persistent-topics/${topicId}/enrichment/contexts/${granularity}/${period}`,
			body,
		);
	}

	/** 重生成：POST /contexts/:granularity/regenerate?period=…（period 省略=当前周期）。 */
	async function regenerateContext(
		topicId: number,
		granularity: ContextGranularity,
		period?: string,
	): Promise<ApiResponse<ContextRow>> {
		const qs = period ? `?period=${period}` : "";
		return apiClient.post(
			`/persistent-topics/${topicId}/enrichment/contexts/${granularity}/regenerate${qs}`,
		);
	}

	// ── Table 2: results ────────────────────────────────────────────────────
	async function listResults(
		topicId: number,
	): Promise<ApiResponse<ResultSummaryRow[]>> {
		return apiClient.get(`/persistent-topics/${topicId}/enrichment/results`);
	}

	async function getResult(
		topicId: number,
		id: number,
	): Promise<ApiResponse<ResultDetailRow>> {
		return apiClient.get(
			`/persistent-topics/${topicId}/enrichment/results/${id}`,
		);
	}

	async function triggerEnrichment(
		topicId: number,
	): Promise<ApiResponse<TriggerEnrichmentResponse>> {
		return apiClient.post(
			`/persistent-topics/${topicId}/enrichment/results/trigger`,
		);
	}

	// ── Table 3: reviews ────────────────────────────────────────────────────
	async function listReviews(
		topicId: number,
	): Promise<ApiResponse<ReviewRow[]>> {
		return apiClient.get(`/persistent-topics/${topicId}/enrichment/reviews`);
	}

	async function createReview(
		topicId: number,
		body: CreateReviewBody,
	): Promise<ApiResponse<ReviewRow>> {
		return apiClient.post(
			`/persistent-topics/${topicId}/enrichment/reviews`,
			body,
		);
	}

	async function updateReviewDeviation(
		topicId: number,
		id: number,
		body: { deviation_summary: string },
	): Promise<ApiResponse<ReviewRow>> {
		return apiClient.put(
			`/persistent-topics/${topicId}/enrichment/reviews/${id}`,
			body,
		);
	}

	async function applyReview(
		topicId: number,
		id: number,
	): Promise<ApiResponse<ReviewRow>> {
		return apiClient.post(
			`/persistent-topics/${topicId}/enrichment/reviews/${id}/apply`,
		);
	}

	// ── Stock debate (FinGenius) ────────────────────────────────────────────
	// 触发：不传 body，后端默认从 result.sectors 提取 symbols 自动跑。
	async function triggerDebate(
		topicId: number,
		resultId: number,
	): Promise<ApiResponse<{ debates: StockDebateResult[] }>> {
		return apiClient.post(
			`/persistent-topics/${topicId}/enrichment/results/${resultId}/debates`,
		);
	}

	async function listDebates(
		topicId: number,
		resultId: number,
	): Promise<ApiResponse<StockDebateResult[]>> {
		return apiClient.get(
			`/persistent-topics/${topicId}/enrichment/results/${resultId}/debates`,
		);
	}

	// ── Board data sources ──────────────────────────────────────────────────
	async function listDataSources(
		boardId: number,
	): Promise<ApiResponse<DataSourceRow[]>> {
		return apiClient.get(`/semantic-boards/${boardId}/data-sources`);
	}

	async function upsertDataSource(
		boardId: number,
		body: UpsertDataSourceBody,
	): Promise<ApiResponse<DataSourceRow>> {
		return apiClient.put(`/semantic-boards/${boardId}/data-sources`, body);
	}

	async function deleteDataSource(
		boardId: number,
		sourceType: string,
	): Promise<ApiResponse<{ deleted: boolean }>> {
		return apiClient.delete(
			`/semantic-boards/${boardId}/data-sources/${sourceType}`,
		);
	}

	// ── QA (causal-analysis-agent 阶段3：报告追问 + 沉淀) ─────────────────────
	/** 问一轮：POST .../results/:id/qa body {question} → {answer,tool_calls,refs}。 */
	async function askQA(
		topicId: number,
		resultId: number,
		question: string,
	): Promise<ApiResponse<AskQAResponse>> {
		return apiClient.post(
			`/persistent-topics/${topicId}/enrichment/results/${resultId}/qa`,
			{ question },
		);
	}

	/** 多轮历史：GET .../results/:id/qa → TopicEnrichmentQA[]（oldest first）。 */
	async function listQA(
		topicId: number,
		resultId: number,
	): Promise<ApiResponse<TopicEnrichmentQA[]>> {
		return apiClient.get(
			`/persistent-topics/${topicId}/enrichment/results/${resultId}/qa`,
		);
	}

	/** 沉淀一轮：POST .../qa/:id/sediment → 更新后的 TopicEnrichmentQA（sedimented=true）。 */
	async function sedimentQA(
		topicId: number,
		qaId: number,
	): Promise<ApiResponse<TopicEnrichmentQA>> {
		return apiClient.post(
			`/persistent-topics/${topicId}/enrichment/qa/${qaId}/sediment`,
		);
	}

	return {
		listContexts,
		getContext,
		updateContext,
		regenerateContext,
		listResults,
		getResult,
		triggerEnrichment,
		listReviews,
		createReview,
		updateReviewDeviation,
		applyReview,
		triggerDebate,
		listDebates,
		listDataSources,
		upsertDataSource,
		deleteDataSource,
		// qa (causal-analysis-agent 阶段3)
		askQA,
		listQA,
		sedimentQA,
	};
}
