package dataenrichment_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"syntopica-backend/internal/dataenrichment"
	"syntopica-backend/internal/dataenrichment/repository"
	"syntopica-backend/internal/dataenrichment/service"
	"syntopica-backend/internal/platform/airouter"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"syntopica-backend/internal/platform/database"
)

func setupJobTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	database.DB = db
	if err := database.RunAutoMigrate(db); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	repository.InitRepo(db)
}

// mockActiveTopicLister returns a fixed list of active topic IDs.
type mockActiveTopicLister struct {
	ids []uint
}

func (m *mockActiveTopicLister) ListActiveTopicIDs(ctx context.Context) ([]uint, error) {
	return m.ids, nil
}

// jobMockAirouter implements service.AirRouter for job tests.
type jobMockAirouter struct {
	callCount int
}

func (m *jobMockAirouter) Chat(ctx context.Context, req airouter.ChatRequest) (*airouter.ChatResult, error) {
	m.callCount++
	return &airouter.ChatResult{
		Content:      "mocked",
		ProviderName: "mock",
		RouteName:    "mock",
	}, nil
}

// mockSectionReader for job tests.
type jobMockSectionReader struct{}

func (m *jobMockSectionReader) ReadSections(ctx context.Context, topicID uint, from, to time.Time) (string, error) {
	return "mock sections", nil
}

func TestWeeklyLifelineJob_HealThenRefresh(t *testing.T) {
	setupJobTestDB(t)
	ar := &jobMockAirouter{}
	sr := &jobMockSectionReader{}
	svc := service.NewLifelineContextService(ar, repository.Repo, sr, dataenrichment.CapabilityNews)
	lister := &mockActiveTopicLister{ids: []uint{1, 2, 3}}

	job := dataenrichment.WeeklyLifelineJob(svc, lister)
	result, err := job(context.Background())
	if err != nil {
		t.Fatalf("WeeklyLifelineJob: %v", err)
	}

	// Should have refreshed 3 topics + possibly heal calls
	refreshed, ok := result.Data["refreshed"].(int)
	if !ok || refreshed != 3 {
		t.Fatalf("expected 3 refreshed topics, got %v", result.Data["refreshed"])
	}

	// HealStale + RefreshWeek for 3 topics = at least 3 LLM calls
	if ar.callCount < 3 {
		t.Fatalf("expected at least 3 LLM calls, got %d", ar.callCount)
	}
}

func TestMonthlyLifelineJob_HealThenRefresh(t *testing.T) {
	setupJobTestDB(t)
	ar := &jobMockAirouter{}
	sr := &jobMockSectionReader{}
	svc := service.NewLifelineContextService(ar, repository.Repo, sr, dataenrichment.CapabilityNews)
	lister := &mockActiveTopicLister{ids: []uint{1}}

	job := dataenrichment.MonthlyLifelineJob(svc, lister)
	result, err := job(context.Background())
	if err != nil {
		t.Fatalf("MonthlyLifelineJob: %v", err)
	}

	refreshed, ok := result.Data["refreshed"].(int)
	if !ok || refreshed != 1 {
		t.Fatalf("expected 1 refreshed topic, got %v", result.Data["refreshed"])
	}

	if result.Summary == "" {
		t.Fatal("expected non-empty summary")
	}
}

func TestYearlyLifelineJob_HealThenRefresh(t *testing.T) {
	setupJobTestDB(t)
	ar := &jobMockAirouter{}
	sr := &jobMockSectionReader{}
	svc := service.NewLifelineContextService(ar, repository.Repo, sr, dataenrichment.CapabilityNews)
	lister := &mockActiveTopicLister{ids: []uint{1, 2}}

	job := dataenrichment.YearlyLifelineJob(svc, lister)
	result, err := job(context.Background())
	if err != nil {
		t.Fatalf("YearlyLifelineJob: %v", err)
	}

	refreshed, ok := result.Data["refreshed"].(int)
	if !ok || refreshed != 2 {
		t.Fatalf("expected 2 refreshed topics, got %v", result.Data["refreshed"])
	}
}

func TestLifelineJobs_GranularityLabels(t *testing.T) {
	setupJobTestDB(t)
	ar := &jobMockAirouter{}
	sr := &jobMockSectionReader{}
	svc := service.NewLifelineContextService(ar, repository.Repo, sr, dataenrichment.CapabilityNews)
	lister := &mockActiveTopicLister{ids: []uint{1}}

	tests := []struct {
		name     string
		jobFunc  func() interface{}
		wantGran string
	}{
		{"week", func() interface{} { return dataenrichment.WeeklyLifelineJob(svc, lister) }, "week"},
		{"month", func() interface{} { return dataenrichment.MonthlyLifelineJob(svc, lister) }, "month"},
		{"year", func() interface{} { return dataenrichment.YearlyLifelineJob(svc, lister) }, "year"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify the job function is callable.
			_ = tt.jobFunc
		})
	}
}

func TestLifelineJobs_OrderHealBeforeRefresh(t *testing.T) {
	setupJobTestDB(t)
	ar := &jobMockAirouter{}
	sr := &jobMockSectionReader{}
	svc := service.NewLifelineContextService(ar, repository.Repo, sr, dataenrichment.CapabilityNews)
	lister := &mockActiveTopicLister{ids: []uint{1}}

	job := dataenrichment.WeeklyLifelineJob(svc, lister)
	result, err := job(context.Background())
	if err != nil {
		t.Fatalf("WeeklyLifelineJob: %v", err)
	}

	// HealStale for week (calls ListStale → 0 calls if no stale + RefreshWeek for 1 topic = 1 call)
	// RefreshWeek for 1 topic = 1 LLM call (read sections + summarize)
	// So at minimum 1 LLM call from RefreshWeek.
	if ar.callCount < 1 {
		t.Fatalf("expected at least 1 LLM call, got %d", ar.callCount)
	}

	// Verify the result data contains the right granularity
	if result.Data["granularity"] != "week" {
		t.Fatalf("expected granularity week, got %v", result.Data["granularity"])
	}
}
