## ADDED Requirements

### Requirement: 能力与业务用途绑定
系统 SHALL 为每个 AI capability 维护与业务用途的唯一绑定：`summary` SHALL 驱动文章自动总结；`digest_polish` SHALL 驱动日报生成；`topic_tagging` SHALL 驱动事件标签提取与标签相关的语义操作；`embedding` SHALL 驱动向量嵌入。每个业务流程 SHALL 仅通过其绑定的 capability 加载路由与 provider。

#### Scenario: 文章总结使用 summary 路由
- **WHEN** 文章自动总结流程（`summarizeContent`）调用 LLM
- **THEN** 系统 SHALL 通过 `summary` capability 加载路由与 provider

#### Scenario: 日报生成使用 digest_polish 路由
- **WHEN** 日报生成流程的任一 LLM 调用（聚类、要闻、叙事）执行
- **THEN** 系统 SHALL 通过 `digest_polish` capability 加载路由与 provider

### Requirement: 默认并发配额独立
系统 SHALL 为每个 capability 提供独立的默认并发上限信号量，可被路由级 `MaxConcurrency` 覆盖。不同 capability 的并发配额 SHALL 互不挤占。

#### Scenario: digest_polish 与 topic_tagging 并发隔离
- **WHEN** 日报生成与标签提取同时进行
- **THEN** `digest_polish` 与 `topic_tagging` SHALL 使用各自独立的信号量，一方占满配额时 SHALL NOT 阻塞另一方

### Requirement: 废弃的 article_completion
系统 SHALL NOT 定义 `article_completion` capability 常量，SHALL NOT 在任何 LLM 调用中使用 `article_completion` 作为 capability。

#### Scenario: 无 article_completion 调用
- **WHEN** 系统发起任何 LLM 调用
- **THEN** 该调用的 capability SHALL NOT 为 `article_completion`
