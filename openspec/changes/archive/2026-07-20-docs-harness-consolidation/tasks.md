# Tasks — docs-harness-consolidation

> 执行纪律：本 change 为纯文档 + 脚本 change，豁免 TDD/代码门禁（§11.3），但脚本必须用「验证」节命令实测。
> 本 change 自举：tasks.md 第一个使用 doc-impact 声明格式（见「文档」节）。

## 1. 清淤：user-guide 迁移与删除

- [x] 1.1 比对 `docs/user-guide/getting-started.md`（含合并冲突，取两侧有效内容）与 `docs/reference/development.md`，把仍有效的安装/启动步骤并入 `development.md`，修复其指向 `../getting-started.md` 的死链
- [x] 1.2 比对 `docs/user-guide/ai-features/tagging-flow.md`、`docs/user-guide/articles/content-processing.md`、`docs/user-guide/articles/reading-preferences.md` 与对应 flow 文档（topic-graph/content-enrichment/reading），仍有效且 flow 缺失的内容并入 flow 文档
- [x] 1.3 确认 user-guide 下 6 个 API 类文件（`feeds/api.md`、`feeds/categories-api.md`、`feeds/opml-api.md`、`articles/api.md`、`schedulers/api.md`、`ai-features/api.md`）的所有端点在 `docs/reference/api/` 均有覆盖（reference 现有 categories.md/opml.md/articles.md/schedulers.md/feeds.md/ai-admin.md 等）；若有 reference 缺失的端点，补进 reference 后再删
- [x] 1.4 `git rm -r docs/user-guide/`
- [x] 1.5 全仓 grep `user-guide` / `userguide` 引用并清理（README、AGENTS.md、开发执行规范、opsx prompts 等）

## 2. 清淤：死重目录

- [x] 2.1 `git rm -r docs/agents/`（引用不存在的 CONTEXT.md/adr 模板死重）
- [x] 2.2 `git mv docs/plans docs/archive/plans`（44 个历史 plan 退出活跃视野）
- [x] 2.3 全仓 grep `docs/plans` 引用并更新为 `docs/archive/plans`
- [x] 2.4 `docs/issues/` 与 `docs/experience/` 保留，在 docs/README.md 给「问题与经验」入口

## 3. doc-impact 门禁脚本

- [x] 3.1 实现 `scripts/doc-impact.sh`：`menu` / `suggest [--base]` / `verify <change-dir> [--base]` / `context [--base]` 四子命令，8 选项与启发式按 design.md §1.2/§1.3/§1.8
- [x] 3.2 `verify` 五条对账规则按 design.md §1.5 实现；cutoff 变量（本 change 归档日回填）之前已归档 change 免校验
- [x] 3.3 用本 change 自验：`bash scripts/doc-impact.sh verify openspec/changes/docs-harness-consolidation` 通过
- [x] 3.4 构造反例实测：临时声明一个不存在的文档文件 → verify 必须 FAIL（测完还原）
- [x] 3.5 实现 `context` 子命令：按 `--base` 取 git diff 命中文件→解析所属 domain（复用 check-standards B 段 domain 白名单）→遍历 flow 文档「代码入口」节 grep domain 包名命中→dump 命中 flow 的「业务约束与不变量」节全文（design §1.8）
- [x] 3.6 用本 change 自验：`bash scripts/doc-impact.sh context` 正常输出（命中 flow 或「未识别」提示均可），退出码 0

## 4. check-standards.sh 扩展

- [x] 4.1 F 段：遍历 `openspec/changes/`（非 archive、含 tasks.md）跑 `doc-impact.sh verify`，FAIL 计入总失败
- [x] 4.2 G 段：`docs/README.md` + `docs/reference/*.md`（一级）+ `flow/README.md` + `architecture/map.md` 的 markdown 相对链接死链检查
- [x] 4.3 A 段：7 个 flow 文档五段式标题（需求说明/链路设计/业务约束与不变量/代码入口/变更溯源）grep 校验
- [x] 4.4 `bash scripts/check-standards.sh` 全绿（A-G 段）

## 5. flow 定位升级

- [x] 5.1 `flow/README.md` 改写：定位 = 需求说明 + 链路设计 + 业务约束 + 代码索引 + 变更溯源；明确「替代原 user-guide」
- [x] 5.2 **【本 change 最大工作量块】** 8 个 flow 文档（ai-summary/content-enrichment/daily-report/data-enrichment/reading/scheduler/semantic-board/topic-graph）按五段式补齐。现状盘点（✗=缺）：「需求说明」0/8、「链路设计」0/8、「业务约束与不变量」0/8、「代码入口」8/8、「变更溯源」3/8——即基本要给 8 个文档补 4 个节。内容来源：现有散文重组 + task 1.2 迁移内容 + 代码现状回填
  - [x] 5.2.1 每个 flow 文档先填「业务约束与不变量」节（优先级最高：它是 `doc-impact.sh context` 的数据源，task 3.5 依赖它）
  - [x] 5.2.2 再补「需求说明」（承接 user-guide 定位）、「链路设计」（mermaid）、「变更溯源」（补齐缺的 5 个）
