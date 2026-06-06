## Context

> **Note:** The `MatchAndSaveRelations` function mentioned below has been superseded by `RebuildBoardRelations` (Hungarian bipartite matching). See `docs/plans/2026-06-06-bipartite-relation-matching.md`.

日报系统当前有两层结构：Section（话题/聚类）和 Thread（线索）。Section 之间通过 `prev_section_id` 形成单链式跨天延续，Thread 通过 `prev_thread_id` 和 tag 交集做类似的链式追踪。

实际数据暴露了两个问题：
1. `prev_section_id` 只能表达一对一关系，无法表达叙事分化（一个叙事分裂为多个子叙事）和合并（多个叙事汇聚为一个）
2. Thread 级别的 lineage 追踪价值有限——用户关心的是叙事如何演变，而非具体线索的延续

聚类 prompt 已调整为叙事级标题（`cluster.go` 的 `buildClusterSystemPrompt`），Section 定位为"跨事件的解释框架"。

## Goals / Non-Goals

**Goals:**
- 用多对多关系表替代单链 `prev_section_id`，支持 split（一对多）和 merge（多对一）
- 从关系表拓扑动态推导 Section status（emerging / split / merge / continuing）和 ended 标记
- 简化 Thread 为纯事件条目，移除 status 和 prev_thread_id
- 前端 Gantt 图可视化 split/merge 关系

**Non-Goals:**
- 不改聚类算法或 prompt（已单独调整）
- 不改 embedding 生成逻辑（仍基于 cluster_label）
- 不做 narrative 聚合层（现有两层结构足够）

## Decisions

### D1: 关系表 vs 多值 prev_section_ids

**选择**：新增 `daily_report_section_relations` 表

**备选**：把 `prev_section_id` 改为 `prev_section_ids INT[]`

**理由**：数组字段难以做反向查询（查 merge 需要找"谁指向我"），关系表天然支持双向遍历，且可以存储 distance 用于排序和过滤。同时保留 `from_section_id`（前一天的 section）→ `to_section_id`（当天的 section）的方向语义，与时间方向一致。

### D2: 关系写入时机

**选择**：在 `SaveReport()` 事务中，新 section 保存后立即写入关系

**流程**：
1. 新 sections 写入数据库（获得 ID）
2. 对每个新 section，用 embedding 查询同 board 下所有**非当日** section，找所有 distance < 0.35 的匹配
3. 为每个匹配插入一条 relation（from=旧 section, to=新 section, distance）

**阈值**：0.35（比之前的 0.3 略宽）。叙事级标题语义范围更广，"开发者 Agent 工具链进入平台化竞争" 和 "AI Agent 生态进入平台化竞争" 应该能匹配上。

### D3: Status 动态推导 vs 存储

**选择**：动态推导，不存储 status

**理由**：status 完全由关系拓扑决定，存储会导致数据不一致风险。timeline/lifecycle API 查询时推导即可。

推导规则（两阶段，status 和 ended 独立推导）：

**阶段一：关系状态（status）**——描述该 section 与前驱的关系
1. 无 from 关系 → `emerging`
2. 有多个 from 关系（to 入度 > 1）→ `merge`
3. from 的出度 > 1（前驱 section 还被其他新 section 指向）→ `split`
4. 有 from 关系且 from 出度 = 1 → `continuing`

注意：一个 section 可以同时是 split 和 merge（一个叙事分化后又与另一个合并）。取优先级 merge > split > continuing。

**阶段二：结束标记（ended）**——描述该 section 是否无后继
- 无 to 关系（无 relation 的 from_section_id 指向它）且 不是最新一天 → `ended = true`
- 最新一天的 section 即使无 to 关系 SHALL NOT 被标记为 ended

API 返回每个 section 包含 `status`（emerging/continuing/split/merge）和 `ended`（boolean）两个字段，前端可分别用于节点颜色和结束视觉标记（如降低透明度或灰色边框）。

### D4: Thread 简化范围

**选择**：移除 `status` 和 `prev_thread_id`，保留其他字段

**保留**：title, summary, tag_ids, confidence, related_article_ids, section_id, report_id
**移除**：status, prev_thread_id
**移除的 API**：thread lineage API、thread timeline API
**移除的前端**：ThreadLineagePanel 组件、thread sitemap 图标按钮

### D5: 前端 DAG 可视化（引入 d3-dag 布局引擎）

**选择**：引入 `d3-dag` 作为 DAG 布局引擎 + 自定义 Vue 3 SVG 渲染

**备选**：
- `@dagrejs/dagre`：更轻量(13KB)但算法较旧，维护一般
- `@vue-flow/core`：完整渲染框架，但对只读 DAG 视图过重(~80KB)
- `elkjs`：最强大但 423KB gzip，不可接受
- `@gitgraph/js`：专为 git graph 设计，但已停更，且命令式 API 与 Vue 响应式不匹配

**理由**：
- d3-dag 提供 Sugiyama 分层布局算法，天然支持分支/合并的 x/y 坐标计算和边的路径点
- TypeScript-first，活跃维护（2026），40KB gzip 合理
- 只做布局计算，渲染完全由 Vue 模板控制，不引入额外渲染框架
- 项目已有 3d-force-graph + three，SVG 层面加个布局库合理

**工作模式**：`API 数据 (nodes + edges) → d3-dag 布局 (x/y + path) → Vue SVG 模板 (渲染)`

**两个组件的改造方向**：

#### D5a: SectionLifecyclePanel → 垂直 DAG（git log --graph 风格）

