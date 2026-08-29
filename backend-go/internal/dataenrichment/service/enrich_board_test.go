package service_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"syntopica-backend/internal/dataenrichment/repository"
	"syntopica-backend/internal/dataenrichment/service"
)

// ── EnrichBoard 编排（tasks 3.4 / M5）——SQLite 全链路（mock LLM）──────────────

// mockBoardResolver: canned board config by board ID.
type mockBoardResolver struct {
	enabled bool
}

func (m *mockBoardResolver) GetBoardConfigByBoardID(ctx context.Context, boardID uint) (*service.BoardEnrichmentConfig, error) {
	cfg := service.DefaultBoardConfig()
	cfg.EnrichmentEnabled = m.enabled
	return cfg, nil
}

func newEnrichBoardOrch(t *testing.T, enabled bool) (*service.OrchestratorService, *mockAirRouter, *repository.Repository) {
	t.Helper()
	repo := setupOrchTestDB(t)
	router := newMockAirRouter()
	orch := service.NewOrchestratorService(
		router, repo, &orchMockLifelineReader{}, service.NewLifelineRenderer(),
		service.NewRegistry(&nilFetcher{}), &mockBoardConfigReader{cfg: service.DefaultBoardConfig()}, testCap,
	)
	orch.SetBoardConfigResolver(&mockBoardResolver{enabled: enabled})
	orch.SetFreshnessRefresher(&noopFreshnessRefresher{})
	return orch, router, repo
}

type noopFreshnessRefresher struct{}

func (noopFreshnessRefresher) RefreshGranularity(ctx context.Context, topicID uint, granularity string, now time.Time) error {
	return nil
}

func (noopFreshnessRefresher) RefreshPeriod(ctx context.Context, topicID uint, granularity, period string, now time.Time) error {
	return nil
}

func (noopFreshnessRefresher) SectionDates(ctx context.Context, topicID uint) ([]time.Time, error) {
	return nil, nil
}

func seedBoardLane(t *testing.T, db *gorm.DB, id uint, boardID uint, label string) {
	t.Helper()
	mustExec(t, db, fmt.Sprintf(
		`INSERT INTO board_persistent_topics (id, semantic_board_id, label, status, source, first_seen_date, last_seen_date, hit_count, consecutive_hits, created_at, updated_at)
		 VALUES (%d, %d, '%s', 'active', 'auto', '2026-08-01', '2026-08-26', 20, 8, datetime('now'), datetime('now'))`,
		id, boardID, label))
}

// boardLLMScript wires the full happy-path mock responses in call order:
// 1 board_interpret → N agent loops (finish immediately) → 1 analyze → (2nd run) + 1 review_judge.
func boardLLMScript(router *mockAirRouter, agentLoops int) {
	router.addResponse(`{"form":"board","candidates":[
		{"thesis":"板块命题甲","hook":"钩子甲","angle":"概念重命名"},
		{"thesis":"板块命题乙","hook":"钩子乙","angle":"机制保质期"}],
		"chosen_index":0,"reason":"首选"}`)
	for i := 0; i < agentLoops; i++ {
		router.addResponse(fmt.Sprintf(`{"action":"finish","thought":"done","summary":"方向%d素材"}`, i))
	}
	router.addResponse(`{"scope":"board","form":"board","thesis":"板块命题甲","lens":"概念重命名",
		"candidates":[],"argument":{"intro":"开篇","layers":[{"layer":"机制一","deep_logic":"逻辑","basis":"依据"}],"boundary":"边界","conclusion":{"cert":"medium","judgment":"结论"}},
		"depth":{"system_reframe":"系统重定位","mechanism_layers":[{"layer":"机制一","deep_logic":"逻辑","basis":"依据"}],
			"historical_analogy":{"case":"案例","mechanism":"机制","diff":"差异"},"regime_shift":null,"boundary":"边界",
			"evidence_chain":[{"source_type":"lane","ref":"901","note":"泳道论据","kind":"quote"},{"source_type":"web","url":"https://x.example","quote":"原文","institution":"机构","date":"2026-08-20"}]},
		"lane_refs":[{"lane_id":901,"note":"贡献论据"}]}`)
}

