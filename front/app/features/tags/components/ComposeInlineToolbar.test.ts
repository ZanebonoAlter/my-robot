/**
 * ComposeInlineToolbar — 就地编排态顶部浮工具条测试（inline-compose-lane 切片②）.
 *
 * 纯展示组件，只测 props→DOM/emit 接线（composable 逻辑由 D1 覆盖）：
 *  - 计数正确显示（已勾总数 = unassigned + moveOut；平均距离 3 位小数）
 *  - moveOutCount>0 显移出副标（警示色）；===0 隐藏
 *  - outlierCount>0 离群警示色
 *  - canSave/saving → 保存按钮禁用；点保存 emit('save')；点取消 emit('cancel')
 *  - 泳道名输入 emit('update:laneName')
 */
import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import ComposeInlineToolbar from './ComposeInlineToolbar.vue'
import AppButton from '~/components/ui/AppButton.vue'

function mountToolbar(over: Record<string, unknown> = {}) {
  return mount(ComposeInlineToolbar, {
    props: {
      laneName: '',
      memberCount: 0,
      meanDistance: 0,
      outlierCount: 0,
      unassignedCount: 0,
      moveOutCount: 0,
      saving: false,
      canSave: false,
      ...over,
    },
  })
}

/** 模板里取消在前、保存在后，故 AppButton 组件顺序固定为 [取消, 保存]。 */
function getButtons(wrapper: ReturnType<typeof mountToolbar>) {
  const buttons = wrapper.findAllComponents(AppButton)
  return { cancel: buttons[0]!, save: buttons[1]! }
}

describe('ComposeInlineToolbar — 计数与质量卡', () => {
  it('已勾总数 = unassigned + moveOut；成员/离群数正确显示', () => {
    const wrapper = mountToolbar({
      unassignedCount: 3,
      moveOutCount: 2,
      memberCount: 5,
      outlierCount: 1,
    })
    expect(wrapper.text()).toContain('已勾 5 条')
    expect(wrapper.text()).toContain('成员 5')
    expect(wrapper.text()).toContain('离群 1')
  })

  it('平均距离保留 3 位小数', () => {
    const wrapper = mountToolbar({ meanDistance: 0.1234 })
    expect(wrapper.text()).toContain('平均距离 0.123')
  })

  it('moveOutCount>0 显移出副标（警示色）；outlierCount>0 离群警示色', () => {
    const wrapper = mountToolbar({ moveOutCount: 2, outlierCount: 1 })
    const sub = wrapper.find('.cit-count__moveout')
    expect(sub.exists()).toBe(true)
    expect(sub.text()).toContain('2 条来自现有泳道·将移出')
    expect(wrapper.find('.cit-quality__item.is-warning').exists()).toBe(true)
  })

  it('moveOutCount===0 不渲染移出副标', () => {
    const wrapper = mountToolbar({ moveOutCount: 0 })
    expect(wrapper.find('.cit-count__moveout').exists()).toBe(false)
  })
})

describe('ComposeInlineToolbar — 按钮状态与事件', () => {
  it('canSave=false 时保存按钮禁用', () => {
    const wrapper = mountToolbar({ canSave: false })
    expect(getButtons(wrapper).save.props('disabled')).toBe(true)
  })

  it('canSave=true 时保存按钮可点', () => {
    const wrapper = mountToolbar({ canSave: true, memberCount: 1 })
    expect(getButtons(wrapper).save.props('disabled')).toBe(false)
  })

  it('saving=true 时保存按钮禁用且 loading', () => {
    const wrapper = mountToolbar({ canSave: true, saving: true })
    const { save } = getButtons(wrapper)
    expect(save.props('disabled')).toBe(true)
    expect(save.props('loading')).toBe(true)
  })

  it('moveOutCount>0 时保存按钮文案带移出提示', () => {
    const wrapper = mountToolbar({ canSave: true, moveOutCount: 3 })
    expect(getButtons(wrapper).save.text()).toContain('保存（含 3 条移出）')
  })

  it('点保存 emit("save")', async () => {
    const wrapper = mountToolbar({ canSave: true })
    await getButtons(wrapper).save.trigger('click')
    expect(wrapper.emitted('save')).toBeTruthy()
  })

  it('点取消 emit("cancel")', async () => {
    const wrapper = mountToolbar()
    await getButtons(wrapper).cancel.trigger('click')
    expect(wrapper.emitted('cancel')).toBeTruthy()
  })
})

describe('ComposeInlineToolbar — 泳道名输入', () => {
  it('输入泳道名 emit("update:laneName")', async () => {
    const wrapper = mountToolbar({ laneName: '' })
    await wrapper.find('input').setValue('美伊博弈')
    const evt = wrapper.emitted('update:laneName')
    expect(evt).toBeTruthy()
    expect(evt![0]).toEqual(['美伊博弈'])
  })
})
