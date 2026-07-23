package service_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"syntopica-backend/internal/dataenrichment/service"
)

// ── Exploration data-source mocks ──────────────────────────────────────────

type mockBoardLister struct {
	boards []service.BoardSummary
	err    error
}

func (m *mockBoardLister) ListBoards(_ context.Context) ([]service.BoardSummary, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.boards, nil
}

type mockLaneLister struct {
	lanes map[uint][]service.LaneSummary
	err   error
	last  uint
}

func (m *mockLaneLister) ListLanes(_ context.Context, boardID uint) ([]service.LaneSummary, error) {
	m.last = boardID
	if m.err != nil {
		return nil, m.err
	}
	return m.lanes[boardID], nil
}

type mockLaneDetailRenderer struct {
	text       string
	err        error
	lastLaneID uint
	lastWindow int
}

func (m *mockLaneDetailRenderer) RenderLaneDetail(_ context.Context, laneID uint, windowDays int) (string, error) {
	m.lastLaneID = laneID
	m.lastWindow = windowDays
	if m.err != nil {
		return "", m.err
	}
	return m.text, nil
}

func newExplorationRegistry(bl service.BoardLister, ll service.LaneLister, ldr service.LaneDetailRenderer) *service.Registry {
	return service.NewRegistry(&nilFetcher{},
		service.WithBoardLister(bl),
		service.WithLaneLister(ll),
		service.WithLaneDetailRenderer(ldr),
	)
}

func TestListBoards_ReturnsBoardPanorama(t *testing.T) {
	bl := &mockBoardLister{boards: []service.BoardSummary{
		{ID: 1, Name: "半导体产业链", ActiveLanes: 5},
		{ID: 2, Name: "新能源车", ActiveLanes: 3},
	}}
	registry := newExplorationRegistry(bl, nil, nil)

	output, err := registry.Execute(context.Background(), "list_boards", nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("parse output: %v (raw: %s)", err, output)
	}
	if count, _ := result["board_count"].(float64); int(count) != 2 {
		t.Fatalf("board_count = %v, want 2", result["board_count"])
	}
	boards, _ := result["boards"].([]any)
	if len(boards) != 2 {
		t.Fatalf("boards length = %d, want 2", len(boards))
	}
	first := boards[0].(map[string]any)
	if first["name"] != "半导体产业链" {
		t.Fatalf("first board name = %v", first["name"])
	}
}

func TestListLanes_ByBoardID(t *testing.T) {
	ll := &mockLaneLister{lanes: map[uint][]service.LaneSummary{
		7: {
			{LaneID: 101, Label: "光刻机突破", Status: "active", HitCount: 9, ConsecutiveHits: 4},
			{LaneID: 102, Label: "存储涨价", Status: "candidate", HitCount: 2, ConsecutiveHits: 1},
		},
	}}
	registry := newExplorationRegistry(nil, ll, nil)

	output, err := registry.Execute(context.Background(), "list_lanes", map[string]any{"board_id": float64(7)})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if ll.last != 7 {
		t.Fatalf("ListLanes called with boardID %d, want 7", ll.last)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("parse output: %v (raw: %s)", err, output)
	}
	if bid, _ := result["board_id"].(float64); int(bid) != 7 {
		t.Fatalf("board_id = %v, want 7", result["board_id"])
	}
	lanes, _ := result["lanes"].([]any)
	if len(lanes) != 2 {
		t.Fatalf("lanes length = %d, want 2", len(lanes))
	}
}

func TestListLanes_EmptyBoard_Hint(t *testing.T) {
	ll := &mockLaneLister{lanes: map[uint][]service.LaneSummary{}}
	registry := newExplorationRegistry(nil, ll, nil)

	output, _ := registry.Execute(context.Background(), "list_lanes", map[string]any{"board_id": float64(99)})
	if !strings.Contains(output, "没有持久话题泳道") {
		t.Fatalf("expected empty-board hint, got: %s", output)
	}
}

func TestListLanes_InvalidBoardID_ParamError(t *testing.T) {
	registry := newExplorationRegistry(nil, &mockLaneLister{}, nil)
	output, _ := registry.Execute(context.Background(), "list_lanes", map[string]any{"board_id": float64(0)})
	if !strings.Contains(output, "参数错误") {
		t.Fatalf("expected param error for board_id=0, got: %s", output)
	}
}

func TestGetLaneDetail_ReusesRenderer(t *testing.T) {
	ldr := &mockLaneDetailRenderer{text: "# 持久话题演进脉络\n## 话题本体\n- 名称: 光刻机"}
	registry := newExplorationRegistry(nil, nil, ldr)

	output, err := registry.Execute(context.Background(), "get_lane_detail", map[string]any{
		"lane_id":     float64(42),
		"window_days": float64(30),
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if ldr.lastLaneID != 42 {
		t.Fatalf("RenderLaneDetail laneID = %d, want 42", ldr.lastLaneID)
	}
	if ldr.lastWindow != 30 {
		t.Fatalf("RenderLaneDetail windowDays = %d, want 30", ldr.lastWindow)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("parse output: %v (raw: %s)", err, output)
	}
	if !strings.Contains(result["detail"].(string), "持久话题演进脉络") {
		t.Fatalf("detail should be the renderer output, got: %v", result["detail"])
	}
}

func TestGetLaneDetail_DefaultWindow14(t *testing.T) {
	ldr := &mockLaneDetailRenderer{text: "x"}
	registry := newExplorationRegistry(nil, nil, ldr)
	_, _ = registry.Execute(context.Background(), "get_lane_detail", map[string]any{"lane_id": float64(1)})
	if ldr.lastWindow != 14 {
		t.Fatalf("default windowDays = %d, want 14", ldr.lastWindow)
	}
}

// TestGetLaneDetail_AdapterReusesRenderLifelineForAgent verifies the production
// adapter (NewRendererLaneDetailAdapter) delegates to the SAME renderer as cycle A,
// not a parallel format. This is the "复用 renderer" red line.
func TestGetLaneDetail_AdapterReusesRenderLifelineForAgent(t *testing.T) {
	reader := &orchMockLifelineReader{data: service.SectionTimelineData{
		Topic: service.TopicBrief{ID: 5, Label: "测试话题", Status: "active"},
	}}
	renderer := service.NewLifelineRenderer()
	adapter := service.NewRendererLaneDetailAdapter(reader, renderer)

	out, err := adapter.RenderLaneDetail(context.Background(), 5, 14)
	if err != nil {
		t.Fatalf("RenderLaneDetail: %v", err)
	}
	// Must match the RenderLifelineForAgent format header exactly.
	if !strings.HasPrefix(out, "# 持久话题演进脉络") {
		t.Fatalf("adapter output should equal RenderLifelineForAgent format, got: %s", out)
	}
}

// TestToolsForExploration verifies the exploration entry points + web_search are
// registered (change tasks 3.6). They must be callable even without exploration
// data sources injected (graceful degradation).
func TestToolsForExploration(t *testing.T) {
	registry := service.NewRegistry(&nilFetcher{}) // no exploration deps
	tools := registry.Tools()
	required := []string{"list_boards", "list_lanes", "get_lane_detail", "web_search"}
	for _, name := range required {
		if tools[name] == nil {
			t.Errorf("tool %q should be registered", name)
		}
		if tools[name].Execute == nil {
			t.Errorf("tool %q should have an Execute func", name)
		}
	}
}
