# Feed Icon Management (Delta)

## MODIFIED Requirements

### Requirement: RefreshFeed 按状态机决定是否重算 icon
`RefreshFeed` SHALL 仅当 `icon_source` 为 `auto` 或 `fallback` 时考虑重算 icon。当 `icon_source` 为 `custom` 时，SHALL 跳过 icon 重算，保持用户设定值不变。此外，当 `icon_source` 为 `auto` 且 icon 已是本地化路径（`/icons/` 开头）时，SHALL 跳过重算——已落盘的好图标不因远程临时故障被覆盖为 fallback；`auto` 但 icon 仍为远程 URL（存量未本地化数据）时 SHALL 执行重算以完成本地化。

#### Scenario: custom 状态不被刷新覆盖
- **WHEN** feed.icon_source = `custom`，执行 RefreshFeed
- **THEN** icon 和 icon_source 均不变

#### Scenario: fallback 状态触发重算
- **WHEN** feed.icon_source = `fallback`，执行 RefreshFeed
- **THEN** 尝试用候选管线重算并下载本地化 icon

#### Scenario: auto + 本地路径跳过重算
- **WHEN** feed.icon_source = `auto` 且 icon 为 `/icons/feeds/42.png`，执行 RefreshFeed
- **THEN** icon 管线不执行（不下载、不探测首页），icon 与 icon_source 均不变

#### Scenario: auto + 存量远程 URL 触发本地化
- **WHEN** feed.icon_source = `auto` 且 icon 为 `https://example.com/favicon.ico`（存量数据），执行 RefreshFeed
- **THEN** 执行候选管线，下载成功后 icon 换为本地路径

### Requirement: icon 选图优先级
当 RefreshFeed 重算 icon 时，SHALL 按以下候选顺序尝试获取并**下载验证**：1) RSS `<image>` 标签 URL（parsed.Image）；2) 站点首页 HTML 中 `<link rel="icon" / "shortcut icon" / "apple-touch-icon">` 的 href（相对路径转绝对）；3) RSS channel link 的 host + `/favicon.ico` 猜测。首个下载成功的候选落盘为本地文件，置 icon = 本地路径、icon_source = `auto`；全部失败则保持 icon_source = `fallback`、icon = `mdi:rss`。候选 2 仅在候选 1 缺失时执行（避免每次 refresh 都请求站点首页）。

#### Scenario: RSS image 优先
- **WHEN** parsed.Image 非空且下载成功
- **THEN** icon = 下载后的本地路径，icon_source = `auto`

#### Scenario: RSS image 缺失时用 HTML link 探测
- **WHEN** parsed.Image 为空，站点首页 HTML 含 `<link rel="icon" href="/static/icon.png">` 且下载成功
- **THEN** icon = 该 URL 下载后的本地路径，icon_source = `auto`

#### Scenario: HTML link 缺失时回退 favicon.ico 猜测
- **WHEN** parsed.Image 为空、首页 HTML 无 icon link 标签，`{host}/favicon.ico` 下载成功
- **THEN** icon = 下载后的本地路径，icon_source = `auto`

#### Scenario: 候选下载失败顺延
- **WHEN** parsed.Image 非空但下载失败（404/超时/非图片/超限）
- **THEN** 继续尝试后续候选，不直接判 fallback

#### Scenario: 全部候选失败
- **WHEN** 三个候选均缺失或下载失败
- **THEN** icon = `mdi:rss`，icon_source = `fallback`

#### Scenario: icon 下载失败不影响 refresh 结果
- **WHEN** 文章抓取成功但全部 icon 候选下载失败
- **THEN** RefreshFeed 返回成功（RefreshStatus = success），仅 icon 保持 fallback

### Requirement: FetchFaviconURL 使用站点 URL 而非 feed URL
favicon 探测 SHALL 以站点首页 URL（RSS channel link）为基准，SHALL NOT 使用 RSS feed URL（聚合器域名）或 Google s2 等第三方服务。探测 SHALL 包含首页 HTML `<link rel="icon">` 解析与 `{scheme}://{host}/favicon.ico` 猜测两个层级；站点 URL 为空或无法解析 host 时返回空（由调用方保持 fallback 态）。

