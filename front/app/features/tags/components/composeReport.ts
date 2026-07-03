/**
 * 手动建泳道编排态纯函数（切片③ task 3.6）。
 *
 * 镜像后端 aggregateEmbeddings / detectOutliers 的语义（mean pooling + 距离 >
 * threshold×1.3 标离群），让前端实时算"section 到聚合锚点距离/离群/聚类质量"，
 * 不在每次勾选时往返后端。与 Vue 响应式解耦，单独单测覆盖（见
 * composeReport.test.ts）。
 *
 * 设计依据：design §4.4「离群只标黄、不自动删」、§4.2「embedding 来自选中
 * section 聚合（mean pooling）」、spec Scenario「预览泳道实时反映勾选」。
 */
import type { ComposeCandidate } from '~/api/persistentTopics'

/** cosine 距离 = 1 − cosine 相似度；维度不匹配或零向量返回 +∞（视作非匹配）。
 *  与后端 cosineDistance 对齐。 */
export function cosineDistance(a: number[], b: number[]): number {
  if (!a || !b || a.length === 0 || b.length === 0 || a.length !== b.length) {
    return Number.POSITIVE_INFINITY
  }
  let dot = 0, na = 0, nb = 0
  for (let i = 0; i < a.length; i++) {
    const av = a[i]!, bv = b[i]!
    dot += av * bv
    na += av * av
    nb += bv * bv
  }
  if (na === 0 || nb === 0) return Number.POSITIVE_INFINITY
  return 1 - dot / (Math.sqrt(na) * Math.sqrt(nb))
}

export interface AggregateResult {
  /** 选中向量的 mean（语义中心）。全空/全不可用时为 null。 */
  mean: number[] | null
  /** 维度不匹配或空向量被跳过的数量。 */
  skipped: number
}

/**
 * mean pooling 聚合：以第一个可用向量定维度，维度不一致或空向量跳过并计数。
 * 镜像后端 aggregateEmbeddings。空输入返回 { mean: null, skipped: 0 }。
 */
export function aggregatePreview(vectors: number[][]): AggregateResult {
  if (!vectors || vectors.length === 0) {
    return { mean: null, skipped: 0 }
  }
  let dim = 0
  let mean: number[] | null = null
  let usable = 0
  let skipped = 0
  for (const v of vectors) {
    if (!v || v.length === 0) { skipped++; continue }
    if (dim === 0) {
      dim = v.length
      mean = new Array<number>(dim).fill(0)
    }
    if (v.length !== dim) { skipped++; continue }
    for (let j = 0; j < dim; j++) mean![j]! += v[j]!
    usable++
  }
  if (usable === 0) return { mean: null, skipped }
  for (let j = 0; j < dim; j++) mean![j]! /= usable
  return { mean, skipped }
}

/**
 * 离群标记：distance > threshold×1.3 为离群。与后端 detectOutliers 对齐
 * （边界值不标：恰好等于 threshold×1.3 不算离群）。空输入返回 null。
 */
export function outlierFlags(distances: number[], threshold: number): boolean[] | null {
  if (!distances || distances.length === 0) return null
  const cutoff = threshold * 1.3
  return distances.map(d => d > cutoff)
}

export type DistanceTier = 'good' | 'boundary' | 'outlier' | 'far'

/**
 * 距离分级（候选池标签 + 预览节点三态共用）：
 *  - good（贴合）：d ≤ threshold
 *  - boundary（边界）：d ≤ threshold×1.3
 *  - outlier（离群）：d ≤ threshold×2
 *  - far（远）：d > threshold×2（语义偏离过大，mockup 中未勾选的远距离项）
 *
 * spec 明确定义"贴合/边界/离群"三态（distance×1.3 为离群门）；"远"是 mockup
 * 中更远的未选项的展示分级，不影响保存。
 */
export function distanceTier(distance: number, threshold: number): DistanceTier {
  if (distance <= threshold) return 'good'
  if (distance <= threshold * 1.3) return 'boundary'
  if (distance <= threshold * 2) return 'outlier'
  return 'far'
}

export const TIER_LABEL: Record<DistanceTier, string> = {
  good: '贴合',
  boundary: '边界',
  outlier: '离群',
  far: '远',
}

export interface CrashMoveOut {
  topicId: string
  label: string
  count: number
}

