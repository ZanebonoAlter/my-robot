import { ref } from 'vue'
import { useProxyApi } from '~/api'

export function useProxyConfig() {
  const proxyUrl = ref('')
  const proxyConfigured = ref(false)
  const proxyLoading = ref(false)
  const proxyError = ref<string | null>(null)
  const proxySuccess = ref<string | null>(null)

  async function loadProxySettings() {
    proxyLoading.value = true
    proxyError.value = null
    try {
      const { getStatus } = useProxyApi()
      const response = await getStatus()
      if (response.success && response.data) {
        proxyUrl.value = response.data.http_proxy_url
        proxyConfigured.value = response.data.configured
      }
    } catch {
      proxyError.value = '加载代理设置失败'
    } finally {
      proxyLoading.value = false
    }
  }

  async function saveProxySettings() {
    proxyLoading.value = true
    proxyError.value = null
    proxySuccess.value = null
    try {
      const { saveSettings } = useProxyApi()
      const response = await saveSettings({
        http_proxy_url: proxyUrl.value,
      })
      if (!response.success) {
        throw new Error(response.error || '保存失败')
      }
      if (response.data) {
        proxyUrl.value = response.data.http_proxy_url
        proxyConfigured.value = response.data.configured
      }
      proxySuccess.value = '代理设置已保存并即时生效'
      setTimeout(() => { proxySuccess.value = null }, 2000)
    } catch {
      proxyError.value = '保存代理设置失败（请检查 URL 格式）'
    } finally {
      proxyLoading.value = false
    }
  }

  return {
    proxyUrl, proxyConfigured, proxyLoading,
    proxyError, proxySuccess,
    loadProxySettings, saveProxySettings,
  }
}
