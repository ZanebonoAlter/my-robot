import { ref, computed, reactive, onMounted } from 'vue'
import { useAIAdminApi } from '~/api'
import type { AIProvider, AIRoute, AIProviderUpsertRequest } from '~/types'
import { useAI } from '~/composables/useAI'

const routeLabels: Record<string, string> = {
  summary: '文章总结',
  topic_tagging: '主题提取',
  digest_polish: '日报润色',
  embedding: '向量嵌入',
  feed_discovery: '订阅源发现',
}

const capabilityOrder = ['summary', 'topic_tagging', 'digest_polish', 'embedding', 'feed_discovery']

export function useAIRouterSettings() {
  const loading = ref(false)
  const saving = ref(false)
  const testing = ref(false)
  const error = ref<string | null>(null)
  const success = ref<string | null>(null)

  const providers = ref<AIProvider[]>([])
  const routes = ref<AIRoute[]>([])
  const routeSelections = ref<Record<string, number[]>>({})

  const primaryProviderId = ref<number | null>(null)
  const primaryProviderForm = reactive<AIProviderUpsertRequest & { time_range: number }>({
    name: 'default-primary',
    provider_type: 'openai_compatible',
    base_url: '',
    api_key: '',
    model: '',
    enabled: true,
    timeout_seconds: 120,
    enable_thinking: false,
    time_range: 180,
  })

  const newProviderForm = reactive<AIProviderUpsertRequest>({
    name: '',
    provider_type: 'openai_compatible',
    base_url: '',
    api_key: '',
    model: '',
    enabled: true,
    timeout_seconds: 120,
    enable_thinking: false,
  })

  const showNewProviderForm = ref(false)
  const showPrimaryApiKey = ref(false)
  const showNewProviderApiKey = ref(false)
  const { loadSettings: reloadAISettings } = useAI()
  const editingProviderId = ref<number | null>(null)
  const draggingProviderId = ref<number | null>(null)
  const draggingCapability = ref<string | null>(null)
  const editProviderForm = reactive<AIProviderUpsertRequest>({
    name: '',
    provider_type: 'openai_compatible',
    base_url: '',
    api_key: '',
    model: '',
    enabled: true,
    timeout_seconds: 120,
    enable_thinking: false,
    clear_api_key: false,
  })
  const showEditProviderApiKey = ref(false)

  const backupProviders = computed(() => providers.value.filter(p => p && typeof p.id === 'number' && p.id !== primaryProviderId.value))

  // ---- Helpers ----
  function routeSummary(capability: string): number[] {
    return routeSelections.value[capability] || []
  }

  function providerName(providerId: number): string {
    return providers.value.find(p => p.id === providerId)?.name || `#${providerId}`
  }

  function isProviderLinked(providerId: number): boolean {
    return routes.value.some(r => r.route_providers.some(l => l.provider_id === providerId))
  }

  // ---- Hydration ----
  function hydratePrimaryProvider() {
    const preferred = providers.value.find(p => p.id === primaryProviderId.value)
      || providers.value.find(p => p.name === 'default-primary')
      || providers.value[0]
    primaryProviderId.value = preferred?.id || null
    applyPrimaryProvider(preferred || null)
  }

  function applyPrimaryProvider(provider: AIProvider | null | undefined) {
    primaryProviderId.value = provider?.id || null
    primaryProviderForm.name = provider?.name || 'default-primary'
    primaryProviderForm.provider_type = provider?.provider_type || 'openai_compatible'
    primaryProviderForm.base_url = provider?.base_url || ''
    primaryProviderForm.api_key = ''
    primaryProviderForm.model = provider?.model || ''
    primaryProviderForm.enabled = provider?.enabled ?? true
    primaryProviderForm.timeout_seconds = provider?.timeout_seconds || 120
    primaryProviderForm.enable_thinking = provider?.enable_thinking ?? false
  }

  function hydrateRouteSelections() {
    const next: Record<string, number[]> = {}
    for (const cap of capabilityOrder) {
      const route = routes.value.find(r => r.capability === cap)
      if (!route) { next[cap] = []; continue }
      next[cap] = route.route_providers
        .slice().sort((a, b) => a.priority - b.priority).map(l => l.provider_id)
    }
    routeSelections.value = next
  }

  // ---- Data loading ----
  async function loadData() {
    loading.value = true
    error.value = null
    try {
      const aiAdminApi = useAIAdminApi()
      const [settingsRes, providersRes, routesRes] = await Promise.all([
        aiAdminApi.getSettings(),
        aiAdminApi.listProviders(),
        aiAdminApi.listRoutes(),
      ])
      if (!providersRes.success || !routesRes.success) {
        throw new Error(providersRes.error || routesRes.error || '加载 AI 配置失败')
      }
      providers.value = providersRes.data || []
      routes.value = routesRes.data || []
      primaryProviderId.value = settingsRes.success && settingsRes.data?.provider_id
        ? Number(settingsRes.data.provider_id) : null
      primaryProviderForm.time_range = settingsRes.success && typeof settingsRes.data?.time_range === 'number'
        ? settingsRes.data.time_range : 180
      hydratePrimaryProvider()
      hydrateRouteSelections()
    } catch (err) {
      error.value = err instanceof Error ? err.message : '加载 AI 配置失败'
    } finally {
      loading.value = false
    }
  }

  // ---- Notification ----
  function pushMessage(kind: 'success' | 'error', message: string) {
    if (kind === 'success') {
      success.value = message
      error.value = null
      setTimeout(() => { success.value = null }, 2500)
    } else {
      error.value = message
      success.value = null
    }
  }

  // ---- Route management ----
  function addProviderToRoute(capability: string, providerId: number) {
    const current = routeSummary(capability)
    if (current.includes(providerId)) return
    routeSelections.value[capability] = [...current, providerId]
  }

  function removeProviderFromRoute(capability: string, providerId: number) {
    routeSelections.value[capability] = routeSummary(capability).filter(id => id !== providerId)
  }

  function isPrimaryInRoute(capability: string): boolean {
    return routeSummary(capability).includes(primaryProviderId.value ?? -1)
  }

  function removePrimaryFromRoute(capability: string) {
    if (primaryProviderId.value) removeProviderFromRoute(capability, primaryProviderId.value)
  }

  function addPrimaryToRoute(capability: string) {
    if (primaryProviderId.value) addProviderToRoute(capability, primaryProviderId.value)
  }

  function moveProvider(capability: string, providerId: number, direction: -1 | 1) {
    const current = [...routeSummary(capability)]
    const index = current.indexOf(providerId)
    const nextIndex = index + direction
    if (index < 0 || nextIndex < 0 || nextIndex >= current.length) return
    const cur = current[index]; const nxt = current[nextIndex]
    if (cur === undefined || nxt === undefined) return
    current[index] = nxt; current[nextIndex] = cur
    routeSelections.value[capability] = current
  }

  function handleDragStart(capability: string, providerId: number) {
    draggingCapability.value = capability
    draggingProviderId.value = providerId
  }

  function handleDragEnd() {
    draggingCapability.value = null
    draggingProviderId.value = null
  }

  function handleDropOnProvider(capability: string, targetProviderId: number) {
    if (draggingCapability.value !== capability || draggingProviderId.value === null) {
      handleDragEnd(); return
    }
    const current = [...routeSummary(capability)]
    const fromIdx = current.indexOf(draggingProviderId.value)
    const toIdx = current.indexOf(targetProviderId)
    if (fromIdx < 0 || toIdx < 0 || fromIdx === toIdx) { handleDragEnd(); return }
    const [moved] = current.splice(fromIdx, 1)
    if (moved === undefined) { handleDragEnd(); return }
    current.splice(toIdx, 0, moved)
    routeSelections.value[capability] = current
    handleDragEnd()
  }

  // ---- Primary provider CRUD ----
  async function savePrimaryProvider() {
    if (!primaryProviderForm.base_url || !primaryProviderForm.model) {
      pushMessage('error', '主模型至少要有 Base URL 和 Model'); return
    }
    saving.value = true; error.value = null
    try {
      const aiAdminApi = useAIAdminApi()
      let providerId = primaryProviderId.value
      if (providerId) {
        const res = await aiAdminApi.updateProvider(providerId, primaryProviderForm)
        if (!res.success) throw new Error(res.error || '更新主模型失败')
      } else {
        const res = await aiAdminApi.createProvider(primaryProviderForm)
        if (!res.success || !res.data) throw new Error(res.error || '创建主模型失败')
        providerId = res.data.id
        primaryProviderId.value = providerId
      }
      const providerSnapshot: AIProvider = {
        id: providerId || 0, name: primaryProviderForm.name,
        provider_type: primaryProviderForm.provider_type || 'openai_compatible',
        base_url: primaryProviderForm.base_url, model: primaryProviderForm.model,
        enabled: primaryProviderForm.enabled ?? true,
        timeout_seconds: primaryProviderForm.timeout_seconds || 120,
        max_tokens: primaryProviderForm.max_tokens ?? null,
        temperature: primaryProviderForm.temperature ?? null,
        enable_thinking: primaryProviderForm.enable_thinking ?? false,
        metadata: primaryProviderForm.metadata, api_key_configured: true,
      }
      if (providerId) {
        for (const capability of capabilityOrder) {
          const existingRoute = routes.value.find(r => r.capability === capability)
          let ids = routeSummary(capability)
          if (!ids.includes(providerId)) ids = [providerId, ...ids]
          const res = await aiAdminApi.updateRoute(capability, {
            name: existingRoute?.name || 'default', enabled: existingRoute?.enabled ?? true,
            description: existingRoute?.description, provider_ids: ids,
          })
          if (!res.success) throw new Error(res.error || `同步 ${routeLabels[capability]} 主路由失败`)
        }
      }
      await loadData()
      applyPrimaryProvider(providerSnapshot)
      await reloadAISettings(true)
      pushMessage('success', '主模型配置已保存')
    } catch (err) {
      pushMessage('error', err instanceof Error ? err.message : '保存失败')
    } finally {
      saving.value = false
    }
  }

  // ---- Backup provider CRUD ----
  async function saveNewProvider() {
    if (!newProviderForm.name || !newProviderForm.base_url || !newProviderForm.model) {
      pushMessage('error', '备用模型表单还没填完整'); return
    }
    saving.value = true
    try {
      const aiAdminApi = useAIAdminApi()
      const res = await aiAdminApi.createProvider(newProviderForm)
      if (!res.success) throw new Error(res.error || '创建备用模型失败')
      newProviderForm.name = ''
      newProviderForm.base_url = ''
      newProviderForm.api_key = ''
      newProviderForm.model = ''
      newProviderForm.enabled = true
      newProviderForm.timeout_seconds = 120
      newProviderForm.enable_thinking = false
      showNewProviderForm.value = false
      await loadData()
      pushMessage('success', '备用模型已添加')
    } catch (err) {
      pushMessage('error', err instanceof Error ? err.message : '创建失败')
    } finally {
      saving.value = false
    }
  }

  function startEditingProvider(provider: AIProvider) {
    editingProviderId.value = provider.id
    editProviderForm.name = provider.name
    editProviderForm.provider_type = provider.provider_type
    editProviderForm.base_url = provider.base_url
    editProviderForm.api_key = ''
    editProviderForm.model = provider.model
    editProviderForm.enabled = provider.enabled
    editProviderForm.timeout_seconds = provider.timeout_seconds
    editProviderForm.enable_thinking = provider.enable_thinking ?? false
    editProviderForm.clear_api_key = false
  }

  function cancelEditingProvider() {
    editingProviderId.value = null
    editProviderForm.name = ''
    editProviderForm.provider_type = 'openai_compatible'
    editProviderForm.base_url = ''
    editProviderForm.api_key = ''
    editProviderForm.model = ''
    editProviderForm.enabled = true
    editProviderForm.timeout_seconds = 120
    editProviderForm.enable_thinking = false
    editProviderForm.clear_api_key = false
  }

  async function saveEditedProvider() {
    if (!editingProviderId.value) return
    if (!editProviderForm.name || !editProviderForm.base_url || !editProviderForm.model) {
      pushMessage('error', '编辑备用模型时，名称、Base URL 和 Model 不能为空'); return
    }
    saving.value = true
    try {
      const aiAdminApi = useAIAdminApi()
      const res = await aiAdminApi.updateProvider(editingProviderId.value, editProviderForm)
      if (!res.success) throw new Error(res.error || '更新备用模型失败')
      cancelEditingProvider()
      await loadData()
      pushMessage('success', '备用模型已更新')
    } catch (err) {
      pushMessage('error', err instanceof Error ? err.message : '更新失败')
    } finally {
      saving.value = false
    }
  }

  async function deleteBackupProvider(provider: AIProvider) {
    if (!confirm(`确定删除备用模型 ${provider.name} 吗？`)) return
    saving.value = true
    try {
      const aiAdminApi = useAIAdminApi()
      const res = await aiAdminApi.deleteProvider(provider.id)
      if (!res.success) throw new Error(res.error || '删除备用模型失败')
      for (const capability of capabilityOrder) {
        removeProviderFromRoute(capability, provider.id)
      }
      if (editingProviderId.value === provider.id) cancelEditingProvider()
      await loadData()
      pushMessage('success', '备用模型已删除')
    } catch (err) {
      pushMessage('error', err instanceof Error ? err.message : '删除失败')
    } finally {
      saving.value = false
    }
  }

  // ---- Route save ----
  async function saveRoutes() {
    const hasProviders = capabilityOrder.some(cap => routeSummary(cap).length > 0)
    if (!hasProviders) { pushMessage('error', '至少为一条能力路由配置一个 provider'); return }
    saving.value = true
    try {
      const aiAdminApi = useAIAdminApi()
      for (const capability of capabilityOrder) {
        const ids = routeSummary(capability)
        if (ids.length === 0) continue
        const res = await aiAdminApi.updateRoute(capability, {
          name: 'default', enabled: true, provider_ids: ids,
        })
        if (!res.success) throw new Error(res.error || `保存 ${routeLabels[capability]} 路由失败`)
      }
      await loadData()
      await reloadAISettings(true)
      pushMessage('success', '多路由顺序已保存')
    } catch (err) {
      pushMessage('error', err instanceof Error ? err.message : '保存路由失败')
    } finally {
      saving.value = false
    }
  }

  // ---- Test connection ----
  async function testPrimaryProvider() {
    if (!primaryProviderForm.base_url || !primaryProviderForm.model) {
      pushMessage('error', '测试连接前请填入 Base URL 和 Model'); return
    }
    testing.value = true
    try {
      const aiAdminApi = useAIAdminApi()
      const res = await aiAdminApi.testConnection({
        base_url: primaryProviderForm.base_url,
        api_key: primaryProviderForm.api_key || undefined,
        model: primaryProviderForm.model,
        provider_type: primaryProviderForm.provider_type,
      })
      if (!res.success) throw new Error(res.error || '连接测试失败')
      pushMessage('success', res.message || '连接测试成功')
    } catch (err) {
      pushMessage('error', err instanceof Error ? err.message : '连接测试失败')
    } finally {
      testing.value = false
    }
  }

  onMounted(() => { void loadData() })

  return {
    // State
    loading, saving, testing, error, success,
    providers, routes, routeSelections,
    primaryProviderId, primaryProviderForm,
    newProviderForm, showNewProviderForm,
    showPrimaryApiKey, showNewProviderApiKey,
    editingProviderId, editProviderForm, showEditProviderApiKey,
    draggingProviderId, draggingCapability,
    backupProviders,

    // Constants
    routeLabels, capabilityOrder,

    // Route helpers
    routeSummary, providerName, isProviderLinked,
    addProviderToRoute, removeProviderFromRoute,
    isPrimaryInRoute, removePrimaryFromRoute, addPrimaryToRoute,
    moveProvider, handleDragStart, handleDragEnd, handleDropOnProvider,

    // CRUD
    loadData, savePrimaryProvider,
    saveNewProvider, startEditingProvider, cancelEditingProvider,
    saveEditedProvider, deleteBackupProvider,
    saveRoutes,
    testPrimaryProvider,
  }
}
