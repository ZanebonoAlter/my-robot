import { ref, computed, onUnmounted } from "vue";
import { useNotify } from "~/composables/useNotify";
import {
	useBoardEnrichmentApi,
	type ContextRow,
	type ContextGranularity,
	type ResultSummaryRow,
	type ResultDetailRow,
	type ReviewRow,
	type DataSourceRow,
	type StockDebateResult,
	type CreateReviewBody,
	type UpsertDataSourceBody,
	type TopicEnrichmentQA,
	type AskQAResponse,
	type BoardAnalysisResultRow,
} from "~/api/boardEnrichment";
import {
	useDailyReportsApi,
	type BoardTopicListItem,
} from "~/api/dailyReports";

/**
 * 数据增强面板的 composable（仿 useBoardCRUD 模式）。
 *
 * 维度拆分：
 *  - topic 维度（表 1/2/3）：contexts / results / reviews，挂在 selectedTopicId 上；
 *  - board 维度（数据源绑定）：dataSources，挂在 boardId 上。
 *
 * 面板组件直接 `const en = useBoardEnrichment()`，board/topic 切换时显式调
 * loadTopics / selectTopic / loadDataSources（不自动 watch，保持与 useBoardCRUD 一致
 * 的显式调用风格，便于测试与可控加载）。
 */
