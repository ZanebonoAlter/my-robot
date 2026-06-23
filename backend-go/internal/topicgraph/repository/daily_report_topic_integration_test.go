package repository

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"syntopica-backend/internal/platform/testutil"
)

// seedTopicSection inserts a section with a real (test) embedding and a
// cluster_label, returning its id. Embeddings are 3-dim; the test DB vector
// column is untyped-dimension so this is accepted.
func seedTopicSection(t *testing.T, db *gorm.DB, reportID uint, label string, embedding string) uint {
	t.Helper()
	sec := DailyReportSection{
		ReportID:     reportID,
		ClusterLabel: label,
		ArticleCount: 1,
		Embedding:    embedding,
	}
	require.NoError(t, db.Create(&sec).Error)
	return sec.ID
}

// TestBackfill_GroupsDriftedLabelsIntoOneTopic is the headline behaviour test:
// the same narrative reported under drifted labels across days ("AI 编程竞争" /
// "开发者生态重构" / "AI 工具内卷") should collapse into a SINGLE persistent
// topic after backfill, not three. This is the core value proposition and the
// thing real data must be checked against (see verification-report).
func TestBackfill_GroupsDriftedLabelsIntoOneTopic(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewTopicGraphRepository(db)
	Repo = repo // assignAndUpdateTopics / backfill use the package global

	boardID := seedTestBoard(t, db)
	// Three near-identical 3-dim embeddings (the "same story" signal); one
	// orthogonal outlier that must seed its own topic.
	sameStory := vecStr(1.0, 0.0, 0.0)
	d1 := NormalizeReportDate(time.Now().AddDate(0, 0, -2))
	d2 := NormalizeReportDate(time.Now().AddDate(0, 0, -1))
	d3 := NormalizeReportDate(time.Now())
	r1 := seedTestReport(t, db, boardID, d1)
	r2 := seedTestReport(t, db, boardID, d2)
	r3 := seedTestReport(t, db, boardID, d3)
	seedTopicSection(t, db, r1, "AI 编程竞争", sameStory)
	seedTopicSection(t, db, r2, "开发者生态重构", vecStr(0.99, 0.01, 0.0)) // ~same
	seedTopicSection(t, db, r3, "AI 工具内卷", vecStr(0.98, 0.0, 0.02)) // ~same

	created, err := repo.BackfillPersistentTopics(boardID)
	require.NoError(t, err)
	// All three drifted labels are within ClusterThreshold (0.30) of each
	// other → one topic, not three.
	require.Equal(t, 1, created, "drifted labels of the same story should merge into one topic")

	// Every section must be assigned (no orphans).
	var unassigned int64
	db.Model(&DailyReportSection{}).Where("persistent_topic_id IS NULL").Count(&unassigned)
	assert.Equal(t, int64(0), unassigned, "all sections must be assigned after backfill")

	// Exactly one active topic for this board.
	topics, err := repo.ListAllTopicsByBoard(boardID)
	require.NoError(t, err)
	require.Len(t, topics, 1)
	assert.Equal(t, TopicStatusActive, topics[0].Status)
	assert.Equal(t, 3, topics[0].HitCount, "hit_count should equal member count")
}

// TestBackfill_SeparatesDistinctNarratives ensures orthogonal stories do not
// get merged. Without this, a too-loose threshold would lump everything
// together — the opposite failure mode of the drift test above.
func TestBackfill_SeparatesDistinctNarratives(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewTopicGraphRepository(db)
	Repo = repo

	boardID := seedTestBoard(t, db)
	d := NormalizeReportDate(time.Now())
	r := seedTestReport(t, db, boardID, d)
	seedTopicSection(t, db, r, "AI 编程", vecStr(1.0, 0.0, 0.0))
	seedTopicSection(t, db, r, "中东局势", vecStr(0.0, 1.0, 0.0)) // orthogonal

	created, err := repo.BackfillPersistentTopics(boardID)
	require.NoError(t, err)
	assert.Equal(t, 2, created, "orthogonal narratives must seed separate topics")
}

