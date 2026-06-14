package repository

import (
	"testing"

	"syntopica-backend/internal/models"
	"syntopica-backend/internal/platform/testutil"
)

func TestEnqueueTagJobUpgradesForceRetag(t *testing.T) {
	db := testutil.SetupTestDB(t)
	InitRepository(db)

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
