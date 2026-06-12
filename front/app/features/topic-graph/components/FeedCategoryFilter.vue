<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { Icon } from '@iconify/vue'
import { useFeedsStore } from '~/stores/feeds'
import type { RssFeed } from '~/types/feed'
import type { Category } from '~/types/category'

interface Props {
  selectedCategoryId: string | null
  selectedFeedId: string | null
}

const props = defineProps<Props>()

const emit = defineEmits<{
  'update:selectedCategoryId': [value: string | null]
  'update:selectedFeedId': [value: string | null]
}>()

const feedsStore = useFeedsStore()
const expandedCategoryId = ref<string | null>(null)

const categories = computed<Category[]>(() => feedsStore.categories)
const feeds = computed<RssFeed[]>(() => feedsStore.feeds)

function autoSelectFirstCategory() {
  if (!props.selectedCategoryId && !props.selectedFeedId && categories.value.length > 0) {
    const first = categories.value[0]
    if (first) {
      emit('update:selectedCategoryId', first.id)
      expandedCategoryId.value = first.id
    }
  }
}

onMounted(() => {
  autoSelectFirstCategory()
})

watch(categories, () => {
  autoSelectFirstCategory()
})

const feedsByCategory = computed(() => {
  const grouped = new Map<string, RssFeed[]>()
  const uncategorized: RssFeed[] = []
  for (const feed of feeds.value) {
    if (feed.category) {
      const catId = feed.category
      const list = grouped.get(catId) || []
      list.push(feed)
      grouped.set(catId, list)
    } else {
      uncategorized.push(feed)
    }
  }
  return { grouped, uncategorized }
})

function selectAll() {
  expandedCategoryId.value = null
  emit('update:selectedCategoryId', null)
  emit('update:selectedFeedId', null)
}

function toggleCategory(categoryId: string) {
  if (expandedCategoryId.value === categoryId) {
    expandedCategoryId.value = null
  } else {
    expandedCategoryId.value = categoryId
  }
  emit('update:selectedCategoryId', categoryId)
  emit('update:selectedFeedId', null)
}

function selectFeed(feedId: string) {
  emit('update:selectedFeedId', feedId)
  emit('update:selectedCategoryId', null)
}
</script>

<template>
  <div class="feed-category-filter space-y-1">
    <button
      class="fcf-item"
      :class="{ 'fcf-item--active': !props.selectedCategoryId && !props.selectedFeedId }"
      @click="selectAll"
    >
      <Icon icon="mdi:view-grid-outline" class="text-base" />
      <span>全部</span>
    </button>

    <div
      v-for="category in categories"
      :key="category.id"
    >
      <button
        class="fcf-item fcf-item--category"
        :class="{ 'fcf-item--active': props.selectedCategoryId === category.id }"
        @click="toggleCategory(category.id)"
      >
        <Icon :icon="category.icon || 'mdi:folder-outline'" class="text-lg" :style="{ color: category.color || undefined }" />
        <span class="fcf-item__label">{{ category.name }}</span>
        <span
          class="fcf-item__count"
          :style="{ backgroundColor: category.color ? `${category.color}22` : 'var(--color-bg-hover)' }"
        >
          {{ feedsByCategory.grouped.get(category.id)?.length || 0 }}
        </span>
        <Icon
          icon="mdi:chevron-down"
          class="fcf-item__chevron"
          :class="{ 'rotate-180': expandedCategoryId === category.id }"
        />
      </button>

      <div v-if="expandedCategoryId === category.id" class="space-y-0.5 mt-0.5">
        <button
          v-for="feed in feedsByCategory.grouped.get(category.id) || []"
          :key="feed.id"
          class="fcf-item fcf-item--feed"
          :class="{ 'fcf-item--active': props.selectedFeedId === feed.id }"
          @click="selectFeed(feed.id)"
        >
          <Icon :icon="feed.icon || 'mdi:rss'" class="text-base fcf-item__icon-muted" />
          <span class="fcf-item__label-muted truncate">{{ feed.title }}</span>
        </button>
      </div>
    </div>

    <div v-if="feedsByCategory.uncategorized.length > 0">
      <button
        class="fcf-item fcf-item--category"
        :class="{ 'fcf-item--active': props.selectedCategoryId === '__uncategorized__' }"
        @click="toggleCategory('__uncategorized__')"
      >
        <Icon icon="mdi:help-circle-outline" class="text-lg fcf-item__icon-muted" />
        <span class="fcf-item__label">未分类</span>
        <span class="fcf-item__count" style="background: var(--color-bg-hover)">
          {{ feedsByCategory.uncategorized.length }}
        </span>
        <Icon
          icon="mdi:chevron-down"
          class="fcf-item__chevron"
          :class="{ 'rotate-180': expandedCategoryId === '__uncategorized__' }"
        />
      </button>

      <div v-if="expandedCategoryId === '__uncategorized__'" class="space-y-0.5 mt-0.5">
        <button
          v-for="feed in feedsByCategory.uncategorized"
          :key="feed.id"
          class="fcf-item fcf-item--feed"
          :class="{ 'fcf-item--active': props.selectedFeedId === feed.id }"
          @click="selectFeed(feed.id)"
        >
          <Icon :icon="feed.icon || 'mdi:rss'" class="text-base fcf-item__icon-muted" />
          <span class="fcf-item__label-muted truncate">{{ feed.title }}</span>
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.fcf-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 12px;
  border-radius: 9999px;
  font-size: 13px;
  cursor: pointer;
  border: none;
  background: transparent;
  color: var(--color-text-secondary);
  transition: background 0.15s, color 0.15s;
  width: 100%;
  text-align: left;
}

.fcf-item:hover {
  color: var(--color-text-primary);
  background: var(--color-bg-hover);
}

.fcf-item--active {
  background: var(--color-accent-subtle);
  color: var(--color-accent);
}

.fcf-item--category {
  padding: 8px 12px;
  border-radius: 12px;
}

.fcf-item--feed {
  padding: 6px 12px 6px 32px;
  border-radius: 8px;
  font-size: 13px;
}

.fcf-item__label {
  flex: 1;
  color: var(--color-text-primary);
}

.fcf-item__label-muted {
  color: var(--color-text-secondary);
}

.fcf-item__count {
  font-size: 11px;
  padding: 2px 6px;
  border-radius: 9999px;
  color: var(--color-text-muted);
}

.fcf-item__chevron {
  color: var(--color-text-muted);
  transition: transform 0.15s;
}

.fcf-item__icon-muted {
  color: var(--color-text-muted);
}
</style>
