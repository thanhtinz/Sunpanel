<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import QRCode from 'qrcode'
import { useThemeStore } from '@/stores/theme'

const props = withDefaults(defineProps<{ value: string; size?: number }>(), { size: 200 })

const theme = useThemeStore()
const canvas = ref<HTMLCanvasElement | null>(null)

/**
 * Mã QR được sinh ngay trong trình duyệt.
 *
 * Đây là yêu cầu bảo mật chứ không phải lựa chọn kỹ thuật: khóa xác thực hai
 * lớp không bao giờ được gửi tới một dịch vụ sinh mã QR bên ngoài.
 */
async function render(): Promise<void> {
  if (!canvas.value || !props.value) return

  await QRCode.toCanvas(canvas.value, props.value, {
    width: props.size,
    margin: 1,
    color: {
      // Nền phải theo giao diện sáng/tối, nếu không mã QR sẽ chìm mất ở nền tối.
      dark: theme.isDark ? '#e5e7eb' : '#101014',
      light: theme.isDark ? '#18181c' : '#ffffff',
    },
  })
}

onMounted(render)
watch(() => [props.value, props.size, theme.isDark], render)
</script>

<template>
  <div class="qr-wrap">
    <canvas ref="canvas" />
  </div>
</template>

<style scoped>
.qr-wrap {
  display: flex;
  justify-content: center;
  padding: 8px;
}
</style>
