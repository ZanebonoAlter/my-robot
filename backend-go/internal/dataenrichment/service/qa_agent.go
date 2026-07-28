package service

import (
	"context"
	"encoding/json"
	"fmt"

	"go.opentelemetry.io/otel"
	"syntopica-backend/internal/dataenrichment/repository"
	"syntopica-backend/internal/platform/airouter"
	"syntopica-backend/internal/platform/tracing"
)

// QAAgent answers report follow-up questions. It reuses the SAME exploration
// agent loop (runToolLoop) as the enrichment flow — same three defenses
// (/no_think prefix, full tool history, dedup) and same tool registry — but
// seeds the system prompt with the immutable report snapshot (form/lens/analysis)
// instead of a research topic. Spec causal-analysis-agent 阶段2b "报告追问".
//
// Invariants respected:
//   - 报告不可变 (业务约束#2): Ask only appends a topic_enrichment_qa row under
//     the result_id; it never writes to topic_enrichment_result.
//   - Allowed tools are the always-on exploration set (multi-level entry points
//   - web_search), identical to the enrichment agent's floor.
type QAAgent struct {
	airouter     AirRouter
	toolRegistry *Registry
	repo         *repository.Repository
	capability   airouter.Capability
}

// NewQAAgent constructs a QA agent. The tool registry must be the same instance
// the orchestrator uses so the exploration tools (list_boards/list_lanes/
// get_lane_detail/web_search) are available.
func NewQAAgent(airouter AirRouter, toolRegistry *Registry, repo *repository.Repository, capability airouter.Capability) *QAAgent {
	return &QAAgent{
		airouter:     airouter,
		toolRegistry: toolRegistry,
		repo:         repo,
		capability:   capability,
	}
}

// QAAnswer is the structured output of one follow-up round.
type QAAnswer struct {
	Answer    string           `json:"answer"`     // final summary from the agent loop (finish.summary)
	ToolCalls []ToolCallRecord `json:"tool_calls"` // tool calls made during this round
	Refs      []Ref            `json:"refs"`       // dual-class refs (📰news report context / 🔧tool results)
}

// qaSystemPrompt is the system prompt for the report follow-up agent.
// It embeds the immutable report snapshot so the agent answers FROM the report
// and only reaches for tools to supplement fresh data.
//
// Placeholders: 分析视角 / 话题形态 / 报告分析(JSON) / 可用工具.
const qaSystemPrompt = `你是一位产业探索判断分析师的追问助手。用户正在阅读一份已生成的探索判断报告，对报告内容有追问。

报告背景:
- 分析视角: %s
- 话题形态: %s
- 报告分析(JSON): %s

你的任务: 针对用户的追问，基于报告已有分析作答。如需补充最新数据可调用工具（实时行情/版块全景/泳道详情/网络搜索）。

回答纪律:
1. 先基于报告已有分析作答，不要凭空臆测
2. 如需补充最新数据，调工具获取后再回答
3. 答案末尾标注确定性（已验证/推演有据/假设情景/存疑待证）
4. 引用来源：报告依据(news)或工具结果(tool)

可用工具:
%s

每一轮输出严格 JSON,二选一:
- 继续调工具:{"action": "call_tool", "thought": "...", "tool": "工具名", "args": {...}}
- 宣布完成:{"action": "finish", "thought": "...", "summary": "给用户的完整回答(含确定性标注)"}

不要输出 JSON 以外的任何内容。`

