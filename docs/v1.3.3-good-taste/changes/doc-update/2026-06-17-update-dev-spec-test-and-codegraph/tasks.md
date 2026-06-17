# Tasks — update-dev-spec-test-and-codegraph

## 1. §0 适用范围：删除 Python 测试套件表

- [x] 1.1 删除「额外测试套件」表（`tests/workflow/` pytest + `tests/firecrawl/` Python 两行），改为「测试分层」说明：Go 后端集成测试用 testcontainer（需 Docker）、单元测试用内存 SQLite，详见 `testing.md`
- [x] 1.2 确认 apply 启动说明段落（「根据变更所涉及的项目栈，对应执行相应章节」）措辞与新表一致

## 2. §4.2 测试规范：补充 testcontainer 主路径

- [x] 2.1 将「需要数据库的测试：使用内存 SQLite」改为分层描述：集成测试用 `testutil.SetupTestDB(t)`（testcontainer pgvector，需 Docker）；无 pgvector 依赖的轻量 CRUD 测试用内存 SQLite
- [x] 2.2 保留 `setupFeedsTestDB`（`internal/reader/service/feed_service_test.go`）引用作为 SQLite 模式示例
- [x] 2.3 指向 `testing.md` 为测试细节权威文档

## 3. §6 集成测试规范：从 Python pytest 重写为 Go testcontainer

- [x] 3.1 删除 §6.0 Python 前置条件（`.venv`/`requirements.txt`/Go 后端运行于 5000）
- [x] 3.2 重写 §6.0 前置条件：Docker Desktop/daemon 运行（testcontainer 需启动 pgvector 容器）
- [x] 3.3 重写 §6.1 运行方式：`go test ./...`（含集成测试，需 Docker）、`go test -short ./...`（仅单元，跳过集成）
- [x] 3.4 重写 §6.2 编写规范：集成测试入口 `testutil.SetupTestDB(t)`、`-short` 跳过约定、文件命名（`*_test.go` 集成 / `*_unit_test.go` 单元），指向 `testing.md` 不重复内容

## 4. §7.1 架构体检：gitnexus → codegraph

- [x] 4.1 将 `gitnexus_impact({target, direction})` 替换为 `codegraph impact <符号>`（影响分析）
- [x] 4.2 将 `gitnexus_detect_changes({scope})` 替换为 `codegraph affected [files...]`（变更检测）
- [x] 4.3 补充 codegraph 已知局限：无法追踪 Gin handler 注册（`group.GET/POST`），删除前用 grep 二次校验
- [x] 4.4 附注 code-review-graph 作为 PostToolUse hook 自动触发（非门禁）

## 5. 测试

本 change 为纯文档更新（§11.3 路径），豁免 `go test`/`pnpm test`。一致性校验见 §7 验证节。

## 6. 文档

- [x] 6.1 `docs/reference/开发执行规范.md` — §0/§4.2/§6/§7.1 已更新，与 `testing.md`、`testutil.go`、`.codegraph/` 代码现状一致
- [x] 6.2 确认 `docs/reference/testing.md` 未被修改（本 change 不动该文件）

## 7. 验证

每条为「可执行命令 + 期望结果」，归档前重跑确认零失败。

- [x] 7.1 `tests/workflow` / `tests/firecrawl` / `pytest` 在 `docs/reference/开发执行规范.md` 零残留 → 实测 0/0/0（PowerShell `[regex]::Matches` 计数）
- [x] 7.2 `gitnexus` 在 `docs/reference/开发执行规范.md` 零残留 → 实测 0
- [x] 7.3 `codegraph` 在 `docs/reference/开发执行规范.md` 命中 → 实测 4（替换生效）
- [x] 7.4 `testutil.SetupTestDB` / `testcontainer` 在 `docs/reference/开发执行规范.md` 命中 → 实测 4/6
- [x] 7.5 人工对照：§6 的 testcontainer 描述与 `docs/reference/testing.md` 的「后端测试分层约定」段无矛盾（容器镜像 `pgvector/pgvector:pg18-trixie`、入口 `SetupTestDB`、`-short` 跳过、文件命名、Ryuk 清理、安全约定均一致）
