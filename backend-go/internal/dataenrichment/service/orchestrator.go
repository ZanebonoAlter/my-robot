package service

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	mrand "math/rand"
	"strings"

	"gorm.io/gorm"

	"syntopica-backend/internal/dataenrichment/repository"
	"syntopica-backend/internal/platform/airouter"
	"syntopica-backend/internal/platform/logging"
)

// capabilityAnalysis is the airouter Capability for cycle-B LLM calls.
// Mirrors capabilityAnalysis (defined in root package's capability.go).
const capabilityAnalysis airouter.Capability = "data_enrichment_analysis"

// ── Types ───────────────────────────────────────────────────────────────────

// ToolCallRecord mirrors PoC roles.py ToolCall for agent loop tracing.
type ToolCallRecord struct {
	Step          int            `json:"step"`
	Thought       string         `json:"thought"`
	Tool          string         `json:"tool"`
	Args          map[string]any `json:"args"`
	ResultPreview string         `json:"result_preview"`
	ResultFull    string         `json:"result_full"`
}

// AgentLoopResult holds the output of a single agent loop run.
type AgentLoopResult struct {
	Topic     string           `json:"topic"`
	ToolCalls []ToolCallRecord `json:"tool_calls"`
	FinalData string           `json:"final_data"`
	Loops     int              `json:"loops"`
	Error     string           `json:"error,omitempty"`
}

// EnrichmentOutput is the composite result of one EnrichTopic run.
type EnrichmentOutput struct {
	Result     *repository.TopicEnrichmentResult
	Review     *repository.TopicEnrichmentReview // nil if no review generated
	AgentLoops []*AgentLoopResult                // per-topic agent loop traces
}

// ── OrchestratorService ─────────────────────────────────────────────────────

// OrchestratorService runs the cycle-B data enrichment flow (三角色编排).
// See design.md §3 and overview.md §4.
type OrchestratorService struct {
	airouter          AirRouter
	repo              *repository.Repository
	lifelineReader    LifelineReader
	renderer          *LifelineRenderer
	toolRegistry      *Registry
	boardConfigReader BoardConfigReader
}

// NewOrchestratorService creates a new orchestrator with required dependencies.
func NewOrchestratorService(
	airouter AirRouter,
	repo *repository.Repository,
	lifelineReader LifelineReader,
	renderer *LifelineRenderer,
	toolRegistry *Registry,
	boardConfigReader BoardConfigReader,
) *OrchestratorService {
	return &OrchestratorService{
		airouter:          airouter,
		repo:              repo,
		lifelineReader:    lifelineReader,
		renderer:          renderer,
		toolRegistry:      toolRegistry,
		boardConfigReader: boardConfigReader,
	}
}

// ── EnrichTopic: main entry ─────────────────────────────────────────────────