export interface CrashReport {
  /** 选中 section 当前归属的话题分布（含"未归属"）。 */
  distribution: CrashMoveOut[]
  /** 将从原话题移出的总数（即选中里有 persistentTopicId 的条数）。 */
  moveOutCount: number
  /** 按原话题分组的移出明细（不含"未归属"），用于"N 条将从『X』移出"提示。 */
  moveOutByTopic: CrashMoveOut[]
}

const UNASSIGNED = '__unassigned__'

/**
 * 撞车检查：聚合选中 section 的当前归属分布 + 移出明细。
 * - distribution：每个原话题（含未归属）→ 选中条数。
 * - moveOutByTopic：有归属的话题 → 将被覆盖移出的条数（单值覆盖语义，design §4.3）。
 * - moveOutCount：移出总数 = distribution 中非"未归属"之和。
 *
 * existingTopics 提供 label 解析（按 id）；缺失 id 用 `话题 #id` 兜底，不抛错。
 */
export function crashReport(
  selected: ComposeCandidate[],
  existingTopics: { id: string, label: string }[],
): CrashReport {
  const labelById = new Map<string, string>()
  for (const t of existingTopics) labelById.set(t.id, t.label)

  const dist = new Map<string, CrashMoveOut>()
  for (const s of selected) {
    const key = s.persistentTopicId ?? UNASSIGNED
    const label = key === UNASSIGNED ? '未归属' : (labelById.get(key) ?? `话题 #${key}`)
    const prev = dist.get(key)
    if (prev) prev.count++
    else dist.set(key, { topicId: key, label, count: 1 })
  }

  const distribution = [...dist.values()].sort((a, b) => b.count - a.count)
  const moveOutByTopic = distribution.filter(d => d.topicId !== UNASSIGNED)
  const moveOutCount = moveOutByTopic.reduce((sum, d) => sum + d.count, 0)

  return { distribution, moveOutCount, moveOutByTopic }
}

/**
 * 按时间范围窗口过滤候选池（前端兜底，与后端窗口语义一致）。
 * 锚定池中最新 section 的日期：days<=0 表示全部历史；否则保留
 * [maxDate − (days−1), maxDate]。输入不被修改（只读派生）。
 */
export function filterPoolByRange(sections: ComposeCandidate[], days: number): ComposeCandidate[] {
  if (!sections || sections.length === 0) return []
  if (days == null || days <= 0) return [...sections]
  let maxMs = -Infinity
  for (const s of sections) {
    const t = Date.parse(s.periodDate)
    if (!Number.isNaN(t) && t > maxMs) maxMs = t
  }
  if (maxMs === -Infinity) return [...sections]
  const cutoff = maxMs - (days - 1) * 86_400_000
  return sections.filter(s => {
    const t = Date.parse(s.periodDate)
    return !Number.isNaN(t) && t >= cutoff
  })
}

/**
 * 候选池语义排序（编排态渐进收敛，切片④ task 3.11）。
 *
 * 排序信号优先级（对应 spec「编排态候选池语义搜索」）：
 *  1. anchor（已选集合聚合向量 mean）非空 → 按到 anchor 的 cosine 距离升序。
 *     已选是用户确证的信号，优先级最高；勾选越多锚点越准（渐进收敛）。
 *  2. 否则若 queryVec（文本搜索向量）非空 → 按到 queryVec 距离升序（冷启动种子）。
 *  3. 都没有 → 保持原序（调用方负责默认序，如按 periodDate 倒序）。
 *
 * 距离非有限（维度不匹配/零向量 → cosineDistance 返回 +∞）的排到末尾；
 * 同距离项 stable sort 保持原序，避免抖动。不修改入参，返回新数组。
 *
 * 设计依据：design §4.2 + 用户拍板「已选聚合接管主排序，文本为冷启动种子」。
 */
export function rankCandidates(
  pool: ComposeCandidate[],
  anchor: number[] | null,
  queryVec: number[] | null,
): ComposeCandidate[] {
  const ref = anchor ?? queryVec
  if (!ref || ref.length === 0) return [...pool]
  return [...pool].sort((a, b) => {
    const da = cosineDistance(a.embedding, ref)
    const db = cosineDistance(b.embedding, ref)
    if (!Number.isFinite(da) && !Number.isFinite(db)) return 0
    if (!Number.isFinite(da)) return 1
    if (!Number.isFinite(db)) return -1
    return da - db
  })
}
