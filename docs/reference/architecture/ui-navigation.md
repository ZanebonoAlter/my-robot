# UI 导航地图（给 E2E / Playwright 用）

> 本文档**增量维护**：每验证过一个需要多步导航的界面链路，就把路径 + 选择器 + 断言锚点补进来。供 **DeepSeek v4 Flash 按需功能验证**（见 `.agents/skills/playwright-e2e/`）和 Playwright 导航复用，避免每次重新踩点。不沉淀固定回归脚本（脆弱）。
>
> 约定：URL 默认 `http://localhost:3000`；后端 `http://localhost:5000/api`。访问性以 Playwright `navigate` 实测为准（见 skill references/network-and-navigation.md）。

## 通用前置

| 步骤 | 操作 | 说明 |
|------|------|------|
| 0 | `navigate http://localhost:3000/<page>` | Nuxt SSR 只给壳，内容异步 hydrate |
| 0.5 | 关闭「叙事工坊」引导弹窗 | 首屏大概率弹出 dialog 挡住业务区，找 `aria-label="Close"` 或 × 按钮点掉，否则后续 `evaluate` 查不到业务 DOM |
| 0.6 | 每步导航后 `wait_for` 1~3s 或 `await setTimeout` | hydration / 数据加载是异步的 |

---

## 链路 1：板块日报 → 话题 → section → thread（thread-fit 等验证走这条）

**入口页**：`/tags`

### 1.1 选板块（侧栏）

- 侧栏容器：`aside.tags-sidebar` → `.sb-list`
- 板块项：`.sb-item`，选中态加 `.sb-item--active`
- 点击切板块：`document.querySelectorAll('.sb-item').find(i => /<板块名>/.test(i.textContent)).click()`
- 标签文本在 `.sb-item-label`

### 1.2 切到「日报」tab

- 主区 tab 按钮组（`main button`）："板块内容" / **"日报"** / "文章"
- 点日报：`Array.from(document.querySelectorAll('main button')).find(b => b.textContent.trim() === '日报').click()`

### 1.3 选日期打开日报详情

- 日报列表项是日期按钮，文本形如 `6 月 26 日 · 周五completed<当天摘要…>`
- 多个日期时按摘要匹配避免误点：`find(b => /6 月 26 日/.test(b.textContent) && /<摘要关键词>/.test(b.textContent))`
- 点击后进入日报详情视图（按"质量分区"组织）

### 1.4 日报详情视图结构（断言锚点）

```
.drm-zone[data-zone=...]            质量分区（active/briefs）
  header.drm-zone__header           "N 个话题"
  .drm-zone__topics
    article.drm-topic               话题分组
      button.drm-topic__header      可点击 toggle（aria-expanded）
        .drm-topic__heading strong  ← 话题标题（断言用它定位话题）
      .drm-topic__body              仅展开时存在
        .drm-topic__sections
          article.drm-section-card  一个 section（一个叙事簇）
            .drm-section-card__head （tier/anchor 徽章 + 可选标题）
            .drm-section-card__threads
              article.drm-thread    一条 thread（事件）
                button.drm-thread__header   toggle 关联文章（aria-expanded）
                  strong            ← thread 标题
                  .drm-thread__meta  右侧：离群标记 + 文章数 + 折叠箭头
                .drm-articles       仅展开时存在：关联文章 + .drm-thread__fit-probe
              button.drm-thread__hint  仅有离群 thread 时出现："另有 N 条可能跑题的线索"
```

**注意**：
- **默认只展开第一个话题**（`expandedTopics` watch 自动展开首个 active 话题）。含目标数据的话题多半折叠，要主动点 `.drm-topic__header` 展开。判断某话题是否展开：`!!topic.querySelector('.drm-topic__body')` 或 header 的 `aria-expanded`。
- **定位含特定数据的话题**：先查 API（见 skill references/assertion-recipe.md §2）拿到 `{board, date, section}`，再用话题标题文本 `.drm-topic__heading strong` 匹配展开。

### 1.5 关键状态标记（断言用）

| 标记 | 选择器 | 含义 |
|------|--------|------|
| thread 离群（软降级） | `.drm-thread--demoted` | fit_distance > 阈值（当前 0.28） |
| thread 内容已展开 | `.drm-thread` 内存在 `.drm-articles` | toggleThread 后出现 |
| 离群提示行 | `.drm-thread__hint` | section 有 ≥1 离群 thread 时渲染 |
| 贴合度探针 | `.drm-thread__fit-probe` | 展开后可见，含 fit_distance 数值 |

### 1.6 thread-fit 软降级验证要点

- **测试夹具**：当前全库离群 thread 集中在 2026-06-26，分布于「生成式 AI 与大模型厂商」「中国国内新闻」「虚拟现实相关新闻」「美国新闻」四个板块（全球科技巨头板块**无**离群，别拿它验证）。
- **断言**：点 `.drm-thread__hint` 前后对比，翻转的 thread 必须全是 `.drm-thread--demoted`、正常 thread 的展开态不变。脚本见 skill references/assertion-recipe.md §1。（注：设计 A 后 `.drm-thread__hint` 已改为不可点 `<p>` 状态说明，此断言仅供历史参考；当前验证重点是「图标左对齐」「状态说明不可点」。）
- **交互语义 caveat**（2026-06-27 评审 + 设计 A 修订）：初版 hint 是可点 button 批量展开关联文章，但「展开文章对提示跑题无意义」。设计 A 改为：跑题 thread 行保持可见（灰显），alert 图标移至标题左侧，hint 行降级为 `<p>` 纯状态说明。见 `standard/frontend/interaction-conventions.md` §1§2。

---

<!-- 新链路按上面格式增量追加 -->
