# test-cases: watch-materialized-topic

> **故事总纲**：让"我在追踪"从只读命中提示（导航条跳转）升级为**日报里直接长出的专属领地**——关键字场景把当天含词文章聚合成固定名临时板块（零 AI、可捞 tag 漏网），一句话场景用向量检索辅助标签、物化为挂专属持久话题的板块（跨天延续、lane 锚定一等公民）。收尾追加：旧提示轨（label/keyword）创建入口在前端退役隐藏。
>
> 五个故事：S1 关键字物化（文章扫描→机械聚合→临时板块）→ S2 一句话物化（向量检索→标签解析→持久话题板块）→ S3 管线边界与互斥（排除规则面）→ S4 生命周期与删除联动（consecutive_hits 推进 / 归档确认）→ S5 创建与管理 UI（物化轨双选 + 旧入口退役）。
>
> **层选择总则**：DNF 匹配/余弦/组装契约 = 函数单测（无 DB，无网络）；SQL 窗口/择优/FK/migration = testcontainer PG；管线钩子与生命周期 = service 级 PG 集成；HTTP 契约（四类型创建/删除确认）= handler PG 测试；组件行为/排序 = Vitest。

## 故事 S1: 关键字物化——当天含词文章聚合成固定名板块

（锚 Requirement: 关键字轨物化生成 + watch-materialized-topic lane_tier 标记）

用户建「关键字话题」关注（表达式如 `harness`），夜里日报生成时扫描当天**全量未归档文章**（标题+择优摘要层），命中文章机械聚合为一条「关键字『harness』相关话题」section（`lane_tier=watch_keyword`，每文章一条 thread，零 AI 调用）；没被打 event tag 的漏网文章也能被捞回。

### 主链路（节拍串联）

| 步 | 动作 | 来源 Scenario | 期望 | 层 | 落点 |
| --- | --- | --- | --- | --- | --- |
| 1 | POST type=keyword_topic label=`harness` | 物化轨创建（handler） | 写入行 type=keyword_topic，无话题关联 | handler PG | topic_watch_handler_pg_test.go |
| 2 | 日报 Step7.5 扫描当天文章池 | 含关键字文章聚合为固定话题 | section 固定名 + 每命中文章一条 thread，零 AI | 函数单测 | watch_materialize_keyword_test.go |
| 3 | 扫描 SQL：窗口/归档/择优/上限 | 文章池口径 | pub_date 窗口边界（右开）、archived 排除、AI摘要>Firecrawl>正文>描述、5000 截断 | testcontainer | topic_watch_repository_scan_test.go |
| 4 | 漏网文章（无 tag 无 board 匹配） | 漏网文章可被捞回 | 标题含词即命中（TagIDs NULL 合法降级） | testcontainer + 函数单测 | 同上两文件 |
| 5 | 当天无命中 | 无命中不产空 section | 不追加 section，非报错 | 集成 | watch_materialize_integration_test.go |

### 变体走查

| # | 变体 | 期望答案 | 层 | 落点 |
| --- | --- | --- | --- | --- |
| V1 表达式·DNF 语义 | `ASML\|镓锗 出口` ⇒ (ASML OR 镓锗) AND 出口 | 与提示轨 parseKeywordExpr 同源：空格 AND、`\|` OR、大小写不敏感、字面匹配 | 函数单测 | watch_materialize_keyword_test.go |
| V2 表达式·非法 | 尾部 `\|` / 纯分隔符 | 匹配空集（与 handler 400 校验对齐） | 函数单测 | 同上 |
| V3 文本·只有标题 | Summary 空 | 降级 title-only 匹配 | 函数单测 | 同上 |
| V4 文本·正则元字符/emoji | `C++`、`🇺🇸` | 字面 contains，不解析正则 | 函数单测 | 同上 |
| P1 前置·归档文章 | archived=true 当天发布 | 扫描排除 | testcontainer | topic_watch_repository_scan_test.go |
| P2 前置·窗口边界 | 恰好次日 0 点 / 窗前 1 秒 | 右开区间排除两者 | testcontainer | 同上 |
| P3 前置·量级 | 当天超 5000 篇 | 截断 + 告警，不报错 | 代码常量 | watch_materialize_keyword.go（KeywordArticleScanLimit） |
| C1 组装·字段契约 | section 落库形态 | ClusterIndex 续排 / ClusterTagIDs=`[]` / BestTier=4 / AvgScore=0 / Embedding 空(NULL) / PersistentTopicID=NULL / Confidence=1.0 / RelatedArticleIDs=[自身] / 摘要截 300 runes | 函数单测 | watch_materialize_keyword_test.go |
| I1 幂等·同日重跑 | 重生成日报 | SaveReport 删旧重建，物化 section 重建不重复 | 集成 | watch_materialize_integration_test.go |

