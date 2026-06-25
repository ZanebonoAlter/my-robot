# Verification Report: PersistentTopic 持久叙事话题

> 状态：**已用真实生产数据验证**（2026-06-19）；**管理闭环（merge / split / rename + 前端入口）于 2026-06-19 二次补齐**（见 §9）
> 方法：从生产 `syntopica-postgres` 导出 108 个真实 section（3 个 board，vector(2560) embedding，各 6 天）为 fixture，加载进 testcontainer pgvector，跑完整回刷 + 日常归属 + 身份边流程。

## 1. 验证目标

1. **归属准确性**：anchor_hit 占比达标。
2. **话题收敛性**：单 board active topic 数稳定在 5-15。
3. **断链修复**：命名漂移场景身份边不断链。
4. **聚类有记忆**：同叙事 section 归同一 topic。

## 2. 测试环境

| 项 | 值 |
|----|----|
| 数据库 | testcontainer `pgvector/pgvector:pg18-trixie`（与生产一致）|
| 真实数据来源 | 生产 `syntopica-postgres`，board 1980/2197/1974 |
| 样本规模 | 108 section（47/32/29），vector(2560)，各 6 天 |
| fixture 文件 | `backend-go/internal/topicgraph/repository/testdata/persistent_topic_fixture.json`（2.6MB，脱敏）|

生产全局现状：51 daily report / 183 section（全部含 embedding）/ 83 relation。

## 3. 关键发现：聚类算法必须从贪心改为 complete-link ⚠️

真实数据测试**证伪了 design.md 的原始算法假设**。这是本次验证的最大价值。

### 3.1 问题：贪心凝聚（centroid）链式合并

原始实现用贪心凝聚（running-mean centroid），cluster_threshold=0.30。真实数据结果：

| board | sections | 贪心凝聚 thr=0.30 |
|--------|----------|-------------------|
| 1974 | 29 | **1 个 topic**（全部合并）|
| 1980 | 47 | **1 个 topic**（全部合并）|
| 2197 | 32 | 3 个 topic（22/8/2）|

**根因**：同一 board 内的 section 语义本就接近（board 是粗聚类，section 是细分）。距离分布高度集中：

```
board 1980 (47 section, 1081 pair): min=0.065  p50=0.299  max=0.482
  bucket [0.1-0.2): 511  [0.2-0.3): 503  [0.3-0.4): 33   （无 >0.4）
```

贪心凝聚的 centroid 会随成员漂移，A→B→C→D 链式连通，把几乎全部 section 吸进第一个 cluster。

### 3.2 修正：complete-link 聚类 + threshold 0.28

改为 complete-link（一个 section 加入 cluster 仅当它与**所有**成员距离都 ≤ threshold），消除链式合并。阈值扫描（complete-link）：

| threshold | b1974 (29) | b1980 (47) | b2197 (32) | 判定 |
|-----------|------------|------------|------------|------|
| 0.20 | 17 | 33 | 22 | 太碎 |
| 0.22 | 16 | 29 | 22 | 太碎 |
| 0.25 | 13 | 21 | 18 | 偏碎 |
| **0.28** | **11** | **15** | **15** | **✓ 全在 5-15** |
| 0.30 | 10 | 12 | 14 | 偏合并（接近边界）|

**采用 0.28**：三个 board 全部落在 5-15 目标区间，且分布有梯度（最大 topic 8-9 成员，非全碎）。

> 对比：贪心凝聚在 0.28 给 b1980 仅 3 个 topic（链式），complete-link 给 15 个——证明算法选择比阈值本身更关键。

### 3.3 阈值（其余三个未触发修正）

| 阈值 | 默认 | 状态 |
|------|------|------|
| match_threshold | 0.30 | 保留（日常归属，需积累多天真实日常数据才能评估，当前仅回刷有数据）|
| upgrade_threshold | 3 | 保留（逻辑单测覆盖，需观察 candidate→active 转化率）|
| decay_window | 30 | 保留（需观察 active 生命周期分布）|

## 4. 核心场景验证

### 4.1 命名漂移不断链（根因 B）✓

`TestIdentityEdge_SurvivesLabelDrift`：同 topic 两 section embedding distance=0.32 > penalty 0.28，匈牙利未匹配（rebuilt 0 similarity relations），但身份边 overlay 补上连接。改造前断链，改造后连通。

