package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"syntopica-backend/internal/dataenrichment/repository"
	"syntopica-backend/internal/platform/airouter"
	"syntopica-backend/internal/platform/logging"
)

// ── EnrichBoard：版块级分析编排入口（tasks 3.4 / M5）─────────────────────────
//
// 流程：enrichment_enabled 门槛 → D9 新鲜度门（先补齐滞后 lifeline）→ 态势卡
// 装配 → board_interpret（命题生成）→ 每研究方向 board agent loop → board
// analyze（层级递进论证）→ evidence lane 幽灵引用清洗 → result 写入（scope=board）
// → 自动 review judge（对比上一份 board 档快照；review 不回写 lifeline，红线）。

// BoardEnrichmentOutput mirrors EnrichmentOutput for the board scope.
type BoardEnrichmentOutput struct {
	Result    *repository.TopicEnrichmentResult `json:"result"`
	Review    *repository.TopicEnrichmentReview `json:"review,omitempty"`
	Freshness *FreshnessGateReport              `json:"freshness_report,omitempty"`
}

// boardResultPayload is the sectors jsonb shape for scope=board (D1):
// {scope, thesis, candidates, argument, depth, lane_refs}.
type boardResultPayload struct {
	Scope       string                `json:"scope"` // always "board"
	Form        string                `json:"form"`  // board|sparse
	Thesis      string                `json:"thesis"`
	Angle       string                `json:"angle,omitempty"`
	Candidates  []boardCandidate      `json:"candidates"`
	ChosenIndex int                   `json:"chosen_index"`
	Reason      string                `json:"reason,omitempty"`
	Argument    json.RawMessage       `json:"argument,omitempty"`
	Depth       Depth                 `json:"depth"`
	LaneRefs    []laneRef             `json:"lane_refs"`
	Interpret   *boardInterpretOutput `json:"interpret_meta,omitempty"` // degraded/all_sparse 透出
}

type laneRef struct {
	LaneID uint   `json:"lane_id"`
	Note   string `json:"note,omitempty"`
}

// BoardConfigResolver reads a board's enrichment config by board ID (the
// existing BoardConfigReader resolves by topic; EnrichBoard starts from the
// board).
type BoardConfigResolver interface {
	GetBoardConfigByBoardID(ctx context.Context, boardID uint) (*BoardEnrichmentConfig, error)
}

// ginBoardConfigResolver adapts via a raw query (avoids importing topicgraph;
// mirrors board_config_impl.go's raw-table style).
type dbBoardConfigResolver struct {
	db *gorm.DB
}

// NewDBBoardConfigResolver creates the production resolver.
func NewDBBoardConfigResolver(db *gorm.DB) BoardConfigResolver { return &dbBoardConfigResolver{db: db} }

func (r *dbBoardConfigResolver) GetBoardConfigByBoardID(ctx context.Context, boardID uint) (*BoardEnrichmentConfig, error) {
	var enabled bool
	var windowDays int
	if err := r.db.WithContext(ctx).
		Table("semantic_labels").
		Select("enrichment_enabled, COALESCE(window_days, 14)").
		Where("id = ? AND label_type = ?", boardID, "board").
		Row().Scan(&enabled, &windowDays); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return DefaultBoardConfig(), nil
		}
		return nil, fmt.Errorf("board config %d: %w", boardID, err)
	}
	cfg := DefaultBoardConfig()
	cfg.EnrichmentEnabled = enabled
	if windowDays > 0 {
		cfg.WindowDays = windowDays
	}
	return cfg, nil
}

// SetBoardConfigResolver wires the resolver post-construction (nil = default deny).
func (o *OrchestratorService) SetBoardConfigResolver(r BoardConfigResolver) { o.boardResolver = r }

