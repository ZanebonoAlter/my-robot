import type { SchedulerArticleRef, SchedulerStatus } from '~/types/scheduler'

const contentCompletionAliases = new Set(['content_completion', 'ai_summary'])

type ContentCompletionPanelStatus = Pick<SchedulerStatus, 'name' | 'overview' | 'is_executing' | 'current_article'>
type SchedulerStatusLike = Pick<SchedulerStatus, 'name' | 'status' | 'is_executing' | 'database_state'>
type ContentCompletionArticleStatus = Pick<SchedulerStatus, 'name' | 'is_executing' | 'current_article' | 'stale_processing_article' | 'last_run_summary'>

export function isContentCompletionScheduler(name: string): boolean {
  return contentCompletionAliases.has(name)
}

export function isHotScheduler(name: string): boolean {
  return name === 'auto_refresh' || isContentCompletionScheduler(name) || name === 'firecrawl'
}

export function getSchedulerDisplayName(name: string): string {
  if (isContentCompletionScheduler(name)) {
    return '文章总结'
  }

  const names: Record<string, string> = {
    'auto_refresh': '后台刷新',
    'firecrawl': '全文爬取',
    'tag_hierarchy_cleanup': '标签清理',
    'aux_label_cleanup': '辅助标签清理',
    'preference_update': '阅读偏好更新',
    'tag_quality_score': '标签质量评分',
    'log_cleanup': '日志清理',
    'daily_report': '每日报告',
    'blocked_article_recovery': '阻塞文章恢复',
  }

  return names[name] || name
}

export function getSchedulerIcon(name: string): string {
  if (isContentCompletionScheduler(name)) {
    return 'mdi:text-box-search-outline'
  }

  const icons: Record<string, string> = {
    'auto_refresh': 'mdi:refresh',
    'firecrawl': 'mdi:spider-web',
    'tag_hierarchy_cleanup': 'mdi:tag-remove-outline',
    'aux_label_cleanup': 'mdi:tag-minus-outline',
    'preference_update': 'mdi:account-cog-outline',
    'tag_quality_score': 'mdi:tag-star-outline',
    'log_cleanup': 'mdi:broom',
    'daily_report': 'mdi:newspaper-variant-outline',
    'blocked_article_recovery': 'mdi:file-restore-outline',
  }

  return icons[name] || 'mdi:cog'
}

export function getSchedulerColor(name: string): string {
  if (isContentCompletionScheduler(name)) {
    return 'from-amber-500 to-orange-500'
  }

  const colors: Record<string, string> = {
    'auto_refresh': 'from-blue-500 to-cyan-500',
    'firecrawl': 'from-rose-500 to-orange-500',
    'tag_hierarchy_cleanup': 'from-violet-500 to-purple-600',
    'aux_label_cleanup': 'from-teal-500 to-emerald-500',
    'preference_update': 'from-indigo-500 to-blue-500',
    'tag_quality_score': 'from-purple-500 to-pink-500',
    'log_cleanup': 'from-slate-500 to-gray-500',
    'daily_report': 'from-amber-500 to-yellow-500',
    'blocked_article_recovery': 'from-orange-500 to-red-500',
  }

  return colors[name] || 'from-gray-500 to-gray-600'
}

export function shouldShowContentCompletionPanel(scheduler: ContentCompletionPanelStatus): boolean {
  return isContentCompletionScheduler(scheduler.name)
    && Boolean(scheduler.overview || scheduler.is_executing || scheduler.current_article)
}

export function getSchedulerStatusLabel(scheduler: SchedulerStatusLike): string | undefined {
  if (isContentCompletionScheduler(scheduler.name) && scheduler.is_executing !== true && scheduler.status) {
    return mapStatusToChinese(scheduler.status)
  }

  const raw = scheduler.database_state?.status || scheduler.status
  return raw ? mapStatusToChinese(raw) : raw
}

export function mapStatusToChinese(status: string): string {
  const map: Record<string, string> = {
    'idle': '空闲',
    'running': '执行中',
    'error': '失败',
    'triggered': '已触发',
    'stopped': '已停用',
    'disabled': '已停用',
    'failed': '失败',
  }
  return map[status] || status
}

export function getCurrentContentCompletionArticle(scheduler: ContentCompletionArticleStatus): SchedulerArticleRef | null | undefined {
  if (scheduler.current_article) {
    return scheduler.current_article
  }

  if (isContentCompletionScheduler(scheduler.name) && scheduler.is_executing !== true) {
    return scheduler.stale_processing_article || scheduler.last_run_summary?.stale_processing_article || null
  }

  return null
}