// EnrichTopic runs the full cycle-B flow for a persistent topic.
// Returns the immutable result snapshot and optionally a review.
func (o *OrchestratorService) EnrichTopic(ctx context.Context, topicID uint) (*EnrichmentOutput, error) {
	// 1. Generate session ID.
	sessionID := generateSessionID(topicID)

	// 2. Read board config.
	cfg, err := o.boardConfigReader.GetBoardConfig(ctx, topicID)
	if err != nil {
		return nil, fmt.Errorf("enrich topic %d: board config: %w", topicID, err)
	}
	if !cfg.EnrichmentEnabled {
		return nil, fmt.Errorf("enrich topic %d: enrichment not enabled for this board", topicID)
	}

	// 3. Render lifeline (14-day detail).
	lifelineText, err := o.renderer.RenderLifelineForAgent(o.lifelineReader, topicID, cfg.WindowDays)
	if err != nil {
		return nil, fmt.Errorf("enrich topic %d: render lifeline: %w", topicID, err)
	}

	// 4. Read context layers (filtered by config).
	contextText, contextSnap, err := o.readContextLayers(ctx, topicID, cfg.ContextLayers)
	if err != nil {
		return nil, fmt.Errorf("enrich topic %d: read context: %w", topicID, err)
	}

	// 5. Read applied reviews (for interpret input).
	appliedReviews, err := o.repo.ListAppliedTopicEnrichmentReviews(ctx, topicID)
	if err != nil {
		return nil, fmt.Errorf("enrich topic %d: list reviews: %w", topicID, err)
	}
	reviewIDs := make([]uint, 0, len(appliedReviews))
	reviewText := ""
	for _, r := range appliedReviews {
		reviewIDs = append(reviewIDs, r.ID)
		reviewText += fmt.Sprintf("- [review #%d] %s\n", r.ID, r.DeviationSummary)
	}

	// 6. Step 1: Interpret — extract topics to research.
	topics, err := o.interpret(ctx, sessionID, lifelineText, contextText, reviewText)
	if err != nil {
		return nil, fmt.Errorf("enrich topic %d: interpret: %w", topicID, err)
	}

	// 7. Step 2: Agent loop per topic.
	agentResults := make([]*AgentLoopResult, 0, len(topics))
	topicsData := make([]map[string]any, 0, len(topics))
	for _, t := range topics {
		ar, err := o.runAgentLoop(ctx, sessionID, t.topic, lifelineText)
		agentResults = append(agentResults, ar)
		if err != nil {
			// Agent loop error is non-fatal; record and continue.
			ar.Error = err.Error()
		}
		topicsData = append(topicsData, map[string]any{
			"topic": t.topic,
			"data":  ar.FinalData,
		})
	}

	// 8. Step 3: Analyze — synthesize context + market data.
	analysis, err := o.analyze(ctx, sessionID, lifelineText, contextText, topicsData)
	if err != nil {
		return nil, fmt.Errorf("enrich topic %d: analyze: %w", topicID, err)
	}

	// 9. Build input snapshot for traceability.
	inputSnap := buildInputSnapshot(contextSnap, reviewIDs, cfg.WindowDays, cfg.ContextLayers)

	// 10. Build tool_calls JSON from all agent loops.
	allToolCalls := make([]ToolCallRecord, 0)
	for _, ar := range agentResults {
		allToolCalls = append(allToolCalls, ar.ToolCalls...)
	}
	toolCallsJSON, _ := json.Marshal(allToolCalls)

	// 11. Write immutable result.
	sectorsJSON, _ := json.Marshal(analysis.sectors)
	snapJSON, _ := json.Marshal(inputSnap)

	result := &repository.TopicEnrichmentResult{
		PersistentTopicID:   topicID,
		EvolutionAssessment: analysis.evolutionAssessment,
		Sectors:             sectorsJSON,
		CausalChain:         analysis.causalChain,
		ToolCalls:           toolCallsJSON,
		InputSnapshot:       snapJSON,
		SessionID:           sessionID,
	}
	if err := o.repo.CreateTopicEnrichmentResult(ctx, result); err != nil {
		return nil, fmt.Errorf("enrich topic %d: save result: %w", topicID, err)
	}

	// 12. Step 4: Review judge (if prev result exists).
	var review *repository.TopicEnrichmentReview
	prevResult, prevErr := o.repo.GetPrevLatestTopicEnrichmentResult(ctx, topicID, result.ID)
	if prevErr != nil {
		if !errors.Is(prevErr, gorm.ErrRecordNotFound) {
			// Real DB error — log and skip review.
			logging.Warnf("enrich topic %d: get prev result for review judge: %v", topicID, prevErr)
		}
		// No previous result (ErrRecordNotFound) is normal — silently skip review.
	}
	if prevErr == nil && prevResult != nil {
		prevJSON, _ := json.Marshal(map[string]any{
			"evolution_assessment": prevResult.EvolutionAssessment,
			"sectors":              json.RawMessage(prevResult.Sectors),
		})
		currJSON, _ := json.Marshal(map[string]any{
			"evolution_assessment": result.EvolutionAssessment,
			"sectors":              json.RawMessage(result.Sectors),
		})
		rj, rjErr := o.runReviewJudge(ctx, sessionID, string(prevJSON), string(currJSON))
		if rjErr == nil && rj != nil && rj.ShouldReview {
			conf := rj.Confidence
			review = &repository.TopicEnrichmentReview{
				PersistentTopicID: topicID,
				PrevResultID:      &prevResult.ID,
				CurrResultID:      result.ID,
				DeviationSummary:  rj.DeviationSummary,
				AffectedContext:   rj.AffectedContext,
				Confidence:        &conf,
				Applied:           false,
				Source:            "llm_assisted",
			}
			if err := o.repo.CreateTopicEnrichmentReview(ctx, review); err != nil {
				return nil, fmt.Errorf("enrich topic %d: save review: %w", topicID, err)
			}
		}
	}

	return &EnrichmentOutput{
		Result:     result,
		Review:     review,
		AgentLoops: agentResults,
	}, nil
}

