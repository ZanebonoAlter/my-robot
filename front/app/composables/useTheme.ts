import { ref, readonly, computed } from 'vue'
import { useHead } from '#imports'

type Theme = 'editorial' | 'dark'

const STORAGE_KEY = 'syntopica-theme'

// 模块级单例 — 所有组件共享同一个状态
const currentTheme = ref<Theme>('editorial')

// 客户端初始化标记
let initialized = false

function initTheme() {
  if (initialized) return
  initialized = true
  if (import.meta.client) {
    const stored = localStorage.getItem(STORAGE_KEY) as Theme | null
    if (stored === 'editorial' || stored === 'dark') {
      currentTheme.value = stored
    }
  }
}

export function useTheme() {
  initTheme()

  function setTheme(theme: Theme) {
    currentTheme.value = theme
    if (import.meta.client) {
      localStorage.setItem(STORAGE_KEY, theme)
      document.documentElement.setAttribute('data-theme', theme)
    }
    useHead({ htmlAttrs: { 'data-theme': theme } })
  }

  function toggleTheme() {
    setTheme(currentTheme.value === 'editorial' ? 'dark' : 'editorial')
  }

  // SSR: 通过 useHead 设置 html 属性避免 FOUC
  useHead({ htmlAttrs: { 'data-theme': currentTheme.value } })

  return {
    theme: readonly(currentTheme),
    setTheme,
    toggleTheme,
    isDark: computed(() => currentTheme.value === 'dark'),
  }
}

export type { Theme }
