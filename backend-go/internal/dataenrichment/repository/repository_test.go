package repository_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"syntopica-backend/internal/dataenrichment/repository"
	"syntopica-backend/internal/platform/database"
)

func setupRepoTestDB(t *testing.T) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	database.DB = db

	if err := database.RunAutoMigrate(db); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}

	repo := repository.NewRepository(db)
	repository.SetRepo(repo)
}

func TestBoardDataSource_CreateAndGetByBoardAndType(t *testing.T) {
	setupRepoTestDB(t)

	ctx := context.Background()

	ds := &repository.BoardDataSource{
		SemanticBoardID: 1,
		SourceType:      "etf_quote",
		Config:          map[string]any{"keywords": []string{"半导体"}},
		Enabled:         true,
	}
	if err := repository.Repo.CreateBoardDataSource(ctx, ds); err != nil {
		t.Fatalf("create: %v", err)
	}
	if ds.ID == 0 {
		t.Fatal("expected ID to be set after create")
	}

	got, err := repository.Repo.GetBoardDataSourceByBoardAndType(ctx, 1, "etf_quote")
	if err != nil {
		t.Fatalf("get by board and type: %v", err)
	}
	if got.SemanticBoardID != 1 {
		t.Fatalf("board id = %d, want 1", got.SemanticBoardID)
	}
}

func TestBoardDataSource_Upsert(t *testing.T) {
	setupRepoTestDB(t)

	ctx := context.Background()

	ds := &repository.BoardDataSource{
		SemanticBoardID: 1,
		SourceType:      "etf_quote",
		Config:          map[string]any{"keywords": []string{"半导体"}},
		Enabled:         true,
	}
	if err := repository.Repo.UpsertBoardDataSource(ctx, ds); err != nil {
		t.Fatalf("upsert #1: %v", err)
	}

	ds.Enabled = false
	if err := repository.Repo.UpsertBoardDataSource(ctx, ds); err != nil {
		t.Fatalf("upsert #2: %v", err)
	}

	got, err := repository.Repo.GetBoardDataSourceByBoardAndType(ctx, 1, "etf_quote")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Enabled {
		t.Fatal("expected enabled=false after upsert update")
	}
}

// TestBoardDataSource_UpsertTOCTOU verifies that rapid sequential upserts
// with the same unique key are atomic (no duplicate-key error).
func TestBoardDataSource_UpsertTOCTOU(t *testing.T) {
	setupRepoTestDB(t)
	ctx := context.Background()

	// First upsert.
	ds := &repository.BoardDataSource{
		SemanticBoardID: 1,
		SourceType:      "etf_quote",
		Enabled:         true,
	}
	if err := repository.Repo.UpsertBoardDataSource(ctx, ds); err != nil {
		t.Fatalf("upsert #1: %v", err)
	}

	// Second upsert with same key — must not fail.
	ds2 := &repository.BoardDataSource{
		SemanticBoardID: 1,
		SourceType:      "etf_quote",
		Enabled:         false,
	}
	if err := repository.Repo.UpsertBoardDataSource(ctx, ds2); err != nil {
		t.Fatalf("upsert #2 (TOCTOU): %v", err)
	}

	// Verify only one row exists.
	list, err := repository.Repo.ListBoardDataSourcesByBoardID(ctx, 1)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 row after upserts, got %d", len(list))
	}
	if list[0].Enabled {
		t.Fatal("expected enabled=false after second upsert")
	}
}

func TestBoardDataSource_ListByBoardID(t *testing.T) {
	setupRepoTestDB(t)

	ctx := context.Background()

	ds1 := &repository.BoardDataSource{SemanticBoardID: 1, SourceType: "etf_quote", Enabled: true}
	ds2 := &repository.BoardDataSource{SemanticBoardID: 1, SourceType: "exchange_rate", Enabled: true}
	ds3 := &repository.BoardDataSource{SemanticBoardID: 2, SourceType: "etf_quote", Enabled: true}

	for _, ds := range []*repository.BoardDataSource{ds1, ds2, ds3} {
		if err := repository.Repo.CreateBoardDataSource(ctx, ds); err != nil {
			t.Fatalf("create: %v", err)
		}
	}

	list, err := repository.Repo.ListBoardDataSourcesByBoardID(ctx, 1)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("list count = %d, want 2", len(list))
	}
}

