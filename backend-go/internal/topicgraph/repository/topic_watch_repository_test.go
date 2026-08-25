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

	watch, err := repo.CreateWatch(CreateWatchInput{SemanticBoardID: 42, Label: "美伊会不会真打起来", Type: ""})
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
	_, err := repo.CreateWatch(CreateWatchInput{SemanticBoardID: 1, Label: "watch-a", Type: ""})
	require.NoError(t, err)
	_, err = repo.CreateWatch(CreateWatchInput{SemanticBoardID: 1, Label: "watch-b", Type: ""})
	require.NoError(t, err)

	// Create a watch for a different board
	_, err = repo.CreateWatch(CreateWatchInput{SemanticBoardID: 2, Label: "watch-c", Type: ""})
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

	watch, err := repo.CreateWatch(CreateWatchInput{SemanticBoardID: 1, Label: "original label", Type: ""})
	require.NoError(t, err)

	// Update label only
	newLabel := "updated label"
	updated, err := repo.UpdateWatch(watch.ID, &newLabel, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "updated label", updated.Label)
	assert.Equal(t, WatchStatusActive, updated.Status) // status unchanged

	// Update status only
	paused := WatchStatusPaused
	updated, err = repo.UpdateWatch(watch.ID, nil, nil, &paused)
	require.NoError(t, err)
	assert.Equal(t, "updated label", updated.Label) // label unchanged
	assert.Equal(t, WatchStatusPaused, updated.Status)

	// Update both
	bothLabel := "both updated"
	active := WatchStatusActive
	updated, err = repo.UpdateWatch(watch.ID, &bothLabel, nil, &active)
	require.NoError(t, err)
	assert.Equal(t, "both updated", updated.Label)
	assert.Equal(t, WatchStatusActive, updated.Status)

	// Update non-existent watch
	_, err = repo.UpdateWatch(99999, &newLabel, nil, nil)
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

	watch, err := repo.CreateWatch(CreateWatchInput{SemanticBoardID: 1, Label: "watch to delete", Type: ""})
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

	w1, err := repo.CreateWatch(CreateWatchInput{SemanticBoardID: 1, Label: "watch-1", Type: ""})
	require.NoError(t, err)
	w2, err := repo.CreateWatch(CreateWatchInput{SemanticBoardID: 1, Label: "watch-2", Type: ""})
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

	_, err := repo.CreateWatch(CreateWatchInput{SemanticBoardID: 1, Label: "active-1", Type: ""}) // defaults to active
	require.NoError(t, err)
	w2, err := repo.CreateWatch(CreateWatchInput{SemanticBoardID: 1, Label: "will-be-paused", Type: ""})
	require.NoError(t, err)
	_, err = repo.CreateWatch(CreateWatchInput{SemanticBoardID: 1, Label: "active-2", Type: ""})
	require.NoError(t, err)

	// Pause w2
	paused := WatchStatusPaused
	_, err = repo.UpdateWatch(w2.ID, nil, nil, &paused)
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

	w, err := repo.CreateWatch(CreateWatchInput{SemanticBoardID: 1, Label: "paused-only", Type: ""})
	require.NoError(t, err)
	paused := WatchStatusPaused
	_, err = repo.UpdateWatch(w.ID, nil, nil, &paused)
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
	watch, err := repo.CreateWatch(CreateWatchInput{SemanticBoardID: 1, Label: "test watch", Type: ""})
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
	watch, err := repo.CreateWatch(CreateWatchInput{SemanticBoardID: 1, Label: "test watch", Type: ""})
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

	watch, err := repo.CreateWatch(CreateWatchInput{SemanticBoardID: 1, Label: "test watch", Type: ""})
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

// ── watch-keyword-and-quickadd: type column ──────────────────────────────────

func TestCreateWatch_TypeVariants(t *testing.T) {
	repo := setupWatchTestDB(t)

	// Backward-compatible: empty type defaults to label.
	w1, err := repo.CreateWatch(CreateWatchInput{SemanticBoardID: 1, Label: "legacy style", Type: ""})
	require.NoError(t, err)
	assert.Equal(t, WatchTypeLabel, w1.Type)

	// Explicit keyword type.
	w2, err := repo.CreateWatch(CreateWatchInput{SemanticBoardID: 1, Label: "ASML|镓锗 出口", Type: WatchTypeKeyword})
	require.NoError(t, err)
	assert.Equal(t, WatchTypeKeyword, w2.Type)

	// Explicit label type.
	w3, err := repo.CreateWatch(CreateWatchInput{SemanticBoardID: 1, Label: "explicit", Type: WatchTypeLabel})
	require.NoError(t, err)
	assert.Equal(t, WatchTypeLabel, w3.Type)
}

