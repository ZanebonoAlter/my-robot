# Design — Feed Icon 状态机

## 1. 状态机定义

### 1.1 字段

```
feeds.icon        string  (值：iconify id 或图片 URL，如 "mdi:rss" / "https://...")
feeds.icon_source string  (来源：auto | custom | fallback，default fallback)
```

`Icon` 只承载"画什么"，`IconSource` 承载"谁决定的、能不能被刷新覆盖"。两者必须组合解读，单独看 `Icon` 的值无法判断来源。

### 1.2 三态语义

| IconSource | 含义 | RefreshFeed 是否覆盖 | 典型 Icon 值 |
|------------|------|---------------------|--------------|
| `fallback` | 系统兜底，尚未成功获取图标 | ✅ 尝试升级到 auto | `"mdi:rss"` |
| `auto` | 系统自动抓取的（RSS image / 站点 favicon） | ✅ 可刷新换更优图 | `http(s)://...` URL |
| `custom` | 用户显式设定，系统主权 | ❌ 永不覆盖 | 用户填的 iconify id 或 URL |

### 1.3 状态流转图

```
                         创建 Feed (API / OPML)
                                  │
                                  ▼
                    ┌─────────────────────────────┐
                    │  IconSource = fallback       │
                    │  Icon = "mdi:rss"            │
                    └──────────────┬──────────────┘
                                   │
                      RefreshFeed (每次刷新)
                                   │
                    ┌──────────────▼──────────────┐
                    │ IconSource in (auto,fallback)?│
                    │  YES → 重算选图              │
                    │  NO  (custom) → 跳过         │
                    └──────────────┬──────────────┘
                                   │ (auto/fallback 分支)
                    ┌──────────────▼──────────────┐
                    │  优先级选图：                │
                    │   1. parsed.Image (RSS<image>)│
                    │   2. siteFavicon (channel link│
                    │      → /favicon.ico)         │
                    └───┬───────────────────┬──────┘
                        │                   │
                   取到图 URL          两个都失败
                        │                   │
                        ▼                   ▼
              IconSource = auto    IconSource = fallback
              Icon = URL           Icon = "mdi:rss"
                        │                   │
                        │  ◀── 可重复刷新 ──┘
                        │     (下次刷新尝试换更优)
                        │
                        │      用户编辑 icon (UpdateFeed)
                        │             │
                        │             ▼
                        │   ┌─────────────────────┐
                        └──▶│ IconSource = custom  │ ★冻结
                            │ Icon = 用户填的值    │ 永不被 RefreshFeed 覆盖
                            └─────────────────────┘
```

### 1.4 关键不变式（Invariants）

- **I1**：`IconSource == "custom"` 时，`RefreshFeed` 不得修改 `feeds.icon` 和 `feeds.icon_source`。
- **I2**：`IconSource == "fallback"` 时，`Icon` 应为 `"mdi:rss"`（系统兜底图标）。
- **I3**：`IconSource == "auto"` 时，`Icon` 应为 `http(s)://` 开头的图片 URL。
- **I4**：用户通过 `UpdateFeed` 设置 icon 时，`IconSource` 必须置为 `custom`，无论原值是什么。

---

## 2. favicon 获取重写

### 2.1 现状问题

```go
// rss_parser.go:209-217 (现状，有问题)
func (p *RSSParser) FetchFaviconURL(feedURL string) string {
    parsedURL, _ := url.Parse(feedURL)  // feedURL 是 RSS 地址！
    return fmt.Sprintf("https://www.google.com/s2/favicons?domain=%s&sz=32", parsedURL.Host)
}
```

- `feedURL` 是 RSS feed 地址，host 常为 `feedburner.com` / `rsshub.app` → 取到聚合器 favicon，非内容站的。
- Google s2 在国内被墙 → 100% 加载失败。

### 2.2 新方案：RSS channel link → /favicon.ico

```go
// 新签名：接收站点首页 URL（来自 ParsedFeed.Link，即 RSS channel <link>）
func (p *RSSParser) FetchFaviconURL(siteURL string) string {
    parsedURL, err := url.Parse(siteURL)
    if err != nil || parsedURL.Host == "" {
        return ""  // 解析失败返回空，由调用方保持 fallback 态
    }
    return fmt.Sprintf("%s://%s/favicon.ico", parsedURL.Scheme, parsedURL.Host)
}
```

- **数据来源**：`ParsedFeed.Link`（`rss_parser.go:105` 已解析 RSS channel `<link>`，即站点首页）。已有数据，零额外请求。
- **不再依赖 Google s2**：直接用站点根目录 `/favicon.ico`，大部分网站都支持。
- **失败处理**：返回空字符串 → `RefreshFeed` 保持 `fallback` 态，下次刷新再试。前端 `<img @error>` 降级到 `mdi:rss`。

### 2.3 调用方改动