func TestBoardDataSource_UniqueConstraint(t *testing.T) {
	setupRepoTestDB(t)

	ctx := context.Background()

	ds := &repository.BoardDataSource{SemanticBoardID: 1, SourceType: "etf_quote"}
	if err := repository.Repo.CreateBoardDataSource(ctx, ds); err != nil {
		t.Fatalf("create #1: %v", err)
	}

	ds2 := &repository.BoardDataSource{SemanticBoardID: 1, SourceType: "etf_quote"}
	if err := repository.Repo.CreateBoardDataSource(ctx, ds2); err == nil {
		t.Fatal("expected UNIQUE constraint error for duplicate board+source_type")
	}
}

func TestBoardDataSource_Delete(t *testing.T) {
	setupRepoTestDB(t)

	ctx := context.Background()

	ds := &repository.BoardDataSource{SemanticBoardID: 1, SourceType: "etf_quote"}
	if err := repository.Repo.CreateBoardDataSource(ctx, ds); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := repository.Repo.DeleteBoardDataSource(ctx, ds.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err := repository.Repo.GetBoardDataSourceByBoardAndType(ctx, 1, "etf_quote")
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

// ── TopicLifelineContext ────────────────────────────────────────────────────

func TestTopicLifelineContext_UpsertAndGet(t *testing.T) {
	setupRepoTestDB(t)
	ctx := context.Background()

	lc := &repository.TopicLifelineContext{
		PersistentTopicID: 1,
		Granularity:       "week",
		Content:           "本周原油价格波动较大",
		Source:            "llm_assisted",
	}
	if err := repository.Repo.UpsertTopicLifelineContext(ctx, lc); err != nil {
		t.Fatalf("upsert #1: %v", err)
	}

	// Update
	lc.Content = "updated content"
	if err := repository.Repo.UpsertTopicLifelineContext(ctx, lc); err != nil {
		t.Fatalf("upsert #2: %v", err)
	}

	got, err := repository.Repo.GetTopicLifelineContext(ctx, 1, "week")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Content != "updated content" {
		t.Fatalf("content = %q, want 'updated content'", got.Content)
	}
}

// TestTopicLifelineContext_UpsertTOCTOU verifies that rapid sequential upserts
// with the same unique key are atomic (no duplicate-key error).
func TestTopicLifelineContext_UpsertTOCTOU(t *testing.T) {
	setupRepoTestDB(t)
	ctx := context.Background()

	lc1 := &repository.TopicLifelineContext{
		PersistentTopicID: 1,
		Granularity:       "week",
		Content:           "first upsert",
	}
	if err := repository.Repo.UpsertTopicLifelineContext(ctx, lc1); err != nil {
		t.Fatalf("upsert #1: %v", err)
	}

	lc2 := &repository.TopicLifelineContext{
		PersistentTopicID: 1,
		Granularity:       "week",
		Content:           "second upsert",
	}
	if err := repository.Repo.UpsertTopicLifelineContext(ctx, lc2); err != nil {
		t.Fatalf("upsert #2 (TOCTOU): %v", err)
	}

	// Verify only one row exists.
	list, err := repository.Repo.ListTopicLifelineContextsByTopic(ctx, 1)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 row after upserts, got %d", len(list))
	}
	if list[0].Content != "second upsert" {
		t.Fatalf("content = %q, want 'second upsert'", list[0].Content)
	}
}

func TestTopicLifelineContext_ListByTopic(t *testing.T) {
	setupRepoTestDB(t)
	ctx := context.Background()

	for _, g := range []string{"week", "month", "year", "all"} {
		lc := &repository.TopicLifelineContext{
			PersistentTopicID: 1, Granularity: g, Content: g + " summary",
		}
		if err := repository.Repo.UpsertTopicLifelineContext(ctx, lc); err != nil {
			t.Fatalf("upsert %s: %v", g, err)
		}
	}

	list, err := repository.Repo.ListTopicLifelineContextsByTopic(ctx, 1)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 4 {
		t.Fatalf("list count = %d, want 4", len(list))
	}
}

func TestTopicLifelineContext_ListStale(t *testing.T) {
	setupRepoTestDB(t)
	ctx := context.Background()

	// Insert one recent context
	fresh := &repository.TopicLifelineContext{
		PersistentTopicID: 1, Granularity: "week", Content: "fresh",
	}
	if err := repository.Repo.UpsertTopicLifelineContext(ctx, fresh); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// ListStale should find topics with as_of_date before cutoff
	// (fresh records have zero-value as_of_date, which is before now)
	stale, err := repository.Repo.ListStaleTopicLifelineContexts(ctx, "week", 365)
	if err != nil {
		t.Fatalf("list stale: %v", err)
	}
	if len(stale) == 0 {
		t.Fatal("expected stale records with zero as_of_date")
	}
}

// ── TopicEnrichmentResult ───────────────────────────────────────────────────

func TestTopicEnrichmentResult_CreateAndList(t *testing.T) {
	setupRepoTestDB(t)
	ctx := context.Background()

	r1 := &repository.TopicEnrichmentResult{
		PersistentTopicID:   1,
		EvolutionAssessment: "first assessment",
		SessionID:           "session-1",
	}
	if err := repository.Repo.CreateTopicEnrichmentResult(ctx, r1); err != nil {
		t.Fatalf("create #1: %v", err)
	}

	r2 := &repository.TopicEnrichmentResult{
		PersistentTopicID:   1,
		EvolutionAssessment: "second assessment",
		SessionID:           "session-2",
	}
	if err := repository.Repo.CreateTopicEnrichmentResult(ctx, r2); err != nil {
		t.Fatalf("create #2: %v", err)
	}

	list, err := repository.Repo.ListTopicEnrichmentResultsByTopic(ctx, 1)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("list count = %d, want 2", len(list))
	}
	// Should be in descending order by id
	if list[0].ID < list[1].ID {
		t.Fatal("expected descending order by id")
	}
}

