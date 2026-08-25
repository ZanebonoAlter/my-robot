import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import TopicWatchCreateDialog from './TopicWatchCreateDialog.vue'
import type { CreateWatchResult } from '~/api/topicWatches'

// —— api mock：组件自包含创建，全 mock 不依赖后端 ——
const apiMocks = vi.hoisted(() => ({
  createWatch: vi.fn(),
}))

vi.mock('~/api/topicWatches', () => ({
  useTopicWatchesApi: () => ({
    createWatch: apiMocks.createWatch,
  }),
}))

vi.mock('@iconify/vue', () => ({
  Icon: { name: 'Icon', inheritAttrs: true, template: '<span class="icon-stub" aria-hidden="true" />' },
}))

// —— fixture ——
function result(over: Partial<CreateWatchResult> = {}): CreateWatchResult {
  return {
    id: '42',
    semanticBoardId: '1974',
    label: '新关注',
    type: 'keyword_topic',
    status: 'active',
    createdAt: '2026-08-25T00:00:00Z',
    updatedAt: '2026-08-25T00:00:00Z',
    ...over,
  }
}

async function mountDialog(modelValue = true) {
  const wrapper = mount(TopicWatchCreateDialog, {
    props: { modelValue, boardId: 1974 },
    attachTo: document.body,
  })
  await nextTick()
  return wrapper
}

function dialogEl(): HTMLElement | null {
  return document.querySelector('.app-dialog')
}

/** AppDialog Teleport 到 body，事件需在实际 DOM 上派发。 */
function setInputValue(selector: string, value: string) {
  const input = document.querySelector(selector) as HTMLInputElement
  input.value = value
  input.dispatchEvent(new Event('input', { bubbles: true }))
}

beforeEach(() => {
  apiMocks.createWatch.mockReset()
  document.body.innerHTML = ''
})

// ============================================================
// 类型双选（物化轨；旧提示轨创建入口退役隐藏）
// ============================================================
describe('TopicWatchCreateDialog — 类型双选（物化轨）', () => {
  it('仅两张物化轨卡片：label / keyword 旧类型卡不存在', async () => {
    const wrapper = await mountDialog()
    expect(dialogEl()).not.toBeNull()
    expect(dialogEl()!.querySelector('[data-testid="watch-type-keyword_topic"]')).not.toBeNull()
    expect(dialogEl()!.querySelector('[data-testid="watch-type-sentence_topic"]')).not.toBeNull()
    // 退役：旧提示轨卡不再渲染
    expect(dialogEl()!.querySelector('[data-testid="watch-type-label"]')).toBeNull()
    expect(dialogEl()!.querySelector('[data-testid="watch-type-keyword"]')).toBeNull()

    wrapper.unmount()
  })

  it('默认 keyword_topic 态：关键字输入 + 语法提示 + 解析预览 + 物化说明三件套', async () => {
    const wrapper = await mountDialog()

    const kwCard = dialogEl()!.querySelector('[data-testid="watch-type-keyword_topic"]')!
    expect(kwCard.className).toContain('is-on')
    expect(dialogEl()!.querySelector('[data-testid="watch-keyword-input"]')).not.toBeNull()
    expect(dialogEl()!.textContent).toContain('空格')
    expect(dialogEl()!.textContent).toContain('当天文章标题 + 摘要')
    expect(dialogEl()!.querySelector('[data-testid="keyword-parse-preview"]')).not.toBeNull()
    expect(dialogEl()!.textContent).toContain('聚合成固定名称的独立板块')
    // sentence 输入不可见
    expect(dialogEl()!.querySelector('[data-testid="watch-sentence-name-input"]')).toBeNull()

    wrapper.unmount()
  })

  it('切到 sentence_topic：话题名 + 检索句输入 + 生效说明', async () => {
    const wrapper = await mountDialog()
    await (dialogEl()!.querySelector('[data-testid="watch-type-sentence_topic"]')! as HTMLElement).click()
    await nextTick()

    expect(dialogEl()!.querySelector('[data-testid="watch-sentence-name-input"]')).not.toBeNull()
    expect(dialogEl()!.querySelector('[data-testid="watch-sentence-query-input"]')).not.toBeNull()
    expect(dialogEl()!.textContent).toContain('跨天延续成一条话题生命线')
    // keyword 三件套不可见
    expect(dialogEl()!.querySelector('[data-testid="watch-keyword-input"]')).toBeNull()

    wrapper.unmount()
  })

  it('切换类型清空错误提示（不残留上次的提交错误）', async () => {
    const wrapper = await mountDialog()
    await (dialogEl()!.querySelector('[data-testid="watch-type-sentence_topic"]')! as HTMLElement).click()
    await nextTick()
    // 输入后清空，Enter 提交触发话题名空值错误
    setInputValue('[data-testid="watch-sentence-name-input"] input', '先输入')
    await nextTick()
    setInputValue('[data-testid="watch-sentence-name-input"] input', '')
    await nextTick()
    const input = document.querySelector('[data-testid="watch-sentence-name-input"] input') as HTMLInputElement
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }))
    await nextTick()
    expect(dialogEl()!.textContent).toContain('请填写话题名')
    // 切回 keyword_topic，错误清空
    await (dialogEl()!.querySelector('[data-testid="watch-type-keyword_topic"]')! as HTMLElement).click()
    await nextTick()
    expect(dialogEl()!.textContent).not.toContain('请填写话题名')

    wrapper.unmount()
  })
})

