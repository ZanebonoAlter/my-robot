package service_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"syntopica-backend/internal/dataenrichment/service"
	"syntopica-backend/internal/platform/airouter"
)

// ── Test helpers ────────────────────────────────────────────────────────────

// distillMockAirouter returns a canned ChatResult.
type distillMockAirouter struct {
	content     string
	err         error
	lastRequest airouter.ChatRequest
}

func (m *distillMockAirouter) Chat(ctx context.Context, req airouter.ChatRequest) (*airouter.ChatResult, error) {
	m.lastRequest = req
	if m.err != nil {
		return nil, m.err
	}
	return &airouter.ChatResult{Content: m.content}, nil
}

func newDistiller(ar service.AirRouter) *service.DebateDistiller {
	return service.NewDebateDistiller(ar, airouter.Capability("data_enrichment.debate_distill"))
}

// cannedDistillJSON builds a complete distill response JSON.
func cannedDistillJSON(agents []map[string]any, verdict, consensus string, up, flat, down int) string {
	type agent struct {
		Role    string `json:"role"`
		Stance  string `json:"stance"`
		Note    string `json:"note"`
		RawVote string `json:"raw_vote"`
	}
	list := make([]map[string]any, len(agents))
	for i, a := range agents {
		list[i] = a
	}
	full := map[string]any{
		"agents":    list,
		"verdict":   verdict,
		"consensus": consensus,
		"votes":     map[string]any{"up": up, "flat": flat, "down": down},
	}
	b, _ := json.Marshal(full)
	return string(b)
}

// simpleResearch builds a minimal research map.
func simpleResearch() map[string]any {
	return map[string]any{
		"sentiment_agent": "分析文本",
		"risk_agent":      "风险分析",
		"macro_agent":     "宏观分析",
		"tech_agent":      "技术分析",
		"fund_agent":      "资金分析",
		"flow_agent":      "资金流向",
	}
}

func simpleBattle() map[string]any {
	return map[string]any{
		"final_decision": "bullish",
		"final_votes": map[string]any{
			"sentiment_agent": "bullish",
			"risk_agent":      "bullish",
			"macro_agent":     "bullish",
			"tech_agent":      "bullish",
			"fund_agent":      "bearish",
			"flow_agent":      "bearish",
		},
	}
}

// ── Distill: full flow ──────────────────────────────────────────────────────

func TestDistill_ParsesAllFields(t *testing.T) {
	agents := []map[string]any{
		{"role": "sentiment_agent", "stance": "up", "note": "情绪积极", "raw_vote": "bullish"},
		{"role": "risk_agent", "stance": "down", "note": "风险积聚", "raw_vote": "bearish"},
		{"role": "macro_agent", "stance": "up", "note": "宏观偏多", "raw_vote": "bullish"},
		{"role": "tech_agent", "stance": "up", "note": "技术突破", "raw_vote": "bullish"},
		{"role": "fund_agent", "stance": "up", "note": "资金流入", "raw_vote": "bullish"},
		{"role": "flow_agent", "stance": "down", "note": "主力流出", "raw_vote": "bearish"},
	}
	content := cannedDistillJSON(agents, "up", "4/6", 4, 0, 2)

	mock := &distillMockAirouter{content: content}
	d := newDistiller(mock)

	result, err := d.Distill(context.Background(), "sess-1", simpleResearch(), simpleBattle())
	if err != nil {
		t.Fatalf("Distill: %v", err)
	}

	// Agents
	if len(result.Agents) != 6 {
		t.Fatalf("agents: want 6, got %d", len(result.Agents))
	}
	if result.Agents[0].Role != "sentiment_agent" {
		t.Fatalf("agent[0].role: want sentiment_agent, got %s", result.Agents[0].Role)
	}
	if result.Agents[0].Stance != "up" {
		t.Fatalf("agent[0].stance: want up, got %s", result.Agents[0].Stance)
	}
	if result.Agents[0].Note != "情绪积极" {
		t.Fatalf("agent[0].note: want 情绪积极, got %s", result.Agents[0].Note)
	}
	if result.Agents[0].RawVote != "bullish" {
		t.Fatalf("agent[0].raw_vote: want bullish, got %s", result.Agents[0].RawVote)
	}
	if result.Agents[1].Stance != "down" {
		t.Fatalf("agent[1].stance: want down, got %s", result.Agents[1].Stance)
	}
	if result.Agents[1].RawVote != "bearish" {
		t.Fatalf("agent[1].raw_vote: want bearish, got %s", result.Agents[1].RawVote)
	}

	// Verdict
	if result.Verdict != "up" {
		t.Fatalf("verdict: want up, got %s", result.Verdict)
	}

	// Consensus
	if result.Consensus != "4/6" {
		t.Fatalf("consensus: want 4/6, got %s", result.Consensus)
	}

	// Votes
	if result.Votes.Up != 4 {
		t.Fatalf("votes.up: want 4, got %d", result.Votes.Up)
	}
	if result.Votes.Flat != 0 {
		t.Fatalf("votes.flat: want 0, got %d", result.Votes.Flat)
	}
	if result.Votes.Down != 2 {
		t.Fatalf("votes.down: want 2, got %d", result.Votes.Down)
	}
}

