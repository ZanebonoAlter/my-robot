# test-cases: watch-keyword-and-quickadd

> **故事总纲**：让"我在追踪"从一个藏在日报顶栏、只会语义联想、第二天才知道结果的隐秘功能，变成**看得见、点得到、立刻见效**的正经能力。
>
> 五个故事：S1 盯词（keyword 双轨判定）→ S2 立刻见效（即时匹配）→ S3 关注的家（版块级管理面板）→ S4 追踪是导航，不是第二篇日报 → S5 暗线（老用户无感 + 只读叠加不变量）。
>
> **层选择总则**：表达式解析/匹配 = 函数单测（`keyword_match_unit_test.go`，无 DB）；SQL/约束/迁移/去重 = testcontainer PG；HTTP 契约 = handler 测试（内存 SQLite，topicgraph/handler 允许）；组件行为 = Vitest；**完整交互故事 = opencli 端到端**（S1、S3 各至少一条主链路落点，见各故事主链路表末行）。

## 故事 S1: 用户盯一个具体词，下一期日报把它抓出来

（锚 Requirement: 关注标记命中判定（label 走 AI / keyword 走文本）+ 关注标记实体模型 + 关注标记日报顶部独立栏位）

用户打开版块关注管理面板，新建类型选「关注关键字」，输入 `ASML|镓锗 出口` 提交（解析预览 chips 实时确认语义）；夜里日报生成，keyword 分支扫 threads 标题+摘要纯文本命中；明早在日报时间线记录下看到 `# ASML` 预告，打开详情后在「追踪关键字」优先分区点索引定位命中 section。

### 主链路（节拍串联）

| 步 | 动作 | 来源 Scenario | 期望 | 层 | 落点（测试/人工） |
| --- | --- | --- | --- | --- | --- |
| 1 | POST type=keyword label=`ASML\|镓锗 出口` | 创建 keyword 类关注 / 创建 keyword 关注 | 写入行 type=keyword status=active | handler | topic_watch_handler_test.go |
| 2 | 日报生成末尾 EvaluateWatchHits | label 类走 AI 单信号 | label 类 watch 仍走批量 AI（回归不变） | service 单测 | daily_report_watch_test.go |
| 3 | keyword 分支扫 threads 文本 | keyword 类走文本匹配 | 写 hit，reason=「含关键字『ASML』」，chat 调用=0 | service 单测 | daily_report_watch_test.go |
| 4 | 多词语法判定 | keyword 多词 AND/OR / 大小写不敏感 | AND 全含才命中 / OR 任一 / asml≡ASML | 函数单测 | keyword_match_unit_test.go |
| 5 | 两类命中合并写表 | 两类命中合并写表 | 同表，复合唯一索引去重 | testcontainer | daily_report_watch_integration_test.go |
| 6 | 时间线预告并打开详情 | 时间线命中预告 / 详情优先索引与定位 / 无命中隐藏 | 记录下显示 #/✦ tag；详情追踪分区在关心话题前，可定位且不重复 reason/正文 | 组件 | BoardDailyReportTimeline.test.ts / DailyReportWatchIndex.test.ts |
| 7 | 完整旅程走查 | （S1 全部节拍） | 面板建 keyword → 夜里生成 → 早上看到命中 | opencli | V.8 ②③⑤（留痕） |

### 变体走查

