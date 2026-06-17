## MODIFIED Requirements

### Requirement: development.md SHALL use correct Go version

The `development.md` document SHALL specify the correct Go version requirement.

#### Scenario: Go version matches go.mod
- **WHEN** a developer reads the "前置条件" section
- **THEN** the Go version requirement SHALL be `1.25+` (matching `go.mod`'s `go 1.25.0`)

### Requirement: development.md SHALL describe current backend directory structure

The `development.md` document SHALL reflect the current backend directory conventions.

#### Scenario: Backend directory conventions are accurate
- **WHEN** a developer reads the "后端目录约定" section
- **THEN** the directory table SHALL list:
  - `cmd/server/` - 应用入口
  - `internal/app/` - HTTP 路由、中间件、运行时装配
  - `internal/admin/` - 管理后台（AI、调度器、偏好）
  - `internal/reader/` - 订阅与文章域
  - `internal/tagmanagement/` - 标签系统域
  - `internal/topicgraph/` - 主题图谱域
  - `internal/platform/` - 共享基础设施
  - `internal/models/` - 共享 GORM 模型
  - `configs/` - 配置文件
- **AND** SHALL NOT reference `internal/domain/*` as the primary business logic location
- **AND** the platform description SHALL NOT list `ai` or `opennotebook`

### Requirement: development.md SHALL NOT list deleted cmd commands

The `development.md` document SHALL NOT document `cmd/` commands that do not exist.

#### Scenario: Auxiliary commands table is accurate
- **WHEN** a developer reads the "辅助工具命令" section
- **THEN** the table SHALL NOT list `cmd/migrate-digest`, `cmd/test-digest`, `cmd/migrate-tags`, or `cmd/migrate-db` (none exist; `backend-go/cmd/` contains only `server/`)

### Requirement: development.md SHALL use correct test example paths

The `development.md` document SHALL use backend test paths that exist in the codebase.

#### Scenario: Test command examples are accurate
- **WHEN** a developer reads the backend test examples
- **THEN** the examples SHALL NOT reference `internal/domain/feeds` (deleted)
- **AND** SHALL reference `internal/reader/service` (the current location of feed service tests)

### Requirement: 开发执行规范.md SHALL use correct backend paths

The `开发执行规范.md` document SHALL reference backend paths that exist in the codebase.

#### Scenario: Test convention references are accurate
- **WHEN** a developer reads the test conventions section
- **THEN** the document SHALL NOT reference `internal/domain/feeds/service_test.go` (deleted)
- **AND** SHALL reference `internal/reader/service/feed_service_test.go` (the current location)

#### Scenario: Layering convention is accurate
- **WHEN** a developer reads the backend layering convention ("业务逻辑在 ...")
- **THEN** the document SHALL NOT state that business logic lives in `internal/domain/*`
- **AND** SHALL state that business logic lives in the domain packages (`internal/reader/`, `internal/tagmanagement/`, `internal/topicgraph/`, `internal/admin/`)

#### Scenario: No deleted cmd references
- **WHEN** a developer reads the database migration checklist
- **THEN** the document SHALL NOT reference `cmd/migrate-db/` (deleted)