// Ask runs one follow-up round for a report (result) and persists it as a new
// topic_enrichment_qa row (source="qa"). The report itself is never modified.
func (q *QAAgent) Ask(ctx context.Context, resultID uint, question string) (*QAAnswer, error) {
	ctx, span := otel.Tracer(tracing.ServiceName).Start(ctx, "QAAgent.Ask")
	defer span.End()
	// 1. Read the immutable result snapshot for report context.
	result, err := q.repo.GetTopicEnrichmentResultByID(ctx, resultID)
	if err != nil {
		return nil, fmt.Errorf("qa ask: load result %d: %w", resultID, err)
	}

	// 2. Parse the composite sectors JSON ({form,lens,analysis}) for the prompt.
	var sectors analyzeOutput
	if len(result.Sectors) > 0 {
		// Best-effort parse: a malformed/empty sectors degrades to empty context
		// rather than failing the whole ask (old-format rows lack the form field).
		_ = json.Unmarshal(result.Sectors, &sectors)
	}
	system := q.buildSystemPrompt(sectors)

	// 3. Reuse the exploration agent loop. Allowed tools = the always-on
	//    exploration floor (multi-level entry points + web_search), the same
	//    set the enrichment agent gets unconditionally.
	sessionID := fmt.Sprintf("data_enrichment_qa_%d_%s", resultID, RandomHex(8))
	loop, err := runToolLoop(ctx, q.airouter, q.toolRegistry, q.capability, toolLoopParams{
		sessionID:    sessionID,
		systemPrompt: system,
		taskLine:     "用户追问: " + question,
		operation:    "data_enrichment.qa_tool_use",
		allowedTools: explorationToolNames,
		maxLoops:     maxAgentLoops,
	})
	if err != nil {
		return nil, fmt.Errorf("qa ask: agent loop: %w", err)
	}

	// 4. Build the structured answer. Answer = the loop's final summary; refs
	//    = dual-class refs (news from report context + tool from this round).
	answer := &QAAnswer{
		Answer:    loop.FinalData,
		ToolCalls: loop.ToolCalls,
		Refs:      buildQARefs(sectors, loop.ToolCalls),
	}

	// 5. Persist the round as an append-only qa row. Source is fixed "qa".
	toolCallsJSON, _ := json.Marshal(loop.ToolCalls)
	qaRow := &repository.TopicEnrichmentQA{
		TopicEnrichmentResultID: resultID,
		Question:                question,
		Answer:                  loop.FinalData,
		ToolCalls:               toolCallsJSON,
		Source:                  "qa",
	}
	if err := q.repo.CreateTopicEnrichmentQA(ctx, qaRow); err != nil {
		return nil, fmt.Errorf("qa ask: persist round: %w", err)
	}

	return answer, nil
}

// buildSystemPrompt renders the report-context system prompt. A compact JSON
// snapshot of the analysis body is embedded so the agent answers from the
// report; tools are advertised via the shared buildToolsDesc helper.
func (q *QAAgent) buildSystemPrompt(sectors analyzeOutput) string {
	analysisJSON, _ := json.Marshal(sectors.Analysis)
	if len(analysisJSON) == 0 {
		analysisJSON = []byte("{}")
	}
	toolsDesc := buildToolsDesc(q.toolRegistry, explorationToolNames)
	return fmt.Sprintf(qaSystemPrompt, sectors.Lens, sectors.Form, string(analysisJSON), toolsDesc)
}

// buildQARefs assembles the dual-class refs for a QA round:
//   - news refs: evidence carried by the report's analysis (the report's own
//     news-derived grounding; surfaced so the UI can link back to the report)
//   - tool refs: each tool call made during this round (machine-verifiable,
//     source_type=tool)
func buildQARefs(sectors analyzeOutput, toolCalls []ToolCallRecord) []Ref {
	refs := make([]Ref, 0)

	// News refs from the report's analysis body.
	for _, r := range collectAnalysisNewsRefs(sectors.Analysis) {
		if r.SourceType == "" {
			r.SourceType = "news"
		}
		refs = append(refs, r)
	}

	// Tool refs from this round's calls.
	for _, tc := range toolCalls {
		preview := tc.ResultPreview
		if preview == "" {
			preview = tc.ResultFull
		}
		refs = append(refs, Ref{
			SourceType: "tool",
			Ref:        tc.Tool,
			Quote:      preview,
		})
	}
	return refs
}

// collectAnalysisNewsRefs walks an AnalysisBody and collects the news-typed
// evidence refs embedded in it. Returns nil for forms with no evidence lists.
func collectAnalysisNewsRefs(body AnalysisBody) []Ref {
	switch b := body.(type) {
	case EventChainAnalysis:
		var out []Ref
		for _, f := range b.FactLayer {
			out = append(out, f.Evidence...)
		}
		for _, t := range b.Timeline {
			if t.Ref != nil {
				out = append(out, *t.Ref)
			}
		}
		for _, ins := range b.InsightLayer {
			out = append(out, ins.Evidence...)
			out = append(out, ins.WebVerified...)
		}
		return out
	case ThemeVeinAnalysis:
		var out []Ref
		for _, v := range b.Veins {
			out = append(out, v.Evidence...)
		}
		for _, ins := range b.CrossInsight {
			out = append(out, ins.Evidence...)
			out = append(out, ins.WebVerified...)
		}
		return out
	case SinglePointAnalysis:
		return append([]Ref(nil), b.Evidence...)
	case SparseAnalysis:
		return nil
	}
	return nil
}
