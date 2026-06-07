## 1. 实体+数值提取器

- [ ] 1.1 创建 `backend-go/internal/domain/tagging/entityfilter/` 包，定义 `LabelEntities` 结构体和 `ExtractEntities` 函数
- [ ] 1.2 实现数值提取：正则匹配数字+单位（亿、万、%、美元、亿元、万亿元、点等），归一化（去逗号）
- [ ] 1.3 实现关键词提取：按空格/标点/中英文边界切分，去停用词
- [ ] 1.4 实现英文缩写提取：大写字母开头的连续 token
- [ ] 1.5 实现 `ShouldConsiderMerge(labelA, labelB string) (bool, string)` 过滤函数
- [ ] 1.6 编写单元测试：覆盖数值不同、实体无交集、数值相同、单方无数值等场景

## 2. 集成到增量路径

- [ ] 2.1 修改 `RecordMergeSuggestions`：写入前对每个 candidate 调用 `ShouldConsiderMerge`，跳过被过滤的
- [ ] 2.2 添加 debug 日志：记录被过滤掉的候选对及原因

## 3. 集成到全量扫描路径

- [ ] 3.1 修改 `runFullScan`：对每个 FindSimilarTags 结果调用 `ShouldConsiderMerge`，跳过被过滤的
- [ ] 3.2 SSE 进度消息增加 `filtered` 计数字段
- [ ] 3.3 验证：运行全量扫描，确认过滤生效且日志正常

## 4. 验证与清理

- [ ] 4.1 用现有 146 条 pending 数据批量测试 ShouldConsiderMerge，统计过滤率和准确性
- [ ] 4.2 确认编译和 lint 通过：`go build ./...` && `go vet ./...` && `golangci-lint run ./internal/domain/tagging/...`