// EnrichBoard runs the full board-level cycle-B flow (manual trigger only).
func (o *OrchestratorService) EnrichBoard(ctx context.Context, boardID uint) (*BoardEnrichmentOutput, error) {
	// 0. Board config gate (M5.1).
	if o.boardResolver == nil {
		return nil, fmt.Errorf("enrich board %d: board config resolver not wired", boardID)
	}
	cfg, err := o.boardResolver.GetBoardConfigByBoardID(ctx, boardID)
	if err != nil {
		return nil, fmt.Errorf("enrich board %d: board config: %w", boardID, err)
	}
	if !cfg.EnrichmentEnabled {
		return nil, fmt.Errorf("enrich board %d: enrichment not enabled for this board", boardID)
	}

	sessionID := generateBoardSessionID(boardID) // M5.6

	// 1. Active lanes of this board (also the lane-evidence whitelist).
	laneIDs, err := o.boardActiveLaneIDs(ctx, boardID)
	if err != nil {
		return nil, fmt.Errorf("enrich board %d: list lanes: %w", boardID, err)
	}

	// 2. D9 freshness gate — top up stale lifelines BEFORE assembly.
	freshness := o.ensureLaneFreshness(ctx, laneIDs)

	// 3. Situation cards.
	cards, err := o.assembleSituationCards(ctx, boardID)
	if err != nil {
		return nil, fmt.Errorf("enrich board %d: situation cards: %w", boardID, err)
	}
	if len(cards) == 0 {
		return nil, fmt.Errorf("enrich board %d: no active lanes to analyze", boardID)
	}
	cardsMD := RenderSituationCardsMarkdown(cards)
	activeLanes := make(map[uint]bool, len(cards))
	for _, c := range cards {
		activeLanes[c.LaneID] = true
	}
	allSparse := true
	for _, c := range cards {
		if c.FactsSource != "none" && c.Signals.SparseHistory < situationCardSparseDegraded {
			allSparse = false
			break
		}
	}

	// 4. Board-level applied review digest.
	reviewText, err := o.boardAppliedReviewDigest(ctx, boardID)
	if err != nil {
		// Non-fatal: reviews are advisory context.
		logging.Warnf("enrich board %d: list applied reviews: %v", boardID, err)
	}

	// 5. board_interpret（命题生成，M2 contract）.
	interp, err := o.boardInterpret(ctx, boardInterpretInput{
		SessionID: sessionID,
		CardsMD:   cardsMD,
		ReviewTxt: reviewText,
		AllSparse: allSparse,
		TopCard:   &cards[0],
	})
	if err != nil {
		return nil, fmt.Errorf("enrich board %d: board interpret: %w", boardID, err)
	}

	// 6. Sparse path: honest-decline result, no agent/analyze loops.
	if interp.Form == boardInterpretFormSparse {
		payload := boardResultPayload{
			Scope: "board", Form: boardInterpretFormSparse,
			Thesis: firstThesis(interp), Candidates: interp.Candidates,
			ChosenIndex: 0, Reason: interp.Reason,
			Depth: Depth{}, LaneRefs: []laneRef{},
			Interpret: interp,
		}
		return o.persistBoardResult(ctx, boardID, sessionID, payload, nil, freshness)
	}

	chosen := interp.Candidates[interp.ChosenIndex]

	// 7. Research directions: derive 2-3 from the situation cards' hot lanes
	// (mechanical — the interpret candidates already frame the directions).
	directions := o.boardResearchDirections(cards, chosen)

	// 8. Board agent loop per direction (same loop defenses, max_loops=6).
	allowedTools := o.buildAgentAllowedTools(cfg.AllowedTools)
	agentResults := make([]*AgentLoopResult, 0, len(directions))
	topicsData := make([]map[string]any, 0, len(directions))
	for _, dir := range directions {
		ar, aerr := o.runBoardAgentLoop(ctx, sessionID, dir, chosen.Thesis, cardsMD, allowedTools)
		if aerr != nil || ar.Error != "" {
			// Non-fatal per direction; record and continue.
			logging.Warnf("enrich board %d: direction %q loop error: %v / %s", boardID, dir, aerr, ar.Error)
		}
		agentResults = append(agentResults, ar)
		topicsData = append(topicsData, map[string]any{"topic": dir, "data": ar.FinalData})
	}

	// 9. Board analyze (层级递进论证骨架).
	analyzePromptStr := o.assembleBoardAnalyzePrompt(ctx, chosen.Thesis, chosen.Angle, cardsMD, topicsData)
	rawOut, err := o.boardAnalyzeCall(ctx, sessionID, analyzePromptStr, activeLanes)
	if err != nil {
		// M5.5: no partial result rows — analyze failure aborts before write.
		return nil, fmt.Errorf("enrich board %d: analyze: %w", boardID, err)
	}

	// 10. Compose payload.
	toolCallsJSON := boardToolCallsJSON(agentResults)
	payload := boardResultPayload{
		Scope: "board", Form: "board",
		Thesis: chosen.Thesis, Angle: chosen.Angle,
		Candidates: interp.Candidates, ChosenIndex: interp.ChosenIndex, Reason: interp.Reason,
		Argument: rawOut.Argument, Depth: rawOut.Depth, LaneRefs: rawOut.LaneRefs,
		Interpret: interp,
	}
	out, err := o.persistBoardResult(ctx, boardID, sessionID, payload, toolCallsJSON, freshness)
	if err != nil {
		return nil, err
	}
	_ = toolCallsJSON // used inside persistBoardResult via closure param
	return out, nil
}

