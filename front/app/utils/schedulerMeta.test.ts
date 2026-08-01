import { describe, expect, it } from 'vitest'
import type { SchedulerTask } from '~/types/scheduler'

import {
  getCurrentContentCompletionArticle,
  getSchedulerColor,
  getSchedulerDisplayName,
  getSchedulerIcon,
  getSchedulerStatusLabel,
  isContentCompletionScheduler,
  isHotScheduler,
  mapStatusToChinese,
  shouldShowContentCompletionPanel,
} from './schedulerMeta'

describe('schedulerMeta', () => {
  it('treats content_completion as the canonical article completion scheduler', () => {
    expect(isContentCompletionScheduler('content_completion')).toBe(true)
    expect(getSchedulerDisplayName('content_completion')).toBe('文章总结')
    expect(getSchedulerIcon('content_completion')).toBe('mdi:text-box-search-outline')
    expect(getSchedulerColor('content_completion')).toBe('from-amber-500 to-orange-500')
  })

  it('keeps ai_summary as a backward-compatible alias', () => {
    expect(isContentCompletionScheduler('ai_summary')).toBe(true)
    expect(getSchedulerDisplayName('ai_summary')).toBe('文章总结')
  })

  it('marks content completion as a hot scheduler for polling', () => {
    expect(isHotScheduler('content_completion')).toBe(true)
    expect(isHotScheduler('firecrawl')).toBe(true)
  })

  it('shows content completion panel while a current article is executing even without overview', () => {
    expect(shouldShowContentCompletionPanel({
      name: 'content_completion',
      is_executing: true,
      current_article: {
        id: 1,
        feed_id: 2,
        title: '正在补全的文章',
      },
    })).toBe(true)
  })

  it('prefers runtime idle over stale database running status for content completion', () => {
    expect(getSchedulerStatusLabel({
      name: 'content_completion',
      status: 'idle',
      is_executing: false,
      database_state: {
        status: 'running',
      } as SchedulerTask,
    })).toBe('空闲')
  })

  it('falls back to stale processing article when no live current article exists', () => {
    expect(getCurrentContentCompletionArticle({
      name: 'content_completion',
      is_executing: false,
      current_article: null,
      stale_processing_article: {
        id: 44471,
        feed_id: 25,
        title: '遗留 pending 文章',
      },
    })?.title).toBe('遗留 pending 文章')
  })

  it('maps newly added scheduler names to Chinese display names', () => {
    expect(getSchedulerDisplayName('preference_profile_update')).toBe('兴趣画像重算')
    expect(getSchedulerDisplayName('rsshub_catalog_sync')).toBe('订阅源目录同步')
    expect(getSchedulerDisplayName('tag_quality_score')).toBe('标签质量评分')
    expect(getSchedulerDisplayName('log_cleanup')).toBe('日志清理')
    expect(getSchedulerDisplayName('daily_report')).toBe('每日报告')
    expect(getSchedulerDisplayName('blocked_article_recovery')).toBe('阻塞文章恢复')
  })

  it('maps scheduler status values to Chinese', () => {
    expect(mapStatusToChinese('idle')).toBe('空闲')
    expect(mapStatusToChinese('running')).toBe('执行中')
    expect(mapStatusToChinese('error')).toBe('失败')
    expect(mapStatusToChinese('triggered')).toBe('已触发')
    expect(mapStatusToChinese('stopped')).toBe('已停用')
    expect(mapStatusToChinese('disabled')).toBe('已停用')
    expect(mapStatusToChinese('failed')).toBe('失败')
    expect(mapStatusToChinese('unknown')).toBe('unknown')
  })

  it('returns Chinese status label for non-content-completion schedulers', () => {
    expect(getSchedulerStatusLabel({
      name: 'auto_refresh',
      status: 'idle',
      is_executing: false,
      database_state: undefined as any,
    })).toBe('空闲')

    expect(getSchedulerStatusLabel({
      name: 'auto_refresh',
      status: 'running',
      is_executing: true,
      database_state: { status: 'running' } as any,
    })).toBe('执行中')
  })
})
