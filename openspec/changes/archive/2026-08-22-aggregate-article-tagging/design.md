# Design: aggregate-article-tagging

## Context

打标签管线现状（`backend-go/internal/tagmanagement/service/core/`）：

- `article_tagger.go` 的 `buildArticleSummary` 取 AIContentSummary（优先）/ FirecrawlContent / Content，截断到 2000 runes
- `extractor_enhanced.go` 双分支并行 LLM 提取（event/person 一个调用 + keyword 一个调用），`mergeExtractedTags` 合并后硬顶 5 个，`maxArticleTags=5` 再顶一次
- event 标签入库后自动 enqueue embedding（本地服务）
- 摘要生成在 `reader/service/content_completion_service.go:378` `summarizeContent`，输入是 firecrawl 全文（几万字），maxTokens 16000——摘要调用看得到全文，打标调用看不到摘要全文

数据佐证（2026-08 库）：阮一峰 feed 摘要均长 6365 字，AI 摘要开头"导读"即全期栏目概览但 2000 字截断只覆盖第一个栏目；该 feed 产出的 27 个 event + 5 个 person 标签 100% 只被 1 篇文章引用。博客园排行榜（3 个 feed）、少数派同属聚合型（摘要均长 4300-8700 字、13-22 个 markdown 标题）。

## Goals / Non-Goals

**Goals:**

- 聚合型文章打标不漏栏目：摘要里每个实质栏目都有机会产出标签
- 消灭"捏造合并标签"（两条无关新闻捏成一个 event）
- 聚合判定零用户配置：利用摘要调用对全文的既有视野顺带产出形态标记
- 标签有 score 区分度：聚合路径按栏目位置分层（0.9/0.7/0.5）
- LLM 调用预算受控：聚合 4-6 次/篇，单主题维持 2 次

**Non-Goals:**

- 不清理存量孤岛标签（用户明确决策：存量不管）
- 不做 feed 级手动"聚合型"配置
- 不拆子条目为一等实体（子条目独立打标/embedding/推荐留待未来 change；本次切片产物是其地基）
- 不改标签表结构、不改前端
- 不改 event 标签定位（辅助标签聚合载体 + 未来分析锚点）

## Decisions

### D1: 聚合判定放在摘要调用，输出 HTML 注释标记

**决策**：`summarizeContent` 的 system prompt 要求摘要第一行输出 `<!-- form: mono|aggregate -->`，代码解析后剥离注释再存 AIContentSummary，标记值存 `articles.content_form`（varchar）。

**为什么不是别的**：

- *feed 级手动配置*：用户已否决（心智负担）；且同一 feed 可能混发单篇与合集
- *文章级规则判定（标题数/长度阈值）*：实测区分不开——单主题长教程也有 20+ 个章节标题（博客园 .NET 实战篇 29 个标题讲一件事），少数派 13 个标题是半聚合。真正的区分信号是"栏目间语义异构度"，只有看过全文的 LLM 能判
- *独立判定调用*：白花一次调用。摘要调用本就看全文，边际成本≈0

**标记格式选 HTML 注释的原因**：不渲染、前端零感知、代码正则一行剥离。若解析失败（模型没输出），content_form 落空 → 走 mono 路径，天然降级。

### D2: 切片器是纯代码，按 `##` 栏目级切

**决策**：对 AIContentSummary（markdown）按 `## ` 切片；片长 <300 runes 的短栏目向后并入相邻片；目标 4-6 片；若 `##` 切完超过 8 片，对超长栏目按 `### ` 细分；仍超 8 片则从尾部合并相邻片压回 8。

**为什么按 `##` 不按 `####` 条目级**：条目级会把一期周刊切成 20-50 片，LLM 调用失控。栏目级（导读/正文专题/科技动态/工具/言论/图片）是摘要生成 prompt 的固有结构（实测阮一峰、博客园、少数派摘要同构：`# 标题` + `## 导读` + `## 正文整理` + `###/####` 层级），4-6 片正好落在 4-6 次调用预算内。

**导读栏目处理**：`## 导读` 片与首个正文栏目信息高度重叠，切片时跳过导读片（其内容是对正文的概括，map 它只会产出与正文片重复的标签，reduce 后没有增量）。

### D3: map 阶段每片单次调用，三分类融合 prompt

