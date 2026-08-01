# Design — docs-harness-consolidation

## 1. 选项式文档门禁（doc-impact）

### 1.1 问题定义

现状：tasks.md「文档」节靠 agent 自由回忆要更新哪些文档，无菜单、无验证。回忆必然遗漏——`docs/README.md` 积累 10+ 死链、user-guide 带合并冲突提交均为实证。

设计目标：把「回忆题」变成「选择题」。固定选项菜单 + 启发式预勾选 + 机器可读声明 + 归档前确定性对账。全程 bash 脚本，不烧 token。

### 1.2 八个固定文档域选项

| key | 覆盖路径 | 预勾选启发式（按 git diff 改动路径） |
| --- | --- | --- |
| `flow` | `docs/reference/flow/` | 改了 `backend-go/internal/{admin,reader,tagmanagement,topicgraph,dataenrichment}/service/` 或 `front/app/features/` |
| `api` | `docs/reference/api/` | 改了 `backend-go/internal/*/handler/` 或 `internal/app/router.go` |
| `database` | `docs/reference/database/` | 改了 `backend-go/internal/models/` 或 diff 含 `AutoMigrate`/`CREATE TABLE`/`ALTER TABLE` |
| `architecture` | `docs/reference/architecture/` | 改了 `backend-go/internal/app/`、新增/删除 `internal/` 包目录、`internal/platform/tracing/` |
| `standard` | `docs/reference/standard/` | 改了 `.golangci.yml` / `eslint.config.js` / 测试配置文件 |
| `configuration` | `docs/reference/configuration.md` | 改了 `config*.yaml` / `internal/platform/config/` |
| `deployment` | `docs/reference/deployment.md` | 改了 `Dockerfile*` / `docker-compose*.yml` / `init.ps1`/`init.sh` |
| `none` | — | 纯内部重构；声明时必须附理由 |

启发式只用于 `suggest` 预勾选与 `verify` 反向校验，**不构成穷举**——agent 最终对声明负责。

### 1.3 脚本接口 `scripts/doc-impact.sh`

```bash
bash scripts/doc-impact.sh menu                    # 打印 8 选项菜单（agent apply 启动时读）
bash scripts/doc-impact.sh suggest [--base <ref>]  # 按 git diff 输出预勾选菜单+命中理由
bash scripts/doc-impact.sh context  [--base <ref>] # 按 git diff 命中 domain dump 相关 flow「业务约束与不变量」节（apply 改代码前必读）
bash scripts/doc-impact.sh verify <change-dir> [--base <ref>]  # 归档前对账
```

- `--base` 默认 `HEAD` 之前的 change 起点难以自动判定，统一用「工作区 + 暂存 + 未跟踪」diff（`git diff --name-only HEAD` + `git ls-files --others`），与 `quality-gate.ts` 的路由逻辑一致。若 change 跨多次提交，`--base main` 手动指定。
- `suggest` 输出格式（示例）：

```
文档域预勾选（--base HEAD）：
  [x] flow          — 命中: backend-go/internal/reader/service/crawler.go
  [x] api           — 命中: backend-go/internal/reader/handler/firecrawl_handler.go
  [ ] database
  [ ] architecture
  [ ] standard
  [x] configuration — 命中: backend-go/internal/platform/config/config.go
  [ ] deployment
请确认后写入 <change>/tasks.md「文档」节：
<!-- doc-impact: flow api configuration -->
```

### 1.4 tasks.md 声明格式

文档节第一行必须是机器可读注释，后跟人读 checkbox：

```markdown
## N. 文档

<!-- doc-impact: flow api configuration -->
- [ ] docs/reference/flow/content-enrichment.md — 补降级链 + 变更溯源（archive 后补链接）
- [ ] docs/reference/api/articles.md — 新增 GET /api/xxx
- [ ] docs/reference/configuration.md — firecrawl 改可选
```

无文档影响：`<!-- doc-impact: none(纯内部重构，无对外行为变化) -->`

### 1.5 verify 对账规则（全部 FAIL 即归档门禁失败）

verify 解析两层信息，独立报错：

- **(a) 域注释** `<!-- doc-impact: -->` 中的文档域 → 与 git diff 反向启发式对账（规则 3、4）；
- **(b) 文档节 checkbox** 列出的文件路径 → 校验存在性 + 是否在 git diff 中（规则 2、5）。

域注释决定「该不该声明这个域」，checkbox 文件决定「声明的具体文件改没改」。两层分开，避免「声明了域但漏列文件」或「列了文件但没声明域」互相掩盖。

| # | 规则 | 失败消息 |
| --- | --- | --- |
| 1 | tasks.md 缺 `doc-impact` 注释 | `未声明 doc-impact（apply 启动时跑 suggest 补声明）` |
| 2 | 声明的文档文件不在 changed 集合 | `声明了未更新: <path>` |
| 3 | 反向启发式命中未声明域（如 handler 改动 + 无 `api`） | `疑似遗漏: 改了 handler 未声明 api` |
| 4 | 声明 `none` 但任何启发式命中 | `声明 none 但命中 <域>` |
| 5 | 声明文件路径不存在（typo 防呆） | `声明的文档不存在: <path>` |

历史存量：F 段设 cutoff（本 change 归档日），之前已归档的 change 免校验，同 E 段策略。

### 1.6 挂载点

