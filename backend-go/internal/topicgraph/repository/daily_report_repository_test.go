package repository

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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

func TestGetReportByID_AttachesTopicBriefs(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewTopicGraphRepository(db)

	boardID := seedTestBoard(t, db)
	now := NormalizeReportDate(time.Now())
	reportID := seedTestReport(t, db, boardID, now)
	topic := BoardPersistentTopic{
		SemanticBoardID: boardID,
		Label:           "Tracked topic",
		Embedding:       FloatsToPgVector([]float64{0}),
		Status:          TopicStatusActive,
		FirstSeenDate:   now,
		LastSeenDate:    now,
		HitCount:        3,
		ConsecutiveHits: 3,
	}
	require.NoError(t, db.Create(&topic).Error)

	sectionID := seedTestSection(t, db, reportID, "tracked section")
	require.NoError(t, db.Model(&DailyReportSection{}).
		Where("id = ?", sectionID).
		Update("persistent_topic_id", topic.ID).Error)

	report, err := repo.GetReportByID(reportID)
	require.NoError(t, err)
	require.Len(t, report.Sections, 1)
	require.NotNil(t, report.Sections[0].PersistentTopicID)
	require.Equal(t, topic.ID, *report.Sections[0].PersistentTopicID)
	require.NotNil(t, report.Sections[0].PersistentTopic)
	require.Equal(t, topic.ID, report.Sections[0].PersistentTopic.ID)
	require.Equal(t, topic.Label, report.Sections[0].PersistentTopic.Label)
	require.Equal(t, TopicStatusActive, report.Sections[0].PersistentTopic.Status)
}

