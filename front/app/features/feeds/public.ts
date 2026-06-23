/**
 * Feeds feature — public facade
 *
 * Cross-feature consumers should import stable feeds Interfaces from here,
 * not from internal composables directly.
 */

export { useGlobalAutoRefresh } from './composables/useAutoRefresh'
export { useRefreshPolling } from './composables/useRefreshPolling'