// M5.1 enrichment_enabled=false → 拒绝。
func TestEnrichBoard_DisabledBoardRejected(t *testing.T) {
	orch, router, repo := newEnrichBoardOrch(t, false)
	seedBoardLane(t, repo.DB(), 901, 8801, "泳道甲")

	_, err := orch.EnrichBoard(context.Background(), 8801)
	if err == nil || !strings.Contains(err.Error(), "not enabled") {
		t.Fatalf("want not-enabled rejection, got %v", err)
	}
	if len(router.Calls) != 0 {
		t.Fatal("no LLM calls before the gate")
	}
}

// M5.2 正常触发 → 五字段齐备 + lane 幽灵引用清洗 + M5.6 SessionID。
func TestEnrichBoard_HappyPath(t *testing.T) {
	orch, router, repo := newEnrichBoardOrch(t, true)
	db := repo.DB()
	seedBoardLane(t, db, 901, 8801, "泳道甲")
	seedBoardLane(t, db, 902, 8801, "泳道乙")
	// Week lifeline on the top lane → its digest is non-empty → the
	// hook-verification direction fires → 3 directions total.
	seedWeekLifeline(t, db, 901, "2026-W34", "周内容", time.Now())
	boardLLMScript(router, 3) // 3 directions (命题机制+钩子核实+跨泳道对照)

	out, err := orch.EnrichBoard(context.Background(), 8801)
	if err != nil {
		t.Fatalf("EnrichBoard: %v", err)
	}
	res := out.Result
	if res.AnalysisScope != "board" {
		t.Fatalf("scope: want board, got %s", res.AnalysisScope)
	}
	if res.PersistentTopicID != nil {
		t.Fatal("board result must have NULL topic id")
	}
	if !strings.HasPrefix(res.SessionID, "data_enrichment_board_8801_") {
		t.Fatalf("session id: %s", res.SessionID)
	}

	var payload map[string]any
	if err := json.Unmarshal(res.Sectors, &payload); err != nil {
		t.Fatalf("payload: %v", err)
	}
	for _, f := range []string{"thesis", "candidates", "argument", "depth", "lane_refs"} {
		if _, ok := payload[f]; !ok {
			t.Fatalf("sectors missing %q", f)
		}
	}
	if payload["thesis"] != "板块命题甲" {
		t.Fatalf("thesis: %v", payload["thesis"])
	}
	// Ghost lane (999) dropped from evidence; real lane (901) kept.
	depth := payload["depth"].(map[string]any)
	ec := depth["evidence_chain"].([]any)
	laneSeen := false
	for _, e := range ec {
		em := e.(map[string]any)
		if em["source_type"] == "lane" {
			if em["ref"] == "999" {
				t.Fatal("ghost lane evidence must be dropped")
			}
			laneSeen = true
		}
	}
	if !laneSeen {
		t.Fatal("legit lane evidence must survive")
	}
	// lane_refs ghost filtered too.
	lrs := payload["lane_refs"].([]any)
	if len(lrs) != 1 || lrs[0].(map[string]any)["lane_id"].(float64) != 901 {
		t.Fatalf("lane_refs wrong: %v", lrs)
	}
}