// ── Stance: bullish + up text → up ─────────────────────────────────────────

func TestDistill_StanceBullishUp(t *testing.T) {
	agents := []map[string]any{
		{"role": "macro_agent", "stance": "up", "note": "宏观向好", "raw_vote": "bullish"},
	}
	content := cannedDistillJSON(agents, "up", "1/1", 1, 0, 0)

	mock := &distillMockAirouter{content: content}
	d := newDistiller(mock)
	result, err := d.Distill(context.Background(), "sess-1", simpleResearch(), simpleBattle())
	if err != nil {
		t.Fatalf("Distill: %v", err)
	}
	if result.Agents[0].Stance != "up" || result.Agents[0].RawVote != "bullish" {
		t.Fatalf("stance/raw_vote mismatch: stance=%s raw_vote=%s", result.Agents[0].Stance, result.Agents[0].RawVote)
	}
}

// ── Stance: bearish + down text → down ──────────────────────────────────────

func TestDistill_StanceBearishDown(t *testing.T) {
	agents := []map[string]any{
		{"role": "risk_agent", "stance": "down", "note": "风险高企", "raw_vote": "bearish"},
	}
	content := cannedDistillJSON(agents, "down", "1/1", 0, 0, 1)

	mock := &distillMockAirouter{content: content}
	d := newDistiller(mock)
	result, err := d.Distill(context.Background(), "sess-1", simpleResearch(), simpleBattle())
	if err != nil {
		t.Fatalf("Distill: %v", err)
	}
	if result.Agents[0].Stance != "down" || result.Agents[0].RawVote != "bearish" {
		t.Fatalf("stance/raw_vote mismatch: stance=%s raw_vote=%s", result.Agents[0].Stance, result.Agents[0].RawVote)
	}
}

// ── Stance: contradicting → flat ────────────────────────────────────────────

func TestDistill_StanceContradictionFlat(t *testing.T) {
	// bullish vote but LLM determined text is bearish → flat
	agents := []map[string]any{
		{"role": "fund_agent", "stance": "flat", "note": "投票偏多但数据差", "raw_vote": "bullish"},
	}
	content := cannedDistillJSON(agents, "flat", "0/1 分歧", 0, 1, 0)

	mock := &distillMockAirouter{content: content}
	d := newDistiller(mock)
	result, err := d.Distill(context.Background(), "sess-1", simpleResearch(), simpleBattle())
	if err != nil {
		t.Fatalf("Distill: %v", err)
	}
	if result.Agents[0].Stance != "flat" {
		t.Fatalf("contradiction should yield flat, got %s", result.Agents[0].Stance)
	}
}

// ── Verdict: majority up → up ──────────────────────────────────────────────

func TestDistill_VerdictUp(t *testing.T) {
	agents := []map[string]any{
		{"role": "a1", "stance": "up", "note": "c1", "raw_vote": "bullish"},
		{"role": "a2", "stance": "up", "note": "c2", "raw_vote": "bullish"},
		{"role": "a3", "stance": "up", "note": "c3", "raw_vote": "bullish"},
		{"role": "a4", "stance": "up", "note": "c4", "raw_vote": "bullish"},
		{"role": "a5", "stance": "down", "note": "c5", "raw_vote": "bearish"},
		{"role": "a6", "stance": "down", "note": "c6", "raw_vote": "bearish"},
	}
	content := cannedDistillJSON(agents, "up", "4/6", 4, 0, 2)

	mock := &distillMockAirouter{content: content}
	d := newDistiller(mock)
	result, err := d.Distill(context.Background(), "sess-1", simpleResearch(), simpleBattle())
	if err != nil {
		t.Fatalf("Distill: %v", err)
	}
	if result.Verdict != "up" {
		t.Fatalf("verdict: want up, got %s", result.Verdict)
	}
}

// ── Verdict: majority down → down ───────────────────────────────────────────

