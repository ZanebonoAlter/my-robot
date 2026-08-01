<script setup lang="ts">
import { Icon } from '@iconify/vue'
import { useDiscovery } from '../composables/useDiscovery'
import { useRsshubApi } from '~/api/rsshub'
import DiscoveryCard from './DiscoveryCard.vue'

const { store, groups, catalogEmpty } = useDiscovery()

const question = ref('')

// RSSHub 官方文档基址（design D4）：面板初始化拉一次生效值，注入各卡片；失败则兜底默认常量
const docBase = ref('')
const rsshubApi = useRsshubApi()
onMounted(async () => {
  const res = await rsshubApi.getStatus()
  if (res.success && res.data?.rsshub_doc_base) docBase.value = res.data.rsshub_doc_base
})

function submitQuestion() {
  const q = question.value.trim()
  if (!q) return
  void store.ask(q).then((ok) => {
    if (ok) question.value = ''
  })
}
</script>

<template>
  <div class="discovery-panel">
    <!-- 问答输入 -->
    <div class="discovery-ask">
      <Icon icon="mdi:chat-question-outline" width="20" height="20" class="discovery-ask__icon" />
      <input
        v-model="question"
        type="text"
        class="discovery-ask__input"
        placeholder="告诉我想看什么，比如：我想看 AI 芯片相关的中文资讯"
        :disabled="store.asking"
        @keyup.enter="submitQuestion"
      />
      <AppButton size="md" :loading="store.asking" :disabled="!question.trim()" @click="submitQuestion">
        帮我找源
      </AppButton>
    </div>

    <!-- 工具行 -->
    <div class="discovery-toolbar">
      <AppButton
        size="sm"
        variant="secondary"
        :loading="store.refreshing"
        :disabled="catalogEmpty"
        @click="store.refresh()"
      >
        <Icon icon="mdi:refresh" width="14" height="14" />
        换一批
      </AppButton>
      <p v-if="store.catalogStatus" class="discovery-toolbar__hint">
        目录共 {{ store.catalogStatus.total }} 条路由
      </p>
    </div>

    <!-- 加载中 -->
    <div v-if="store.loading && groups.length === 0" class="discovery-empty">
      <Icon icon="mdi:loading" width="40" height="40" class="animate-spin" style="color: var(--color-link)" />
      <p class="discovery-empty__text">加载推荐中...</p>
    </div>

    <!-- 空态：目录未同步 -->
    <div v-else-if="catalogEmpty" class="discovery-empty">
      <Icon icon="mdi:radar" width="48" height="48" style="color: var(--color-text-muted)" />
      <p class="discovery-empty__title">订阅源目录还没准备好</p>
      <p class="discovery-empty__text">
        先从 RSSHub 实例同步一份路由目录（约 3000+ 条），之后才能给你推荐。
      </p>
      <AppButton size="md" :loading="store.syncingCatalog" @click="store.syncCatalog()">
        同步目录
      </AppButton>
    </div>

    <!-- 空态：无推荐 -->
    <div v-else-if="groups.length === 0" class="discovery-empty">
      <Icon icon="mdi:telescope" width="48" height="48" style="color: var(--color-text-muted)" />
      <p class="discovery-empty__title">暂时没有推荐</p>
      <p class="discovery-empty__text">
        在上面输入你的兴趣（问答会记住你的偏好），或点「换一批」让系统按你的阅读习惯推荐。
      </p>
    </div>

    <!-- 推荐卡片流（按版块分组） -->
    <div v-else class="discovery-groups">
      <section v-for="group in groups" :key="group.label" class="discovery-group">
        <h3 class="discovery-group__title">
          {{ group.label }}
          <span class="discovery-group__count">{{ group.cards.length }}</span>
        </h3>
        <div class="discovery-group__cards">
          <DiscoveryCard v-for="card in group.cards" :key="card.id" :card="card" :doc-base="docBase" />
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped>
.discovery-panel {
  display: flex;
  flex-direction: column;
  gap: 16px;
  max-width: 860px;
  margin: 0 auto;
}

.discovery-ask {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 14px;
  border: 1px solid var(--color-border-subtle);
  border-radius: 12px;
  background: var(--color-bg-elevated);
}

.discovery-ask__icon {
  flex-shrink: 0;
  color: var(--color-text-muted);
}

.discovery-ask__input {
  flex: 1;
  min-width: 0;
  border: none;
  outline: none;
  background: transparent;
  font-size: 14px;
  color: var(--color-text-primary);
}

.discovery-ask__input::placeholder {
  color: var(--color-text-muted);
}

.discovery-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
}

.discovery-toolbar__hint {
  margin: 0;
  font-size: 12px;
  color: var(--color-text-muted);
}

.discovery-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  padding: 56px 24px;
  text-align: center;
}

.discovery-empty__title {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: var(--color-text-primary);
}

.discovery-empty__text {
  margin: 0;
  font-size: 13px;
  line-height: 1.7;
  color: var(--color-text-muted);
  max-width: 420px;
}

.discovery-groups {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.discovery-group__title {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0 0 10px;
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text-secondary);
}

.discovery-group__count {
  font-size: 11px;
  font-weight: 500;
  padding: 1px 8px;
  border-radius: 999px;
  background: var(--color-bg-sunken);
  color: var(--color-text-muted);
}

.discovery-group__cards {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
</style>