- 以点击话题为中心，后端 BFS 已返回完整连通子图（所有上下游 sections + relations）
- d3-dag Sugiyama 布局，rankdir=TB（从上到下按时间分层）
- 同层（同一天）的节点水平排列，跨层通过边连接
- 分支点：一个节点扇出多条边到多个子节点
- 合并点：多条边汇聚到一个节点
- 当前选中的话题（sectionId）高亮，其余节点正常显示
- 节点可点击跳转到对应日报
- 面板宽度不变(320px)，内部用 SVG 渲染，支持滚动

```
     5/20  ●  AI Safety Debate (emerging)
            │
     5/22  ●  AI Safety Debate (continuing)
           ╱ ╲
  5/24  ●     ●  AI Ethics / AI Regulation (split)
          ╲   ╱
     5/26  ●  AI Safety & Regulation (merge)
            │
     5/28  ○  AI Regulation (ending, 透明度降低)
```

#### D5b: BoardThreadBrowser → 水平时间线 DAG

- 横轴=日期列，纵轴=话题链（lane）
- d3-dag Sugiyama 布局，rankdir=LR（从左到右按时间）
- 分支时子话题分配到新 lane（新行），合并时多个 lane 汇聚
- 节点颜色按 status：emerging=绿、continuing=蓝、split=橙、merge=紫
- ended=true 的节点降低透明度或灰色边框
- 连线使用 SVG path（贝塞尔曲线），视觉上呈现分支/合并
- 点击节点弹出详情卡片

```
         5/20    5/22    5/24    5/26    5/28
Topic A ──── ●───────●───────●
                        ╲
Topic B                   ●───────●───────●
                         ╱
Topic C  ───────────────●
```

### D6: 同日 Section 合并策略（两阶段）

**选择**：embedding 确定性合并 + LLM 仲裁灰色地带

**背景**：聚类 LLM 经常将同一叙事框架的 tags 拆成多个 cluster（如 6 月 2 日 "Claude Code 核心功能" vs "Claude Code 工作流" distance=0.117，"AI 编程生态" vs "AI 编程技能" distance=0.144）。通过实际数据分析：

- `dist < 0.20`：9 对该合，0 对误合（100% 精确）
- `0.20 - 0.25`：3 对该合，15 对不该合（17% 精确）
- `> 0.25`：无该合的

纯 embedding 阈值在 0.20-0.25 区间无法区分，需要 LLM 介入。

**Stage 1：确定性合并（embedding）**
- distance < 0.20 → 直接合并
- 合并方式：保留 article_count 最大的 section 作为主 section，将被合并 section 的 threads 迁移到主 section 下
- 主 section 的 cluster_label 不变，tag_ids 合并

**Stage 2：LLM 仲裁（灰色地带）**
- distance 0.20 - 0.25 的 pair 批量送 LLM 判断
- 输入：两个 section 的 `(cluster_label, tag_labels[])`
- 输出：`merge: true/false`
- 批量调用（非逐对），减少 API 开销

**时机**：在 `SaveReport()` 事务中，sections 写入后、relations 写入前执行合并。合并后减少 section 数量，关系写入基于合并后的 sections。

**备选**：纯 embedding 0.20 阈值（简单但漏掉 ~25% 该合的）

### D7: distance=0.0 bug 修复

**问题**：`MatchAndSaveRelations` 中 `FirstOrCreate` 导致 distance 全部存储为 0.0。

**根因**：SQL 查询返回的 distance 正确，但 GORM `FirstOrCreate` 在记录不存在时创建新记录会忽略已设置的 Distance 字段（或 `numeric` 列与 `float64` 的 scan 映射问题）。

**修复**：将 `FirstOrCreate` 改为 `FirstOrCreate` + `Updates`，或改用 raw SQL INSERT ON CONFLICT。

## Risks / Trade-offs

- **[关系表膨胀]** → 每个 section 平均 1-3 条 relation，14 天 × 40 sections ≈ 2000 条，可控。加 `(from_section_id, to_section_id) UNIQUE` 约束防重复。
- **[embedding 匹配噪声]** → 叙事级标题比事件级更抽象，embedding 距离可能更近，阈值 0.35 可能需要后续调整。在 relation 表存 distance 方便后续过滤。
- **[同日合并 LLM 成本]** → 每次生成日报多一次 LLM 调用，但只处理灰色地带 pairs（0.20-0.25），预计每天 3-10 对。可用低优先级模型（如 Gemini Flash）降低成本。
- **[合并后标题信息丢失]** → 被合并 section 的 cluster_label 丢弃，主 section 标题可能不够全面。可接受，因为标签级别信息仍保留在 tag_ids 中。
- **[Thread 简化后前端功能缺失]** → ThreadLineagePanel 移除后，用户无法查看线程级血统。可接受，因为真正的追踪维度已上移到叙事级。
- **[d3-dag 布局性能]** → 节点数通常 <100（14天 × 最多10条活跃叙事），d3-dag Sugiyama 在此规模下毫秒级完成。若扩展到 90 天可能有数百节点，可考虑虚拟滚动或分层加载。
- **[d3-dag 依赖维护]** → d3-dag 活跃维护（2026年4月更新），但仍是个人项目。布局算法相对稳定，风险可控。如需替换可切换到 @dagrejs/dagre（API 差异不大）。
- **[SaveReport upsert 悬挂关系]** → 重生成同一 board+date 报告时，需先清理旧 section 相关的 relation 记录再删除旧 section，否则其他天的 section 会指向已删除的 section。

## Migration Plan

1. 创建 `daily_report_section_relations` 表
2. 从现有 `prev_section_id` 数据迁移：对每个有 prev_section_id 的 section，插入一条 relation
3. `daily_report_threads` 删除 `status`、`prev_thread_id` 列和相关索引
4. `daily_report_sections` 删除 `prev_section_id` 列和 `status` 列
5. 部署新代码

Rollback：保留 `prev_section_id` 列直到确认新关系表工作正常。