// M5.3/M5.4 两连触发：第二份独立快照 + 自动 review（对比 board 档），review 后 lifeline 无写入。
func TestEnrichBoard_SecondRunReviews(t *testing.T) {
	orch, router, repo := newEnrichBoardOrch(t, true)
	db := repo.DB()
	seedBoardLane(t, db, 901, 8801, "泳道甲")
	seedWeekLifeline(t, db, 901, "2026-W34", "周内容", time.Now())

	// First run: interpret + 2 agents (1 lane → 命题机制+钩子核实) + analyze.
	boardLLMScript(router, 2)
	out1, err := orch.EnrichBoard(context.Background(), 8801)
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	if out1.Review != nil {
		t.Fatal("first run must have no review (no prev)")
	}

	// Snapshot lifeline rows before run 2 (M5.4 red line).
	var lifelineBefore int
	mustScanCount(t, db, &lifelineBefore, "SELECT COUNT(*) FROM topic_lifeline_context")

	// Second run: same script + review judge response.
	boardLLMScript(router, 2)
	router.addResponse(`{"should_review":true,"reason":"新发现","new_findings":["发现"],"overturned":[],"confidence_shift":-0.1,"affected_context":"week","confidence":0.7}`)
	out2, err := orch.EnrichBoard(context.Background(), 8801)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if out2.Review == nil {
		t.Fatal("second run must auto-review")
	}
	if out2.Review.SemanticBoardID == nil || *out2.Review.SemanticBoardID != 8801 {
		t.Fatal("review must be board-scoped")
	}
	if out2.Result.ID == out1.Result.ID {
		t.Fatal("second result must be an independent snapshot")
	}
	// Review compares against the previous BOARD-scope result.
	if out2.Review.PrevResultID == nil || *out2.Review.PrevResultID != out1.Result.ID {
		t.Fatalf("review prev must point at first board result, got %v", out2.Review.PrevResultID)
	}

	// M5.4 red line: review never wrote lifeline.
	var lifelineAfter int
	mustScanCount(t, db, &lifelineAfter, "SELECT COUNT(*) FROM topic_lifeline_context")
	if lifelineAfter != lifelineBefore {
		t.Fatalf("lifeline rows changed: %d → %d", lifelineBefore, lifelineAfter)
	}
}

// M5.5 analyze 失败 → 无半成品 result。
func TestEnrichBoard_AnalyzeFailureNoPartialRow(t *testing.T) {
	orch, router, repo := newEnrichBoardOrch(t, true)
	db := repo.DB()
	seedBoardLane(t, db, 901, 8801, "泳道甲")

	// interpret OK, agents finish, analyze garbage twice → error.
	router.addResponse(`{"form":"board","candidates":[{"thesis":"T","hook":"H","angle":"A"}],"chosen_index":0,"reason":"r"}`)
	for i := 0; i < 3; i++ {
		router.addResponse(`{"action":"finish","thought":"d","summary":"s"}`)
	}
	router.addResponse(`garbage`)
	router.addResponse(`garbage2`)

	var before int
	mustScanCount(t, db, &before, "SELECT COUNT(*) FROM topic_enrichment_result WHERE analysis_scope = 'board'")
	if _, err := orch.EnrichBoard(context.Background(), 8801); err == nil {
		t.Fatal("analyze failure must abort")
	}
	var after int
	mustScanCount(t, db, &after, "SELECT COUNT(*) FROM topic_enrichment_result WHERE analysis_scope = 'board'")
	if after != before {
		t.Fatalf("partial result leaked: %d → %d", before, after)
	}
}

// sparse 板块 → 诚实降级路径（无 agent/analyze 调用）。
func TestEnrichBoard_SparseHonestDecline(t *testing.T) {
	orch, router, repo := newEnrichBoardOrch(t, true)
	seedBoardLane(t, repo.DB(), 901, 8801, "泳道甲") // no lifeline/sections → FactsSource none → allSparse

	router.addResponse(`{"form":"sparse","reason":"全板块素材稀薄","candidates":[{"thesis":"素材不足，无法形成结构命题","hook":"","angle":""}]}`)

	out, err := orch.EnrichBoard(context.Background(), 8801)
	if err != nil {
		t.Fatalf("sparse path: %v", err)
	}
	var payload map[string]any
	_ = json.Unmarshal(out.Result.Sectors, &payload)
	if payload["form"] != "sparse" {
		t.Fatalf("form: want sparse, got %v", payload["form"])
	}
	// No agent/analyze calls: only the interpret call happened.
	if len(router.Calls) != 1 {
		t.Fatalf("sparse path must skip agent/analyze, LLM calls=%d", len(router.Calls))
	}
}

func mustScanCount(t *testing.T, db *gorm.DB, dst *int, sql string) {
	t.Helper()
	if err := db.Raw(sql).Scan(dst).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
}
