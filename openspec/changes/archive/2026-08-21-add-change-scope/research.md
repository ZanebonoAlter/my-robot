# DeepSeek Harness (dsh) 调研记录

> 调研日期：2026-08-21 ｜ 方式：opencli 驱动真实 Chrome 打开 GitHub，页面内 fetch GitHub API 拉全量文件树（9060 文件）+ 逐个抓 README/AGENTS.md/notes/架构文档/skill 原文。
> 动机：`openspec/changes/add-change-scope` 的前置调研；同时回答"标准如何注入/流程纪律如何机械化"。
> ⚠ dsh 处于 developer preview 迭代极快，本文是**快照不是权威**；引用均带仓库内路径，新会话可按路径重验。

## 一、是什么

DeepSeek 官方开源的 agent harness（与 pi 同类产品），口号 "Everything is a Plugin"。Node pnpm monorepo：`apps/`(cli+web) + `packages/`(50+ 包) + `python/`(SDK) + `vendor/`(内嵌 Cordis) + `website/`(VitePress)。

## 二、架构核心（对本项目仅作背景）

- **微内核 + 全插件**：基于 Cordis（DI + 类型化事件 + 可逆副作用）。模型适配器、工具注册表、会话日志、**连 agent-loop 本身都是插件**。注册即副作用（`ctx.effect()`/`ctx.on()` 返回 disposer），卸载自动回滚。
- **Profile/Bundle 分层组装**：运行实例 = base bundle → web/headless bundle → 用户 `cordis.patch.yml` → CLI overlay 逐层叠出来。`dsh --dump-config` 打印实际插件树，任意行可被 patch 替换。
- **事件溯源会话**：SessionEvent 只追加；硬不变量 **"Model-visible ⟺ logged"**——凡进模型请求的内容必须能从会话日志重建，否则不许加（新增模型可见输入必须配 session 事件）。

## 三、工程流程精华（对本项目有价值的部分）

### 3.1 Agent Notes 决策记录体系（`.agents/notes/`）

- 路径编码双轴：`{lifecycle}/{class}/yyyy-mm-dd-topic-title.md`
  - lifecycle：`proposed/`（实现前评审）→ `implemented/`（已上线，**随代码演进保持事实同步**：代码改名/挪文件时同一 PR 内更新 note 里的路径事实）→ `rejected/`（否决留存，仅在"理由能防止一个诱人的重复犯错"时保留，否则删）→ `archived/`（冻结历史，不当现行为权威）
  - class 六类封闭集（脚本 `scripts/agent-note-tree.ts` 门禁）：`feature` / `bug-fix` / **`simplification`** / `architecture` / `process` / `testing`。判据"可观察行为变没变"（刻意不含 refactor）
- 文件格式机器校验（`verify-agent-note-format`）：头三行固定 `# Agent Note: <title>` + `Status: <proposed|implemented|rejected — 一行理由>`，Status 必须与所在 lifecycle 文件夹互验；body 以 `## Problem` 开头，proposed 有固定节（Proposal/Alternatives considered/Acceptance criteria/Risks）
- 归档冻结：三件套（英文/中文/i18n sidecar 哈希清单）永久冻结，`verify-archived-agent-notes` 校验 hash + append-only manifest
- **硬规矩：每个非平凡 PR 必须新增/更新至少一条 Agent Note**（纯机械/局部编辑豁免）
- 禁止中心化 INDEX.md（有专门 note 论证过为什么不要）

### 3.2 文档治理

- **tier 分层 + one home per fact**：root AGENTS.md 只放"常备命令，每条 1-3 行 + 链到唯一的家"；architecture.md 是有序地图；subsystems/ 每子系统一页；notes 管决策 why；cookbook 管步骤 how-to；postmortem 管事故叙事。事实放错层会被 review 打回
- **文档预算门禁**：`verify-doc-budgets` 限制各文档字数上限，"内容真放不下要显式 raise 上限"
- **生成目录 freshness 门禁**：tool-catalog/config-catalog/module-graph/persistence-catalog 从源码再生，过期即挂（`doc-sync` 汇总所有文档门禁）
- CLAUDE.md → symlink AGENTS.md（多 harness 零漂移）

### 3.3 change-scope.ts（已采纳移植 → `add-change-scope` change）

7.6KB TS 脚本，本质：git merge-base diff → 四段路径 JSON（committed/staged/unstaged/untracked），agent 据此**选最小验证集**，配 `dsh-pre-push-checks` skill："never reflexively full suite；CI 才拥有全量"。关键代码见附录。

### 3.4 simplification 一等公民

- 专门的 note class + 专门 skill `dsh-find-simplifications`（14.7KB）：强候选标准 = 无生产消费者的公开面、两处镜像同一事实、纯为测试/demo 存在的包、投机性泛化
- 两个 doctrine："tests-are-not-golden-truth"、"notes-are-not-golden-truth"（测试和 note 都只描述当时，不是不可动的圣旨）
- pre-release 立场：无外部消费者时"宁可改对地基不搞兼容垫片，改名重打包随改随更新所有引用"

### 3.5 agent skills 清单（11 个，`.agents/skills/`）

