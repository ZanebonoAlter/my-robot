/**
 * Vitest setup – mocks Nuxt/Vue auto-imports not available in test environment.
 */
import { ref, computed, onUnmounted, onMounted, watch } from 'vue'

// Nuxt's useState returns a Ref<T>; we mock it with Vue's ref
// eslint-disable-next-line @typescript-eslint/no-explicit-any
globalThis.useState = function useState<T = any>(key: string, init?: () => T) {
  return init ? ref(init()) : ref()
}

// Nuxt's useRuntimeConfig returns runtime config
globalThis.useRuntimeConfig = function useRuntimeConfig() {
  return {
    public: {
      apiBase: 'http://localhost:5000',
      wsUrl: 'ws://localhost:5000',
    },
    app: { baseURL: '/' },
  }
}

// Vue composable APIs that may not be auto-imported in test env
globalThis.ref = ref
globalThis.computed = computed
globalThis.onUnmounted = onUnmounted
globalThis.onMounted = onMounted
globalThis.watch = watch
