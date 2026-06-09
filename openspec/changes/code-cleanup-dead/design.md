# Design: code-cleanup-dead

## Context

The backend and frontend have accumulated dead and deprecated code over multiple iterations. Key symptoms:

### Backend
- `NarrativeSummaryScheduler` is marked `Deprecated` but still runs daily via the scheduler
- `narrative/service.go` contains methods (`GenerateAndSaveGlobal`, `GenerateAndSaveForAllCategories`) with zero callers
- `narrative/watched_narrative.go` has ~200 lines of unused functions
- `narrative/board_narrative_generator.go` has unused `SaveNarrativesForBoard` (~60 lines) and `LoadBoardEventTags` (~40 lines)
- `narrative/board_collector.go` has 2 functions (`CollectPreviousDayBoards`, `CollectPreviousBoardNarratives`) with zero callers — entire file dead
- `narrative/collector.go` has `CollectCategoryNarrativeSummaries` with zero callers
- `narrative/generator.go` has `GenerateNarratives` with zero production callers (1 test only)
- `platform/database/datamigrate/` is an entirely unused package (8 functions, zero imports anywhere) — SQLite→PG migration tooling from an old migration
- `jobs/cleanup_budget.go` (`CleanupBudget` type) has zero production callers — only used in its own tests
- `database/migrator.go` has `autoMigrateModels()` with zero callers
- `TopicTag.Kind` is marked deprecated but still written in 12+ places across `topicgraph/service.go` and `tagging/tagger.go`
- `airouter/embedding.go` contains a hand-rolled Newton's method `sqrt` instead of `math.Sqrt`
- CST timezone constant is duplicated in 6+ files (inline `time.FixedZone(...)` calls)
- `database/db.go` has unused `Migrate()` and `EnsureTables()` functions

### Frontend
- `front/app/api/topicGraph.ts`: `useNarrativeApi()` composable (8 API methods) is defined but never called
- `front/app/api/topicGraph.ts`: 7 narrative-related types only used by dead API methods
- `front/app/features/topic-graph/components/NarrativeDetailCard.vue`: zero imports, ~210 lines
- `front/app/features/tags/components/BoardNarrativeTimeline.vue`: zero imports, ~120 lines
- `front/app/api/semanticBoards.ts`: `getBoardNarratives` and `triggerNarrativeGeneration` are dead methods

This cleanup is scheduled **before** the `scheduler-cron` change (which will refactor the scheduler registry) and the `narrative-scope-query` normalization to minimize interference.

## Goals

1. **Delete dead code** — Remove all functions/methods with zero callers listed above, along with their tests
2. **Remove NarrativeSummaryScheduler** — Delete the entire `jobs/narrative_summary.go` file and its registration in `runtime.go`, `handler.go`, and `runtimeinfo/schedulers.go`
3. **Delete dead narrative helpers** — Remove `board_collector.go` entirely, plus dead functions in `board_narrative_generator.go`, `collector.go`, `generator.go`
4. **Delete entirely unused packages** — Remove `platform/database/datamigrate/` and `jobs/cleanup_budget.go`
5. **Eliminate TopicTag.Kind writes** — Remove all `.Kind =` assignments; keep only `.Category` as the authoritative field. Do not remove the `Kind` field from the struct yet (that requires a DB migration)
6. **Replace hand-rolled sqrt** — Swap Newton's method in `airouter/embedding.go` with `math.Sqrt`
7. **Unify CST timezone** — Consolidate to a single `CST` constant in `models/utils.go`; update all call sites to import from there
8. **Remove dead tests** — Delete ~13 deprecated test cases in `narrative/service_test.go` and `cleanup_budget_test.go`
9. **Clean up frontend dead code** — Remove dead narrative API module, types, and components from the frontend

## Non-Goals

- **Scheduler registry refactor** — The `runtimeinfo/schedulers.go` service locator (9 `interface{}` vars) will be fully refactored in the `scheduler-cron` change. This change only removes the `NarrativeSummarySchedulerInterface` var.
- **BlockedArticleRecovery scheduler** — Runs but not in `schedulerDescriptors`; leave for `scheduler-cron` change.
- **Remove `Kind` field from `TopicTag` struct** — Requires DB migration; defer to a schema-focused change.
- **Simplify `normalizeTopicCategory()` frontend fallback** — When backend stops writing `Kind`, the `kind` parameter will always be undefined. Simplification of this fallback logic is deferred to the `Kind` field removal change.
- **Other zero-caller functions in `content/`, `preferences/`, `daily_report/repository.go`, `tagging/`** — 40+ additional dead functions found but out of scope for this change. Can be addressed in a follow-up cleanup pass.
- **Dead platform-layer functions** (`logging.ConfigureStdlib`, `logging.Errorln`, `tracing.TraceIDFromContext`, `tracing.MustStartSpan`, `tracing.TraceAsyncOp`, `ai.TestConnection`, `jsonutil.TruncateStr`, `database.SlowLogger.Trace`) — low priority, deferred to future cleanup.
- **Any API or DB schema changes** — This is pure code deletion and internal cleanup.

## Decisions

### D1: Deletion order — tests first, then implementation

Delete the deprecated test cases first so they don't reference functions that are about to be removed. Then delete the dead functions/files.

### D2: `NarrativeSummaryScheduler` — full file deletion