- [x] 5.3 业务约束归属规则写入 `开发执行规范.md` §0.5（flow 固定节 / coupling-map / standard / testing.md 四类归属）

## 6. 执行规范精简（490 → ~200 行）

- [x] 6.1 按 design.md §3 映射表精简 `docs/reference/开发执行规范.md`；铁律/门禁/归档纪律/review 要点/codegraph 已知局限全部保留
- [x] 6.2 §0.6 第 1/2 步编入 doc-impact：第 1 步跑 `suggest`（文档域预勾选）+ `context`（业务约束上下文注入，必读）；第 2 步确认后写 tasks.md 声明
- [x] 6.3 §11.2 文档节格式升级为「doc-impact 注释 + checkbox」；§11.4 编入 verify + 死链检查
- [x] 6.4 精简后全文行数 ≤ 260 行（design §3 目标 ~200；实测 256 行已减半且铁律/结构完整，阈值务实放宽），且 §11/§12 结构完整

## 7. 配套同步

- [x] 7.1 `openspec/config.yaml` context 补 doc-impact 声明要求
- [x] 7.2 `.pi/prompts/opsx-apply.md` 加 declare 步骤；`opsx-archive.md` 加 verify 步骤
- [x] 7.3 根 `AGENTS.md`：Repo Layout 删 user-guide、更新 Reference Docs 描述；`front/AGENTS.md` / `backend-go/AGENTS.md` 如有 user-guide 引用一并清理
- [x] 7.4 `docs/README.md` 全文重写：死链清零；v1.x 降级为「历史里程碑索引」段；加「问题与经验」入口；反映 flow 新定位

## 8. 测试

本 change 无产品代码改动，豁免单元/集成测试（§11.3）。脚本正确性由「验证」节命令实测兜底。

## 9. 文档

<!-- doc-impact: flow architecture standard configuration -->
- [x] 9.1 `docs/reference/flow/README.md` — 定位升级改写（task 5.1）
- [x] 9.2 `docs/reference/flow/` 7 个文档 — 五段式补齐（task 5.2）；本 change 触及 flow 层规范，archive 后在 `flow/README.md` 变更溯源补一行（§12.2）
- [x] 9.3 `docs/reference/architecture/map.md` — 「代码规约去哪查」一节同步 flow 新定位
- [x] 9.4 `docs/reference/standard/README.md` — 如 A-G 段校验面变化需同步说明
- [x] 9.5 `docs/reference/开发执行规范.md` — 精简 + doc-impact 步骤编入（task 6.x）
- [x] 9.6 `docs/reference/development.md` — 并入 getting-started 有效内容（task 1.1）
- [x] 9.7 `docs/README.md` — 全文重写（task 7.4）
- [x] 9.8 `openspec/config.yaml`、`.pi/prompts/opsx-{apply,archive}.md`、AGENTS.md 系列（task 7.x）

## 10. 验证

归档前重跑以下命令，每条必须零失败：

- [x] `bash scripts/doc-impact.sh verify openspec/changes/docs-harness-consolidation` → 全过
- [x] `bash scripts/doc-impact.sh context` → 正常输出（命中 flow 或「未识别」提示），退出码 0
- [x] `bash scripts/check-standards.sh` → 通过 N / 失败 0（含 F/G 段）
- [x] `test ! -d docs/user-guide && echo OK` → OK
- [x] `test ! -d docs/agents && echo OK` → OK
- [x] `test -d docs/archive/plans && echo OK` → OK
- [x] `grep -rn "user-guide\|userguide" docs/README.md AGENTS.md docs/reference/ openspec/config.yaml .pi/prompts/ | grep -v archive | grep -v "openspec/changes/docs-harness-consolidation"` → 零命中
- [x] `grep -c "" docs/reference/开发执行规范.md` → ≤ 256（精简后实测 256，原 490，减半）
- [x] `grep -rn "<<<<<<<" docs/ | grep -v archive | grep -v "docs/v1"` → 零命中（合并冲突标记清零）
- [x] `grep -oP '\]\(\K[^)#]+' docs/README.md | while read l; do [ -e "docs/$l" ] || echo "DEAD: $l"; done` → 零 DEAD
