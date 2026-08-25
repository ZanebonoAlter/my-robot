package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"syntopica-backend/internal/models"
	"syntopica-backend/internal/platform/airouter"
	"syntopica-backend/internal/platform/testutil"
	"syntopica-backend/internal/topicgraph/repository"
)

// fakeWatchChat returns a watchChatFunc that responds with the given JSON
// string, bypassing the real AI provider.
func fakeWatchChat(jsonResp string) watchChatFunc {
	return func(_ context.Context, _ airouter.ChatRequest) (*airouter.ChatResult, error) {
		return &airouter.ChatResult{Content: jsonResp}, nil
	}
}

// TestWatchHitIntegration_DoesNotChangePersistentTopicID asserts that
// EvaluateWatchHits writes a watch hit row BUT does NOT alter the matched
// section's persistent_topic_id (keeping the invariant: watch hits are a
// read-only overlay).
func TestWatchHitIntegration_DoesNotChangePersistentTopicID(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repository.Repo = repository.NewTopicGraphRepository(db)

	boardID := seedBoard(t, db)
	now := repository.NormalizeReportDate(time.Now())

	// Topic #8 to which the section belongs.
	topic := repository.BoardPersistentTopic{
		SemanticBoardID: boardID,
		Label:           "Persistent Topic 8",
		Embedding:       repository.FloatsToPgVector([]float64{0}),
		Status:          repository.TopicStatusActive,
		FirstSeenDate:   now,
		LastSeenDate:    now,
		HitCount:        3,
		ConsecutiveHits: 2,
	}
	require.NoError(t, db.Create(&topic).Error)
	topicID := topic.ID // topic #8

	// Active watch.
	watch := repository.BoardTopicWatch{
		SemanticBoardID: boardID,
		Label:           "Test Watch X",
		Status:          repository.WatchStatusActive,
	}
	require.NoError(t, db.Create(&watch).Error)

	// Report.
	report := repository.BoardDailyReport{
		SemanticBoardID: boardID,
		PeriodDate:      now,
		Title:           "Test Report - Watch Hit Integration",
		Status:          "completed",
	}
	require.NoError(t, db.Create(&report).Error)

	// Section with persistent_topic_id=8.
	sec := repository.DailyReportSection{
		ReportID:          report.ID,
		ClusterLabel:      "Section under topic 8",
		ArticleCount:      1,
		Embedding:         repository.FloatsToPgVector([]float64{0}),
		PersistentTopicID: &topicID,
	}
	require.NoError(t, db.Create(&sec).Error)

	// Fake chat: hits the section with our watch.
	respJSON := fmt.Sprintf(`{"hits":[{"watch_id":%d,"section_id":%d,"reason":"test hit"}]}`, watch.ID, sec.ID)
	chat := fakeWatchChat(respJSON)

	ctx := context.Background()
	err := evaluateWatchHitsWithChat(ctx, boardID, &report, []repository.DailyReportSection{sec}, chat)
	require.NoError(t, err, "evaluateWatchHitsWithChat must not error")

	// Assert: topic_watch_hits has 1 row.
	var hitCount int64
	db.Model(&repository.TopicWatchHit{}).Count(&hitCount)
	assert.Equal(t, int64(1), hitCount, "one watch hit must be written")

	// Assert: section's persistent_topic_id is STILL 8 (unchanged).
	var reloaded repository.DailyReportSection
	require.NoError(t, db.First(&reloaded, sec.ID).Error)
	require.NotNil(t, reloaded.PersistentTopicID,
		"persistent_topic_id must not be nil after watch hit")
	assert.Equal(t, topic.ID, *reloaded.PersistentTopicID,
		"persistent_topic_id must remain unchanged (=8) after watch hit")
}