func TestDistill_VerdictDown(t *testing.T) {
	agents := []map[string]any{
		{"role": "a1", "stance": "down", "note": "c1", "raw_vote": "bearish"},
		{"role": "a2", "stance": "down", "note": "c2", "raw_vote": "bearish"},
		{"role": "a3", "stance": "down", "note": "c3", "raw_vote": "bearish"},
		{"role": "a4", "stance": "down", "note": "c4", "raw_vote": "bearish"},
		{"role": "a5", "stance": "up", "note": "c5", "raw_vote": "bullish"},
		{"role": "a6", "stance": "flat", "note": "c6", "raw_vote": "bullish"},
	}
	content := cannedDistillJSON(agents, "down", "4/6", 1, 1, 4)

	mock := &distillMockAirouter{content: content}
	d := newDistiller(mock)
	result, err := d.Distill(context.Background(), "sess-1", simpleResearch(), simpleBattle())
	if err != nil {
		t.Fatalf("Distill: %v", err)
	}
	if result.Verdict != "down" {
		t.Fatalf("verdict: want down, got %s", result.Verdict)
	}
}

// ── Verdict: 3:3 split → flat ──────────────────────────────────────────────

func TestDistill_VerdictSplitFlat(t *testing.T) {
	agents := []map[string]any{
		{"role": "a1", "stance": "up", "note": "c1", "raw_vote": "bullish"},
		{"role": "a2", "stance": "up", "note": "c2", "raw_vote": "bullish"},
		{"role": "a3", "stance": "up", "note": "c3", "raw_vote": "bullish"},
		{"role": "a4", "stance": "down", "note": "c4", "raw_vote": "bearish"},
		{"role": "a5", "stance": "down", "note": "c5", "raw_vote": "bearish"},
		{"role": "a6", "stance": "down", "note": "c6", "raw_vote": "bearish"},
	}
	content := cannedDistillJSON(agents, "flat", "3/6 分歧", 3, 0, 3)

	mock := &distillMockAirouter{content: content}
	d := newDistiller(mock)
	result, err := d.Distill(context.Background(), "sess-1", simpleResearch(), simpleBattle())
	if err != nil {
		t.Fatalf("Distill: %v", err)
	}
	if result.Verdict != "flat" {
		t.Fatalf("verdict: want flat for 3:3 split, got %s", result.Verdict)
	}
}

// ── Consensus: 4/6 format ──────────────────────────────────────────────────

func TestDistill_ConsensusMajority(t *testing.T) {
	agents := []map[string]any{
		{"role": "a1", "stance": "up", "note": "c1", "raw_vote": "bullish"},
		{"role": "a2", "stance": "up", "note": "c2", "raw_vote": "bullish"},
		{"role": "a3", "stance": "up", "note": "c3", "raw_vote": "bullish"},
		{"role": "a4", "stance": "up", "note": "c4", "raw_vote": "bullish"},
		{"role": "a5", "stance": "down", "note": "c5", "raw_vote": "bearish"},
		{"role": "a6", "stance": "down", "note": "c6", "raw_vote": "bearish"},
	}
	content := cannedDistillJSON(agents, "up", "4/6", 4, 0, 2)

	mock := &distillMockAirouter{content: content}
	d := newDistiller(mock)
	result, err := d.Distill(context.Background(), "sess-1", simpleResearch(), simpleBattle())
	if err != nil {
		t.Fatalf("Distill: %v", err)
	}
	if result.Consensus != "4/6" {
		t.Fatalf("consensus: want 4/6, got %s", result.Consensus)
	}
}

// ── Consensus: 2/6 分歧 format ──────────────────────────────────────────────

func TestDistill_ConsensusDivergence(t *testing.T) {
	agents := []map[string]any{
		{"role": "a1", "stance": "up", "note": "c1", "raw_vote": "bullish"},
		{"role": "a2", "stance": "up", "note": "c2", "raw_vote": "bullish"},
		{"role": "a3", "stance": "flat", "note": "c3", "raw_vote": "bullish"},
		{"role": "a4", "stance": "down", "note": "c4", "raw_vote": "bearish"},
		{"role": "a5", "stance": "down", "note": "c5", "raw_vote": "bearish"},
		{"role": "a6", "stance": "flat", "note": "c6", "raw_vote": "bearish"},
	}
	content := cannedDistillJSON(agents, "flat", "2/6 分歧", 2, 2, 2)

	mock := &distillMockAirouter{content: content}
	d := newDistiller(mock)
	result, err := d.Distill(context.Background(), "sess-1", simpleResearch(), simpleBattle())
	if err != nil {
		t.Fatalf("Distill: %v", err)
	}
	if !strings.Contains(result.Consensus, "分歧") {
		t.Fatalf("consensus should contain 分歧 for 2:2:2, got %s", result.Consensus)
	}
}

// ── Votes: three-stance count ───────────────────────────────────────────────