// ── Interpret (角色①) ──────────────────────────────────────────────────────

type interpretTopic struct {
	topic  string
	reason string
}

// interpretPrompt is the LLM prompt for the interpreter role.
// Ported from PoC roles_evolved.py:interpret_lifeline.
const interpretPrompt = `你是一位资深产业分析师。下面是一个持久话题的演进脉络(跨多天的事件演进)。

你的任务:基于这个演进脉络,提炼出**需要查询哪些产业/板块的实时行情数据**,以便佐证或丰富对"这次最新进展在整个演进里意味着什么"的判断。

要求:
- 主题必须是 A 股有对应 ETF 的产业方向(如:石油/能源、黄金/贵金属、军工、航空、航运/物流、光伏/新能源、化工、半导体等)
- 每个主题给出"为什么要查它"的理由(关联到演进脉络的哪一天/哪个环节)
- 提炼 3-5 个主题,聚焦最能佐证演进判断的方向

输出严格 JSON:
{"topics": [{"topic": "产业主题词", "reason": "关联演进:...所以需要查它的实时表现"}]}`

func (o *OrchestratorService) interpret(ctx context.Context, sessionID, lifelineText, contextText, reviewText string) ([]interpretTopic, error) {
	prompt := interpretPrompt + "\n\n---\n"

	if contextText != "" {
		prompt += "分层新闻上下文:\n" + contextText + "\n\n"
	}
	if reviewText != "" {
		prompt += "历史认知记录(避免重蹈已知偏差):\n" + reviewText + "\n\n"
	}
	prompt += "话题演进脉络:\n" + lifelineText

	resp, err := o.airouter.Chat(ctx, airouter.ChatRequest{
		Capability:  capabilityAnalysis,
		Operation:   "data_enrichment.interpret",
		SessionID:   sessionID,
		Messages:    []airouter.Message{{Role: "user", Content: prompt}},
		Temperature: floatPtr(0.2),
		JSONMode:    true,
	})
	if err != nil {
		return nil, fmt.Errorf("interpret chat: %w", err)
	}

	parsed, err := ParseJSONResponse(resp.Content)
	if err != nil {
		return nil, fmt.Errorf("interpret parse: %w", err)
	}

	topicsRaw, ok := parsed["topics"].([]any)
	if !ok {
		return nil, fmt.Errorf("interpret: missing or invalid 'topics' field in response")
	}

	topics := make([]interpretTopic, 0, len(topicsRaw))
	for _, t := range topicsRaw {
		tm, ok := t.(map[string]any)
		if !ok {
			continue
		}
		topic, _ := tm["topic"].(string)
		reason, _ := tm["reason"].(string)
		if topic != "" {
			topics = append(topics, interpretTopic{topic: topic, reason: reason})
		}
	}

	if len(topics) == 0 {
		return nil, fmt.Errorf("interpret: no topics extracted from response")
	}

	return topics, nil
}

// ── Agent Loop (角色②) ─────────────────────────────────────────────────────