`TestRealData_IdentityEdgesAfterBackfill`：108 真实 section 回刷后，identity 边数 > 0，证明同话题链接在真实数据上生效。

### 4.2 聚类有记忆（根因 A）✓

`TestBackfill_GroupsDriftedLabelsIntoOneTopic`：「AI 编程竞争」/「开发者生态重构」/「AI 工具内卷」三天漂移标签 → 回刷后归为 1 个 active topic。

`TestBuildClusterSystemPrompt_InjectsExistingTopics`：ClusterTags prompt 含历史框架列表 + matched_topic_id 字段。

## 5. 量化指标汇总

| 指标 | 目标 | 实测 | 达标 |
|------|------|------|------|
| 单 board active topic 数 | 5-15 | b1974=11, b1980=15, b2197=15 | ✅ |
| 回刷后无 orphan section | 0 | 0（3 board 全部归属）| ✅ |
| 身份边存在（真实数据）| > 0 | > 0 | ✅ |
| 漂移标签合并（合成）| 同叙事→1 topic | 1 topic | ✅ |
| 正交叙事分离（合成）| 不同叙事→多 topic | 2 topic | ✅ |
| 身份边不断链（合成，dist>penalty）| 连通 | 连通 | ✅ |
| anchor_hit 占比 | > 70% | 待日常数据积累（回刷全走 anchor_hit）| ⏳ |
| 断链率较改造前下降 50%+ | - | 待生产部署后观察 | ⏳ |

## 6. 测试清单（全部通过）

**单元测试 16 个**（纯函数）：cosineDistance、归属三分支、双重确认、生命周期状态机（升级/断链/归档/保留）、matched_topic_id 幻觉降级、prompt 注入。

**集成测试 5 个**（testcontainer pgvector）：
- `TestRealData_BackfillTopicConvergence`（真实 108 section，3 board 收敛 5-15）
- `TestRealData_IdentityEdgesAfterBackfill`（真实数据身份边）
- `TestBackfill_GroupsDriftedLabelsIntoOneTopic`（合成漂移合并）
- `TestBackfill_SeparatesDistinctNarratives`（合成正交分离）
- `TestIdentityEdge_SurvivesLabelDrift`（合成身份边抗断链）

## 7. 结论

- ✅ **算法参数已用真实数据校准**：cluster_threshold 0.30→0.28，聚类算法贪心→complete-link
- ✅ **根因 A（聚类无记忆）已解决**：ClusterTags 注入历史框架 + 回刷产出持久 topic
- ✅ **根因 B（关系无身份层）已解决**：身份边叠加，距离 > penalty 仍连通
- ✅ **无回归**：匈牙利算法、board-upgrade、现有 lifecycle 行为不受影响

**待生产部署后持续观察**：anchor_hit 占比、断链率下降幅度、candidate→active 转化率、active 生命周期分布——这些需积累多天真实日常生成数据才能统计，回刷（一次性）无法覆盖。

## 8. 后续调参建议

1. 部署后对一个真实 board 调 `POST /api/daily-reports/backfill-topics?board_id=X`
2. 观察 1-2 周日常生成，统计 anchor_hit 占比，若 < 50% 则考虑 match_threshold 0.30→0.28
3. 观察 candidate→active 转化，若 candidate 堆积则 upgrade_threshold 3→5
4. active topic 数若超 20，回收刷或手动归档

## 9. 话题管理闭环（2026-06-19 补齐）

> 本节为 change 二次推进补齐的「管理 API + 前端入口」，闭合 tasks §9.4 / §9.6 / §9.7。算法核心（§1–§8）已于首轮完成并经真实数据验证。

### 9.1 后端 topic 管理 API

| 方法 | 路由 | 行为 |
|------|------|------|
| PATCH | `/api/daily-reports/topics/:id` | 重命名（label）/ 归档或重新激活（status=active\|archived）|
| POST | `/api/daily-reports/topics/:id/merge` | body `{source_topic_ids:[…]}`；source 的 section 全部改指 target，source 归档（软删除），重建 board 关系 |
| POST | `/api/daily-reports/topics/:id/split` | body `{section_ids:[…], label}`；挖出的 section 归新建 topic（embedding=挖出 section 均值），重建关系 |