```go
// feed_service.go RefreshFeed 选图逻辑 (改造后)
if feed.IconSource == "auto" || feed.IconSource == "fallback" {
    switch {
    case parsed.Image != "":
        feed.Icon = parsed.Image
        feed.IconSource = "auto"
    case siteFavicon := s.rssParser.FetchFaviconURL(parsed.Link); siteFavicon != "":
        feed.Icon = siteFavicon
        feed.IconSource = "auto"
    default:
        feed.Icon = "mdi:rss"
        feed.IconSource = "fallback"
    }
}
// 移除: firstArticleImage 分支（文章图不再用作 feed icon）
```

注意：`parsed.Link` 可能为空（部分 RSS 不提供 channel link）。此时 `FetchFaviconURL` 返回空，feed 保持 `fallback` 态——这是可接受的降级，不是错误。

---

## 3. 存量数据迁移

### 3.1 迁移策略

GORM AutoMigrate 加 `icon_source` 列时，所有存量行 default 填 `fallback`。但存量行的 `icon` 值语义各异，需要一条幂等 SQL 精修 `icon_source`：

```sql
-- 幂等迁移：按现有 icon 值推断 icon_source
UPDATE feeds
SET icon_source = CASE
    WHEN icon IS NULL OR icon = '' OR icon = 'rss' OR icon = 'mdi:rss' THEN 'fallback'
    WHEN icon LIKE 'http://%' OR icon LIKE 'https://%' THEN 'auto'
    ELSE 'custom'  -- 其他 iconify id（如 mdi:xxx），保守归入 custom
END
WHERE icon_source = '' OR icon_source IS NULL;
```

### 3.2 迁移后清理（fallback 态的 icon 统一）

```sql
-- fallback 态的 icon 值统一为 mdi:rss（消除 "rss" / "" 占位符）
UPDATE feeds SET icon = 'mdi:rss'
WHERE icon_source = 'fallback' AND icon IN ('', 'rss');
```

### 3.3 幂等性

- 两条 SQL 都带 `WHERE` 条件，重复执行只会影响未迁移的行，安全。
- AutoMigrate 加列带 `default 'fallback'`，重跑不会报错（列已存在则跳过）。
- 回滚：`ALTER TABLE feeds DROP COLUMN icon_source;`

---

## 4. 前端降级修复

### 4.1 FeedIcon.vue @error 降级（核心修复）

```vue
<!-- 现状（有问题）：@error 直接 display:none，留白 -->
<img v-if="isUrl" :src="displayIcon" @error="(e) => { e.target.style.display = 'none' }">

<!-- 改造后：@error 切换到 Icon 组件渲染 mdi:rss -->
<script setup>
const imgFailed = ref(false)
</script>
<template>
  <img v-if="isUrl && !imgFailed" :src="displayIcon" @error="imgFailed = true">
  <Icon v-else :icon="(!isUrl ? displayIcon : 'mdi:rss') || 'mdi:rss'" ... />
</template>
```

### 4.2 移除前端重复的 favicon 拼接

现状 `FeedIcon.vue:47-57` 的 `getFaviconFromUrl`（前端自行拼 `/favicon.ico`）与后端 favicon 获取职责重叠。favicon 获取责任应统一归到后端状态机：
- `icon_source=auto` 且为 URL → 前端直接用，失败降级 mdi:rss。
- `icon_source=fallback` → 后端会在下次刷新尝试升级，前端只渲染 mdi:rss。
- 删除 `getFaviconFromUrl` 及 `fallbackIcon` 里基于 `articleLink`/`feedId` 拼 favicon 的分支。

### 4.3 侦探墙补渲染

`TimelineItem.vue` 和 `TopicGraphArticleCard.vue` 已通过 composable 拿到 `feedIcon`，但模板没渲染。补一个 `<FeedIcon :icon="item.feedIcon" :size="14" />` 即可。同时修正 `useTopicTimeline.ts:204` 的 `feedIcon: article.image_url`（把文章图当 feed icon 的语义错位）→ 改用后端返回的 `feed_icon` 字段。

---

## 5. icon 编辑 UI（可选，列为后续）

后端 `UpdateFeed` 已支持 `Icon` 字段更新，但缺少：
- `FeedDetailEditor.vue` 没有 icon 输入框
- 更新 icon 时未置 `IconSource=custom`

本 change 范围内**只做后端侧**（UpdateFeed 置 custom），前端编辑 UI 列为后续增量。理由：避免本 change 膨胀，且 custom 状态机即使没有编辑 UI，也能正确保护通过 API 直接设置的自定义 icon。

---

## 6. 决策记录

| 决策 | 选择 | 理由 |
|------|------|------|
| 状态机字段 | 新增 `IconSource`（方案B） | 显式状态优于前缀协议，custom 保护是硬契约而非碰巧 |
| favicon 来源 | RSS channel link → /favicon.ico | 已有数据零额外请求，不依赖被墙的 Google s2 |
| 文章图复用 | 不再用文章图当 feed icon | 文章封面 ≠ 站点 logo，语义错位 |
| 编辑 UI | 本 change 只做后端，UI 列后续 | 避免 change 膨胀，状态机本身独立成立 |
| 迁移方式 | AutoMigrate + 幂等 CASE SQL | 加列带 default 安全，CASE SQL 推断来源语义准确 |
