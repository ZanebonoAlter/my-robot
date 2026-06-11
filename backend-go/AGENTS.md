# Backend Agent Guide

遵循根 `AGENTS.md` 的所有规则。以下为后端特有差异。

## Backend-Specific Conventions
- Routes in `internal/app/router.go`. Business logic in `internal/<domain>/service/`.
- PostgreSQL + pgvector for persistence. Use Docker: `pgvector/pgvector:pg18-trixie`.
- Handler response: `gin.H{"success": bool, "data"|"error"|"message": ...}`.
- JSON struct tags: `snake_case`. Wrap errors with `fmt.Errorf(... %w ...)`.
- Imports: stdlib → blank line → third-party → blank line → local.
- Naming: PascalCase exported, lowerCamelCase private. Short package names.
- Validate params before touching DB. Early returns for errors.

## Package Organization Convention

All domain packages follow a **unified three-layer structure**:

```
internal/<domain>/
├── routes.go        # route registration (called by app/router.go)
├── wire.go          # singleton init + re-exports for external callers
├── handler/         # Gin handlers (package handler)
├── service/         # business logic (package service)
└── repository/      # data access (package repository)
```

**Rules:**
- Root package only contains `routes.go` and `wire.go`. No handler/service/repository files.
- Handlers go in `handler/` sub-package (package name: `handler`).
- `wire.go` re-exports types/functions that external packages need, so callers import only the root package.
- `routes.go` imports `handler/` and wires routes to handler functions.

**Current domain packages:** `admin`, `reader`, `tagmanagement`, `topicgraph` — all follow this pattern.

## Anti-Patterns
- No business logic in `router.go`. No direct DB access from handlers. No `panic` for errors.
- **Don't put handler files in the domain root package.** Always create a `handler/` subdirectory.
- Don't bypass `wire.go` — if external packages need a symbol, add a re-export in `wire.go`.

## Commands
```bash
go mod tidy  &&  go run cmd/server/main.go
golangci-lint run ./...  &&  go vet ./...
go test ./...  &&  go build ./...
# Single: go test ./internal/admin/scheduler -run TestName -v
```