| # | 变体（组/条目） | 期望答案 | 层 | 落点 |
| --- | --- | --- | --- | --- |
| V1 输入·空串 | type=keyword label="" | 400 拒绝（「关键字表达式无效」） | handler | topic_watch_handler_test.go |
| V2 输入·纯空白 | label="　 \t"（全角+tab） | 400 拒绝 | handler | topic_watch_handler_test.go |
| V3 输入·纯分隔符 | "ASML\|" / "\| \| / 首尾\|" | 400 拒绝（解析后无有效词组） | handler | topic_watch_handler_test.go |
| V4 输入·单词 | "ASML" | 正常创建，OR/AND 退化为单词匹配 | 函数单测 | keyword_match_unit_test.go |
| V5 输入·大小写 | 表达式 "asml" vs 文本含 "ASML" | 命中（双向不敏感） | 函数单测 | keyword_match_unit_test.go |
| V6 输入·正则元字符 | `C++`、`.*`、`（）` | 按字面文本处理，不解析为正则，不报错 | 函数单测 | keyword_match_unit_test.go |
| V7 输入·超长表达式 | 数百字符多词 | 创建不设硬上限；顶部栏超长 label 截断省略展示 | 组件 | DailyReportWatchBar.test.ts |
| V8 输入·emoji/引号 | "🇺🇸 制裁" | 字面匹配，正常 | 函数单测 | keyword_match_unit_test.go |
| P1 前置·无 threads | section 无 thread（空聚类） | 该 section 不命中，不报错 | 函数单测 | keyword_match_unit_test.go |
| P2 前置·threads 缺 summary | 只有 title | 拼接文本= title，照常匹配 | 函数单测 | keyword_match_unit_test.go |
| P3 前置·board 无关注 | 全部 paused | 判定跳过，无 AI 调用 | service 单测 | daily_report_watch_test.go |
| I1 幂等·同期重跑 | 同日重跑日报（覆盖式重建） | hits 复合唯一索引去重，无重复行 | testcontainer | daily_report_watch_integration_test.go |
| U1 可用性·误输入反馈 | 空 label / 无效表达式提交 | 对话框提示错误，已输内容保留，不关闭 | 组件 | TopicWatchCreateDialog.test.ts |
| U2 可用性·错误态 | createWatch API 5xx | toast 报错，对话框不关，内容保留 | 组件 | TopicWatchCreateDialog.test.ts |
| U3 可用性·重复提交 | 双击提交按钮 | 提交中按钮禁用，只建一条 | 组件 | TopicWatchCreateDialog.test.ts |
| 时间窗口 | —— | 不适用（S1 判定对象=当期全部 section，无窗口；划除留痕） | — | — |

## 故事 S2: 建完立刻见效（即时匹配）

（锚 Requirement: keyword 类即时匹配）

用户建 keyword 关注后不用等夜里日报——系统立刻扫近 14 天 section 文本，命中马上写表，刷新就能在对应日期日报的顶部栏看到。即时匹配失败不阻断建关注（吞错记日志）；label 类不即时匹配。

### 主链路（节拍串联）

| 步 | 动作 | 来源 Scenario | 期望 | 层 | 落点 |
| --- | --- | --- | --- | --- | --- |
| 1 | 建 keyword 关注（含历史数据 board） | 建关注后立即命中历史 section | 立即写 hit（report_id=section 所属，period_date=section 日期），不等下一期 | testcontainer | daily_report_watch_integration_test.go |
| 2 | 刷新打开 14 天内某期日报 | （同上） | 时间线记录下已有 keyword tag，详情索引可定位命中 section | opencli | V.9（留痕） |
| 3 | 夜里日报生成再次匹配同 section | 即时与日报匹配幂等去重 | 无重复行（OnConflict DoNothing） | testcontainer | daily_report_watch_integration_test.go |
| 4 | 即时匹配内部报错 | 即时匹配失败不阻断建关注 | watch 建成功，失败仅 log | service 单测 | daily_report_watch_test.go |
| 5 | 建 label 类关注 | label 类不即时匹配 | MatchKeywordInstant 零调用 | service 单测 | daily_report_watch_test.go |

### 变体走查

| # | 变体 | 期望答案 | 层 | 落点 |
| --- | --- | --- | --- | --- |
| T1 时间·窗口边界 | 第 14 天当天的 section / 第 15 天的 | 含当天往前 14 个 period_date：第 14 天命中、第 15 天不扫 | service 单测 | daily_report_watch_test.go |
| T2 时间·空窗口 | 新 board 无任何日报 | 零命中，不报错，watch 建成功 | service 单测 | daily_report_watch_test.go |
| T3 时间·跨窗口命中回填 | keyword 曾出现在 20 天前 | 不回填（窗口外），仅当期起增量 | service 单测 | daily_report_watch_test.go |
| P1 前置·越界 boardID | board 不存在 | 零 section 可扫，正常返回（watch 挂在该 board 下本身由创建校验） | service 单测 | daily_report_watch_test.go |
| I1 幂等·重复建同表达式 watch | 两个 watch 同 keyword | 各自独立 hit 行（watch_id 不同），互不干扰 | testcontainer | daily_report_watch_integration_test.go |
| I2 幂等·即时失败后重试 | 即时匹配失败，次日日报生成 | 日报匹配自然补上，无重复 | testcontainer | daily_report_watch_integration_test.go |
| 并发 | —— | 不适用（不声称线程安全；单用户单写路径；划除留痕） | — | — |

