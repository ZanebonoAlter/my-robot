import type { InjectionKey } from 'vue'

/**
 * 版块级关注管理面板的打开句柄（TagsPage provide → 日报栏等 descendant inject）。
 *
 * - TagsPage（唯一 provider）：`provide(WATCH_PANEL_KEY, openWatchPanel)`
 * - 日报追踪索引：详情页只读使用，不注入管理能力
 *   （如单测 / 非工作台挂载）时降级 no-op，组件不炸。
 */
export const WATCH_PANEL_KEY: InjectionKey<() => void> = Symbol('openWatchPanel')
