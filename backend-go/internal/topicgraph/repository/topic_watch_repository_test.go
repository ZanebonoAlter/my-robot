package repository

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm/clause"

	"syntopica-backend/internal/platform/testutil"
)

// setupWatchTestDB provisions a testcontainer PostgreSQL DB (golden schema) and
// returns a fresh repository instance. BoardTopicWatch / TopicWatchHit —
// including the OnDelete:CASCADE foreign key on Hits and the composite unique
// index idx_watch_section_report on (watch_id, section_id, report_id) — are
// created by the production AutoMigrate run inside testutil.SetupTestDB, so no
// manual AutoMigrate is needed. SQLite formerly needed PRAGMA foreign_keys=ON
// for the cascade; on PostgreSQL the schema-level FK carries it.
func setupWatchTestDB(t *testing.T) *TopicGraphRepository {
	t.Helper()
	db := testutil.SetupTestDB(t)
	return NewTopicGraphRepository(db)
}

// RED test: CreateWatch writes a watch with status=active by default
func TestCreateWatch(t *testing.T) {
	repo := setupWatchTestDB(t)

	watch, err := repo.CreateWatch(42, "美伊会不会真打起来")
	require.NoError(t, err)
	require.NotNil(t, watch)
	assert.Equal(t, uint(42), watch.SemanticBoardID)
	assert.Equal(t, "美伊会不会真打起来", watch.Label)
	assert.Equal(t, WatchStatusActive, watch.Status)
	assert.False(t, watch.CreatedAt.IsZero())
	assert.Equal(t, watch.CreatedAt.Unix(), watch.UpdatedAt.Unix())
}

// RED test: ListWatchesByBoard returns all watches for a board
func TestListWatchesByBoard(t *testing.T) {
	repo := setupWatchTestDB(t)

	// Create watches for board 1
	_, err := repo.CreateWatch(1, "watch-a")
	require.NoError(t, err)
	_, err = repo.CreateWatch(1, "watch-b")
	require.NoError(t, err)

	// Create a watch for a different board
	_, err = repo.CreateWatch(2, "watch-c")
	require.NoError(t, err)

	watches, err := repo.ListWatchesByBoard(1)
	require.NoError(t, err)
	require.Len(t, watches, 2)
	assert.Equal(t, "watch-a", watches[0].Label)
	assert.Equal(t, "watch-b", watches[1].Label)

	watches2, err := repo.ListWatchesByBoard(2)
	require.NoError(t, err)
	require.Len(t, watches2, 1)
	assert.Equal(t, "watch-c", watches2[0].Label)

	// Empty board
	watches3, err := repo.ListWatchesByBoard(999)
	require.NoError(t, err)
	assert.Empty(t, watches3)
}

// RED test: UpdateWatch can change label and status
func TestUpdateWatch(t *testing.T) {
	repo := setupWatchTestDB(t)

	watch, err := repo.CreateWatch(1, "original label")
	require.NoError(t, err)

	// Update label only
	newLabel := "updated label"
	updated, err := repo.UpdateWatch(watch.ID, &newLabel, nil)
	require.NoError(t, err)
	assert.Equal(t, "updated label", updated.Label)
	assert.Equal(t, WatchStatusActive, updated.Status) // status unchanged

	// Update status only
	paused := WatchStatusPaused
	updated, err = repo.UpdateWatch(watch.ID, nil, &paused)
	require.NoError(t, err)
	assert.Equal(t, "updated label", updated.Label) // label unchanged
	assert.Equal(t, WatchStatusPaused, updated.Status)

	// Update both
	bothLabel := "both updated"
	active := WatchStatusActive
	updated, err = repo.UpdateWatch(watch.ID, &bothLabel, &active)
	require.NoError(t, err)
	assert.Equal(t, "both updated", updated.Label)
	assert.Equal(t, WatchStatusActive, updated.Status)

	// Update non-existent watch
	_, err = repo.UpdateWatch(99999, &newLabel, nil)
	assert.Error(t, err)
}

