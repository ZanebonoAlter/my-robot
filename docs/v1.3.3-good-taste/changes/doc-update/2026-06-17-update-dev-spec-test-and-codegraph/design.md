# Design — update-dev-spec-test-and-codegraph

## 决策 1：§6 保留为独立章节，不合并进 §4

§4 是「后端执行规范」，以**命令门禁**为组织维度（`golangci-lint`/`go vet`/`go test`/`go build`）；§6 是「集成测试规范」，以**测试分层纪律**为组织维度（前置条件 / 运行方式 / 编写规范）。

合并的代价：
- §4 已经 4 个子节（4.0 前置/4.1 门禁/4.2 测试规范/4.3 lint 配置/4.4 代码规范），再塞入集成测试细节会冲淡门禁主题
- §6 当前被 §11.1（归档门禁前置三条件）引用为「§6 集成」门禁——若合并进 §4，需要同步改 §11.1 的引用，扩大改动面

结论：§6 保留为独立章节，重写内容但保留章节编号，§11.1 引用不变。

## 决策 2：codegraph 已知局限写进 §7.1，强制 grep 二次校验

依据：`docs/v1.3.3-good-taste/changes/dead-code-clean/2026-06-11-code-cleanup-dead/design.md` 的实战记录——codegraph 无法追踪 Gin 的 `group.GET/POST(..., fn)` 函数引用注册模式，会误报 Gin handler 为无调用者（约 26 个误报）。

处理方式：
- §7.1 主推 codegraph `impact`/`affected` 做 prologue 影响分析（替换 gitnexus）
- 显式写出该局限，要求删除 Gin handler 前用 grep 二次校验（避免误删）
- code-review-graph 作为 PostToolUse hook 自动触发（`.claude/settings.json`/`.codex/hooks.json` 已配置），仅附注说明，**不写进强制门禁**——agent 主动调用的是 codegraph，hook 是辅助

## 决策 3：§4.2 SQLite 引用保留，措辞修正

`setupFeedsTestDB`（`internal/reader/service/feed_service_test.go:18`）仍存在且仍用内存 SQLite（`glebarez/sqlite` + `mode=memory`）。`testing.md` 确认：部分 package（`admin/handler`、`platform/airouter`、`reader/*`、`topicgraph/handler`）仍使用内存 SQLite，仅用于无 pgvector 依赖的 CRUD 测试。

处理方式：§4.2 保留 `setupFeedsTestDB` 引用，但措辞从「需要数据库的测试：使用内存 SQLite」改为「集成测试用 testcontainer；无 pgvector 依赖的轻量 CRUD 测试用内存 SQLite」，与 `testing.md` 口径对齐。

## 决策 4：纯文档 change，走 §11.3 路径

无代码/配置/依赖变更，豁免 `go test`/`pnpm test`。验证手段为 grep 一致性校验（Python 残留零命中、gitnexus 零命中、codegraph/testcontainer 命中），符合 §11.3 纯文档 change 测试门禁要求。