func TestTopicEnrichmentResult_GetLatest(t *testing.T) {
	setupRepoTestDB(t)
	ctx := context.Background()

	r1 := &repository.TopicEnrichmentResult{PersistentTopicID: 1, EvolutionAssessment: "first"}
	r2 := &repository.TopicEnrichmentResult{PersistentTopicID: 1, EvolutionAssessment: "latest"}
	_ = repository.Repo.CreateTopicEnrichmentResult(ctx, r1)
	_ = repository.Repo.CreateTopicEnrichmentResult(ctx, r2)

	got, err := repository.Repo.GetLatestTopicEnrichmentResult(ctx, 1)
	if err != nil {
		t.Fatalf("get latest: %v", err)
	}
	if got.EvolutionAssessment != "latest" {
		t.Fatalf("assessment = %q, want 'latest'", got.EvolutionAssessment)
	}
}

func TestTopicEnrichmentResult_GetPrevLatest(t *testing.T) {
	setupRepoTestDB(t)
	ctx := context.Background()

	r1 := &repository.TopicEnrichmentResult{PersistentTopicID: 1, EvolutionAssessment: "prev"}
	r2 := &repository.TopicEnrichmentResult{PersistentTopicID: 1, EvolutionAssessment: "curr"}
	_ = repository.Repo.CreateTopicEnrichmentResult(ctx, r1)
	_ = repository.Repo.CreateTopicEnrichmentResult(ctx, r2)

	prev, err := repository.Repo.GetPrevLatestTopicEnrichmentResult(ctx, 1, r2.ID)
	if err != nil {
		t.Fatalf("get prev: %v", err)
	}
	if prev.EvolutionAssessment != "prev" {
		t.Fatalf("assessment = %q, want 'prev'", prev.EvolutionAssessment)
	}
}

