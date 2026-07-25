import type {
	SchedulerArticleRef,
	SchedulerLastRunSummary,
	SchedulerStatus,
} from "~/types/scheduler";

const contentCompletionAliases = new Set(["content_completion", "ai_summary"]);

type ContentCompletionPanelStatus = Pick<
	SchedulerStatus,
	"name" | "overview" | "is_executing" | "current_article"
>;
type SchedulerStatusLike = Pick<
	SchedulerStatus,
	"name" | "status" | "is_executing" | "database_state"
>;
type ContentCompletionArticleStatus = Pick<
	SchedulerStatus,
	| "name"
	| "is_executing"
	| "current_article"
	| "stale_processing_article"
	| "last_run_summary"
>;

export function isContentCompletionScheduler(name: string): boolean {
	return contentCompletionAliases.has(name);
}

export function isHotScheduler(name: string): boolean {
	return (
		name === "auto_refresh" ||
		isContentCompletionScheduler(name) ||
		name === "firecrawl"
	);
}

export function getSchedulerDisplayName(name: string): string {
	if (isContentCompletionScheduler(name)) {
		return "文章总结";
	}

	const names: Record<string, string> = {
		auto_refresh: "后台刷新",
		firecrawl: "全文爬取",
		tag_hierarchy_cleanup: "标签清理",
		aux_label_cleanup: "辅助标签清理",
		preference_profile_update: "兴趣画像重算",
		rsshub_catalog_sync: "订阅源目录同步",
		tag_quality_score: "标签质量评分",
		log_cleanup: "日志清理",
		daily_report: "每日报告",
		board_upgrade_suggest: "版块升级建议",
		blocked_article_recovery: "阻塞文章恢复",
		lifeline_weekly: "周度新闻汇总",
		lifeline_monthly: "月度新闻汇总",
		lifeline_yearly: "年度新闻汇总",
	};

	return names[name] || name;
}

export function getSchedulerIcon(name: string): string {
	if (isContentCompletionScheduler(name)) {
		return "mdi:text-box-search-outline";
	}

	const icons: Record<string, string> = {
		auto_refresh: "mdi:refresh",
		firecrawl: "mdi:spider-web",
		tag_hierarchy_cleanup: "mdi:tag-remove-outline",
		aux_label_cleanup: "mdi:tag-minus-outline",
		preference_profile_update: "mdi:account-heart-outline",
		rsshub_catalog_sync: "mdi:radar",
		tag_quality_score: "mdi:tag-star-outline",
		log_cleanup: "mdi:broom",
		daily_report: "mdi:newspaper-variant-outline",
		blocked_article_recovery: "mdi:file-restore-outline",
		lifeline_weekly: "mdi:calendar-week-begin-outline",
		lifeline_monthly: "mdi:calendar-month-outline",
		lifeline_yearly: "mdi:calendar-star",
	};

	return icons[name] || "mdi:cog";
}

export function getSchedulerColor(name: string): string {
	if (isContentCompletionScheduler(name)) {
		return "from-amber-500 to-orange-500";
	}

	const colors: Record<string, string> = {
		auto_refresh: "from-blue-500 to-cyan-500",
		firecrawl: "from-rose-500 to-orange-500",
		tag_hierarchy_cleanup: "from-violet-500 to-purple-600",
		aux_label_cleanup: "from-teal-500 to-emerald-500",
		preference_profile_update: "from-indigo-500 to-blue-500",
		rsshub_catalog_sync: "from-cyan-500 to-blue-500",
		tag_quality_score: "from-purple-500 to-pink-500",
		log_cleanup: "from-slate-500 to-gray-500",
		daily_report: "from-amber-500 to-yellow-500",
		blocked_article_recovery: "from-orange-500 to-red-500",
		lifeline_weekly: "from-sky-500 to-blue-500",
		lifeline_monthly: "from-cyan-500 to-teal-500",
		lifeline_yearly: "from-emerald-500 to-green-500",
	};

	return colors[name] || "from-gray-500 to-gray-600";
}

export function shouldShowContentCompletionPanel(
	scheduler: ContentCompletionPanelStatus,
): boolean {
	return (
		isContentCompletionScheduler(scheduler.name) &&
		Boolean(
			scheduler.overview || scheduler.is_executing || scheduler.current_article,
		)
	);
}

