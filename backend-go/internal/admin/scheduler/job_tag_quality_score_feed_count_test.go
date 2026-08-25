package scheduler

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"syntopica-backend/internal/models"
	"syntopica-backend/internal/platform/testutil"
)

// TestRecalculateTopicTagFeedCounts verifies the periodic feed_count
// reconciliation added in fix-quality-audit-p0: the denormalized counter is
// rebuilt as COUNT(DISTINCT articles.feed_id) via article_topic_tags, fixing
// drift left by tagging paths that never maintain it incrementally.
func TestRecalculateTopicTagFeedCounts(t *testing.T) {
	db := testutil.SetupTestDB(t)

	feeds := []models.Feed{
		{Title: "f1", URL: "https://example.com/1"},
		{Title: "f2", URL: "https://example.com/2"},
	}
	for i := range feeds {
		require.NoError(t, db.Create(&feeds[i]).Error)
	}

	articles := []models.Article{
		{FeedID: feeds[0].ID, Title: "a1"}, // tag A via feed 1
		{FeedID: feeds[1].ID, Title: "a2"}, // tag A via feed 2 → distinct feeds = 2
		{FeedID: feeds[0].ID, Title: "a3"}, // tag B via feed 1 → distinct feeds = 1
	}
	for i := range articles {
		require.NoError(t, db.Create(&articles[i]).Error)
	}

	// Seed feed_count with stale values to prove full recalculation.
	tags := []models.TopicTag{
		{Label: "tagA", Slug: "tag-a", Category: "keyword", FeedCount: 99},
		{Label: "tagB", Slug: "tag-b", Category: "keyword", FeedCount: 5},
		{Label: "tagC", Slug: "tag-c", Category: "keyword", FeedCount: 7}, // no links → 0
	}
	for i := range tags {
		require.NoError(t, db.Create(&tags[i]).Error)
	}

	links := []models.ArticleTopicTag{
		{ArticleID: articles[0].ID, TopicTagID: tags[0].ID},
		{ArticleID: articles[1].ID, TopicTagID: tags[0].ID},
		{ArticleID: articles[2].ID, TopicTagID: tags[1].ID},
	}
	for i := range links {
		require.NoError(t, db.Create(&links[i]).Error)
	}

	require.NoError(t, RecalculateTopicTagFeedCounts(db))

	var got []int
	require.NoError(t, db.Model(&models.TopicTag{}).Order("id").Pluck("feed_count", &got).Error)
	require.Equal(t, []int{2, 1, 0}, got)
}

// TestRecalculateTopicTagFeedCountsFailure ensures the function surfaces SQL
// errors to its caller (TagQualityScoreJob logs a warning and continues,
// mirroring the auxiliary ref_count reconcile's fault tolerance).
func TestRecalculateTopicTagFeedCountsFailure(t *testing.T) {
	db := testutil.SetupTestDB(t)

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return tx.Exec("DROP TABLE article_topic_tags").Error
	}))
	t.Cleanup(func() {
		// Restore the golden schema for sibling tests sharing this DB:
		// recreate the table from the live model definition.
		require.NoError(t, db.Migrator().CreateTable(&models.ArticleTopicTag{}))
	})

	err := RecalculateTopicTagFeedCounts(db)
	require.Error(t, err, "reconcile must surface SQL failure to the caller")
}