#### Scenario: 从站点 URL 拼 favicon
- **WHEN** siteURL = `https://example.com/articles`，HTML 中无 icon link
- **THEN** 猜测候选为 `https://example.com/favicon.ico`

#### Scenario: HTML link 相对路径转绝对
- **WHEN** siteURL = `https://example.com/articles`，HTML 含 `<link rel="icon" href="/a/b.png">`
- **THEN** 探测候选为 `https://example.com/a/b.png`

#### Scenario: 站点 URL 无法解析
- **WHEN** siteURL 为空或解析失败
- **THEN** 返回空（由调用方保持 fallback 态）

## ADDED Requirements

### Requirement: icon 下载本地化存储
后端 SHALL 将下载成功的 feed icon 写入本地文件系统（默认 `data/icons/feeds/<feedID>.<ext>`，目录可通过配置覆盖），并以「临时文件 + rename」原子写入。下载 SHALL 带超时（≤10s）与大小上限（≤256KB），并校验响应为图片内容。DB `icon` 字段存 `/icons/feeds/<feedID>.<ext>` 形式的同源相对路径。

#### Scenario: 下载成功落盘
- **WHEN** 候选 URL 返回 200 且 Content-Type 为 image/*（或 .ico 内容），大小 ≤256KB
- **THEN** 文件写入 `data/icons/feeds/<feedID>.<ext>`，DB icon 更新为对应 `/icons/...` 路径

#### Scenario: 超限或非图片拒绝写入
- **WHEN** 响应大小超 256KB 或内容非图片
- **THEN** 不落盘，视为该候选下载失败

#### Scenario: 同 feed 换扩展名清理旧文件
- **WHEN** feed 已有 `<feedID>.ico`，新下载图标为 png
- **THEN** 写入 `<feedID>.png` 并删除旧的 `<feedID>.ico`

### Requirement: 本地 icon 静态服务
后端 SHALL 注册 `/icons` 静态路由，直接服务 icon 存储目录，使前端可通过后端同源地址访问本地 icon。该路由独立于前端产物托管逻辑，dev 与生产模式均可用。

#### Scenario: 访问已落盘 icon
- **WHEN** GET `/icons/feeds/42.png` 且文件存在
- **THEN** 返回 200 与图片字节，带合理 Content-Type，且响应头含 `X-Content-Type-Options: nosniff` 与 `Content-Security-Policy: sandbox`（防 SVG 存储型 XSS）

#### Scenario: 访问不存在的 icon
- **WHEN** GET `/icons/feeds/999.png` 且文件不存在
- **THEN** 返回 404，不影响其他接口

### Requirement: 删除 feed 时清理 icon 文件
删除 feed 时 SHALL 一并删除其本地 icon 文件（若存在）；清理失败 SHALL NOT 阻断删除流程。

#### Scenario: 删除带本地 icon 的 feed
- **WHEN** DELETE feed 且 `data/icons/feeds/<feedID>.*` 存在
- **THEN** feed 记录删除成功，对应 icon 文件被删除

### Requirement: 前端同源路径解析与远程 URL 兼容
前端 `FeedIcon.vue` SHALL 识别三类 icon 值：iconify id（如 `mdi:rss`）、远程 http(s) URL（存量数据）、`/` 开头的同源相对路径。相对路径 SHALL 用 `getApiOrigin()` 拼成后端绝对地址渲染 `<img>`；远程 URL 按原逻辑直接渲染，保证存量数据在下次 refresh 前仍可见。

#### Scenario: 本地路径拼源渲染
- **WHEN** icon = `/icons/feeds/42.png`，dev 模式前端端口 3000
- **THEN** `<img>` src 渲染为 `http://localhost:5000/icons/feeds/42.png`

#### Scenario: 存量远程 URL 仍渲染
- **WHEN** icon = `https://example.com/favicon.ico`（未完成本地化的存量数据）
- **THEN** `<img>` 按原远程地址渲染，失败仍降级 `mdi:rss`
