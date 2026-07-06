package service_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"syntopica-backend/internal/dataenrichment"
	"syntopica-backend/internal/dataenrichment/repository"
	"syntopica-backend/internal/dataenrichment/service"
	"syntopica-backend/internal/platform/airouter"
	"syntopica-backend/internal/platform/database"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// ── Test helpers ────────────────────────────────────────────────────────────

func setupLifelineTestDB(t *testing.T) {
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

// mockAirouter records Chat calls and returns canned responses.
type mockAirouter struct {
	calls   []airouter.ChatRequest
	results map[string]string // keyed by operation+first message snippet, returns content
}

func newMockAirouter() *mockAirouter {
	return &mockAirouter{
		results: make(map[string]string),
	}
}

func (m *mockAirouter) add(operation string, msgPrefix string, content string) {
	m.results[operation+"|"+msgPrefix] = content
}

func (m *mockAirouter) Chat(ctx context.Context, req airouter.ChatRequest) (*airouter.ChatResult, error) {
	m.calls = append(m.calls, req)

	// Try exact match first
	firstMsg := ""
	if len(req.Messages) > 0 {
		firstMsg = req.Messages[0].Content
	}
	key := req.Operation + "|" + firstMsg

	// Fall back to prefix match
	for k, v := range m.results {
		if strings.HasPrefix(key, k) || strings.HasPrefix(k, key) {
			return &airouter.ChatResult{Content: v}, nil
		}
	}

	// Default: return the operation as content so tests can verify.
	return &airouter.ChatResult{Content: fmt.Sprintf("summarized: %s", req.Operation)}, nil
}

// mockSectionReader returns canned section text for topic+range queries.
type mockSectionReader struct {
	byRange map[string]string // key: "topicID|from|to"
}

func newMockSectionReader() *mockSectionReader {
	return &mockSectionReader{
		byRange: make(map[string]string),
	}
}

func (m *mockSectionReader) add(topicID uint, from, to time.Time, text string) {
	m.byRange[fmt.Sprintf("%d|%s|%s", topicID, from.Format("2006-01-02"), to.Format("2006-01-02"))] = text
}

func (m *mockSectionReader) ReadSections(ctx context.Context, topicID uint, from, to time.Time) (string, error) {
	key := fmt.Sprintf("%d|%s|%s", topicID, from.Format("2006-01-02"), to.Format("2006-01-02"))
	if text, ok := m.byRange[key]; ok {
		return text, nil
	}
	// Default: return a placeholder
	return fmt.Sprintf("sections for topic %d from %s to %s", topicID, from.Format("2006-01-02"), to.Format("2006-01-02")), nil
}

func newTestService(airouter service.AirRouter, sectionReader service.SectionReader) *service.LifelineContextService {
	return service.NewLifelineContextService(
		airouter,
		repository.Repo,
		sectionReader,
		dataenrichment.CapabilityNews,
	)
}

// fixedMonday returns a time that is a known Monday.
// 2026-07-06 is a Monday (verified: July 2026 calendar).
func fixedMonday() time.Time {
	return time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC)
}

// ── Tests: RefreshWeek ──────────────────────────────────────────────────────

func TestRefreshWeek_ReadsCurrentWeekSections(t *testing.T) {
	setupLifelineTestDB(t)
	ar := newMockAirouter()
	sr := newMockSectionReader()
	svc := newTestService(ar, sr)

	now := fixedMonday() // Monday Jul 6, 2026
	// Current week: Mon Jul 6 → Mon Jul 13
	topicID := uint(1)

	// Populate a section in this week's range
	sr.add(topicID,
		time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC),
		"周一 芯片供不应求：thread1; thread2\n周二 产能扩张：thread3")

	err := svc.RefreshWeek(context.Background(), topicID, now)
	if err != nil {
		t.Fatalf("RefreshWeek: %v", err)
	}

	// Verify: context was upserted with as_of_date = end of week (Jul 13)
	ctx, err := repository.Repo.GetTopicLifelineContext(context.Background(), topicID, string(repository.GranularityWeek))
	if err != nil {
		t.Fatalf("get context: %v", err)
	}
	if ctx.AsOfDate.Format("2006-01-02") != "2026-07-13" {
		t.Fatalf("expected as_of_date 2026-07-13, got %s", ctx.AsOfDate.Format("2006-01-02"))
	}
	if ctx.Content == "" {
		t.Fatal("expected non-empty content")
	}
	if ctx.Source != "llm_assisted" {
		t.Fatalf("expected source llm_assisted, got %s", ctx.Source)
	}
}