// TestIdentityEdge_SurvivesLabelDrift verifies the root-cause-B fix end to end:
// two sections of the SAME topic on adjacent days, but whose embedding
// distance EXCEEDS the Hungarian 0.28 penalty, must still be connected by an
// identity edge after relation rebuild. Before this change such a pair would
// have been broken into emerging+ending.
//
// The sections are pre-assigned to one topic (mirroring the steady state after
// a few days of daily assignment), so this isolates the identity-edge behaviour
// from the assignment/clustering thresholds (those are covered by the backfill
// tests above).
func TestIdentityEdge_SurvivesLabelDrift(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewTopicGraphRepository(db)
	Repo = repo

	boardID := seedTestBoard(t, db)
	d1 := NormalizeReportDate(time.Now().AddDate(0, 0, -1))
	d2 := NormalizeReportDate(time.Now())
	r1 := seedTestReport(t, db, boardID, d1)
	r2 := seedTestReport(t, db, boardID, d2)
	// Two drifted labels whose distance (0.32) EXCEEDS MatchPenalty (0.28).
	s1 := seedTopicSection(t, db, r1, "Day1 label", vecStr(1.0, 0.0, 0.0))
	s2 := seedTopicSection(t, db, r2, "Day2 label", vecStr(0.68, 0.0, 0.73))

	// Force both sections onto one topic (the steady state the daily pipeline
	// reaches once a topic is established).
	topic := BoardPersistentTopic{
		SemanticBoardID: boardID, Label: "drifted story",
		Embedding: vecStr(1.0, 0.0, 0.0), Status: TopicStatusActive,
		FirstSeenDate: d1, LastSeenDate: d2, HitCount: 2, ConsecutiveHits: 2,
	}
	require.NoError(t, db.Create(&topic).Error)
	require.NoError(t, db.Model(&DailyReportSection{}).Where("id IN ?", []uint{s1, s2}).
		Updates(map[string]interface{}{"persistent_topic_id": topic.ID}).Error)

	// Rebuild relations — the Hungarian step will NOT connect s1→s2 (distance
	// 0.32 > 0.28 penalty), but the identity overlay must, because they share
	// a topic.
	require.NoError(t, RebuildBoardRelations(db, boardID))

	var rel SectionRelation
	err := db.Where("from_section_id = ? AND to_section_id = ?", s1, s2).First(&rel).Error
	require.NoError(t, err, "identity edge must connect the drifted pair despite >penalty distance")
	assert.Equal(t, "identity", rel.RelationType)
	assert.Greater(t, rel.Distance, 0.28, "identity edge distance should exceed the 0.28 penalty that would have broken the Hungarian chain")
}

// TestSaveReport_AssignsNewSections is the daily-pipeline smoke test: saving a
// report with sections that anchor to an existing topic writes the assignment
// columns and creates no spurious candidate.
func TestSaveReport_AssignsNewSections(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewTopicGraphRepository(db)
	Repo = repo

	boardID := seedTestBoard(t, db)
	// Pre-create an active topic.
	topic := BoardPersistentTopic{
		SemanticBoardID: boardID,
		Label:           "AI 编程工具平台化竞争",
		Embedding:       vecStr(1.0, 0.0, 0.0),
		Status:          TopicStatusActive,
		FirstSeenDate:   NormalizeReportDate(time.Now().AddDate(0, 0, -5)),
		LastSeenDate:    NormalizeReportDate(time.Now().AddDate(0, 0, -1)),
		HitCount:        4,
		ConsecutiveHits: 4,
	}
	require.NoError(t, db.Create(&topic).Error)
	mit := topic.ID

	report := &BoardDailyReport{
		SemanticBoardID: boardID,
		PeriodDate:      NormalizeReportDate(time.Now()),
		Title:           "today", Status: "completed",
	}
	// Section anchors: near the topic embedding AND the LLM agrees.
	sections := []DailyReportSection{{
		ClusterLabel: "AI 编程竞争", Embedding: vecStr(0.99, 0.01, 0.0),
		MatchedTopicID: &mit,
	}}

	err := repo.SaveReport(report, sections, nil)
	require.NoError(t, err)

	// Reload the saved section; it should carry the topic assignment.
	var got DailyReportSection
	require.NoError(t, db.Where("report_id = ?", report.ID).First(&got).Error)
	require.NotNil(t, got.PersistentTopicID)
	assert.Equal(t, topic.ID, *got.PersistentTopicID)
	assert.Equal(t, TopicConfAnchorHit, got.TopicMatchConfidence)

	// The existing active topic should gain a hit (consecutive 4→5).
	var updated BoardPersistentTopic
	require.NoError(t, db.First(&updated, topic.ID).Error)
	assert.Equal(t, 5, updated.ConsecutiveHits)
	assert.Equal(t, 5, updated.HitCount)
}