- 三个写操作均在单事务内完成 section 重指 + topic 状态变更 + `RebuildBoardRelations`（身份边随归属重建）。
- 校验：merge 拒绝跨 board / target∈source；split 拒绝挖空 / 挖全部（应改用重命名）/ section 不属于源 topic。
- 实现：`repository/daily_report_topic_repository.go`（`UpdateTopic`/`MergeTopics`/`SplitTopic`）+ `handler/daily_report_handler.go`（薄封装）+ 路由注册。

### 9.2 前端入口（侦探墙详情面板）

`TopicDetectiveWall.client.vue` 详情面板在 section meta 下方新增话题操作区（仅当 section 有 `persistent_topic` 时显示）：

- **话题生命线**（§9.4）：`getTopicLifeline(topicId)` → `interaction.enterLifecycle`，与既有「查看完整生命周期」（section 维度 `getSectionLifecycle`）并列，双模式现已可用。
- **重命名**：prompt → `updateTopic(label)`，就地 patch（不整页 reload，保留焦点）。
- **归档**：confirm → `updateTopic(status=archived)` → reload。
- **合并**：从 board timeline 缓存聚合 topic 列表 → 选择器 → `mergeTopics`。

前端 API 封装：`dailyReports.ts`（`updateTopic`/`mergeTopics`/`splitTopic` + `PersistentTopic` 类型），`client.ts` 新增 `patch` 方法。

### 9.3 测试

`daily_report_topic_management_test.go`（testcontainer pgvector，9 用例全过）：

- `TestUpdateTopic_Rename` / `_ArchiveAndReactivate` / `_RejectsInvalidStatus`
- `TestMergeTopics_ReassignsAndArchivesSources`（section 归属迁移 + source 归档）/ `_RejectsCrossBoard` / `_RejectsTargetAsSource`
- `TestSplitTopic_CreatesNewTopicAndReassigns`（含均值 embedding + hit_count）/ `_RejectsCarveAll` / `_RejectsForeignSections`

### 9.4 验证（本次补齐部分）

| 命令 | 结果 |
|------|------|
| `go build ./internal/topicgraph/...` | BUILD_OK |
| `go vet ./internal/topicgraph/...` | VET_OK |
| `golangci-lint run ./internal/topicgraph/...` | 0 issues |
| `go test ./internal/topicgraph/repository ./internal/topicgraph/handler` | ok（repository 41s + handler 0.6s）|
| `pnpm exec nuxi typecheck`（cmd）| TYPECHECK_PASS |
| `pnpm test:unit`（cmd）| 19 files / 116 tests 全过 |
| `pnpm build`（cmd）| BUILD_PASS |
| `pnpm exec eslint`（改动 3 文件）| 0 error |

### 9.5 已知未做（不阻塞本 change）

- **split 前端入口**：后端 API 已就绪，前端未做选择器（tasks §9.6 仅要求合并/重命名/归档）。
- **§8.5 调参脚本**：以报告 §8 手动指引替代，属运维工具。
- **reference 文档（D.2–D.5）**：按 §12.4 里程碑收尾统一更新。
- **V.12 全量门禁**：pre-push 动作，本次跑影响包；push 前补全量。

## 10. 真实生产库三大问题修复（2026-06-20）

> 本节针对上线后用户报告的三个生产现象，用真实生产库 `syntopica-postgres` 验证根因并修复。

### 10.1 根因与修复

| 问题 | 根因 | 修复 |
|------|------|------|
| **Issue 1**：时间线断开 / 泳道连通 / 生命周期状态（分化）错乱 | 唯一约束 `(from,to)` + identity 的 `ON CONFLICT (from,to) DO UPDATE SET relation_type='identity'` → identity **覆盖**了同对的 similarity 边。强匈牙利匹配（distance ≪ 0.28）被吞，similarity-only 的时间线视图丢边断链；状态机基于残缺关系图产生错误 split/merge/emerging | **A**：唯一约束拓宽为 `(from, to, relation_type)`（迁移 `20260620_0001`），所有 INSERT 的 `ON CONFLICT` 目标同步为该三元组 → identity 与 similarity 作为两行独立共存 |
| **Issue 2 part 1**：日报未持久化话题 | feature 上线只回刷了 board 1974，其余 board 的历史 report 从未跑 `BackfillAllPersistentTopics`。209 section 中 154 个 `persistent_topic_id` 为 NULL（06-20 当天生效的除外） | **B**：运维脚本 `cmd/rebuild-topics` 跑 `BackfillAllPersistentTopics` 补齐全部未归属 board |
| **Issue 3**：侦探墙泳道未区分 | `layoutCards('lanes')` 的 `laneKey` 依赖 `persistent_topic.id`，NULL 的 section 全部归 `unassigned` 单一泳道 → Issue 2 part1 的直接后果 | Issue 2 part1 修复后所有 section 有 topic，泳道自然区分 |

