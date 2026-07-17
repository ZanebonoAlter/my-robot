package repository

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"syntopica-backend/internal/models"
)

// BoardUpgradeSuggestionRepository persists semantic-board upgrade suggestions
// and drives their lifecycle (pending → confirmed / dismissed). See
// design.md D3 of change board-discovery-expansion.
type BoardUpgradeSuggestionRepository struct {
	db *gorm.DB
}

func NewBoardUpgradeSuggestionRepository(db *gorm.DB) *BoardUpgradeSuggestionRepository {
	return &BoardUpgradeSuggestionRepository{db: db}
}

// InsertPending persists a suggestion row with status='pending'. The partial
// unique index uq_board_upgrade_suggestions_hash ON (suggestion_hash) WHERE
// status='pending' enforces idempotent generation: a second insert with the
// same pending hash is a no-op (ON CONFLICT DO NOTHING), returning
// inserted=false. This makes re-running the generator a safe no-op for an
// unchanged cluster (spec: 建议生成幂等).
func (r *BoardUpgradeSuggestionRepository) InsertPending(ctx context.Context, sug *models.BoardUpgradeSuggestion) (bool, error) {
	sug.Status = "pending"
	res := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		// Target the partial unique index whose predicate is status='pending'.
		Columns:     []clause.Column{{Name: "suggestion_hash"}},
		TargetWhere: clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: "status = 'pending'"}}},
		DoNothing:   true,
	}).Create(sug)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

// MarkConfirmed transitions a pending suggestion to confirmed, recording
// resolved_at=now. It MUST run inside the upgrade-execute transaction (tx) so a
// board_composition write failure rolls the suggestion state back unchanged
// (spec: confirm 联动). Only affects rows currently pending (idempotent against
// a double-confirm of an already-resolved suggestion).
func (r *BoardUpgradeSuggestionRepository) MarkConfirmed(tx *gorm.DB, id uint) error {
	return tx.Model(&models.BoardUpgradeSuggestion{}).
		Where("id = ? AND status = ?", id, "pending").
		Updates(map[string]interface{}{
			"status":      "confirmed",
			"resolved_at": time.Now(),
		}).Error
}

// CountDismissedInCooldown returns how many dismissed suggestions share the
// given hash and were resolved within the last cooldownDays (spec: dismissed
// 冷却期). A positive count means the hash is in cooldown and re-generation
// must be skipped. resolved_at is the cooldown start (set when the row was
// dismissed). The partial unique index allows many non-pending rows per hash,
// so this counts across all statuses='dismissed'.
func (r *BoardUpgradeSuggestionRepository) CountDismissedInCooldown(ctx context.Context, hash string, cooldownDays int) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.BoardUpgradeSuggestion{}).
		Where("suggestion_hash = ? AND status = ? AND resolved_at >= NOW() - (? * INTERVAL '1 day')", hash, "dismissed", cooldownDays).
		Count(&count).Error
	return count, err
}

// CloseWatchSuggestions confirms every pending watch suggestion whose
// auxiliary_label_ids overlaps auxIDs (spec: watch 建议成簇自动关闭). Called
// when a previously single-label cluster grows to ≥2 members: the single-label
// watch is no longer needed and is closed (→ confirmed, resolved_at=now). Only
// decision='watch' rows are touched; create_new/merge suggestions are untouched.
// Returns the number of watch suggestions closed.
func (r *BoardUpgradeSuggestionRepository) CloseWatchSuggestions(ctx context.Context, auxIDs []uint) (int64, error) {
	if len(auxIDs) == 0 {
		return 0, nil
	}
	// auxiliary_label_ids is jsonb; test containment per id via jsonb_build_array.
	// OR-ing the per-id containments gives the overlap.
	parts := make([]string, 0, len(auxIDs))
	args := make([]interface{}, 0, len(auxIDs))
	for _, id := range auxIDs {
		parts = append(parts, "auxiliary_label_ids @> jsonb_build_array(?::int)")
		args = append(args, id)
	}
	overlap := strings.Join(parts, " OR ")
	res := r.db.WithContext(ctx).Model(&models.BoardUpgradeSuggestion{}).
		Where("decision = ? AND status = ?", "watch", "pending").
		Where("("+overlap+")", args...).
		Updates(map[string]interface{}{
			"status":      "confirmed",
			"resolved_at": time.Now(),
		})
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
}