The entire `jobs/narrative_summary.go` (390 lines) is deleted. Its registration in:
- `runtime.go` (startup)
- `handler.go` (scheduler descriptor)
- `runtimeinfo/schedulers.go` (`NarrativeSummarySchedulerInterface` var)

is removed in the same commit.

### D3: `TopicTag.Kind` — remove writes only

Remove all `.Kind = ...` assignments in `topicgraph/service.go` and `tagging/tagger.go`. The struct field `Kind` remains (no DB migration). Readers that still check `.Kind` will simply see the zero value.

### D4: CST timezone — `models/utils.go` as single source

`models/utils.go` already defines a CST variable (`shanghaiTZ`). All other definitions (in `database/db.go`, `tagging/services.go`, `content/content_completion_service.go`, `feed/handler.go`, `feed/rss_parser.go`, `feed/service.go`, and inline `time.FixedZone(...)` calls) will be replaced with an import from `models/utils.go`. No new package is introduced.

### D5: `math.Sqrt` replacement — straightforward swap

The hand-rolled Newton's method in `airouter/embedding.go` (lines 43-57) is replaced with `math.Sqrt`. The function signature and callers are unchanged. The `sqrt` function itself is inlined or deleted.

### D6: `narrative/watched_narrative.go` — delete file entirely

Both `GenerateWatchedTagNarratives` and `generateSingleWatchedNarrative` are dead. The file also contains `WatchedTagNarrativeOutput` which is only used by the dead functions. Delete the entire file.

### D7: `narrative/board_narrative_generator.go` — delete dead functions only

Remove `SaveNarrativesForBoard` (~60 lines) and `LoadBoardEventTags` (~40 lines). Other board narrative generation functions that have callers remain.

### D8: `narrative/board_collector.go` — delete entire file

Both `CollectPreviousDayBoards` and `CollectPreviousBoardNarratives` have zero callers. `PreviousBoardBrief` and `BoardNarrativeBrief` types are only used by these functions. Delete entire file (97 lines).

### D9: `narrative/collector.go` — delete `CollectCategoryNarrativeSummaries` only

Only `CollectCategoryNarrativeSummaries` is dead (category-level narratives replaced by daily_report). Other collector functions (e.g., `CollectSemanticBoardNarrativeInputs`) are still active.

### D10: `narrative/generator.go` — delete `GenerateNarratives` only

`GenerateNarratives` has zero production callers. If `NarrativeOutput` type is only used by dead code, delete it too. Otherwise keep the type.

### D11: `platform/database/datamigrate/` — delete entire directory

This SQLite→PostgreSQL migration tooling package (8 exported functions across 4 files, ~350 lines) has zero imports from the rest of the codebase. It was a one-time migration utility and is no longer needed. Delete the entire directory including tests.

### D12: `jobs/cleanup_budget.go` — delete entire file + tests

`CleanupBudget` type (5 methods) has zero production callers. Only referenced in its own test file `cleanup_budget_test.go`. Delete both files (~150 lines total).

### D13: `database/migrator.go` — delete `autoMigrateModels()`

This deprecated wrapper at line 146 just calls `RunAutoMigrate(db)` and has zero callers. Delete it.

### D14: Frontend dead code — delete files and unused exports

- Delete `front/app/features/topic-graph/components/NarrativeDetailCard.vue` entirely (210 lines, zero imports)
- Delete `front/app/features/tags/components/BoardNarrativeTimeline.vue` entirely (~120 lines, zero imports)
- Remove `useNarrativeApi()` function and all 8 API methods from `front/app/api/topicGraph.ts`
- Remove dead types: `NarrativeItem`, `NarrativeTimelineDay`, `NarrativeScopesResponse`, `BoardNarrativeItem`, `BoardItem`, `TagBrief` (topicGraph.ts), `BoardTimelineDay`
- Remove `getBoardNarratives` and `triggerNarrativeGeneration` from `front/app/api/semanticBoards.ts`
- Remove `BoardNarrative` and `BoardNarrativeTag` types from `front/app/api/semanticBoards.ts` if only used by dead methods

## Risks / Trade-offs

| Risk | Mitigation |
|------|-----------|
| Hidden callers via reflection or dynamic dispatch | Grep for all symbol names before deletion; `go build ./...` will catch compile errors |
| `Kind` zero value changes behavior in a reader somewhere | Grep for all `.Kind` reads to verify they are either deprecated or have a `Category` fallback |
| CST timezone consolidation misses an inline `time.FixedZone("CST", 8*3600)` | Grep for `FixedZone`, `28800`, `8*3600` across all Go files |
| Removing `NarrativeSummaryScheduler` breaks runtime startup | Scheduler registration removal is done alongside file deletion; `go build` verifies |
| `math.Sqrt` precision differs from Newton's method | `math.Sqrt` is IEEE 754 compliant and strictly more accurate; no behavioral regression |
| Deleting `datamigrate/` removes migration capability | Package has zero imports; was a one-time SQLite migration tool no longer needed |
| `narrative/generator.go` `NarrativeOutput` type used elsewhere | Check references before deleting; keep type if still used by `board_narrative_generator.go` |
| Frontend `useNarrativeApi` removal breaks narrative pages | Verified zero callers of the composable; narrative pages use `useSemanticBoardsApi()` for board narratives |
