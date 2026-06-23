package core

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"syntopica-backend/internal/models"
	"syntopica-backend/internal/platform/testutil"
	"syntopica-backend/internal/tagmanagement/repository"
)

func setupArticleTaggerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := testutil.SetupTestDB(t)
	repository.InitRepository(db)
	return db
}

func TestCreateArticleTopicTagLink(t *testing.T) {
	db := setupArticleTaggerTestDB(t)
	article, tag := createArticleTaggerFixtures(t, db)
	link := models.ArticleTopicTag{ArticleID: article.ID, TopicTagID: tag.ID, Score: 0.7, Source: "llm"}

	articleExists, err := createArticleTopicTagLink(&link)

	require.NoError(t, err)
	require.True(t, articleExists)
	require.NotZero(t, link.ID)
}

func TestCreateArticleTopicTagLinkSkipsDeletedArticle(t *testing.T) {
	db := setupArticleTaggerTestDB(t)
	article, tag := createArticleTaggerFixtures(t, db)
	require.NoError(t, db.Delete(&article).Error)
	link := models.ArticleTopicTag{ArticleID: article.ID, TopicTagID: tag.ID, Score: 0.7, Source: "llm"}

	articleExists, err := createArticleTopicTagLink(&link)

	require.NoError(t, err)
	require.False(t, articleExists)

	var count int64
	require.NoError(t, db.Model(&models.ArticleTopicTag{}).Where("article_id = ?", article.ID).Count(&count).Error)
	require.Zero(t, count)
}

func createArticleTaggerFixtures(t *testing.T, db *gorm.DB) (models.Article, models.TopicTag) {
	t.Helper()
	feed := models.Feed{Title: "Tagger Test", URL: "https://example.com/tagger-test"}
	require.NoError(t, db.Create(&feed).Error)

	article := models.Article{FeedID: feed.ID, Title: "Tagger Test Article"}
	require.NoError(t, db.Create(&article).Error)

	tag := models.TopicTag{Label: "Tagger Test", Slug: "tagger-test", Category: "keyword", Status: "active"}
	require.NoError(t, db.Create(&tag).Error)
	return article, tag
}