// firstThesis safely extracts the sparse-path thesis.
func firstThesis(interp *boardInterpretOutput) string {
	if len(interp.Candidates) > 0 {
		return interp.Candidates[0].Thesis
	}
	return interp.Reason
}

// boardResearchDirections derives 2-3 research directions from cards + chosen
// thesis: the top full-detail lanes carry the hooks; directions mirror the
// methodology (机制/数据/对照).
func (o *OrchestratorService) boardResearchDirections(cards []LaneSituationCard, chosen boardCandidate) []string {
	dirs := make([]string, 0, 3)
	// Direction 1: thesis mechanism.
	dirs = append(dirs, fmt.Sprintf("命题机制：%s", chosen.Thesis))
	// Direction 2: top lanes' facts (hook validation).
	if len(cards) > 0 && cards[0].FactsDigest != "" {
		dirs = append(dirs, fmt.Sprintf("钩子核实：%s", cards[0].Label))
	}
	// Direction 3: cross-lane historical precedent.
	if len(cards) > 1 {
		dirs = append(dirs, fmt.Sprintf("跨泳道历史对照：%s 与 %s", cards[0].Label, cards[1].Label))
	}
	return dirs
}

// boardAnalyzeResult is the parsed board analyze output.
type boardAnalyzeResult struct {
	Argument json.RawMessage `json:"argument"`
	Depth    Depth           `json:"depth"`
	LaneRefs []laneRef       `json:"lane_refs"`
}

// boardAnalyzeCall runs the board analyze LLM call + parse + ghost-lane cleanup.
func (o *OrchestratorService) boardAnalyzeCall(ctx context.Context, sessionID, prompt string, activeLanes map[uint]bool) (*boardAnalyzeResult, error) {
	resp, err := o.airouter.Chat(ctx, airouter.ChatRequest{
		Capability: o.capability, Operation: "data_enrichment.analyze", SessionID: sessionID,
		Messages: []airouter.Message{{Role: "user", Content: prompt}}, JSONMode: true,
	})
	if err != nil {
		return nil, fmt.Errorf("board analyze chat: %w", err)
	}
	parsed, err := ParseJSONResponse(resp.Content)
	if err != nil {
		// One retry with the parse error fed back (mirrors single-lane analyze).
		retry := prompt + "\n\n---\n上次输出解析失败：" + err.Error() + "。请重新输出完整 JSON，不要 markdown 包裹。"
		resp2, err2 := o.airouter.Chat(ctx, airouter.ChatRequest{
			Capability: o.capability, Operation: "data_enrichment.analyze", SessionID: sessionID,
			Messages: []airouter.Message{{Role: "user", Content: retry}}, JSONMode: true,
		})
		if err2 != nil {
			return nil, fmt.Errorf("board analyze retry chat: %w", err2)
		}
		parsed, err = ParseJSONResponse(resp2.Content)
		if err != nil {
			return nil, fmt.Errorf("board analyze parse (after retry): %w", err)
		}
	}
	arg, _ := json.Marshal(parsed["argument"])
	out := &boardAnalyzeResult{Argument: json.RawMessage(arg)}
	// parseDepth takes the WHOLE payload and unwraps ["depth"] itself.
	out.Depth = parseDepth(parsed)
	out.Depth.EvidenceChain = sanitizeEvidenceChain(out.Depth.EvidenceChain, activeLanes)
	if lr, ok := parsed["lane_refs"].([]any); ok {
		for _, v := range lr {
			if vm, ok := v.(map[string]any); ok {
				id, ok := vm["lane_id"].(float64)
				if !ok || !activeLanes[uint(id)] {
					continue // ghost lane ref dropped
				}
				note, _ := vm["note"].(string)
				out.LaneRefs = append(out.LaneRefs, laneRef{LaneID: uint(id), Note: note})
			}
		}
	}
	if out.Depth.SystemReframe == "" || len(out.Depth.MechanismLayers) == 0 {
		return nil, fmt.Errorf("board analyze: depth block incomplete (system_reframe/mechanism_layers)")
	}
	return out, nil
}

