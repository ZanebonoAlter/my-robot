package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"syntopica-backend/internal/models"
	"syntopica-backend/internal/platform/testutil"
)

// TestBoardUpgradeSuggestionCloseWatchSuggestions verifies that when an aux
// label that was under single-label watch joins a cluster (≥2), the
// corresponding watch suggestion is closed (→ confirmed) while non-watch
// suggestions are untouched (spec: watch 建议成簇自动关闭).
//
// §3.5: CloseWatchSuggestions([5,6]) closes watch rows overlapping [5,6].
func TestBoardUpgradeSuggestionCloseWatchSuggestions(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewBoardUpgradeSuggestionRepository(db)

	// watch over [5] — closed because 5 is in the closing set.
	require.NoError(t, db.Create(&models.BoardUpgradeSuggestion{
		BatchID: "w1", Mode: "discover_new", Decision: "watch",
		BoardLabel: "Watch 5", AuxiliaryLabelIDs: []uint{5},
		Status: "pending", SuggestionHash: "watch-5",
	}).Error)
	// watch over [6,7] — closed because 6 is in the closing set.
	require.NoError(t, db.Create(&models.BoardUpgradeSuggestion{
		BatchID: "w2", Mode: "discover_new", Decision: "watch",
		BoardLabel: "Watch 6,7", AuxiliaryLabelIDs: []uint{6, 7},
		Status: "pending", SuggestionHash: "watch-67",
	}).Error)
	// create_new pending over [5] — must NOT be touched (only watch closes).
	require.NoError(t, db.Create(&models.BoardUpgradeSuggestion{
		BatchID: "c", Mode: "discover_new", Decision: "create_new",
		BoardLabel: "Create 5", AuxiliaryLabelIDs: []uint{5},
		Status: "pending", SuggestionHash: "create-5",
	}).Error)

	closed, err := repo.CloseWatchSuggestions(context.Background(), []uint{5, 6})
	require.NoError(t, err)
	require.Equal(t, int64(2), closed, "both watch suggestions overlapping [5,6] must close")

	var w1, w2 models.BoardUpgradeSuggestion
	require.NoError(t, db.Where("suggestion_hash = ?", "watch-5").First(&w1).Error)
	require.NoError(t, db.Where("suggestion_hash = ?", "watch-67").First(&w2).Error)
	require.Equal(t, "confirmed", w1.Status, "watch [5] must be confirmed")
	require.Equal(t, "confirmed", w2.Status, "watch [6,7] must be confirmed")
	require.NotNil(t, w1.ResolvedAt)

	var c models.BoardUpgradeSuggestion
	require.NoError(t, db.Where("suggestion_hash = ?", "create-5").First(&c).Error)
	require.Equal(t, "pending", c.Status, "non-watch suggestions must be untouched")
}

// TestBoardUpgradeSuggestionCountDismissedInCooldown verifies the cooldown
// rule for dismissed suggestions (spec: dismissed 冷却期): a suggestion
// dismissed within the cooldown window blocks re-generation; one past the
// window allows re-generation.
//
// §3.3: 3-day-old dismissal blocks (within 14d); 15-day-old allows.
func TestBoardUpgradeSuggestionCountDismissedInCooldown(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewBoardUpgradeSuggestionRepository(db)
	now := time.Now()

	threeDaysAgo := now.AddDate(0, 0, -3)
	fifteenDaysAgo := now.AddDate(0, 0, -15)

	// Dismissed 3 days ago → still inside the default 14-day cooldown.
	require.NoError(t, db.Create(&models.BoardUpgradeSuggestion{
		BatchID: "c-in", Mode: "discover_new", Decision: "create_new",
		BoardLabel: "In Cooldown", SuggestionHash: "cd-in",
		Status: "dismissed", ResolvedAt: &threeDaysAgo,
	}).Error)
	// Dismissed 15 days ago → cooldown window (14d) has elapsed.
	require.NoError(t, db.Create(&models.BoardUpgradeSuggestion{
		BatchID: "c-out", Mode: "discover_new", Decision: "create_new",
		BoardLabel: "Expired", SuggestionHash: "cd-out",
		Status: "dismissed", ResolvedAt: &fifteenDaysAgo,
	}).Error)

	blocked, err := repo.CountDismissedInCooldown(context.Background(), "cd-in", 14)
	require.NoError(t, err)
	require.Equal(t, int64(1), blocked, "dismissed 3 days ago is within the 14-day cooldown")

	expired, err := repo.CountDismissedInCooldown(context.Background(), "cd-out", 14)
	require.NoError(t, err)
	require.Equal(t, int64(0), expired, "dismissed 15 days ago is past the 14-day cooldown")
}