const maxAgentLoops = 6

// agentLoopSystemPrompt is the system prompt for the data query agent.
// Ported from PoC roles_evolved.py:research_topic_evolved.
const agentLoopSystemPrompt = `你是一位 A 股数据查询员。背景:有一个持久话题正在演进(见下方脉络),你需要针对给定的产业主题,查到相关的 ETF 实时行情数据,帮助分析"最新进展在这个演进里的意义"。

可用工具:
%s

工作流程(重要):
1. 先用 list_etf_by_keyword 用主题词查有没有对应 ETF
2. 如果命中很少(0-1 个),换更宽泛或相关的产业词重查(例如"光刻机"→"半导体"/"芯片")。最多换 2-3 个词
3. 拿到 ETF 代码后,用 get_etf_quote 查实时行情,取 3-5 只代表性 ETF 即可
4. 拿到行情数据后,立即宣布完成

关键纪律(违反会导致死循环):
- 工具返回的数据是完整的。total_count 就是真实命中数,不要因为"看起来不全"重查同一个关键词
- 查行情取 3-5 只代表性 ETF 即可,不需要全部代码
- 绝对不要用相同参数重复调用同一个工具

每一轮输出严格 JSON,二选一:
- 继续调工具:{"action": "call_tool", "thought": "...", "tool": "工具名", "args": {...}}
- 宣布完成:{"action": "finish", "thought": "...", "summary": "给分析师的简明数据汇总"}

不要输出 JSON 以外的任何内容。

话题演进脉络(背景):
%s`

// runAgentLoop executes the agent loop for a single research topic.
// Implements the three defenses from spec:
//
//	① /no_think prefix in user message (PoC double-insurance, complements DB-level enable_thinking=false)
//	② Full tool results in history (never truncated)
//	③ Deduplication: same tool+args blocked
func (o *OrchestratorService) runAgentLoop(ctx context.Context, sessionID, topic, lifelineText string) (*AgentLoopResult, error) {
	toolsDesc := buildToolsDesc(o.toolRegistry)
	system := fmt.Sprintf(agentLoopSystemPrompt, toolsDesc, lifelineText)

	result := &AgentLoopResult{Topic: topic}
	historyLines := make([]string, 0)
	seenCalls := make(map[string]bool) // dedup key → true

	for step := 1; step <= maxAgentLoops; step++ {
		result.Loops = step

		historyBlock := strings.Join(historyLines, "\n")
		if historyBlock == "" {
			historyBlock = "(尚无工具调用)"
		}

		// 防御① /no_think prefix (双保险 — DB provider 配置是主防线).
		userMsg := fmt.Sprintf("/no_think\n当前要查询的产业主题: %s\n\n已有的工具调用历史:\n%s\n\n请决定下一步(调工具或宣布完成),输出 JSON。", topic, historyBlock)

		resp, err := o.airouter.Chat(ctx, airouter.ChatRequest{
			Capability:  capabilityAnalysis,
			Operation:   "data_enrichment.tool_use",
			SessionID:   sessionID,
			Messages:    []airouter.Message{{Role: "system", Content: system}, {Role: "user", Content: userMsg}},
			Temperature: floatPtr(0.2),
			JSONMode:    true,
		})
		if err != nil {
			result.Error = fmt.Sprintf("第%d轮 LLM 调用失败: %v", step, err)
			break
		}

		decision, parseErr := ParseJSONResponse(resp.Content)
		if parseErr != nil {
			result.Error = fmt.Sprintf("第%d轮 LLM 输出无法解析: %s", step, prefix(resp.Content, 200))
			break
		}

		action, _ := decision["action"].(string)
		thought, _ := decision["thought"].(string)

		switch action {
		case "finish":
			result.FinalData, _ = decision["summary"].(string)
			return result, nil

		case "call_tool":
			toolName, _ := decision["tool"].(string)
			argsRaw := decision["args"]
			args, ok := argsRaw.(map[string]any)
			if !ok {
				args = map[string]any{}
			}

			// 防御③ Dedup check.
			dedupKey := dedupKeyFor(toolName, args)
			if seenCalls[dedupKey] {
				errJSON := fmt.Sprintf(`{"error":"已用相同参数调用过 %s,不要重复,基于已有数据继续。"}`, toolName)
				tc := ToolCallRecord{
					Step:          step,
					Thought:       thought + " [被拦:重复]",
					Tool:          toolName,
					Args:          args,
					ResultPreview: errJSON,
					ResultFull:    errJSON,
				}
				result.ToolCalls = append(result.ToolCalls, tc)
				historyLines = append(historyLines, fmt.Sprintf("第%d步: 调用 %s(%s) — 结果: %s", step, toolName, argsToJSON(args), errJSON))
				continue
			}
			seenCalls[dedupKey] = true

			// Execute tool.
			toolResult, toolErr := o.toolRegistry.Execute(ctx, toolName, args)
			if toolErr != nil {
				toolResult = fmt.Sprintf(`{"error":"%s"}`, toolErr.Error())
			}

			// 防御② Full result in history (no truncation for LLM).
			preview := toolResult
			if len(preview) > 300 {
				preview = preview[:300]
			}

			tc := ToolCallRecord{
				Step:          step,
				Thought:       thought,
				Tool:          toolName,
				Args:          args,
				ResultPreview: preview,
				ResultFull:    toolResult,
			}
			result.ToolCalls = append(result.ToolCalls, tc)
			historyLines = append(historyLines, fmt.Sprintf("第%d步: 调用 %s(%s) — 想法: %s — 结果: %s", step, toolName, argsToJSON(args), thought, toolResult))

		default:
			result.Error = fmt.Sprintf("第%d轮 action 不合法: %s", step, action)
			return result, nil
		}
	}

	// Exhausted maxLoops.
	if result.Error == "" {
		result.Error = fmt.Sprintf("达到最大循环数 %d 未完成", maxAgentLoops)
	}
	return result, nil
}