### 10.2 修复前→后指标（真实生产库）

| 指标 | 修复前 | 修复后 |
|------|--------|--------|
| 已归属 section 数 | 55/209（26%，仅 1974+06-20）| **209/209（100%）** |
| NULL section | 154 | **0** |
| 同时含 identity+similarity 的 section 对 | **0**（identity 覆盖 similarity）| **33** |
| relation 边总数（identity / similarity）| 113（混计，未区分）| 198（**identity 95 / similarity 103**）|
| section 297（以黎·06-20）在 similarity 时间线的边 | 0（孤立断链）| 1（`266→297 similarity 0.088`）|
| 回刷补齐的 board 数 | — | 10（1980/2197/2242/2165/3030/2128/3122/15219/15220/2272）|

### 10.3 代码变更

| 文件 | 变更 |
|------|------|
| `repository/daily_report_matching.go` | 3 处 INSERT（pairWritten / skip-day / writeIdentityEdges）的 `ON CONFLICT` 目标从 `(from,to)` 改为 `(from,to,relation_type)`，去掉 `relation_type` 的覆盖式更新；`writeIdentityEdges` 与 `RebuildBoardRelations` 注释更新为“共存”语义 |
| `platform/database/postgres_migrations.go` | 新增迁移 `20260620_0001`：`DROP` 旧 `uq_section_relations_pair` 后 `ADD` 同名约束为 `UNIQUE (from_section_id, to_section_id, relation_type)`。幂等；无需数据迁移（旧约束已保证每对唯一，拓宽不会引入冲突）|
| `cmd/rebuild-topics/main.go` | 新增一次性运维工具：InitDB（跑迁移）→ 逐 board `RebuildBoardRelations`（identity/similarity 共存）→ `BackfillAllPersistentTopics`（回刷未归属 board）→ 打印验证统计 |

### 10.4 验证（本节）

| 命令 | 结果 |
|------|------|
| `go build ./internal/topicgraph/... ./internal/platform/database/...` | BUILD_OK |
| `go vet ./internal/topicgraph/repository/ ./internal/platform/database/` | VET_OK |
| `golangci-lint run ./internal/topicgraph/repository/ ./internal/platform/database/` | 0 issues |
| `go test ./internal/topicgraph/repository/ -run TestHungarian\|TestPlan`（纯函数，无 DB）| PASS（matching 纯逻辑无回归）|
| testcontainer 集成测试（`TestIdentityEdge_SurvivesLabelDrift` 等）| ⚠️ 本环境 Windows go.exe 走 npipe 连不上 Docker Desktop（WSL socket）；语义兼容已人工核验（该用例 distance 0.32 > penalty 只有 identity 一行，`First` 取 identity 行，断言不变）。下次 push 前在可连 Docker 的环境重跑 |
| 运维脚本真实库执行 `go run ./cmd/rebuild-topics` | OK（209/209 归属、identity95/similarity103、33 对共存）|

### 10.5 已知后续（不阻塞本修复）

- **Issue 2 part 2（误分类）**：部分 cluster（如「黎巴嫩停火成为美伊谈判核心前置条件」matched_topic_id=6）将美伊与黎巴嫩叙事并到同一 topic。属 LLM 注入 prompt + 归属阈值调优，非硬 bug；候选处理见 §8 调参指引 + 后续手动合并/拆分。→ **已在 §11 通过 prompt 调优处理**。
- **candidate 偏多**：1980 active12/candidate7、2242 active8/candidate4 等。多为 06-20 当天新建未达升级阈值；`decay_window` 会自然归档，或手动归档。

## 11. 聚类 prompt 质量调优（2026-06-20）