// RED test: DeleteWatch cascades to topic_watch_hits
func TestDeleteWatchCascadesHits(t *testing.T) {
	// Asserts DeleteWatch cascade-deletes its topic_watch_hits via the real DB FK
	// `fk_topic_watch_hits_watch` ON DELETE CASCADE (migration 20260801_0002).
	// History: this cascade was previously dead on PG — the GORM model tag declared
	// OnDelete:CASCADE but DisableForeignKeyConstraintWhenMigrating=true left
	// AutoMigrate building no FK, and the watch migrations (20260630_0001/_0002)
	// added none. SQLite masked it (PRAGMA foreign_keys=ON); the PG migration
	// surfaced it. Fixed by fix-watch-delete-cascade (FK versioned migration).
	repo := setupWatchTestDB(t)

	watch, err := repo.CreateWatch(1, "watch to delete")
	require.NoError(t, err)

	// Create some hits for this watch
	now := time.Now().Truncate(24 * time.Hour)
	for i := 0; i < 3; i++ {
		hit := TopicWatchHit{
			WatchID:    watch.ID,
			SectionID:  uint(100 + i),
			ReportID:   uint(200 + i),
			PeriodDate: now,
			Reason:     fmt.Sprintf("reason %d", i),
		}
		require.NoError(t, repo.db.Create(&hit).Error)
	}

	// Verify hits exist before delete
	var count int64
	repo.db.Model(&TopicWatchHit{}).Where("watch_id = ?", watch.ID).Count(&count)
	require.Equal(t, int64(3), count)

	// Delete the watch
	err = repo.DeleteWatch(watch.ID)
	require.NoError(t, err)

	// Verify watch is gone
	var found BoardTopicWatch
	err = repo.db.First(&found, watch.ID).Error
	assert.Error(t, err) // should not be found

	// Verify hits are cascade-deleted
	repo.db.Model(&TopicWatchHit{}).Where("watch_id = ?", watch.ID).Count(&count)
	assert.Equal(t, int64(0), count, "hits should be cascade-deleted")
}

// RED test: DeleteWatch does not affect hits of other watches
func TestDeleteWatchDoesNotAffectOtherWatchHits(t *testing.T) {
	// Asserts deleting w1 cascade-deletes only w1's hits (w2's untouched), via the
	// real DB FK `fk_topic_watch_hits_watch` ON DELETE CASCADE (migration
	// 20260801_0002). See TestDeleteWatchCascadesHits for the SQLite-vs-PG history.
	repo := setupWatchTestDB(t)

	w1, err := repo.CreateWatch(1, "watch-1")
	require.NoError(t, err)
	w2, err := repo.CreateWatch(1, "watch-2")
	require.NoError(t, err)

	now := time.Now()
	repo.db.Create(&TopicWatchHit{WatchID: w1.ID, SectionID: 100, ReportID: 200, PeriodDate: now, Reason: "w1 hit"})
	repo.db.Create(&TopicWatchHit{WatchID: w2.ID, SectionID: 101, ReportID: 200, PeriodDate: now, Reason: "w2 hit"})

	// Delete w1
	require.NoError(t, repo.DeleteWatch(w1.ID))

	// w1 hits gone
	var count int64
	repo.db.Model(&TopicWatchHit{}).Where("watch_id = ?", w1.ID).Count(&count)
	assert.Equal(t, int64(0), count)

	// w2 hits remain
	repo.db.Model(&TopicWatchHit{}).Where("watch_id = ?", w2.ID).Count(&count)
	assert.Equal(t, int64(1), count)

	// w2 itself still exists
	var w2found BoardTopicWatch
	require.NoError(t, repo.db.First(&w2found, w2.ID).Error)
}

// RED test: ListActiveWatchesByBoard only returns status=active
func TestListActiveWatchesByBoard(t *testing.T) {
	repo := setupWatchTestDB(t)

	_, err := repo.CreateWatch(1, "active-1") // defaults to active
	require.NoError(t, err)
	w2, err := repo.CreateWatch(1, "will-be-paused")
	require.NoError(t, err)
	_, err = repo.CreateWatch(1, "active-2")
	require.NoError(t, err)

	// Pause w2
	paused := WatchStatusPaused
	_, err = repo.UpdateWatch(w2.ID, nil, &paused)
	require.NoError(t, err)

	// ListActiveWatchesByBoard
	activeWatches, err := repo.ListActiveWatchesByBoard(1)
	require.NoError(t, err)
	require.Len(t, activeWatches, 2)
	labels := make([]string, len(activeWatches))
	for i, w := range activeWatches {
		labels[i] = w.Label
		assert.Equal(t, WatchStatusActive, w.Status)
	}
	assert.Contains(t, labels, "active-1")
	assert.Contains(t, labels, "active-2")
	assert.NotContains(t, labels, "will-be-paused")
}

// RED test: ListActiveWatchesByBoard returns empty for board with only paused watches
func TestListActiveWatchesByBoard_AllPaused(t *testing.T) {
	repo := setupWatchTestDB(t)

	w, err := repo.CreateWatch(1, "paused-only")
	require.NoError(t, err)
	paused := WatchStatusPaused
	_, err = repo.UpdateWatch(w.ID, nil, &paused)
	require.NoError(t, err)

	activeWatches, err := repo.ListActiveWatchesByBoard(1)
	require.NoError(t, err)
	assert.Empty(t, activeWatches)
}