export function getSchedulerStatusLabel(
	scheduler: SchedulerStatusLike,
): string | undefined {
	if (
		isContentCompletionScheduler(scheduler.name) &&
		scheduler.is_executing !== true &&
		scheduler.status
	) {
		return mapStatusToChinese(scheduler.status);
	}

	const raw = scheduler.database_state?.status || scheduler.status;
	return raw ? mapStatusToChinese(raw) : raw;
}

export function mapStatusToChinese(status: string): string {
	const map: Record<string, string> = {
		idle: "空闲",
		running: "执行中",
		error: "失败",
		triggered: "已触发",
		stopped: "已停用",
		disabled: "已停用",
		failed: "失败",
	};
	return map[status] || status;
}

export function getCurrentContentCompletionArticle(
	scheduler: ContentCompletionArticleStatus,
): SchedulerArticleRef | null | undefined {
	if (scheduler.current_article) {
		return scheduler.current_article;
	}

	if (
		isContentCompletionScheduler(scheduler.name) &&
		scheduler.is_executing !== true
	) {
		return (
			scheduler.stale_processing_article ||
			scheduler.last_run_summary?.stale_processing_article ||
			null
		);
	}

	return null;
}

export function formatLastRunSummary(
	name: string,
	summary: SchedulerLastRunSummary | null | undefined,
): string {
	if (!summary) {
		return "";
	}

	// Try to build a human-readable summary based on scheduler name
	switch (name) {
		case "log_cleanup": {
			const logs = (summary as Record<string, unknown>)
				.last_ai_call_logs_deleted as number | undefined;
			const spans = (summary as Record<string, unknown>)
				.last_otel_spans_deleted as number | undefined;
			const parts: string[] = [];
			if (logs !== undefined) parts.push(`${logs} 条 AI 日志`);
			if (spans !== undefined) parts.push(`${spans} 条追踪`);
			return parts.length > 0
				? `清理了 ${parts.join("、")}`
				: summary.reason || "";
		}
		case "aux_label_cleanup": {
			const count = (summary as Record<string, unknown>).affected_count as
				| number
				| undefined;
			if (count !== undefined) return `清理了 ${count} 个辅助标签`;
			return summary.reason || "";
		}
		case "blocked_article_recovery": {
			const count = (summary as Record<string, unknown>).recovered_count as
				| number
				| undefined;
			if (count !== undefined) return `恢复了 ${count} 篇文章`;
			return summary.reason || "";
		}
		case "preference_profile_update": {
			const count = (summary as Record<string, unknown>).boards_computed as
				| number
				| undefined;
			if (count !== undefined) return `重算了 ${count} 个版块的兴趣画像`;
			return summary.reason || "";
		}
		case "rsshub_catalog_sync": {
			const count = (summary as Record<string, unknown>).total as
				| number
				| undefined;
			if (count !== undefined) return `目录共 ${count} 条路由`;
			return summary.reason || "";
		}
		case "daily_report": {
			const count =
				summary.report_count ??
				((summary as Record<string, unknown>).report_count as
					| number
					| undefined);
			if (count !== undefined) return `生成了 ${count} 份日报`;
			return summary.reason || "";
		}
		case "auto_refresh": {
			const triggered = summary.triggered_feeds;
			const scanned = summary.scanned_feeds;
			if (triggered !== undefined && scanned !== undefined) {
				return `刷新 ${triggered} 个订阅源（扫描 ${scanned} / 到期 ${triggered}）`;
			}
			return summary.reason || "";
		}
		case "firecrawl": {
			const completed = summary.completed_count;
			const failed = summary.failed_count;
			const total = (summary as Record<string, unknown>).total as
				| number
				| undefined;
			if (completed !== undefined && failed !== undefined) {
				return `处理了 ${total ?? completed + failed} 个任务（成功 ${completed} / 失败 ${failed}）`;
			}
			return summary.reason || "";
		}
		case "content_completion": {
			const completed = summary.completed_count;
			const failed = summary.failed_count;
			if (completed !== undefined && failed !== undefined) {
				return `完成 ${completed} 篇、失败 ${failed} 篇`;
			}
			return summary.reason || "";
		}
		default:
			return summary.reason || "";
	}
}
