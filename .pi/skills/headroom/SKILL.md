---
name: headroom
description: "Context compression tools — compress large tool outputs (logs, grep results, JSON, code) to save tokens. Use when a tool returns excessive output, before analyzing large datasets, or when context window pressure is high. Three tools: headroom_compress (shrink + store), headroom_retrieve (restore by hash, with optional keyword filter), headroom_stats (session compression summary)."
---

# Headroom — Context Compression

## When to Use

主动调用 `headroom_compress` 的场景：

| 触发条件 | 示例 |
|----------|------|
| 工具输出 > 500 行 | `grep` 匹配数百行、`find` 大量文件列表 |
| 单次输出 > 20KB | 长 JSON、大段日志、完整源码文件 |
| 需要保留但暂时不分析的输出 | 多个文件内容一次性读取后，压缩供后续按需检索 |
| 上下文窗口紧张时 | 会话较长，模型提示 token 超限 |

**不需要压缩**：少量文本（< 200 行）会被 noop 跳过，直接返回原内容不消耗额外 token。

---

## 三个工具

### 1. `headroom_compress` — 压缩并存储

```
输入: content（字符串，通常是上一个工具的输出）
输出: 压缩后的文本 + 唯一 hash
```

压缩后文本可直接阅读分析，丢失的细节用 hash 取回。

**基本用法**：工具返回大量输出后，立即调用 compress。

```
你: grep "error" logs/*.log
→ 输出 800 行

你: headroom_compress(content="以上 grep 的输出")
→ 返回压缩摘要 + hash="abc123"
→ 基于压缩摘要分析，找到关键线索
```

### 2. `headroom_retrieve` — 按需取回原始内容

```
输入: hash（来自 compress 的返回值）
可选: query（关键词过滤，只返回匹配的条目）
输出: 原始未压缩内容
```

**典型流程**：

```
1. 工具输出过大 → compress → 得到 hash
2. 分析压缩摘要 → 发现可疑点
3. retrieve(hash, query="error 500") → 只取回包含 "error 500" 的行
4. 精确定位后修复
```

`query` 过滤可以大幅减少取回的数据量——不需要把整个原始输出再放回上下文。

### 3. `headroom_stats` — 查看压缩统计

```
输出: 本次会话的压缩次数、节省 token 数、成本估算
```

用于事后了解压缩效果，或在会话结束前确认资源使用。

---

## 典型工作流

### 模式 A：大输出 → 压缩 → 分析 → 按需取回

```
Step 1: bash("grep -r 'TODO' front/")
        → 输出 1200 行，超出合理范围
Step 2: headroom_compress(content="<grep 输出>")
        → 压缩为结构化摘要，hash="xyz789"
Step 3: 分析摘要，发现 front/app/pages/ 下 TODO 最多
Step 4: headroom_retrieve(hash="xyz789", query="pages/")
        → 只取回 pages/ 相关的 TODO 条目
Step 5: 根据取回的具体条目进行修改
```

### 模式 B：批量读取 → 压缩 → 后续检索

```
Step 1: 一次 read 了 5 个大型文件
Step 2: headroom_compress(content="<5 个文件内容>")
        → 一份压缩摘要 + hash
Step 3: 需要某个文件的具体实现时
        headroom_retrieve(hash, query="文件名或函数名")
```

### 模式 C：预防性压缩（上下文紧张时）

```
会话已进行 30+ 轮，模型开始警告 context length。
此时获取任何新数据都先 compress，避免雪上加霜。
```

---

## 注意事项

- **压缩后立即分析**：压缩摘要通常足够定位问题，不需要每次都 retrieve
- **hash 是临时的**：仅在当前会话有效，不要跨会话使用
- **short content noop**：内容过短时 compress 会原样返回（不消耗 hash 配额），可以放心调用
- **不要压缩后再压缩**：已压缩内容再次压缩会丢失信息
- **`headroom_headroom_compress` vs `headroom_compress`**：pi 中两种写法等价。MCP 配置了 `directTools: true`，直接写 `headroom_compress` 即可
