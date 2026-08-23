package service

import (
	"fmt"
	"testing"

	"syntopica-backend/internal/models"
	"syntopica-backend/internal/platform/testutil"
)

// TestArticleContentFormColumnInPGSchema verifies the AutoMigrate-built golden
// schema (real Postgres via testcontainers) includes the new
// articles.content_form column and that the column is writable and readable.
func TestArticleContentFormColumnInPGSchema(t *testing.T) {
	db := testutil.SetupTestDB(t)

	var count int64
	if err := db.Raw(
		"SELECT COUNT(*) FROM information_schema.columns WHERE table_name = 'articles' AND column_name = 'content_form'",
	).Scan(&count).Error; err != nil {
		t.Fatalf("query information_schema: %v", err)
	}
	if count != 1 {
		t.Fatalf("articles.content_form column count = %d, want 1", count)
	}

	feed := models.Feed{Title: "Form mark schema", URL: fmt.Sprintf("https://example.com/%s", t.Name()), ArticleSummaryEnabled: true}
	if err := db.Create(&feed).Error; err != nil {
		t.Fatalf("create feed: %v", err)
	}

	article := models.Article{FeedID: feed.ID, Title: "column probe", Link: fmt.Sprintf("https://example.com/%s/probe", t.Name()), ContentForm: "aggregate"}
	if err := db.Create(&article).Error; err != nil {
		t.Fatalf("create article with content_form: %v", err)
	}

	var refreshed models.Article
	if err := db.First(&refreshed, article.ID).Error; err != nil {
		t.Fatalf("reload article: %v", err)
	}
	if refreshed.ContentForm != "aggregate" {
		t.Fatalf("content_form = %q, want aggregate", refreshed.ContentForm)
	}
}
