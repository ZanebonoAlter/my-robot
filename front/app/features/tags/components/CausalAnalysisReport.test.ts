/**
 * CausalAnalysisReport — 深度层 + structural 形态渲染测试（data-enrichment-structural-depth §5.3）。
 *
 * 本组件是纯 props 渲染组件（消费 ResultDetailRow.sectors 的 AnalyzeOutput 判别联合），
 * 测试只 mount + 断言渲染产物，不 mock composable/API。
 *
 * Covers:
 *  - event_chain 含 depth → 渲染深度层六区块（系统重定位/多层机制/历史类比/范式转折/边界/证据链）
 *  - 证据链：news 📰 / web 🌐 / page 📄 图标；web/page 的 url 渲染为可点击 <a target="_blank">
 *  - regime_shift=null → 不渲染范式转折
 *  - 旧结果无 depth 字段 → 深度层区块不渲染、事实层照渲染（降级不崩）
 *  - structural 形态 → 演化叙述 + 关键阶段 + 深度层
 *  - sparse 形态 → 无深度层（骨感禁深度层）
 */
import { describe, expect, it } from "vitest";
import { mount } from "@vue/test-utils";
import CausalAnalysisReport from "./CausalAnalysisReport.vue";
import type {
	AnalyzeDepth,
	AnalyzeOutput,
	ResultDetailRow,
} from "~/api/boardEnrichment";

function makeResult(sectors: AnalyzeOutput | null): ResultDetailRow {
	return {
		id: 1,
		persistent_topic_id: 7,
		evolution_assessment: "",
		sectors,
		causal_chain: null,
		tool_calls: [],
		input_snapshot: null,
		session_id: "s-1",
		created_at: "2026-08-05T10:00:00Z",
	};
}

function makeDepth(over: Partial<AnalyzeDepth> = {}): AnalyzeDepth {
	return {
		system_reframe: "这不是单一事件，而是全球货币体系重构的一环。",
		mechanism_layers: [
			{
				layer: "结算层",
				deep_logic: "本币结算绕开 SWIFT 降低美元依赖",
				basis: "CIPS 交易量持续上升",
			},
		],
		historical_analogy: [
			{
				case: "1973 年石油美元确立",
				mechanism: "大宗商品计价绑定货币霸权",
				diff: "本轮是多极化而非单极替代",
			},
		],
		regime_shift: {
			judgment: "可能处于范式转折早期",
			evidence: "美元储备份额连续多季下降",
		},
		boundary: "目前还不能断定去美元化已不可逆。",
		evidence_chain: [
			{ source_type: "news", ref: "ctx:12", quote: "新闻引文一" },
			{
				source_type: "web",
				url: "https://example.com/bis-report",
				quote: "网页引文二",
				institution: "BIS",
				date: "2026-07",
			},
			{
				source_type: "page",
				url: "https://example.com/fulltext",
				quote: "正文摘录三",
			},
		],
		...over,
	};
}

function makeEventChainOutput(depth?: AnalyzeDepth): AnalyzeOutput {
	return {
		form: "event_chain",
		lens: "货币体系视角",
		analysis: {
			fact_layer: [{ claim: "事实一", evidence: [], verified: true }],
			timeline: [{ date: "2026-07-01", event: "节点一" }],
			insight_layer: [],
			...(depth ? { depth } : {}),
		},
	};
}

function mountReport(sectors: AnalyzeOutput | null) {
	return mount(CausalAnalysisReport, {
		props: { result: makeResult(sectors), topicLabel: "人民币国际化" },
	});
}

