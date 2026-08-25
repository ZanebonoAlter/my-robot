# Proposal: add-change-scope

## Why

「测试只跑本次修改影响的包」目前是写在 AGENTS.md 里靠 agent 自觉的规则，没有机械化判定：quality-gate.ts 明确注释「不跑 go test：影响包无法自动判定（codegraph affected 实测误报）」，归档门禁也靠人记路径。调研 DeepSeek Harness（deepseek-ai/deepseek-harness）后确认其 `scripts/change-scope.ts`（git diff merge-base → 路径集合 JSON → agent 据此选最小测试集）是成熟的低成本解法，值得移植为本仓库的路径→验证命令映射脚本。

## What Changes

- 新增 `scripts/change-scope.sh`：输出本次改动（committed/staged/unstaged/untracked）的文件集合 + 按**路径→验证命令映射表**推导影响面，打印「建议执行的最小验证命令清单」。子命令风格对齐现有 `doc-impact.sh`（WSL bash 可跑，`set -u`）。
- 映射规则（首版）：
  - `backend-go/internal/<domain>/**` → `go test ./internal/<domain>` + 该 domain 的 lint
  - `backend-go/internal/platform/<pkg>/**` → `go test ./internal/platform/<pkg>`
  - `backend-go/internal/{app,models}/**` 或 `cmd/**` → 骨架/共享变更，提示升级为 `go build ./... + go vet ./...`（不自动全量 test）
  - `backend-go/go.mod` / `go.sum` → 依赖变更，提示全量 `go test ./...`
  - `front/**`（非 .md）→ 提示 `pnpm lint`（WSL 安全）；typecheck/test:unit/build 标注「需 cmd.exe」
  - `docs/**` / `*.md` → 无代码测试，提示文档一致性检查
- 升级 `.pi/extensions/quality-gate.ts`：turn_end 门禁从「后端只 lint」升级为「调用 change-scope.sh 判定影响包 → 自动跑对应 `go test`（限内部 domain/platform 包，超时保护）」，解决注释中「影响包无法自动判定」的问题。前端维持只 lint（cmd.exe 限制不变）。
- 文档接入：`docs/reference/开发执行规范.md` §「测试只跑影响包」段落改为指向脚本命令；AGENTS.md 对应行更新为「用 `bash scripts/change-scope.sh` 判定」。

## Capabilities

### New Capabilities

- `change-scope-gate`: 改动范围→最小验证命令的机械判定门禁（脚本 + quality-gate 集成），与 `doc-impact-gate`（文档域命中）对仗：doc-impact 管「文档要更新哪些」，change-scope 管「测试要跑哪些」。

### Modified Capabilities

（无——`agent-quota-gate` 的 turn_end 行为增强在实现层，spec 层要求不变；如 review 时认定需要，再补 delta spec。）

## Impact

- 新增：`scripts/change-scope.sh`（~150 行 bash，含映射表与输出格式）。
- 修改：`.pi/extensions/quality-gate.ts`（go test 调用路径，~30 行增量）。
- 修改：`docs/reference/开发执行规范.md`（§0.6/§4.1 引用）、`AGENTS.md`（Build & Verify 段一行）。
- 风险与约束：
  - 映射表是**保守白名单**：命中不了的路径回退到「提示手动判定」，绝不猜测（吸取 codegraph affected 误报教训）。
  - go test 需 Docker DB 的包（integration 类）在门禁里标注跳过、只提示，不阻塞 turn_end（避免 flaky 阻塞开发节奏）。
  - 前端编译类命令维持「agent 手动跑 cmd.exe」现状，本 change 不试图绕过跨平台限制。

## 参考

- **调研记录（数据源）**：[`research.md`](research.md)（同 change 目录）——dsh 全量调研快照（仓库结构/工程流程/关键代码摘录），含本 change 的采纳决策表与不抄清单。落点规则：change 强相关调研随 change 走，无 change 归属的调研进 `docs/research/`（见 amend-dev-workflow）。
- 现有基础：`scripts/doc-impact.sh` 的 `changed_files()` 已实现同类文件集合收集（含 `core.checkStat=minimal` DrvFS 优化、中文路径 quotepath 处理），直接复用经验。