// TestWatchHitIntegration_DoesNotAdvanceConsecutiveHits asserts that
// EvaluateWatchHits writes a watch hit row BUT does NOT increment the
// topic's consecutive_hits (keeping the lifecycle invariant).
func TestWatchHitIntegration_DoesNotAdvanceConsecutiveHits(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repository.Repo = repository.NewTopicGraphRepository(db)

	boardID := seedBoard(t, db)
	now := repository.NormalizeReportDate(time.Now())

	// Topic #8 with consecutive_hits=2.
	topic := repository.BoardPersistentTopic{
		SemanticBoardID: boardID,
		Label:           "Persistent Topic with hits",
		Embedding:       repository.FloatsToPgVector([]float64{0}),
		Status:          repository.TopicStatusActive,
		FirstSeenDate:   now,
		LastSeenDate:    now,
		HitCount:        5,
		ConsecutiveHits: 2, // the invariant value
	}
	require.NoError(t, db.Create(&topic).Error)
	topicID := topic.ID

	// Active watch.
	watch := repository.BoardTopicWatch{
		SemanticBoardID: boardID,
		Label:           "Test Watch - lifecycle",
		Status:          repository.WatchStatusActive,
	}
	require.NoError(t, db.Create(&watch).Error)

	// Report.
	report := repository.BoardDailyReport{
		SemanticBoardID: boardID,
		PeriodDate:      now,
		Title:           "Test Report - Watch Hit Lifecycle",
		Status:          "completed",
	}
	require.NoError(t, db.Create(&report).Error)

	// Section belonging to topic #8.
	sec := repository.DailyReportSection{
		ReportID:          report.ID,
		ClusterLabel:      "Section under watched topic",
		ArticleCount:      1,
		Embedding:         repository.FloatsToPgVector([]float64{0}),
		PersistentTopicID: &topicID,
	}
	require.NoError(t, db.Create(&sec).Error)

	// Fake chat: hits the section.
	respJSON := fmt.Sprintf(`{"hits":[{"watch_id":%d,"section_id":%d,"reason":"test lifecycle hit"}]}`, watch.ID, sec.ID)
	chat := fakeWatchChat(respJSON)

	ctx := context.Background()
	err := evaluateWatchHitsWithChat(ctx, boardID, &report, []repository.DailyReportSection{sec}, chat)
	require.NoError(t, err, "evaluateWatchHitsWithChat must not error")

	// Assert: topic_watch_hits has 1 row.
	var hitCount int64
	db.Model(&repository.TopicWatchHit{}).Count(&hitCount)
	assert.Equal(t, int64(1), hitCount, "one watch hit must be written")

	// Assert: topic #8's consecutive_hits is STILL 2 (unchanged).
	var reloaded repository.BoardPersistentTopic
	require.NoError(t, db.First(&reloaded, topic.ID).Error)
	assert.Equal(t, 2, reloaded.ConsecutiveHits,
		"consecutive_hits must remain unchanged (=2) after watch hit")
}

// TestWatchHitIntegration_HallucinatedWatchIDIsFiltered asserts that
// a hit referencing a watch_id not in the valid set is silently filtered
// and never written to topic_watch_hits.
func TestWatchHitIntegration_HallucinatedWatchIDIsFiltered(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repository.Repo = repository.NewTopicGraphRepository(db)

	boardID := seedBoard(t, db)
	now := repository.NormalizeReportDate(time.Now())

	// One real watch (ID=X).
	watch := repository.BoardTopicWatch{
		SemanticBoardID: boardID,
		Label:           "Real Watch",
		Status:          repository.WatchStatusActive,
	}
	require.NoError(t, db.Create(&watch).Error)

	// Report.
	report := repository.BoardDailyReport{
		SemanticBoardID: boardID,
		PeriodDate:      now,
		Title:           "Test Report - Hallucination Filter",
		Status:          "completed",
	}
	require.NoError(t, db.Create(&report).Error)

	// Section.
	sec := repository.DailyReportSection{
		ReportID:     report.ID,
		ClusterLabel: "Test section",
		ArticleCount: 1,
		Embedding:    repository.FloatsToPgVector([]float64{0}),
	}
	require.NoError(t, db.Create(&sec).Error)

	// Hallucinated watch_id=9999 does NOT exist in DB.
	respJSON := fmt.Sprintf(`{"hits":[{"watch_id":9999,"section_id":%d,"reason":"hallucinated"}]}`, sec.ID)
	chat := fakeWatchChat(respJSON)

	ctx := context.Background()
	err := evaluateWatchHitsWithChat(ctx, boardID, &report, []repository.DailyReportSection{sec}, chat)
	require.NoError(t, err, "evaluateWatchHitsWithChat must not error (hallucinations are silently filtered)")

	// Assert: NO watch hit row was written.
	var hitCount int64
	db.Model(&repository.TopicWatchHit{}).Count(&hitCount)
	assert.Equal(t, int64(0), hitCount, "no watch hit must be written for hallucinated watch_id")
}

