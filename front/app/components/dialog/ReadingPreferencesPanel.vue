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
  if (score >= 0.7) return 'bg-green-500'
  if (score >= 0.4) return 'bg-yellow-500'
  return 'bg-gray-400'
}
</script>

<template>
  <div class="space-y-6">
    <div v-if="preferencesError" class="p-3 bg-red-50 border border-red-200 rounded-lg text-sm text-red-600">
      {{ preferencesError }}
    </div>

    <div v-if="preferencesLoading" class="flex items-center justify-center py-12">
      <div class="text-center">
        <Icon icon="mdi:loading" width="48" height="48" class="animate-spin text-blue-500 mx-auto mb-3" />
        <p class="text-sm text-gray-500">加载偏好数据...</p>
      </div>
    </div>

    <template v-else>
      <!-- Stats -->
      <div class="grid grid-cols-3 gap-4" v-if="readingStats">
        <div class="bg-gradient-to-br from-blue-50 to-indigo-50 rounded-xl p-4 text-center">
          <p class="text-2xl font-bold text-blue-600">{{ readingStats.total_articles }}</p>
          <p class="text-xs text-gray-500 mt-1">总阅读量</p>
        </div>
        <div class="bg-gradient-to-br from-green-50 to-emerald-50 rounded-xl p-4 text-center">
          <p class="text-2xl font-bold text-green-600">{{ readingStats.read_ratio ? (readingStats.read_ratio * 100).toFixed(0) : 0 }}%</p>
          <p class="text-xs text-gray-500 mt-1">已读率</p>
        </div>
        <div class="bg-gradient-to-br from-amber-50 to-orange-50 rounded-xl p-4 text-center">
          <p class="text-2xl font-bold text-amber-600">{{ readingStats.fav_ratio ? (readingStats.fav_ratio * 100).toFixed(0) : 0 }}%</p>
          <p class="text-xs text-gray-500 mt-1">收藏率</p>
        </div>
      </div>

      <!-- Filter Toggle -->
      <div class="flex items-center gap-4">
        <span class="text-sm font-medium text-gray-700">偏好范围：</span>
        <div class="flex bg-gray-100 rounded-lg p-0.5">
          <button
            type="button"
            class="px-3 py-1.5 text-sm rounded-md transition-colors"
            :class="preferenceType === 'feed' ? 'bg-white shadow-sm text-gray-800 font-medium' : 'text-gray-500 hover:text-gray-700'"
            @click="preferenceType = 'feed'"
          >
            按订阅源
          </button>
          <button
            type="button"
            class="px-3 py-1.5 text-sm rounded-md transition-colors"
            :class="preferenceType === 'category' ? 'bg-white shadow-sm text-gray-800 font-medium' : 'text-gray-500 hover:text-gray-700'"
            @click="preferenceType = 'category'"
          >
            按分类
          </button>
        </div>
        <button
          type="button"
          class="ml-auto px-3 py-1.5 text-sm bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-1.5"
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
          class="flex items-center gap-4 p-3 bg-gray-50 rounded-xl"
        >
          <div class="flex-1 min-w-0">
            <p class="text-sm font-medium text-gray-800 truncate">
              {{ pref.feed_title || pref.category_name || '未知' }}
            </p>
            <p class="text-xs text-gray-500">{{ pref.feed_title ? '订阅源' : '分类' }}</p>
          </div>
          <div class="flex items-center gap-3 text-sm">
            <span class="text-gray-500">阅读分</span>
            <div class="w-20 h-2 bg-gray-200 rounded-full overflow-hidden">
              <div
                class="h-full rounded-full transition-all"
                :class="getScoreColor(pref.read_score ?? 0)"
                :style="{ width: `${Math.min((pref.read_score ?? 0) * 100, 100)}%` }"
              />
            </div>
            <span class="w-8 text-right font-mono text-xs text-gray-600">{{ ((pref.read_score ?? 0) * 100).toFixed(0) }}</span>
          </div>
          <div class="flex items-center gap-3 text-sm">
            <span class="text-gray-500">兴趣分</span>
            <div class="w-24 h-2 bg-gray-200 rounded-full overflow-hidden">
              <div
                class="h-full rounded-full transition-all"
                :class="getScoreColor(pref.interest_score ?? 0)"
                :style="{ width: `${Math.min((pref.interest_score ?? 0) * 100, 100)}%` }"
              />
            </div>
            <span class="w-8 text-right font-mono text-xs text-gray-600">{{ ((pref.interest_score ?? 0) * 100).toFixed(0) }}</span>
          </div>
        </div>
      </div>

      <div v-else class="text-center py-12">
        <Icon icon="mdi:book-open-outline" width="48" height="48" class="mx-auto text-gray-300" />
        <p class="mt-3 text-sm text-gray-500">暂无偏好数据，多读些文章再回来看看吧</p>
      </div>
    </template>
  </div>
</template>