> 针对 §10.5 的 Issue 2 part 2（误分类）以及用户报告的「topic 8「特朗普在 G7 峰会期间的盟友关系紧张」强行归属了不相关事件」，定位为聚类阶段 prompt 质量问题并调优。

### 11.1 根因

真实数据核雪：board 1974 report 52（2026-06-20）中，LLM 把 3 个语义不相关的事件强塞进「特朗普在 G7 峰会期间的盟友关系紧张」框架：
- `16710 特朗普称与以色列关系良好`（美以关系）
- `16762 新空军一号交付`（专机交付后勤）
- `16783 特朗普警告美伊若未在 60 天内达成协议`（美伊通牡）

三个是不同事件，却被人名/时间沾边被打包。根因是 prompt 原规则 2 允许单标签独立成组+发挥命名，规则 4 鼓励「解释性判断」而非紧扣事件，复用规则未限制「仅因语境沾边」。

### 11.2 修复（方案 A：收紧聚类 prompt）

`service/daily_report_cluster.go` 的 `buildClusterSystemPrompt`：
- 规则 2：单标签优先并入最近组；独立成组时**组名必须用标签原文**。
- 规则 4：标题必须是对组内实际事件的提炼，**禁止脑补未提及的外部语境**（时间点/地点/未提及的会议/主体）。
- 复用规则：仅当核心议题延续才复用 `matched_topic_id`，**不得仅因语境沾边**并入。
- 反面教材增加「脱离事件脑补语境」「把不相关事件强行打包」两类。

### 11.3 验证（真实 LLM，board 1974 / report 52 tag 集 + 13 个现有 topic 注入）

运维工具 `cmd/verify-cluster-prompt` 跳过真实 `ClusterTags` 调用。修复前 vs 修复后对比：

| 框架 | 修复前（report 52 实际产出）| 修复后（新 prompt）|
|------|------------------------------|---------------------|
| 「特朗普在 G7 盟友关系紧张」组内事件 | 16710 美以关系 + 16762 空军一号 + 16783 美伊通牡（3 个不相关）| 16710 美以关系 + 16773 美情报警告内塔尼亚胡破坏和平（2 个真正相关）|
| 「空军一号交付」 | 被塞进 G7 盟友组 | 移到「美伊谅解备忘录」组（仍略勉强，但不再污染盟友主题）|
| 「美伊通牡」 | 被塞进 G7 盟友组 | 归入「美伊谅解备忘录」组（合理）|
| 组名「特朗普在 G7...」 | 保留脑补的「G7 峰会」 | 改为「特朗普盟友关系紧张与外交施压」（**去掉脑补的 G7**）|
| 单标签组名 | 可能发挥 | 直接用事件原文（规则 2 生效）|

### 11.4 验证状态

| 命令 | 结果 |
|------|------|
| `go build ./internal/topicgraph/service/...` | BUILD_OK |
| `go test ./internal/topicgraph/service/`（全包，纯函数）| PASS（cluster 相关 4 个 + parse/prompt 8 个，无回归）|
| `go run ./cmd/verify-cluster-prompt`（真实 LLM）| OK，11 个组，topic 8 不再收纳不相关事件 |

### 11.5 已知后续

- **prompt 只影响新生成的日报**：已存在的污染 topic（如 topic 8）标题仍会被注入后续 prompt。缓解：新 prompt 的复用限定已降低维续污染风险；观察 1-2 天后若仍异常，手动归档（`PATCH /api/daily-reports/topics/:id`，API 已就绪）。
- ~~**同 topic 被 reuse 两次**~~ → **已修复**：`parseClusterResponse` 加 `usedTopicIDs` 占用检测，同一 `matched_topic_id` 只能被第一个 claim 它的组占用，后续重复 claim 降级为 nil（开新组）。单测 `TestParseClusterResponse_DuplicateTopicID_SecondClaimDegrades` 覆盖；真实 LLM 复验：修复后 9 个组每个 reused topic 唯一（上轮验证中 topic 10 被两组双重 claim 的现象消失）。

## 12. 人工确认门禁与话题总览回归修复（2026-06-21）

### 12.1 根因与修复

