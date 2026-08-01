/**
 * QAPanel — 报告追问 chat 组件测试（causal-analysis-agent 阶段3b-ii）.
 *
 * QAPanel 是纯 props/events 组件（useBoardEnrichment 是 instance-per-call，
 * 不能在子组件里二次调用，故走 props-down/events-up）。因此测试只 mount + 断言
 * 事件与渲染，不 mock composable/API。
 *
 * Covers:
 *  - 挂载 / resultId 变更 → emit load
 *  - 输入 + 发送 → emit ask（trim）+ 清空 draft；Enter 提交 / Shift+Enter 换行
 *  - 空态 / 错误态 / 加载禁用
 *  - 多轮线程渲染（Q + A + tool_calls trace）
 *  - refs 复用 AnalyzeRefChip（仅 latestAnswer 按 answer 匹配的轮有）
 *  - 沉淀按钮 → emit sediment；sedimented=true 显徽章、隐按钮
 */
import { describe, expect, it, vi } from "vitest";
import { mount } from "@vue/test-utils";
import { nextTick } from "vue";
import QAPanel from "./QAPanel.vue";
import type { TopicEnrichmentQA, AskQAResponse } from "~/api/boardEnrichment";

vi.mock("@iconify/vue", () => ({
	Icon: {
		name: "Icon",
		inheritAttrs: true,
		template: '<span class="icon-stub" aria-hidden="true" />',
	},
}));

function makeQA(over: Partial<TopicEnrichmentQA> = {}): TopicEnrichmentQA {
	return {
		id: over.id ?? 1,
		topic_enrichment_result_id: over.topic_enrichment_result_id ?? 10,
		question: over.question ?? "半导体短缺会持续多久？",
		answer: over.answer ?? "据报告推演，约 2-3 个季度（推演有据）。",
		tool_calls: over.tool_calls ?? [
			{ name: "web_search" },
			{ name: "get_lane_detail" },
		],
		source: over.source ?? "qa",
		sedimented: over.sedimented ?? false,
		created_at: over.created_at ?? "2026-06-21T10:00:00Z",
	};
}

function mountPanel(props: Record<string, unknown> = {}) {
	return mount(QAPanel, {
		props: {
			resultId: 10,
			qaList: [],
			qaLoading: false,
			qaError: null,
			latestAnswer: null,
			...props,
		},
	});
}

describe("QAPanel — 加载历史", () => {
	it("挂载时按 resultId emit load", () => {
		const wrapper = mountPanel({ resultId: 10 });
		expect(wrapper.emitted("load")).toBeTruthy();
		expect(wrapper.emitted("load")![0]).toEqual([10]);
	});

	it("resultId 变更后 emit load 新 id", async () => {
		const wrapper = mountPanel({ resultId: 10 });
		expect(wrapper.emitted("load")!.length).toBe(1);
		await wrapper.setProps({ resultId: 17 });
		expect(wrapper.emitted("load")!.length).toBe(2);
		expect(wrapper.emitted("load")![1]).toEqual([17]);
	});

	it("resultId=null 时不 emit load", () => {
		const wrapper = mountPanel({ resultId: null });
		expect(wrapper.emitted("load")).toBeFalsy();
	});
});

describe("QAPanel — 输入与提交", () => {
	it("输入后点发送 → emit ask（trim）并清空 draft", async () => {
		const wrapper = mountPanel();
		await wrapper.find(".qa-input").setValue("  下一轮会降价吗？  ");
		await wrapper.find(".qa-send").trigger("click");
		expect(wrapper.emitted("ask")).toBeTruthy();
		expect(wrapper.emitted("ask")![0]).toEqual(["下一轮会降价吗？"]);
		expect(
			(wrapper.find(".qa-input").element as HTMLTextAreaElement).value,
		).toBe("");
	});

	it("Enter 提交；Shift+Enter 不提交（换行）", async () => {
		const wrapper = mountPanel();
		await wrapper.find(".qa-input").setValue("问题A");
		await wrapper
			.find(".qa-input")
			.trigger("keydown", { key: "Enter", shiftKey: false });
		expect(wrapper.emitted("ask")).toBeTruthy();
		expect(wrapper.emitted("ask")![0]).toEqual(["问题A"]);

		// Shift+Enter 不提交
		await wrapper.find(".qa-input").setValue("问题B");
		await wrapper
			.find(".qa-input")
			.trigger("keydown", { key: "Enter", shiftKey: true });
		expect(wrapper.emitted("ask")!.length).toBe(1);
	});

	it("空 draft 时发送禁用、不 emit", async () => {
		const wrapper = mountPanel();
		const send = wrapper.find(".qa-send");
		expect(send.attributes("disabled")).toBeDefined();
		await send.trigger("click");
		expect(wrapper.emitted("ask")).toBeFalsy();
	});

	it("qaLoading=true 时发送禁用并显「追问中…」", () => {
		const wrapper = mountPanel({ qaLoading: true });
		const send = wrapper.find(".qa-send");
		expect(send.attributes("disabled")).toBeDefined();
		expect(send.text()).toContain("追问中…");
	});

	it("resultId=null 时即使有 draft 也不能提交", async () => {
		const wrapper = mountPanel({ resultId: null });
		await wrapper.find(".qa-input").setValue("问题");
		// canSubmit 为 false → disabled
		expect(wrapper.find(".qa-send").attributes("disabled")).toBeDefined();
	});
});

describe("QAPanel — 状态态", () => {
	it("空 qaList + 非 loading → 显空态提示", () => {
		const wrapper = mountPanel();
		expect(wrapper.find(".qa-empty").exists()).toBe(true);
		expect(wrapper.find(".qa-empty").text()).toContain("还没有追问");
	});

	it("qaError → 显错误条", () => {
		const wrapper = mountPanel({ qaError: "网络故障" });
		expect(wrapper.find(".qa-error").exists()).toBe(true);
		expect(wrapper.find(".qa-error").text()).toContain("网络故障");
	});
});

