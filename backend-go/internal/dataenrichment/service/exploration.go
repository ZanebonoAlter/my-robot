package service

import (
	"context"
	"fmt"
)

// ── Exploration data-source interfaces ──────────────────────────────────────
//
// These let the exploration tools (list_boards / list_lanes / get_lane_detail)
// reach board + persistent-topic data without the service package depending on
// gorm or the dataenrichment/topicgraph packages. DB-backed implementations
// live in the dataenrichment package (see board_listers_impl.go), mirroring how
// board_config_impl.go implements BoardConfigReader. When an implementation is
// not injected, the corresponding tool returns a graceful error JSON.

// BoardSummary is one board panorama entry for list_boards.
type BoardSummary struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	ActiveLanes int    `json:"active_lanes"` // count of active persistent topics
}

// LaneSummary is one persistent-topic (lane) entry for list_lanes.
// LaneID is a board_persistent_topics.id (the same ID used by get_lane_detail).
type LaneSummary struct {
	LaneID          uint   `json:"lane_id"`
	Label           string `json:"label"`
	Status          string `json:"status"`
	HitCount        int    `json:"hit_count"`
	ConsecutiveHits int    `json:"consecutive_hits"`
}

// BoardLister lists semantic boards (版块) for the panorama tool.
type BoardLister interface {
	ListBoards(ctx context.Context) ([]BoardSummary, error)
}

// LaneLister lists persistent topics (lanes) under one board.
type LaneLister interface {
	ListLanes(ctx context.Context, boardID uint) ([]LaneSummary, error)
}

// LaneDetailRenderer renders a lane's lifeline as agent-readable text.
// The default implementation (NewRendererLaneDetailAdapter) reuses
// LifelineRenderer.RenderLifelineForAgent — the SAME renderer/format as cycle A
// (lifeline_context), so the agent sees a consistent view. windowDays maps 1:1
// to the renderer's day window (board default 14).
type LaneDetailRenderer interface {
	RenderLaneDetail(ctx context.Context, laneID uint, windowDays int) (string, error)
}

// rendererLaneDetailAdapter adapts a (LifelineReader, *LifelineRenderer) pair to
// LaneDetailRenderer by delegating to RenderLifelineForAgent.
type rendererLaneDetailAdapter struct {
	reader   LifelineReader
	renderer *LifelineRenderer
}

// NewRendererLaneDetailAdapter wraps a LifelineReader + LifelineRenderer so the
// get_lane_detail tool reuses the cycle-A lifeline rendering verbatim.
func NewRendererLaneDetailAdapter(reader LifelineReader, renderer *LifelineRenderer) LaneDetailRenderer {
	return &rendererLaneDetailAdapter{reader: reader, renderer: renderer}
}

func (a *rendererLaneDetailAdapter) RenderLaneDetail(_ context.Context, laneID uint, windowDays int) (string, error) {
	if a.renderer == nil || a.reader == nil {
		return "", fmt.Errorf("lane detail renderer not configured")
	}
	if windowDays <= 0 {
		windowDays = 14
	}
	return a.renderer.RenderLifelineForAgent(a.reader, laneID, windowDays)
}

// ── Tool: list_boards ───────────────────────────────────────────────────────

func (r *Registry) executeListBoards(ctx context.Context, _ map[string]any) (string, error) {
	if r.boardLister == nil {
		return jsonError("list_boards 未配置数据源"), nil
	}
	boards, err := r.boardLister.ListBoards(ctx)
	if err != nil {
		return jsonError("加载版块失败: " + err.Error()), nil
	}
	out := map[string]any{
		"board_count": len(boards),
		"boards":      boards,
		"note":        "每个版块含若干持久话题泳道,用 list_lanes(board_id) 展开",
	}
	b, _ := jsonMarshal(out)
	return string(b), nil
}

// ── Tool: list_lanes ────────────────────────────────────────────────────────

func (r *Registry) executeListLanes(ctx context.Context, args map[string]any) (string, error) {
	boardID, ok := toUint(args["board_id"])
	if !ok || boardID == 0 {
		return jsonError("参数错误: board_id 必须为正整数"), nil
	}
	if r.laneLister == nil {
		return jsonError("list_lanes 未配置数据源"), nil
	}
	lanes, err := r.laneLister.ListLanes(ctx, boardID)
	if err != nil {
		return jsonError("加载泳道失败: " + err.Error()), nil
	}
	if len(lanes) == 0 {
		return jsonError(fmt.Sprintf("版块 %d 下没有持久话题泳道", boardID)), nil
	}
	b, _ := jsonMarshal(map[string]any{
		"board_id":   boardID,
		"lane_count": len(lanes),
		"lanes":      lanes,
		"note":       "用 get_lane_detail(lane_id, window_days) 查看泳道详情",
	})
	return string(b), nil
}

// ── Tool: get_lane_detail ───────────────────────────────────────────────────

func (r *Registry) executeGetLaneDetail(ctx context.Context, args map[string]any) (string, error) {
	laneID, ok := toUint(args["lane_id"])
	if !ok || laneID == 0 {
		return jsonError("参数错误: lane_id 必须为正整数"), nil
	}
	windowDays := 14
	if d, ok := toUint(args["window_days"]); ok && d > 0 {
		windowDays = int(d)
	}
	if r.laneDetailRenderer == nil {
		return jsonError("get_lane_detail 未配置数据源"), nil
	}
	text, err := r.laneDetailRenderer.RenderLaneDetail(ctx, laneID, windowDays)
	if err != nil {
		return jsonError("渲染泳道详情失败: " + err.Error()), nil
	}
	b, _ := jsonMarshal(map[string]any{
		"lane_id":     laneID,
		"window_days": windowDays,
		"detail":      text,
	})
	return string(b), nil
}
