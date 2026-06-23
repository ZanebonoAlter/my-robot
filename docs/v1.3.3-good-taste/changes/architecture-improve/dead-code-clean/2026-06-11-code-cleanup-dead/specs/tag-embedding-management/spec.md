# Delta Spec: tag-embedding-management (code-cleanup-dead)

## MODIFIED Requirements

### Requirement: 余弦相似度计算使用 math.Sqrt

`airouter/embedding.go` 中的手写 Newton's method `sqrt` 函数 SHALL 替换为标准库 `math.Sqrt`。原实现（lines 43-57）使用迭代法计算平方根，精度和性能均不如 `math.Sqrt`。替换后函数签名和调用方式不变。

#### Scenario: embedding.go 不包含手写 sqrt 函数
- **WHEN** 检查 `internal/platform/airouter/embedding.go` 中的未导出函数
- **THEN** 不存在手写的 `sqrt` 或 Newton's method 实现的平方根函数

#### Scenario: 余弦相似度使用 math.Sqrt
- **WHEN** 检查 `airouter/embedding.go` 中的余弦相似度计算（cosineSimilarity 或类似函数）
- **THEN** 使用 `math.Sqrt` 计算向量模长，不调用自定义 sqrt 实现

#### Scenario: math 包已导入
- **WHEN** 检查 `airouter/embedding.go` 的 import 列表
- **THEN** 包含 `"math"` 导入