func TestTopicEnrichmentResult_GetByID(t *testing.T) {
	setupRepoTestDB(t)
	ctx := context.Background()

	r := &repository.TopicEnrichmentResult{PersistentTopicID: 1, EvolutionAssessment: "test"}
	_ = repository.Repo.CreateTopicEnrichmentResult(ctx, r)

	got, err := repository.Repo.GetTopicEnrichmentResultByID(ctx, r.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if got.EvolutionAssessment != "test" {
		t.Fatalf("assessment = %q, want 'test'", got.EvolutionAssessment)
	}
}

// ── TopicEnrichmentReview ───────────────────────────────────────────────────

func TestTopicEnrichmentReview_CreateAndList(t *testing.T) {
	setupRepoTestDB(t)
	ctx := context.Background()

	rv := &repository.TopicEnrichmentReview{
		PersistentTopicID: 1,
		CurrResultID:      10,
		DeviationSummary:  "核心判断反转",
		Source:            "llm_assisted",
	}
	if err := repository.Repo.CreateTopicEnrichmentReview(ctx, rv); err != nil {
		t.Fatalf("create: %v", err)
	}
	if rv.ID == 0 {
		t.Fatal("expected ID after create")
	}

	list, err := repository.Repo.ListTopicEnrichmentReviewsByTopic(ctx, 1)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("list count = %d, want 1", len(list))
	}
}

func TestTopicEnrichmentReview_ApplyAndListApplied(t *testing.T) {
	setupRepoTestDB(t)
	ctx := context.Background()

	// Create two reviews, one applied, one not
	rv1 := &repository.TopicEnrichmentReview{
		PersistentTopicID: 1, CurrResultID: 1, DeviationSummary: "applied", Applied: true,
	}
	rv2 := &repository.TopicEnrichmentReview{
		PersistentTopicID: 1, CurrResultID: 2, DeviationSummary: "not applied", Applied: false,
	}
	_ = repository.Repo.CreateTopicEnrichmentReview(ctx, rv1)
	_ = repository.Repo.CreateTopicEnrichmentReview(ctx, rv2)

	applied, err := repository.Repo.ListAppliedTopicEnrichmentReviews(ctx, 1)
	if err != nil {
		t.Fatalf("list applied: %v", err)
	}
	if len(applied) != 1 {
		t.Fatalf("applied count = %d, want 1", len(applied))
	}
	if !applied[0].Applied {
		t.Fatal("expected applied=true")
	}
}

func TestTopicEnrichmentReview_UpdateFields(t *testing.T) {
	setupRepoTestDB(t)
	ctx := context.Background()

	rv := &repository.TopicEnrichmentReview{
		PersistentTopicID: 1, CurrResultID: 1, DeviationSummary: "original summary",
	}
	_ = repository.Repo.CreateTopicEnrichmentReview(ctx, rv)

	// Update deviation summary
	if err := repository.Repo.UpdateTopicEnrichmentReviewDeviation(ctx, rv.ID, "updated summary"); err != nil {
		t.Fatalf("update deviation: %v", err)
	}

	got, err := repository.Repo.GetTopicEnrichmentReviewByID(ctx, rv.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.DeviationSummary != "updated summary" {
		t.Fatalf("deviation = %q, want 'updated summary'", got.DeviationSummary)
	}

	// Apply
	if err := repository.Repo.ApplyTopicEnrichmentReview(ctx, rv.ID); err != nil {
		t.Fatalf("apply: %v", err)
	}

	got, _ = repository.Repo.GetTopicEnrichmentReviewByID(ctx, rv.ID)
	if !got.Applied {
		t.Fatal("expected applied=true after ApplyTopicEnrichmentReview")
	}
}
