import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { ref, computed } from 'vue'
import TagHierarchy from './TagHierarchy.vue'
import TagHierarchyRow from './TagHierarchyRow.vue'
import type { TagHierarchyNode } from '~/types/topicTag'

// Setup globals for Nuxt auto-imported functions
const testGlobals = globalThis as typeof globalThis & {
  ref: typeof ref
  computed: typeof computed
  useNotify: () => { error: (...args: unknown[]) => void; success: (...args: unknown[]) => void; warn: (...args: unknown[]) => void; dismiss: (...args: unknown[]) => void; toasts: never[] }
}

// Mocks
const mockNotify = {
  error: vi.fn(),
  success: vi.fn(),
  warn: vi.fn(),
  dismiss: vi.fn(),
  toasts: [],
}

testGlobals.ref = ref
testGlobals.computed = computed
testGlobals.useNotify = () => mockNotify

const fetchHierarchy = vi.fn()
const listWatchedTags = vi.fn()

vi.mock('~/api/abstractTags', () => ({
  useAbstractTagApi: () => ({
    fetchHierarchy,
    updateAbstractName: vi.fn(),
    detachChild: vi.fn(),
    reassignTag: vi.fn(),
  }),
}))

vi.mock('~/api/watchedTags', () => ({
  useWatchedTagsApi: () => ({
    listWatchedTags,
    watchTag: vi.fn(),
    unwatchTag: vi.fn(),
  }),
}))

vi.mock('~/features/topic-graph/composables/useOrganizeWebSocket', () => ({
  useOrganizeWebSocket: () => ({
    status: ref('idle'),
    totalUnclassified: ref(0),
    processed: ref(0),
    error: ref(null),
    reset: vi.fn(),
  }),
}))

vi.mock('@iconify/vue', () => ({
  Icon: { template: '<span />' },
}))

vi.mock('~/composables/useNotify', () => ({
  useNotify: () => mockNotify,
}))

afterEach(() => {
  vi.useRealTimers()
  fetchHierarchy.mockReset()
  listWatchedTags.mockReset()
})

function createNode(overrides: Partial<TagHierarchyNode> = {}): TagHierarchyNode {
  return {
    id: 1,
    label: 'AI Agent',
    slug: 'ai-agent',
    category: 'keyword',
    icon: 'mdi:tag',
    feedCount: 3,
    articleCount: 5,
    isActive: true,
    children: [],
    ...overrides,
  }
}

describe('TagHierarchyRow', () => {
  it('emits select-node when clicking a tag label', async () => {
    const node = createNode()
    const wrapper = mount(TagHierarchyRow, {
      props: {
        node,
        depth: 0,
        editingId: null,
        saving: false,
      },
    })

    vi.useFakeTimers()

    await wrapper.find('.th-label').trigger('click')
    await vi.advanceTimersByTimeAsync(250)

    expect(wrapper.emitted('select-node')).toEqual([[node]])
  })
})

describe('TagHierarchy', () => {
  it('re-emits selected tag from hierarchy rows', async () => {
    fetchHierarchy.mockResolvedValue({
      success: true,
      data: {
        nodes: [createNode()],
        total: 1,
      },
    })
    listWatchedTags.mockResolvedValue({ success: true, data: [] })

    const wrapper = mount(TagHierarchy, {
      props: {
        selectable: true,
      },
      global: {
        stubs: {
          Teleport: true,
        },
      },
    })

    await flushPromises()

    await wrapper.find('.th-label').trigger('click')
    await new Promise(resolve => setTimeout(resolve, 300))

    expect(wrapper.emitted('select-tag')).toEqual([['ai-agent', 'keyword']])
  })
})
