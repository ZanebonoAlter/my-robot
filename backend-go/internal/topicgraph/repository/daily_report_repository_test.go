package repository

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"syntopica-backend/internal/models"
	"syntopica-backend/internal/platform/testutil"
)

func TestNormalizeReportDateKeepsRequestedDate(t *testing.T) {
	requested, err := time.ParseInLocation("2006-01-02", "2026-05-26", models.ShanghaiTZ)
	require.NoError(t, err)

	got := NormalizeReportDate(requested)

	require.Equal(t, "2026-05-26", got.Format("2006-01-02"))
	require.Equal(t, time.UTC, got.Location())
	require.Equal(t, 12, got.Hour())
}

func TestListReports_DefaultDays(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewTopicGraphRepository(db)

	boardID := seedTestBoard(t, db)
	now := NormalizeReportDate(time.Now())

	// Seed reports: today, 3 days ago, 10 days ago
	seedTestReport(t, db, boardID, now)
	seedTestReport(t, db, boardID, now.AddDate(0, 0, -3))
	seedTestReport(t, db, boardID, now.AddDate(0, 0, -10))

	// days=0 should default to 7, returning today and 3 days ago
	items, err := repo.ListReports(boardID, 0)
	require.NoError(t, err)
	require.Len(t, items, 2)
	require.Equal(t, now.Format("2006-01-02"), items[0].PeriodDate)
	require.Equal(t, now.AddDate(0, 0, -3).Format("2006-01-02"), items[1].PeriodDate)
}

func TestListReports_SevenDays(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewTopicGraphRepository(db)

	boardID := seedTestBoard(t, db)
	now := NormalizeReportDate(time.Now())

	seedTestReport(t, db, boardID, now)
	seedTestReport(t, db, boardID, now.AddDate(0, 0, -5))
	seedTestReport(t, db, boardID, now.AddDate(0, 0, -10))

	items, err := repo.ListReports(boardID, 7)
	require.NoError(t, err)
	require.Len(t, items, 2) // 10 days ago is outside 7-day window
	require.Equal(t, now.Format("2006-01-02"), items[0].PeriodDate)
}

func TestListReports_FortyTwoDays(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewTopicGraphRepository(db)

	boardID := seedTestBoard(t, db)
	now := NormalizeReportDate(time.Now())

	seedTestReport(t, db, boardID, now)
	seedTestReport(t, db, boardID, now.AddDate(0, 0, -20))
	seedTestReport(t, db, boardID, now.AddDate(0, 0, -40))
	seedTestReport(t, db, boardID, now.AddDate(0, 0, -50)) // outside 42 days

	// days=42 should NOT be truncated to 30
	items, err := repo.ListReports(boardID, 42)
	require.NoError(t, err)
	require.Len(t, items, 3) // 50 days ago is outside 42-day window
	// Verify descending order
	require.Equal(t, now.Format("2006-01-02"), items[0].PeriodDate)
	require.Equal(t, now.AddDate(0, 0, -20).Format("2006-01-02"), items[1].PeriodDate)
	require.Equal(t, now.AddDate(0, 0, -40).Format("2006-01-02"), items[2].PeriodDate)
}

func seedTestBoard(t *testing.T, db *gorm.DB) uint {
	t.Helper()
	board := models.SemanticLabel{
		Label:     "test-board",
		Slug:      "test-board",
		LabelType: "board",
		Status:    "active",
	}
	err := db.Create(&board).Error
	require.NoError(t, err)
	return board.ID
}

func seedTestReport(t *testing.T, db *gorm.DB, boardID uint, date time.Time) uint {
	t.Helper()
	report := BoardDailyReport{
		SemanticBoardID: boardID,
		PeriodDate:      NormalizeReportDate(date),
		Title:           "Test Report",
		Summary:         "Test Summary",
		Status:          "completed",
	}
	err := db.Create(&report).Error
	require.NoError(t, err)
	return report.ID
}

func seedTestSection(t *testing.T, db *gorm.DB, reportID uint, label string) uint {
	t.Helper()
	// daily_report_sections.embedding is a non-nullable `vector` column (model field
	// is `string`, zero value ""); pgvector rejects the empty string. Production
	// always populates Embedding via FloatsToPgVector before insert, so mirror
	// that here. The column is untyped-dimension in the test DB (runtime
	// EnsureVectorDimensionOnce is never invoked), so a 1-dim vector is accepted.
	section := DailyReportSection{
		ReportID:     reportID,
		ClusterLabel: label,
		ArticleCount: 1,
		Embedding:    FloatsToPgVector([]float64{0}),
	}
	err := db.Create(&section).Error
	require.NoError(t, err)
	return section.ID
}

func seedTestRelation(t *testing.T, db *gorm.DB, fromID, toID uint, distance float64) {
	t.Helper()
	rel := SectionRelation{
		FromSectionID: fromID,
		ToSectionID:   toID,
		Distance:      distance,
	}
	err := db.Create(&rel).Error
	require.NoError(t, err)
}

