# Proposal: aggregate-tagging-resilience

## Why

aggregate-article-tagging 归档验证发现聚合打标路径的标签产出被解析链路系统性吞掉：两篇派早报 7 片全败 → 整篇 0 标签（无兜底）；两篇周刊 18 次片级失败中 aux 校验失败 7 次全是 event 标签触发且整片陪葬 → 聚合路径至今 0 个 event 标签（30/30 全 keyword）；首个正文栏目片（应 0.9）2/2 零产出。change 的核心价值（聚合文章产出真实 event、主打栏目标签）实际未落地。

## What Changes

- **aux 校验降级**：融合路径（`parseSectionTags`）中 event/person 标签的 auxiliary_labels 校验失败时，丢弃 aux 保留标签入库（记 warning），不再因单个标签 aux 不合规让整片报废；mono 路径（`parseEventPersonTags`）同步适用同样降级策略
- **聚合路径兜底**：`tagAggregateArticle` 所有片处理完后标签数为 0（全片失败或全部返回空）时，回落 mono 路径（双分支提取 + heuristic 兜底），消除整篇裸奔
- **JSON 尾逗号容错**：`parseRawTagObjects` 解析前对 JSON 文本做尾逗号修复（对象/数组末元素多余逗号），降低模型 JSON 语法错误导致的重试耗尽率（日志中 `invalid character ']'` 类失败的主因）
- **keyword description 缺失降级**：融合路径 keyword 标签 description 为空时降级为补默认描述（"聚合打标提取的关键词"）或跳过该标签记 warning，不再整片报废（与 aux 降级同一原则：单标签问题单标签兜）
- 不做：不改 prompt 结构、不改切片器、不改 score 分层、不改 aux 表结构

## Capabilities

### New Capabilities

（无）

### Modified Capabilities

- `aggregate-tagging`: 逐片融合提取的容错行为变化——aux/description 校验失败从整片失败改为单标签降级；聚合路径零产出时回落 mono；JSON 解析增加尾逗号容错

## Impact

- **后端**：
  - `backend-go/internal/tagmanagement/service/core/extractor_section.go`：`parseSectionTags` 校验降级逻辑
  - `backend-go/internal/tagmanagement/service/core/extractor_enhanced.go`：`parseEventPersonTags` 同步降级；`parseRawTagObjects` 尾逗号修复（或提取公共 helper）
  - `backend-go/internal/tagmanagement/service/core/article_tagger.go`：`tagAggregateArticle` 返回 0 标签时的回落编排
- **前端**：无
- **行为影响**：聚合文章 event 标签存活率显著回升；派早报类全败文章不再 0 标签；aux 质量略降（部分 event 标签无 aux 锚点，依赖 description 生成链路后补）
- **部署影响**：合并后即生效，无需迁移、无需手动操作；对已入库的 2 篇 0 标签派早报可用 RetagArticle 补标
