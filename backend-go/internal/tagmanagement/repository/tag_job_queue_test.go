package repository

import (
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"syntopica-backend/internal/models"
)

func setupTagJobQueueTestDB(t *testing.T) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	InitRepository(db)
	if err := Repo.DB().AutoMigrate(&models.TagJob{}); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
}

func TestEnqueueTagJobUpgradesForceRetag(t *testing.T) {
	setupTagJobQueueTestDB(t)

	queue := NewTagJobQueue(Repo.DB())
	request := TagJobRequest{ArticleID: 42, FeedName: "Feed", ForceRetag: false}
	if err := queue.Enqueue(request); err != nil {
		t.Fatalf("first enqueue: %v", err)
	}

	request.ForceRetag = true
	if err := queue.Enqueue(request); err != nil {
		t.Fatalf("second enqueue: %v", err)
	}

	var jobs []models.TagJob
	if err := Repo.DB().Order("id asc").Find(&jobs).Error; err != nil {
		t.Fatalf("load jobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("job count = %d, want 1", len(jobs))
	}
	if !jobs[0].ForceRetag {
		t.Fatal("expected active job to be upgraded to force retag")
	}
}
