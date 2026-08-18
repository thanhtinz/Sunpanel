<script setup lang="ts">
import { computed } from 'vue'
import { NCard, NProgress, NText } from 'naive-ui'

const props = defineProps<{
  label: string
  /** Giá trị phần trăm dùng cho vòng tiến độ. */
  percent: number
  /** Dòng chú thích phía dưới, ví dụ "3.2 GB / 16 GB". */
  detail?: string
}>()

/**
 * Màu đổi theo mức sử dụng để mắt bắt được tình trạng nguy hiểm ngay lập tức,
 * không phải đọc con số: xanh lá bình thường, cam cảnh báo, đỏ nguy cấp.
 */
const color = computed(() => {
  if (props.percent >= 90) return '#d03050'
  if (props.percent >= 75) return '#f0a020'
  return '#18a058'
})

const clamped = computed(() => Math.min(100, Math.max(0, props.percent)))
</script>

<template>
  <NCard size="small" class="stat-card">
    <div class="stat-body">
      <NProgress
        type="circle"
        :percentage="clamped"
        :color="color"
        :stroke-width="8"
        class="stat-ring"
      >
        <span class="stat-value sp-metric">{{ clamped.toFixed(1) }}%</span>
      </NProgress>

      <div class="stat-meta">
        <NText strong class="stat-label">{{ label }}</NText>
        <NText v-if="detail" depth="3" class="stat-detail sp-metric">{{ detail }}</NText>
      </div>
    </div>
  </NCard>
</template>

<style scoped>
.stat-body {
  display: flex;
  align-items: center;
  gap: 16px;
}

.stat-ring {
  flex: none;
  width: 76px;
}

.stat-value {
  font-size: 15px;
  font-weight: 600;
  /* Không cho xuống dòng: "34.9%" dài hơn "7.3%" và sẽ vỡ làm hai dòng bên trong
     vòng tròn, đẩy con số lệch khỏi tâm. */
  white-space: nowrap;
}

.stat-meta {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 4px;
  /* min-width: 0 cho phép cột chữ co lại thay vì giữ nguyên bề rộng nội dung.
     Thiếu nó, trên màn hẹp trình duyệt ép cột xuống đúng bề rộng một ký tự và
     nhãn "Bộ nhớ" vỡ thành từng chữ cái xếp dọc. */
  min-width: 0;
}

.stat-label {
  overflow: hidden;
  font-size: 15px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.stat-detail {
  overflow: hidden;
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