| 时机 | 挂载 | 动作 |
| --- | --- | --- |
| apply 启动（§0.6 第 1 步后） | `.pi/prompts/opsx-apply.md` + `开发执行规范.md` §0.6 | 跑 `suggest`（确认后写 tasks.md）+ 跑 `context`（业务约束注入，必读） |
| 归档前（§11.4） | `check-standards.sh` F 段 | 对每个 active change 跑 `verify` |
| 归档前（§11.4） | `check-standards.sh` G 段 | markdown 相对链接死链检查（docs/README.md + docs/reference/ 一级 + flow/） |
| turn_end 增量层 | **不挂**（保持增量层安静，归档层兜底） | — |

### 1.7 G 段死链检查范围

只检查「导航层」文档的相对链接：`docs/README.md`、`docs/reference/*.md`（一级）、`docs/reference/flow/README.md`、`docs/reference/architecture/map.md`。正文层文档不查（历史存量链接多，一次性爆 FAIL）。实现：grep 出 `](xxx.md)` / `](dir/)` 相对链接，拼路径 `[ -e ]` 校验。

### 1.8 业务约束上下文获取（context 子命令）

目的：apply 改代码前把相关业务不变量注入 agent 上下文，避免违反状态机/幂等/去重/限额等红线。**非归档断言，是前置必读**——不进 check-standards FAIL，但 §0.6 编排强制 apply 第 1 步跑、agent 必读。

机制：`doc-impact.sh context` 按 `--base` 取 git diff 命中文件，解析其所属 domain（复用 check-standards.sh B 段 domain 白名单逻辑），遍历 `docs/reference/flow/*.md`，凡某 flow 文档「代码入口」节命中该 domain（grep domain 包名），dump 该 flow「业务约束与不变量」节全文。

- **不硬编码 domain→flow 映射表**，靠 flow 文档自身「代码入口」节关联——故 task 5.2 五段式补齐（尤其「代码入口」「业务约束」两节）是 context 生效的前提。
- 多 flow 命中则依次 dump，每段标 flow 名作分隔。
- 无命中输出：`未识别到相关业务约束 flow；如改动涉及业务逻辑，请主动查阅 docs/reference/flow/`。
- 挂载点见 §1.6 表「apply 启动」行：suggest 之后跑 context。

## 2. flow 五段式模板

每个 `docs/reference/flow/<功能>.md` 固定结构（check-standards.sh A 段 grep 校验五个标题存在）：

1. `## 需求说明` — 功能给用户解决什么问题（承接 user-guide 定位）
2. `## 链路设计` — mermaid + 状态流转
3. `## 业务约束与不变量` — 状态机/幂等/去重/限额等业务红线（同时是 `doc-impact.sh context` 的数据源，apply 改代码前注入，见 §1.8）
4. `## 代码入口` — 后端 handler/service + 前端 feature 入口
5. `## 变更溯源` — archive 链接表（§12.2 已有）

现有 7 个 flow 文档（reading/content-enrichment/ai-summary/daily-report/topic-graph/semantic-board/scheduler + data-enrichment）按模板补齐缺的节；已有的「资料来源」等尾部散文并入对应节或删除。

## 3. 执行规范精简映射（490 → ~200 行）

| 章节 | 处理 |
| --- | --- |
| §0 适用范围 | 保留两表，删散文 |
| §0.5 文档归属 | 保留表格，加 `experience/issues` 行 + 业务约束归属规则 |
| §0.6 编排六步 | 保留六步表 + review 要点清单（ocr 教训不砍）；第 2 步加 doc-impact declare；「何时偏离」压成 3 条 |
| §1 任务拆解 | 3 行 |
| §2 TDD | 压成：铁律 1 句 + 分级表 + 红旗 1 句 + 「完整工作流加载 tdd 技能」 |
| §3 原型 | 5 行（触发条件保留） |
| §4/§5 前后端 | 前置检查合并为一个 checklist；门禁命令表保留；「权威定义见 standard/」重复段每处压成 1 行指针 |
| §6 集成测试 | 保留命令 + DSN 红线；删与 testing.md 重复 |
| §7 架构体检 | 保留 codegraph 检查 + 两个已知局限（实战教训），压 prose |
| §8 变更控制 | 保留程度判据表，删散文 |
| §9 产出物清单 | 压成一张表 |
| §10 数据兼容 | 保留红线条目，压 prose |
| §11 归档门禁 | 保留结构；§11.4 加 doc-impact verify + 死链检查 |
| §12 文档流转 | 保留两段式表 + 溯源格式；§12.4 压成 3 行 |

## 4. 清淤决策

| 对象 | 处置 | 理由 |
| --- | --- | --- |
| `docs/user-guide/` | 整目录删除（先迁移有效内容） | API 拷贝已漂移；getting-started/tagging-flow 带合并冲突；定位由 flow 五段式承接 |
| `docs/agents/` | 删除 | 引用不存在的 CONTEXT.md/adr，模板死重 |
| `docs/plans/`（44 文件） | `git mv` → `docs/archive/plans/` | openspec 前的历史工作文档，保留历史但退出活跃视野 |
| `docs/v1.x/`（7 目录） | 物理保留，README 降级为历史索引 | 历史快照，零风险不动；新增 change 不再归入 |
| `docs/issues/` + `docs/experience/` | 保留，README 给「问题与经验」入口 | 「出现问题有记录」的落点 |

## 5. 本 change 自举（dogfooding）

本 change 的 tasks.md 文档节第一个使用 `<!-- doc-impact: -->` 声明；`doc-impact.sh` 完成后立即用 verify 自验。