## 故事 S2: 一句话物化——向量检索辅助标签聚合成持久话题板块

（锚 Requirement: 一句话轨辅助标签检索 + 一句话轨持久话题联动）

用户建「一句话话题」关注（话题名「AI 编程工具进展」+ 检索句），创建时检索句 embed 一次缓存；每期日报用缓存向量在 board 辅助标签池余弦检索 top-K（阈值 0.55 可配），命中标签解析为当天有文章的 event tag，文章并集聚合为挂**专属持久话题**（manual/active）的板块。

### 主链路（节拍串联）

| 步 | 动作 | 来源 Scenario | 期望 | 层 | 落点 |
| --- | --- | --- | --- | --- | --- |
| 1 | POST type=sentence_topic + query | 创建一句话物化关注 | label=话题名、query=检索句、embedding_cache 可空（惰性补算） | handler PG | topic_watch_handler_pg_test.go |
| 2 | 向量缓存读取 | 检索命中并物化 | 缓存向量直接用（不重 embed） | 函数单测 | watch_materialize_sentence_test.go |
| 3 | 辅助标签池检索 | 阈值过滤 | 相似度 ≥ 阈值才命中，top-K 截断，无 embedding 标签跳过 | 函数单测 + testcontainer | watch_materialize_sentence_test.go / topic_watch_repository_sentence_test.go |
| 4 | 标签→tag→文章解析 | 检索命中并物化 | active event tag、当天窗口、文章并集去重 | testcontainer | topic_watch_repository_sentence_test.go |
| 5 | 专属话题首建/复用 | 物化 section 推进话题延续 | 首建 manual/active/Embedding=Centroid=检索句向量/计数 0；复用同 ID | testcontainer | 同上 + watch_materialize_integration_test.go |
| 6 | SaveReport 生命周期推进 | 物化 section 推进话题延续 | 当期 consecutive_hits/hit_count 推到 1（无 day-1 双计） | 集成 | watch_materialize_integration_test.go |

### 变体走查

| # | 变体 | 期望答案 | 层 | 落点 |
| --- | --- | --- | --- | --- |
| V1 检索·正交标签 | 向量夹角 90° | 相似度 0，不命中（阈值过滤 Scenario） | 函数单测 | watch_materialize_sentence_test.go |
| V2 检索·缓存失效 | PATCH label/query | embedding_cache 置空，下次生成惰性补算并回写 | testcontainer | topic_watch_repository_test.go |
| V3 检索·query 回退 | 创建时只填 label | 检索句=话题名（watchQuerySentence 回退语义） | 函数单测 | watch_materialize_sentence_test.go |
| V4 检索·惰性补算失败 | embed 调用报错 | 返回 nil，该 watch 当期跳过（不阻断日报） | 函数单测 | 同上 |
| V5 检索·全池低相似 | 最高分 < 阈值 | 0 命中，当期无 section | 函数单测 | 同上 |
| P1 前置·标签无 embedding | 池成员向量 NULL | 跳过（合法降级） | testcontainer | topic_watch_repository_sentence_test.go |
| P2 前置·tag 非 active | inactive tag 关联命中标签 | 解析排除 | testcontainer | 同上 |
| P3 前置·当天无文章 | 标签命中但文章窗口空 | 当期无 section（话题自然衰减） | testcontainer | watch_materialize_integration_test.go |
| C1 组装·归属字段 | section 落库形态 | lane_tier=watch_sentence / PersistentTopicID=专属话题 / confidence=manual / distance=0 / TopicStatusAtReport=active / 无 embedding | 集成 | watch_materialize_integration_test.go |
| C2 组装·Centroid 双写 | 话题创建 | Embedding 与 Centroid 都=检索句向量（lane 锚用 Centroid） | testcontainer | topic_watch_repository_sentence_test.go |
| L1 生命周期·跨期延续 | day1→day2 连续物化 | consecutive_hits 1→2，hit_count 同步 | 集成 | watch_materialize_integration_test.go |
| L2 生命周期·GORM 零值陷阱 | 首建计数 | model default:1 与零值 skip 双坑：显式 UPDATE 种 0（day-1 =1 非 2） | 集成 | 同上 |
| L3 生命周期·不进质心刷新 | watch_sentence 话题 | 不在 touched 集（锚点=用户检索句，质心不漂移） | 代码 | daily_report_assignment.go D5 注释锚 |

