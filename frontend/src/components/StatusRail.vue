<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMonitorStream } from '@/composables/useMonitorStream'

/**
 * Thanh trạng thái máy chủ ghim ở đáy menu.
 *
 * Người quản trị luôn muốn biết máy đang thế nào, bất kể đang mở trang nào —
 * phải bấm về Tổng quan mới thấy là một bước thừa lặp lại cả ngày. Thanh này
 * đọc chung luồng WebSocket với trang Tổng quan nên không tốn thêm kết nối.
 */
const props = defineProps<{
  /** Menu đang thu gọn thì chỉ còn ba vạch, không còn chữ. */
  compact?: boolean
}>()

const { t } = useI18n()
const { connected, latest } = useMonitorStream()

type Meter = { key: string; label: string; value: number }

// Chỉ CPU và ổ đĩa: ba vạch ở đáy menu là để liếc qua, còn phần bộ nhớ đã có
// thẻ riêng đầy đủ ở trang Tổng quan.
const meters = computed<Meter[]>(() => [
  { key: 'cpu', label: t('dashboard.cpu'), value: latest.value?.cpu ?? 0 },
  { key: 'disk', label: t('dashboard.disk'), value: latest.value?.disk ?? 0 },
])

/**
 * Màu đổi theo mức sử dụng để mắt bắt được tình trạng nguy hiểm ngay, không
 * phải đọc con số. Cùng ngưỡng với thẻ chỉ số ở trang Tổng quan.
 */
function toneOf(value: number): string {
  if (value >= 90) return 'var(--sp-danger)'
  if (value >= 75) return 'var(--sp-warn)'
  return 'var(--sp-action)'
}

function clamp(value: number): number {
  return Math.min(100, Math.max(0, value))
}
</script>

<template>
  <div class="rail" :class="{ 'rail-compact': props.compact }">
    <div v-if="!props.compact" class="rail-head">
      <span class="sp-eyebrow">{{ t('dashboard.thisServer') }}</span>
      <span class="rail-dot" :class="{ live: connected }" :title="
        connected ? t('dashboard.connected') : t('dashboard.disconnected')
      "></span>
    </div>

    <div
      v-for="meter in meters"
      :key="meter.key"
      class="meter"
      :title="`${meter.label} ${clamp(meter.value).toFixed(1)}%`"
    >
      <div v-if="!props.compact" class="meter-head">
        <span class="meter-label">{{ meter.label }}</span>
        <span class="meter-value sp-metric">{{ clamp(meter.value).toFixed(0) }}%</span>
      </div>

      <div class="meter-track">
        <div
          class="meter-fill"
          :style="{ width: `${clamp(meter.value)}%`, background: toneOf(meter.value) }"
        ></div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.rail {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px 14px 14px;
  border-top: 1px solid var(--sp-border);
}

.rail-compact {
  gap: 7px;
  padding: 10px 12px;
}

.rail-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

/* Chấm sống: đứng yên khi mất kết nối, để "đang cập nhật" và "số liệu đóng
   băng" không trông giống hệt nhau. */
.rail-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--sp-text-faint);
  flex: none;
}

.rail-dot.live {
  background: var(--sp-action);
  animation: rail-pulse 2.4s ease-in-out infinite;
}

@keyframes rail-pulse {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.35;
  }
}

.meter {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.meter-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 6px;
}

.meter-label {
  overflow: hidden;
  font-size: 12px;
  color: var(--sp-text-muted);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.meter-value {
  font-size: 12px;
  font-weight: 600;
  color: var(--sp-text);
  flex: none;
}

.meter-track {
  height: 3px;
  overflow: hidden;
  border-radius: 2px;
  background: var(--sp-surface-sunken);
}

.meter-fill {
  height: 100%;
  border-radius: 2px;
  transition: width 0.4s ease, background 0.4s ease;
}
</style>
