## Why

`docs/reference/开发执行规范.md`（OpenSpec 通用执行规范）作为所有 Proposal 的默认施工纪律，其测试与架构体检章节已与代码现状严重脱节，会误导 apply 阶段的执行。以代码为权威来源（`docs/reference/testing.md`、`backend-go/internal/platform/testutil/testutil.go`、`.codegraph/`、`.mcp.json`）核对，发现三类矛盾：

1. **集成测试栈已迁移，规范未跟进**：§0 列出两套 Python 测试套件（`tests/workflow/` pytest 集成测试、`tests/firecrawl/` Firecrawl 检查），但仓库 `tests/` 顶层目录已删除，集成测试早已迁移到 Go 的 testcontainer 方案（`backend-go/internal/platform/testutil/testutil.go`，14+ 个 `_test.go` 在用，带 `-short` 跳过）。§6 整节仍描述已废弃的 Python pytest 方案（`.venv`、`requirements.txt`、`utils/database.py` 等已不存在的文件）。

2. **架构体检工具已替换，规范未更新**：§7.1 强制每个子任务执行 `gitnexus_impact({target, direction})` 与 `gitnexus_detect_changes({scope})`，但 gitnexus 已被 codegraph 取代（MCP 注册于 `.mcp.json`，产物在 `.codegraph/`，工具名为 `impact`/`affected`/`callers`/`callees`/`query`，非 `gitnexus_*` 命名风格）。

3. **单元测试措辞与权威文档矛盾**：§4.2 写「需要数据库的测试：使用内存 SQLite」，但 `testing.md`（权威测试文档）明确集成测试以 testcontainer 为主、内存 SQLite 仅用于无 pgvector 依赖的轻量 CRUD 测试。规范与 `testing.md` 口径不一致，apply 阶段无所适从。

> 注：`testing.md` 本身已是正确的（testcontainer 主导 + SQLite 辅助，无 Python 残留），本次不改它。问题集中在 `开发执行规范.md` 一个文件。

## What Changes

- 更新 `开发执行规范.md` §0 适用范围：删除「额外测试套件」表中的 Python 行（`tests/workflow/`、`tests/firecrawl/`），改为说明 Go 测试分层（testcontainer 集成 + 内存 SQLite 单元）并指向 `testing.md` 为权威
- 更新 `开发执行规范.md` §4.2 测试规范：补充 testcontainer 为主路径（`testutil.SetupTestDB(t)`），内存 SQLite 降为「无 pgvector 依赖的轻量 CRUD 测试」辅助路径，保留 `setupFeedsTestDB` 引用（该模式仍有效），指向 `testing.md`
- 重写 `开发执行规范.md` §6 集成测试规范：从「Python pytest」改为「Go 集成测试规范」——前置（Docker 运行）、`-short` 跳过约定、集成 vs 单元分层纪律、门禁命令，不重复 `testing.md` 已有内容
- 更新 `开发执行规范.md` §7.1 架构体检：gitnexus → codegraph（`impact <符号>` 做影响分析、`affected` 做变更检测），附 codegraph 无法追踪 Gin handler 注册的已知局限与 grep 二次校验要求，附注 code-review-graph 作为 PostToolUse hook 自动触发（非门禁）

## Capabilities

### Modified Capabilities

- `development-docs`：修正 `开发执行规范.md` 的 §0/§4.2/§6/§7.1，使其与 `testing.md`、`testutil.go`、`.codegraph/` 代码现状一致

## Impact

- **受影响的文件**（1 个）：`docs/reference/开发执行规范.md`
- **不改的文件**：`docs/reference/testing.md`（已正确）、`docs/reference/development.md`（本次无关）
- **无代码变更**：本次只更新文档，不涉及任何代码/配置/依赖修改
- **属 §11.3 纯文档 change**：豁免 `go test`/`pnpm test`，以 grep 一致性校验为验证手段
- **范围边界**：与 `update-reference-docs` change 不重叠——后者 proposal 明确把 `testing.md` 列入「已核查无过时引用、本次不改」清单，且其 `开发执行规范.md` 改动仅限「测试规范引用路径、后端分层、cmd/migrate-db」三处文本修正，不覆盖 pytest→testcontainer、gitnexus→codegraph 这两类结构性更新
