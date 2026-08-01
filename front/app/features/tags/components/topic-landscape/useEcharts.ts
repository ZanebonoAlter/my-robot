/**
 * ECharts 轻封装 composable（design D1）。
 *
 * 自研约 40 行封装，不引 vue-echarts：
 *  - echarts/core 模块化按需注册（Bar/Scatter/Line + Grid/Tooltip/DataZoom/Legend + Canvas）。
 *  - onMounted init / ResizeObserver 自动 resize / onBeforeUnmount dispose。
 *  - 暴露 elRef（模板绑定）、setOption（重建 option）、on（绑定 echarts 事件，懒挂载）。
 *
 * init 放 onMounted（client-only），SSR 安全；模板需用 <ClientOnly> 包裹图表容器（design D6）。
 */
import { onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'
import * as echarts from 'echarts/core'
import { BarChart, ScatterChart, LineChart } from 'echarts/charts'
import {
  GridComponent,
  TooltipComponent,
  DataZoomComponent,
  LegendComponent,
} from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import type { EChartsCoreOption, EChartsType } from 'echarts/core'

// 模块级注册一次（ES module 缓存，多组件 import 只执行一次；echarts.use 幂等）。
echarts.use([
  BarChart,
  ScatterChart,
  LineChart,
  GridComponent,
  TooltipComponent,
  DataZoomComponent,
  LegendComponent,
  CanvasRenderer,
])

/** 事件回调首个参数（仅取我们用到的字段，避免引入 echarts 内部深类型）。 */
export interface ChartEventHandlerParams {
  value?: unknown
  // 其余字段按需透传，组件不强类型化。
  [key: string]: unknown
}

/** echarts 实例的 on 最小结构视图（规避其重载泛型推断）。 */
type EChartsOnLike = {
  on: (event: string, handler: (params: unknown) => void) => void
}

export function useEcharts() {
  const elRef = ref<HTMLElement | null>(null)
  const chart = shallowRef<EChartsType | null>(null)
  // init 前注册的事件，init 后批量挂载。
  const pendingHandlers: Array<{ event: string; handler: (p: unknown) => void }> = []
  let ro: ResizeObserver | null = null

  /** 重建 option（notMerge=true，主题/数据切换时彻底替换，避免残留）。 */
  function setOption(option: EChartsCoreOption) {
    chart.value?.setOption(option, true)
  }

  /** 绑定 echarts 事件（init 前入队，init 后直挂）。 */
  function on(event: string, handler: (p: ChartEventHandlerParams) => void) {
    // 包一层适配：echarts 传入的 params 对象透传给业务 handler。
    const wrap = (p: unknown) => handler(p as ChartEventHandlerParams)
    if (chart.value) {
      (chart.value as unknown as EChartsOnLike).on(event, wrap)
    } else {
      pendingHandlers.push({ event, handler: wrap })
    }
  }

  /**
   * init：仅当容器已挂载且未 init 时执行一次。
   * 挂载时序防御：模板若被 <ClientOnly>/v-if 延迟渲染，onMounted 时 elRef 可能仍为 null，
   * 用 watch(elRef) 兜底在容器真正挂载后 init（曾因 <ClientOnly> 时序导致图表全部空白）。
   */
  function init() {
    if (chart.value || !elRef.value) return
    chart.value = echarts.init(elRef.value)
    const inst = chart.value as unknown as EChartsOnLike
    for (const { event, handler } of pendingHandlers) inst.on(event, handler)
    pendingHandlers.length = 0
    ro = new ResizeObserver(() => chart.value?.resize())
    ro.observe(elRef.value)
  }

  onMounted(init)
  // 兜底：容器晚于 onMounted 挂载（延迟渲染）时，挂载后立刻 init。
  watch(elRef, (el) => {
    if (el && !chart.value) init()
  })

  onBeforeUnmount(() => {
    ro?.disconnect()
    ro = null
    chart.value?.dispose()
    chart.value = null
  })

  return { elRef, setOption, on }
}
