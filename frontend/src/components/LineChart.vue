<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'
import * as echarts from 'echarts/core'
import { LineChart } from 'echarts/charts'
import {
  GridComponent,
  LegendComponent,
  TooltipComponent,
} from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { useThemeStore } from '@/stores/theme'

// Chỉ nạp đúng các thành phần cần dùng thay vì cả thư viện ECharts —
// giảm đáng kể kích thước gói mà giao diện phải tải về.
echarts.use([LineChart, GridComponent, TooltipComponent, LegendComponent, CanvasRenderer])

export interface Series {
  name: string
  data: number[]
  color: string
}

const props = defineProps<{
  labels: string[]
  series: Series[]
  /** Hậu tố đơn vị hiện trong chú giải, ví dụ "%" hoặc "B/s". */
  unit?: string
  /** Cố định trần trục tung; để trống thì ECharts tự co giãn. */
  max?: number
  /** Hàm định dạng giá trị hiển thị trong chú giải. */
  formatter?: (value: number) => string
  height?: string
}>()

const theme = useThemeStore()
const container = ref<HTMLElement | null>(null)
const chart = shallowRef<echarts.ECharts | null>(null)
let observer: ResizeObserver | null = null

function buildOption(): echarts.EChartsCoreOption {
  const axisColor = theme.isDark ? '#2d2d31' : '#e5e7eb'
  const textColor = theme.isDark ? '#9ca3af' : '#6b7280'
  const format = props.formatter ?? ((v: number) => `${v.toFixed(1)}${props.unit ?? ''}`)

  return {
    animation: false,
    // containLabel để ECharts tự chừa đủ chỗ cho nhãn trục — nhãn tốc độ mạng
    // ("123.4 KB/s") dài hơn nhãn phần trăm rất nhiều, lề cố định sẽ cắt cụt nó.
    grid: { top: 32, right: 16, bottom: 8, left: 8, containLabel: true },
    tooltip: {
      trigger: 'axis',
      valueFormatter: (value: unknown) => format(Number(value)),
    },
    legend: {
      show: props.series.length > 1,
      top: 0,
      right: 0,
      textStyle: { color: textColor },
      icon: 'roundRect',
      itemWidth: 10,
      itemHeight: 10,
    },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: props.labels,
      axisLine: { lineStyle: { color: axisColor } },
      axisLabel: { color: textColor, fontSize: 11 },
      axisTick: { show: false },
    },
    yAxis: {
      type: 'value',
      max: props.max,
      splitLine: { lineStyle: { color: axisColor, type: 'dashed' } },
      axisLabel: { color: textColor, fontSize: 11, formatter: format },
    },
    series: props.series.map((s) => ({
      name: s.name,
      type: 'line',
      data: s.data,
      smooth: true,
      // Chuỗi chỉ có một hai điểm thì không có đoạn thẳng nào để vẽ: giấu điểm
      // đi là biểu đồ trống trơn trong khi số liệu vẫn có.
      showSymbol: s.data.length <= 2,
      lineStyle: { width: 2, color: s.color },
      itemStyle: { color: s.color },
      areaStyle: {
        color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
          { offset: 0, color: `${s.color}40` },
          { offset: 1, color: `${s.color}00` },
        ]),
      },
    })),
  }
}

function render(): void {
  chart.value?.setOption(buildOption(), { notMerge: true })
}

onMounted(() => {
  if (!container.value) return

  chart.value = echarts.init(container.value)
  render()

  // Biểu đồ nằm trong bố cục co giãn, phải vẽ lại khi khung chứa đổi kích thước.
  observer = new ResizeObserver(() => chart.value?.resize())
  observer.observe(container.value)
})

onBeforeUnmount(() => {
  observer?.disconnect()
  chart.value?.dispose()
  chart.value = null
})

// Đổi giao diện sáng/tối phải vẽ lại vì màu trục và chữ được tính sẵn.
watch(() => [props.labels, props.series, theme.isDark], render, { deep: true })
</script>

<template>
  <div ref="container" class="chart" :style="{ height: height ?? '260px' }" />
</template>

<style scoped>
.chart {
  width: 100%;
}
</style>