func TestGetSectionLifecycle_MultiHopChain(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewTopicGraphRepository(db)

	boardID := seedTestBoard(t, db)
	now := NormalizeReportDate(time.Now())

	r1 := seedTestReport(t, db, boardID, now.AddDate(0, 0, -2))
	r2 := seedTestReport(t, db, boardID, now.AddDate(0, 0, -1))
	r3 := seedTestReport(t, db, boardID, now)

	// #40 -> #50 -> #60
	s40 := seedTestSection(t, db, r1, "section-40")
	s50 := seedTestSection(t, db, r2, "section-50")
	s60 := seedTestSection(t, db, r3, "section-60")

	seedTestRelation(t, db, s40, s50, 0.8)
	seedTestRelation(t, db, s50, s60, 0.7)

	// Query from s60 should return all 3 sections
	resp, err := repo.GetSectionLifecycle(s60)
	require.NoError(t, err)
	require.Len(t, resp.Sections, 3)
	require.Len(t, resp.Relations, 2)
}

func TestGetSectionLifecycle_Fork(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewTopicGraphRepository(db)

	boardID := seedTestBoard(t, db)
	now := NormalizeReportDate(time.Now())

	r1 := seedTestReport(t, db, boardID, now.AddDate(0, 0, -2))
	r2 := seedTestReport(t, db, boardID, now.AddDate(0, 0, -1))

	// #40 -> #50, #50 -> #60, #50 -> #61
	s40 := seedTestSection(t, db, r1, "section-40")
	s50 := seedTestSection(t, db, r2, "section-50")
	s60 := seedTestSection(t, db, r2, "section-60")
	s61 := seedTestSection(t, db, r2, "section-61")

	seedTestRelation(t, db, s40, s50, 0.8)
	seedTestRelation(t, db, s50, s60, 0.7)
	seedTestRelation(t, db, s50, s61, 0.6)

	// Query from s60 should return all 4 sections and 3 relations
	resp, err := repo.GetSectionLifecycle(s60)
	require.NoError(t, err)
	require.Len(t, resp.Sections, 4)
	require.Len(t, resp.Relations, 3)
}

func TestGetSectionLifecycle_Merge(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewTopicGraphRepository(db)

	boardID := seedTestBoard(t, db)
	now := NormalizeReportDate(time.Now())

	r1 := seedTestReport(t, db, boardID, now.AddDate(0, 0, -2))
	r2 := seedTestReport(t, db, boardID, now.AddDate(0, 0, -1))

	// #50 -> #70, #55 -> #70, #70 -> #80
	s50 := seedTestSection(t, db, r1, "section-50")
	s55 := seedTestSection(t, db, r1, "section-55")
	s70 := seedTestSection(t, db, r2, "section-70")
	s80 := seedTestSection(t, db, r2, "section-80")

	seedTestRelation(t, db, s50, s70, 0.8)
	seedTestRelation(t, db, s55, s70, 0.7)
	seedTestRelation(t, db, s70, s80, 0.6)

	// Query from s50 should return all 4 sections and 3 relations
	resp, err := repo.GetSectionLifecycle(s50)
	require.NoError(t, err)
	require.Len(t, resp.Sections, 4)
	require.Len(t, resp.Relations, 3)
}

func TestGetSectionLifecycle_IsolatedSection(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewTopicGraphRepository(db)

	boardID := seedTestBoard(t, db)
	now := NormalizeReportDate(time.Now())

	r1 := seedTestReport(t, db, boardID, now)
	s20 := seedTestSection(t, db, r1, "section-20")

	// Isolated section: no relations
	resp, err := repo.GetSectionLifecycle(s20)
	require.NoError(t, err)
	require.Len(t, resp.Sections, 1)
	require.Equal(t, s20, resp.Sections[0].ID)
	require.Len(t, resp.Relations, 0)
}

func TestGetSectionLifecycle_Cycle(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewTopicGraphRepository(db)

	boardID := seedTestBoard(t, db)
	now := NormalizeReportDate(time.Now())

	r1 := seedTestReport(t, db, boardID, now)

	// #50 -> #60 -> #70 -> #50 (cycle)
	s50 := seedTestSection(t, db, r1, "section-50")
	s60 := seedTestSection(t, db, r1, "section-60")
	s70 := seedTestSection(t, db, r1, "section-70")

	seedTestRelation(t, db, s50, s60, 0.8)
	seedTestRelation(t, db, s60, s70, 0.7)
	seedTestRelation(t, db, s70, s50, 0.6)

	// Should return all 3 sections once and 3 relations
	resp, err := repo.GetSectionLifecycle(s50)
	require.NoError(t, err)
	require.Len(t, resp.Sections, 3)
	require.Len(t, resp.Relations, 3)
}

func TestGetSectionLifecycle_ExternalRelationIsolation(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewTopicGraphRepository(db)

	boardID := seedTestBoard(t, db)
	now := NormalizeReportDate(time.Now())

	r1 := seedTestReport(t, db, boardID, now)

	// Component A: #50 -> #60 -> #70
	s50 := seedTestSection(t, db, r1, "section-50")
	s60 := seedTestSection(t, db, r1, "section-60")
	s70 := seedTestSection(t, db, r1, "section-70")
	// Component B: #80 (isolated)
	s80 := seedTestSection(t, db, r1, "section-80")

	seedTestRelation(t, db, s50, s60, 0.8)
	seedTestRelation(t, db, s60, s70, 0.7)

	// Query from s50: should NOT include s80 or any relation involving s80
	resp, err := repo.GetSectionLifecycle(s50)
	require.NoError(t, err)
	require.Len(t, resp.Sections, 3)
	require.Len(t, resp.Relations, 2)

	// Verify no relation references s80
	for _, rel := range resp.Relations {
		require.NotEqual(t, s80, rel.FromID)
		require.NotEqual(t, s80, rel.ToID)
	}
}