func TestDistill_VotesCount(t *testing.T) {
	agents := []map[string]any{
		{"role": "a1", "stance": "up", "note": "c1", "raw_vote": "bullish"},
		{"role": "a2", "stance": "up", "note": "c2", "raw_vote": "bullish"},
		{"role": "a3", "stance": "flat", "note": "c3", "raw_vote": "bullish"},
		{"role": "a4", "stance": "flat", "note": "c4", "raw_vote": "bearish"},
		{"role": "a5", "stance": "down", "note": "c5", "raw_vote": "bearish"},
		{"role": "a6", "stance": "down", "note": "c6", "raw_vote": "bearish"},
	}
	content := cannedDistillJSON(agents, "flat", "2/6 分歧", 2, 2, 2)

	mock := &distillMockAirouter{content: content}
	d := newDistiller(mock)
	result, err := d.Distill(context.Background(), "sess-1", simpleResearch(), simpleBattle())
	if err != nil {
		t.Fatalf("Distill: %v", err)
	}
	if result.Votes.Up != 2 {
		t.Fatalf("votes.up: want 2, got %d", result.Votes.Up)
	}
	if result.Votes.Flat != 2 {
		t.Fatalf("votes.flat: want 2, got %d", result.Votes.Flat)
	}
	if result.Votes.Down != 2 {
		t.Fatalf("votes.down: want 2, got %d", result.Votes.Down)
	}
}

// ── raw_vote: preserved ─────────────────────────────────────────────────────

func TestDistill_RawVotePreserved(t *testing.T) {
	agents := []map[string]any{
		{"role": "tech_agent", "stance": "up", "note": "突破上行", "raw_vote": "bullish"},
	}
	content := cannedDistillJSON(agents, "up", "1/1", 1, 0, 0)

	mock := &distillMockAirouter{content: content}
	d := newDistiller(mock)
	result, err := d.Distill(context.Background(), "sess-1", simpleResearch(), simpleBattle())
	if err != nil {
		t.Fatalf("Distill: %v", err)
	}
	if result.Agents[0].RawVote != "bullish" {
		t.Fatalf("raw_vote should be bullish, got %s", result.Agents[0].RawVote)
	}
}

// ── raw_vote: bearish preserved ─────────────────────────────────────────────

func TestDistill_RawVoteBearish(t *testing.T) {
	agents := []map[string]any{
		{"role": "risk_agent", "stance": "down", "note": "风险加大", "raw_vote": "bearish"},
	}
	content := cannedDistillJSON(agents, "down", "1/1", 0, 0, 1)

	mock := &distillMockAirouter{content: content}
	d := newDistiller(mock)
	result, err := d.Distill(context.Background(), "sess-1", simpleResearch(), simpleBattle())
	if err != nil {
		t.Fatalf("Distill: %v", err)
	}
	if result.Agents[0].RawVote != "bearish" {
		t.Fatalf("raw_vote should be bearish, got %s", result.Agents[0].RawVote)
	}
}

// ── Airouter error ──────────────────────────────────────────────────────────

func TestDistill_AirouterError(t *testing.T) {
	mock := &distillMockAirouter{err: context.DeadlineExceeded}
	d := newDistiller(mock)
	_, err := d.Distill(context.Background(), "sess-1", simpleResearch(), simpleBattle())
	if err == nil {
		t.Fatal("Distill should return error when airouter fails")
	}
}

// ── Distill: malformed JSON → error ─────────────────────────────────────────

func TestDistill_MalformedJSON(t *testing.T) {
	mock := &distillMockAirouter{content: "这不是JSON"}
	d := newDistiller(mock)
	_, err := d.Distill(context.Background(), "sess-1", simpleResearch(), simpleBattle())
	if err == nil {
		t.Fatal("Distill should return error on malformed JSON")
	}
}

// ── Distill: verify operation set in request ────────────────────────────────

func TestDistill_OperationSet(t *testing.T) {
	agents := []map[string]any{
		{"role": "a1", "stance": "up", "note": "c1", "raw_vote": "bullish"},
	}
	content := cannedDistillJSON(agents, "up", "1/1", 1, 0, 0)
	mock := &distillMockAirouter{content: content}
	d := newDistiller(mock)

	_, err := d.Distill(context.Background(), "sess-1", simpleResearch(), simpleBattle())
	if err != nil {
		t.Fatalf("Distill: %v", err)
	}

	if mock.lastRequest.Operation != "data_enrichment.debate_distill" {
		t.Fatalf("operation: want data_enrichment.debate_distill, got %s", mock.lastRequest.Operation)
	}
	if mock.lastRequest.SessionID != "sess-1" {
		t.Fatalf("session_id: want sess-1, got %s", mock.lastRequest.SessionID)
	}
	if !mock.lastRequest.JSONMode {
		t.Fatal("JSONMode should be true")
	}
	if mock.lastRequest.Temperature == nil || *mock.lastRequest.Temperature != 0.1 {
		t.Fatal("temperature should be 0.1")
	}
}