**决策**：聚合路径不复用现有双分支（event/person + keyword 两次调用），每片用**一个融合 prompt**（现有两个 system prompt 合并 + 切片上下文），返回该片的 event/person/keyword 候选，每片上限 3-4 个标签。

**为什么敢单调用**：现有双分支拆开的动机是整篇文章上下文太重、分类间互相干扰；每片只有 500-1500 runes，单 prompt 三分类完全扛得住。4-6 片 × 1 次 = 4-6 次调用，符合预算。

**JSON schema**：融合现有两个 schema——tags 数组每项 label/category（event|person|keyword）/aliases/description/auxiliary_labels（event/person 必填 3-5 个，keyword 不用）。解析复用 `parseRawTagObjects` + `parseAuxiliaryLabels`（已兼容对象数组与校验规则）。

### D4: reduce 是纯代码，slug 去重 + 文章级上限

**决策**：跨片候选按 `Slugify(label)` 去重（同名保留首栏目出现的，即 score 更高的），文章级上限 15 个；不做 LLM 仲裁（片间同主题撞车靠 slug 去重足够，语义级撞车如"AI 缓存"vs"提示词缓存"留给既有的 embedding 合并建议机制 `tag-merge-suggestions` 处理，不新增机制）。

### D5: score 按栏目位置分层

**决策**：聚合路径产出标签 score 取所在片的层级：首个正文栏目 0.9 / 中间栏目 0.7 / 尾部栏目（言论/图片/往期回顾类）0.5。mono 路径维持一律 0.7。

**为什么**：现有 score 无区分度（全部 0.7），下游排序拿不到信号；栏目位置是免费的质量代理（周刊第一个专题通常是本期主打）。

### D6: mono 路径参数放宽，不重构

**决策**：`maxSummaryRunesForTagging` 2000→4000，`maxArticleTags` 5→6，仅此而已。mono 路径继续用双分支提取。

**为什么**：4000 runes 覆盖绝大多数单主题文章摘要（库内单主题摘要普遍 <4000）；不动双分支避免把本次 change 的爆炸半径扩大到所有存量路径。

### D7: 分流点在 `tagArticle` 编排层

**决策**：`tagArticle` 读 `article.ContentForm`，aggregate 走新函数 `tagAggregateArticle`（同文件或新文件），产出后共用现有 `findOrCreateTag` / `createArticleTopicTagLink` / auxiliary labels / event embedding enqueue 链路。

**为什么**：extraction 子包保持纯文本提取语义（tagging-domain spec 约束"extraction 只做文本提取"）；分流是编排层职责，域结构不动。

## Risks / Trade-offs

- **[摘要模型不配合输出标记]** → 解析失败 content_form 为空，走 mono 老路径。效果退化为现状（不会更糟）。可观察 `content_form` 空值率，若显著再收紧 prompt
- **[切片器遇到结构异常的摘要]**（无 `##`、纯文本）→ 切片器退化为单片（整篇作为 1 片），等效 mono 但走融合 prompt；防御性边界：片数 <1 或全部为空时直接回落 mono 路径
- **[半聚合文章（少数派 iOS 27 三条 Beta 合一）]** → 判成 aggregate 会多花调用，但标签产出更全，无害；判成 mono 也不比现状差
- **[聚合路径 LLM 部分片失败]** → 单片失败记录 warning 跳过该片（沿用现有 per-tag error 容错风格），其余片正常入库；不整篇重试
- **[标签数 15/篇喂给下游（偏好画像、看板匹配）]** → 画像/匹配消费 `article_topic_tags` 全量，标签变多+score 有区分度后，依赖 score 的排序逻辑自然受益；不依赖 score 的逻辑最多多算几个候选，本地 embedding 成本忽略（用户已确认）
- **[maxArticleTags 5→6 影响 mono 存量重打]** → 只对新打标生效；RetagArticle 语义不变（force 重打时用新参数），可接受

## Migration Plan

1. 迁移：`articles` 加 `content_form varchar` 可空列（先迁移后发码，向后兼容——老代码不读该列）
2. 后端发布后：新完成摘要的文章开始携带形态标记；新打标文章按分流走双路径
3. 存量文章（content_form 为空）全部走 mono 路径，行为与现状一致（截断放宽到 4000、上限 6 是唯一的轻微变化）
4. 回滚：代码回滚即可，列留着无害；无数据回填无清理需求

## Open Questions

（无——调用预算、embedding 成本、存量策略、判定方式均已与用户确认）
