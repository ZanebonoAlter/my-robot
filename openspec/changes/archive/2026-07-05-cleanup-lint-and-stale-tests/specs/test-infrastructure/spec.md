## ADDED Requirements

### Requirement: 全量门禁零失败基线

仓库后端 SHALL 维持全量门禁零失败：`golangci-lint run ./...` 与 `go test ./...`（含 testcontainer 集成测试）在全干工作树上执行 MUST 零失败。既存 lint 债（gofmt / unused / errcheck 等）与 pre-existing 坏测试 MUST 即时清理，不得积压阻塞《开发执行规范》§11.1 归档门禁。新增代码引入新 lint / 测试失败时，MUST 在同一 change 内修复。

#### Scenario: 全量 lint 零失败

- **WHEN** 在干净工作树上执行 `cd backend-go && golangci-lint run ./...`
- **THEN** SHALL 返回 0 issues（exit 0）

#### Scenario: 全量测试零失败

- **WHEN** 在 Docker daemon 可用的环境下，干净工作树上执行 `cd backend-go && go test ./...`
- **THEN** 所有包 SHALL PASS，无 pre-existing 失败用例
