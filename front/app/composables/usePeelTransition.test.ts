import { describe, expect, it } from 'vitest'
import { ref } from 'vue'
import {
  DEFAULT_PEEL_OPTIONS,
  peelClipPath,
  peelEnterInit,
  peelEnterTarget,
  peelLeaveOrigin,
  peelLeaveTarget,
  usePeelTransition,
  type PeelDirection,
} from './usePeelTransition'

describe('usePeelTransition - 纯函数（horizontal）', () => {
  const opts = DEFAULT_PEEL_OPTIONS

  it('peelClipPath 横向裁剪从左侧（前进方向 d=-1）', () => {
    expect(peelClipPath('horizontal')).toBe('inset(0 0 0 100%)')
  })

  it('peelEnterInit 初始态：从右侧铺入，带平移/旋转/透明', () => {
    const init = peelEnterInit('horizontal', opts)
    // x 轴：transformOrigin 右侧；clipPath 同侧裁剪；x 正向偏移；rotateY 正向
    expect(init.transformOrigin).toBe('right center')
    expect(init.clipPath).toBe('inset(0 0 0 100%)')
    expect(init.x).toBe(opts.dist) // -dist*d = -64*-1 = 64
    expect(init.y).toBe(0)
    expect(init.opacity).toBe(0)
    expect(init.rotateY).toBe(opts.rot) // -rot*d = 16
    expect(init.rotateX).toBe(0)
  })

  it('peelEnterTarget 终态：回归常态（满裁剪/无偏移/不透明）', () => {
    const target = peelEnterTarget(opts)
    expect(target.clipPath).toBe('inset(0% 0% 0% 0%)')
    expect(target.x).toBe(0)
    expect(target.y).toBe(0)
    expect(target.opacity).toBe(1)
    expect(target.rotateY).toBe(0)
    expect(target.rotateX).toBe(0)
    expect(target.duration).toBe(opts.enterDuration)
    expect(target.ease).toBe(opts.enterEase)
  })

  it('peelLeaveOrigin 离开侧 transformOrigin 为左侧', () => {
    expect(peelLeaveOrigin('horizontal')).toBe('left center')
  })

  it('peelLeaveTarget 终态：向左侧卷起消失', () => {
    const target = peelLeaveTarget('horizontal', opts)
    expect(target.clipPath).toBe('inset(0 0 0 100%)')
    expect(target.x).toBe(-opts.dist) // dist*d = 64*-1 = -64（向左飞出）
    expect(target.y).toBe(0)
    expect(target.opacity).toBe(0)
    expect(target.rotateY).toBe(-opts.rot) // rot*d = -16
    expect(target.rotateX).toBe(0)
    expect(target.duration).toBe(opts.leaveDuration)
    expect(target.ease).toBe(opts.leaveEase)
  })
})

describe('usePeelTransition - 纯函数（vertical）', () => {
  const opts = DEFAULT_PEEL_OPTIONS

  it('peelClipPath 纵向裁剪从顶部', () => {
    expect(peelClipPath('vertical')).toBe('inset(100% 0 0 0)')
  })

  it('peelEnterInit 初始态：从底部铺入', () => {
    const init = peelEnterInit('vertical', opts)
    expect(init.transformOrigin).toBe('center bottom')
    expect(init.clipPath).toBe('inset(100% 0 0 0)')
    expect(init.x).toBe(0)
    expect(init.y).toBe(opts.dist) // -dist*d = 64
    expect(init.opacity).toBe(0)
    expect(init.rotateY).toBe(0)
    expect(init.rotateX).toBe(opts.rot) // -rot*d = 16
  })

  it('peelLeaveOrigin 离开侧 transformOrigin 为顶部', () => {
    expect(peelLeaveOrigin('vertical')).toBe('center top')
  })

  it('peelLeaveTarget 终态：向顶部卷起消失', () => {
    const target = peelLeaveTarget('vertical', opts)
    expect(target.clipPath).toBe('inset(100% 0 0 0)')
    expect(target.x).toBe(0)
    expect(target.y).toBe(-opts.dist) // dist*d = -64（向上飞出）
    expect(target.opacity).toBe(0)
    expect(target.rotateY).toBe(0)
    expect(target.rotateX).toBe(-opts.rot) // rot*d = -16
  })
})

describe('usePeelTransition - 工厂与参数覆盖', () => {
  it('默认参数与 demo 固化值一致', () => {
    expect(DEFAULT_PEEL_OPTIONS).toEqual({
      dist: 64,
      rot: 16,
      enterDuration: 0.55,
      leaveDuration: 0.5,
      enterEase: 'power3.out',
      leaveEase: 'power2.in',
    })
  })

  it('支持参数覆盖', () => {
    const { options } = usePeelTransition(ref('horizontal'), { dist: 100, rot: 20 })
    expect(options.dist).toBe(100)
    expect(options.rot).toBe(20)
    // 未覆盖项保留默认
    expect(options.enterDuration).toBe(0.55)
    expect(options.leaveEase).toBe('power2.in')
  })

  it('返回全部 6 个 Transition JS hooks', () => {
    const hooks = usePeelTransition(ref('horizontal'))
    for (const name of ['beforeEnter', 'onEnter', 'afterEnter', 'beforeLeave', 'onLeave', 'afterLeave']) {
      expect(typeof hooks[name as keyof typeof hooks]).toBe('function')
    }
  })

  it('同时支持 getter 与 Ref 作为方向来源', () => {
    const dir = ref<PeelDirection>('horizontal' as const)
    const fromRef = usePeelTransition(dir)
    const fromGetter = usePeelTransition(() => 'vertical')
    // 仅断言不抛错并返回 hooks（方向在转场瞬间由 hooks 内部读取）
    expect(typeof fromRef.onEnter).toBe('function')
    expect(typeof fromGetter.onLeave).toBe('function')
  })
})