// ── Analyze (角色③) ────────────────────────────────────────────────────────

// analyzeOutput is the structured output of the analyst role.
type analyzeOutput struct {
	evolutionAssessment string
	sectors             []map[string]any
	causalChain         string
	overall             string
}

// analyzePrompt is the LLM prompt for the analyst role.
// Ported from PoC roles_evolved.py:analyze_evolved_impact.
const analyzePrompt = `你是一位资深 A 股策略分析师。下面是一个持久话题的完整演进脉络,以及补充查到的 ETF 实时行情数据。

你的任务:**结合演进脉络和实时数据,判断最新进展在这个话题演进里意味着什么**。

分析要求:
- 不要孤立地判断"利好/利空",而要回答:这次进展是**强化了既有趋势**,还是**出现了转折/扩散**?
- 引用演进脉络里具体哪一天的线索作为对比基准(比如"相比7-02的化工承压,7-04的数据显示...")
- 用查到的实时行情数据佐证你的判断(具体到 ETF 涨跌)
- 识别演进中的**因果链**(油价飙升 → 哪些板块连锁反应)
- 如有数据与演进叙事矛盾的,明确指出

输出严格 JSON:
{"evolution_assessment": "最新进展在演进中的定位(强化/转折/扩散 + 理由)",
 "sectors": [
   {"sector": "...", "evolution_role": "在因果链中的位置(源头/传导/末端)", "current_signal": "实时数据给出的信号", "vs_history": "相比前几日的演变", "judgment": "利好/利空/中性", "confidence": "高/中/低"}
 ],
 "causal_chain": "演进中的因果链描述",
 "overall": "一句话总结这次进展在整个演进里的意义"}`

