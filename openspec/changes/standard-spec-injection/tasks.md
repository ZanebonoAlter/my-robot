# Tasks — standard-spec-injection

> 纯文档 + 脚本 change，豁免 TDD/代码门禁（§11.3），脚本用「验证」节实测。
> doc-impact 声明：本节 apply 第 2 步跑 `doc-impact.sh suggest` 后，由 agent 在文档节首行补机器可读的域声明注释（格式见 `doc-impact.sh menu` 的 8 选项；propose 阶段不写）。

## 1. ai-logging.md spec 化（试点）

- [ ] 1.1 把 `ai-logging.md` 的 R1-R4 红线重构成 `## Requirements` → 4 个 `### Requirement`（级别 MUST）+ 各自 `#### Scenario: WHEN/THEN`
- [ ] 1.2 文档头加 HTML 注释 `doc-impact-applies: internal/platform/airouter/, backend-go/internal/dataenrichment/service/`
- [ ] 1.3 Anti-Patterns 并入对应 Requirement 的 `AND NOT` 句式；已接入功能清单保留作附注
- [ ] 1.4 自验：`grep '^## Requirements' docs/reference/standard/backend/ai-logging.md` 命中；每个 Requirement 有级别 + Scenario

## 2. doc-impact.sh context 扩展（how 注入）

- [ ] 2.1 `cmd_context` 扩展：除 flow 业务约束（what），新增遍历 `docs/reference/standard/**/*.md`，按文档头 `doc-impact-applies` 命中改动代码路径 → dump `## Requirements` 节
- [ ] 2.2 MUST 条目（`**级别**: MUST`）注入时前缀 🛑；SHOULD 前缀 `[SHOULD]`
- [ ] 2.3 输出分两段：业务规范（what）/ 执行规范（how），各自标头
- [ ] 2.4 无 standard 命中时 how 段输出「未识别到相关执行规范」
- [ ] 2.5 `applies` 路径匹配支持 glob（如 `internal/platform/airouter/` 前缀匹配）

## 3. 验证

- [ ] 3.1 构造：临时改 `internal/platform/airouter/` 下代码 → `doc-impact.sh context` how 段含 ai-logging 的 Requirements（含 🛑 MUST）
- [ ] 3.2 构造：改非 AI 代码 → how 段输出「未识别到相关执行规范」
- [ ] 3.3 `check-standards.sh` 通过；`doc-impact.sh verify openspec/changes/standard-spec-injection` 通过

## 4. 文档

- [ ] 4.1 `docs/reference/standard/backend/ai-logging.md` — spec 化（task 1.x）
- [ ] 4.2 `docs/reference/开发执行规范.md` §0.5 — 业务约束归属表加「执行规范」行：代码写法红线 → standard Requirements（spec 化）；§0.6 第 1 步 context 说明补「双源注入（业务规范 what + 执行规范 how）」
- [ ] 4.3 `docs/reference/flow/README.md` — 五段式说明处补一句 context 现含 how 维度（可选）

## 5. 验证节

归档前重跑，零失败：

- [ ] `bash scripts/doc-impact.sh context`（改 airouter 代码后）→ how 段含 ai-logging Requirements
- [ ] `bash scripts/check-standards.sh` → 通过 N / 失败 0
- [ ] `bash scripts/doc-impact.sh verify openspec/changes/standard-spec-injection` → 通过
