<script setup lang="ts">
import { computed } from 'vue'
import { useThemeStore } from '@/stores/theme'

/**
 * Biểu trưng ứng dụng trong chợ.
 *
 * Danh mục mang theo tệp ảnh dưới dạng data URI, nhúng vào thẻ <img> chứ không
 * chèn thẳng vào trang: danh mục tự thêm của quản trị viên cũng là dữ liệu
 * ngoài, mà một tệp SVG chèn thẳng vào trang thì chạy được mã kịch bản bên
 * trong.
 */
const props = defineProps<{
  /** Data URI của biểu trưng; để trống thì dựng ô chữ cái đầu thay thế. */
  icon?: string
  /** Bản dùng khi panel ở chế độ tối, cho logo nét đen trên nền trong suốt. */
  iconDark?: string
  /** Tên ứng dụng, dùng cho ô chữ cái đầu và thuộc tính alt. */
  name: string
  size?: number
}>()

const theme = useThemeStore()

const box = computed(() => `${props.size ?? 40}px`)

/**
 * Chỉ nhận data URI ảnh. Địa chỉ web bị chính sách nội dung của panel chặn nên
 * chỉ cho ra ô vỡ, còn "javascript:" thì không được phép lọt tới thuộc tính src.
 */
function safe(value?: string): string {
  return value?.startsWith('data:image/') ? value : ''
}

const source = computed(() => {
  const dark = safe(props.iconDark)
  return theme.isDark && dark ? dark : safe(props.icon)
})

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
  /* Logo chính thức có hình khối riêng, cái vuông cái tròn cái chữ nằm ngang.
     Không bo góc và không đóng khung: mọi cái khung thêm vào đều bóp méo một
     phần trong số đó. Kích thước cố định giữ lưới thẳng hàng. */
  object-fit: contain;
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
  border-radius: 11px;
}
</style>