export function useBoardEnrichment() {
	const api = useBoardEnrichmentApi();
	const reportsApi = useDailyReportsApi();
	const { success: notifySuccess, error: notifyError } = useNotify();

	// ── topic selector ──────────────────────────────────────────────────────
	const topics = ref<BoardTopicListItem[]>([]);
	const topicsLoading = ref(false);
	const selectedTopicId = ref<number | null>(null);

	// ── table 1: contexts ───────────────────────────────────────────────────
	const contexts = ref<ContextRow[]>([]);
	const contextsLoading = ref(false);
	const regenerating = ref<ContextGranularity | null>(null);

	// ── table 2: results ────────────────────────────────────────────────────
	const results = ref<ResultSummaryRow[]>([]);
	const resultsLoading = ref(false);
	const triggering = ref(false);

	// ── table 3: reviews ────────────────────────────────────────────────────
	const reviews = ref<ReviewRow[]>([]);
	const reviewsLoading = ref(false);

	// ── board data sources ──────────────────────────────────────────────────
	const dataSources = ref<DataSourceRow[]>([]);
	const dataSourcesLoading = ref(false);

	// ── stock debates (FinGenius) ───────────────────────────────────────────
	const debates = ref<StockDebateResult[]>([]);
	const debatesLoading = ref(false);
	const debateTriggering = ref(false);
	const debateError = ref<string | null>(null);
	let debatePollTimer: ReturnType<typeof setInterval> | null = null;

	// ── qa (causal-analysis-agent 阶段3：报告追问) ──────────────────────────
	const qaList = ref<TopicEnrichmentQA[]>([]);
	const qaLoading = ref(false);
	const qaError = ref<string | null>(null);
	// 最近一次 ask 的即时响应（含 refs）。持久化 QA 行无 refs 列（后端只回显不落库），
	// 故仅最新这轮能渲染双类引用；QAPanel 按 answer 文本匹配挂到对应行。
	const latestAnswer = ref<AskQAResponse | null>(null);

	// ── workbench UI state（原型认知工作台）───────────────────────────────
	// ①周期筛选器：gran=按周/月/年（原型挊了 all），periodList 由 contexts 派生
	type PickerGran = "week" | "month" | "year";
	const selectedGran = ref<PickerGran>("week");
	const selectedPeriodIdx = ref(0); // 0=最新周期
	// loopnav 当前高亮 section（IntersectionObserver 写入）
	const activeSection = ref<string>("sec1");
	// ④辩论四态依赖的最新 result 详情（sectors 原始 + 辩论归属）
	const latestResultDetail = ref<ResultDetailRow | null>(null);
	const latestResultDetailLoading = ref(false);

	// ── board-level analysis（board-level-deep-analysis D2/D8：版块档分析）─
	const boardResults = ref<BoardAnalysisResultRow[]>([]);
	const boardResultsLoading = ref(false);
	const boardAnalysisTriggering = ref(false);
	// 当前展示的版块报告 id（默认最新；点历史可切换）
	const selectedBoardResultId = ref<number | null>(null);
	const selectedBoardResult = computed<BoardAnalysisResultRow | null>(() => {
		const list = boardResults.value;
		if (!list.length) return null;
		const id = selectedBoardResultId.value;
		return list.find((r) => r.id === id) ?? list[0] ?? null;
	});

	const error = ref<string | null>(null);

	// ── topic selector actions ──────────────────────────────────────────────
	async function loadTopics(boardId: number) {
		topicsLoading.value = true;
		error.value = null;
		const res = await reportsApi.listBoardTopics(boardId);
		if (res.success && res.data) {
			topics.value = res.data.topics ?? [];
			const stillValid =
				selectedTopicId.value !== null &&
				topics.value.some((t) => t.id === selectedTopicId.value);
			if (!stillValid) {
				const firstActive = topics.value.find((t) => t.status === "active");
				selectedTopicId.value = topics.value.length
					? (firstActive?.id ?? topics.value[0]?.id ?? null)
					: null;
			}
		} else {
			topics.value = [];
			error.value = res.error || "加载话题失败";
		}
		topicsLoading.value = false;
	}

	async function selectTopic(topicId: number) {
		selectedTopicId.value = topicId;
		await loadAllTopicTables(topicId);
	}

	// ── table loaders ───────────────────────────────────────────────────────
	async function loadContexts(topicId: number) {
		contextsLoading.value = true;
		const res = await api.listContexts(topicId);
		if (res.success && res.data) {
			contexts.value = res.data;
		} else {
			contexts.value = [];
		}
		contextsLoading.value = false;
	}

	async function loadResults(topicId: number) {
		resultsLoading.value = true;
		const res = await api.listResults(topicId);
		if (res.success && res.data) {
			results.value = res.data;
		} else {
			results.value = [];
		}
		resultsLoading.value = false;
	}

	async function loadReviews(topicId: number) {
		reviewsLoading.value = true;
		const res = await api.listReviews(topicId);
		if (res.success && res.data) {
			reviews.value = res.data;
		} else {
			reviews.value = [];
		}
		reviewsLoading.value = false;
	}

	async function loadAllTopicTables(topicId: number) {
		await Promise.all([
			loadContexts(topicId),
			loadResults(topicId),
			loadReviews(topicId),
		]);
		selectedPeriodIdx.value = 0; // 切话题回到最新周期
		await loadLatestResultDetail(topicId);
		if (latestResultId.value !== null) {
			await loadDebates(latestResultId.value);
		}
	}

	// ── workbench helpers（原型认知工作台）──────────────────────────────
	/** 周期列表：当前粒度下的所有 period，按 period 降序（最新在前）。 */
	const periodList = computed(() =>
		contexts.value
			.filter((c) => c.granularity === selectedGran.value)
			.map((c) => c.period)
			.sort((a, b) => (a < b ? 1 : a > b ? -1 : 0)),
	);

	/** 当前选中的周期 context（periodList 为空时为 undefined）。 */
	const currentContext = computed<ContextRow | undefined>(() => {
		const p = periodList.value[selectedPeriodIdx.value];
		if (!p) return undefined;
		return contexts.value.find(
			(c) => c.granularity === selectedGran.value && c.period === p,
		);
	});

	function setGran(g: PickerGran) {
		if (selectedGran.value === g) return;
		selectedGran.value = g;
		selectedPeriodIdx.value = 0;
	}

	function shiftPeriod(delta: number) {
		const max = periodList.value.length - 1;
		if (max < 0) return;
		selectedPeriodIdx.value = Math.max(
			0,
			Math.min(max, selectedPeriodIdx.value + delta),
		);
	}

	/**
	 * 选中指定 period（7.3.1 手动补生成用）：设粒度 + 定位到 periodList 中的索引。
	 * regenerate 成功后 contexts 已含新行、periodList 重算，故能定位到刚生成的周期。
	 */
	function selectPeriod(granularity: PickerGran, period: string) {
		if (selectedGran.value !== granularity) selectedGran.value = granularity;
		const idx = periodList.value.indexOf(period);
		selectedPeriodIdx.value = idx >= 0 ? idx : 0;
	}

	function setActiveSection(id: string) {
		activeSection.value = id;
	}

	/** 最新 result id（results[0]），②③④ 都挂它。 */
	const latestResultId = computed<number | null>(
		() => results.value[0]?.id ?? null,
	);

	/** 拉最新 result 详情（含 sectors 原始结构）。 */
	async function loadLatestResultDetail(topicId: number) {
		const id = latestResultId.value;
		if (id === null) {
			latestResultDetail.value = null;
			return;
		}
		latestResultDetailLoading.value = true;
		const res = await api.getResult(topicId, id);
		if (res.success && res.data) latestResultDetail.value = res.data;
		else latestResultDetail.value = null;
		latestResultDetailLoading.value = false;
	}

	/** ④辩论四态：empty（无 result/无辩论）|loading（触发中或 running）|error|result。 */
	const debateStage = computed<"empty" | "loading" | "error" | "result">(() => {
		if (debateError.value) return "error";
		if (debateTriggering.value) return "loading";
		if (debates.value.some((d) => d.distill_status === "running"))
			return "loading";
		if (debates.value.length === 0) return "empty";
		return "result";
	});

	// ── table 1 actions ─────────────────────────────────────────────────────
	async function saveContext(
		topicId: number,
		granularity: ContextGranularity,
		period: string,
		content: string,
	): Promise<boolean> {
		const res = await api.updateContext(topicId, granularity, period, {
			content,
		});
		if (res.success && res.data) {
			const idx = contexts.value.findIndex(
				(c) => c.granularity === granularity && c.period === period,
			);
			if (idx >= 0) contexts.value[idx] = res.data;
			else contexts.value.push(res.data);
			notifySuccess("已保存上下文");
			return true;
		}
		notifyError(res.error || "保存失败");
		return false;
	}

	async function regenerateContext(
		topicId: number,
		granularity: ContextGranularity,
		period?: string,
	): Promise<boolean> {
		regenerating.value = granularity;
		try {
			const res = await api.regenerateContext(topicId, granularity, period);
			if (res.success && res.data) {
				const row = res.data;
				const idx = contexts.value.findIndex(
					(c) => c.granularity === granularity && c.period === row.period,
				);
				if (idx >= 0) contexts.value[idx] = row;
				else contexts.value.push(row);
				notifySuccess(
					`${granularity}${period ? " " + period : ""} 上下文已重生成`,
				);
				return true;
			}
			notifyError(res.error || "重生成失败");
			return false;
		} finally {
			regenerating.value = null;
		}
	}

	// ── table 2 actions ─────────────────────────────────────────────────────
	// 异步轮询（fix-board-analysis-material 8.x）：trigger 立即返回，分析
	// 后台跑；这里定时轮询 status，完成后拉新结果。离开页面只停轮询，
	// 后台分析不受影响（回来时 sync 恢复显示）。
	let boardPollTimer: ReturnType<typeof setInterval> | null = null;
	let topicPollTimer: ReturnType<typeof setInterval> | null = null;
	const POLL_INTERVAL_MS = 3000;

	function stopBoardPoll() {
		if (boardPollTimer) {
			clearInterval(boardPollTimer);
			boardPollTimer = null;
		}
		boardAnalysisTriggering.value = false;
	}

	function startBoardPoll(boardId: number) {
		stopBoardPoll();
		boardAnalysisTriggering.value = true;
		boardPollTimer = setInterval(() => {
			void pollBoardOnce(boardId);
		}, POLL_INTERVAL_MS);
	}

	async function pollBoardOnce(boardId: number) {
		const res = await api.getAnalysisStatus("board", boardId);
		if (!res.success || !res.data) return; // transient error — keep polling
		if (res.data.running) return;
		stopBoardPoll();
		if (res.data.error) {
			notifyError(`版块分析失败：${res.data.error}`);
			return;
		}
		notifySuccess("版块分析完成");
		await loadBoardAnalysisResults(boardId);
		if (res.data.result_id) selectedBoardResultId.value = res.data.result_id;
	}

	/** 进入面板时同步一次状态：后台在跑则恢复轮询显示。 */
	async function syncBoardAnalysisStatus(boardId: number) {
		const res = await api.getAnalysisStatus("board", boardId);
		if (res.success && res.data?.running) startBoardPoll(boardId);
	}

	function stopTopicPoll() {
		if (topicPollTimer) {
			clearInterval(topicPollTimer);
			topicPollTimer = null;
		}
		triggering.value = false;
	}

	function startTopicPoll(topicId: number) {
		stopTopicPoll();
		triggering.value = true;
		topicPollTimer = setInterval(() => {
			void pollTopicOnce(topicId);
		}, POLL_INTERVAL_MS);
	}

	async function pollTopicOnce(topicId: number) {
		const res = await api.getAnalysisStatus("topic", topicId);
		if (!res.success || !res.data) return;
		if (res.data.running) return;
		stopTopicPoll();
		if (res.data.error) {
			notifyError(`增强失败：${res.data.error}`);
			return;
		}
		notifySuccess("增强完成");
		await loadResults(topicId);
		await loadReviews(topicId);
		await loadLatestResultDetail(topicId);
	}

	async function triggerEnrichment(
		topicId: number,
		prefillLens?: string,
	): Promise<boolean> {
		triggering.value = true;
		try {
			const res = await api.triggerEnrichment(topicId, prefillLens);
			if (res.success) {
				notifySuccess("分析已在后台开始，可离开页面");
				startTopicPoll(topicId);
				return true;
			}
			// 409 = 已在跑：恢复轮询显示即可
			if (res.error?.includes("already running")) {
				startTopicPoll(topicId);
				return true;
			}
			// 400 = 板块未开启增强
			notifyError(res.error || "触发失败：需先在板块编辑开启增强开关");
			triggering.value = false;
			return false;
		} catch {
			triggering.value = false;
			return false;
		}
	}

	// ── board-level analysis actions（board-level-deep-analysis D2/D8）───
	async function loadBoardAnalysisResults(boardId: number) {
		boardResultsLoading.value = true;
		try {
			const res = await api.listBoardAnalysisResults(boardId);
			if (res.success && res.data) {
				boardResults.value = res.data;
				selectedBoardResultId.value = res.data[0]?.id ?? null;
			} else {
				boardResults.value = [];
			}
		} finally {
			boardResultsLoading.value = false;
		}
	}

	async function triggerBoardAnalysis(boardId: number): Promise<boolean> {
		boardAnalysisTriggering.value = true;
		try {
			const res = await api.triggerBoardAnalysis(boardId);
			if (res.success) {
				notifySuccess("版块分析已在后台开始，可离开页面");
				startBoardPoll(boardId);
				return true;
			}
			if (res.error?.includes("already running")) {
				startBoardPoll(boardId);
				return true;
			}
			notifyError(res.error || "版块分析触发失败");
			boardAnalysisTriggering.value = false;
			return false;
		} catch {
			boardAnalysisTriggering.value = false;
			return false;
		}
	}

	/** 选历史报告（null = 回最新）。 */
	function selectBoardResult(id: number | null) {
		selectedBoardResultId.value = id;
	}

	onUnmounted(() => {
		stopBoardPoll();
		stopTopicPoll();
	});

	// ── table 3 actions ─────────────────────────────────────────────────────
	async function saveReviewDeviation(
		topicId: number,
		reviewId: number,
		deviation: string,
	): Promise<boolean> {
		const res = await api.updateReviewDeviation(topicId, reviewId, {
			deviation_summary: deviation,
		});
		if (res.success && res.data) {
			const idx = reviews.value.findIndex((r) => r.id === reviewId);
			if (idx >= 0) reviews.value[idx] = res.data;
			notifySuccess("已保存偏差说明");
			return true;
		}
		notifyError(res.error || "保存失败");
		return false;
	}

	async function applyReview(
		topicId: number,
		reviewId: number,
	): Promise<boolean> {
		const res = await api.applyReview(topicId, reviewId);
		if (res.success && res.data) {
			const idx = reviews.value.findIndex((r) => r.id === reviewId);
			if (idx >= 0) reviews.value[idx] = res.data;
			notifySuccess("已采纳");
			return true;
		}
		notifyError(res.error || "采纳失败");
		return false;
	}

	async function createReview(
		topicId: number,
		body: CreateReviewBody,
	): Promise<boolean> {
		const res = await api.createReview(topicId, body);
		if (res.success && res.data) {
			reviews.value.unshift(res.data);
			notifySuccess("已添加批注");
			return true;
		}
		notifyError(res.error || "添加批注失败");
		return false;
	}

	// ── board data source actions ───────────────────────────────────────────
	async function loadDataSources(boardId: number) {
		dataSourcesLoading.value = true;
		const res = await api.listDataSources(boardId);
		if (res.success && res.data) {
			dataSources.value = res.data;
		} else {
			dataSources.value = [];
		}
		dataSourcesLoading.value = false;
	}

	async function saveDataSource(
		boardId: number,
		body: UpsertDataSourceBody,
	): Promise<boolean> {
		const res = await api.upsertDataSource(boardId, body);
		if (res.success && res.data) {
			await loadDataSources(boardId);
			notifySuccess("已保存数据源");
			return true;
		}
		notifyError(res.error || "保存失败");
		return false;
	}

	async function removeDataSource(
		boardId: number,
		sourceType: string,
	): Promise<boolean> {
		const res = await api.deleteDataSource(boardId, sourceType);
		if (res.success) {
			dataSources.value = dataSources.value.filter(
				(d) => d.source_type !== sourceType,
			);
			notifySuccess("已删除数据源");
			return true;
		}
		notifyError(res.error || "删除失败");
		return false;
	}

	// ── stock debate actions ────────────────────────────────────────────────
	function stopDebatePolling() {
		if (debatePollTimer) {
			clearInterval(debatePollTimer);
			debatePollTimer = null;
		}
	}

	function startDebatePolling(resultId: number) {
		stopDebatePolling();
		debatePollTimer = setInterval(async () => {
			if (selectedTopicId.value === null) {
				stopDebatePolling();
				return;
			}
			const res = await api.listDebates(selectedTopicId.value, resultId);
			if (res.success && res.data) {
				debates.value = res.data;
				const stillRunning = res.data.some(
					(d) => d.distill_status === "running",
				);
				if (!stillRunning) stopDebatePolling();
			}
		}, 5000);
	}

	async function loadDebates(resultId: number) {
		if (selectedTopicId.value === null) {
			debates.value = [];
			return;
		}
		debatesLoading.value = true;
		const res = await api.listDebates(selectedTopicId.value, resultId);
		if (res.success && res.data) {
			debates.value = res.data;
			if (res.data.some((d) => d.distill_status === "running"))
				startDebatePolling(resultId);
		} else {
			debates.value = [];
		}
		debatesLoading.value = false;
	}

	async function triggerDebate(resultId: number): Promise<boolean> {
		if (selectedTopicId.value === null) return false;
		debateTriggering.value = true;
		debateError.value = null;
		try {
			const res = await api.triggerDebate(selectedTopicId.value, resultId);
			if (res.success && res.data) {
				debates.value = res.data.debates ?? [];
				notifySuccess("辩论已触发");
				startDebatePolling(resultId);
				return true;
			}
			debateError.value = res.error || "触发辩论失败";
			notifyError(debateError.value);
			return false;
		} finally {
			debateTriggering.value = false;
		}
	}

	// ── qa actions（causal-analysis-agent 阶段3：报告追问 + 沉淀）────────────
	/** 加载某 result 的多轮追问历史（oldest first）。 */
	async function loadQA(resultId: number): Promise<boolean> {
		if (selectedTopicId.value === null) {
			qaList.value = [];
			return false;
		}
		qaLoading.value = true;
		qaError.value = null;
		const res = await api.listQA(selectedTopicId.value, resultId);
		if (res.success && res.data) {
			qaList.value = res.data;
		} else {
			qaList.value = [];
			qaError.value = res.error || "加载追问历史失败";
		}
		qaLoading.value = false;
		return res.success;
	}

	/**
	 * 问一轮（挂 latestResultId）。
	 * 后端 Ask 已 append 一行 source="qa" 的 QA，但响应只回 {answer,tool_calls,refs}
	 * （缺 id/source/sedimented/created_at），无法拼出完整行，故成功后重拉列表。
	 */
	async function askQuestion(question: string): Promise<boolean> {
		if (selectedTopicId.value === null) return false;
		const resultId = latestResultId.value;
		if (resultId === null) {
			qaError.value = "暂无报告，无法追问";
			notifyError(qaError.value);
			return false;
		}
		qaLoading.value = true;
		qaError.value = null;
		try {
			const res = await api.askQA(selectedTopicId.value, resultId, question);
			if (res.success) {
				// 捕获即时响应（含 refs），供 QAPanel 给最新这轮渲染双类引用。
				latestAnswer.value = res.data ?? null;
				await loadQA(resultId);
				return true;
			}
			qaError.value = res.error || "追问失败";
			notifyError(qaError.value);
			return false;
		} finally {
			qaLoading.value = false;
		}
	}

	/** 沉淀一轮（用后端回写的完整行替换 qaList 对应项，sedimented=true）。 */
	async function sedimentAnswer(qaId: number): Promise<boolean> {
		if (selectedTopicId.value === null) return false;
		const res = await api.sedimentQA(selectedTopicId.value, qaId);
		if (res.success && res.data) {
			const idx = qaList.value.findIndex((q) => q.id === qaId);
			if (idx >= 0) qaList.value[idx] = res.data;
			notifySuccess("已沉淀");
			return true;
		}
		notifyError(res.error || "沉淀失败");
		return false;
	}

	onUnmounted(stopDebatePolling);

	return {
		// topic selector
		topics,
		topicsLoading,
		selectedTopicId,
		loadTopics,
		selectTopic,
		// table 1
		contexts,
		contextsLoading,
		regenerating,
		loadContexts,
		saveContext,
		regenerateContext,
		// table 2
		results,
		resultsLoading,
		triggering,
		loadResults,
		triggerEnrichment,
		// table 3
		reviews,
		reviewsLoading,
		loadReviews,
		saveReviewDeviation,
		applyReview,
		createReview,
		// data sources
		dataSources,
		dataSourcesLoading,
		loadDataSources,
		saveDataSource,
		removeDataSource,
		// stock debates
		debates,
		debatesLoading,
		debateTriggering,
		debateError,
		debateStage,
		loadDebates,
		triggerDebate,
		// qa (causal-analysis-agent 阶段3)
		qaList,
		qaLoading,
		qaError,
		latestAnswer,
		loadQA,
		askQuestion,
		sedimentAnswer,
		// workbench UI（原型认知工作台）
		selectedGran,
		selectedPeriodIdx,
		periodList,
		currentContext,
		setGran,
		shiftPeriod,
		selectPeriod,
		activeSection,
		setActiveSection,
		latestResultId,
		latestResultDetail,
		latestResultDetailLoading,
		loadLatestResultDetail,
		// board-level analysis (board-level-deep-analysis)
		boardResults,
		boardResultsLoading,
		boardAnalysisTriggering,
		selectedBoardResult,
		selectedBoardResultId,
		loadBoardAnalysisResults,
		triggerBoardAnalysis,
		syncBoardAnalysisStatus,
		selectBoardResult,
		// misc
		error,
		loadAllTopicTables,
	};
}
