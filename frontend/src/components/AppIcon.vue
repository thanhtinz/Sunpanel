<script setup lang="ts">
import { computed } from 'vue'

/**
 * Biểu trưng ứng dụng trong chợ.
 *
 * Danh mục mang theo tệp SVG, nhúng vào thẻ <img> qua data URI chứ không chèn
 * thẳng vào trang: danh mục tự thêm của quản trị viên cũng là dữ liệu ngoài, mà
 * SVG chèn thẳng vào trang thì chạy được mã kịch bản bên trong.
 */
const props = defineProps<{
  /** Nội dung tệp SVG; để trống thì dựng ô chữ cái đầu thay thế. */
  icon?: string
  /** Tên ứng dụng, dùng cho ô chữ cái đầu và thuộc tính alt. */
  name: string
  size?: number
}>()

const box = computed(() => `${props.size ?? 40}px`)

const source = computed(() =>
  props.icon ? `data:image/svg+xml;charset=utf-8,${encodeURIComponent(props.icon)}` : '',
)

/** Chữ cái đầu của tên, viết hoa — đủ để phân biệt các ô trong một lưới. */
const initial = computed(() => (props.name.trim()[0] ?? '?').toUpperCase())
</script>

<template>
  <img
    v-if="source"
    class="app-icon"
    :src="source"
    :alt="name"
    :style="{ width: box, height: box }"
  />
  <span v-else class="app-icon app-icon-fallback" :style="{ width: box, height: box }">
    {{ initial }}
  </span>
</template>

<style scoped>
.app-icon {
  display: block;
  flex: none;
  border-radius: 11px;
  /* Viền mảnh để biểu trưng nền tối (Ghost, Umami) không lẫn vào mặt thẻ khi
     panel đang ở chế độ tối. */
  box-shadow: 0 0 0 1px var(--sp-border);
}

.app-icon-fallback {
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 17px;
  font-weight: 700;
  color: var(--sp-text-muted);
  background: var(--sp-surface-sunken);
  border: 1px solid var(--sp-border);
}
</style>
