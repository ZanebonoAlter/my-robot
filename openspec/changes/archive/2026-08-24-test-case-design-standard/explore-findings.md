
## constraint-injection JIT 机制实现要点

JIT 注入机制事实（design D1/D2 的依据，实现任务 4.1 冒烟断言时直接用）：

1. 标签扫描：`scanAppliesTags(cwd)`（constraint-injection.ts ~L347）扫 flow+standard 文档**头部 15 行**的 `doc-impact-applies: path, ... | section=节名` 标签（正则 APPLIES_TAG_RE L311，兼容裸行与 `<!-- -->` 形态），按文件 mtime 缓存。
2. **一文档一组 {signals, section}**：map 按 doc 键 set，多标签行后者覆盖前者——不支持多路分节注入，故设计用单一「JIT 注入摘要」节。
3. 命中语义：编辑路径 `p.includes(signal)` **子串匹配**（L1220 附近 `tag.signals.some((s) => p.includes(s))`），无通配符。选定信号：`openspec/changes/, _test.go, .spec.ts, .test.ts`。
4. 命中会话内粘性只增不减（jitDocHits Map，L120）；注入组装时 jit 层排在 keyword 层之后（L758-761）。
5. 预算 32KB 超线分层降级 keyword → jit（既有机制，无需新规则）；摘要节硬约束 ≤3KB（spec 锁定）。
6. 新文档路径：`docs/reference/standard/shared/test-design.md`——walkDocs 只收 `.md` 且排除 README.md，shared/ 目录在扫描范围（commit-pr.md 无标签但会被扫到，无标签返回 null 不注入）。

<!-- pinned 2026-08-23T15:20:43Z -->

## spec-gate 结构与冒烟测试惯例

spec-gate 检查⑤ 与冒烟惯例实现要点：

1. spec-gate.ts（225 行）现有结构：拦 `openspec archive` bash 命令；检查①doc-impact.sh verify ②check-standards.sh ③尾三节（TAIL_SECTIONS=["测试","文档","验证"]）+`<!-- doc-impact:` 标记 ④scenario-trace.sh；任一失败 block。
2. warning 留痕模式（检查⑤复用）：`pi.sendMessage({ customType: "spec-gate-warning", content, display: true }, {steer:true})`——steer 送达不打断回合；失败降级 console.warn（L203-211 `落 custom_message 留痕` 函数）。
3. 豁免短路（--force / SPEC_GATE_BYPASS=1）在 L71 附近**先于检查链**执行——检查⑤天然被豁免路径短路，无需额外处理（任务 2.2 验收点）。
4. 自身异常 fail-open：console.warn + 留痕不 block（既有策略，⑤整体 try/catch 包住即可）。
5. 冒烟惯例：esbuild 打包 .ts → 临时 .cjs → node 跑断言 → rm 清理（见 run-harness-smoke.sh）；constraint-injection 冒烟在 `run-smoke.sh`（constraint-injection.smoke.cjs 已 27KB 有现成断言结构可扩展）；**spec-gate 目前无冒烟**，新建 spec-gate.smoke.cjs 后挂进 run-harness-smoke.sh。
6. 纯函数抽取要点：scanAcceptanceWording(tasksMd string, changeDirFiles []string) []string 不 import fs——⑤a 需要文件列表作参数（test-cases*.md 存在性判定在外层做好传入）；⑤b 判定粒度=单个任务行（`- [ ]`/`- [x]` 起到行尾）。
7. 无独立 typecheck 入口（.pi 下无 tsconfig/package.json）——esbuild bundle 成功 + 冒烟 PASS 即验证。

<!-- pinned 2026-08-23T15:20:43Z -->
