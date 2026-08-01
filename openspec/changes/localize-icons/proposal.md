# Proposal: localize-icons

## Why

项目两套 icon 体系都依赖运行时联网加载，开代理或离线时大面积加载失败：

1. **UI 图标**：全项目 80+ 个 `mdi:*` 图标通过 `@iconify/vue` 渲染，但从未注册本地图标数据，每次渲染都向 `api.iconify.design` 发 HTTP 请求；`@iconify-icons/mdi` 本地数据包已安装却 0 处引用（死依赖）。开代理时该域名请求失败 → 界面图标空白。
2. **Feed 图标**：后端只把「远程 URL 字符串」存进 DB，前端 `<img>` 浏览器直连 20+ 个不同域名加载，无本地缓存层。代理环境下这些杂散域名请求易失败 → 降级 `mdi:rss` → 而 `mdi:rss` 又要连 iconify → 双杀全灭。且 favicon 仅靠猜 `{host}/favicon.ico`，猜不中的站点（如 InfoQ、部分博客园 feed）永远停在 fallback。

## What Changes

- **UI 图标本地化**：生成项目实际使用的 mdi 图标子集，通过 Nuxt plugin 在启动时 `addCollection` 注册到 `@iconify/vue`，运行时零联网；提供可重跑的生成脚本，新增图标后重新生成。
- **Feed 图标下载本地化**：RefreshFeed 重算 icon 时，后端将选中的图标**下载到本地文件系统**（`data/icons/feeds/`），DB `icon` 字段存同源路径（`/icons/feeds/<id>.<ext>`），由后端静态服务提供；前端只访问同源地址，代理问题根治。
- **Favicon 探测增强**：候选顺序调整为 RSS `<image>` → 站点首页 HTML `<link rel="icon">` 解析 → `{host}/favicon.ico` 猜测；由于改为后端实际下载，候选有效性由下载结果自然判定（下载失败即试下一候选），治"猜不中"的那批 feed。
- **存量降级**：已有 feed 的远程 icon URL 保持可用，随下次 refresh 自然本地化；不强制一次性回填。

## Capabilities

### New Capabilities
- `ui-icon-localization`: 前端 UI 图标（mdi 系列）的本地化注册机制——图标子集生成、启动时注册、运行时零网络依赖、新增图标的维护流程。

### Modified Capabilities
- `feed-icon-management`: icon 重算从「存远程 URL」改为「下载存本地、DB 存同源路径」；favicon 获取从单纯 `/favicon.ico` 猜测增强为「HTML link 解析 + 猜测 + 下载验证」的多候选探测。

## Impact

- **后端**：`internal/reader/service/feed_service.go`（resolveFeedIcon 重构）、`internal/reader/service/rss_parser.go`（favicon 探测增强）、新增 icon 下载/存储组件、`internal/app/static.go` 或 router（本地 icons 静态服务）；`data/icons/` 目录（部署持久化）。
- **前端**：新增 `app/plugins/iconify-local.ts`（或类似）、图标子集生成脚本与产物文件；`FeedIcon.vue` 对同源相对路径的解析（如需拼 API base）。
- **依赖**：前端启用已安装的 `@iconify-icons/mdi` + `@iconify/utils`（无新增运行时依赖）；后端仅用标准库 + 既有 HTTP 客户端。
- **DB**：无 schema 变更；`icon` 字段值语义从「远程 URL」扩展为「同源路径或远程 URL」，前端两种形态都兼容。
- **文档**：flow（feed icon 链路）、api（icon 静态服务）、configuration（icons 存储目录）、architecture。
