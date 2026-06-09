# Tasks: test-standardize

## 1. Create testutil Package

- [ ] 1.1 Create `backend-go/internal/platform/testutil/testutil.go` with `OpenTestDB(t *testing.T) *gorm.DB` — reads `TEST_DB_DSN` env var (default `host=localhost port=5432 user=postgres password=postgres dbname=syntopica sslmode=disable`), connects to Postgres, fails with clear message if unavailable
- [ ] 1.2 Add `SetupTestDB(t *testing.T) *gorm.DB` — calls `OpenTestDB`, runs `AutoMigrate` on all domain models, calls `TruncateAllTables`, returns `*gorm.DB`
- [ ] 1.3 Add `TruncateAllTables(t *testing.T, db *gorm.DB)` — queries `information_schema.tables` for user tables, executes `TRUNCATE TABLE ... CASCADE` in reverse-dependency order
- [ ] 1.4 Verify testutil compiles: `cd backend-go && go build ./internal/platform/testutil/...`

## 2. Migrate Priority Test Files (tagging/)

- [ ] 2.1 Migrate `backend-go/internal/domain/tagging/semantic_board_matching_test.go` — replace `setupXxxTestDB()` with `testutil.SetupTestDB(t)`, pass `*gorm.DB` explicitly instead of setting `database.DB`, remove SQLite import
- [ ] 2.2 Migrate `backend-go/internal/domain/tagging/embedding_test.go` — replace SQLite setup with `testutil.SetupTestDB(t)`, adapt vector column assertions to Postgres semantics
- [ ] 2.3 Migrate `backend-go/internal/domain/tagging/auxiliary_label_service_test.go` — replace SQLite setup with `testutil.SetupTestDB(t)`, ensure label matching tests work with pgvector similarity queries
- [ ] 2.4 Migrate `backend-go/internal/domain/tagging/semantic_board_upgrade_test.go` — replace SQLite setup with `testutil.SetupTestDB(t)`, verify upgrade suggestion tests pass against Postgres
- [ ] 2.5 Migrate `backend-go/internal/domain/tagging/semantic_board_handler_test.go` — replace SQLite setup with `testutil.SetupTestDB(t)`, verify handler tests pass against Postgres

## 3. Remove SQLite Fallback from Production Code

- [ ] 3.1 In `backend-go/internal/domain/tagging/auxiliary_label_service.go`, evaluate `sqlMergeMatcher` Go-side cosine computation: benchmark pgvector SQL vs Go-side path for 2560-dim vectors per design decision D5
- [ ] 3.2 If pgvector SQL is comparable or faster, replace Go-side cosine computation in `sqlMergeMatcher` with pgvector `ORDER BY embedding <=> $1` query
- [ ] 3.3 If Go-side is faster, remove SQLite-specific comments and reframe as intentional performance optimization (not SQLite fallback)
- [ ] 3.4 Verify production code no longer has SQLite-specific code paths: `grep -r "sqlite\|SQLite" backend-go/internal/domain/tagging/auxiliary_label_service.go`

## 4. Remove SQLite Dependency

- [ ] 4.1 After all 5 tagging test files are migrated, verify no remaining test files in `backend-go/internal/domain/tagging/` import `github.com/glebarez/sqlite`
- [ ] 4.2 Check if other packages (aiadmin, article, content, feed, narrative, topicgraph) still import `github.com/glebarez/sqlite` — if yes, dependency stays in `go.mod` for now (out of scope per design)
- [ ] 4.3 If all SQLite imports are removed across the codebase, run `cd backend-go && go mod tidy` to remove `github.com/glebarez/sqlite` from `go.mod`

## 5. Define Test Tier Convention

- [ ] 5.1 Add test tier documentation to `backend-go/AGENTS.md` or `docs/reference/testing.md`: unit tests (`go test -short`) use stubs/mocks without DB; integration tests (`go test`) require Postgres via `TEST_DB_DSN`
- [ ] 5.2 Ensure each migrated test file checks `testing.Short()` at the top of each test function — skip with `t.Skip("requires Postgres")` when `-short` is set (consistent with existing `tag_context_dump_test.go` precedent)

## 6. Verify

- [ ] 6.1 Ensure Docker Postgres is running: `docker compose -f docker-compose.pg.yml up -d`
- [ ] 6.2 Run migrated tagging tests: `cd backend-go && go test ./internal/domain/tagging/... -v` — all pass
- [ ] 6.3 Run with `-short` flag: `cd backend-go && go test ./internal/domain/tagging/... -short` — migrated tests are skipped, non-DB tests still run
- [ ] 6.4 Run full backend build: `cd backend-go && go build ./...` — no compilation errors
- [ ] 6.5 Run linter on changed packages: `cd backend-go && golangci-lint run ./internal/platform/testutil/... ./internal/domain/tagging/...`
