# Proposal: aggregate-article-tagging

## Why

聚合型文章（阮一峰周刊、博客园排行榜、少数派 roundup 等，现库 5 个 feed 400+ 篇）的打标签机制系统性漏信息：`buildArticleSummary` 截断到 2000 字（周刊摘要实际 5000-12000 字，后半部分栏目根本没进 LLM 视野）+ `maxArticleTags=5` 双重硬顶（一期周刊 10-20 个主题装不下），迫使 LLM 捏造"合并标签"（如"带蓝牙识别的交通摄像头与声波灭火技术动态"把两条无关新闻捏成一个 event），且产出的 event 标签 100% 只被 1 篇文章引用（孤岛，无聚合价值）。

## What Changes

- 摘要生成调用（`content_completion_service.go`，本就阅读 firecrawl 全文）prompt 增加输出内容形态标记：开头一行 `<!-- form: mono|aggregate -->`；代码解析剥离 HTML 注释后存入 `articles.content_form` 新列（存量文章该列为空）
- `TagArticle` 按 `content_form` 分流：
  - `mono`（单主题）：沿用现有双分支 LLM 提取，输入截断放宽到 4000 runes，文章级标签上限放宽到 6
  - `aggregate`（聚合）：纯代码切片（按摘要 markdown `##` 栏目级切，短栏目 <300 字向后并入相邻片，目标 4-6 片）→ 每片 1 次融合 prompt LLM 调用（event/person/keyword 三分类合并到单 prompt）→ 纯代码 reduce（slug 去重，文章级上限 ~15）
  - 列为空（存量/未生成摘要）：走 mono 老路径（与"存量不治理"决策对齐）
- 聚合路径 score 分层：首个正栏目产出的标签 0.9 / 中间栏目 0.7 / 尾部栏目 0.5（现有路径 score 一律 0.7 不变）
- event 标签自动 enqueue embedding 的既有逻辑对聚合路径同样生效（embedding 为本地服务，成本忽略）
- 不做：用户手动配置 feed 内容形态（靠摘要调用自动判定，零配置零心智负担）；存量孤岛标签清理；标签结构/表结构其他变更

## Capabilities

### New Capabilities

- `aggregate-tagging`: 聚合型文章识别（content_form 标记的产生与存储）+ 切片 map-reduce 打标路径（切片器、融合 prompt、reduce 去重、score 分层）

### Modified Capabilities

- `tagging-domain`: TagArticle 输入构造与上限变化——单主题路径截断从 2000→4000 runes、上限从 5→6；新增按 content_form 分流的编排行为（extraction 仍是纯文本提取，分流在 tagger 编排层）

## Impact

- **后端**：
  - `backend-go/internal/reader/service/content_completion_service.go`：summarize prompt 加形态标记要求、解析剥离、存列
  - `backend-go/internal/tagmanagement/service/core/article_tagger.go`：分流逻辑、聚合路径编排、上限调整
  - `backend-go/internal/tagmanagement/service/core/extractor_enhanced.go`（或新文件）：融合 prompt（现有 event/person 与 keyword 两个 prompt 合一）、切片器（纯代码）
  - `backend-go/internal/models/article.go` + 数据库迁移：`articles` 加 `content_form` varchar 列
- **前端**：无必需改动（形态标记是 HTML 注释，剥离后摘要展示不变；标签数变多由现有 UI 自然承载）
- **AI 调用成本**：聚合型 4-6 次/篇（现状 2 次），单主题维持 2 次；摘要调用零增量（只加输出字段）
- **数据库迁移**：加一列，无回填，无破坏性
- **部署影响**：合并后新入库文章自动获得 content_form 分流打标；存量文章与未开摘要的 feed 行为不变；阮一峰/博客园/少数派等 5 个聚合 feed 从下一篇文章开始标签覆盖明显变全（每期 15 个左右真实话题标签，替代现在 5 个捏造标签）
