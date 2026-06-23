# Issue 02: 辅助标签聚类自连接 — Go 侧并发点积 + 内存缓存

> **Status:** done
> **Priority:** high
> **Component:** backend-go/internal/tagmanagement/handler

## 问题描述

`GET /api/auxiliary-labels/clusters`（`semanticBoardHandler.clusterAuxiliaryLabels`）
内嵌一条**全量两两余弦距离自连接**：

```sql
SELECT a.id AS id1, b.id AS id2, a.embedding <=> b.embedding AS distance
FROM semantic_labels a
JOIN semantic_labels b ON a.id < b.id
WHERE a.label_type = 'auxiliary' AND a.status = 'active' AND a.embedding IS NOT NULL
  AND b.label_type = 'auxiliary' AND b.status = 'active' AND b.embedding IS NOT NULL
  AND a.embedding <=> b.embedding < 0.2
```

实测 **1h35m20s**（5715794ms），返回 37227 对。

## 根因分析

与 issue 01 同源，都是 **pgvector 无法为 vector(2560) 建 ANN 索引**（`vector` 类型 HNSW ≤2000 维，
`halfvec` 表达式索引查询优化器不识别）。但本查询的暴露方式不同：

1. **O(N²) 无界自连接**：N=10015 active auxiliary → 约 **5000 万次**两两距离计算。
   issue 01 的 `sqlMergeMatcher` 是「1 vs N」单边查询，这里是「N vs N」全连接，规模完全不同级。
2. **无任何过滤缩小范围**：不像 `FindSimilarTagsAmongSet` 带 `WHERE id IN (...)` 小集合，
   本查询对全部 active auxiliary 笼统自连接，DB 侧无法走索引、无法提前剪枝。
3. **EXPLAIN 证实纯嵌套循环全表扫描**：
   ```
   Nested Loop (cost=0.00..2162255.59 rows=11812969)
     Join Filter: ((a.id < b.id) AND (a.embedding <=> b.embedding < 0.2))
     -> Seq Scan on semantic_labels a (rows=10311)
     -> Materialize -> Seq Scan on semantic_labels b (rows=10311)
   ```
4. **为何此前未暴露**：其它走 `<=>` 的查询（`FindSimilarTags`、`FindSimilarTagsAmongSet`）
   都带 KNN/小集合过滤，单边比较次数少；本查询是项目里**唯一的全量两两自连接**。

95 分钟 / 5000 万次 ≈ 8800 次/秒，符合「无索引、JIT 单线程逐元素算」的量级。

## 修复方案（已完成）

延续 issue 01 既定的「Go 侧 cosine 计算」路线，延伸到这个遗漏的 handler。

### 1. 一次查询拉数据 + float32 预归一化

`SELECT id, label, slug, ref_count, embedding` 一次拉全；解析为 `[]float32` 连续 buffer，
L2 归一化后余弦距离 = `1 - 点积`。复用 `service.ParsePgVector`（与 `sqlMergeMatcher` 同一解析路径）。

### 2. 并发点积建图

`runtime.NumCPU()` 个 worker 分片计算 `i<j` 的点积，`dot > 1-0.2` 视为邻接边，
最后 BFS 求连通分量。聚类语义、排序、`unclustered_count` 与原实现完全一致。

### 3. 10 分钟内存缓存

只读聚合 + embedding 变更稀疏，结果带 TTL 缓存；`?refresh=true` 强制重算。
双锁（RLock 命中 / 独立 computeMu）保证并发请求复用同一次计算，不重复 O(N²)。

## 验证

| 项目 | 值 |
|------|------|
| 规模 | 10015 active auxiliary × vector(2560) |
| benchmark（纯计算） | **10.8s**（16 线程，AMD Ryzen 7 9700X） |
| 首次请求（拉取+解析+计算） | ~15-20s |
| 缓存命中 | 毫秒级 |
| 提速倍数 | 相比 95 分钟 ≈ **528×** |

- `go fmt` / `go vet ./internal/tagmanagement/handler/` / `go build ./...` 通过
- `go test ./internal/tagmanagement/handler/` 通过
- 该函数原先无单测，本次亦未引入回归

## 影响范围

- `tagmanagement/handler/board_crud_handler.go` — `clusterAuxiliaryLabels` 重写；
  新增 `normalizeAuxEmbeddings` / `clusterAuxLabels` 及包级缓存变量
- 未触碰数据层：不建索引、不改列类型、不动迁移（与 issue 01 结论一致，2560 维下索引无效）

## 后续事项

若要求「首次也秒级」，仍需 issue 01 提到的长期方案之一：

- 降维到 ≤2000 维 + HNSW（Matryoshka / PCA）
- 引入专用向量库（Milvus / Qdrant / ES kNN）

当前方案对「低频只读聚合 + 缓存」场景已足够。