func TestRefreshWeek_LLMCallParams(t *testing.T) {
	setupLifelineTestDB(t)
	ar := newMockAirouter()
	sr := newMockSectionReader()
	svc := newTestService(ar, sr)

	now := fixedMonday()
	topicID := uint(2)

	err := svc.RefreshWeek(context.Background(), topicID, now)
	if err != nil {
		t.Fatalf("RefreshWeek: %v", err)
	}

	if len(ar.calls) == 0 {
		t.Fatal("expected at least one Chat call")
	}

	req := ar.calls[0]
	// Verify Capability
	if req.Capability != dataenrichment.CapabilityNews {
		t.Fatalf("expected capability %s, got %s", dataenrichment.CapabilityNews, req.Capability)
	}
	// Verify Operation
	if req.Operation != "data_enrichment.summarize_context" {
		t.Fatalf("expected operation summarize_context, got %s", req.Operation)
	}
	// Verify SessionID format
	if !strings.HasPrefix(req.SessionID, "lifeline_context_") {
		t.Fatalf("expected session_id prefix lifeline_context_, got %s", req.SessionID)
	}
	if !strings.Contains(req.SessionID, "_week_") {
		t.Fatalf("expected session_id to contain _week_, got %s", req.SessionID)
	}
	// Verify temperature ~0.3
	if req.Temperature == nil || *req.Temperature < 0.2 || *req.Temperature > 0.4 {
		t.Fatalf("expected temperature ~0.3, got %v", req.Temperature)
	}
}

func TestRefreshWeek_NoOldContext(t *testing.T) {
	setupLifelineTestDB(t)
	ar := newMockAirouter()
	sr := newMockSectionReader()
	svc := newTestService(ar, sr)

	now := fixedMonday()
	topicID := uint(3)

	// No old week context exists — should still work (direct summary)
	err := svc.RefreshWeek(context.Background(), topicID, now)
	if err != nil {
		t.Fatalf("RefreshWeek: %v", err)
	}

	// LLM should receive only sections (no old context in prompt)
	if len(ar.calls) == 0 {
		t.Fatal("expected Chat call")
	}
	prompt := ar.calls[0].Messages[0].Content
	if strings.Contains(prompt, "已有汇总") || strings.Contains(prompt, "旧汇总") {
		t.Fatal("week prompt should not contain old context reference")
	}
	if !strings.Contains(prompt, "一周") {
		t.Fatal("week prompt should mention week")
	}
}

// ── Tests: RefreshMonth (incremental + old) ─────────────────────────────────

func TestRefreshMonth_IncrementalMerge(t *testing.T) {
	setupLifelineTestDB(t)
	ar := newMockAirouter()
	sr := newMockSectionReader()
	svc := newTestService(ar, sr)

	// Pre-populate old month context
	oldCtx := &repository.TopicLifelineContext{
		PersistentTopicID: 4,
		Granularity:       string(repository.GranularityMonth),
		Content:           "6月摘要：芯片板块整理为主，存储芯片价格回升",
		AsOfDate:          time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), // as_of_date = June 1st
		Source:            "llm_assisted",
	}
	if err := repository.Repo.UpsertTopicLifelineContext(context.Background(), oldCtx); err != nil {
		t.Fatalf("insert old context: %v", err)
	}

	// Mock sections since June 1st
	now := time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC)
	topicID := uint(4)
	sr.add(topicID,
		time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		"增量：7月初突发，AI芯片出口管制")

	err := svc.RefreshMonth(context.Background(), topicID, now)
	if err != nil {
		t.Fatalf("RefreshMonth: %v", err)
	}

	// Verify: old context merged with new sections
	if len(ar.calls) == 0 {
		t.Fatal("expected Chat call")
	}
	prompt := ar.calls[0].Messages[0].Content
	if !strings.Contains(prompt, "6月摘要") {
		t.Fatal("month prompt should contain old summary")
	}
	if !strings.Contains(prompt, "增量") {
		t.Fatal("month prompt should contain incremental sections")
	}

	// Verify: as_of_date advanced
	ctx, err := repository.Repo.GetTopicLifelineContext(context.Background(), topicID, string(repository.GranularityMonth))
	if err != nil {
		t.Fatalf("get context: %v", err)
	}
	// as_of_date should be July 1st (end of June month period, or start of July)
	if ctx.AsOfDate.Before(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected as_of_date >= 2026-07-01, got %s", ctx.AsOfDate.Format("2006-01-02"))
	}
}

// ── Tests: HealStale (self-healing) ─────────────────────────────────────────