// ============================================================
// 解析预览 chips（keyword_topic：实时解析预览，chips 展示解析结果）
// ============================================================
describe('TopicWatchCreateDialog — keyword 解析预览', () => {
  it('混用表达式：ASML|镓锗 出口 → [ASML|镓锗] × [出口]', async () => {
    const wrapper = await mountDialog()
    setInputValue('[data-testid="watch-keyword-input"] input', 'ASML|镓锗 出口')
    await nextTick()

    const slots = dialogEl()!.querySelectorAll('[data-testid="keyword-parse-slot"]')
    expect(slots).toHaveLength(2)
    expect(slots[0]!.textContent!.replace(/\s+/g, '')).toBe('ASML|镓锗')
    expect(slots[1]!.textContent).toBe('出口')
    // 槽间「且」分隔符
    expect(dialogEl()!.querySelector('.twcd-preview__and')).not.toBeNull()
    // 无红字
    expect(dialogEl()!.querySelector('[data-testid="keyword-parse-invalid"] p')).toBeNull()

    wrapper.unmount()
  })

  it('单词 / 多词 AND / 多词 OR 的预览', async () => {
    const wrapper = await mountDialog()

    setInputValue('[data-testid="watch-keyword-input"] input', 'ASML')
    await nextTick()
    expect(dialogEl()!.querySelectorAll('[data-testid="keyword-parse-slot"]')).toHaveLength(1)

    setInputValue('[data-testid="watch-keyword-input"] input', '出口 限制')
    await nextTick()
    const andSlots = dialogEl()!.querySelectorAll('[data-testid="keyword-parse-slot"]')
    expect(andSlots).toHaveLength(2)
    expect(andSlots[0]!.textContent).toBe('出口')
    expect(andSlots[1]!.textContent).toBe('限制')

    setInputValue('[data-testid="watch-keyword-input"] input', 'ASML|镓锗')
    await nextTick()
    expect(dialogEl()!.querySelectorAll('[data-testid="keyword-parse-slot"]')).toHaveLength(1)

    wrapper.unmount()
  })

  it('纯分隔符输入：红字提示 + 提交禁用（与后端 400 对齐）', async () => {
    const wrapper = await mountDialog()
    setInputValue('[data-testid="watch-keyword-input"] input', '| |')
    await nextTick()

    expect(dialogEl()!.querySelectorAll('[data-testid="keyword-parse-slot"]')).toHaveLength(0)
    expect(dialogEl()!.textContent).toContain('未解析出有效关键字')

    const submit = dialogEl()!.querySelector('[data-testid="watch-create-submit"]') as HTMLButtonElement
    expect(submit.disabled).toBe(true)
    expect(apiMocks.createWatch).not.toHaveBeenCalled()

    wrapper.unmount()
  })

  it('空串 / 槽内空洞（ASML|）同样禁提交', async () => {
    const wrapper = await mountDialog()

    // 空串：提示引导文案（非错误红字），提交禁用
    let submit = dialogEl()!.querySelector('[data-testid="watch-create-submit"]') as HTMLButtonElement
    expect(submit.disabled).toBe(true)
    expect(dialogEl()!.textContent).toContain('输入表达式后这里会实时预览解析结果')

    // 槽内空洞
    setInputValue('[data-testid="watch-keyword-input"] input', 'ASML|')
    await nextTick()
    submit = dialogEl()!.querySelector('[data-testid="watch-create-submit"]') as HTMLButtonElement
    expect(submit.disabled).toBe(true)
    expect(dialogEl()!.textContent).toContain('未解析出有效关键字')

    wrapper.unmount()
  })
})