// 类型约束 scenario: the DB CHECK (migration 20260824_0002) rejects any type
// outside ('label','keyword').
func TestWatchTypeConstraint_RejectsInvalidType(t *testing.T) {
	repo := setupWatchTestDB(t)

	err := repo.db.Exec(`INSERT INTO board_topic_watches (semantic_board_id, label, type, status) VALUES (1, 'bad', 'regex', 'active')`).Error
	require.Error(t, err, "CHECK chk_board_topic_watches_type must reject invalid type")
	assert.Contains(t, err.Error(), "chk_board_topic_watches_type")
}

// GetWatchByID returns the watch (keyword instant-match loader).
func TestGetWatchByID(t *testing.T) {
	repo := setupWatchTestDB(t)

	w, err := repo.CreateWatch(CreateWatchInput{SemanticBoardID: 1, Label: "ASML", Type: WatchTypeKeyword})
	require.NoError(t, err)

	got, err := repo.GetWatchByID(w.ID)
	require.NoError(t, err)
	assert.Equal(t, w.ID, got.ID)
	assert.Equal(t, WatchTypeKeyword, got.Type)
	assert.Equal(t, "ASML", got.Label)

	_, err = repo.GetWatchByID(99999)
	assert.Error(t, err)
}

func TestGetWatchHitsByReport_OnlyActiveWithWatchDescriptor(t *testing.T) {
	repo := setupWatchTestDB(t)
	now := NormalizeReportDate(time.Now())

	active, err := repo.CreateWatch(CreateWatchInput{SemanticBoardID: 1, Label: "ASML", Type: WatchTypeKeyword})
	require.NoError(t, err)
	paused, err := repo.CreateWatch(CreateWatchInput{SemanticBoardID: 1, Label: "paused", Type: WatchTypeLabel})
	require.NoError(t, err)
	pausedStatus := WatchStatusPaused
	_, err = repo.UpdateWatch(paused.ID, nil, nil, &pausedStatus)
	require.NoError(t, err)
	deleted, err := repo.CreateWatch(CreateWatchInput{SemanticBoardID: 1, Label: "deleted", Type: WatchTypeLabel})
	require.NoError(t, err)

	for _, hit := range []TopicWatchHit{
		{WatchID: active.ID, SectionID: 1, ReportID: 500, PeriodDate: now, Reason: "active"},
		{WatchID: paused.ID, SectionID: 2, ReportID: 500, PeriodDate: now, Reason: "paused"},
		{WatchID: deleted.ID, SectionID: 3, ReportID: 500, PeriodDate: now, Reason: "deleted"},
	} {
		require.NoError(t, repo.db.Create(&hit).Error)
	}
	require.NoError(t, repo.DeleteWatch(deleted.ID))

	hits, err := repo.GetWatchHitsByReport(500)
	require.NoError(t, err)
	require.Len(t, hits, 1)
	assert.Equal(t, active.ID, hits[0].WatchID)
	assert.Equal(t, "ASML", hits[0].WatchLabel)
	assert.Equal(t, WatchTypeKeyword, hits[0].WatchType)
}

// ── watch-materialized-topic: sentence-track fields & materialized listing ──

// TestCreateWatchSentenceTopicFields verifies the sentence_track creation
// payload round-trips: type/query/embedding_cache persist and the hint-track
// defaults stay untouched (query empty, no cache, no topic link).
func TestCreateWatchSentenceTopicFields(t *testing.T) {
	repo := setupWatchTestDB(t)

	watch, err := repo.CreateWatch(CreateWatchInput{
		SemanticBoardID: 42,
		Label:           "AI 编程工具进展",
		Type:            WatchTypeSentenceTopic,
		Query:           "AI coding assistant 的进展和竞争格局",
		EmbeddingCache:  strPtr("[0.1,0.2,0.3]"),
	})
	require.NoError(t, err)
	assert.Equal(t, WatchTypeSentenceTopic, watch.Type)
	assert.Equal(t, "AI coding assistant 的进展和竞争格局", watch.Query)
	require.NotNil(t, watch.EmbeddingCache)
	assert.Equal(t, "[0.1,0.2,0.3]", *watch.EmbeddingCache)
	assert.Nil(t, watch.PersistentTopicID, "fresh sentence watch has no topic link yet")

	// Round-trip through a fresh read (GORM serialization included).
	reread, err := repo.GetWatchByID(watch.ID)
	require.NoError(t, err)
	assert.Equal(t, WatchTypeSentenceTopic, reread.Type)
	assert.Equal(t, "AI coding assistant 的进展和竞争格局", reread.Query)
	require.NotNil(t, reread.EmbeddingCache)
	assert.Equal(t, "[0.1,0.2,0.3]", *reread.EmbeddingCache)
}