## 故事 S3: 管线边界与提示轨互斥

（锚 Requirement: 物化 section 管线边界 + 物化轨不参与命中提示 + 流水线物化失败降级）

物化 section 是叠加板块不是聚类公民：keyword 板块不被自动归属/L3 收编、不进关系图、不进同日合并；提示轨（label AI 扫描 / keyword 文本回扫）对物化板块和物化轨关注双向不可见；任一关注物化失败降级跳过不阻断日报。

### 主链路（节拍串联）

| 步 | 动作 | 来源 Scenario | 期望 | 层 | 落点 |
| --- | --- | --- | --- | --- | --- |
| 1 | SaveReport assignAndUpdateTopics | 不参与同日合并（keyword 板块） | watch_keyword 排除出 planner（无候选收编）；board 话题数= sentence 话题+常规 L3 候选 | 集成 | watch_materialize_integration_test.go |
| 2 | RebuildBoardRelations | （同上） | 相似度边 + 身份边 SQL 均 `lane_tier NOT LIKE 'watch_%'` | 代码 | daily_report_matching.go（两处过滤） |
| 3 | EvaluateWatchHits 分流 | 物化轨无命中记录 | keyword_topic/sentence_topic 不进 label/keyword 轨 | 集成 | watch_materialize_integration_test.go |
| 4 | 提示轨输入过滤 | （同上） | keyword 提示 SQL 过滤 watch_* section；label prompt 构建剔除 | 代码 | topic_watch_repository.go watchSectionTextSQL / daily_report_watch.go hintSections |
| 5 | report 级计数 | 计数不重复 | article_count/event_tag_count/cluster_count 保持聚类口径（Step6 已定稿，物化不改） | 代码 | daily_report_orchestrator.go Step7.5 |

### 变体走查

| # | 变体 | 期望答案 | 层 | 落点 |
| --- | --- | --- | --- | --- |
| F1 失败·单轨挂 | sentence 检索报错 | 该 watch 跳过（log warn），日报 status 正常 completed | 集成 + 代码 | orchestrator Step7.5 降级分支 |
| F2 失败·文章双处可见 | 同文章在常规+物化 section | 共存（用户拍板）；report 计数不重复累计 | spec | watch-materialized-topic/spec.md 计数 Scenario |
| E1 落库·空 embedding | watch section/thread 无向量 | Omit 路径写 NULL（pgvector 拒绝空串 ''）；ID 回填到原 slice | 集成 | watch_materialize_integration_test.go（隐式）+ daily_report_repository.go |
| M1 migration·旧形状 | 20260824_0002 两值 CHECK + VARCHAR(10) | 扩宽 16、四值 CHECK、三新列、FK ON DELETE SET NULL、幂等重跑 | testcontainer | watch_materialized_migration_test.go |
| M2 migration·FK 行为 | 删话题行 | watch.persistent_topic_id 置 NULL（不删 watch） | testcontainer | 同上 |

## 故事 S4: 删除联动与主权

（锚 Requirement: 删除关注联动）

keyword_topic 删除=仅停止后续物化（历史板块保留）；sentence_topic 删除需显式确认（confirm_archive_topic=true），确认后软归档专属话题（历史 section 归属不变），不确认 400 且错误信息含话题名。

### 主链路（节拍串联）

| 步 | 动作 | 来源 Scenario | 期望 | 层 | 落点 |
| --- | --- | --- | --- | --- | --- |
| 1 | DELETE sentence_topic 无确认 | 删除一句话轨确认归档 | 400，错误文案含话题名；watch 存活 | handler PG | topic_watch_handler_pg_test.go |
| 2 | DELETE ?confirm_archive_topic=true | （同上） | 话题 status=archived（软归档走 UpdateTopic）+ watch 删除 | handler PG | 同上 |
| 3 | DELETE keyword_topic | 删除关键字轨保留历史 | 直接 200，无确认门槛 | handler PG | 同上 |

### 变体走查

