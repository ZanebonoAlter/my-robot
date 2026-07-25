<script setup lang="ts">
import { Icon } from '@iconify/vue'
import { usePreferenceProfile } from '~/features/settings/composables/usePreferenceProfile'
import { formatDate } from '~/utils/date'

const { groups, loading, recomputing, topTags, maxWeight, recompute } = usePreferenceProfile()

function formatComputedAt(iso: string | null): string {
  if (!iso) return '未知'
  try {
    return formatDate(iso)
  } catch {
    return '未知'
  }
}
</script>

<template>
  <div class="space-y-6">
    <!-- 工具行 -->
    <div class="flex items-center gap-3">
      <p class="text-sm flex-1" style="color: var(--color-text-muted)">
        兴趣画像由你的阅读行为与问答表达聚合而成，是「发现订阅源」推荐的基础。
      </p>
      <AppButton size="sm" variant="secondary" :loading="recomputing" @click="recompute">
        <Icon icon="mdi:refresh" width="14" height="14" />
        重新计算
      </AppButton>
    </div>

    <!-- 加载中 -->
    <div v-if="loading" class="flex items-center justify-center py-12">
      <Icon icon="mdi:loading" width="40" height="40" class="animate-spin" style="color: var(--color-link)" />
    </div>

    <!-- 空态引导（不出现恒 0 伪分数） -->
    <div v-else-if="groups.length === 0" class="profile-empty">
      <Icon icon="mdi:account-heart-outline" width="48" height="48" style="color: var(--color-text-muted)" />
      <p class="profile-empty__title">还没有兴趣画像</p>
      <p class="profile-empty__text">
        多读几篇文章（打开、深读、收藏都会计入），或去发现页用问答直接告诉我你的兴趣。
      </p>
      <NuxtLink to="/discovery" class="profile-empty__link">
        <Icon icon="mdi:compass-outline" width="14" height="14" />
        去发现页
      </NuxtLink>
    </div>

    <!-- 画像：按版块分组 -->
    <div v-else class="space-y-5">
      <section
        v-for="group in groups"
        :key="group.boardLabel"
        class="rounded-xl p-4"
        style="background: var(--color-bg-sunken)"
      >
        <h4 class="text-sm font-semibold mb-3" style="color: var(--color-text-primary)">
          {{ group.boardLabel }}
        </h4>

        <div class="space-y-4">
          <div v-for="item in group.items" :key="`${group.boardLabel}-${item.source}`">
            <div class="flex items-center gap-2 mb-2">
              <span
                class="profile-source"
                :class="item.source === 'seed' ? 'profile-source--seed' : 'profile-source--behavior'"
              >
                {{ item.source === 'seed' ? '问答种子' : '阅读行为' }}
              </span>
              <span class="text-xs" style="color: var(--color-text-muted)">
                最后计算：{{ formatComputedAt(item.lastComputedAt) }}
              </span>
            </div>

            <div class="space-y-1.5">
              <div
                v-for="t in topTags(item)"
                :key="t.tag"
                class="flex items-center gap-3"
              >
                <span class="profile-tag">{{ t.tag }}</span>
                <div class="flex-1 h-1.5 rounded-full overflow-hidden" style="background: var(--color-border-medium)">
                  <div
                    class="h-full rounded-full"
                    :style="{
                      width: `${maxWeight(topTags(item)) > 0 ? Math.round((t.weight / maxWeight(topTags(item))) * 100) : 0}%`,
                      background: 'var(--color-accent)',
                    }"
                  />
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped>
.profile-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  padding: 48px 24px;
  text-align: center;
}

.profile-empty__title {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: var(--color-text-primary);
}

.profile-empty__text {
  margin: 0;
  font-size: 13px;
  line-height: 1.7;
  color: var(--color-text-muted);
  max-width: 420px;
}

.profile-empty__link {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin-top: 4px;
  padding: 6px 14px;
  border-radius: 8px;
  font-size: 13px;
  font-weight: 500;
  background: var(--color-accent);
  color: var(--color-text-inverted);
  transition: background 0.15s;
}

.profile-empty__link:hover {
  background: var(--color-accent-hover);
}

.profile-source {
  font-size: 11px;
  padding: 2px 8px;
  border-radius: 999px;
  font-weight: 500;
}

.profile-source--behavior {
  background: var(--color-accent-subtle);
  color: var(--color-accent);
}

.profile-source--seed {
  background: var(--color-success-subtle);
  color: var(--color-success);
}

.profile-tag {
  width: 96px;
  flex-shrink: 0;
  font-size: 13px;
  color: var(--color-text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
