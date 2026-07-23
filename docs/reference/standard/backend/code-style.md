# 后端代码规范（Code Style）

> **权威源**：本文件是后端代码规范的唯一权威。`backend-go/AGENTS.md` 的 Backend-Specific Conventions 红线要点深链指向本文件。
> 包结构见 [package-layout.md](./package-layout.md)。

## 格式与静态分析

- 使用 `gofmt` 格式化
- 静态分析 `golangci-lint run ./...`（配置与启用 linter见 [lint.md](./lint.md)）
- 不要新增 linter / formatter / 其他工具，除非明确要求

## 导入顺序

```
标准库

第三方库

项目内部包
```

## 命名

- 导出符号 `PascalCase`，私有符号 `lowerCamelCase`
- 包名简短

## 错误处理

- 用 `fmt.Errorf("context: %w", err)` 包装
- **禁止 `panic` 处理错误**
- 校验参数再碰 DB；错误早返回（early return）

## 响应与序列化

- Handler 响应格式：`gin.H{"success": bool, "data"|"error"|"message": ...}`
- JSON struct tag 使用 `snake_case`

## 日志

- 优先复用 `internal/platform/logging`
- 避免继续用裸 `log.Printf` + 文本前缀人工区分级别

## 业务归属

- 业务逻辑按 domain 组织在 `internal/<domain>/service/`（domain 白名单见 package-layout.md）
- HTTP 路由注册在 `internal/app/router.go`
- Handler 不写复杂业务，不直接访问 DB

## GORM model tag 与迁移

**显式迁移管的表，model tag 不写 `not null`——让显式迁移（`postgres_migrations.go`）唯一管 NOT NULL 约束，AutoMigrate 不重复施加。** 但 **`default:` tag 是例外，必须保留**（见下「default tag 不可删」）。

**为什么禁 `not null`**：model tag 写 `not null` 会让 AutoMigrate 启动时尝试 `ADD COLUMN ... NOT NULL`，与显式迁移的"ADD NULL → 回填 → SET NOT NULL"三步竞争；在有历史数据的库上 AutoMigrate 先失败（`column ... contains null values`），污染启动日志（`ai-call-logging-schema` 的事故教训）。

**⚠️ default tag 不可删（2026-07-23 事故教训）**：`default:xxx` 不仅是 DB 约束，还**控制 GORM 的零值省略行为**——有 `default:` tag 的字段，GORM 在 Go 零值（`""`/`0`/`false`）时**省略该列**，让 DB DEFAULT 生效；没有 `default:` tag 时，GORM **显式插入零值**，覆盖 DB DEFAULT。曾一次性删除 `topic_tags.status` 的 `default:active` tag，导致新建 tag 被写成 `status=""`（而非 DB DEFAULT 的 'active'），使 `getBoardArticles`（按 `status='active'` 过滤）查不到相关 tag 和文章。**结论：标量字段（string/int/bool）的 `default:` tag 是功能必需，不可移除**；NOT NULL 由显式迁移兜底即可（见 `postgres_migrations.go` 迁移 + `constraints_test.go` 约束断言）。

**正确做法**：
- model tag：`gorm:"size:20;default:active;index"`（类型 + **default 保留** + 索引 + json），不写 `not null`
- DB 约束（NOT NULL/CHECK）：写在显式迁移；DEFAULT 由 `default:` tag + 迁移双保险
- "必填"语义：靠代码入口校验（如 `Router.Chat` 强制 `Operation != ""`），不靠 model tag 反射

**JSONB 列空值**：非指针 `string` 字段写 `gorm:"type:jsonb"` 列时，零值 `""` 不是合法 JSON——入库前用 `db.Omit("col").Create()` 跳过空值列（DB 置 NULL），或改 `*string`/`datatypes.JSON`。`serializer:json` 的集合字段（如 `[]string`/`map`）必须保留 `default:'[]'`/`default:'{}'`（同 default tag 零值省略原理）。详见 testing.md「JSONB 列空串陷阱」。

## Anti-Patterns（硬禁）

- ❌ `router.go` 里写业务逻辑
- ❌ Handler 直接访问 DB
- ❌ `panic` 处理错误

## 资料来源

收敛自原 `backend-go/AGENTS.md`（Backend-Specific Conventions / Anti-Patterns）与 `development.md` §后端代码风格、《开发执行规范》§4.4。
