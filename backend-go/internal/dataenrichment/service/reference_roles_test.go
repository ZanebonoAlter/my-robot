package service_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"syntopica-backend/internal/dataenrichment/repository"
	"syntopica-backend/internal/dataenrichment/service"
)

// ── Reference role appendix injection (board-level-deep-analysis tasks 2.2 / M7) ──

func newRoleOrch(t *testing.T) (*service.OrchestratorService, *repository.Repository) {
	t.Helper()
	repo := setupOrchTestDB(t)
	toolRegistry := service.NewRegistry(&nilFetcher{})
	boardReader := &mockBoardConfigReader{cfg: service.DefaultBoardConfig()}
	orch := service.NewOrchestratorService(
		newMockAirRouter(), repo, &orchMockLifelineReader{}, service.NewLifelineRenderer(),
		toolRegistry, boardReader, testCap,
	)
	return orch, repo
}

// M7.4 + M7.5: empty/disabled library → empty appendix (prompt byte-identical
// to the no-feature state); reads hit the DB fresh on every call.
func TestReferenceRoleAppendix_EmptyAndDisabled(t *testing.T) {
	orch, repo := newRoleOrch(t)
	ctx := context.Background()

	if got := orch.ReferenceRoleAppendixForTest(ctx); got != "" {
		t.Fatalf("empty library: want empty appendix, got %q", got)
	}

	// Disabled roles are excluded.
	if err := repo.CreateReferenceRole(ctx, &repository.ReferenceRole{
		Name: "off", Content: "方法", Enabled: false,
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if got := orch.ReferenceRoleAppendixForTest(ctx); got != "" {
		t.Fatalf("disabled-only library: want empty appendix, got %q", got)
	}
}

// M7.1 + M7.2: enabled roles inject in updated_at DESC order under a fixed header.
func TestReferenceRoleAppendix_FormatAndOrder(t *testing.T) {
	orch, repo := newRoleOrch(t)
	ctx := context.Background()

	old := &repository.ReferenceRole{Name: "older", Title: "旧角色", Content: "旧方法内容", Enabled: true}
	newer := &repository.ReferenceRole{Name: "newer", Title: "新角色", Content: "新方法内容", Enabled: true}
	// Deterministic ordering: explicit UpdatedAt one hour apart (raw-SQL bumps
	// clash with GORM's timestamp format and break lexicographic DESC order).
	now := time.Now()
	old.CreatedAt, old.UpdatedAt = now, now
	newer.CreatedAt, newer.UpdatedAt = now, now.Add(time.Hour)
	if err := repo.CreateReferenceRole(ctx, old); err != nil {
		t.Fatalf("seed old: %v", err)
	}
	if err := repo.CreateReferenceRole(ctx, newer); err != nil {
		t.Fatalf("seed newer: %v", err)
	}

	got := orch.ReferenceRoleAppendixForTest(ctx)
	if !strings.Contains(got, "【分析方法参考】") {
		t.Fatalf("appendix missing fixed header: %q", got[:60])
	}
	if !strings.Contains(got, "只给方法，不给任何事实/结论") {
		t.Fatalf("appendix missing method-not-fact guardrail line")
	}
	// Order: newer (updated_at DESC) first.
	iNew, iOld := strings.Index(got, "新方法内容"), strings.Index(got, "旧方法内容")
	if iNew < 0 || iOld < 0 || iNew > iOld {
		t.Fatalf("appendix order wrong: newer at %d, older at %d", iNew, iOld)
	}
	// Entry format: markdown heading + name marker.
	if !strings.Contains(got, "## 新角色（newer）") {
		t.Fatalf("appendix entry format wrong: %q", got[:120])
	}
}

// M7.3: whole-entry truncation at the ~4k cap — oversized entries drop whole,
// the appendix never exceeds the cap, and a mid-entry slice cannot leak.
func TestReferenceRoleAppendix_TruncationCap(t *testing.T) {
	orch, repo := newRoleOrch(t)
	ctx := context.Background()

	fits := &repository.ReferenceRole{Name: "fits", Title: "小角色", Content: strings.Repeat("甲", 100), Enabled: true}
	if err := repo.CreateReferenceRole(ctx, fits); err != nil {
		t.Fatalf("seed fits: %v", err)
	}
	huge := &repository.ReferenceRole{Name: "huge", Title: "巨角色", Content: strings.Repeat("乙", 5000), Enabled: true}
	if err := repo.CreateReferenceRole(ctx, huge); err != nil {
		t.Fatalf("seed huge: %v", err)
	}

	got := orch.ReferenceRoleAppendixForTest(ctx)
	if len(got) > service.ReferenceRoleInjectionCapForTest()+200 { // header + entry chrome slack
		t.Fatalf("appendix exceeds cap: %d chars", len(got))
	}
	if strings.Contains(got, "乙") {
		t.Fatalf("oversized entry must drop whole, not slice: appendix contains truncated body")
	}
	if !strings.Contains(got, "甲") {
		t.Fatalf("small entry must survive: appendix missing fits role")
	}
}