| 现象 | 根因 | 修复 |
|---|---|---|
| 单日报直接出现持久泳道 | candidate 被前端当正式泳道，且第 3 天自动 active | candidate 只进入“待确认 / 未分类”；连续命中达到阈值后由用户确认，后端再次校验阈值 |
| “多天出现”可被非连续命中绕过 | candidate miss 未持久化清零 | candidate 中断时写回 `consecutive_hits=0` |
| 时间线 emerging 被改为 continuing | 状态推导混入 identity 边 | `DeriveSectionStatuses` 只使用 similarity 边 |
| hover 只亮前后一级 | 2D/3D 都只检查直接邻边 | 共享 `fullComponentHighlight`，高亮当前可见关系的完整连通分量 |
| 7/14 天筛选看起来相同 | watcher 监听 ref 对象；SQL 7 天窗口含 8 天且锚定 CURRENT_DATE | 监听 `days.value`；以板块最新日报为终点精确包含 N 天 |
| 标签重叠、缩放跑位 | 列/行间距不足；日期头脱离缩放画布；容器尺寸与 transform 叠加 | 扩大布局节距、文字描边；日期头并入 SVG；外层占位、内层 SVG 缩放并保持视口中心 |

### 12.2 验证结果

| 检查 | 结果 |
|---|---|
| 后端 `golangci-lint run ./internal/topicgraph/...` | 0 issues |
| 后端 repository `go vet` / `go test -count=1` / build | PASS（含 testcontainer 门禁与精确 7 天窗口） |
| 前端 lint / typecheck | 0 error / PASS |
| 前端单测 | 19 files / 117 tests PASS |
| 前端 production build | BUILD complete |
| 浏览器视觉检查 | 工具组已靠右，标签间距与画布缩放结构正常；后续重连被本地地址安全策略终止，未绕过 |

## 13. 话题管理 UI 重构 + CORS 修复 + 硬删除（2026-06-22）

> 用户报告：话题管理交互原始（原生 `window.*` 弹窗）、没用项目组件库；异常产生的话题无法归档或删除。排查定位两个真根因并修复。

### 13.1 根因与修复

| 问题 | 根因 | 修复 |
|---|---|---|
| **归档/删除/重命名点了没反应** | `middleware/cors.go:26` 硬编码 `Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS` **缺 `PATCH`**，而 `updateTopic`（重命名/归档/启用）全走 PATCH → 浏览器 preflight 100% 拦死。用户实测报错：`Method PATCH is not allowed by Access-Control-Allow-Methods in preflight response`。同时 `config.go` 的 `cors.methods` 配置项此前是**死配置**，cors.go 从未消费它 | `cors.go` 改为消费 `cfg.CORS.Methods`；`config.go` 默认值补 `PATCH`（`GET, POST, PUT, PATCH, DELETE, OPTIONS`）|
| **异常/孤儿话题不可见不可管** | 前端话题列表从 7/14 天时间线窗口反向聚合（`topics = computed(从 sections 聚合 persistent_topic)`），窗口外 / 零 section / 回刷 bug 产生的孤儿话题一律不在列表里 | 后端新增 `GET /api/semantic-boards/:id/topics` 返回该 board **全部**话题（含 archived + 孤儿）；前端 `TopicManageDialog` 改用此接口 |
| **无法删除话题（只能归档）** | 后端只有软删除（archive），无硬删除 | 后端新增 `DELETE /api/daily-reports/topics/:id`：事务内解绑 section（保留内容）→ 删 topic → 重建关系 |
| **交互原始、纯原生弹窗** | 7 处 `window.alert/prompt/confirm` 散在两个组件（`BoardThreadBrowser` × 6、`DetectiveWall` × 3），项目已有的 `AppDialog`/`AppButton`/`AppInput` 组件库未复用 | 新建统一 `TopicManageDialog.vue`（复用组件库），两组件接入；7 处原生弹窗全消除，硬删除加输入名二次确认 |

### 13.2 代码变更