// TestBoardUpgradeSuggestionInsertPendingIsIdempotent verifies the partial
// unique index uq_board_upgrade_suggestions_hash makes a second insert with the
// same pending hash a no-op (spec: 建议生成幂等).
//
// §3.2: same cluster + same decision re-generated → original kept, no duplicate.
func TestBoardUpgradeSuggestionInsertPendingIsIdempotent(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewBoardUpgradeSuggestionRepository(db)

	first := &models.BoardUpgradeSuggestion{
		BatchID: "b2", Mode: "discover_new", Decision: "create_new",
		BoardLabel: "Dup Board", AuxiliaryLabelIDs: []uint{7},
		Confidence: "llm", Status: "pending", SuggestionHash: "dup-hash-3-2",
	}
	inserted1, err := repo.InsertPending(context.Background(), first)
	require.NoError(t, err)
	require.True(t, inserted1, "first insert must succeed")

	// Re-generate the same cluster+decision → same suggestion_hash, still pending.
	repeated := &models.BoardUpgradeSuggestion{
		BatchID: "b2-later", Mode: "discover_new", Decision: "create_new",
		BoardLabel: "Dup Board", AuxiliaryLabelIDs: []uint{7},
		Confidence: "llm", Status: "pending", SuggestionHash: "dup-hash-3-2",
	}
	inserted2, err := repo.InsertPending(context.Background(), repeated)
	require.NoError(t, err, "duplicate pending hash must be a no-op, not an error")
	require.False(t, inserted2, "duplicate must report inserted=false")

	var count int64
	require.NoError(t, db.Model(&models.BoardUpgradeSuggestion{}).Where("suggestion_hash = ?", "dup-hash-3-2").Count(&count).Error)
	require.Equal(t, int64(1), count, "only one pending row for the hash may exist")
}

// TestBoardUpgradeSuggestionInsertPendingPersistsRows verifies InsertPending
// stores each suggestion as a pending row (spec: 升级建议持久化存储).
//
// §3.1: 3 non-skip suggestions → 3 pending rows persisted.
func TestBoardUpgradeSuggestionInsertPendingPersistsRows(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewBoardUpgradeSuggestionRepository(db)

	base := time.Now()
	sugs := []*models.BoardUpgradeSuggestion{
		{BatchID: "b1", Mode: "discover_new", Decision: "create_new", BoardLabel: "Board A", Description: "d A", AuxiliaryLabelIDs: []uint{101}, Confidence: "llm", Status: "pending", SuggestionHash: "hash-3-1-a", CreatedAt: base},
		{BatchID: "b1", Mode: "discover_new", Decision: "create_new", BoardLabel: "Board B", Description: "d B", AuxiliaryLabelIDs: []uint{102}, Confidence: "llm", Status: "pending", SuggestionHash: "hash-3-1-b", CreatedAt: base},
		{BatchID: "b1", Mode: "discover_new", Decision: "create_new", BoardLabel: "Board C", Description: "d C", AuxiliaryLabelIDs: []uint{103}, Confidence: "llm", Status: "pending", SuggestionHash: "hash-3-1-c", CreatedAt: base},
	}
	for _, s := range sugs {
		inserted, err := repo.InsertPending(context.Background(), s)
		require.NoError(t, err)
		require.True(t, inserted, "distinct-hash insert must succeed")
	}

	var rows []models.BoardUpgradeSuggestion
	require.NoError(t, db.Order("id ASC").Find(&rows).Error)
	require.Len(t, rows, 3, "three non-skip suggestions must be persisted")
	for _, r := range rows {
		require.Equal(t, "pending", r.Status, "new suggestions start as pending")
		require.Equal(t, "discover_new", r.Mode)
		require.Equal(t, "create_new", r.Decision)
	}
}
