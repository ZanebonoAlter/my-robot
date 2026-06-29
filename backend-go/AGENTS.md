# Backend Agent Guide

遵循根 `AGENTS.md` 的所有规则。以下为后端特有差异。openspec change 执行走 `docs/reference/开发执行规范.md` §0.6 标准编排流程。

> **权威源声明（方案 B，防孤立）**：后端代码规范、包结构、domain 白名单、lint/测试配置的**唯一权威源**在 `docs/reference/standard/backend/`。本文件只保留 agent 秒级要看到的**红线速查**，每节末尾给 `→ standard/xxx.md` 深链。规范有出入时以 standard/ 为准。

## Commands

```bash
go mod tidy  &&  go run cmd/server/main.go
golangci-lint run ./...  &&  go vet ./...
go test ./...  &&  go build ./...
# Single: go test ./internal/admin/scheduler -run TestName -v
```

> AGENTS.md 约定：日常验证**只跑本次修改影响的包**，不跑全量。

## 红线速查（Anti-Patterns — 硬禁）

- ❌ `router.go` 里写业务逻辑
- ❌ Handler 直接访问 DB（绕过 service/repository）
- ❌ `panic` 处理错误
- ❌ handler 文件放 domain 根包（必须在 `handler/` 子包）
- ❌ 绕过 `wire.go` 对外暴露符号（外部需要就加 re-export）

→ 完整 Anti-Patterns：`docs/reference/standard/backend/package-layout.md`

## 包结构红线（统一三层）

```
internal/<domain>/
├── routes.go    # 路由注册（app/router.go 调用）
├── wire.go      # 单例初始化 + 对外 re-export
├── handler/     # Gin handler（package handler）
├── service/     # 业务逻辑（package service）
└── repository/  # 数据访问（package repository）
```

- 根包**只**放 `routes.go` + `wire.go`
- **Domain 白名单**：`admin` / `reader` / `tagmanagement` / `topicgraph`（新增 domain 必须先登记）

→ 完整结构 + 白名单表 + Anti-Patterns：`docs/reference/standard/backend/package-layout.md`

## 关键约定速查

- 路由在 `internal/app/router.go`；业务逻辑在 `internal/<domain>/service/`
- PostgreSQL + pgvector，Docker 镜像 `pgvector/pgvector:pg18-trixie`
- Handler 响应：`gin.H{"success": bool, "data"|"error"|"message": ...}`
- JSON struct tag `snake_case`；错误 `fmt.Errorf("...: %w", err)`；禁止 `panic`
- 导入顺序：stdlib → 空行 → 第三方 → 空行 → 本地
- 命名 PascalCase 导出 / lowerCamelCase 私有；包名简短
- 校验参数再碰 DB，错误早返回
- 日志复用 `internal/platform/logging`，避免裸 `log.Printf`

→ 完整代码规范：`docs/reference/standard/backend/code-style.md`

## 测试与 Lint

- 测试：标准 `testing` + `testify`，`*_test.go` 与源同包，偏好表驱动
- 集成测试：testcontainer pgvector（`testutil.SetupTestDB`，需 Docker）
- 🛑 **DSN 安全红线**：`testutil` 无默认 DSN、不读环境变量，禁止重引入「默认数据库连接」（历史事故曾清空业务数据）
- Lint：`golangci-lint run ./...`（配置 `backend-go/.golangci.yml`）

→ `docs/reference/standard/backend/testing.md`（含 DSN 红线全文）、`docs/reference/standard/backend/lint.md`