// TestWatchHitIntegration_DuplicateHitDeduped asserts that when the AI
// returns duplicate (watch_id, section_id) pairs for the same report, the
// upsert logic silently deduplicates and writes only one row per unique key.
func TestWatchHitIntegration_DuplicateHitDeduped(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repository.Repo = repository.NewTopicGraphRepository(db)

	boardID := seedBoard(t, db)
	now := repository.NormalizeReportDate(time.Now())

	// Active watch.
	watch := repository.BoardTopicWatch{
		SemanticBoardID: boardID,
		Label:           "Watch for dedup test",
		Status:          repository.WatchStatusActive,
	}
	require.NoError(t, db.Create(&watch).Error)

	// Report.
	report := repository.BoardDailyReport{
		SemanticBoardID: boardID,
		PeriodDate:      now,
		Title:           "Test Report - Duplicate Hit Dedup",
		Status:          "completed",
	}
	require.NoError(t, db.Create(&report).Error)

	// Section.
	sec := repository.DailyReportSection{
		ReportID:     report.ID,
		ClusterLabel: "Test section for dedup",
		ArticleCount: 1,
		Embedding:    repository.FloatsToPgVector([]float64{0}),
	}
	require.NoError(t, db.Create(&sec).Error)

	// Fake chat returns TWO hits with the same (watch_id, section_id).
	respJSON := fmt.Sprintf(`{"hits":[
		{"watch_id":%d,"section_id":%d,"reason":"first hit"},
		{"watch_id":%d,"section_id":%d,"reason":"duplicate hit"}
	]}`, watch.ID, sec.ID, watch.ID, sec.ID)
	chat := fakeWatchChat(respJSON)

	ctx := context.Background()
	err := evaluateWatchHitsWithChat(ctx, boardID, &report, []repository.DailyReportSection{sec}, chat)
	require.NoError(t, err, "evaluateWatchHitsWithChat must not error on duplicates")

	// Assert: only 1 row in topic_watch_hits (duplicate silently skipped).
	var hitCount int64
	db.Model(&repository.TopicWatchHit{}).Count(&hitCount)
	assert.Equal(t, int64(1), hitCount, "duplicate must be deduplicated: only 1 row")

	// Assert: the written row has the expected values.
	var hitRows []repository.TopicWatchHit
	require.NoError(t, db.Where("report_id = ?", report.ID).Find(&hitRows).Error)
	if assert.Len(t, hitRows, 1) {
		assert.Equal(t, watch.ID, hitRows[0].WatchID)
		assert.Equal(t, sec.ID, hitRows[0].SectionID)
	}
}

// TestMigration_WatchHitUniqueIndex_Idempotent verifies that the
// CREATE UNIQUE INDEX IF NOT EXISTS statement can be executed twice
// without error (migration idempotency).
func TestMigration_WatchHitUniqueIndex_Idempotent(t *testing.T) {
	db := testutil.SetupTestDB(t)

	sql := `CREATE UNIQUE INDEX IF NOT EXISTS idx_watch_section_report ON topic_watch_hits(watch_id, section_id, report_id)`

	// First execution creates the index.
	require.NoError(t, db.Exec(sql).Error, "first create index must succeed")

	// Second execution no-ops without error.
	require.NoError(t, db.Exec(sql).Error, "second create index (IF NOT EXISTS) must succeed")

	// Verify the index exists.
	var count int64
	require.NoError(t, db.Raw(`SELECT count(*) FROM pg_indexes WHERE indexname = 'idx_watch_section_report'`).Scan(&count).Error)
	assert.Equal(t, int64(1), count, "index must exist")
}

// seedBoard creates a test semantic board and returns its ID.
func seedBoard(t *testing.T, db *gorm.DB) uint {
	t.Helper()
	board := models.SemanticLabel{
		Label:     "test-board-watch",
		Slug:      "test-board-watch",
		LabelType: "board",
		Status:    "active",
	}
	require.NoError(t, db.Create(&board).Error)
	return board.ID
}

// ── watch-keyword-and-quickadd: keyword track integration (PG) ───────────────