func (o *OrchestratorService) analyze(ctx context.Context, sessionID, lifelineText, contextText string, topicsData []map[string]any) (*analyzeOutput, error) {
	topicsBlock := ""
	for _, td := range topicsData {
		topic, _ := td["topic"].(string)
		data, _ := td["data"].(string)
		if data == "" {
			data = "(无数据)"
		}
		topicsBlock += fmt.Sprintf("【%s】\n查询数据:\n%s\n\n", topic, data)
	}

	prompt := analyzePrompt
	if contextText != "" {
		prompt += "\n\n---\n分层新闻上下文:\n" + contextText
	}
	prompt += "\n\n话题演进脉络:\n" + lifelineText + "\n\n各主题实时数据:\n" + topicsBlock

	resp, err := o.airouter.Chat(ctx, airouter.ChatRequest{
		Capability:  capabilityAnalysis,
		Operation:   "data_enrichment.analyze",
		SessionID:   sessionID,
		Messages:    []airouter.Message{{Role: "user", Content: prompt}},
		Temperature: floatPtr(0.3),
		JSONMode:    true,
	})
	if err != nil {
		return nil, fmt.Errorf("analyze chat: %w", err)
	}

	parsed, err := ParseJSONResponse(resp.Content)
	if err != nil {
		return nil, fmt.Errorf("analyze parse: %w", err)
	}

	ea, _ := parsed["evolution_assessment"].(string)
	cc, _ := parsed["causal_chain"].(string)
	ov, _ := parsed["overall"].(string)

	sectorsRaw, _ := parsed["sectors"].([]any)
	sectors := make([]map[string]any, 0, len(sectorsRaw))
	for _, s := range sectorsRaw {
		if sm, ok := s.(map[string]any); ok {
			sectors = append(sectors, sm)
		}
	}

	return &analyzeOutput{
		evolutionAssessment: ea,
		sectors:             sectors,
		causalChain:         cc,
		overall:             ov,
	}, nil
}

// ── Review Judge ────────────────────────────────────────────────────────────

// runReviewJudge compares previous and current enrichment results.
// See design.md §4.3 and spec "分析认知循环 review judge".
func (o *OrchestratorService) runReviewJudge(ctx context.Context, sessionID, prevResultJSON, currResultJSON string) (*ReviewJudgeOutput, error) {
	prompt := fmt.Sprintf(reviewJudgePrompt, prevResultJSON, currResultJSON)

	resp, err := o.airouter.Chat(ctx, airouter.ChatRequest{
		Capability:  capabilityAnalysis,
		Operation:   "data_enrichment.review_judge",
		SessionID:   sessionID,
		Messages:    []airouter.Message{{Role: "user", Content: prompt}},
		Temperature: floatPtr(0.2),
		JSONMode:    true,
	})
	if err != nil {
		return nil, fmt.Errorf("review judge chat: %w", err)
	}

	parsed, err := ParseJSONResponse(resp.Content)
	if err != nil {
		return nil, fmt.Errorf("review judge parse: %w", err)
	}

	shouldReview, _ := parsed["should_review"].(bool)
	reason, _ := parsed["reason"].(string)
	deviationSummary, _ := parsed["deviation_summary"].(string)
	affectedContext, _ := parsed["affected_context"].(string)
	confidence, _ := parsed["confidence"].(float64)

	return &ReviewJudgeOutput{
		ShouldReview:     shouldReview,
		Reason:           reason,
		DeviationSummary: deviationSummary,
		AffectedContext:  affectedContext,
		Confidence:       confidence,
	}, nil
}

// ── Helpers ─────────────────────────────────────────────────────────────────

// generateSessionID creates a session ID like "data_enrichment_{topicID}_{uuid8}".
func generateSessionID(topicID uint) string {
	uuid8 := RandomHex(8)
	return fmt.Sprintf("data_enrichment_%d_%s", topicID, uuid8)
}

// RandomHex returns n random hex characters using crypto/rand.
// Falls back to math/rand if crypto/rand fails (session IDs do not require
// cryptographic strength). Never panics on nil index.
func RandomHex(n int) string {
	const chars = "0123456789abcdef"
	b := make([]byte, n)
	for i := range b {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			//nolint:gosec // intentional fallback — session IDs don't require crypto strength
			b[i] = chars[mrand.Intn(len(chars))]
		} else {
			b[i] = chars[idx.Int64()]
		}
	}
	return string(b)
}

