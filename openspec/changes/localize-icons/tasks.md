# Tasks: localize-icons

## 1. 前端：UI 图标本地化

- [x] 1.1 编写图标名提取逻辑：扫描 `front/app/` 源码中的 `"<prefix>:<name>"` 图标名（复用调研时的正则规则）
- [x] 1.2 编写生成脚本 `front/scripts/generate-icon-subset.mjs`：从 `@iconify-icons/mdi/icons.json` 用 `@iconify/utils` 提取子集，写 `app/assets/iconify-subset.json`；`package.json` 增加 `generate:icons` script
- [x] 1.3 运行生成脚本，提交首版子集产物
- [x] 1.4 新建 `front/app/plugins/iconify-local.ts`：import 子集 + `addCollection` 注册
- [x] 1.5 单元测试：断言「源码扫描图标名 ⊆ 子集产物」（放 `front/tests/unit/` 或就近），失败时输出缺失图标名与 `pnpm generate:icons` 提示

## 2. 后端：icon 下载与存储

- [x] 2.1 新增 icon 存储组件（`internal/reader/service/icon_store.go`）：下载（超时 10s、上限 256KB、图片 Content-Type 校验）→ 临时文件 + rename 落盘 → 返回 `/icons/feeds/<feedID>.<ext>` 路径；同 feed 换扩展名时清理旧文件
- [x] 2.2 viper 配置 `storage.icon_dir`，default `data/icons`
- [x] 2.3 router 注册 `r.Static("/icons", iconDir)`（独立于前端产物托管）
- [x] 2.4 favicon 探测增强（`rss_parser.go`）：首页 HTML `<link rel="icon|shortcut icon|apple-touch-icon">` 解析（相对转绝对）+ `/favicon.ico` 猜测两级候选
- [x] 2.5 重构 `resolveFeedIcon` 为候选管线：RSS image → HTML link → favicon.ico，逐候选下载验证，成功置 auto + 本地路径，全失败保持 fallback；下载失败不影响 RefreshStatus
- [x] 2.6 删除 feed 流程挂 icon 文件清理（失败不阻断）

## 3. 前端：FeedIcon 同源路径

- [x] 3.1 `FeedIcon.vue` 增加 `/` 开头分支：用 `getApiOrigin()` 拼绝对地址渲染 `<img>`
- [x] 3.2 更新/补充 `FeedIcon.test.ts`：本地路径拼源、远程 URL 兼容、onerror 降级

## 4. 测试

- [x] 4.1 后端单测（TDD，先写后实现）：icon_store 下载成功/超限/非图片/换扩展名清理；favicon 两级探测（httptest server）；resolveFeedIcon 候选顺延与全失败兜底；删除 feed 清理
  - 运行：`cd backend-go && go test ./internal/reader/...`
- [x] 4.2 前端单测：`pnpm test:unit`（FeedIcon + 子集一致性）
  - 运行：`cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit 2>&1"`
- [x] 4.3 受影响包编译与静态检查：`golangci-lint run ./...` / `go vet ./...` / `go build ./...`；前端 `pnpm lint` + `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"`

## 5. 文档

<!-- doc-impact: flow,api,configuration -->

- [x] 5.1 `docs/reference/flow/` feed 相关 flow：更新 icon 获取链路描述（候选管线 + 本地化存储）与业务约束
- [x] 5.2 `docs/reference/api/`：补充 `/icons` 静态路由说明
- [x] 5.3 `docs/reference/configuration.md`：补充 `storage.icon_dir` 配置项
- [x] 5.4 `front/AGENTS.md` 或 standard/frontend：说明新增图标后需 `pnpm generate:icons`

## 6. 验证

- [x] 6.1 断网/代理场景 UI 图标：`cd front && pnpm build` 产物中 grep 不含 `api.iconify.design` 运行时请求代码路径；浏览器 devtools Network 面板无 iconify 域名请求
- [x] 6.2 子集一致性校验：`cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm test:unit 2>&1"` 中子集断言通过
- [x] 6.3 端到端：启动后端 + 前端，手动 refresh 一个 fallback feed（id=8 InfoQ / id=9 博客园），`docker exec <pg> psql -U postgres -d syntopica -c "SELECT id, icon_source, icon FROM feeds WHERE id IN (8,9);"` 期望 icon_source=auto 且 icon 为 `/icons/...` 路径；`ls data/icons/feeds/` 有对应文件；浏览器访问 `http://localhost:5000/icons/feeds/8.*` 返回 200
- [ ] 6.4 页面验证（受环境阻塞：:5000 被 WSL 镜像网络残留 relay 占用，用户需 `wsl --shutdown` 后重启前后端再验）
- [x] 6.5 归档门禁：`./scripts/doc-impact.sh verify` 与 `./scripts/check-standards.sh` 通过