## 故事 S3: 关注的家——版块级管理面板

（锚 Requirement: 关注管理面板（版块级中枢））

用户在版块工作台选中版块后，tab 栏右端始终有「我在追踪 (3)」（板块内容/话题总览/日报/文章/数据增强五个 tab 下都在），点开管理面板：新建（类型双选 + 解析预览）、看全部关注（带类型标识）、暂停吵的、删掉不要的；keyword 建完立刻看到回扫反馈（命中 N 条可点）。入口位置与职责不变，图标和文案保持单行。

### 主链路（节拍串联）

| 步 | 动作 | 来源 Scenario | 期望 | 层 | 落点 |
| --- | --- | --- | --- | --- | --- |
| 1 | 任一内容 tab 下看 tab 栏右端 | 入口常驻版块内容区 | 「我在追踪 (N)」chip 可见，N=关注总数（active+paused） | 组件 | TagsPage.test.ts |
| 2 | 面板内切 keyword 提交表达式 | 从管理面板新建关键字关注 | 建 type=keyword + 回扫反馈（近 14 天命中 N 条可点） | 组件 | WatchManagePanel.test.ts |
| 3 | 面板列表暂停 / 删除关注 | 管理面板内暂停或删除关注 | paused 下期不判定；删除连命中记录一起清 | 组件 | WatchManagePanel.test.ts |
| 4 | 检查 tab 栏入口 | 入口常驻版块内容区 | 入口位置/职责不变，图标和文案不换行 | 组件 | TagsPage.test.ts |
| 5 | 完整旅程走查 | （S3 全部节拍） | 从面板建 keyword → 回扫反馈 → 翻历史日报看到命中 → 面板暂停它 | opencli | V.8 ①⑥（留痕） |

### 变体走查

| # | 变体 | 期望答案 | 层 | 落点 |
| --- | --- | --- | --- | --- |
| P1 前置·空关注列表 | 新版块无任何 watch | 入口 N=0；面板空态提示「还没有关注」+ 新建引导 | 组件 | WatchManagePanel.test.ts |
| P2 前置·关注多 | 20+ 条 watch | 列表可滚动，类型标识/状态标识齐全不混行 | 组件 | WatchManagePanel.test.ts |
| U1 可用性·误输入反馈 | 空 label / 无效表达式提交 | 同 S1·U1（提示+内容保留，对话框不关） | 组件 | TopicWatchCreateDialog.test.ts |
| U2 可用性·加载态 | 列表加载中 / 提交中 | 列表骨架屏禁操作；提交按钮 loading 禁用 | 组件 | WatchManagePanel.test.ts |
| U3 可用性·错误态 | 列表 API 5xx / 操作 5xx | 面板内错误提示可重试，不白屏 | 组件 | WatchManagePanel.test.ts |
| U4 可用性·删除二次确认 | 点删除 | 确认弹层（防误删连带命中记录），取消不删 | 组件 | WatchManagePanel.test.ts |
| 输入/时间/幂等 | —— | 不适用（表达式输入与窗口已由 S1/S2 覆盖；划除留痕） | — | — |

## 故事 S4: 追踪是导航，不是第二篇日报

（锚 Requirement: 关注命中在日报时间线与详情中的呈现）

用户浏览 TagsPage 日报时间线，日期顺序不变；有 active watch 命中的记录在日期下显示最多两个紧凑 tag 与 `+N`。打开日报时，「追踪关键字」「追踪话题」置于「关心的话题」之前，只显示可点击单行索引；点索引滚到既有 section。paused 或已删除 watch 的历史 hit 不得泄露到时间线或详情。

| 步 | 动作 | 期望 | 层 | 落点 |
| --- | --- | --- | --- | --- |
| 1 | 拉日报列表 | 一次列表响应含每日报告 active watch 摘要；同 watch 多 section 去重，日期顺序不变 | repository / handler | daily_report_repository_test.go |
| 2 | 看时间线记录 | 最多 #/✦ 两个 tag，余项 +N；不逐报告请求 | 组件 | BoardDailyReportTimeline.test.ts |
| 3 | 打开详情并点索引 | keyword/topic 分区在关心话题前，定位 `report-section-{id}`，无正文/reason 堆叠 | 组件 | DailyReportWatchIndex.test.ts / DailyReportTopicSection.test.ts |
| 4 | 暂停或删除 watch 后刷新 | 摘要与详情均无该 watch | repository / handler | topic_watch_repository_test.go |
| 5 | 真实交互 | 时间线 tag → 详情索引 → 目标 section；chip 无换行 | opencli + Luna | V.9 |