// seedKeywordReport seeds one report + section + thread whose text contains
// the given words. Returns (report, section).
func seedKeywordReport(t *testing.T, db *gorm.DB, boardID uint, daysAgo int, threadTitle, threadSummary string) (*repository.BoardDailyReport, *repository.DailyReportSection) {
	t.Helper()
	date := repository.NormalizeReportDate(time.Now()).AddDate(0, 0, -daysAgo)
	report := repository.BoardDailyReport{
		SemanticBoardID: boardID,
		PeriodDate:      date,
		Title:           "keyword integration report",
		Status:          "completed",
	}
	require.NoError(t, db.Create(&report).Error)
	sec := repository.DailyReportSection{
		ReportID:     report.ID,
		ClusterLabel: "section " + threadTitle,
		Embedding:    repository.FloatsToPgVector([]float64{0}),
	}
	require.NoError(t, db.Create(&sec).Error)
	require.NoError(t, db.Create(&repository.DailyReportThread{
		ReportID:  report.ID,
		SectionID: sec.ID,
		Title:     threadTitle,
		Summary:   threadSummary,
		// Non-pointer string vector column: zero-value "" is rejected by
		// pgvector (testing.md pitfall — same class as DailyReportSection).
		Embedding: repository.FloatsToPgVector([]float64{0}),
	}).Error)
	return &report, &sec
}

// keyword 命中不改 section.persistent_topic_id（S4 不变量，keyword fixture 复测）。
func TestWatchKeywordIntegration_DoesNotChangePersistentTopicID(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repository.Repo = repository.NewTopicGraphRepository(db)

	boardID := seedBoard(t, db)
	now := repository.NormalizeReportDate(time.Now())

	topic := repository.BoardPersistentTopic{
		SemanticBoardID: boardID,
		Label:           "Persistent Topic KW",
		Embedding:       repository.FloatsToPgVector([]float64{0}),
		Status:          repository.TopicStatusActive,
		FirstSeenDate:   now,
		LastSeenDate:    now,
		HitCount:        3,
		ConsecutiveHits: 2,
	}
	require.NoError(t, db.Create(&topic).Error)
	topicID := topic.ID

	watch := repository.BoardTopicWatch{
		SemanticBoardID: boardID,
		Label:           "ASML|镓锗 出口",
		Type:            repository.WatchTypeKeyword,
		Status:          repository.WatchStatusActive,
	}
	require.NoError(t, db.Create(&watch).Error)

	report, sec := seedKeywordReport(t, db, boardID, 0, "ASML 光刻机", "对华出口政策收紧")
	sec.PersistentTopicID = &topicID
	require.NoError(t, db.Model(&repository.DailyReportSection{}).Where("id = ?", sec.ID).Update("persistent_topic_id", topicID).Error)

	// Keyword-only evaluation: chat must never be invoked.
	chatCalled := false
	boom := func(_ context.Context, _ airouter.ChatRequest) (*airouter.ChatResult, error) {
		chatCalled = true
		return &airouter.ChatResult{Content: `{"hits":[]}`}, nil
	}
	require.NoError(t, evaluateWatchHitsWithChat(context.Background(), boardID, report, []repository.DailyReportSection{*sec}, boom))
	assert.False(t, chatCalled, "keyword track must not call AI")

	var hitCount int64
	db.Model(&repository.TopicWatchHit{}).Where("watch_id = ?", watch.ID).Count(&hitCount)
	assert.Equal(t, int64(1), hitCount, "keyword hit must be written")

	var reloaded repository.DailyReportSection
	require.NoError(t, db.First(&reloaded, sec.ID).Error)
	require.NotNil(t, reloaded.PersistentTopicID)
	assert.Equal(t, topicID, *reloaded.PersistentTopicID, "persistent_topic_id must remain unchanged")

	// keyword 命中不推进 topic 生命周期（consecutive_hits 不变）。
	var reloadedTopic repository.BoardPersistentTopic
	require.NoError(t, db.First(&reloadedTopic, topic.ID).Error)
	assert.Equal(t, 2, reloadedTopic.ConsecutiveHits, "consecutive_hits must remain unchanged")
	assert.Equal(t, 3, reloadedTopic.HitCount, "hit_count must remain unchanged")
}

