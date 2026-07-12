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

/** 方向（涨/跌/横盘）。debate.verdict / ResultSector.direction / financial_view 方向复用（演进主线不再用）。 */
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

// ── Evolution analysis（主线重定位：金融涨跌 → 演进定位）──────────────────────
// 后端"分析员"与"review judge"输出已从 sectors 涨跌改为演进定位复合对象。
// 字段名 `sectors` 在表2 JSONB 里不变，但内容形状变了：旧 ResultSector[] → 新
// EvolutionAnalysis（复合对象）。ResultSector 类型保留给 financial_view.sectors 用。

/** 演进定位四档：reinforcing=强化 / turning=转折 / expanding=扩散 / fading=衰减。 */
export type EvolutionPosition =
	| "reinforcing"
	| "turning"
	| "expanding"
	| "fading";

/** 跨泳道信号：lane=持久话题泳道名，signal=该泳道发生的变化，mechanism=传导机制。 */
export interface EvolutionSignal {
	lane: string;
	signal: string;
	mechanism: string;
	[key: string]: unknown;
}

/** 演进分析证据条目（双类引用）：source_type 区分新闻(news)/工具(tool)。 */
export interface EvolutionEvidence {
	context_id?: string | number;
	period?: "week" | "month" | "year" | "all" | string;
	quote?: string;
	/** ★ news=来自订阅源新闻报道 / tool=agent 自主调用 opencli skill 查证。 */
	source_type?: "news" | "tool";
	/** 工具引用的 skill 名（web-search / market-quote / browser…），仅 source_type=tool。 */
	tool_ref?: string;
	[key: string]: unknown;
}

/** 金融视角下的板块方向（可选，仅金融话题；非主线，低调展示）。 */
export interface FinancialView {
	sectors: Array<{
		sector: string;
		direction: "up" | "down" | "flat" | "unknown" | string;
		supporting_data?: string;
		[key: string]: unknown;
	}>;
	[key: string]: unknown;
}

/** 表2 result.sectors 的演进定位复合对象（新 schema）。 */
export interface EvolutionAnalysis {
	position: EvolutionPosition | string;
	signals: EvolutionSignal[];
	evidence: EvolutionEvidence[];
	/** 可选，仅金融话题——非主线。 */
	financial_view?: FinancialView;
	[key: string]: unknown;
}

/** 两次 result 之间的定位迁移（review.verdict 新 schema）。 */
export interface PositionChange {
	from: EvolutionPosition | string;
	to: EvolutionPosition | string;
	summary: string;
	[key: string]: unknown;
}

/** 表2 列表项（slim 版，后端 listResults 返回）。 */
export interface ResultSummaryRow {
	id: number;
	evolution_assessment: string;
	/** 字段名不变；内容从 ResultSector[] 变为 EvolutionAnalysis 复合对象。 */
	sectors: EvolutionAnalysis | null;
	tool_calls_count: number;
	session_id: string;
	created_at: string;
}

/** 表2 详情（后端 getResult 返回，含 tool_calls / input_snapshot / causal_chain）。 */
export interface ResultDetailRow {
	id: number;
	persistent_topic_id?: number;
	evolution_assessment: string;
	/** 字段名不变；内容从 ResultSector[] 变为 EvolutionAnalysis 复合对象。 */
	sectors: EvolutionAnalysis | null;
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
		/** 字段名不变；内容为 EvolutionAnalysis 复合对象。 */
		sectors: EvolutionAnalysis | null;
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
	/** 定位迁移（新 schema：from→to + summary）。旧 verdict 数组结算已废弃。 */
	verdict?: PositionChange | null;
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
	};
}