### 变体走查

| 变体 | 期望答案 | 层 | 落点 |
| --- | --- | --- | --- |
| 无命中日报 | 时间线无 tag，详情两个追踪分区均隐藏 | 组件 | BoardDailyReportTimeline.test.ts / DailyReportWatchIndex.test.ts |
| 同 watch 命中多个 section | 时间线摘要只出现一次；详情仍保留多个可定位索引 | repository / 组件 | daily_report_repository_test.go / DailyReportWatchIndex.test.ts |
| 超过两个 watch | 时间线显示前两项与 +N，不换行撑破记录 | 组件 | BoardDailyReportTimeline.test.ts |
| 超长 watch label | 单行省略，不挤压 icon/日期 | 组件 | BoardDailyReportTimeline.test.ts / TagsPage.test.ts |

## 故事 S5: 暗线——老用户无感 + 只读叠加不变量

（锚 Requirement: 关注标记实体模型（历史兼容 scenarios）+ 继承的关注标记与持久话题隔离）

老用户已有的 watch 全部默认 label，行为一个字不变；keyword 命中是只读叠加，不碰 section 归属、不推话题生命周期。

### 主链路（节拍串联）

| 步 | 动作 | 来源 Scenario | 期望 | 层 | 落点 |
| --- | --- | --- | --- | --- | --- |
| 1 | 迁移执行（有历史行） | 历史 watch 默认类型 | 历史行 type=label，不报错 | testcontainer | watch_type_column_test.go |
| 2 | 写入非法 type/status | 状态约束 / 类型约束 | CHECK 拒绝 | testcontainer | topic_watch_repository_test.go |
| 3 | keyword 命中某 section | （继承：命中不改归属 / 命中不推进持久话题生命周期） | persistent_topic_id、consecutive_hits 均不变 | testcontainer | daily_report_watch_integration_test.go |
| 4 | 暂停/删除关注 | 暂停的关注不参与判定 / 删除关注级联清理命中 | paused 跳过判定；删除连 hits 一起清 | testcontainer | daily_report_watch_test.go / watch_hit_fk_cascade_test.go |

### 变体走查

| # | 变体 | 期望答案 | 层 | 落点 |
| --- | --- | --- | --- | --- |
| I1 幂等·迁移重复执行 | 二次跑迁移 | 幂等无错（IF NOT EXISTS 模式） | testcontainer | watch_type_column_test.go |
| I2 幂等·迁移部分失败重试 | CHECK 加了一半中断重跑 | 重跑补齐，终态一致 | testcontainer | watch_type_column_test.go |
| 输入·JSON 向后兼容 | 老客户端不传 type | 缺省 label，响应含 type 可选字段 | handler | topic_watch_handler_test.go |
| 时间/并发/可用性 | —— | 不适用（纯 DB 兼容层；划除留痕） | — | — |

## 继承与调整（问句⓪：本 change 含 MODIFIED×3 + REMOVED×1，必填）

> 旧资产反查：`bash scripts/test-assets.sh topic-watch` → 主 specs 基线（2026-08-24 补回）+ 既有测试文件。旧测试可能仍断言旧契约（全 watch 走 AI），跑绿 ≠ 对。