func floatPtr(f float64) *float64 { return &f }

// dedupKeyFor creates a stable deduplication key from tool name + canonical args JSON.
// Go's json.Marshal on map[string]any sorts keys alphabetically, providing canonical form.
func dedupKeyFor(toolName string, args map[string]any) string {
	canonical, _ := json.Marshal(args)
	return toolName + "\x00" + string(canonical)
}

func argsToJSON(args map[string]any) string {
	b, _ := json.Marshal(args)
	return string(b)
}

// buildToolsDesc renders tool descriptions for the agent system prompt.
// Ported from PoC roles.py:_build_tools_desc.
func buildToolsDesc(registry *Registry) string {
	parts := make([]string, 0)
	for name, tool := range registry.Tools() {
		schemaJSON, _ := json.Marshal(tool.InputSchema)
		parts = append(parts, fmt.Sprintf("**%s**: %s\n  参数: %s", name, tool.Description, string(schemaJSON)))
	}
	return strings.Join(parts, "\n\n")
}

func prefix(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// ── Context layers ──────────────────────────────────────────────────────────

// contextSnapshot wraps metadata about the context layers read.
type contextSnapshot struct {
	Layers map[string]contextLayerInfo `json:"layers"`
}

type contextLayerInfo struct {
	AsOf string `json:"as_of"`
}

// inputSnapshotJSON is the shape stored in TopicEnrichmentResult.InputSnapshot.
type inputSnapshotJSON struct {
	ContextLayers map[string]contextLayerInfo `json:"context_layers"`
	LifelineRange lifelineRange               `json:"lifeline_range_section"`
	ReviewIDs     []uint                      `json:"review_ids"`
	WindowDays    int                         `json:"window_days"`
	ConfigLayers  []string                    `json:"config_context_layers"`
}

type lifelineRange struct {
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
}

// readContextLayers reads pre-computed context summaries from topic_lifeline_context.
// Returns concatenated text for prompts and a snapshot for traceability.
func (o *OrchestratorService) readContextLayers(ctx context.Context, topicID uint, layers []string) (string, *contextSnapshot, error) {
	contexts, err := o.repo.ListTopicLifelineContextsByTopic(ctx, topicID)
	if err != nil {
		return "", nil, fmt.Errorf("read context layers: %w", err)
	}

	// Index by granularity.
	byGran := make(map[string]*repository.TopicLifelineContext)
	for i := range contexts {
		byGran[contexts[i].Granularity] = &contexts[i]
	}

	// Filter by requested layers, skip ungenerated ones.
	snap := &contextSnapshot{Layers: make(map[string]contextLayerInfo)}
	var parts []string
	for _, layer := range layers {
		lc, ok := byGran[layer]
		if !ok {
			// Layer not generated yet — skip (don't fail).
			continue
		}
		parts = append(parts, fmt.Sprintf("## %s 层级上下文 (截止 %s)\n%s", layer, lc.AsOfDate.Format("2006-01-02"), lc.Content))
		snap.Layers[layer] = contextLayerInfo{AsOf: lc.AsOfDate.Format("2006-01-02")}
	}

	return strings.Join(parts, "\n\n"), snap, nil
}

// buildInputSnapshot creates the traceability snapshot for the result.
func buildInputSnapshot(snap *contextSnapshot, reviewIDs []uint, windowDays int, configLayers []string) *inputSnapshotJSON {
	return &inputSnapshotJSON{
		ContextLayers: snap.Layers,
		LifelineRange: lifelineRange{}, // populated when lifeline data has date range
		ReviewIDs:     reviewIDs,
		WindowDays:    windowDays,
		ConfigLayers:  configLayers,
	}
}

// ── helper ────────────────────────────────────────────────────────────────