describe("CausalAnalysisReport — 深度层渲染", () => {
	it("event_chain 含 depth → 渲染深度层六区块", () => {
		const wrapper = mountReport(makeEventChainOutput(makeDepth()));
		const depth = wrapper.find(".depth-layer");
		expect(depth.exists()).toBe(true);
		const text = depth.text();
		// 系统重定位 / 多层机制 / 历史类比 / 范式转折 / 边界限定 / 可核查证据链
		expect(text).toContain("系统重定位");
		expect(text).toContain("这不是单一事件，而是全球货币体系重构的一环");
		expect(text).toContain("多层机制");
		expect(text).toContain("结算层");
		expect(text).toContain("CIPS 交易量持续上升");
		expect(text).toContain("历史类比");
		expect(text).toContain("1973 年石油美元确立");
		expect(text).toContain("本轮是多极化而非单极替代");
		expect(text).toContain("范式转折");
		expect(text).toContain("可能处于范式转折早期");
		expect(text).toContain("边界限定");
		expect(text).toContain("目前还不能断定去美元化已不可逆");
		expect(text).toContain("可核查证据链");
	});

	it("证据链：news 📰 / web 🌐 / page 📄 图标，web/page url 可点击", () => {
		const wrapper = mountReport(makeEventChainOutput(makeDepth()));
		const text = wrapper.find(".depth-layer").text();
		expect(text).toContain("📰");
		expect(text).toContain("🌐");
		expect(text).toContain("📄");
		// web/page 类 url 渲染为可点击外链
		const links = wrapper.findAll(".depth-layer a.dp-ev-link");
		const hrefs = links.map((a) => a.attributes("href"));
		expect(hrefs).toContain("https://example.com/bis-report");
		expect(hrefs).toContain("https://example.com/fulltext");
		for (const a of links) {
			expect(a.attributes("target")).toBe("_blank");
			expect(a.attributes("rel")).toContain("noopener");
		}
		// 机构 + 日期 + 原文摘录
		expect(text).toContain("BIS");
		expect(text).toContain("2026-07");
		expect(text).toContain("网页引文二");
		expect(text).toContain("正文摘录三");
	});

	it("regime_shift=null → 不渲染范式转折区块", () => {
		const wrapper = mountReport(
			makeEventChainOutput(makeDepth({ regime_shift: null })),
		);
		const depth = wrapper.find(".depth-layer");
		expect(depth.exists()).toBe(true);
		expect(depth.find(".dp-regime").exists()).toBe(false);
		expect(depth.text()).not.toContain("范式转折");
	});

	it("旧结果无 depth 字段 → 深度层不渲染、事实层照渲染（降级不崩）", () => {
		const wrapper = mountReport(makeEventChainOutput(undefined));
		expect(wrapper.find(".depth-layer").exists()).toBe(false);
		// 既有事实层渲染不受影响
		expect(wrapper.find(".fact-layer").exists()).toBe(true);
		expect(wrapper.text()).toContain("事实一");
	});
});

describe("CausalAnalysisReport — structural 形态", () => {
	it("渲染演化叙述 + 关键阶段 + 深度层", () => {
		const wrapper = mountReport({
			form: "structural",
			lens: "货币体系视角",
			analysis: {
				evolution_narrative: "人民币国际化经历三个阶段的长时段演化。",
				phases: [
					{ period: "2009-2015", event: "跨境贸易结算试点" },
					{
						period: "2016",
						event: "加入 SDR",
						ref: { source_type: "news", ref: "ctx:9" },
					},
				],
				depth: makeDepth(),
			},
		});
		const text = wrapper.text();
		expect(text).toContain("结构演化");
		expect(text).toContain("人民币国际化经历三个阶段的长时段演化");
		expect(text).toContain("关键阶段");
		expect(text).toContain("2009-2015");
		expect(text).toContain("加入 SDR");
		// 深度层同样渲染
		expect(wrapper.find(".depth-layer").exists()).toBe(true);
	});
});

describe("CausalAnalysisReport — sparse 禁深度层", () => {
	it("sparse 形态不渲染深度层", () => {
		const wrapper = mountReport({
			form: "sparse",
			lens: "",
			analysis: {
				notice: "信息不足，暂无法展开完整因果分析。",
				summary: "仅有一条相关报道。",
			},
		});
		expect(wrapper.find(".depth-layer").exists()).toBe(false);
		expect(wrapper.find(".sparse-notice").exists()).toBe(true);
	});
});
