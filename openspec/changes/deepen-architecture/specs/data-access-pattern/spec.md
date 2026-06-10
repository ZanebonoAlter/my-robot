## ADDED Requirements

### Requirement: 每个 domain 包应有明确的 DB 访问层
每个 `internal/domain/<name>/` 业务包 SHALL 将 DB 访问集中在 `repository.go` 或 `service.go` 中，handler 层不直接使用 `database.DB`。

#### Scenario: Article handler 获取文章列表
- **WHEN** `GET /api/articles` 被请求
- **THEN** `article/handler.go` 调用 `articleService.List(filters)`，由 service 内部访问 DB，handler 不出现 `database.DB.Create/Find/First/Update/Delete`

### Requirement: daily_report 的 repository 模式视为参考实现
`internal/domain/daily_report/repository.go` SHALL 作为其他 domain 包实现数据访问层的参考模板。

#### Scenario: 新 domain 包参考 daily_report
- **WHEN** 创建一个新 domain 包
- **THEN** 参照 `daily_report/repository.go` 模式：函数接收 `*gorm.DB` 参数或依赖注入，不直接使用全局 `database.DB`

### Requirement: platform/ai 职责归入 airouter
`platform/ai/service.go` 中的 `GetSystemPrompt()`、`PrepareArticleContent()`、`ParseSummaryMarkdown()` 工具函数 SHALL 迁移到 `platform/airouter/` 包，`platform/ai/` 目录随后删除。

#### Scenario: ContentCompletionService 使用迁移后的工具
- **WHEN** `ContentCompletionService.summarizeContent()` 需要构建 prompt
- **THEN** 调用 `airouter.GetSystemPrompt("zh")` 而非 `platformai.GetSystemPrompt("zh")`

### Requirement: 消除 DB Model 与 Domain Type 双定义（长期方向）
`domain/models/` 中的 GORM 模型 SHALL 逐步按所属领域迁移到对应 domain 包中。跨域共享的模型保留在 `models/`，但每个 domain 包优先维护自己的领域类型。

#### Scenario: TopicTag 领域类型的 model→domain 转换
- **WHEN** `tagging.GetArticleTags(articleID)` 从 DB 读取 `models.TopicTag`
- **THEN** 通过集中的 `toDomainTag(model)` 映射器转换为 `tagging.TopicTag`，而非在 6 处 call site 各自手动映射

#### Scenario: 多个 domain 包引用 Article 模型
- **WHEN** `feed` 和 `tagging` 包都需要 Article 数据结构
- **THEN** `models.Article` 保留在 `models/` 中，但各 domain 包通过 ID 引用或使用自己包内的子集类型，不直接操作 `models.Article` 的 GORM 关系
