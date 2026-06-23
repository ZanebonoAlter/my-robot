import { ref } from 'vue'
import { useFirecrawlApi } from '~/api'

export function useFirecrawlConfig() {
  const firecrawlEnabled = ref(false)
  const firecrawlApiUrl = ref('')
  const firecrawlApiKey = ref('')
  const firecrawlMode = ref('scrape')
  const firecrawlTimeout = ref(60)
  const firecrawlMaxContentLength = ref(50000)
  const firecrawlApiKeyVisible = ref(false)
  const firecrawlLoading = ref(false)
  const firecrawlError = ref<string | null>(null)
  const firecrawlSuccess = ref<string | null>(null)

  async function loadFirecrawlSettings() {
    firecrawlLoading.value = true
    firecrawlError.value = null
    try {
      const { getStatus } = useFirecrawlApi()
      const response = await getStatus()
      if (response.success && response.data) {
        firecrawlEnabled.value = response.data.enabled
        firecrawlApiUrl.value = response.data.api_url
        firecrawlMode.value = response.data.mode || 'scrape'
        firecrawlTimeout.value = response.data.timeout || 60
        firecrawlMaxContentLength.value = response.data.max_content_length || 50000
      }
    } catch {
      firecrawlError.value = '加载 Firecrawl 设置失败'
    } finally {
      firecrawlLoading.value = false
    }
  }

  async function saveFirecrawlSettings() {
    firecrawlLoading.value = true
    firecrawlError.value = null
    firecrawlSuccess.value = null
    try {
      const { saveSettings } = useFirecrawlApi()
      const response = await saveSettings({
        enabled: firecrawlEnabled.value,
        api_url: firecrawlApiUrl.value,
        api_key: firecrawlApiKey.value,
        mode: firecrawlMode.value,
        timeout: firecrawlTimeout.value,
        max_content_length: firecrawlMaxContentLength.value,
      })
      if (!response.success) {
        throw new Error(response.error || '保存失败')
      }
      firecrawlSuccess.value = 'Firecrawl 设置已保存'
      setTimeout(() => { firecrawlSuccess.value = null }, 2000)
    } catch {
      firecrawlError.value = '保存 Firecrawl 设置失败'
    } finally {
      firecrawlLoading.value = false
    }
  }

  return {
    firecrawlEnabled, firecrawlApiUrl, firecrawlApiKey, firecrawlMode,
    firecrawlTimeout, firecrawlMaxContentLength, firecrawlApiKeyVisible, firecrawlLoading,
    firecrawlError, firecrawlSuccess,
    loadFirecrawlSettings, saveFirecrawlSettings,
  }
}
