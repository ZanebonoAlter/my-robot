import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import type { Config } from 'driver.js'

// Mock driver.js — capture config passed to driver() and stub instance methods.
// (CSS import in the composable is a no-op under Vitest.)
const driveMock = vi.fn()
const destroyMock = vi.fn()
const driverMock = vi.fn<(config?: Config) => unknown>(() => ({
  drive: driveMock,
  destroy: destroyMock,
}))
vi.mock('driver.js', () => ({
  driver: driverMock,
}))

// Import AFTER mock declaration (hoisted by Vitest).
const { useOnboarding, __onboardingClient } = await import('./useOnboarding')

const STORAGE_KEY = 'syntopica_onboarding_complete'

/** Toggle the client guard (the import.meta.client token; not runtime-overridable under Vitest). */
function setClient(value: boolean) {
  __onboardingClient.value = value
}

describe('useOnboarding', () => {
  let reloadSpy: ReturnType<typeof vi.spyOn>

  beforeEach(() => {
    setClient(true)
    // Tear down any driver instance left over from a previous test — the
    // driverInstance is a module-level singleton, so without this, later
    // startTour() calls hit the `if (driverInstance) return` guard.
    useOnboarding().dismissTour()
    localStorage.clear()
    driverMock.mockClear()
    driveMock.mockClear()
    destroyMock.mockClear()
    // Stub location.reload so resetOnboarding() does not navigate away.
    reloadSpy = vi.spyOn(window.location, 'reload').mockImplementation(() => {})
  })

  afterEach(() => {
    reloadSpy.mockRestore()
    // Restore the client default for any tests running after this file.
    setClient(true)
  })

  describe('first-run detection (isFirstRun)', () => {
    it('is true when syntopica_onboarding_complete is absent', () => {
      const { isFirstRun } = useOnboarding()
      expect(isFirstRun.value).toBe(true)
    })

    it('is false when syntopica_onboarding_complete === "true"', () => {
      localStorage.setItem(STORAGE_KEY, 'true')
      const { isFirstRun } = useOnboarding()
      expect(isFirstRun.value).toBe(false)
    })
  })

  describe('dismissTour()', () => {
    it('writes syntopica_onboarding_complete = "true" to localStorage', () => {
      const { dismissTour } = useOnboarding()
      expect(localStorage.getItem(STORAGE_KEY)).toBeNull()
      dismissTour()
      expect(localStorage.getItem(STORAGE_KEY)).toBe('true')
    })

    it('destroys an active driver instance', async () => {
      const { startTour, dismissTour } = useOnboarding()
      await startTour()
      dismissTour()
      expect(destroyMock).toHaveBeenCalledTimes(1)
    })
  })

  describe('resetOnboarding()', () => {
    it('removes syntopica_onboarding_complete and reloads the page', () => {
      localStorage.setItem(STORAGE_KEY, 'true')
      const { resetOnboarding } = useOnboarding()
      resetOnboarding()
      expect(localStorage.getItem(STORAGE_KEY)).toBeNull()
      expect(reloadSpy).toHaveBeenCalledTimes(1)
    })
  })

  describe('client guard (import.meta.client = false)', () => {
    beforeEach(() => {
      setClient(false)
    })

    it('isFirstRun defaults to false', () => {
      const { isFirstRun } = useOnboarding()
      expect(isFirstRun.value).toBe(false)
    })

    it('isTourActive defaults to false', () => {
      const { isTourActive } = useOnboarding()
      expect(isTourActive.value).toBe(false)
    })

    it('startTour() does not access localStorage or create a driver instance', async () => {
      localStorage.setItem(STORAGE_KEY, 'guard-marker')
      const { startTour } = useOnboarding()
      await startTour()
      expect(driverMock).not.toHaveBeenCalled()
      // localStorage untouched by startTour under the guard
      expect(localStorage.getItem(STORAGE_KEY)).toBe('guard-marker')
    })
  })

  describe('missing element pre-filter', () => {
    it('keeps only the welcome step when no data-onboarding elements exist', async () => {
      const { startTour } = useOnboarding()
      await startTour()
      expect(driverMock).toHaveBeenCalledTimes(1)
      const config = driverMock.mock.calls[0]?.[0] as Config | undefined
      // welcome step (no element) always survives; 4 selector steps filtered out
      expect(config?.steps).toHaveLength(1)
      expect(config?.steps?.[0]?.popover?.title).toBe('欢迎使用 Syntopica')
    })

    it('keeps a step whose data-onboarding element is present in the DOM', async () => {
      const el = document.createElement('button')
      el.setAttribute('data-onboarding', 'nav-tags')
      document.body.appendChild(el)

      const { startTour } = useOnboarding()
      await startTour()

      const config = driverMock.mock.calls[0]?.[0] as Config | undefined
      const titles = (config?.steps ?? []).map(s => s.popover?.title)
      // welcome + nav-tags
      expect(titles).toContain('欢迎使用 Syntopica')
      expect(titles).toContain('标签管理')
      // absent selectors still filtered
      expect(titles).not.toContain('主题图谱')

      el.remove()
    })

    it('marks complete via onDestroyed when the tour ends', async () => {
      const { startTour } = useOnboarding()
      await startTour()
      const config = driverMock.mock.calls[0]?.[0] as Config | undefined
      expect(localStorage.getItem(STORAGE_KEY)).toBeNull()
      // The composable's onDestroyed ignores its args; invoke with none.
      ;(config?.onDestroyed as undefined | ((...args: unknown[]) => void))?.()
      expect(localStorage.getItem(STORAGE_KEY)).toBe('true')
    })
  })
})
