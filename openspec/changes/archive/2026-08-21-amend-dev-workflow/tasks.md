# Tasks: amend-dev-workflow

## 1. 目录迁移与规则落地

- [x] 1.1 新建 `docs/research/`；迁移 `docs/experience/extensions-research` → `docs/research/extensions`（源文件未跟踪过 git，git mv 不适用改用 mv）；grep 全仓确认无残留旧路径引用
- [x] 1.2 `docs/research/README.md`：定位（通用调研池）、与 experience 的分界判据（事前数据 vs 事后教训）、命名规范（kebab-case topic）

## 2. 流程文档重写

- [x] 2.1 `docs/reference/开发执行规范.md` §2 重写：「实现纪律 — TDD」→「实现纪律 — 用例先行」（黑盒 Scenario 即用例 / 复杂逻辑白盒用例文档 / 顺序解绑 / 两条底线：bug 先复现 + 断言判据主线程定 / 新红旗清单：动手前 specs 无 Scenario、复杂档缺白盒用例、归档无映射、bug 修复无复现测试）
- [x] 2.2 §0.6 编排六步更新：步骤 1 补「调研驱动型 change 产出 research.md（含关键代码摘录+源路径+快照日期）」；步骤 3 前补「复杂逻辑先派子线程枚举白盒用例」；步骤 2 计划要求含 Scenario→测试文件映射表雏形
- [x] 2.3 §0.5 文档归属规则补两行：`docs/research/<topic>.md`（无归属通用调研）/ change 目录 research.md（change 强相关调研）
- [x] 2.4 根 `AGENTS.md` 同步：superpowers 替代表中 test-driven-development 行改为「开发执行规范 §2（用例先行，以 §2 表述为准）」；若 §11 归档门禁措辞引用 TDD 字样一并更新

## 3. 既有引用清理

- [x] 3.1 grep `experience/extensions-research`、`严格 TDD`、`没有失败测试` 等，清理残留引用（含 .pi/、skills、docs）

## 5. 测试

纯流程文档 change，豁免代码测试，附 grep 一致性校验：

- [x] T1 grep 校验见验证节 V1-V4

## 6. 文档

<!-- doc-impact: none(纯流程文档修订+目录迁移，不触七域启发式路径) -->
<!-- doc-impact-excuse: flow=工作区其他进行中 change 脏文件命中，非本 change 改动; api=同上; database=同上; architecture=同上; configuration=同上 -->

- [x] D1 `docs/reference/开发执行规范.md` §0.5/§0.6/§2/§11 更新（任务 2.1-2.3）
- [x] D2 根 `AGENTS.md` superpowers 替代表与归档措辞同步（任务 2.4）
- [x] D3 `docs/research/README.md` 新建（任务 1.2）

## 7. 验证

<!-- 归档门禁：逐条「命令 + 期望结果」 -->

- [x] V1 `grep -rn "experience/extensions-research" --include="*.md" --include="*.ts" --include="*.json" . | grep -v openspec/changes/ | grep -v docs/research/` → 空输出（旧引用仅允许存在于本 change 与归档目录）
- [x] V2 `grep -n "用例先行" docs/reference/开发执行规范.md | head -3` → ≥1 命中（§2 已重写）
- [x] V3 `grep -c "先写复现测试\|复现测试" docs/reference/开发执行规范.md` → ≥1（bug 底线保留）
- [x] V4 `ls docs/research/README.md docs/research/extensions/constraint-injection.json` → 两文件均存在（迁移完成）
- [x] V5 `bash scripts/doc-impact.sh verify openspec/changes/amend-dev-workflow` → 通过
- [x] V6 `bash scripts/check-standards.sh` → 100/2（A-E/H 全 OK；F 段 2 失败为存量进行中 change 的 doc-impact 对账，非本 change 引入，与前一会话 add-change-scope V4 记录一致）

## 备注

- 本 change 自身按新规则执行示范：spec.md 的 Scenario 即用例（纯文档 change，测试=T1 grep 校验 + V 节命令）；无复杂逻辑，无需白盒用例文档。
- doc-impact.sh `context` 子命令的去留由 `port-constraint-injection` 决定，本 change 不动。
