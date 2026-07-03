import type { DailyReportSection } from '~/api/dailyReports'
import type { TopicWatch, TopicWatchHit } from '~/api/topicWatches'

/** 每组命中默认展示前 N 条，超出折叠为「还有 N 条命中」。 */
export const WATCH_COLLAPSE_THRESHOLD = 2

/** 命中 + 解析出的 section 标题（标题来自已加载的日报 sections）。 */
export interface ResolvedWatchHit {
  hit: TopicWatchHit
  sectionTitle: string
}

/** 一个关注的命中分组：watch + 其命中的 section 列表（带标题）。 */
export interface WatchHitGroup {
  watch: TopicWatch
  items: ResolvedWatchHit[]
}

/** 从已加载的日报 sections 构建 sectionId → cluster_label 查找表。 */
export function buildSectionTitleLookup(sections: Pick<DailyReportSection, 'id' | 'cluster_label'>[]): Map<string, string> {
  const map = new Map<string, string>()
  for (const s of sections) {
    map.set(String(s.id), s.cluster_label ?? '')
  }
  return map
}

/**
 * 将命中按关注分组。组顺序遵循传入的 watches 顺序（= 后端创建顺序），
 * 仅产出命中数 ≥ 1 的关注；命中引用了不在列表中的关注（已删除/边界竞态）则丢弃。
 */
export function groupHitsByWatch(
  hits: TopicWatchHit[],
  watches: TopicWatch[],
  sectionTitleById: Map<string, string>,
): WatchHitGroup[] {
  const hitsByWatch = new Map<string, TopicWatchHit[]>()
  for (const h of hits) {
    const arr = hitsByWatch.get(h.watchId)
    if (arr) arr.push(h)
    else hitsByWatch.set(h.watchId, [h])
  }

  const groups: WatchHitGroup[] = []
  for (const w of watches) {
    const wh = hitsByWatch.get(w.id)
    if (!wh || wh.length === 0) continue
    groups.push({
      watch: w,
      // 防御性 String()：即便上游漏过未归一化的数字 id 也能命中标题表
      items: wh.map(hit => ({ hit, sectionTitle: sectionTitleById.get(String(hit.sectionId)) ?? '' })),
    })
  }
  return groups
}

/** 折叠提示文案。 */
export function formatMoreLabel(remaining: number): string {
  return `还有 ${remaining} 条命中 ↓`
}

/** 按状态切分关注：active 参与命中判定；paused 不参与（顶部栏只展示 active 命中）。 */
export function partitionByStatus(watches: TopicWatch[]): { active: TopicWatch[], paused: TopicWatch[] } {
  const active: TopicWatch[] = []
  const paused: TopicWatch[] = []
  for (const w of watches) {
    if (w.status === 'paused') paused.push(w)
    else active.push(w)
  }
  return { active, paused }
}
