<script setup lang="ts">
import { Icon } from '@iconify/vue'
import type { RssFeed } from '~/types'

const props = defineProps<{
  feed: RssFeed
}>()

const emit = defineEmits<{
  close: []
  updated: []
  deleted: []
}>()

const apiStore = useApiStore()

const url = ref(props.feed.url)
const categoryId = ref<number | undefined>(props.feed.category ? Number(props.feed.category) : undefined)
const title = ref(props.feed.title)
const description = ref(props.feed.description)
const loading = ref(false)
const deleting = ref(false)
const error = ref<string | null>(null)
const showDeleteConfirm = ref(false)

const articleSummaryEnabled = ref(props.feed.articleSummaryEnabled ?? false)
const completionOnRefresh = ref(props.feed.completionOnRefresh ?? true)
const maxCompletionRetries = ref(props.feed.maxCompletionRetries ?? 3)
const firecrawlEnabled = ref(props.feed.firecrawlEnabled ?? false)

watch(() => props.feed, (newFeed) => {
  if (!newFeed) return

  url.value = newFeed.url
  categoryId.value = newFeed.category ? Number(newFeed.category) : undefined
  title.value = newFeed.title
  description.value = newFeed.description
  articleSummaryEnabled.value = newFeed.articleSummaryEnabled ?? false
  completionOnRefresh.value = newFeed.completionOnRefresh ?? true
  maxCompletionRetries.value = newFeed.maxCompletionRetries ?? 3
  firecrawlEnabled.value = newFeed.firecrawlEnabled ?? false
}, { deep: true })

const capabilityItems = computed(() => [
  {
    label: '自动总结',
    enabled: articleSummaryEnabled.value,
    icon: 'mdi:brain',
  },
  {
    label: '全文抓取',
    enabled: firecrawlEnabled.value,
    icon: 'mdi:spider-web',
  },
  {
    label: '刷新后自动总结',
    enabled: completionOnRefresh.value,
    icon: 'mdi:refresh-auto',
  },
])

async function handleSubmit() {
  if (!url.value) return

  loading.value = true
  error.value = null

  const response = await apiStore.updateFeed(props.feed.id, {
    url: url.value,
    category_id: categoryId.value,
    article_summary_enabled: articleSummaryEnabled.value,
    completion_on_refresh: completionOnRefresh.value,
    max_completion_retries: maxCompletionRetries.value,
    firecrawl_enabled: firecrawlEnabled.value,
  })

  loading.value = false

  if (response.success) {
    emit('updated')
    emit('close')
    return
  }

  error.value = response.error || '更新订阅源失败'
}

async function handleDelete() {
  deleting.value = true
  error.value = null

  const response = await apiStore.deleteFeed(props.feed.id)

  deleting.value = false

  if (response.success) {
    emit('deleted')
    emit('close')
    return
  }

  error.value = response.error || '删除订阅源失败'
  showDeleteConfirm.value = false
}
</script>

