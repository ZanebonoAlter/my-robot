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
	capability        airouter.Capability
}

// NewOrchestratorService creates a new orchestrator with required dependencies.
func NewOrchestratorService(
	airouter AirRouter,
	repo *repository.Repository,
	lifelineReader LifelineReader,
	renderer *LifelineRenderer,
	toolRegistry *Registry,
	boardConfigReader BoardConfigReader,
	capability airouter.Capability,
) *OrchestratorService {
	return &OrchestratorService{
		airouter:          airouter,
		repo:              repo,
		lifelineReader:    lifelineReader,
		renderer:          renderer,
		toolRegistry:      toolRegistry,
		boardConfigReader: boardConfigReader,
		capability:        capability,
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
	allowedTools := cfg.AllowedTools
	agentResults := make([]*AgentLoopResult, 0, len(topics))
	topicsData := make([]map[string]any, 0, len(topics))
	for _, t := range topics {
		ar, err := o.runAgentLoop(ctx, sessionID, t.topic, lifelineText, allowedTools)
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
	sectorsObj := map[string]any{
		"position": analysis.position,
		"signals":  analysis.signals,
		"evidence": analysis.evidence,
	}
	if analysis.financialView != nil {
		sectorsObj["financial_view"] = analysis.financialView
	}
	sectorsJSON, _ := json.Marshal(sectorsObj)
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
		// Guard: skip review if prev result has old-format sectors (no position field).
		if extractPosition(prevResult.Sectors) == "" {
			logging.Infof("enrich topic %d: prev result has old-format sectors (no position), skip review judge", topicID)
		} else {
			prevJSON, _ := json.Marshal(map[string]any{
				"evolution_assessment": prevResult.EvolutionAssessment,
				"analysis":             json.RawMessage(prevResult.Sectors),
			})
			currJSON, _ := json.Marshal(map[string]any{
				"evolution_assessment": result.EvolutionAssessment,
				"analysis":             json.RawMessage(result.Sectors),
			})
			rj, rjErr := o.runReviewJudge(ctx, sessionID, string(prevJSON), string(currJSON))
			if rjErr == nil && rj != nil && rj.ShouldReview {
				conf := rj.Confidence
				var verdictJSON json.RawMessage
				if len(rj.PositionChange) > 0 {
					verdictJSON, _ = json.Marshal(rj.PositionChange)
				}
				review = &repository.TopicEnrichmentReview{
					PersistentTopicID: topicID,
					PrevResultID:      &prevResult.ID,
					CurrResultID:      result.ID,
					Verdict:           verdictJSON,
					DeviationSummary:  rj.ChangeSummary,
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
		Capability:  o.capability,
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
func (o *OrchestratorService) runAgentLoop(ctx context.Context, sessionID, topic, lifelineText string, allowedTools []string) (*AgentLoopResult, error) {
	toolsDesc := buildToolsDesc(o.toolRegistry, allowedTools)
	system := fmt.Sprintf(agentLoopSystemPrompt, toolsDesc, lifelineText)

	// Build allowedTools set for O(1) lookup.
	allowedSet := make(map[string]bool, len(allowedTools))
	for _, t := range allowedTools {
		allowedSet[t] = true
	}

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
			Capability:  o.capability,
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

			// Guard: reject tools not in allowedTools.
			if len(allowedSet) > 0 && !allowedSet[toolName] {
				errJSON := `{"error":"该工具当前不可用"}`
				tc := ToolCallRecord{
					Step:          step,
					Thought:       thought + " [被拦:工具不可用]",
					Tool:          toolName,
					Args:          args,
					ResultPreview: errJSON,
					ResultFull:    errJSON,
				}
				result.ToolCalls = append(result.ToolCalls, tc)
				historyLines = append(historyLines, fmt.Sprintf("第%d步: 调用 %s(%s) — 结果: %s", step, toolName, argsToJSON(args), errJSON))
				continue
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
// See design.md §3.3: topic evolution positioning.
type analyzeOutput struct {
	evolutionAssessment string
	position            string
	signals             []map[string]any
	evidence            []map[string]any
	financialView       *map[string]any // optional: nil when no financial data bound
	causalChain         string
	overall             string
}

// analyzePrompt is the LLM prompt for the analyst role.
// Design §3.3: topic evolution positioning (not financial direction prediction).
const analyzePrompt = `你是一位产业演进分析师,负责判断一个持久话题的最新进展在话题生命线中的定位。

下面提供:
1) 话题演进脉络 (lifeline):该话题从出现到最新的时间线节点
2) 分层新闻上下文:按 week/month/year/all 分层的历史背景
3) 各主题实时数据:agent 补充查到的信息 (可能是行情数据,也可能是一般搜索)

你的任务:基于这些信息,输出**话题演进定位** JSON。核心是判断话题处在什么演化阶段,而不是预测涨跌。

分析要求:

**1. position 定位 (必填,四选一):**
- reinforcing (强化):最新进展延续并加强了既有趋势,方向未变、力度加大
- turning (转折):最新进展表明趋势方向发生反转,或触发质变 (从缓和转向紧张、从上升转向下滑等)
- expanding (扩散):最新进展将影响传导到了新领域、新主体、新地域,话题范围在扩大
- fading (衰减):话题热度显著下降,新信号减弱或消失,不再有新的实质进展

**2. signals 信号列表 (必填):**
- lane:使用**持久话题泳道名** (如"美伊冲突""芯片制裁""AI监管",不要用粗版块大类如"能源""科技")
- signal:该泳道产生的具体信号描述 (一句话)
- mechanism:该信号通过什么传导/关联机制影响话题演进
- 每个 signal 对应一个泳道,不要重复

**3. evidence 证据链 (必填):**
- context_id:引用的上下文 ID
- period:引用的时间粒度 (week/month/year/all)
- quote:直接从 context 中摘录的原话 (不要改写)
- source_type:数据来源 (news=来自分层新闻上下文, tool=agent 查到的实时数据)
- tool_ref:source_type 是 tool 时,指向原始 tool_calls 的哪条 (source_type=news 时可为空)

**4. causal_chain 因果链 (必填):**
- 一句话描述从触发事件到最新进展的因果传导路径 (如 "产油国遭袭 → 油价上涨 → 各国释放储备 → 油价回落")

**5. overall 一句话总结 (必填):**
- 用一句话概括这次进展在整个话题演进中的意义

**6. financial_view 金融行情 (可选):**
仅当话题命中了绑定了金融数据源的版块、且有真实行情数据可以佐证时才输出。非金融话题直接省略此字段。
格式: {"sectors": [{"sector": "版块名", "direction": "up|down|flat|unknown", "supporting_data": "涨跌幅等支撑数据"}]}
行情数据只作为佐证,不作为 position 的主判断依据。

输出严格 JSON (不要 markdown 包裹,不要额外文字):
{
  "evolution_assessment": "一句话演进判断",
  "position": "reinforcing|turning|expanding|fading",
  "signals": [
    {"lane": "持久话题泳道名", "signal": "信号描述", "mechanism": "传导/关联机制"}
  ],
  "evidence": [
    {"context_id": "...", "period": "week|month|year|all", "quote": "引用原话", "source_type": "news|tool", "tool_ref": ""}
  ],
  "causal_chain": "因果链描述",
  "overall": "一句话总结"
}`

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
		Capability:  o.capability,
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
	pos, _ := parsed["position"].(string)
	cc, _ := parsed["causal_chain"].(string)
	ov, _ := parsed["overall"].(string)

	signalsRaw, _ := parsed["signals"].([]any)
	signals := make([]map[string]any, 0, len(signalsRaw))
	for _, s := range signalsRaw {
		if sm, ok := s.(map[string]any); ok {
			signals = append(signals, sm)
		}
	}

	evidenceRaw, _ := parsed["evidence"].([]any)
	evidence := make([]map[string]any, 0, len(evidenceRaw))
	for _, e := range evidenceRaw {
		if em, ok := e.(map[string]any); ok {
			evidence = append(evidence, em)
		}
	}

	var fv *map[string]any
	if fvRaw, ok := parsed["financial_view"].(map[string]any); ok {
		fv = &fvRaw
	}

	return &analyzeOutput{
		evolutionAssessment: ea,
		position:            pos,
		signals:             signals,
		evidence:            evidence,
		financialView:       fv,
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
		Capability:  o.capability,
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
	changeSummary, _ := parsed["change_summary"].(string)
	affectedContext, _ := parsed["affected_context"].(string)
	confidence, _ := parsed["confidence"].(float64)

	var positionChange map[string]any
	if pcRaw, ok := parsed["position_change"].(map[string]any); ok {
		positionChange = pcRaw
	}

	return &ReviewJudgeOutput{
		ShouldReview:    shouldReview,
		Reason:          reason,
		ChangeSummary:   changeSummary,
		AffectedContext: affectedContext,
		Confidence:      confidence,
		PositionChange:  positionChange,
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
// Only includes tools in allowedTools; returns empty string if allowedTools is empty.
// Ported from PoC roles.py:_build_tools_desc.
func buildToolsDesc(registry *Registry, allowedTools []string) string {
	if len(allowedTools) == 0 {
		return ""
	}
	allowedSet := make(map[string]bool, len(allowedTools))
	for _, t := range allowedTools {
		allowedSet[t] = true
	}

	parts := make([]string, 0)
	for name, tool := range registry.Tools() {
		if !allowedSet[name] {
			continue
		}
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
// Groups by granularity, selects the LATEST period for each layer (not all historical).
// Returns concatenated text for prompts and a snapshot for traceability.
func (o *OrchestratorService) readContextLayers(ctx context.Context, topicID uint, layers []string) (string, *contextSnapshot, error) {
	contexts, err := o.repo.ListTopicLifelineContextsByTopic(ctx, topicID)
	if err != nil {
		return "", nil, fmt.Errorf("read context layers: %w", err)
	}

	// Group by granularity, keep the MAX period per granularity (latest).
	byGran := make(map[string]*repository.TopicLifelineContext)
	for i := range contexts {
		lc := &contexts[i]
		existing, ok := byGran[lc.Granularity]
		if !ok || lc.Period > existing.Period {
			byGran[lc.Granularity] = lc
		}
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
		parts = append(parts, fmt.Sprintf("## %s 层级上下文 (period=%s, 截止 %s)\n%s", layer, lc.Period, lc.AsOfDate.Format("2006-01-02"), lc.Content))
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

// extractPosition extracts the "position" field from composite sectors JSON.
// Returns empty string if sectors is old-format or position is missing.
// Guards against review judge running on pre-transition old data. See design.md §4.3.
func extractPosition(sectorsJSON json.RawMessage) string {
	if len(sectorsJSON) == 0 {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(sectorsJSON, &m); err != nil {
		return ""
	}
	pos, _ := m["position"].(string)
	return pos
}
