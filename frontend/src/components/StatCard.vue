<script setup lang="ts">
import { computed } from 'vue'
import { NCard } from 'naive-ui'

const props = defineProps<{
  label: string
  /** Giá trị phần trăm hiện tại. */
  percent: number
  /** Dòng chú thích phía dưới, ví dụ "3.2 GB / 16 GB". */
  detail?: string
  /** Chuỗi giá trị gần đây (%) để vẽ đường xu hướng ở đáy thẻ. */
  series?: number[]
}>()

/**
 * Màu đổi theo mức sử dụng để mắt bắt được tình trạng nguy hiểm ngay lập tức,
 * không phải đọc con số: xanh lá bình thường, hổ phách cần chú ý, đỏ nguy cấp.
 * Cùng ngưỡng với thanh trạng thái ở menu.
 */
const tone = computed(() => {
  if (props.percent >= 90) return 'var(--sp-danger)'
  if (props.percent >= 75) return 'var(--sp-warn)'
  return 'var(--sp-action)'
})

const clamped = computed(() => Math.min(100, Math.max(0, props.percent)))

/** Kích thước hệ tọa độ của đường xu hướng; SVG tự co giãn theo bề rộng thẻ. */
const VIEW = { w: 100, h: 28 } as const

/**
 * Con số chỉ nói máy đang thế nào *lúc này*. Đường xu hướng ở đáy thẻ trả lời
 * câu hỏi thực sự quan trọng: đang đi lên hay vừa hạ xuống. Vẽ thẳng bằng SVG
 * thay vì gọi thư viện biểu đồ — bốn thẻ này cập nhật mỗi giây.
 */
const points = computed(() => {
  const data = props.series ?? []
  if (data.length < 2) return ''

  const step = VIEW.w / (data.length - 1)
  return data
    .map((value, index) => {
      const y = VIEW.h - (Math.min(100, Math.max(0, value)) / 100) * VIEW.h
      return `${(index * step).toFixed(2)},${y.toFixed(2)}`
    })
    .join(' ')
})

/** Cùng đường đó nhưng khép kín xuống đáy để tô nền mờ dưới nét. */
const area = computed(() => (points.value ? `0,${VIEW.h} ${points.value} ${VIEW.w},${VIEW.h}` : ''))
</script>

<template>
  <NCard size="small" class="stat-card">
    <div class="stat-label sp-eyebrow">{{ label }}</div>
    <div class="stat-figure sp-figure" :style="{ color: tone }">{{ clamped.toFixed(1) }}%</div>
    <div v-if="detail" class="stat-detail sp-metric">{{ detail }}</div>

    <svg
      v-if="points"
      class="stat-spark"
      :viewBox="`0 0 ${VIEW.w} ${VIEW.h}`"
      preserveAspectRatio="none"
      aria-hidden="true"
    >
      <polygon :points="area" :fill="tone" opacity="0.1" />
      <polyline
        :points="points"
        fill="none"
        :stroke="tone"
        stroke-width="1.4"
        vector-effect="non-scaling-stroke"
        stroke-linejoin="round"
      />
    </svg>

    <!-- Chưa đủ mẫu để vẽ đường: giữ đúng chỗ đó bằng một vạch mức, để thẻ không
         nhảy cao khi mẫu thứ hai tới. -->
    <div v-else class="stat-bar">
      <div class="stat-bar-fill" :style="{ width: `${clamped}%`, background: tone }"></div>
    </div>
  </NCard>
</template>

<style scoped>
.stat-card {
  position: relative;
  overflow: hidden;
}

.stat-card :deep(.n-card__content) {
  padding-bottom: 34px;
}

.stat-label {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.stat-figure {
  margin-top: 6px;
}

.stat-detail {
  margin-top: 4px;
  overflow: hidden;
  font-size: 12px;
  color: var(--sp-text-muted);
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* Đường xu hướng chạm mép thẻ: nó là nền của con số, không phải một ô riêng. */
.stat-spark {
  position: absolute;
  right: 0;
  bottom: 0;
  left: 0;
  display: block;
  width: 100%;
  height: 28px;
}

.stat-bar {
  position: absolute;
  right: 0;
  bottom: 0;
  left: 0;
  height: 3px;
  background: var(--sp-surface-sunken);
}

.stat-bar-fill {
  height: 100%;
  transition: width 0.4s ease, background 0.4s ease;
}
</style>