// persistBoardResult writes the immutable board-scope result, then runs the
// review judge against the previous board-scope snapshot (M5.3/M5.4).
func (o *OrchestratorService) persistBoardResult(ctx context.Context, boardID uint, sessionID string, payload boardResultPayload, toolCallsJSON json.RawMessage, freshness *FreshnessGateReport) (*BoardEnrichmentOutput, error) {
	sectorsJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("enrich board %d: marshal payload: %w", boardID, err)
	}
	freshJSON, _ := json.Marshal(freshness)

	result := &repository.TopicEnrichmentResult{
		SemanticBoardID: repository.BoardIDPtr(boardID),
		AnalysisScope:   "board",
		Sectors:         sectorsJSON,
		ToolCalls:       toolCallsJSON,
		InputSnapshot:   freshJSON, // D9: freshness report in metadata
		SessionID:       sessionID,
	}
	if err := o.repo.CreateTopicEnrichmentResult(ctx, result); err != nil {
		return nil, fmt.Errorf("enrich board %d: save result: %w", boardID, err)
	}

	// Review judge vs previous board-scope result (scope-filtered query).
	var review *repository.TopicEnrichmentReview
	prevResult, prevErr := o.repo.GetPrevLatestBoardEnrichmentResult(ctx, boardID, result.ID)
	if prevErr != nil && !errors.Is(prevErr, gorm.ErrRecordNotFound) {
		logging.Warnf("enrich board %d: get prev result: %v", boardID, prevErr)
	}
	if prevErr == nil && prevResult != nil {
		prevJSON, _ := json.Marshal(map[string]any{"analysis": json.RawMessage(prevResult.Sectors)})
		currJSON, _ := json.Marshal(map[string]any{"analysis": json.RawMessage(result.Sectors)})
		rj, rjErr := o.runReviewJudge(ctx, sessionID, string(prevJSON), string(currJSON))
		if rjErr == nil && rj != nil && rj.ShouldReview {
			conf := rj.Confidence
			verdictJSON, _ := json.Marshal(map[string]any{
				"new_findings": rj.NewFindings, "overturned": rj.Overturned, "confidence_shift": rj.ConfidenceShift,
			})
			review = &repository.TopicEnrichmentReview{
				SemanticBoardID:  repository.BoardIDPtr(boardID),
				PrevResultID:     uintPtr(prevResult.ID),
				CurrResultID:     result.ID,
				Verdict:          verdictJSON,
				DeviationSummary: rj.Reason,
				AffectedContext:  rj.AffectedContext,
				Confidence:       &conf,
				Applied:          false,
				Source:           "llm_assisted",
			}
			if err := o.repo.CreateTopicEnrichmentReview(ctx, review); err != nil {
				return nil, fmt.Errorf("enrich board %d: save review: %w", boardID, err)
			}
		}
	}

	return &BoardEnrichmentOutput{Result: result, Review: review, Freshness: freshness}, nil
}

// boardActiveLaneIDs lists active lane IDs for the board.
// BoardEnrichmentEnabled is the cheap synchronous pre-flight used by the
// async trigger handler: disabled boards are rejected with a client error
// before a background job is scheduled (M6.1 semantics preserved).
func (o *OrchestratorService) BoardEnrichmentEnabled(ctx context.Context, boardID uint) error {
	if o.boardResolver == nil {
		return fmt.Errorf("enrich board %d: board config resolver not wired", boardID)
	}
	cfg, err := o.boardResolver.GetBoardConfigByBoardID(ctx, boardID)
	if err != nil {
		return fmt.Errorf("enrich board %d: board config: %w", boardID, err)
	}
	if !cfg.EnrichmentEnabled {
		return fmt.Errorf("enrich board %d: enrichment not enabled for this board", boardID)
	}
	return nil
}

func (o *OrchestratorService) boardActiveLaneIDs(ctx context.Context, boardID uint) ([]uint, error) {
	var ids []uint
	err := o.repo.DB().WithContext(ctx).
		Table("board_persistent_topics").
		Where("semantic_board_id = ? AND status = ?", boardID, "active").
		Order("id ASC").
		Pluck("id", &ids).Error
	if err != nil {
		return nil, err
	}
	return ids, nil
}

// boardAppliedReviewDigest renders applied board-scope reviews for interpret.
func (o *OrchestratorService) boardAppliedReviewDigest(ctx context.Context, boardID uint) (string, error) {
	reviews, err := o.repo.ListAppliedBoardEnrichmentReviews(ctx, boardID)
	if err != nil {
		return "", err
	}
	if len(reviews) == 0 {
		return "", nil
	}
	out := ""
	for _, r := range reviews {
		out += fmt.Sprintf("- [review #%d] %s\n", r.ID, r.DeviationSummary)
	}
	return out, nil
}

func boardToolCallsJSON(results []*AgentLoopResult) json.RawMessage {
	all := make([]ToolCallRecord, 0)
	for _, ar := range results {
		all = append(all, ar.ToolCalls...)
	}
	if len(all) == 0 {
		return nil
	}
	b, _ := json.Marshal(all)
	return b
}

// uintPtr returns a pointer to v (review FK columns are pointer-typed).
func uintPtr(v uint) *uint { return &v }