describe("QAPanel — 多轮线程渲染", () => {
	it("按 created_at 升序渲染 Q + A", async () => {
		const older = makeQA({
			id: 1,
			question: "旧问",
			answer: "旧答",
			created_at: "2026-06-20T10:00:00Z",
		});
		const newer = makeQA({
			id: 2,
			question: "新问",
			answer: "新答",
			created_at: "2026-06-21T10:00:00Z",
		});
		const wrapper = mountPanel({ qaList: [newer, older] }); // 故意倒序传入
		await nextTick();
		const turns = wrapper.findAll(".qa-turn");
		expect(turns.length).toBe(2);
		// 升序：旧在前
		expect(turns[0]!.find(".qa-q-text").text()).toBe("旧问");
		expect(turns[0]!.find(".qa-a-text").text()).toBe("旧答");
		expect(turns[1]!.find(".qa-q-text").text()).toBe("新问");
	});

	it("tool_calls 渲染为折叠 trace（低调）", async () => {
		const wrapper = mountPanel({
			qaList: [
				makeQA({
					tool_calls: [{ name: "web_search" }, { name: "get_lane_detail" }],
				}),
			],
		});
		await nextTick();
		const trace = wrapper.find(".qa-trace");
		expect(trace.exists()).toBe(true);
		expect(trace.find("summary").text()).toContain("2 次工具调用");
		expect(trace.findAll(".qa-trace-name").length).toBe(2);
	});

	it("无 tool_calls 时不渲染 trace（后端 jsonb 可能为 null / 空数组）", async () => {
		// 显式覆盖 tool_calls=null（绕开 makeQA 的 ?? 默认值），模拟后端 jsonb null
		const wrapper = mountPanel({ qaList: [{ ...makeQA(), tool_calls: null }] });
		await nextTick();
		expect(wrapper.find(".qa-trace").exists()).toBe(false);

		// 空数组同样不渲染
		const wrapper2 = mountPanel({ qaList: [{ ...makeQA(), tool_calls: [] }] });
		await nextTick();
		expect(wrapper2.find(".qa-trace").exists()).toBe(false);
	});
});

describe("QAPanel — refs 复用 AnalyzeRefChip", () => {
	const matchedAnswer = "据报告推演，约 2-3 个季度（推演有据）。";
	const latest: AskQAResponse = {
		answer: matchedAnswer,
		tool_calls: [],
		refs: [
			{ source_type: "news", ref: "ctx-7", quote: "供应链紧张" },
			{ source_type: "tool", ref: "web_search:半导体" },
		],
	};

	it("latestAnswer.answer 匹配的轮渲染 refs（复用 AnalyzeRefChip 的 .ref-chip）", async () => {
		const wrapper = mountPanel({
			qaList: [makeQA({ id: 1, answer: matchedAnswer })],
			latestAnswer: latest,
		});
		await nextTick();
		const turn = wrapper.findAll(".qa-turn")[0]!;
		expect(turn.find(".qa-refs").exists()).toBe(true);
		// AnalyzeRefChip 渲染为 .ref-chip（news/tool 两类）
		expect(turn.findAll(".ref-chip").length).toBe(2);
		expect(turn.findAll(".ref-chip.news").length).toBe(1);
		expect(turn.findAll(".ref-chip.tool").length).toBe(1);
	});

	it("latestAnswer 不匹配的轮不渲染 refs", async () => {
		const wrapper = mountPanel({
			qaList: [makeQA({ id: 1, answer: "完全不同的回答" })],
			latestAnswer: latest,
		});
		await nextTick();
		expect(wrapper.find(".qa-refs").exists()).toBe(false);
		expect(wrapper.findAll(".ref-chip").length).toBe(0);
	});

	it("无 latestAnswer 时不渲染 refs（历史轮无 refs）", async () => {
		const wrapper = mountPanel({ qaList: [makeQA()], latestAnswer: null });
		await nextTick();
		expect(wrapper.find(".qa-refs").exists()).toBe(false);
	});
});

describe("QAPanel — 沉淀交互", () => {
	it("未沉淀轮显「沉淀到报告」按钮；点击 emit sediment(id)", async () => {
		const wrapper = mountPanel({
			qaList: [makeQA({ id: 42, sedimented: false })],
		});
		await nextTick();
		const btn = wrapper.find(".qa-sed-btn");
		expect(btn.exists()).toBe(true);
		expect(btn.text()).toContain("沉淀到报告");
		await btn.trigger("click");
		expect(wrapper.emitted("sediment")).toBeTruthy();
		expect(wrapper.emitted("sediment")![0]).toEqual([42]);
	});

	it("已沉淀轮显「已沉淀」徽章、无沉淀按钮", async () => {
		const wrapper = mountPanel({
			qaList: [makeQA({ id: 42, sedimented: true })],
		});
		await nextTick();
		expect(wrapper.find(".qa-sed-badge").exists()).toBe(true);
		expect(wrapper.find(".qa-sed-badge").text()).toContain("已沉淀");
		expect(wrapper.find(".qa-sed-btn").exists()).toBe(false);
		// 已沉淀回答框换色（sedimented class）
		expect(wrapper.find(".qa-a.sedimented").exists()).toBe(true);
	});

	it("qaLoading 时沉淀按钮禁用", async () => {
		const wrapper = mountPanel({
			qaList: [makeQA({ id: 42 })],
			qaLoading: true,
		});
		await nextTick();
		expect(wrapper.find(".qa-sed-btn").attributes("disabled")).toBeDefined();
	});
});
