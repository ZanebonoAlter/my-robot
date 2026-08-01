# Competitive Filter Implementation Plan

> **SUPERSEDED:** The `competitiveFilter` and `shouldWriteRelation` functions described here have been replaced by Hungarian bipartite matching (`RebuildBoardRelations`). See `docs/archive/plans/2026-06-06-bipartite-relation-matching.md`.

> **REQUIRED SUB-SKILL:** Use the executing-plans skill to implement this plan task-by-task.

**Goal:** Add competitive filtering to `MatchAndSaveRelations` and `BackfillRelations` so that each section only retains its best-matching relations, reducing merge/split noise in dense boards.

**Architecture:** Extract a pure `competitiveFilter` function that takes candidates passing `shouldWriteRelation` and retains only the best (gap ≥ 0.03 → keep #1 only) or a tight cluster (gap < 0.03 → keep all within best + 0.03). Insert this filter between `shouldWriteRelation` and DB write in both `MatchAndSaveRelations` and `BackfillRelations`.

**Tech Stack:** Go 1.x, testing/testify, pgvector

---

### Task 1: competitiveFilter 纯函数 + 单元测试

**Files:**
- Create: `backend-go/internal/domain/daily_report/match_relations_test.go` (append tests)
- Modify: `backend-go/internal/domain/daily_report/repository.go` (add `matchCandidate` struct + `competitiveFilter` function)

**Context:**
- `match_relations_test.go` already has 7 tests for `shouldWriteRelation`. Append new tests after existing ones.
- `repository.go` line ~530 (after `hasContinuationInIntermediateDays`) is where to add the new struct and function.
- `shouldWriteRelation` currently decides per-candidate. `competitiveFilter` runs after, looking at all candidates for one section together.

**Step 1: Write the failing tests**

Append to `match_relations_test.go`:

```go
func TestCompetitiveFilter_Empty(t *testing.T) {
	result := competitiveFilter(nil)
	require.Len(t, result, 0)
}

func TestCompetitiveFilter_SingleCandidate(t *testing.T) {
	candidates := []matchCandidate{
		{FromID: 100, Distance: 0.20},
	}
	result := competitiveFilter(candidates)
	require.Len(t, result, 1)
	require.Equal(t, uint(100), result[0].FromID)
}

func TestCompetitiveFilter_GapAboveThreshold_KeepBest(t *testing.T) {
	// best=0.15, 2nd=0.22, gap=0.07 ≥ 0.03 → only keep best
	candidates := []matchCandidate{
		{FromID: 100, Distance: 0.15},
		{FromID: 101, Distance: 0.22},
		{FromID: 102, Distance: 0.30},
	}
	result := competitiveFilter(candidates)
	require.Len(t, result, 1)
	require.Equal(t, uint(100), result[0].FromID)
}

func TestCompetitiveFilter_GapBelowThreshold_KeepCluster(t *testing.T) {
	// best=0.20, 2nd=0.22, gap=0.02 < 0.03 → keep all ≤ 0.20+0.03=0.23
	candidates := []matchCandidate{
		{FromID: 100, Distance: 0.20},
		{FromID: 101, Distance: 0.22},
		{FromID: 102, Distance: 0.24}, // > 0.23, filtered
		{FromID: 103, Distance: 0.28}, // > 0.23, filtered
	}
	result := competitiveFilter(candidates)
	require.Len(t, result, 2)
	require.Equal(t, uint(100), result[0].FromID)
	require.Equal(t, uint(101), result[1].FromID)
}

func TestCompetitiveFilter_ExactGapThreshold_KeepBest(t *testing.T) {
	// best=0.10, 2nd=0.13, gap=0.03 == 0.03 → only keep best (≥ means keep best only)
	candidates := []matchCandidate{
		{FromID: 100, Distance: 0.10},
		{FromID: 101, Distance: 0.13},
	}
	result := competitiveFilter(candidates)
	require.Len(t, result, 1)
	require.Equal(t, uint(100), result[0].FromID)
}

func TestCompetitiveFilter_AllSimilarDistances_KeepAll(t *testing.T) {
	// best=0.298, 2nd=0.300, gap=0.002 < 0.03 → keep all ≤ 0.298+0.03=0.328
	candidates := []matchCandidate{
		{FromID: 100, Distance: 0.298},
		{FromID: 101, Distance: 0.300},
		{FromID: 102, Distance: 0.300},
		{FromID: 103, Distance: 0.303},
	}
	result := competitiveFilter(candidates)
	require.Len(t, result, 4)
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /mnt/d/project/Syntopica/backend-go && go test ./internal/domain/daily_report/ -run TestCompetitiveFilter -v`
Expected: compile error — `matchCandidate` and `competitiveFilter` not defined

**Step 3: Write minimal implementation**

Add after `hasContinuationInIntermediateDays` in `repository.go` (after line ~540):

```go
// matchCandidate represents a relation candidate that passed shouldWriteRelation,
// awaiting competitive filtering.
type matchCandidate struct {
	FromID   uint
	FromDate time.Time
	Distance float64
}

// competitiveFilter applies competitive matching to a section's relation candidates.
//
// Rules:
//   - 0 or 1 candidates → pass through
//   - If gap between best and 2nd ≥ 0.03 → keep only best
//   - If gap < 0.03 → keep all candidates with distance ≤ best + 0.03
func competitiveFilter(candidates []matchCandidate) []matchCandidate {
	if len(candidates) <= 1 {
		return candidates
	}

	// Sort by distance ascending
	slices.SortFunc(candidates, func(a, b matchCandidate) int {
		switch {
		case a.Distance < b.Distance:
			return -1
		case a.Distance > b.Distance:
			return 1
		default:
			return 0
		}
	})

	best := candidates[0].Distance
	gap := candidates[1].Distance - best

	if gap >= 0.03 {
		// Significant gap: only keep the best match
		return candidates[:1]
	}

	// Tight cluster: keep all within best + 0.03
	threshold := best + 0.03
	end := len(candidates)
	for i, c := range candidates {
		if c.Distance > threshold {
			end = i
			break
		}
	}
	return candidates[:end]
}
```

Also add `"slices"` to imports if not already there.

**Step 4: Run tests to verify they pass**

Run: `cd /mnt/d/project/Syntopica/backend-go && go test ./internal/domain/daily_report/ -run TestCompetitiveFilter -v`
Expected: all 6 tests PASS

**Step 5: Commit**

```bash
git add backend-go/internal/domain/daily_report/repository.go backend-go/internal/domain/daily_report/match_relations_test.go
git commit -m "feat: add competitiveFilter for relation candidate filtering"
```

---

### Task 2: 集成 competitiveFilter 到 MatchAndSaveRelations

**Files:**
- Modify: `backend-go/internal/domain/daily_report/repository.go` (MatchAndSaveRelations function, ~line 445-470)

**Context:**
Current flow in `MatchAndSaveRelations` (per section):
```go
for _, m := range matches {
    if !shouldWriteRelation(...) { continue }
    tx.Exec(INSERT ...)
}
```

New flow:
```go
var candidates []matchCandidate
for _, m := range matches {
    if !shouldWriteRelation(...) { continue }
    candidates = append(candidates, matchCandidate{...})
}
survivors := competitiveFilter(candidates)
for _, c := range survivors {
    tx.Exec(INSERT, c.FromID, ...)
}
```

**Step 1: Modify MatchAndSaveRelations**

Replace the inner loop in `MatchAndSaveRelations` (the `for _, m := range matches` block) with:

```go
		// Collect candidates that pass time-dimension filtering
		var candidates []matchCandidate
		for _, m := range matches {
			if !shouldWriteRelation(m.MatchID, m.MatchDate, sec.ID, reportDate, m.Distance, adjacency, sectionDateMap, dateSet) {
				continue
			}
			candidates = append(candidates, matchCandidate{
				FromID:   m.MatchID,
				FromDate: m.MatchDate,
				Distance: m.Distance,
			})
		}

		// Apply competitive filtering
		survivors := competitiveFilter(candidates)

		// Write surviving relations
		for _, c := range survivors {
			if err := tx.Exec(`
				INSERT INTO daily_report_section_relations (from_section_id, to_section_id, distance)
				VALUES (?, ?, ?)
				ON CONFLICT (from_section_id, to_section_id) DO UPDATE SET distance = EXCLUDED.distance
			`, c.FromID, sec.ID, c.Distance).Error; err != nil {
				logging.Warnf("MatchAndSaveRelations: save relation failed: %v", err)
			} else {
				adjacency[c.FromID] = append(adjacency[c.FromID], sec.ID)
				sectionDateMap[sec.ID] = reportDate
			}
		}
```

**Step 2: Build to verify compilation**

Run: `cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go build ./..."`
Expected: success, no errors

**Step 3: Run existing tests**

Run: `cd /mnt/d/project/Syntopica/backend-go && go test ./internal/domain/daily_report/ -v`
Expected: all existing tests still pass (competitiveFilter doesn't break shouldWriteRelation tests)

**Step 4: Commit**

```bash
git add backend-go/internal/domain/daily_report/repository.go
git commit -m "feat: integrate competitiveFilter into MatchAndSaveRelations"
```

---

### Task 3: 集成 competitiveFilter 到 BackfillRelations

**Files:**
- Modify: `backend-go/internal/domain/daily_report/repository.go` (BackfillRelations function, ~line 830-855)

**Context:**
Same pattern as Task 2 but in `BackfillRelations`. The inner loop (`for _, m := range matches`) has the identical structure. Apply the same collect → filter → write pattern.

**Step 1: Modify BackfillRelations inner loop**

Replace the `for _, m := range matches` block in `BackfillRelations` with:

```go
		// Collect candidates that pass time-dimension filtering
		var candidates []matchCandidate
		for _, m := range matches {
			if !shouldWriteRelation(m.MatchID, m.MatchDate, sec.ID, sec.PeriodDate, m.Distance, adjacency, sectionDateMap, dateSet) {
				continue
			}
			candidates = append(candidates, matchCandidate{
				FromID:   m.MatchID,
				FromDate: m.MatchDate,
				Distance: m.Distance,
			})
		}

		// Apply competitive filtering
		survivors := competitiveFilter(candidates)

		// Write surviving relations
		for _, c := range survivors {
			if wErr := tx.Exec(`
				INSERT INTO daily_report_section_relations (from_section_id, to_section_id, distance)
				VALUES (?, ?, ?)
				ON CONFLICT (from_section_id, to_section_id) DO UPDATE SET distance = EXCLUDED.distance
			`, c.FromID, sec.ID, c.Distance).Error; wErr != nil {
				logging.Warnf("BackfillRelations: write relation failed: %v", wErr)
			} else {
				adjacency[c.FromID] = append(adjacency[c.FromID], sec.ID)
				rebuilt++
			}
		}
```

**Step 2: Build to verify compilation**

Run: `cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go build ./..."`
Expected: success

**Step 3: Run tests**

Run: `cd /mnt/d/project/Syntopica/backend-go && go test ./internal/domain/daily_report/ -v`
Expected: all pass

**Step 4: Commit**

```bash
git add backend-go/internal/domain/daily_report/repository.go
git commit -m "feat: integrate competitiveFilter into BackfillRelations"
```

---

### Task 4: 验证 — 回刷 board 2853 + 3639，检查状态分布

**Files:** No code changes — verification only

**Step 1: Restart backend with new code**

Run: `cd backend-go && go run cmd/server/main.go`

**Step 2: Backfill board 2853 (伊朗局势)**

Run: `curl -s -X POST "http://localhost:5000/api/daily-reports/backfill-relations?board_id=2853"`

**Step 3: Query status distribution for 2853**

Query the section-timeline endpoint and compute status counts per date. Expect:
- 6/3 sections: in-degree drops from 4-5 to 1-2
- `continuing` increases, `merge` decreases significantly

**Step 4: Backfill board 3639 (AI 技术)**

Run: `curl -s -X POST "http://localhost:5000/api/daily-reports/backfill-relations?board_id=3639"`

**Step 5: Check 3639 status distribution**

Expect:
- 6/3 sections that had in=8-12 now have in=1-3
- Clear `continuing` chains emerge (e.g., #940 Agent架构 → #1000 should be 1:1)

**Step 6: Mark tasks 5.1-5.4 complete in tasks.md**

Update checkboxes in `openspec/changes/relation-skip-day-filter/tasks.md`.