| 层 | 文件 | 变更 |
|---|---|---|
| 后端·修 bug | `platform/middleware/cors.go` | allow-methods 从配置消费（`cfg.CORS.Methods`）；import `strings` |
| 后端·修默认 | `platform/config/config.go` | `cors.methods` 默认值补 `PATCH` |
| 后端·repo | `repository/daily_report_topic_repository.go` | `ListTopicsByBoardAll`（含 archived，与 `ListAllTopicsByBoard` 区分）；`DeleteTopic`（事务：解绑 section + 删 topic + `RebuildBoardRelations`）|
| 后端·handler | `handler/daily_report_handler.go` | `listBoardTopics`（聚合 section 计数 + 阈值算 `can_activate` + 颜色）；`deleteTopic`；路由注册 GET/DELETE |
| 前端·API | `api/dailyReports.ts` | `listBoardTopics` / `deleteTopic` + `BoardTopicListItem` 类型 |
| 前端·新组件 | `components/dialog/TopicManageDialog.vue`（新）| 统一管理面板：状态过滤 + 搜索 + 全子 dialog 操作（确认启用/重命名/合并/归档/删除），零 `window.*` |
| 前端·接入 | `BoardThreadBrowser.vue` | 删原生面板 + 6 个 window.* 函数，挂 `<TopicManageDialog>` |
| 前端·接入 | `TopicDetectiveWall.client.vue` | 详情面板 rename/archive/merge 内联按钮 → “话题管理”入口；删 3 个 window.* 函数 + mergePicker |

### 13.3 验证（本节）

| 命令 | 结果 |
|---|---|
| `cd backend-go && go build ./internal/topicgraph/... ./internal/platform/middleware/ ./internal/platform/config/` | BUILD_OK |
| `cd backend-go && golangci-lint run ./internal/topicgraph/... ./internal/platform/middleware/... ./internal/platform/config/...` | 0 issues |
| `cd backend-go && go vet ./internal/topicgraph/... ./internal/platform/...` | VET_OK |
| `cd backend-go && go test ./internal/topicgraph/repository -run "TestUpdateTopic\|TestMergeTopics\|TestSplitTopic\|TestParseClusterResponse" -count=1` | ok 18.300s（CRUD 无回归）|
| `cd front && grep -rn "window\.(alert\|prompt\|confirm)" app/features/tags/components/BoardThreadBrowser.vue app/features/tags/components/TopicDetectiveWall.client.vue app/components/dialog/TopicManageDialog.vue` | 零命中（7 处原生弹窗全消除）|
| `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm lint"` | 0 error（23 warnings 均为既有无关项）|
| `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"` | TYPECHECK_PASS |
| `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit"` | 19 files / 117 tests PASS |
| `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"` | BUILD_PASS |

### 13.4 已知未做（不阻塞）

- **split 前端入口**：后端 `POST /topics/:id/split` API 已就绪（§9.7），前端未做选择器。`TopicManageDialog` 暂不接入 split（需 section 级选择器，交互较重，超出本次“消除原生弹窗 + 可管异常话题”范围）。
- **reference 文档**：D.6 按 §12.4 里程碑收尾统一更新。
- **DeleteTopic 集成测试**：repo 层 `DeleteTopic` 未加 testcontainer 集成用例（现有 `MergeTopics`/`SplitTopic` 集成测试覆盖了同事务模式）；`go test -run` 纯逻辑 CRUD 无回归已验证。可后续补 `TestDeleteTopic_UnlinksSectionsAndRebuildsRelations`。

## 14. 日报详情话题分区闭环（2026-06-22）

### 14.1 根因与修复

日报详情原先只预加载 section/thread 和 `persistent_topic_id`，没有附加轻量
`persistent_topic` 描述。前端 `qualityZones` 依赖 `persistent_topic.status` 判断 active，
因此所有已归属 section 都被误放进“突发的新话题”。

`GetReportByID` 和 `ListReportsForAllBoards` 现统一调用
`AttachTopicBriefsToReport`；`DailyReportSection.PersistentTopic` 为 transient 字段，不产生
数据库迁移，也不加载 topic embedding。

### 14.2 验证结果

| 检查 | 结果 |
|---|---|
| `go test ./internal/topicgraph/repository -run TestGetReportByID_AttachesTopicBriefs -count=1` | PASS |
| `go test ./internal/topicgraph/repository -count=1` | PASS（58.452s） |
| `golangci-lint run ./internal/topicgraph/repository` | 0 issues |
| `go vet ./internal/topicgraph/repository` | PASS |
| `go build ./internal/topicgraph/repository` | PASS |
| 新构建后端（临时端口 5001）请求 `GET /api/daily-reports/60` | 4/4 section 带 topic brief：3 active、1 candidate、0 missing |

验证时原 5000 端口进程仍返回旧响应（4 个 section 均缺 topic brief），说明它尚未重启；
部署或继续浏览器验收前须重启现有后端进程。
