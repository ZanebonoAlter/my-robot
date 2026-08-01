# 后端 Lint 配置（golangci-lint）

> **权威源**：本文件是后端静态检查配置的唯一权威。门禁命令见《开发执行规范》§4.1。

## 配置文件

`backend-go/.golangci.yml`

## 推荐启用 linters

| Linter | 用途 |
|--------|------|
| `staticcheck` | Go 静态分析（golint 的继任） |
| `go vet` | 官方检查 |
| `revive` | 代码风格（golint 替代品） |
| `gosec` | 安全扫描 |
| `errcheck` | 未检查的错误返回值 |
| `gocritic` | 代码质量建议 |
| `unused` | 未使用的代码 |

## 安装（缺失时作为首个子任务）

```bash
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
```

## 运行

```bash
cd backend-go
golangci-lint run ./...
```

## 资料来源

收敛自原《开发执行规范》§4.3 与 `development.md` §后端代码风格。