<template>
  <AppDialog :model-value="true" @update:model-value="emit('close')" title="编辑订阅源" width="672px">
    <div class="grid gap-6 lg:grid-cols-[minmax(0,1fr)_280px]">
      <div class="space-y-5">
        <div class="rounded-2xl border-2 border-[var(--color-border-subtle)] bg-linear-to-br from-[var(--color-bg-hover)] to-[var(--color-bg-elevated)] p-4">
          <div class="flex items-start gap-3">
            <div class="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-linear-to-br from-[var(--color-bg-sunken)] to-[var(--color-bg-sunken)] shadow-md">
              <Icon icon="mdi:rss" class="text-white" width="22" height="22" />
            </div>
            <div class="min-w-0 flex-1">
              <h3 class="truncate font-semibold" style="color: var(--color-text-primary)">{{ title }}</h3>
              <p class="mt-1 line-clamp-2 text-sm" style="color: var(--color-text-secondary)">
                {{ description || '暂无描述' }}
              </p>
            </div>
          </div>
        </div>

        <div class="space-y-2">
          <label class="flex items-center gap-2 text-sm font-semibold" style="color: var(--color-text-primary)">
            <Icon icon="mdi:link-variant" width="16" height="16" class="text-[var(--color-text-secondary)]" />
            RSS 地址
            <span style="color: var(--color-error)">*</span>
          </label>
          <AppInput v-model="url" type="url" placeholder="https://example.com/feed.xml" />
        </div>

        <div class="space-y-2">
          <label class="flex items-center gap-2 text-sm font-semibold" style="color: var(--color-text-primary)">
            <Icon icon="mdi:folder" width="16" height="16" class="text-[var(--color-text-secondary)]" />
            分类
            <span class="text-xs font-normal" style="color: var(--color-text-muted)">可选</span>
          </label>
          <select v-model="categoryId" class="input w-full cursor-pointer">
            <option :value="undefined">未分类</option>
            <option
              v-for="category in apiStore.categories"
              :key="category.id"
              :value="Number(category.id)"
            >
              {{ category.name }}
            </option>
          </select>
        </div>

        <div class="space-y-4 rounded-2xl p-4" style="border: 1px solid var(--color-link-border); background: var(--color-link-subtle)">
          <div class="flex items-center gap-2 text-sm font-semibold" style="color: var(--color-link)">
            <Icon icon="mdi:brain" width="16" height="16" />
            自动总结设置
          </div>

          <div class="flex cursor-pointer items-center justify-between gap-4">
            <div>
              <div class="text-sm font-medium" style="color: var(--color-text-primary)">启用自动总结</div>
              <div class="text-xs" style="color: var(--color-text-muted)">Firecrawl 完成后进入总结队列</div>
            </div>
            <AppToggle v-model="articleSummaryEnabled" />
          </div>

          <div class="flex cursor-pointer items-center justify-between gap-4">
            <div>
              <div class="text-sm font-medium" style="color: var(--color-text-primary)">刷新后自动总结</div>
              <div class="text-xs" style="color: var(--color-text-muted)">新文章创建后自动进入处理链路</div>
            </div>
            <AppToggle v-model="completionOnRefresh" />
          </div>

          <div class="space-y-2">
            <label class="text-sm font-medium" style="color: var(--color-text-primary)">最大重试次数</label>
            <AppInput v-model="maxCompletionRetries" type="number" min="1" max="10" />
          </div>
        </div>

        <div class="space-y-4 rounded-2xl p-4" style="border: 1px solid var(--color-info-bg, rgba(61, 122, 138, 0.2)); background: var(--color-info-bg, rgba(61, 122, 138, 0.08))">
          <div class="flex items-center gap-2 text-sm font-semibold" style="color: var(--color-info)">
            <Icon icon="mdi:spider-web" width="16" height="16" />
            全文抓取设置
          </div>

          <div class="flex cursor-pointer items-center justify-between gap-4">
            <div>
              <div class="text-sm font-medium" style="color: var(--color-text-primary)">启用全文抓取</div>
              <div class="text-xs" style="color: var(--color-text-muted)">使用 Firecrawl 抓取文章全文后再交给总结能力</div>
            </div>
            <AppToggle v-model="firecrawlEnabled" />
          </div>
        </div>

        <div
          v-if="error"
          class="flex items-start gap-3 rounded-xl p-4"
          style="border: 2px solid var(--color-error-border, rgba(196, 47, 60, 0.3)); background: var(--color-error-bg, rgba(196, 47, 60, 0.08))"
        >
          <Icon icon="mdi:alert-circle" width="20" height="20" class="mt-0.5 shrink-0" style="color: var(--color-error)" />
          <p class="text-sm font-medium" style="color: var(--color-error)">{{ error }}</p>
        </div>

        <div
          v-if="showDeleteConfirm"
          class="rounded-xl p-4"
          style="border: 2px solid var(--color-error-border, rgba(196, 47, 60, 0.3)); background: var(--color-error-bg, rgba(196, 47, 60, 0.08))"
        >
          <div class="flex items-start gap-3">
            <Icon icon="mdi:alert-circle" class="mt-0.5 shrink-0" style="color: var(--color-error)" width="22" height="22" />
            <div class="flex-1">
              <h4 class="mb-1 font-semibold" style="color: var(--color-text-primary)">确认删除订阅源？</h4>
              <p class="mb-3 text-sm" style="color: var(--color-text-secondary)">
                删除"{{ title }}"后，该订阅源下的文章也会一起删除，这个操作不可撤销。
              </p>
              <div class="flex gap-2">
                <AppButton variant="secondary" size="sm" :disabled="deleting" @click="showDeleteConfirm = false">
                  取消
                </AppButton>
                <AppButton variant="danger" size="sm" :disabled="deleting" @click="handleDelete">
                  <Icon :icon="deleting ? 'mdi:loading' : 'mdi:delete'" :class="{ 'animate-spin': deleting }" width="16" height="16" />
                  {{ deleting ? '删除中...' : '确认删除' }}
                </AppButton>
              </div>
            </div>
          </div>
        </div>
      </div>

      <aside class="space-y-4 rounded-2xl border border-[var(--color-border-subtle)] bg-[var(--color-bg-elevated)]/70 p-4">
        <div>
          <h3 class="text-sm font-semibold text-[var(--color-text-primary)]">当前能力状态</h3>
          <p class="mt-1 text-xs leading-5 text-[var(--color-text-secondary)]">这里展示这个订阅源会不会进入全文抓取和自动总结链路。</p>
        </div>

        <div class="space-y-3">
          <div
            v-for="item in capabilityItems"
            :key="item.label"
            class="flex items-center justify-between gap-3 rounded-xl px-3 py-2"
            style="border: 1px solid var(--color-border-subtle); background: var(--color-bg-sunken)"
          >
            <div class="flex items-center gap-2 text-sm" style="color: var(--color-text-primary)">
              <Icon :icon="item.icon" width="16" height="16" />
              <span>{{ item.label }}</span>
            </div>
            <span
              class="inline-flex items-center rounded-full px-2.5 py-1 text-xs font-semibold"
              :style="item.enabled
                ? 'background: rgba(61, 138, 74, 0.12); color: var(--color-success); border: 1px solid rgba(61, 138, 74, 0.25)'
                : 'background: var(--color-bg-hover); color: var(--color-text-muted); border: 1px solid var(--color-border-subtle)'"
            >
              {{ item.enabled ? '已开启' : '未开启' }}
            </span>
          </div>
        </div>

        <div class="rounded-xl border border-dashed p-4" style="border-color: var(--color-border-medium); background: var(--color-bg-sunken)">
          <div class="text-xs uppercase tracking-[0.08em] text-[var(--color-text-muted)]">重试策略</div>
          <div class="mt-2 text-2xl font-semibold text-[var(--color-text-primary)]">{{ maxCompletionRetries }}</div>
          <div class="mt-1 text-sm text-[var(--color-text-secondary)]">自动总结失败后最多重试 {{ maxCompletionRetries }} 次。</div>
        </div>
      </aside>
    </div>

    <template #footer>
      <div class="flex w-full items-center justify-between">
        <button
          v-if="!showDeleteConfirm"
          class="flex items-center gap-2 rounded-xl px-4 py-2.5 font-semibold transition-all active:scale-95"
          style="color: var(--color-error)"
          @click="showDeleteConfirm = true"
        >
          <Icon icon="mdi:delete" width="18" height="18" />
          删除订阅源
        </button>
        <div v-else />

        <div v-if="!showDeleteConfirm" class="flex gap-2">
          <AppButton variant="secondary" @click="emit('close')">取消</AppButton>
          <AppButton
            variant="primary"
            :disabled="!url || loading"
            @click="handleSubmit"
          >
            <Icon :icon="loading ? 'mdi:loading' : 'mdi:check'" :class="{ 'animate-spin': loading }" width="18" height="18" />
            {{ loading ? '保存中...' : '保存更改' }}
          </AppButton>
        </div>
      </div>
    </template>
  </AppDialog>
</template>