| # | 变体 | 期望答案 | 层 | 落点 |
| --- | --- | --- | --- | --- |
| V1 前置·话题未建 | sentence watch 从未物化命中 | PersistentTopicID=NULL：确认后直接删（无话题可归档） | handler PG | 同上 |
| V2 归档·历史保留 | 删除后看历史日报 | 物化 section 保留且 persistent_topic_id 不变（快照不可变红线） | spec | watch-materialized-topic/spec.md |

## 故事 S5: 创建入口与管理 UI（含旧提示轨退役）

（锚 Requirement: 关注标记管理 API（delta）+ 前端四类型 → 双物化轨收尾）

新建对话框仅物化轨双选（默认关键字话题，关键字三件套语法提示/解析预览/物化说明沿袭；一句话轨话题名+检索句）；旧提示轨（label/keyword）创建入口退役隐藏，存量关注四类型徽标继续展示可暂停/删除；日报板块 watch 角标 + 物化板块排分区末尾。

### 主链路（节拍串联）

| 步 | 动作 | 来源 Scenario | 期望 | 层 | 落点 |
| --- | --- | --- | --- | --- | --- |
| 1 | 打开新建对话框 | （UI 契约） | 仅 keyword_topic/sentence_topic 两卡；label/keyword 卡不渲染 | Vitest | TopicWatchCreateDialog.test.ts |
| 2 | sentence 态提交（带/不带检索句） | （UI 契约） | createWatch(boardId, 名, 'sentence_topic', query?)；空 query 不传 | Vitest | 同上 |
| 3 | API normalizer | （UI 契约） | query/persistentTopicId 序列化；四类型 type 归一；deleteWatch 确认参数 | Vitest | topicWatches.test.ts |
| 4 | 管理面板删除确认 | 删除联动（前端侧） | sentence 三态确认文案（归档明示）；deleteWatch(id, type==='sentence_topic') | Vitest | WatchManagePanel.test.ts |
| 5 | 日报物化板块渲染 | （UI 契约） | SectionWatchBadge 方形角标；watch_* 排分区末尾（tier/score 之前） | Vitest | dailyReportMagazine.test.ts |

### 变体走查

| # | 变体 | 期望答案 | 层 | 落点 |
| --- | --- | --- | --- | --- |
| V1 输入·表达式无效 | 尾部 `\|` / 纯分隔符 | 预览红字 + 提交禁用（与后端 400 对齐） | Vitest | TopicWatchCreateDialog.test.ts |
| V2 输入·sentence 话题名空 | 误提交 | 错误提示可见、输入保留、不关闭 | Vitest | 同上 |
| V3 输入·检索句回退 | 只填话题名 | query 传 undefined（后端回退 label） | Vitest | 同上 |
| U1 退役·存量可见 | 已有 label/keyword 关注 | 管理面板 TYPE_META 四类型徽标照常展示/暂停/删除 | Vitest | WatchManagePanel.test.ts |
| U2 退役·回扫 banner | keyword 创建路径不可达 | banner 死代码已删（navigateDaily 联动移除） | 代码 | WatchManagePanel.vue |
| U3 重开·状态重置 | 关闭再开 | 回默认 keyword_topic 态、输入清空 | Vitest | TopicWatchCreateDialog.test.ts |

## 白盒用例锚点（复杂档红旗对照）

| 锚点 | 断言判据 | 落点 |
| --- | --- | --- |
| DNF 匹配语义与提示轨同源 | matchKeywordArticles 复用 parseKeywordExpr/matchKeywordGroups，两轨语义零漂移 | watch_materialize_keyword_test.go |
| SaveReport 排除/放行边界 | plannerSections 过滤 watch_keyword；watch_sentence 进 hitTopicIDs 但不进 touched | watch_materialize_integration_test.go + daily_report_assignment.go |
| 提示轨互斥双向 | watch 类型分流 continue + watchSectionTextSQL `NOT LIKE 'watch_%%'` + label hintSections 剔除 | watch_materialize_integration_test.go |
| embedding 检索阈值与 top-K | 阈值边界（≥ vs <）、top-K 截断、排序（分数 desc, id asc） | watch_materialize_sentence_test.go |
| GORM 零值陷阱 | default:1 tag + Create 零值 skip ⇒ 显式 UPDATE 种 0 | watch_materialize_integration_test.go L2 |
| pgvector 空串拒绝 | 空 embedding 走 Omit 写 NULL + ID 回填 | watch_materialize_integration_test.go E1 |
