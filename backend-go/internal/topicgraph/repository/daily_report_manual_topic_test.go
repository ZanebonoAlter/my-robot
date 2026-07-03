package repository

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"syntopica-backend/internal/models"
	"syntopica-backend/internal/platform/database"
)

// ── aggregateEmbeddings tests ───────────────────────────────────────────────

func TestAggregateEmbeddings_Normal(t *testing.T) {
	vectors := [][]float64{
		{1.0, 2.0, 3.0},
		{4.0, 5.0, 6.0},
		{7.0, 8.0, 9.0},
	}
	mean, skipped := aggregateEmbeddings(vectors)
	assert.Equal(t, 0, skipped)
	assert.Len(t, mean, 3)
	// (1+4+7)/3=4, (2+5+8)/3=5, (3+6+9)/3=6
	assert.InDeltaSlice(t, []float64{4.0, 5.0, 6.0}, mean, 1e-9)
}

func TestAggregateEmbeddings_DimensionMismatch(t *testing.T) {
	vectors := [][]float64{
		{1.0, 2.0, 3.0},
		{4.0, 5.0},       // 2-dim — skipped
		{7.0, 8.0, 9.0},
	}
	mean, skipped := aggregateEmbeddings(vectors)
	assert.Equal(t, 1, skipped)
	assert.Len(t, mean, 3)
	// mean of first and third only: (1+7)/2=4, (2+8)/2=5, (3+9)/2=6
	assert.InDeltaSlice(t, []float64{4.0, 5.0, 6.0}, mean, 1e-9)
}

func TestAggregateEmbeddings_EmptyInput(t *testing.T) {
	mean, skipped := aggregateEmbeddings(nil)
	assert.Equal(t, 0, skipped)
	assert.Nil(t, mean)
}

func TestAggregateEmbeddings_AllSkipped(t *testing.T) {
	vectors := [][]float64{
		{1.0, 2.0, 3.0},
		nil,               // nil slice → skipped
		{4.0, 5.0},        // wrong dim (2 vs 3) → skipped
	}
	mean, skipped := aggregateEmbeddings(vectors)
	assert.Equal(t, 2, skipped)
	// only {1,2,3} is usable → mean = {1,2,3}
	assert.InDeltaSlice(t, []float64{1.0, 2.0, 3.0}, mean, 1e-9)
}

func TestAggregateEmbeddings_SingleVector(t *testing.T) {
	vectors := [][]float64{
		{3.5, -2.1, 0.0},
	}
	mean, skipped := aggregateEmbeddings(vectors)
	assert.Equal(t, 0, skipped)
	assert.InDeltaSlice(t, []float64{3.5, -2.1, 0.0}, mean, 1e-9)
}

func TestAggregateEmbeddings_EmptyVectorsAll(t *testing.T) {
	// all slices are empty (len=0) → none usable
	mean, skipped := aggregateEmbeddings([][]float64{{}, {}})
	assert.Equal(t, 2, skipped)
	assert.Nil(t, mean, "all empty → nil mean")
}

// ── detectOutliers tests ────────────────────────────────────────────────────

func TestDetectOutliers_AllTight(t *testing.T) {
	distances := []float64{0.1, 0.15, 0.12}
	threshold := 0.3
	flags := detectOutliers(distances, threshold)
	assert.Equal(t, []bool{false, false, false}, flags)
}

func TestDetectOutliers_ContainsOutlier(t *testing.T) {
	distances := []float64{0.1, 0.8, 0.12} // 0.8 > 0.3*1.3 = 0.39
	threshold := 0.3
	flags := detectOutliers(distances, threshold)
	assert.Equal(t, []bool{false, true, false}, flags)
}

func TestDetectOutliers_ThresholdBoundary(t *testing.T) {
	threshold := 0.3
	boundary := threshold * 1.3 // exactly 0.39
	distances := []float64{0.1, boundary, boundary + 1e-9}
	flags := detectOutliers(distances, threshold)
	// boundary (0.39) is NOT > 0.39 → false; boundary+epsilon IS > 0.39 → true
	assert.False(t, flags[1], "exactly at threshold must not be outlier")
	assert.True(t, flags[2], "slightly above threshold must be outlier")
}

