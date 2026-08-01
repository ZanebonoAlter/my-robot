/**
 * 方向化 Peel 翻页转场（composable）
 *
 * 基于 GSAP + Vue `<Transition :css="false">` 的 JS hooks 实现可复用的翻页转场。
 * 从临时 demo（已删除）抽取并参数化：距离 / 旋转 / 时长 / 缓动。
 *
 * 两种语义方向：
 * - `horizontal`：横向翻页（同级内容，如日期）
 * - `vertical`：纵向翻页（跨类切换，如版块）
 *
 * 进出方向相反：旧内容向一侧卷起消失，新内容从对侧铺开出现。
 * 仅在切换瞬间触发，完成后清除内联 transform/clip-path/opacity，无残留特效。
 */
import gsap from 'gsap'
import type { Ref } from 'vue'

export type PeelDirection = 'horizontal' | 'vertical'

export interface PeelTransitionOptions {
  /** 卷起平移距离（px） */
  dist?: number
  /** 翻转旋转角度（deg） */
  rot?: number
  /** 进入时长（s） */
  enterDuration?: number
  /** 离开时长（s） */
  leaveDuration?: number
  /** 进入缓动 */
  enterEase?: string
  /** 离开缓动 */
  leaveEase?: string
}

/** 默认参数（demo 验证固化值） */
export const DEFAULT_PEEL_OPTIONS: Required<PeelTransitionOptions> = {
  dist: 64,
  rot: 16,
  enterDuration: 0.55,
  leaveDuration: 0.5,
  enterEase: 'power3.out',
  leaveEase: 'power2.in',
}

interface Axis {
  /** true=横向（x 轴），false=纵向（y 轴） */
  x: boolean
  /** 前进符号：固定 -1（与 demo 的 *-next 一致，新旧向相反方向运动） */
  d: -1 | 1
}

/** 方向→轴。固定"前进"符号 d=-1，保证进出始终相反且一致。 */
function axisOf(direction: PeelDirection): Axis {
  return direction === 'horizontal'
    ? { x: true, d: -1 }
    : { x: false, d: -1 }
}

/** 离开侧（=进入起始侧）的同侧裁剪 clip-path，形成"翻帘"感。 */
export function peelClipPath(direction: PeelDirection): string {
  const { x, d } = axisOf(direction)
  if (x) return d < 0 ? 'inset(0 0 0 100%)' : 'inset(0 100% 0 0)'
  return d < 0 ? 'inset(100% 0 0 0)' : 'inset(0 0 100% 0)'
}

const FULL_CLIP = 'inset(0% 0% 0% 0%)'

/** 进入：初始 gsap.set 属性（新页从对侧铺入）。 */
export function peelEnterInit(direction: PeelDirection, opts: Required<PeelTransitionOptions>) {
  const { x, d } = axisOf(direction)
  return {
    transformOrigin: x
      ? (d < 0 ? 'right center' : 'left center')
      : (d < 0 ? 'center bottom' : 'center top'),
    clipPath: peelClipPath(direction),
    x: x ? -opts.dist * d : 0,
    y: x ? 0 : -opts.dist * d,
    opacity: 0,
    rotateY: x ? -opts.rot * d : 0,
    rotateX: x ? 0 : -opts.rot * d,
  }
}

/** 进入：终态 gsap.to 目标（回归常态，与方向无关）。 */
export function peelEnterTarget(opts: Required<PeelTransitionOptions>) {
  return {
    duration: opts.enterDuration,
    ease: opts.enterEase,
    clipPath: FULL_CLIP,
    x: 0,
    y: 0,
    opacity: 1,
    rotateY: 0,
    rotateX: 0,
  }
}

/** 离开：transformOrigin（离开侧）。 */
export function peelLeaveOrigin(direction: PeelDirection): string {
  const { x, d } = axisOf(direction)
  return x
    ? (d < 0 ? 'left center' : 'right center')
    : (d < 0 ? 'center top' : 'center bottom')
}

/** 离开：终态 gsap.to 目标（向离开侧卷起）。 */
export function peelLeaveTarget(direction: PeelDirection, opts: Required<PeelTransitionOptions>) {
  const { x, d } = axisOf(direction)
  return {
    duration: opts.leaveDuration,
    ease: opts.leaveEase,
    clipPath: peelClipPath(direction),
    x: x ? opts.dist * d : 0,
    y: x ? 0 : opts.dist * d,
    opacity: 0,
    rotateY: x ? opts.rot * d : 0,
    rotateX: x ? 0 : opts.rot * d,
  }
}

function prefersReducedMotion(): boolean {
  return typeof window !== 'undefined'
    && typeof window.matchMedia === 'function'
    && window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

type DirectionSource = Ref<PeelDirection> | (() => PeelDirection)

function resolveDirection(source: DirectionSource): PeelDirection {
  return typeof source === 'function' ? source() : source.value
}

/**
 * Peel 转场 hooks 工厂。
 *
 * @param direction 方向来源（Ref 或 getter）；转场瞬间实时读取，确保拿到最新方向。
 * @param options   参数覆盖（缺省用 DEFAULT_PEEL_OPTIONS）。
 * @returns Vue Transition 的 JS hooks（beforeEnter / enter / afterEnter / beforeLeave / leave / afterLeave）。
 */
export function usePeelTransition(direction: DirectionSource, options: PeelTransitionOptions = {}) {
  const opts = { ...DEFAULT_PEEL_OPTIONS, ...options }

  function beforeEnter(el: Element) {
    gsap.set(el, { willChange: 'transform, clip-path, opacity' })
  }

  function onEnter(el: Element, done: () => void) {
    if (prefersReducedMotion()) {
      gsap.set(el, { clearProps: 'all' })
      done()
      return
    }
    gsap.set(el, peelEnterInit(resolveDirection(direction), opts))
    gsap.timeline({ onComplete: done }).to(el, peelEnterTarget(opts))
  }

  function afterEnter(el: Element) {
    // 转场完成：清除内联特效属性，回归静态可读（spec: 无残留特效）
    gsap.set(el, { willChange: 'auto', clearProps: 'transform,clipPath,opacity,rotateX,rotateY' })
  }

  function beforeLeave(el: Element) {
    // 离开元素脱离文档流覆盖在原位，避免与进入元素共同撑高导致布局跳动
    gsap.set(el, {
      position: 'absolute',
      top: 0,
      left: 0,
      right: 0,
      willChange: 'transform, clip-path, opacity',
    })
  }

  function onLeave(el: Element, done: () => void) {
    if (prefersReducedMotion()) {
      done()
      return
    }
    gsap.set(el, { transformOrigin: peelLeaveOrigin(resolveDirection(direction)) })
    gsap.timeline({ onComplete: done }).to(el, peelLeaveTarget(resolveDirection(direction), opts))
  }

  function afterLeave(el: Element) {
    gsap.set(el, {
      clearProps: 'position,top,left,right,willChange,transformOrigin,transform,clipPath,opacity,rotateX,rotateY',
    })
  }

  return {
    options: opts,
    beforeEnter,
    onEnter,
    afterEnter,
    beforeLeave,
    onLeave,
    afterLeave,
  }
}
