<script setup lang="ts">
/**
 * Biểu trưng SunPanel.
 *
 * Vẽ bằng SVG ngay trong mã chứ không tải tệp ảnh: nó xuất hiện ở màn hình đăng
 * nhập và trên mọi trang, nên một yêu cầu mạng nữa chỉ để lấy hình là chỗ duy
 * nhất người dùng thấy giao diện nhấp nháy lúc mới mở.
 */
withDefaults(defineProps<{ size?: number }>(), { size: 28 })
</script>

<template>
  <svg
    class="brand-logo"
    :width="size"
    :height="size"
    viewBox="0 0 64 64"
    role="img"
    aria-label="SunPanel"
  >
    <defs>
      <clipPath :id="`sp-screen-${size}`"><rect x="7" y="18" width="50" height="36" /></clipPath>
    </defs>

    <rect x="2" y="5" width="60" height="54" rx="10" fill="var(--sp-brand-frame)" />
    <rect x="7" y="18" width="50" height="36" fill="var(--sp-brand-screen)" />

    <circle cx="11.5" cy="11.5" r="2.3" fill="#42f2a3" />
    <circle cx="18.5" cy="11.5" r="2.3" fill="#00d2f6" />
    <circle cx="25.5" cy="11.5" r="2.3" fill="#33aba1" />

    <g :clip-path="`url(#sp-screen-${size})`">
      <g stroke="#f0a500" stroke-width="2.4" stroke-linecap="round">
        <path d="M32 21v5.5" />
        <path d="M21.6 25.3l3.4 4.3" />
        <path d="M42.4 25.3l-3.4 4.3" />
        <path d="M14.6 33.4l4.9 2.4" />
        <path d="M49.4 33.4l-4.9 2.4" />
      </g>
      <circle cx="32" cy="44.6" r="10.6" fill="#f0a500" />
      <!-- Đồi vẽ sau mặt trời để mặt trời mọc lên từ phía sau, đúng như biểu trưng. -->
      <path
        d="M7 54V50.6C10 46.8 15 46.8 18 50.6 22 44 30.6 44 34.6 50.6 38 46.2 43.4 46.2 46.8 50.6 49.4 48 54.4 48 57 50.4V54Z"
        fill="var(--sp-brand-frame)"
      />
    </g>
  </svg>
</template>

<style scoped>
.brand-logo {
  display: block;
  flex: none;
}
</style>