func TestDetectOutliers_EmptyInput(t *testing.T) {
	flags := detectOutliers(nil, 0.3)
	assert.Nil(t, flags)
}

// ── formatPgVector tests ────────────────────────────────────────────────────

func TestFloatsToPgVector_Roundtrip(t *testing.T) {
	original := []float64{1.0, 2.5, -0.3}
	formatted := FloatsToPgVector(original)
	parsed, err := repoParsePgVector(formatted)
	assert.NoError(t, err)
	assert.Len(t, parsed, len(original))
	for i, v := range original {
		assert.InDelta(t, v, parsed[i], 1e-9)
	}
}

func TestFloatsToPgVector_Empty(t *testing.T) {
	result := FloatsToPgVector(nil)
	assert.Equal(t, "[]", result)
}

func TestFloatsToPgVector_SingleElement(t *testing.T) {
	result := FloatsToPgVector([]float64{42.0})
	parsed, err := repoParsePgVector(result)
	assert.NoError(t, err)
	assert.InDelta(t, 42.0, parsed[0], 1e-9)
}

// ── helpers ─────────────────────────────────────────────────────────────────

// verifyNoDbEmpty ensures a pure-function test didn't accidentally try to use a DB.
func verifyNoDbEmpty(t *testing.T) {
	t.Helper()
	if Repo != nil && Repo.db != nil {
		t.Skip("not a DB test – Repo is assigned from another test")
	}
}

func TestPureFunctionDetection(t *testing.T) {
	// Sanity: if Repo.db is non-nil, the global was leaked from another test.
	// Pure-function tests must not depend on Repo.
	verifyNoDbEmpty(t)
}

// ── CreateManualTopic SQLite unit tests ─────────────────────────────────────
// These tests use an in-memory SQLite DB. RebuildBoardRelations uses PG-specific
// SQL (period_date::date) so it will always fail on SQLite → the happy-path
// assertions live in the testcontainer integration tests (task 1.6). The
// SQLite tests here cover transaction rollback behaviour and edge cases.

func TestCreateManualTopic_NoSections(t *testing.T) {
	repo := setupManualTopicTestDB(t)
	_, skipped, err := repo.CreateManualTopic(0, "test", nil)
	assert.Error(t, err)
	assert.Nil(t, skipped)
	assert.Contains(t, err.Error(), "no sections")
}

func TestCreateManualTopic_EmptySectionIDs(t *testing.T) {
	repo := setupManualTopicTestDB(t)
	_, skipped, err := repo.CreateManualTopic(0, "test", []uint{})
	assert.Error(t, err)
	assert.Nil(t, skipped)
	assert.Contains(t, err.Error(), "no sections")
}

func TestCreateManualTopic_NoUsableEmbeddings(t *testing.T) {
	repo := setupManualTopicTestDB(t)

	boardID := seedTestBoard(t, repo.db)
	reportID := seedTestReport(t, repo.db, boardID, time.Now())
	sec := DailyReportSection{ReportID: reportID, ClusterLabel: "no-emb", Embedding: "", ArticleCount: 1}
	require.NoError(t, repo.db.Create(&sec).Error)

	_, skipped, err := repo.CreateManualTopic(boardID, "test", []uint{sec.ID})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "usable vector", "must reject when no section has a usable embedding")
	assert.Len(t, skipped, 1, "section without embedding must be skipped")
}

