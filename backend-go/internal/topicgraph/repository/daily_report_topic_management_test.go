package repository

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"syntopica-backend/internal/models"
	"syntopica-backend/internal/platform/testutil"
)

// seedTopic inserts a persistent topic row (status active) and returns its id.
func seedTopic(t *testing.T, db *gorm.DB, boardID uint, label, embedding string) uint {
	t.Helper()
	topic := BoardPersistentTopic{
		SemanticBoardID: boardID,
		Label:           label,
		Embedding:       embedding,
		Status:          TopicStatusActive,
		FirstSeenDate:   NormalizeReportDate(time.Now()),
		LastSeenDate:    NormalizeReportDate(time.Now()),
		HitCount:        1,
		ConsecutiveHits: 1,
	}
	require.NoError(t, db.Create(&topic).Error)
	return topic.ID
}

// assignSectionTopic binds a section to a topic (used to stage merge/split
// fixtures — seedTopicSection creates the row, this sets the topic pointer).
func assignSectionTopic(t *testing.T, db *gorm.DB, sectionID, topicID uint) {
	t.Helper()
	require.NoError(t, db.Model(&DailyReportSection{}).Where("id = ?", sectionID).
		Update("persistent_topic_id", topicID).Error)
}

// countSectionsByTopic returns how many sections are currently assigned to a topic.
func countSectionsByTopic(t *testing.T, db *gorm.DB, topicID uint) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(&DailyReportSection{}).Where("persistent_topic_id = ?", topicID).Count(&n).Error)
	return n
}

func TestUpdateTopic_Rename(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewTopicGraphRepository(db)
	boardID := seedTestBoard(t, db)
	topicID := seedTopic(t, db, boardID, "旧名称", vecStr(1, 0, 0))

	newLabel := "重命名后"
	got, err := repo.UpdateTopic(topicID, &newLabel, nil)
	require.NoError(t, err)
	assert.Equal(t, "重命名后", got.Label)

	// Persisted.
	var reloaded BoardPersistentTopic
	require.NoError(t, db.First(&reloaded, topicID).Error)
	assert.Equal(t, "重命名后", reloaded.Label)
}

func TestUpdateTopic_ArchiveAndReactivate(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewTopicGraphRepository(db)
	boardID := seedTestBoard(t, db)
	topicID := seedTopic(t, db, boardID, "T", vecStr(1, 0, 0))

	archived := TopicStatusArchived
	got, err := repo.UpdateTopic(topicID, nil, &archived)
	require.NoError(t, err)
	assert.Equal(t, TopicStatusArchived, got.Status)

	// Reactivate is allowed (manual override of the decay state machine).
	active := TopicStatusActive
	got, err = repo.UpdateTopic(topicID, nil, &active)
	require.NoError(t, err)
	assert.Equal(t, TopicStatusActive, got.Status)
}

func TestUpdateTopic_RejectsInvalidStatus(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewTopicGraphRepository(db)
	boardID := seedTestBoard(t, db)
	topicID := seedTopic(t, db, boardID, "T", vecStr(1, 0, 0))

	bad := "frozen"
	_, err := repo.UpdateTopic(topicID, nil, &bad)
	require.Error(t, err)
}

func TestMergeTopics_ReassignsAndArchivesSources(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewTopicGraphRepository(db)
	Repo = repo // RebuildBoardRelations reads the package global via tx helpers

	boardID := seedTestBoard(t, db)
	day := NormalizeReportDate(time.Now())
	reportID := seedTestReport(t, db, boardID, day)

	target := seedTopic(t, db, boardID, "target", vecStr(1, 0, 0))
	srcA := seedTopic(t, db, boardID, "srcA", vecStr(0.99, 0.01, 0))
	srcB := seedTopic(t, db, boardID, "srcB", vecStr(0.98, 0, 0.02))

	// target has 1 section, each source has 1.
	tSec := seedTopicSection(t, db, reportID, "t", vecStr(1, 0, 0))
	assignSectionTopic(t, db, tSec, target)
	aSec := seedTopicSection(t, db, reportID, "a", vecStr(0.99, 0.01, 0))
	assignSectionTopic(t, db, aSec, srcA)
	bSec := seedTopicSection(t, db, reportID, "b", vecStr(0.98, 0, 0.02))
	assignSectionTopic(t, db, bSec, srcB)

	require.Equal(t, int64(1), countSectionsByTopic(t, db, target))

	_, err := repo.MergeTopics(target, []uint{srcA, srcB})
	require.NoError(t, err)

	// All three sections now belong to the target.
	assert.Equal(t, int64(3), countSectionsByTopic(t, db, target))
	// Sources hold zero sections.
	assert.Equal(t, int64(0), countSectionsByTopic(t, db, srcA))
	assert.Equal(t, int64(0), countSectionsByTopic(t, db, srcB))

	// Sources are archived.
	var srcALoaded, srcBLoaded BoardPersistentTopic
	require.NoError(t, db.First(&srcALoaded, srcA).Error)
	require.NoError(t, db.First(&srcBLoaded, srcB).Error)
	assert.Equal(t, TopicStatusArchived, srcALoaded.Status)
	assert.Equal(t, TopicStatusArchived, srcBLoaded.Status)
}