// TestUpdateWatchInvalidatesEmbeddingCache verifies the D3 cache lifecycle:
// updating label OR query must clear embedding_cache; updating only status
// must leave the cache intact.
func TestUpdateWatchInvalidatesEmbeddingCache(t *testing.T) {
	repo := setupWatchTestDB(t)

	watch, err := repo.CreateWatch(CreateWatchInput{
		SemanticBoardID: 42,
		Label:           "AI 编程工具进展",
		Type:            WatchTypeSentenceTopic,
		Query:           "原检索句",
		EmbeddingCache:  strPtr("[0.1,0.2,0.3]"),
	})
	require.NoError(t, err)

	// status-only update keeps the cache.
	paused := WatchStatusPaused
	updated, err := repo.UpdateWatch(watch.ID, nil, nil, &paused)
	require.NoError(t, err)
	require.NotNil(t, updated.EmbeddingCache)
	assert.Equal(t, "[0.1,0.2,0.3]", *updated.EmbeddingCache, "status-only PATCH must not drop the cache")

	// query update invalidates the cache.
	newQuery := "换了检索句"
	updated, err = repo.UpdateWatch(watch.ID, nil, &newQuery, nil)
	require.NoError(t, err)
	assert.Nil(t, updated.EmbeddingCache, "query PATCH must invalidate embedding_cache")
	assert.Equal(t, newQuery, updated.Query)

	// label update also invalidates (query falls back to label semantics).
	updated, err = repo.UpdateWatch(watch.ID, &newQuery, nil, nil)
	require.NoError(t, err)
	assert.Nil(t, updated.EmbeddingCache, "label PATCH must invalidate embedding_cache")
}

// TestListActiveMaterializedWatchesByBoard verifies the materialization-phase
// input selector: only active keyword_topic/sentence_topic watches return;
// hint tracks (label/keyword) and paused watches are excluded.
func TestListActiveMaterializedWatchesByBoard(t *testing.T) {
	repo := setupWatchTestDB(t)

	_, err := repo.CreateWatch(CreateWatchInput{SemanticBoardID: 7, Label: "hint label", Type: WatchTypeLabel})
	require.NoError(t, err)
	_, err = repo.CreateWatch(CreateWatchInput{SemanticBoardID: 7, Label: "hint kw", Type: WatchTypeKeyword})
	require.NoError(t, err)
	kw, err := repo.CreateWatch(CreateWatchInput{SemanticBoardID: 7, Label: "harness", Type: WatchTypeKeywordTopic})
	require.NoError(t, err)
	st, err := repo.CreateWatch(CreateWatchInput{SemanticBoardID: 7, Label: "AI 进展", Type: WatchTypeSentenceTopic, Query: "AI 进展"})
	require.NoError(t, err)
	pausedST, err := repo.CreateWatch(CreateWatchInput{SemanticBoardID: 7, Label: "paused st", Type: WatchTypeSentenceTopic, Query: "x"})
	require.NoError(t, err)
	paused := WatchStatusPaused
	_, err = repo.UpdateWatch(pausedST.ID, nil, nil, &paused)
	require.NoError(t, err)
	_, err = repo.CreateWatch(CreateWatchInput{SemanticBoardID: 8, Label: "other board", Type: WatchTypeKeywordTopic})
	require.NoError(t, err)

	watches, err := repo.ListActiveMaterializedWatchesByBoard(7)
	require.NoError(t, err)
	require.Len(t, watches, 2, "only active keyword_topic + sentence_topic of board 7")
	assert.Equal(t, kw.ID, watches[0].ID)
	assert.Equal(t, st.ID, watches[1].ID)
}

// TestUpdateWatchEmbeddingCacheAndTopicLink verifies the lazy-recompute
// write-back paths: embedding cache refill and first-materialization topic
// binding round-trip.
func TestUpdateWatchEmbeddingCacheAndTopicLink(t *testing.T) {
	repo := setupWatchTestDB(t)

	watch, err := repo.CreateWatch(CreateWatchInput{
		SemanticBoardID: 42,
		Label:           "AI 编程工具进展",
		Type:            WatchTypeSentenceTopic,
		Query:           "AI coding assistant",
	})
	require.NoError(t, err)

	require.NoError(t, repo.UpdateWatchEmbeddingCache(watch.ID, "[0.4,0.5,0.6]"))
	reread, err := repo.GetWatchByID(watch.ID)
	require.NoError(t, err)
	require.NotNil(t, reread.EmbeddingCache)
	assert.Equal(t, "[0.4,0.5,0.6]", *reread.EmbeddingCache)

	// The migration-created FK requires a real topic row; create one directly.
	var topicID uint
	require.NoError(t, repo.db.Raw(`INSERT INTO board_persistent_topics
		(semantic_board_id, label, status, source, first_seen_date, last_seen_date, hit_count, consecutive_hits, created_at, updated_at)
		VALUES (42, 'test watch topic', 'active', 'manual', '2026-08-25', '2026-08-25', 1, 1, now(), now()) RETURNING id`).Scan(&topicID).Error)
	require.NoError(t, repo.SetWatchPersistentTopic(watch.ID, topicID))
	reread, err = repo.GetWatchByID(watch.ID)
	require.NoError(t, err)
	require.NotNil(t, reread.PersistentTopicID)
	assert.Equal(t, topicID, *reread.PersistentTopicID)
}

// strPtr is a test helper for optional pointer fields.
func strPtr(s string) *string { return &s }