func TestHealStale_SkipOneWeekThenCatchUp(t *testing.T) {
	setupLifelineTestDB(t)
	ar := newMockAirouter()
	sr := newMockSectionReader()
	svc := newTestService(ar, sr)

	// Simulate: a topic was last refreshed 1 week ago (skipped last week).
	// Use dates relative to real time so ListStaleTopicLifelineContexts can find them.
	// Since sinceDays=8 for week, as_of_date must be at least 9 days ago.
	now := time.Now().UTC()
	// Compute Monday of the current week.
	currentMonday, _ := weekBoundaries(now)
	currentEnd := currentMonday.AddDate(0, 0, 7)
	// as_of_date = 1 Monday before current Monday (7 days ago), which is >8 days in past
	// if today is near the weekend. For safety, use 14 days.
	oldAsOf := currentMonday.AddDate(0, 0, -14)

	topicID := uint(5)
	oldCtx := &repository.TopicLifelineContext{
		PersistentTopicID: topicID,
		Granularity:       string(repository.GranularityWeek),
		Content:           "两周前的内容",
		AsOfDate:          oldAsOf,
		Source:            "llm_assisted",
	}
	if err := repository.Repo.UpsertTopicLifelineContext(context.Background(), oldCtx); err != nil {
		t.Fatalf("insert old: %v", err)
	}

	// Mock sections for every week from oldAsOf to currentEnd to cover all cycles.
	// HealStale iterates weekly: oldAsOf → oldAsOf+7 → oldAsOf+14 → ...
	for start := oldAsOf; start.Before(currentEnd); start = start.AddDate(0, 0, 7) {
		end := start.AddDate(0, 0, 7)
		if end.After(currentEnd) {
			end = currentEnd
		}
		label := fmt.Sprintf("week %s", start.Format("01-02"))
		sr.add(topicID, start, end, label)
	}

	err := svc.HealStale(context.Background(), string(repository.GranularityWeek), now)
	if err != nil {
		t.Fatalf("HealStale: %v", err)
	}

	// Should have made at least 2 LLM calls: patch missed weeks, then current week
	if len(ar.calls) < 2 {
		t.Fatalf("expected at least 2 Chat calls (patch missed + current), got %d", len(ar.calls))
	}

	// Final as_of_date should be currentEnd (exclusive boundary of current week)
	ctx, err := repository.Repo.GetTopicLifelineContext(context.Background(), topicID, string(repository.GranularityWeek))
	if err != nil {
		t.Fatalf("get context: %v", err)
	}
	if ctx.AsOfDate.Format("2006-01-02") != currentEnd.Format("2006-01-02") {
		t.Fatalf("expected final as_of_date %s, got %s", currentEnd.Format("2006-01-02"), ctx.AsOfDate.Format("2006-01-02"))
	}
}

func TestHealStale_NoStaleContexts(t *testing.T) {
	setupLifelineTestDB(t)
	ar := newMockAirouter()
	sr := newMockSectionReader()
	svc := newTestService(ar, sr)

	// Context that is up to date: as_of_date = today (well within sinceDays=8).
	now := time.Now().UTC()
	topicID := uint(6)
	freshCtx := &repository.TopicLifelineContext{
		PersistentTopicID: topicID,
		Granularity:       string(repository.GranularityWeek),
		Content:           "本周最新",
		AsOfDate:          now.AddDate(0, 0, -1), // yesterday — not stale
		Source:            "llm_assisted",
	}
	repository.Repo.UpsertTopicLifelineContext(context.Background(), freshCtx)

	err := svc.HealStale(context.Background(), string(repository.GranularityWeek), now)
	if err != nil {
		t.Fatalf("HealStale: %v", err)
	}

	// Should be no-op — context is not stale (as_of_date is only 1 day ago, within 8-day threshold)
	if len(ar.calls) > 0 {
		t.Fatalf("expected 0 Chat calls for fresh context, got %d", len(ar.calls))
	}
}

func TestHealStale_AsOfDateAdvancesSequentially(t *testing.T) {
	setupLifelineTestDB(t)
	ar := newMockAirouter()
	sr := newMockSectionReader()
	svc := newTestService(ar, sr)

	// Month granularity: as_of_date = 2 months ago 1st, now = today.
	// Should patch: month1→month2→currentMonth (as_of_date advances sequentially).
	now := time.Now().UTC()
	currentMonth1st := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	nextMonth1st := currentMonth1st.AddDate(0, 1, 0)
	// as_of_date = 2 months before current month 1st
	oldAsOf := currentMonth1st.AddDate(0, -2, 0)
	midAsOf := currentMonth1st.AddDate(0, -1, 0)

	topicID := uint(7)
	oldCtx := &repository.TopicLifelineContext{
		PersistentTopicID: topicID,
		Granularity:       string(repository.GranularityMonth),
		Content:           "旧摘要",
		AsOfDate:          oldAsOf,
		Source:            "llm_assisted",
	}
	repository.Repo.UpsertTopicLifelineContext(context.Background(), oldCtx)

	// Mock sections for the first missed month
	sr.add(topicID, oldAsOf, midAsOf, "第1个遗漏月新闻")
	// Mock sections for the second missed month (up to current)
	sr.add(topicID, midAsOf, nextMonth1st, "第2个遗漏月（含当前月）新闻")

	err := svc.HealStale(context.Background(), string(repository.GranularityMonth), now)
	if err != nil {
		t.Fatalf("HealStale: %v", err)
	}

	// Should have made 2 LLM calls (two missed months → current month inclusive)
	if len(ar.calls) < 2 {
		t.Fatalf("expected 2 Chat calls for two months, got %d", len(ar.calls))
	}

	// Final as_of_date should be nextMonth1st (exclusive boundary of current month)
	ctx, err := repository.Repo.GetTopicLifelineContext(context.Background(), topicID, string(repository.GranularityMonth))
	if err != nil {
		t.Fatalf("get context: %v", err)
	}
	if ctx.AsOfDate.Format("2006-01-02") != nextMonth1st.Format("2006-01-02") {
		t.Fatalf("expected as_of_date %s, got %s", nextMonth1st.Format("2006-01-02"), ctx.AsOfDate.Format("2006-01-02"))
	}
}