func TestMergeTopics_RejectsCrossBoard(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewTopicGraphRepository(db)
	boardA := seedTestBoard(t, db)
	// seedTestBoard pins slug "test-board"; the second board needs a distinct
	// slug to satisfy idx_semantic_labels_slug within the same test DB.
	boardB := models.SemanticLabel{Label: "test-board-b", Slug: "test-board-b", LabelType: "board", Status: "active"}
	require.NoError(t, db.Create(&boardB).Error)

	target := seedTopic(t, db, boardA, "target", vecStr(1, 0, 0))
	source := seedTopic(t, db, boardB.ID, "source", vecStr(1, 0, 0))

	_, err := repo.MergeTopics(target, []uint{source})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "different board")
}

func TestMergeTopics_RejectsTargetAsSource(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewTopicGraphRepository(db)
	boardID := seedTestBoard(t, db)
	target := seedTopic(t, db, boardID, "target", vecStr(1, 0, 0))

	_, err := repo.MergeTopics(target, []uint{target})
	require.Error(t, err)
}

func TestSplitTopic_CreatesNewTopicAndReassigns(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewTopicGraphRepository(db)
	Repo = repo

	boardID := seedTestBoard(t, db)
	day := NormalizeReportDate(time.Now())
	reportID := seedTestReport(t, db, boardID, day)
	source := seedTopic(t, db, boardID, "parent", vecStr(1, 0, 0))

	// Source topic owns three sections; we carve out the first two.
	s1 := seedTopicSection(t, db, reportID, "s1", vecStr(1, 0, 0))
	s2 := seedTopicSection(t, db, reportID, "s2", vecStr(0.99, 0.01, 0))
	s3 := seedTopicSection(t, db, reportID, "s3", vecStr(0.98, 0, 0.02))
	assignSectionTopic(t, db, s1, source)
	assignSectionTopic(t, db, s2, source)
	assignSectionTopic(t, db, s3, source)

	require.Equal(t, int64(3), countSectionsByTopic(t, db, source))

	newTopic, err := repo.SplitTopic(source, []uint{s1, s2}, "拆出的新话题")
	require.NoError(t, err)
	require.NotZero(t, newTopic.ID)
	assert.Equal(t, "拆出的新话题", newTopic.Label)
	assert.Equal(t, TopicStatusActive, newTopic.Status)
	assert.Equal(t, 2, newTopic.HitCount)
	// New topic has a mean embedding derived from the carved sections.
	assert.NotEmpty(t, newTopic.Embedding)

	// s1, s2 moved; s3 stays.
	assert.Equal(t, int64(2), countSectionsByTopic(t, db, newTopic.ID))
	assert.Equal(t, int64(1), countSectionsByTopic(t, db, source))
}

func TestSplitTopic_RejectsCarveAll(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewTopicGraphRepository(db)
	boardID := seedTestBoard(t, db)
	day := NormalizeReportDate(time.Now())
	reportID := seedTestReport(t, db, boardID, day)
	source := seedTopic(t, db, boardID, "parent", vecStr(1, 0, 0))

	s1 := seedTopicSection(t, db, reportID, "s1", vecStr(1, 0, 0))
	s2 := seedTopicSection(t, db, reportID, "s2", vecStr(0.99, 0.01, 0))
	assignSectionTopic(t, db, s1, source)
	assignSectionTopic(t, db, s2, source)

	// Carving every section must fail — that is a rename, not a split.
	_, err := repo.SplitTopic(source, []uint{s1, s2}, "all")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rename")
}

func TestSplitTopic_RejectsForeignSections(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewTopicGraphRepository(db)
	boardID := seedTestBoard(t, db)
	day := NormalizeReportDate(time.Now())
	reportID := seedTestReport(t, db, boardID, day)
	topicA := seedTopic(t, db, boardID, "A", vecStr(1, 0, 0))
	topicB := seedTopic(t, db, boardID, "B", vecStr(0, 1, 0))

	// s_foreign belongs to B; trying to split it out of A must fail.
	sOwn := seedTopicSection(t, db, reportID, "own", vecStr(1, 0, 0))
	sForeign := seedTopicSection(t, db, reportID, "foreign", vecStr(0, 1, 0))
	assignSectionTopic(t, db, sOwn, topicA)
	assignSectionTopic(t, db, sForeign, topicB)

	_, err := repo.SplitTopic(topicA, []uint{sOwn, sForeign}, "x")
	require.Error(t, err)
}