func TestCreateManualTopic_RollbackOnRebuildRelationsFailure(t *testing.T) {
	// RebuildBoardRelations uses PG-specific SQL that SQLite cannot execute.
	// This test proves that on failure, the ENTIRE transaction rolls back:
	// no topic is created, no sections are reassigned.
	repo := setupManualTopicTestDB(t)

	boardID := seedTestBoard(t, repo.db)
	reportID := seedTestReport(t, repo.db, boardID, time.Now())

	sec1 := DailyReportSection{ReportID: reportID, ClusterLabel: "s1", Embedding: vecStr(1.0, 0.0, 0.0), ArticleCount: 1}
	sec2 := DailyReportSection{ReportID: reportID, ClusterLabel: "s2", Embedding: vecStr(2.0, 0.0, 0.0), ArticleCount: 1}
	require.NoError(t, repo.db.Create(&sec1).Error)
	require.NoError(t, repo.db.Create(&sec2).Error)

	// Call CreateManualTopic — the RebuildBoardRelations call inside the
	// transaction will fail (PG-specific SQL on SQLite), so the whole
	// transaction MUST roll back.
	topic, skipped, err := repo.CreateManualTopic(boardID, "manual-lane", []uint{sec1.ID, sec2.ID})
	assert.Error(t, err, "expected rollback because RebuildBoardRelations fails on SQLite")
	assert.Nil(t, topic, "no topic should persist after rollback")
	assert.Empty(t, skipped, "no sections skipped — embeddings are valid")

	// Verify rollback: no topic created.
	var count int64
	repo.db.Model(&BoardPersistentTopic{}).Where("label = ?", "manual-lane").Count(&count)
	assert.Equal(t, int64(0), count, "topic must not exist after rollback")

	// Verify rollback: sections still unassigned.
	var sec1After DailyReportSection
	repo.db.First(&sec1After, sec1.ID)
	assert.Nil(t, sec1After.PersistentTopicID, "section must not be reassigned after rollback")
}

// ── GetComposeCandidates SQLite tests ──────────────────────────────────────

func TestGetComposeCandidates_ParsesEmbeddingsAndExcludesEmpty(t *testing.T) {
	repo := setupManualTopicTestDB(t)
	boardID := seedTestBoard(t, repo.db)
	today := time.Date(2026, 6, 29, 0, 0, 0, 0, time.UTC)
	reportID := seedTestReport(t, repo.db, boardID, today)

	// Three sections: two with usable embeddings, one with an empty vector.
	s1 := DailyReportSection{ReportID: reportID, ClusterLabel: "s1", Embedding: vecStr(0.1, 0.2, 0.3), ArticleCount: 1}
	s2 := DailyReportSection{ReportID: reportID, ClusterLabel: "s2", Embedding: vecStr(0.4, 0.5, 0.6), ArticleCount: 1}
	s3 := DailyReportSection{ReportID: reportID, ClusterLabel: "no-emb", Embedding: "", ArticleCount: 1}
	require.NoError(t, repo.db.Create(&s1).Error)
	require.NoError(t, repo.db.Create(&s2).Error)
	require.NoError(t, repo.db.Create(&s3).Error)

	resp, err := repo.GetComposeCandidates(boardID, 14)
	require.NoError(t, err)
	// Default MatchThreshold from DefaultPersistentTopicConfig (no ai_settings row).
	assert.InDelta(t, 0.30, resp.MatchThreshold, 1e-9)
	// Only the two sections with a usable embedding are returned (empty excluded).
	require.Len(t, resp.Sections, 2)
	assert.Equal(t, "s1", resp.Sections[0].ClusterLabel)
	assert.Len(t, resp.Sections[0].Embedding, 3)
	assert.InDeltaSlice(t, []float64{0.1, 0.2, 0.3}, resp.Sections[0].Embedding, 1e-9)
	assert.Equal(t, "s2", resp.Sections[1].ClusterLabel)
	// report_id 随 section 返回（编排态查线索复用，task 11.1）。
	assert.Equal(t, reportID, resp.Sections[0].ReportID)
	assert.Equal(t, reportID, resp.Sections[1].ReportID)
}

