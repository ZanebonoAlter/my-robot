<script setup lang="ts">
import { Icon } from '@iconify/vue'
import { onMounted, watch } from 'vue'
import { useReadingPreferences } from '~/composables/useReadingPreferences'

const {
  preferenceType, readingStats, userPreferences,
  preferencesLoading, preferencesUpdating, preferencesError,
  loadPreferencesData, triggerPreferenceUpdate,
} = useReadingPreferences()

watch(preferenceType, () => {
  loadPreferencesData()
})

onMounted(() => {
  loadPreferencesData()
})

function getScoreColor(score: number): string {
  if (score >= 0.7) return 'var(--color-success)'
  if (score >= 0.4) return 'var(--color-warning)'
  return 'var(--color-text-muted)'
}
</script>

<template>
  <div class="space-y-6">
    <div v-if="preferencesError" class="p-3 rounded-lg text-sm" style="background: var(--color-error-bg, rgba(196, 47, 60, 0.1)); border: 1px solid var(--color-error-border, rgba(196, 47, 60, 0.25)); color: var(--color-error)">
      {{ preferencesError }}
    </div>

    <div v-if="preferencesLoading" class="flex items-center justify-center py-12">
      <div class="text-center">
        <Icon icon="mdi:loading" width="48" height="48" class="animate-spin mx-auto mb-3" style="color: var(--color-link)" />
        <p class="text-sm" style="color: var(--color-text-muted)">加载偏好数据...</p>
      </div>
    </div>

    <template v-else>
      <!-- Stats -->
      <div class="grid grid-cols-3 gap-4" v-if="readingStats">
        <div class="rounded-xl p-4 text-center" style="background: var(--color-bg-sunken)">
          <p class="text-2xl font-bold" style="color: var(--color-link)">{{ readingStats.total_articles }}</p>
          <p class="text-xs mt-1" style="color: var(--color-text-muted)">总阅读量</p>
        </div>
        <div class="rounded-xl p-4 text-center" style="background: var(--color-bg-sunken)">
          <p class="text-2xl font-bold" style="color: var(--color-success)">{{ readingStats.read_ratio ? (readingStats.read_ratio * 100).toFixed(0) : 0 }}%</p>
          <p class="text-xs mt-1" style="color: var(--color-text-muted)">已读率</p>
        </div>
        <div class="rounded-xl p-4 text-center" style="background: var(--color-bg-sunken)">
          <p class="text-2xl font-bold" style="color: var(--color-warning)">{{ readingStats.fav_ratio ? (readingStats.fav_ratio * 100).toFixed(0) : 0 }}%</p>
          <p class="text-xs mt-1" style="color: var(--color-text-muted)">收藏率</p>
        </div>
      </div>

      <!-- Filter Toggle -->
      <div class="flex items-center gap-4">
        <span class="text-sm font-medium" style="color: var(--color-text-primary)">偏好范围：</span>
        <div class="flex rounded-lg p-0.5" style="background: var(--color-bg-sunken)">
          <button
            type="button"
            class="px-3 py-1.5 text-sm rounded-md transition-colors"
            :style="preferenceType === 'feed' ? 'background: var(--color-bg-elevated); box-shadow: var(--shadow-subtle); color: var(--color-text-primary); font-weight: 500' : 'color: var(--color-text-muted)'"
            @click="preferenceType = 'feed'"
          >
            按订阅源
          </button>
          <button
            type="button"
            class="px-3 py-1.5 text-sm rounded-md transition-colors"
            :style="preferenceType === 'category' ? 'background: var(--color-bg-elevated); box-shadow: var(--shadow-subtle); color: var(--color-text-primary); font-weight: 500' : 'color: var(--color-text-muted)'"
            @click="preferenceType = 'category'"
          >
            按分类
          </button>
        </div>
        <button
          type="button"
          class="ml-auto px-3 py-1.5 text-sm text-white rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-1.5"
          style="background: var(--color-accent)"
          :disabled="preferencesUpdating"
          @click="triggerPreferenceUpdate"
        >
          <Icon v-if="preferencesUpdating" icon="mdi:loading" width="14" class="animate-spin" />
          重新计算偏好
        </button>
      </div>

      <!-- Preferences List -->
      <div v-if="userPreferences.length > 0" class="space-y-2">
        <div
          v-for="pref in userPreferences"
          :key="pref.preference_id || pref.feed_id || pref.category_id"
          class="flex items-center gap-4 p-3 rounded-xl"
          style="background: var(--color-bg-sunken)"
        >
          <div class="flex-1 min-w-0">
            <p class="text-sm font-medium truncate" style="color: var(--color-text-primary)">
              {{ pref.feed_title || pref.category_name || '未知' }}
            </p>
            <p class="text-xs" style="color: var(--color-text-muted)">{{ pref.feed_title ? '订阅源' : '分类' }}</p>
          </div>
          <div class="flex items-center gap-3 text-sm">
            <span style="color: var(--color-text-muted)">阅读分</span>
            <div class="w-20 h-2 rounded-full overflow-hidden" style="background: var(--color-border-medium)">
              <div
                class="h-full rounded-full transition-all"
                :style="{ width: `${Math.min((pref.read_score ?? 0) * 100, 100)}%`, background: getScoreColor(pref.read_score ?? 0) }"
              />
            </div>
            <span class="w-8 text-right font-mono text-xs" style="color: var(--color-text-secondary)">{{ ((pref.read_score ?? 0) * 100).toFixed(0) }}</span>
          </div>
          <div class="flex items-center gap-3 text-sm">
            <span style="color: var(--color-text-muted)">兴趣分</span>
            <div class="w-24 h-2 rounded-full overflow-hidden" style="background: var(--color-border-medium)">
              <div
                class="h-full rounded-full transition-all"
                :style="{ width: `${Math.min((pref.interest_score ?? 0) * 100, 100)}%`, background: getScoreColor(pref.interest_score ?? 0) }"
              />
            </div>
            <span class="w-8 text-right font-mono text-xs" style="color: var(--color-text-secondary)">{{ ((pref.interest_score ?? 0) * 100).toFixed(0) }}</span>
          </div>
        </div>
      </div>

      <div v-else class="text-center py-12">
        <Icon icon="mdi:book-open-outline" width="48" height="48" class="mx-auto" style="color: var(--color-text-muted)" />
        <p class="mt-3 text-sm" style="color: var(--color-text-muted)">暂无偏好数据，多读些文章再回来看看吧</p>
      </div>
    </template>
  </div>
</template>