func TestGetReportByID_SectionWithoutThreadsMarshalsEmptyArray(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewTopicGraphRepository(db)

	boardID := seedTestBoard(t, db)
	now := NormalizeReportDate(time.Now())
	reportID := seedTestReport(t, db, boardID, now)
	// Section with zero thread rows — mirrors the production data where the
	// per-cluster threads LLM call failed and the orchestrator degraded to
	// saving the section without threads.
	seedTestSection(t, db, reportID, "no-thread section")

	report, err := repo.GetReportByID(reportID)
	require.NoError(t, err)
	require.Len(t, report.Sections, 1)

	data, err := json.Marshal(report)
	require.NoError(t, err)
	// threads must serialize as [] (never omitted / null): the frontend reader
	// calls section.threads.filter directly (DailyReportTopicSection.vue).
	require.Contains(t, string(data), `"threads":[]`)
	require.NotNil(t, report.Sections[0].Threads)
	require.Empty(t, report.Sections[0].Threads)
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

// =============================================================================
// ListTopicRecentBriefs tests (Slice D: lane context injection)
// =============================================================================

func seedTestActiveTopic(t *testing.T, db *gorm.DB, boardID uint, label string, lastSeen time.Time) uint {
	t.Helper()
	topic := BoardPersistentTopic{
		SemanticBoardID: boardID,
		Label:           label,
		Embedding:       FloatsToPgVector([]float64{0}),
		Status:          TopicStatusActive,
		FirstSeenDate:   lastSeen,
		LastSeenDate:    lastSeen,
		HitCount:        1,
	}
	err := db.Create(&topic).Error
	require.NoError(t, err)
	return topic.ID
}

func seedTestSectionWithTopic(t *testing.T, db *gorm.DB, reportID uint, label string, topicID uint) uint {
	t.Helper()
	section := DailyReportSection{
		ReportID:          reportID,
		ClusterLabel:      label,
		ArticleCount:      1,
		Embedding:         FloatsToPgVector([]float64{0}),
		PersistentTopicID: &topicID,
	}
	err := db.Create(&section).Error
	require.NoError(t, err)
	return section.ID
}

func seedTestThread(t *testing.T, db *gorm.DB, sectionID uint, title string, fitDistance *float64) uint {
	t.Helper()
	thread := DailyReportThread{
		SectionID:   sectionID,
		Title:       title,
		Embedding:   FloatsToPgVector([]float64{0}),
		FitDistance: fitDistance,
	}
	err := db.Create(&thread).Error
	require.NoError(t, err)
	return thread.ID
}

func TestListTopicRecentBriefs_Basic(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewTopicGraphRepository(db)

	boardID := seedTestBoard(t, db)
	now := NormalizeReportDate(time.Now())

	// Active topic
	topicID := seedTestActiveTopic(t, db, boardID, "AI 编程工具平台化竞争", now)

	// Report + section with threads
	reportID := seedTestReport(t, db, boardID, now.AddDate(0, 0, -1))
	sectionID := seedTestSectionWithTopic(t, db, reportID, "开发者 Agent 平台化", topicID)

	// Threads with different fit_distances: best fit first, NULL last
	dist0 := 0.12
	dist1 := 0.35
	seedTestThread(t, db, sectionID, "Codex 推出第三方模型接入", &dist0)
	seedTestThread(t, db, sectionID, "GitHub Copilot X 发布新功能", &dist1)

	briefs, err := repo.ListTopicRecentBriefs(boardID, 7, 5)
	require.NoError(t, err)
	require.Len(t, briefs, 1)
	require.Contains(t, briefs, topicID)

	items := briefs[topicID]
	require.Len(t, items, 1)
	assert.Equal(t, sectionID, items[0].SectionID)
	assert.Equal(t, "开发者 Agent 平台化", items[0].SectionLabel)
	require.Len(t, items[0].ThreadTitles, 2)
	assert.Equal(t, "Codex 推出第三方模型接入", items[0].ThreadTitles[0]) // best fit first
	assert.Equal(t, "GitHub Copilot X 发布新功能", items[0].ThreadTitles[1])
}

func TestListTopicRecentBriefs_SevenDayWindow(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewTopicGraphRepository(db)

	boardID := seedTestBoard(t, db)
	now := NormalizeReportDate(time.Now())

	topicID := seedTestActiveTopic(t, db, boardID, "以黎冲突升级", now)

	// Section within 7 days
	r1 := seedTestReport(t, db, boardID, now.AddDate(0, 0, -3))
	s1 := seedTestSectionWithTopic(t, db, r1, "真主党越境打击", topicID)
	d0 := 0.08
	seedTestThread(t, db, s1, "真主党向以色列北部发射火箭", &d0)

	// Section outside 7 days (10 days ago)
	r2 := seedTestReport(t, db, boardID, now.AddDate(0, 0, -10))
	s2 := seedTestSectionWithTopic(t, db, r2, "以军空袭黎南部", topicID)
	d1 := 0.05
	seedTestThread(t, db, s2, "以色列空袭黎巴嫩南部目标", &d1)

	briefs, err := repo.ListTopicRecentBriefs(boardID, 7, 5)
	require.NoError(t, err)
	require.Len(t, briefs, 1)

	// Only the 3-day-ago section should be present (7-day window)
	items := briefs[topicID]
	assert.Len(t, items, 1)
	assert.Equal(t, "真主党越境打击", items[0].SectionLabel)
}

func TestListTopicRecentBriefs_PerTopicLimit(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewTopicGraphRepository(db)

	boardID := seedTestBoard(t, db)
	now := NormalizeReportDate(time.Now())

	topicID := seedTestActiveTopic(t, db, boardID, "AI 算力生态", now)

	// Create 6 sections (limit is 5), descending by date
	d0 := 0.10
	for i := 0; i < 6; i++ {
		r := seedTestReport(t, db, boardID, now.AddDate(0, 0, -(i+1)))
		s := seedTestSectionWithTopic(t, db, r, fmt.Sprintf("section-%d", i+1), topicID)
		seedTestThread(t, db, s, fmt.Sprintf("thread-%d", i+1), &d0)
	}

	briefs, err := repo.ListTopicRecentBriefs(boardID, 7, 5)
	require.NoError(t, err)
	require.Len(t, briefs, 1)

	// Should be truncated to 5 newest sections
	items := briefs[topicID]
	assert.Len(t, items, 5, "per-topic limit = 5")
	// Verify newest first (by period_date DESC)
	assert.Equal(t, "section-1", items[0].SectionLabel)
	assert.Equal(t, "section-5", items[4].SectionLabel)
}

func TestListTopicRecentBriefs_ThreadFitDistanceOrdering(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewTopicGraphRepository(db)

	boardID := seedTestBoard(t, db)
	now := NormalizeReportDate(time.Now())

	topicID := seedTestActiveTopic(t, db, boardID, "中东局势", now)
	reportID := seedTestReport(t, db, boardID, now)
	sectionID := seedTestSectionWithTopic(t, db, reportID, "中东冲突升级", topicID)

	// Threads: 3 threads with different fit_distances; LIMIT 2 returns the two smallest
	dLarge := 0.95
	dSmall := 0.15
	var dMedium float64 = 0.50
	seedTestThread(t, db, sectionID, "thread-large-dist", &dLarge)
	seedTestThread(t, db, sectionID, "thread-small-dist", &dSmall)
	seedTestThread(t, db, sectionID, "thread-medium-dist-excluded", &dMedium)

	briefs, err := repo.ListTopicRecentBriefs(boardID, 7, 5)
	require.NoError(t, err)
	require.Len(t, briefs, 1)

	items := briefs[topicID]
	require.Len(t, items, 1)
	require.Len(t, items[0].ThreadTitles, 2, "LIMIT 2 per section")
	// Smallest fit_distance first, then next smallest; largest (0.95) excluded by LIMIT
	assert.Equal(t, "thread-small-dist", items[0].ThreadTitles[0])
	assert.Equal(t, "thread-medium-dist-excluded", items[0].ThreadTitles[1])
}

func TestListTopicRecentBriefs_CandidateExcluded(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewTopicGraphRepository(db)

	boardID := seedTestBoard(t, db)
	now := NormalizeReportDate(time.Now())

	// Candidate topic — should NOT appear in briefs
	candidate := BoardPersistentTopic{
		SemanticBoardID: boardID,
		Label:           "候选话题",
		Embedding:       FloatsToPgVector([]float64{0}),
		Status:          TopicStatusCandidate,
		FirstSeenDate:   now,
		LastSeenDate:    now,
		HitCount:        1,
	}
	require.NoError(t, db.Create(&candidate).Error)

	reportID := seedTestReport(t, db, boardID, now)
	_ = seedTestSectionWithTopic(t, db, reportID, "candidate-section", candidate.ID)

	briefs, err := repo.ListTopicRecentBriefs(boardID, 7, 5)
	require.NoError(t, err)
	assert.Len(t, briefs, 0, "candidate topics excluded from briefs")
}

func TestListTopicRecentBriefs_NoActiveTopics(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewTopicGraphRepository(db)

	boardID := seedTestBoard(t, db)

	briefs, err := repo.ListTopicRecentBriefs(boardID, 7, 5)
	require.NoError(t, err)
	assert.Nil(t, briefs)
}

func TestListTopicRecentBriefs_MultipleTopics(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewTopicGraphRepository(db)

	boardID := seedTestBoard(t, db)
	now := NormalizeReportDate(time.Now())

	topicA := seedTestActiveTopic(t, db, boardID, "Topic A", now)
	topicB := seedTestActiveTopic(t, db, boardID, "Topic B", now)

	r := seedTestReport(t, db, boardID, now)
	sA := seedTestSectionWithTopic(t, db, r, "Section A", topicA)
	sB := seedTestSectionWithTopic(t, db, r, "Section B", topicB)

	d0 := 0.05
	seedTestThread(t, db, sA, "Thread A1", &d0)
	seedTestThread(t, db, sB, "Thread B1", &d0)

	briefs, err := repo.ListTopicRecentBriefs(boardID, 7, 5)
	require.NoError(t, err)
	assert.Len(t, briefs, 2)
	assert.Contains(t, briefs, topicA)
	assert.Contains(t, briefs, topicB)
	assert.Len(t, briefs[topicA], 1)
	assert.Len(t, briefs[topicB], 1)
}