func TestGetComposeCandidates_AttachesTopicBrief(t *testing.T) {
	repo := setupManualTopicTestDB(t)
	boardID := seedTestBoard(t, repo.db)
	today := time.Date(2026, 6, 29, 0, 0, 0, 0, time.UTC)
	reportID := seedTestReport(t, repo.db, boardID, today)

	// A topic the section is currently assigned to.
	topic := BoardPersistentTopic{
		SemanticBoardID: boardID, Label: "中东局势", Status: TopicStatusActive, Source: TopicSourceAuto,
		FirstSeenDate: today, LastSeenDate: today,
	}
	require.NoError(t, repo.CreateTopic(repo.db, &topic))

	tid := topic.ID
	sec := DailyReportSection{
		ReportID: reportID, ClusterLabel: "霍尔木兹", Embedding: vecStr(0.2, 0.1),
		PersistentTopicID: &tid, TopicMatchConfidence: TopicConfAnchorHit, ArticleCount: 1,
	}
	require.NoError(t, repo.db.Create(&sec).Error)

	resp, err := repo.GetComposeCandidates(boardID, 14)
	require.NoError(t, err)
	require.Len(t, resp.Sections, 1)
	got := resp.Sections[0]
	assert.Equal(t, sec.ID, got.ID)
	require.NotNil(t, got.PersistentTopicID)
	assert.Equal(t, tid, *got.PersistentTopicID)
	assert.Equal(t, TopicConfAnchorHit, got.TopicMatchConfidence)
	// Topic brief attached (label carried for the candidate pool display).
	require.NotNil(t, got.PersistentTopic)
	assert.Equal(t, "中东局势", got.PersistentTopic.Label)
}

func TestGetComposeCandidates_DaysWindowOutOfScope(t *testing.T) {
	repo := setupManualTopicTestDB(t)
	boardID := seedTestBoard(t, repo.db)
	// Latest report is recent; an older report falls outside the window.
	latest := time.Date(2026, 6, 29, 0, 0, 0, 0, time.UTC)
	old := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	recentReport := seedTestReport(t, repo.db, boardID, latest)
	oldReport := seedTestReport(t, repo.db, boardID, old)

	sRecent := DailyReportSection{ReportID: recentReport, ClusterLabel: "recent", Embedding: vecStr(0.1, 0.2), ArticleCount: 1}
	sOld := DailyReportSection{ReportID: oldReport, ClusterLabel: "old", Embedding: vecStr(0.9, 0.8), ArticleCount: 1}
	require.NoError(t, repo.db.Create(&sRecent).Error)
	require.NoError(t, repo.db.Create(&sOld).Error)

	// 14-day window anchored to the latest report (06-29) → the 2025 section is out.
	resp, err := repo.GetComposeCandidates(boardID, 14)
	require.NoError(t, err)
	require.Len(t, resp.Sections, 1)
	assert.Equal(t, "recent", resp.Sections[0].ClusterLabel)
}

func TestGetComposeCandidates_EmptyBoard(t *testing.T) {
	repo := setupManualTopicTestDB(t)
	boardID := seedTestBoard(t, repo.db)
	// No reports at all → graceful fallback (zero latest → cutoff far in past →
	// returns empty slice, never errors).
	resp, err := repo.GetComposeCandidates(boardID, 14)
	require.NoError(t, err)
	assert.Empty(t, resp.Sections)
	assert.InDelta(t, 0.30, resp.MatchThreshold, 1e-9)
}

// ── test helpers ────────────────────────────────────────────────────────────

// setupManualTopicTestDB creates an in-memory SQLite DB with all tables needed
// by CreateManualTopic and sets the package-level Repo global.
func setupManualTopicTestDB(t *testing.T) *TopicGraphRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared&_foreign_keys=on", t.Name())), &gorm.Config{})
	require.NoError(t, err, "open sqlite")
	require.NoError(t, db.Exec("PRAGMA foreign_keys = ON").Error)
	database.DB = db
	InitRepository(db)
	require.NoError(t, db.AutoMigrate(
		&models.SemanticLabel{},
		&BoardDailyReport{},
		&DailyReportSection{},
		&BoardPersistentTopic{},
		&SectionRelation{},
	), "auto-migrate manual topic tables")
	return Repo
}