// ── Tests: RefreshGranularity dispatch ─────────────────────────────────────

func TestRefreshGranularity_DispatchesCorrectly(t *testing.T) {
	setupLifelineTestDB(t)
	ar := newMockAirouter()
	sr := newMockSectionReader()
	svc := newTestService(ar, sr)

	now := fixedMonday()
	topicID := uint(8)

	tests := []struct {
		granularity string
		expectOp    string
	}{
		{string(repository.GranularityWeek), "data_enrichment.summarize_context"},
		{string(repository.GranularityMonth), "data_enrichment.summarize_context"},
		{string(repository.GranularityYear), "data_enrichment.summarize_context"},
		{string(repository.GranularityAll), "data_enrichment.summarize_context"},
	}

	for _, tt := range tests {
		t.Run(tt.granularity, func(t *testing.T) {
			ar.calls = nil // reset
			err := svc.RefreshGranularity(context.Background(), topicID, tt.granularity, now)
			if err != nil {
				t.Fatalf("RefreshGranularity(%s): %v", tt.granularity, err)
			}
			if len(ar.calls) == 0 {
				t.Fatalf("expected Chat call for %s", tt.granularity)
			}
			if ar.calls[0].Operation != tt.expectOp {
				t.Fatalf("expected %s, got %s", tt.expectOp, ar.calls[0].Operation)
			}
		})
	}
}

func TestRefreshGranularity_InvalidGranularity(t *testing.T) {
	setupLifelineTestDB(t)
	ar := newMockAirouter()
	sr := newMockSectionReader()
	svc := newTestService(ar, sr)

	err := svc.RefreshGranularity(context.Background(), 1, "daily", fixedMonday())
	if err == nil {
		t.Fatal("expected error for invalid granularity")
	}
	if !strings.Contains(err.Error(), "unknown granularity") {
		t.Fatalf("expected 'unknown granularity' error, got: %v", err)
	}
}

// ── Tests: RefreshAll ──────────────────────────────────────────────────────

func TestRefreshAll_IncrementalMerge(t *testing.T) {
	setupLifelineTestDB(t)
	ar := newMockAirouter()
	sr := newMockSectionReader()
	svc := newTestService(ar, sr)

	// Pre-populate old 'all' context
	oldCtx := &repository.TopicLifelineContext{
		PersistentTopicID: 9,
		Granularity:       string(repository.GranularityAll),
		Content:           "历史全貌：芯片板块5年综述",
		AsOfDate:          time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		Source:            "llm_assisted",
	}
	repository.Repo.UpsertTopicLifelineContext(context.Background(), oldCtx)

	now := time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC)
	topicID := uint(9)
	sr.add(topicID,
		time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		now,
		"增量：7月AI芯片管制新动态")

	err := svc.RefreshAll(context.Background(), topicID, now)
	if err != nil {
		t.Fatalf("RefreshAll: %v", err)
	}

	prompt := ar.calls[0].Messages[0].Content
	if !strings.Contains(prompt, "历史全貌") {
		t.Fatal("all prompt should contain old summary")
	}
	if !strings.Contains(prompt, "增量") {
		t.Fatal("all prompt should contain incremental sections")
	}
}

// weekBoundaries returns (Monday 00:00, Monday 00:00 next week) for t.
func weekBoundaries(t time.Time) (mon, nextMon time.Time) {
	return weekRange(t)
}

// weekRange helper for test use.
func weekRange(t time.Time) (from, to time.Time) {
	weekday := t.Weekday()
	daysFromMonday := int(weekday) - int(time.Monday)
	if daysFromMonday < 0 {
		daysFromMonday += 7
	}
	monday := t.AddDate(0, 0, -daysFromMonday)
	from = time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, t.Location())
	to = from.AddDate(0, 0, 7)
	return
}
