package repository

import (
	"fmt"

	"gorm.io/gorm"
)

// CreateWatch creates a new BoardTopicWatch for a semantic board.
// The new watch starts with status=active by default.
func (r *TopicGraphRepository) CreateWatch(semanticBoardID uint, label string) (*BoardTopicWatch, error) {
	watch := BoardTopicWatch{
		SemanticBoardID: semanticBoardID,
		Label:           label,
		Status:          WatchStatusActive,
	}
	if err := r.db.Create(&watch).Error; err != nil {
		return nil, fmt.Errorf("create watch: %w", err)
	}
	return &watch, nil
}

// ListWatchesByBoard returns all watches for a semantic board (any status).
func (r *TopicGraphRepository) ListWatchesByBoard(boardID uint) ([]BoardTopicWatch, error) {
	var watches []BoardTopicWatch
	if err := r.db.Where("semantic_board_id = ?", boardID).Order("created_at ASC").Find(&watches).Error; err != nil {
		return nil, fmt.Errorf("list watches: %w", err)
	}
	return watches, nil
}

// ListActiveWatchesByBoard returns only active watches for a semantic board.
func (r *TopicGraphRepository) ListActiveWatchesByBoard(boardID uint) ([]BoardTopicWatch, error) {
	var watches []BoardTopicWatch
	if err := r.db.Where("semantic_board_id = ? AND status = ?", boardID, WatchStatusActive).
		Order("created_at ASC").Find(&watches).Error; err != nil {
		return nil, fmt.Errorf("list active watches: %w", err)
	}
	return watches, nil
}

// UpdateWatch updates a watch's label and/or status. Pass nil to leave a field unchanged.
func (r *TopicGraphRepository) UpdateWatch(watchID uint, label, status *string) (*BoardTopicWatch, error) {
	var watch BoardTopicWatch
	if err := r.db.First(&watch, watchID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("watch %d not found", watchID)
		}
		return nil, fmt.Errorf("find watch: %w", err)
	}

	updates := map[string]interface{}{}
	if label != nil {
		updates["label"] = *label
	}
	if status != nil {
		updates["status"] = *status
	}
	if len(updates) == 0 {
		return &watch, nil
	}

	if err := r.db.Model(&watch).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update watch: %w", err)
	}
	// Reload to return fresh data
	if err := r.db.First(&watch, watchID).Error; err != nil {
		return nil, fmt.Errorf("reload watch after update: %w", err)
	}
	return &watch, nil
}

// DeleteWatch deletes a watch and cascade-deletes its hits (via FK OnDelete:CASCADE).
func (r *TopicGraphRepository) DeleteWatch(watchID uint) error {
	result := r.db.Delete(&BoardTopicWatch{}, watchID)
	if result.Error != nil {
		return fmt.Errorf("delete watch: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("watch %d not found", watchID)
	}
	return nil
}

// GetWatchHitsByReport returns all watch hits for a given daily report.
func (r *TopicGraphRepository) GetWatchHitsByReport(reportID uint) ([]TopicWatchHit, error) {
	var hits []TopicWatchHit
	if err := r.db.Where("report_id = ?", reportID).Order("created_at ASC").Find(&hits).Error; err != nil {
		return nil, fmt.Errorf("get watch hits: %w", err)
	}
	return hits, nil
}