// ============================================================
// 提交（keyword_topic / sentence_topic 分型 + 空值误输入反馈）
// ============================================================
describe('TopicWatchCreateDialog — 提交', () => {
  it('sentence 态话题名空值误提交：错误提示可见且已输内容保留', async () => {
    const wrapper = await mountDialog()
    await (dialogEl()!.querySelector('[data-testid="watch-type-sentence_topic"]')! as HTMLElement).click()
    await nextTick()
    // 先输入再清空，模拟「输过又删掉」
    setInputValue('[data-testid="watch-sentence-name-input"] input', 'AI 进展')
    await nextTick()
    setInputValue('[data-testid="watch-sentence-name-input"] input', '')
    await nextTick()
    await (dialogEl()!.querySelector('[data-testid="watch-create-submit"]')! as HTMLElement).click()
    await nextTick()

    expect(apiMocks.createWatch).not.toHaveBeenCalled()

    // 输入内容保留断言
    setInputValue('[data-testid="watch-sentence-name-input"] input', '半句话')
    await nextTick()
    const input = document.querySelector('[data-testid="watch-sentence-name-input"] input') as HTMLInputElement
    expect(input.value).toBe('半句话')

    wrapper.unmount()
  })

  it('sentence 态正常提交：话题名 + 检索句透传，检索句为空不传（回退话题名）', async () => {
    apiMocks.createWatch.mockResolvedValue({ success: true, data: result({ id: '9', label: 'AI 编程工具进展', type: 'sentence_topic' }) })
    const wrapper = await mountDialog()
    await (dialogEl()!.querySelector('[data-testid="watch-type-sentence_topic"]')! as HTMLElement).click()
    await nextTick()
    setInputValue('[data-testid="watch-sentence-name-input"] input', 'AI 编程工具进展')
    await nextTick()

    await (dialogEl()!.querySelector('[data-testid="watch-create-submit"]')! as HTMLElement).click()
    await flushPromises()

    expect(apiMocks.createWatch).toHaveBeenCalledWith(1974, 'AI 编程工具进展', 'sentence_topic', undefined)
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([false])
    expect(wrapper.emitted('created')?.[0]?.[0]).toMatchObject({ id: '9', type: 'sentence_topic' })

    wrapper.unmount()
  })

  it('sentence 态带检索句提交：query 透传', async () => {
    apiMocks.createWatch.mockResolvedValue({ success: true, data: result({ type: 'sentence_topic' }) })
    const wrapper = await mountDialog()
    await (dialogEl()!.querySelector('[data-testid="watch-type-sentence_topic"]')! as HTMLElement).click()
    await nextTick()
    setInputValue('[data-testid="watch-sentence-name-input"] input', 'AI 编程工具进展')
    setInputValue('[data-testid="watch-sentence-query-input"] input', 'AI coding assistant 的进展')
    await nextTick()

    await (dialogEl()!.querySelector('[data-testid="watch-create-submit"]')! as HTMLElement).click()
    await flushPromises()

    expect(apiMocks.createWatch).toHaveBeenCalledWith(1974, 'AI 编程工具进展', 'sentence_topic', 'AI coding assistant 的进展')

    wrapper.unmount()
  })

  it('keyword_topic 态提交：type=keyword_topic 透传', async () => {
    apiMocks.createWatch.mockResolvedValue({
      success: true,
      data: result({ id: '10', label: 'ASML|镓锗 出口' }),
    })
    const wrapper = await mountDialog()
    setInputValue('[data-testid="watch-keyword-input"] input', 'ASML|镓锗 出口')
    await nextTick()

    await (dialogEl()!.querySelector('[data-testid="watch-create-submit"]')! as HTMLElement).click()
    await flushPromises()

    expect(apiMocks.createWatch).toHaveBeenCalledWith(1974, 'ASML|镓锗 出口', 'keyword_topic')
    expect(wrapper.emitted('created')?.[0]?.[0]).toMatchObject({ type: 'keyword_topic' })

    wrapper.unmount()
  })

  it('提交失败：错误提示可见、输入内容保留、对话框不关闭', async () => {
    apiMocks.createWatch.mockResolvedValue({ success: false, error: '关键字表达式无效' })
    const wrapper = await mountDialog()
    setInputValue('[data-testid="watch-keyword-input"] input', 'ASML|镓锗 出口')
    await nextTick()

    await (dialogEl()!.querySelector('[data-testid="watch-create-submit"]')! as HTMLElement).click()
    await flushPromises()

    expect(dialogEl()!.textContent).toContain('关键字表达式无效')
    const input = document.querySelector('[data-testid="watch-keyword-input"] input') as HTMLInputElement
    expect(input.value).toBe('ASML|镓锗 出口')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()

    wrapper.unmount()
  })

  it('重新打开重置类型与输入（面板复用同一实例）', async () => {
    apiMocks.createWatch.mockResolvedValue({ success: true, data: result() })
    const wrapper = await mountDialog()
    setInputValue('[data-testid="watch-keyword-input"] input', 'ASML')
    await nextTick()
    await (dialogEl()!.querySelector('[data-testid="watch-create-submit"]')! as HTMLElement).click()
    await flushPromises()

    // 重新打开
    await wrapper.setProps({ modelValue: false })
    await wrapper.setProps({ modelValue: true })
    await nextTick()

    // 回到 keyword_topic 态、输入为空
    expect(dialogEl()!.querySelector('[data-testid="watch-type-keyword_topic"]')!.className).toContain('is-on')
    expect((dialogEl()!.querySelector('[data-testid="watch-keyword-input"] input')! as HTMLInputElement).value).toBe('')

    wrapper.unmount()
  })
})
