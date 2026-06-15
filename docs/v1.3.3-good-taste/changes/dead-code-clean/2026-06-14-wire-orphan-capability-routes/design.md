## Context

Syntopica 的 AI 能力路由（`airouter`）按 capability 字符串（如 `topic_tagging`、`embedding`）从 `ai_routes` 表加载 provider 列表，支持有序故障转移。前端"全局设置 → 能力路由"面板允许用户为每个 capability 配置 provider 顺序、并发上限、温度。

当前存在严重的"配置与代码脱节"：

```
前端面板配置 (5 条)        后端代码实际调用 (3 个 capability 常量)
─────────────────────     ────────────────────────────────────
summary（文章总结）     →  0 处调用（孤儿）
article_completion       →  1 处 (summarizeContent) ← 名义是"补全"，实际做总结
topic_tagging            →  7 处（含日报生成的 4 处）← 日报寄生于此
digest_polish（日报润色）→  0 处调用（孤儿）
embedding                →  N 处
```

`AISummaryResponse` 结构体保留 `OneSentence` / `KeyPoints` / `Takeaways` / `Tags` 四个结构化字段，但 `summarizeContent` 实际只使用 `Markdown`，其余字段从未被填充——结构化提取职责早已由 `topic_tagging` 承担。

## Goals / Non-Goals

**Goals:**
- 让 `summary` 路由真正驱动文章自动总结（替换 `article_completion`）
- 让 `digest_polish` 路由真正驱动日报生成，使日报可独立配置 provider / 并发 / 温度，与标签提取解耦
- 移除 `article_completion` 这个与产品语义脱节的孤儿 capability
- 清理 `AISummaryResponse` 从未被使用的结构化字段及其关联死代码

**Non-Goals:**
- 不重命名 `completion_*` 数据库列（`completion_attempts`、`completion_error`、`max_completion_retries`、`completion_on_refresh`）。这些列功能正常，仅名字带历史遗留的 "completion"；重命名涉及 DB 迁移 + 全栈改名，属另一个 scope。
- 不改动 `open_notebook` capability（独立功能，前端无标签，不在本次范围）。
- 不重构 `AIService.SummarizeArticle` 的 fallback 路径去留（router 已有多 provider 故障转移；裸 AIService fallback 是否保留单独评估，本次仅保证不破坏）。
- 不引入文章"可读正文 + 结构化摘要"两步流水线（用户已确认标签提取承担结构化职责，摘要无需重复）。

## Decisions

### 决策 1：`summary` 采用轻量换标签（选项 A），不做职责分离

`summarizeContent` 仅将 capability 从 `CapabilityArticleCompletion` 改为 `CapabilitySummary`，prompt 与产出（polished Markdown）保持不变。

**为何不选职责分离（选项 B）**：用户确认结构化摘要无必要——标签提取（`topic_tagging`）已是结构化的大头。再做"正文补全 + 结构化摘要"两步流水线属于无产品价值的复杂化。

### 决策 2：`digest_polish` 接管日报生成的全部 LLM 调用

新增 `CapabilityDigestPolish Capability = "digest_polish"`。将 `daily_report_llm.go`（GenerateHighlights / GenerateNarrative / 第三个调用）与 `daily_report_cluster.go`（聚类）共 4 处 `Capability: CapabilityTopicTagging` 迁移到 `CapabilityDigestPolish`。

`defaultConcurrency` 增加 `CapabilityDigestPolish: 2`（与日报定时任务的串行特性匹配；如需更高可通过路由 `MaxConcurrency` 覆盖）。

**为何从 `topic_tagging` 迁出**：当前 7 处 `topic_tagging` 调用中，4 处属于日报生成，与标签提取是不同业务。混用导致日报无法独立换模型、抢占用同一并发配额（默认 3）。

### 决策 3：`article_completion` 彻底废弃

删除 `CapabilityArticleCompletion` 常量与 `defaultConcurrency` 对应条目。前端移除 `routeLabels` 与 `capabilityOrder` 中的 `article_completion`。

**数据库既有行处理**：不写迁移脚本主动删除 `ai_routes` 表中 `capability='article_completion'` 的行——它们不影响运行（无代码加载该 capability）。在 `design` 此处记录手动清理 SQL 供运维按需执行：
```sql
DELETE FROM ai_route_providers WHERE route_id IN (SELECT id FROM ai_routes WHERE capability = 'article_completion');
DELETE FROM ai_routes WHERE capability = 'article_completion';
```
理由：避免本次 change 引入破坏性 DDL；残留行零影响。

### 决策 4：`AISummaryResponse` 瘦身为仅 `Markdown`

移除 `OneSentence` / `KeyPoints` / `Takeaways` / `Tags`。`summarizeContent` 直接返回 `string`（即 `result.Content`），不再包 `AISummaryResponse`。连带移除：
- `formatAISummary` 中 `Markdown` 为空时的结构化拼接分支（永远不可达，因 `ParseSummaryMarkdown` 必填 `Markdown`）；`formatAISummary` 简化为直接 trim 返回，或内联
- `ParseSummaryMarkdown`（仅服务上述死分支）
- `markdownToPlainText`（仅服务 `OneSentence` 计算）

**替代方案考虑**：保留 `AISummaryResponse` 仅作类型别名——否决，引入无意义的间接层。

## Risks / Trade-offs

- **[BREAKING] 删除 `CapabilityArticleCompletion` 常量** → 任何外部引用编译失败。Mitigation：全仓库 grep 确认引用点（已知 `content_completion_service.go`、`router_test.go`、`store_test.go`、`content_completion_service_test.go`），全部随本次修改。
- **用户已配置 `article_completion` 路由** → 升级后面板不再显示该路由，用户需在 `summary` 下重新配置 provider。Mitigation：升级说明提示；旧 `ai_routes` 行不报错。
- **日报改路由后 provider 未配置** → 日报生成会因 `digest_polish` 无 provider 而失败。Mitigation：`LoadRouteWithProviders` 失败的错误信息已存在；前端面板 `digest_polish` 标签此前就有，用户大概率已配置。tasks 中增加"文档/提示"。
- **`AIService.SummarizeArticle` fallback 路径**仍引用被删的 `AISummaryResponse` 字段 → 需同步简化该函数（它内部调用 `ParseSummaryMarkdown`）。Mitigation：tasks 明确覆盖。

## Migration Plan

1. 后端：新增常量 → 迁移调用点 → 删除旧常量与死代码（顺序保证编译始终通过）。
2. 前端：移除 `article_completion` 面板项。
3. 部署后：用户在 `digest_polish` 路由确认 provider 配置（若之前配在 `topic_tagging` 上，日报用的 provider 需迁移过去）。
4. 回滚：纯代码改动，git revert 即可；无不可逆 DB 变更。

## Open Questions

- 日报 `digest_polish` 默认并发 `2` 是否足够？定时任务串行执行下应无瓶颈，但若未来日报并行多看板需上调（可通过路由 `MaxConcurrency` 覆盖，无需改代码）。
