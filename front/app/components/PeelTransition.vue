<script setup lang="ts">
/**
 * 方向化 Peel 翻页转场组件
 *
 * 包装 Vue `<Transition :css="false">`，内部用 usePeelTransition 编排 GSAP 动画。
 * 透传默认插槽；切换时由父组件改变插槽元素的 `:key` 触发转场。
 *
 * 用法：
 * ```vue
 * <PeelTransition :direction="direction" @end="onPeelEnd">
 *   <article :key="reportId">…</article>
 * </PeelTransition>
 * ```
 *
 * - `direction`：`horizontal`（横向翻）/ `vertical`（纵向翻），转场瞬间读取。
 * - `start` / `end`：转场起止事件，供父组件做动画锁。
 */
import {
  usePeelTransition,
  type PeelDirection,
  type PeelTransitionOptions,
} from '~/composables/usePeelTransition'

const props = defineProps<{
  direction: PeelDirection
  dist?: number
  rot?: number
  enterDuration?: number
  leaveDuration?: number
  enterEase?: string
  leaveEase?: string
}>()

const emit = defineEmits<{
  start: []
  end: []
}>()

const options: PeelTransitionOptions = {
  dist: props.dist,
  rot: props.rot,
  enterDuration: props.enterDuration,
  leaveDuration: props.leaveDuration,
  enterEase: props.enterEase,
  leaveEase: props.leaveEase,
}

const hooks = usePeelTransition(() => props.direction, options)

function handleBeforeEnter(el: Element) {
  emit('start')
  hooks.beforeEnter(el)
}

function handleAfterEnter(el: Element) {
  hooks.afterEnter(el)
  emit('end')
}

function handleAfterLeave(el: Element) {
  hooks.afterLeave(el)
  // 离开-only 转场（如切换到无内容的版块）不会触发 enter，此处兜底释放锁
  emit('end')
}
</script>

<template>
  <Transition
    :css="false"
    @before-enter="handleBeforeEnter"
    @enter="hooks.onEnter"
    @after-enter="handleAfterEnter"
    @before-leave="hooks.beforeLeave"
    @leave="hooks.onLeave"
    @after-leave="handleAfterLeave"
  >
    <slot />
  </Transition>
</template>
