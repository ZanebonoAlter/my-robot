<script setup lang="ts">
import { Icon } from "@iconify/vue";
import { useFeedsApi } from '~/api/feeds'

const feedsApi = useFeedsApi()

const emit = defineEmits<{
  close: []
  added: []
}>()

const apiStore = useApiStore()
const feedsStore = useFeedsStore()

const url = ref('')
const categoryId = ref<number | undefined>(undefined)
const loading = ref(false)
const error = ref<string | null>(null)
const previewing = ref(false)
const preview = ref<any>(null)

async function handlePreview() {
  if (!url.value) return

  previewing.value = true
  error.value = null

  try {
    const response = await feedsApi.fetchFeed(url.value)
    if (categoryId.value !== undefined && response.success && response.data) {
      preview.value = {
        ...response.data,
        category_id: categoryId.value,
      }
      return
    }

    if (response.success && response.data) {
      preview.value = response.data
    } else {
      error.value = response.error || 'Failed to fetch feed'
    }
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to fetch feed'
  } finally {
    previewing.value = false
  }
}

async function handleAdd() {
  if (!url.value) return

  loading.value = true
  error.value = null

  const response = await apiStore.createFeed({
    url: url.value,
    category_id: categoryId.value,
  })

  loading.value = false

  if (response.success) {
    emit('added')
    emit('close')
  } else {
    error.value = response.error || 'Failed to add feed'
  }
}
</script>

<template>
  <AppDialog :model-value="true" @update:model-value="emit('close')" title="添加订阅源">
    <div class="space-y-5">
      <!-- URL Input -->
      <div class="space-y-2">
        <label class="flex items-center gap-2 text-sm font-semibold" style="color: var(--color-text-primary)">
          <Icon icon="mdi:link-variant" width="16" height="16" class="text-[var(--color-text-secondary)]" />
          RSS 订阅地址
          <span style="color: var(--color-error)">*</span>
        </label>
        <div class="flex gap-2" @keydown.enter="handlePreview">
          <AppInput v-model="url" type="url" placeholder="https://example.com/feed.xml" class="flex-1" />
          <AppButton variant="secondary" :disabled="!url || previewing" @click="handlePreview">
            <Icon
              :icon="previewing ? 'mdi:loading' : 'mdi:magnify'"
              :class="{ 'animate-spin': previewing }"
              width="20"
              height="20"
            />
          </AppButton>
        </div>
      </div>

      <!-- Category Select -->
      <div class="space-y-2">
        <label class="flex items-center gap-2 text-sm font-semibold" style="color: var(--color-text-primary)">
          <Icon icon="mdi:folder" width="16" height="16" class="text-[var(--color-text-secondary)]" />
          分类
          <span class="text-xs font-normal" style="color: var(--color-text-muted)">(可选)</span>
        </label>
        <select
          v-model="categoryId"
          class="input w-full cursor-pointer"
        >
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

      <!-- Preview -->
      <Transition
        enter-active-class="transition ease-out duration-200"
        enter-from-class="opacity-0 translate-y-1"
        enter-to-class="opacity-100 translate-y-0"
        leave-active-class="transition ease-in duration-150"
        leave-from-class="opacity-100 translate-y-0"
        leave-to-class="opacity-0 translate-y-1"
      >
        <div
          v-if="preview && !previewing"
          class="p-4 bg-gradient-to-br from-[var(--color-bg-hover)] to-[var(--color-bg-elevated)] rounded-lg border-2 border-[var(--color-border-subtle)]"
        >
          <div class="flex items-start gap-3">
          <div class="w-11 h-11 rounded-lg bg-gradient-to-br from-[var(--color-bg-sunken)] to-[var(--color-bg-sunken)] flex items-center justify-center shrink-0 shadow-md">
              <Icon icon="mdi:rss" class="text-white" width="22" height="22" />
            </div>
            <div class="flex-1 min-w-0">
              <h3 class="font-semibold mb-1 truncate" style="color: var(--color-text-primary)">
                {{ preview.title || '无标题' }}
              </h3>
              <p class="text-sm mb-2.5 line-clamp-2" style="color: var(--color-text-secondary)">
                {{ preview.description || '无描述' }}
              </p>
              <div class="flex items-center gap-2">
                <span class="inline-flex items-center px-3 py-1.5 rounded-lg bg-[var(--color-bg-elevated)] text-[var(--color-text-primary)] text-xs font-semibold">
                  <Icon icon="mdi:file-document" width="14" height="14" class="mr-1.5" />
                  {{ preview.article_count || 0 }} 篇文章
                </span>
              </div>
            </div>
          </div>
        </div>
      </Transition>

      <!-- Error -->
      <Transition
        enter-active-class="transition ease-out duration-200"
        enter-from-class="opacity-0 translate-y-1"
        enter-to-class="opacity-100 translate-y-0"
        leave-active-class="transition ease-in duration-150"
        leave-from-class="opacity-100 translate-y-0"
        leave-to-class="opacity-0 translate-y-1"
      >
        <div
          v-if="error"
          class="flex items-start gap-3 p-4 rounded-lg"
          style="background: var(--color-error-bg, rgba(196, 47, 60, 0.08)); border: 2px solid var(--color-error-border, rgba(196, 47, 60, 0.3))"
        >
          <Icon icon="mdi:alert-circle" width="20" height="20" class="shrink-0 mt-0.5" style="color: var(--color-error)" />
          <p class="text-sm font-medium" style="color: var(--color-error)">{{ error }}</p>
        </div>
      </Transition>
    </div>

    <template #footer>
      <AppButton variant="secondary" @click="emit('close')">
        取消
      </AppButton>
      <AppButton
        variant="primary"
        :disabled="!url || loading"
        @click="handleAdd"
      >
        <Icon
          :icon="loading ? 'mdi:loading' : 'mdi:plus'"
          :class="{ 'animate-spin': loading }"
          width="18"
          height="18"
        />
        {{ loading ? '添加中...' : '添加订阅源' }}
      </AppButton>
    </template>
  </AppDialog>
</template>