// TestTopicWatchHit_DuplicateRejected verifies that inserting a duplicate
// (watch_id, section_id, report_id) pair violates the composite unique index
// and returns an error.
func TestTopicWatchHit_DuplicateRejected(t *testing.T) {
	repo := setupWatchTestDB(t)
	now := time.Now().Truncate(24 * time.Hour)

	// Create a watch so the FK on watch_id is satisfied.
	watch, err := repo.CreateWatch(1, "test watch")
	require.NoError(t, err)

	hit := TopicWatchHit{
		WatchID:    watch.ID,
		SectionID:  100,
		ReportID:   200,
		PeriodDate: now,
		Reason:     "first hit",
	}
	require.NoError(t, repo.db.Create(&hit).Error)

	// Same (watch_id, section_id, report_id) — must be rejected.
	dup := TopicWatchHit{
		WatchID:    watch.ID,
		SectionID:  100,
		ReportID:   200,
		PeriodDate: now,
		Reason:     "duplicate",
	}
	err = repo.db.Create(&dup).Error
	assert.Error(t, err, "duplicate must be rejected by composite unique index")

	// Only 1 row exists.
	var count int64
	repo.db.Model(&TopicWatchHit{}).Count(&count)
	assert.Equal(t, int64(1), count)
}

// TestTopicWatchHit_UpsertDedup verifies that batch create with
// OnConflict{DoNothing} silently skips duplicates and results in exactly 1
// unique row per (watch_id, section_id, report_id) key.
func TestTopicWatchHit_UpsertDedup(t *testing.T) {
	repo := setupWatchTestDB(t)
	now := time.Now().Truncate(24 * time.Hour)

	// Create a watch so the FK on watch_id is satisfied.
	watch, err := repo.CreateWatch(1, "test watch")
	require.NoError(t, err)

	hits := []TopicWatchHit{
		{WatchID: watch.ID, SectionID: 100, ReportID: 200, PeriodDate: now, Reason: "first"},
		{WatchID: watch.ID, SectionID: 100, ReportID: 200, PeriodDate: now, Reason: "duplicate"},
		{WatchID: watch.ID, SectionID: 101, ReportID: 200, PeriodDate: now, Reason: "different section"},
	}

	err = repo.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "watch_id"}, {Name: "section_id"}, {Name: "report_id"}},
		DoNothing: true,
	}).Create(&hits).Error
	require.NoError(t, err)

	var count int64
	repo.db.Model(&TopicWatchHit{}).Count(&count)
	assert.Equal(t, int64(2), count, "should have 2 rows (1 duplicate skipped)")

	// Verify both unique keys are present.
	hitRows, err := repo.GetWatchHitsByReport(200)
	require.NoError(t, err)
	assert.Len(t, hitRows, 2)
	sectionIDs := make([]uint, len(hitRows))
	for i, h := range hitRows {
		sectionIDs[i] = h.SectionID
	}
	assert.Contains(t, sectionIDs, uint(100))
	assert.Contains(t, sectionIDs, uint(101))
}

// RED test: GetWatchHitsByReport returns hits for a specific report
func TestGetWatchHitsByReport(t *testing.T) {
	repo := setupWatchTestDB(t)

	watch, err := repo.CreateWatch(1, "test watch")
	require.NoError(t, err)

	now := time.Now().Truncate(24 * time.Hour)
	// Hits for report 200
	repo.db.Create(&TopicWatchHit{WatchID: watch.ID, SectionID: 100, ReportID: 200, PeriodDate: now, Reason: "r1"})
	repo.db.Create(&TopicWatchHit{WatchID: watch.ID, SectionID: 101, ReportID: 200, PeriodDate: now, Reason: "r2"})
	// Hit for report 201 (different report)
	repo.db.Create(&TopicWatchHit{WatchID: watch.ID, SectionID: 102, ReportID: 201, PeriodDate: now, Reason: "r3"})

	hits, err := repo.GetWatchHitsByReport(200)
	require.NoError(t, err)
	require.Len(t, hits, 2)
	assert.Equal(t, uint(100), hits[0].SectionID)
	assert.Equal(t, uint(101), hits[1].SectionID)

	// Report with no hits
	hits2, err := repo.GetWatchHitsByReport(999)
	require.NoError(t, err)
	assert.Empty(t, hits2)
}