| 旧 Scenario（基线 spec） | 处置 | 旧测试文件 | 动作 |
| --- | --- | --- | --- |
| 创建关注标记（实体模型旧版） | 改语义（type 化；label 默认路径行为不变） | topic_watch_handler_test.go / topic_watch_repository_test.go | 照跑（回归网）+ 补 type 断言 |
| 状态约束 | 继承 | topic_watch_repository_test.go | 照跑 |
| 命中记录（AI 命中判定，REMOVED） | 改语义（label 类保留该路径，keyword 类走文本） | daily_report_watch_test.go | 改 fixture：watch 构造补 type=label，断言不变 |
| 不走双重确认（REMOVED requirement 内） | 改语义（限定 label 类） | daily_report_watch_test.go | 照跑（fixture 标 type=label） |
| 批量单次请求（REMOVED requirement 内） | 继承（label 类批量不变；keyword 类零 AI 天然满足） | daily_report_watch_test.go | 照跑 |
| 命中不改归属（隔离） | 继承+扩展（keyword 命中同样零副作用） | daily_report_watch_integration_test.go | 照跑 + keyword fixture 复测 |
| 命中不推进持久话题生命周期（隔离） | 继承+扩展（同上） | daily_report_watch_integration_test.go | 照跑 + keyword fixture 复测 |
| 命中分组展示（顶部栏位旧版） | 改语义（两类分组 + 视觉区分） | DailyReportWatchBar.test.ts / topicWatchGrouping.test.ts | 改断言（type 分支、机械理由文案） |
| 无命中空态（顶部栏位旧版） | 继承 | DailyReportWatchBar.test.ts | 照跑 |
| 暂停的关注不参与判定（管理 API 旧版） | 继承（两类皆然） | daily_report_watch_test.go | 照跑 + keyword fixture |
| 删除关注级联清理命中（管理 API 旧版） | 继承 | watch_hit_fk_cascade_test.go | 照跑 |

## 白盒附加（复杂档：keyword 表达式解析器）

### 分支表

| # | 条件/分支 | 输入 | 期望 | 测试用例名 |
| --- | --- | --- | --- | --- |
| B1 | 空串 | "" | 空 OR 组列表 | TestParseKeywordExpr_Empty |
| B2 | 纯空白（全角/tab/半角混合） | "　\t " | 空 | TestParseKeywordExpr_WhitespaceOnly |
| B3 | 纯分隔符（首/尾/连续） | "ASML\|" / "\| 出口" | 前者空、后者有效组[出口] | TestParseKeywordExpr_Delimiters |
| B4 | 单词（无分隔符） | "ASML" | [[ASML]] | TestParseKeywordExpr_Single |
| B5 | 空格 AND | "出口 限制" | [[出口,限制]] | TestParseKeywordExpr_And |
| B6 | `\|` OR | "ASML\|镓锗" | [[ASML],[镓锗]] | TestParseKeywordExpr_Or |
| B7 | 混合（先 `\|` 后空格） | "ASML\|镓锗 出口" | [[ASML,出口],[镓锗,出口]] | TestParseKeywordExpr_Mixed |
| B8 | matchKeyword：无 threads | section.Threads=nil | 不命中 | TestMatchKeywordSections_NoThreads |
| B9 | matchKeyword：部分词命中（AND 缺一） | 含"出口"不含"限制" | 不命中 | TestMatchKeywordSections_AndMissing |
| B10 | matchKeyword：OR 组任一 | 含"镓锗" | 命中，命中词=镓锗 | TestMatchKeywordSections_OrAny |

### 边界值清单

| 变量 | 边界值 | 期望 | 测试用例名 |
| --- | --- | --- | --- |
| 即时窗口天数 | 14（含当天）/ 15 | 含 / 不含 | TestMatchKeywordInstant_Window |
| 表达式长度 | 0 / 1 / 数百字符 | 拒绝 / 正常 / 不设上限 | TestParseKeywordExpr_* |
| threads 数量 | 0 / 1 / 多条 | 降级 / 正常 / 全拼接 | TestMatchKeywordSections_* |

### 不适用划除（留痕）

- 并发变体：不适用（单用户单写路径，不声称线程安全）。
- 表达式长度硬上限：不适用（v1 不设限，UI 层截断展示；参数膨胀留给后续）。

## 效果核对（问句④）

- **触发原因**：keyword 命中依赖 threads title/summary 的文本覆盖——若当期摘要不含该词（AI 摘要漏写实体名），keyword 会漏报，测试绿灯测不出来。
- **核对方法**：实现后建 3-5 个真实关键字 watch（如 ASML / 镓锗 / 霍尔木兹），跑即时匹配，人工翻近 14 天日报 threads 对照命中率与漏报原因（词不在文本 vs 摘要没写）。
- **量化结果**：待填（实现后回填：命中 N 条 / 漏报 M 条 / 漏报归因）。
- **结论**：待填（达标交付 ｜ 瓶颈在 threads 摘要覆盖 ｜ 需调整匹配范围到 articles）。
