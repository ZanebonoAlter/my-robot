import { ref } from 'vue'
import { useBochaApi } from '~/api'

const DEFAULT_BOCHA_ENDPOINT = 'https://api.bochaai.com/v1/web-search'

export function useBochaConfig() {
  const bochaEnabled = ref(false)
  const bochaEndpoint = ref(DEFAULT_BOCHA_ENDPOINT)
  const bochaApiKey = ref('')
  const bochaApiKeyConfigured = ref(false)
  const bochaApiKeyVisible = ref(false)
  const bochaLoading = ref(false)
  const bochaError = ref<string | null>(null)
  const bochaSuccess = ref<string | null>(null)

  async function loadBochaSettings() {
    bochaLoading.value = true
    bochaError.value = null
    try {
      const { getStatus } = useBochaApi()
      const response = await getStatus()
      if (response.success && response.data) {
        bochaEnabled.value = response.data.enabled
        bochaEndpoint.value = response.data.endpoint || DEFAULT_BOCHA_ENDPOINT
        // GET 不回显完整 key（脱敏），只记录"已配置"状态，输入框保持留空
        bochaApiKeyConfigured.value =
          response.data.api_key_configured === true || !!response.data.api_key
      }
    } catch {
      bochaError.value = '加载博查搜索设置失败'
    } finally {
      bochaLoading.value = false
    }
  }

  async function saveBochaSettings() {
    bochaLoading.value = true
    bochaError.value = null
    bochaSuccess.value = null
    try {
      const { saveSettings } = useBochaApi()
      const response = await saveSettings({
        enabled: bochaEnabled.value,
        endpoint: bochaEndpoint.value,
        // 空串 = 不修改已有 key（后端约定），非空才覆盖
        api_key: bochaApiKey.value,
      })
      if (!response.success) {
        throw new Error(response.error || '保存失败')
      }
      if (response.data) {
        bochaApiKeyConfigured.value =
          response.data.api_key_configured === true || !!response.data.api_key
      } else if (bochaApiKey.value) {
        bochaApiKeyConfigured.value = true
      }
      bochaSuccess.value = '博查搜索设置已保存'
      setTimeout(() => { bochaSuccess.value = null }, 2000)
    } catch {
      bochaError.value = '保存博查搜索设置失败'
    } finally {
      bochaLoading.value = false
    }
  }

  return {
    bochaEnabled, bochaEndpoint, bochaApiKey, bochaApiKeyConfigured,
    bochaApiKeyVisible, bochaLoading,
    bochaError, bochaSuccess,
    loadBochaSettings, saveBochaSettings,
  }
}
