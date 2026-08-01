package dataenrichment

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"syntopica-backend/internal/dataenrichment/service"
)

// dbBoardLister implements service.BoardLister against the semantic_labels +
// board_persistent_topics tables. Mirrors board_config_impl.go's raw-table
// query style (no model import) to stay consistent with the existing wiring.
type dbBoardLister struct {
	db *gorm.DB
}

// NewDBBoardLister creates a production BoardLister backed by gorm.DB.
func NewDBBoardLister(db *gorm.DB) service.BoardLister {
	return &dbBoardLister{db: db}
}

// ListBoards returns all active semantic boards with their active-lane count
// (active persistent topics per board). Ordered by board id for stable output.
func (l *dbBoardLister) ListBoards(ctx context.Context) ([]service.BoardSummary, error) {
	type row struct {
		ID          uint
		Label       string
		ActiveLanes int
	}
	var rows []row
	err := l.db.WithContext(ctx).
		Table("semantic_labels AS b").
		Select("b.id AS id, b.label AS label, COALESCE(t.active_lanes, 0) AS active_lanes").
		Joins("LEFT JOIN (SELECT semantic_board_id, COUNT(*) AS active_lanes FROM board_persistent_topics WHERE status = 'active' GROUP BY semantic_board_id) t ON t.semantic_board_id = b.id").
		Where("b.label_type = ? AND b.status = ?", "board", "active").
		Order("b.id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list boards: %w", err)
	}
	out := make([]service.BoardSummary, 0, len(rows))
	for _, r := range rows {
		out = append(out, service.BoardSummary{ID: r.ID, Name: r.Label, ActiveLanes: r.ActiveLanes})
	}
	return out, nil
}

// dbLaneLister implements service.LaneLister against board_persistent_topics.
type dbLaneLister struct {
	db *gorm.DB
}

// NewDBLaneLister creates a production LaneLister backed by gorm.DB.
func NewDBLaneLister(db *gorm.DB) service.LaneLister {
	return &dbLaneLister{db: db}
}

// ListLanes returns the persistent topics (lanes) for a board, ordered by
// consecutive hits DESC then hit_count DESC so the hottest lanes surface first.
func (l *dbLaneLister) ListLanes(ctx context.Context, boardID uint) ([]service.LaneSummary, error) {
	type row struct {
		ID              uint
		Label           string
		Status          string
		HitCount        int
		ConsecutiveHits int
	}
	var rows []row
	err := l.db.WithContext(ctx).
		Table("board_persistent_topics").
		Select("id, label, status, hit_count, consecutive_hits").
		Where("semantic_board_id = ?", boardID).
		Order("consecutive_hits DESC, hit_count DESC, id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list lanes for board %d: %w", boardID, err)
	}
	out := make([]service.LaneSummary, 0, len(rows))
	for _, r := range rows {
		out = append(out, service.LaneSummary{
			LaneID:          r.ID,
			Label:           r.Label,
			Status:          r.Status,
			HitCount:        r.HitCount,
			ConsecutiveHits: r.ConsecutiveHits,
		})
	}
	return out, nil
}
