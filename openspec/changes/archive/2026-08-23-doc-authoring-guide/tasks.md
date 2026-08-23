# doc-authoring-guide 任务

## 1. 实测提取注册点事实（design D2：不凭记忆写）

- [x] 1.1 ✅ 已完成（结论）：`doc-impact-applies` 扫 flow(深1)+standard(深2) 头 15 行，裸行/`<!-- -->` 皆可，`| section=节名` 可选；路径命中为**前缀包含**，仅档位激活时生效；flow 注入硬编码抓「业务约束与不变量」节
- [x] 1.2 ✅ 已完成（结论）：check-standards 实际段为 A 文档完整性/B 后端结构/C 前端结构/D 防孤立引用（standard/**含 shared**/*.md 须被 AGENTS/子 AGENTS/standard README/开发执行规范之一引用文件名）/E flow 溯源/F doc-impact 对账/G 导航死链（扫 docs/reference/*.md 一级）/H model tag
- [x] 1.3 ✅ 已完成（结论）：spec-gate 四项＝① doc-impact verify 退出码 0 ② check-standards 退出码 0 ③ tasks 尾三节+声明标记 ④ scenario-trace（验证节 `| Scenario | 测试文件 |` 表，「人工」开头合法）
- [x] 1.4 ✅ 已完成（结论）：standard 文档有先例——`standard/backend/ai-logging.md` 块注释裸行形态 `section=Requirements`；自举走 `<!-- doc-impact-applies: docs/reference/ | section=注册点速查 -->`，standard/shared 深度 2 在扫描范围内

## 2. 编写 doc-authoring.md（主产物，目标 ≤200 行）

- [x] 2.1 ✅ 目录职责表 11 行（含 research/ 无约束定位、development/testing 仅存参考）
- [x] 2.2 ✅ 三注释速查表（语法/实例/后果三列，实例取 daily-report.md 与 ai-logging.md 真实标签，后果与 1.1 实测行为一一对应）+ flow 节名红线（硬编码抓取）
- [x] 2.3 ✅ flow 模板（五段式骨架 + 头部标签 + 变更溯源空表初始态）
- [x] 2.4 ✅ checklist ×2，逐项挂门禁标注；「维护提醒」节含 harness change 须同步本文件的提示
- [x] 2.5 ✅ 头部 `<!-- doc-impact-applies: docs/reference/ | section=注册点速查 -->`（第 4 行≤ 15 行限；standard/shared 在扫描深度 2 内；section 名与文档 `## 注册点速查` 一致）
- [x] 2.6 （用户反馈轮 1）补「最佳实践案例」节：5 行案例表（daily-report / ai-logging / watch-keyword-and-quickadd 域声明 / tier-b 豁免 / scenario-test-mapping-gate 反面教材），全部真实文件
- [x] 2.7 （用户反馈轮 1）目录职责表 research 行修正：`docs/reference/research/` 已删（用户操作），现役落盘区＝顶层 `docs/research/`，归置由用户统一处理
- [x] 2.8 （用户反馈轮 1）注册点速查补「三个注释各是干嘛的、为什么这么设计」动机表（设计动机来自 constraint-injection.ts 实测行为与 43K 全量不可行的实测依据）

## 3. 引用与登记

- [x] 3.1 ✅ §0.6 编排六步「业务域声明」引用块尾追加「文档新增/修订」一段（相对链接）
- [x] 3.2 ✅ 执行规范表 +1 行（相对链接，G 段死链预检过）

## 4. 测试

- [x] 4.1 ✅ 纯文档 change：豁免代码测试（git status 本 change 仅 4 个 docs 文件 + openspec 目录；工作树其余改动属并行 change）

## 5. 文档

<!-- doc-impact: standard -->
<!-- doc-impact-excuse: flow=工作树其余 flow/*.md 修改属其他并行 change/窗口，非本 change 改动; api=同前，api/*.md 非本 change 改动; database=同前，database/*.md 非本 change 改动; architecture=同前，architecture/map.md 非本 change 改动; configuration=同前，configuration.md 非本 change 改动 -->

- [x] 5.1 ✅ 新建 `docs/reference/standard/shared/doc-authoring.md`（93 行）
- [x] 5.2 ✅ 开发执行规范 +引用段
- [x] 5.3 ✅ constraints-index +1 行
- [x] 5.4 ✅ standard/README 「这层装什么」表 +1 行

- [x] 5.5 归档声明：纯工具链/文档标准 change，无 flow 影响（不触及任何业务 flow 文档，E 段豁免）

## 6. 验证

- [x] 6.1 ✅ 第 4 行命中，语法 `docs/reference/ | section=注册点速查` 与 APPLIES_TAG_RE 实测一致
- [x] 6.2 ✅ 三文件各 1 处命中
- [x] 6.3 ✅ check-standards 114 过 / 0 失败（含 D 段「被引用 doc-authoring.md」、F 段 doc-impact 通过、G 段无死链；另有顺手修复：E 段存量债 scenario-test-mapping-gate 归档漏写「无 flow 影响」豁免词，已在其 archive tasks.md 补记 3.2 一行，见落地报告）
- [x] 6.4 ✅ verify 通过（声明: standard  文件: 4 个）；流程中曾 5 FAIL 系并行 change 脏工作树误报，已按 tier-b 先例加 doc-impact-excuse（只豁免疑似遗漏）
- [x] 6.5 ✅ valid
- [x] 6.6 ✅ 93 行 ≤ 220（反馈轮后复核见 6.7）
- [x] 6.7 （反馈轮）`wc -l` 复核 ≤220；案例表引用的 5 个文件逐一存在（daily-report / ai-logging / watch-keyword-and-quickadd proposal / tier-b tasks / scenario-test-mapping-gate tasks）；`bash scripts/check-standards.sh` 复跑零失败

### Scenario → 测试文件映射

| Scenario | 测试文件 |
| --- | --- |
| 新增 flow 域有 checklist 可循 | 人工：按 doc-authoring.md「新增 flow 域 checklist」逐项走查注册项与 1.1-1.3 实测结论一致 |
| 头部注释写错的后果可查 | 人工：三种注释速查表每行「写错的后果」与 constraint-injection.ts/doc-impact.sh 实现行为对账（6.1 辅证） |
| 归档前可知会触发哪些门禁 | 人工：checklist 门禁标注与 check-standards A-H 段 / spec-gate ①-④ 实测清单逐条对账 |
| 标准文档自举与一致性 | 人工：头部标签语法校验（6.1）+ 两处登记存在（6.2）+ 一致性校验（6.3-6.5） |
