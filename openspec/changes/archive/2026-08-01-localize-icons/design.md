# Design: localize-icons

## Context

两套 icon 体系均运行时联网（详见 proposal）。约束：单用户应用；开发模式前端跑 Nuxt dev server（端口 3000+）、后端 5000，非同源，但前端已有 `getApiOrigin()` 工具可拼后端源；生产模式后端直接托管前端构建产物（`internal/app/static.go`），同源。后端已有 `data/` 目录用于持久化（postgres 卷），新增 `data/icons/` 不引入新基础设施。

## Goals / Non-Goals

**Goals:**
- UI 图标（mdi:*）运行时零网络请求，离线/代理环境正常渲染。
- Feed icon 由后端下载落盘，前端只加载同源地址，一次下载长期复用。
- favicon 探测能命中"根路径无 /favicon.ico"的站点（HTML `<link rel="icon">`）。
- 存量 feed 平滑过渡，不强制回填、不破坏现有 `custom` 图标。

**Non-Goals:**
- 不引入图片处理（缩放/转格式/webp 化），存原始字节。
- 不做 icon 定期失效/刷新策略（仍跟随 RefreshFeed 重算节奏）。
- 不做用户上传自定义图片 icon（现有 custom 仅支持 iconify id/URL，保持不变）。
- 不替换 `@iconify/vue` 组件方案，不引入 unplugin-icons 等新构建工具。

## Decisions

### D1: UI 图标——构建期子集 + 启动时 addCollection

写一个生成脚本 `front/scripts/generate-icon-subset.ts`（或 `.mjs`）：
1. 扫描 `app/` 下源码中的 `"<prefix>:<name>"` 图标名（正则提取，与本次调研用同一套规则）；
2. 从 `@iconify-icons/mdi/icons.json` 用 `@iconify/utils` 的 `getIcons(data, names)` 提取子集；
3. 产物写入 `app/assets/iconify-subset.json`（纳入 git 管理）。

Nuxt plugin `app/plugins/iconify-local.ts` 启动时 `import subset from '~/assets/iconify-subset.json'` + `addCollection(subset)`。`@iconify/vue` 查找顺序是本地注册优先，命中后不再发网络请求。

**为什么不用全量 addCollection**：mdi 全集 JSON 数 MB，全量进 bundle 浪费；项目实际只用 ~80 个。**为什么不换 unplugin-icons/@nuxt/icon**：引入新构建依赖、改动面大，项目规范禁止未经要求加工具。**为什么产物纳入 git**：避免每个开发者/CI 都要先跑生成脚本才能 dev；新增图标时人工重跑脚本提交即可（tasks 里附 lint 检查或文档说明）。

### D2: Feed icon——RefreshFeed 时后端下载落盘

新增 `internal/reader/service/icon_store.go`（或就近放 platform/storage）：
- `SaveFeedIcon(feedID, remoteURL) (localPath string, err error)`：HTTP GET（超时 10s、上限 256KB、校验 Content-Type 为 image/* 或 .ico 的 application/octet-stream），按 Content-Type/URL 推断扩展名，写 `data/icons/feeds/<feedID>.<ext>`（同 feed 换扩展名时清理旧文件）。
- 存储根目录走 viper 配置 `storage.icon_dir`，default `data/icons`。
- 后端注册静态路由 `r.Static("/icons", iconDir)`（独立于前端托管的 static.go，dev/生产都走 5000 端口）。
- DB `icon` 字段存 `/icons/feeds/42.png` 这类相对路径。

**为什么文件系统而非 DB bytea**：AGENTS.md 已对齐存文件系统；静态文件直接由 Gin 服务，无需 handler 读库，浏览器可缓存。

### D3: favicon 多候选探测 + 下载即验证

重构 `resolveFeedIcon` 流程为候选管线（仅当 icon_source ∈ {auto, fallback}）：
1. `parsed.Image`（RSS `<image>`）
2. 站点首页 HTML `<link rel="icon|shortcut icon|apple-touch-icon">`（仅当候选 1 缺失时，GET channel link 首页，限 512KB，正则/简易解析提取 href，相对路径转绝对）
3. `{scheme}://{host}/favicon.ico` 猜测

对每个候选**尝试下载**（D2 的 SaveFeedIcon）：成功 → `icon = 本地路径, icon_source = auto`；全部失败 → 保持 `fallback, mdi:rss`。下载失败不计入 feed refresh 错误（icon 是辅助信息，不让它把 refresh 标失败）。

**为什么"下载即验证"替代"HEAD 探测"**：省一次请求；猜错（404/非图片）自然落到下一候选。**为什么 HTML 解析放候选 2**：比猜测多一次请求，只在 RSS image 缺失时付出成本。

### D4: 前端 FeedIcon 同源路径解析

`FeedIcon.vue` 增加一个分支：`icon` 以 `/` 开头 → 用 `getApiOrigin()` 拼成绝对地址渲染 `<img>`（生产同源时 origin 相同，无感知）。其余逻辑（onerror 降级 mdi:rss）不变。由于 UI 图标已本地化（D1），降级兜底在离线时也能正常显示。

**旧远程 URL 兼容**：存量 auto feed 的 icon 仍是 `https://...` 远程 URL，`isUrl` 分支照常渲染，下次 refresh 自动换成本地路径——无需数据迁移。

### D5: 并发与重复下载

RefreshFeed 已有并行化（refresh-parallelization spec），同一 feed 不会并发 refresh（调度器按 feed 串行），文件写用「临时文件 + rename」保证原子性即可，无需锁。

## Risks / Trade-offs

- [新增图标忘记重跑生成脚本 → 该图标运行时回退联网/空白] → tasks 里加一条 lint 或 test 校验：扫描源码图标名 ⊆ 子集产物，CI/pre-push 可发现。
- [favicon 首页 HTML 解析被反爬/超时拖慢 refresh] → 独立短超时（5s），失败即下一候选；且仅候选 1 缺失时触发。
- [恶意/异常站点返回超大或伪装 Content-Type 的文件] → 256KB 硬上限 + 魔数嗅探（可选）+ 存储路径不含用户输入（feedID 数字）。
- [icon 文件只增不减占用磁盘] → 同 feed 覆盖写；feed 删除时清理其 icon 文件（挂在删除 feed 流程）；量级极小（24 feed × 几 KB），不做 GC。
- [dev 模式前端 3000 端口访问 `/icons/...` 打到 Nuxt] → D4 统一用 `getApiOrigin()` 拼绝对地址，不依赖 dev proxy。

## Migration Plan

1. 部署后无需手工操作；存量 feed 随下次 refresh（定时或手动）自动本地化。
2. 回滚：代码回退即可——旧代码遇到 DB 里的 `/icons/...` 相对路径时，`isUrl=false` 会走 Icon 组件渲染兜底（mdi:rss），不白屏但远程 icon 消失；可接受。

## Open Questions

- 无（范围、存储位置、探测增强均已与用户对齐）。
