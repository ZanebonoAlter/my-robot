package service

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"syntopica-backend/internal/models"
	"syntopica-backend/internal/tagmanagement/repository"
)

func setupMergeReembeddingTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	repository.InitRepository(db)
	if err := db.AutoMigrate(&models.TopicTag{}, &models.TopicTagEmbedding{}, &models.TopicTagRelation{}, &models.EmbeddingConfig{}, &models.MergeReembeddingQueue{}, &models.Article{}, &models.ArticleTopicTag{}); err != nil {
		t.Fatalf("migrate test tables: %v", err)
	}
	if err := db.Exec(`CREATE TABLE article_feeds (article_id INTEGER NOT NULL, feed_id INTEGER NOT NULL)`).Error; err != nil {
		t.Fatalf("create article_feeds: %v", err)
	}
	return db
}

func seedMergeQueueTags(t *testing.T, db *gorm.DB) (models.TopicTag, models.TopicTag) {
	t.Helper()
	source := models.TopicTag{Slug: "source-tag", Label: "Source Tag", Category: models.TagCategoryKeyword, Status: "active"}
	target := models.TopicTag{Slug: "target-tag", Label: "Target Tag", Category: models.TagCategoryKeyword, Status: "active"}
	if err := db.Create(&source).Error; err != nil {
		t.Fatalf("create source tag: %v", err)
	}
	if err := db.Create(&target).Error; err != nil {
		t.Fatalf("create target tag: %v", err)
	}
	return source, target
}

func TestMergeTagsEnqueuesReembeddingAfterSuccess(t *testing.T) {
	db := setupMergeReembeddingTestDB(t)
	source, target := seedMergeQueueTags(t, db)

	article := models.Article{FeedID: 1, Title: "merge article"}
	if err := db.Create(&article).Error; err != nil {
		t.Fatalf("create article: %v", err)
	}
	if err := db.Exec("INSERT INTO article_feeds(article_id, feed_id) VALUES (?, ?)", article.ID, 1).Error; err != nil {
		t.Fatalf("create article_feeds row: %v", err)
	}
	if err := db.Create(&models.ArticleTopicTag{ArticleID: article.ID, TopicTagID: source.ID}).Error; err != nil {
		t.Fatalf("create article topic tag: %v", err)
	}
	if err := db.Create(&models.TopicTagEmbedding{TopicTagID: source.ID, EmbeddingVec: "[0.100000]", Dimension: 1, Model: "test", TextHash: "source"}).Error; err != nil {
		t.Fatalf("create source embedding: %v", err)
	}

	if err := MergeTags(source.ID, target.ID); err != nil {
		t.Fatalf("MergeTags returned error: %v", err)
	}

	var queueTasks []models.MergeReembeddingQueue
	if err := db.Order("id ASC").Find(&queueTasks).Error; err != nil {
		t.Fatalf("load queue tasks: %v", err)
	}
	if len(queueTasks) != 1 {
		t.Fatalf("queue task count = %d, want 1", len(queueTasks))
	}
	if queueTasks[0].SourceTagID != source.ID {
		t.Fatalf("queue source_tag_id = %d, want %d", queueTasks[0].SourceTagID, source.ID)
	}
	if queueTasks[0].TargetTagID != target.ID {
		t.Fatalf("queue target_tag_id = %d, want %d", queueTasks[0].TargetTagID, target.ID)
	}
	if queueTasks[0].Status != models.MergeReembeddingQueueStatusPending {
		t.Fatalf("queue status = %s, want pending", queueTasks[0].Status)
	}

	var activeTags []models.TopicTag
	if err := db.Scopes(activeTagFilter).Order("id ASC").Find(&activeTags).Error; err != nil {
		t.Fatalf("load active tags: %v", err)
	}
	if len(activeTags) != 1 || activeTags[0].ID != target.ID {
		t.Fatalf("active tags after merge = %#v, want only target tag", activeTags)
	}
}

type failingMergeReembeddingQueue struct{}

func (f *failingMergeReembeddingQueue) Enqueue(sourceTagID, targetTagID uint) error {
	return errors.New("enqueue unavailable")
}

func TestMergeTagsReturnsErrorWhenReembeddingEnqueueFails(t *testing.T) {
	db := setupMergeReembeddingTestDB(t)
	source, target := seedMergeQueueTags(t, db)

	if err := db.Exec("INSERT INTO article_feeds(article_id, feed_id) VALUES (?, ?)", 1, 1).Error; err != nil {
		t.Fatalf("seed article_feeds: %v", err)
	}

	originalFactory := MergeReembeddingQueueFactory
	MergeReembeddingQueueFactory = func() mergeReembeddingEnqueuer {
		return &failingMergeReembeddingQueue{}
	}
	t.Cleanup(func() {
		MergeReembeddingQueueFactory = originalFactory
	})

	err := MergeTags(source.ID, target.ID)
	if err == nil {
		t.Fatalf("expected MergeTags to return enqueue error")
	}
	if err.Error() != "enqueue merge re-embedding task: enqueue unavailable" {
		t.Fatalf("unexpected error: %v", err)
	}
}
