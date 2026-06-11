import { ref } from 'vue'
import { usePreferencesStore } from '~/stores/preferences'
import type { ReadingStats, UserPreference } from '~/types/reading_behavior'

export function useReadingPreferences() {
  const preferenceType = ref<'feed' | 'category'>('feed')
  const readingStats = ref<ReadingStats | null>(null)
  const userPreferences = ref<UserPreference[]>([])
  const preferencesLoading = ref(false)
  const preferencesUpdating = ref(false)
  const preferencesError = ref<string | null>(null)

  const preferencesStore = usePreferencesStore()

  async function loadPreferencesData() {
    preferencesLoading.value = true
    preferencesError.value = null
    try {
      await Promise.all([
        preferencesStore.fetchStats(),
        preferencesStore.fetchPreferences(preferenceType.value),
      ])
      readingStats.value = preferencesStore.stats
      userPreferences.value = preferencesStore.preferences.filter(
        preference => Boolean(preference.feed_title || preference.category_name),
      )
    } catch {
      preferencesError.value = '加载阅读偏好失败'
    } finally {
      preferencesLoading.value = false
    }
  }

  async function triggerPreferenceUpdate() {
    preferencesUpdating.value = true
    preferencesError.value = null
    try {
      await preferencesStore.triggerUpdate()
      await loadPreferencesData()
    } catch {
      preferencesError.value = '触发更新失败'
    } finally {
      preferencesUpdating.value = false
    }
  }

  return {
    preferenceType, readingStats, userPreferences,
    preferencesLoading, preferencesUpdating, preferencesError,
    loadPreferencesData, triggerPreferenceUpdate,
  }
}
