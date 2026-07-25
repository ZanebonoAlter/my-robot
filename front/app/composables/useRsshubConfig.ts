import { ref } from 'vue'
import { useRsshubApi } from '~/api'

export function useRsshubConfig() {
  const rsshubBaseUrl = ref('')
  const rsshubDefault = ref('')
  const rsshubConfigured = ref(false)
  const rsshubLoading = ref(false)
  const rsshubError = ref<string | null>(null)
  const rsshubSuccess = ref<string | null>(null)

  async function loadRsshubSettings() {
    rsshubLoading.value = true
    rsshubError.value = null
    try {
      const { getStatus } = useRsshubApi()
      const response = await getStatus()
      if (response.success && response.data) {
        rsshubBaseUrl.value = response.data.rsshub_base_url
        rsshubDefault.value = response.data.default
        rsshubConfigured.value = response.data.configured
      }
    } catch {
      rsshubError.value = '加载 RSSHub 设置失败'
    } finally {
      rsshubLoading.value = false
    }
  }

  async function saveRsshubSettings() {
    rsshubLoading.value = true
    rsshubError.value = null
    rsshubSuccess.value = null
    try {
      const { saveSettings } = useRsshubApi()
      const response = await saveSettings({
        rsshub_base_url: rsshubBaseUrl.value,
      })
      if (!response.success) {
        throw new Error(response.error || '保存失败')
      }
      if (response.data) {
        rsshubBaseUrl.value = response.data.rsshub_base_url
        rsshubConfigured.value = response.data.configured
      }
      rsshubSuccess.value = 'RSSHub 设置已保存'
      setTimeout(() => { rsshubSuccess.value = null }, 2000)
    } catch {
      rsshubError.value = '保存 RSSHub 设置失败'
    } finally {
      rsshubLoading.value = false
    }
  }

  return {
    rsshubBaseUrl, rsshubDefault, rsshubConfigured, rsshubLoading,
    rsshubError, rsshubSuccess,
    loadRsshubSettings, saveRsshubSettings,
  }
}