// 两类命中合并写表 + 复合唯一索引去重（label AI hit 与 keyword text hit 同表共存）。
func TestWatchIntegration_BothTracksMergeAndDedupe(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repository.Repo = repository.NewTopicGraphRepository(db)

	boardID := seedBoard(t, db)

	labelWatch := repository.BoardTopicWatch{
		SemanticBoardID: boardID,
		Label:           "半导体",
		Type:            repository.WatchTypeLabel,
		Status:          repository.WatchStatusActive,
	}
	require.NoError(t, db.Create(&labelWatch).Error)
	kwWatch := repository.BoardTopicWatch{
		SemanticBoardID: boardID,
		Label:           "ASML",
		Type:            repository.WatchTypeKeyword,
		Status:          repository.WatchStatusActive,
	}
	require.NoError(t, db.Create(&kwWatch).Error)

	report, sec := seedKeywordReport(t, db, boardID, 0, "ASML 光刻机", "半导体出口管制")

	chat := fakeWatchChat(fmt.Sprintf(`{"hits":[{"watch_id":%d,"section_id":%d,"reason":"AI hit"}]}`, labelWatch.ID, sec.ID))
	require.NoError(t, evaluateWatchHitsWithChat(context.Background(), boardID, report, []repository.DailyReportSection{*sec}, chat))

	var labelHits, kwHits []repository.TopicWatchHit
	require.NoError(t, db.Where("watch_id = ?", labelWatch.ID).Find(&labelHits).Error)
	require.NoError(t, db.Where("watch_id = ?", kwWatch.ID).Find(&kwHits).Error)
	require.Len(t, labelHits, 1, "label (AI) hit written")
	require.Len(t, kwHits, 1, "keyword (text) hit written")
	assert.Equal(t, "AI hit", labelHits[0].Reason)
	assert.Equal(t, "含关键字『ASML』", kwHits[0].Reason)

	// 同期重跑（同日覆盖式重建后 EvaluateWatchHits 再跑一遍）：两类命中
	// 各自保持 1 行（复合唯一索引去重）。
	require.NoError(t, evaluateWatchHitsWithChat(context.Background(), boardID, report, []repository.DailyReportSection{*sec}, chat))
	var total int64
	db.Model(&repository.TopicWatchHit{}).Where("report_id = ?", report.ID).Count(&total)
	assert.Equal(t, int64(2), total, "re-run must not duplicate rows (label=1 + keyword=1)")
}

// 建关注后立即命中历史 section；即时匹配与日报匹配幂等去重（同一
// (watch_id, section_id, report_id) 只一行）。
func TestWatchKeywordIntegration_InstantMatchHitsAndIdempotentWithDaily(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repository.Repo = repository.NewTopicGraphRepository(db)

	boardID := seedBoard(t, db)
	_, sec7 := seedKeywordReport(t, db, boardID, 7, "ASML 十四天内", "光刻机维护")
	report7 := &repository.BoardDailyReport{}
	require.NoError(t, db.First(report7, sec7.ReportID).Error)
	_, sec25 := seedKeywordReport(t, db, boardID, 25, "ASML 窗口外", "不会命中")
	_ = sec25

	watch := repository.BoardTopicWatch{
		SemanticBoardID: boardID,
		Label:           "ASML",
		Type:            repository.WatchTypeKeyword,
		Status:          repository.WatchStatusActive,
	}
	require.NoError(t, db.Create(&watch).Error)

	// Instant match over the last 14 days: day-7 report hits, day-25 does not.
	n, err := MatchKeywordInstant(context.Background(), boardID, watch.ID, KeywordInstantWindowDays)
	require.NoError(t, err)
	assert.Equal(t, 1, n, "only the in-window section may hit")

	var hitRows []repository.TopicWatchHit
	require.NoError(t, db.Where("watch_id = ?", watch.ID).Find(&hitRows).Error)
	require.Len(t, hitRows, 1)
	assert.Equal(t, sec7.ID, hitRows[0].SectionID)
	assert.Equal(t, report7.ID, hitRows[0].ReportID, "hit must reference the section's owning report")
	assert.Equal(t, report7.PeriodDate, hitRows[0].PeriodDate, "period_date must be the section's report date")
	assert.Equal(t, "含关键字『ASML』", hitRows[0].Reason)

	// Daily-report-time evaluation of the same report: the unique index
	// dedupes with the instant-written row (exactly one row remains).
	chat := fakeWatchChat(`{"hits":[]}`)
	require.NoError(t, evaluateWatchHitsWithChat(context.Background(), boardID, report7, []repository.DailyReportSection{*sec7}, chat))
	require.NoError(t, db.Where("watch_id = ?", watch.ID).Find(&hitRows).Error)
	assert.Len(t, hitRows, 1, "instant + daily match must dedupe to one row (I2)")
}