`dsh-archive-agent-notes` / `dsh-code-review` / `dsh-doc-site-sync` / `dsh-doc-standards` / `dsh-find-simplifications` / `dsh-merging-stacked-prs` / `dsh-pre-push-checks` / `dsh-prose-standard` / `dsh-translate-docs` / `dsh-trim-cot-leakage` / `record-browser-gif`。AGENTS.md 在决策点引用对应 skill（"Use dsh-xxx for Y"）。

### 3.6 其他值得记的小件

- **门禁脚本自己带测试**：`scripts/*.ts` 基本都配 `*.spec.ts`（verify 脚本也被 verify）
- 测试策略：每个非平凡的模型/用户可见行为变更，同一 PR 加 **keyless 快照**（真实可运行例子的 transcript 回放）；"CI owns exhaustive coverage and the platform matrix"——本地只跑被 diff 触及的最小集
- 防御式编程前置：`docs/defensive-patterns.md`，改生命周期/并发/子进程/teardown 前必读
- 类型纪律：strict + noImplicitAny；"Trust TypeScript at typed same-process boundaries"（同进程类型化边界不加运行时校验，只在 parser/配置/模型 JSON/文件/wire 边界校验）；跨边界 id 一律 branded type
- README 明说"推荐用 agent 探索本代码库"——整个仓库按 agent 优先设计

## 四、标准注入机制（dsh 怎么让 agent 遵守规范）

**没有魔法自动注入**，靠三件套：

1. **常备上下文预算化**：root AGENTS.md 只放每条 1-3 行 standing orders + 链接，`verify-doc-budgets` 门禁防膨胀——保证"永远在场"的部分小而密，agent 不需要"自觉去读"基本盘。
2. **按需加载的 skill**：细节规则住在 skill 里（dsh-doc-standards 等），AGENTS.md 在决策点引用（"before writing X, use dsh-prose-standard"），由 description 触发加载。
3. **机械门禁兜底**（核心 doctrine，原文）："Wire mechanically checkable invariants into an executed top-level gate and prove each changed acceptance path rejects an invalid case"——能机械查的规则全部变成 verify 脚本/CI gate，不指望自觉。lefthook hook 刻意极窄（pre-commit 只 lint+空白，pre-push 只增量 typecheck）。

对照：Syntopica 的 `doc-impact.sh context`（diff 命中注入双源规范）**注入机制上已强于 dsh**（dsh 只有静态 "read before"）；差距在①只在 apply 启动跑一次，中途漂移无人管；②standard 的 MUST 条目大多没有机械检查牙齿。

## 五、对 Syntopica 的采纳决策

| 项 | 决策 | 去处 |
| --- | --- | --- |
| change-scope 路径→最小验证集 | **已采纳**（bash 移植 + quality-gate 集成） | `openspec/changes/add-change-scope` |
| 调研/决策留痕体系 | **部分采纳**：change 强相关调研落 `openspec/changes/<name>/research.md`（随 change 归档），无 change 归属的落 `docs/research/`；不做完整 Agent Notes 体系（openspec change 已承担决策记录职责） | 待流程 change |
| rejected 决策留存 | 候选：openspec 否决的 change 不删，留一行理由 | 待定 |
| 标准注入门禁化（MUST→机械检查） | 候选：standard MUST 条目逐条转 check-standards/quality-gate 检查 | 待定 |
| simplification 显式入口 | 候选：openspec 加"纯减法"change 类型 | 待定 |
| CLAUDE.md symlink 防漂移 | 候选（便宜） | 待定 |
| verify-md-links 文档内链校验 | 候选 | 待定 |
| per-file 100% 覆盖率门禁 | **不抄**（单人项目过重） | — |
| 中英双语三件套 i18n 机器 | **不抄** | — |
| Cordis 插件架构本身 | **不抄**（业务用不上） | — |
| 密集 CI gate 矩阵 | **不抄**（百人仓玩法） | — |

## 附录：change-scope.ts 关键代码摘录

> 源：`scripts/change-scope.ts` @ master（2026-08-21 快照）。要点：防歧义 ref 解析、UTF-8 严格解码、64MB 输出上限、可选锁禁用（与 WSL DrvFS 优化同思路）。

```ts
// 报告结构
interface ChangeScopeReport {
  formatVersion: 1
  repositoryRoot: string
  input: { base: string; head: string }
  resolved: { baseSha: string; headSha: string; mergeBaseSha: string }
  paths: { committed: string[]; staged: string[]; unstaged: string[]; untracked: string[] }
}

// git 调用要点（所有 git 命令统一走此包装）
spawnSync('git', ['-C', cwd, '-c', 'core.fsmonitor=false', ...args], {
  env: { ...process.env, GIT_OPTIONAL_LOCKS: '0', LANG: 'C', LC_ALL: 'C' },
  maxBuffer: 64 * 1024 * 1024,
})
// ref 解析（防注入：--end-of-options；防歧义：warnAmbiguousRefs）
git rev-parse --verify --end-of-options '<ref>^{commit}'
// 四段路径来源
//   committed:  git diff --name-only <mergeBaseSha> <headSha>
//   staged:     git diff --cached --name-only
//   unstaged:   git diff --name-only            （工作区 vs index）
//   untracked:  git ls-files --others --exclude-standard
```
