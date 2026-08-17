import { writeFileSync } from 'node:fs'
import { fileURLToPath, URL } from 'node:url'
import { defineConfig, type Plugin } from 'vite'
import vue from '@vitejs/plugin-vue'

/**
 * Giữ lại web/dist/.gitkeep sau mỗi lần build.
 *
 * Backend nhúng thư mục này bằng `go:embed all:dist`, và lệnh đó thất bại nếu
 * thư mục không tồn tại. Kết quả build không được đưa vào git, nên phải có một
 * tệp giữ chỗ — mà `emptyOutDir` lại xóa sạch thư mục trước mỗi lần build.
 * Không có tệp này, người vừa clone về sẽ không biên dịch được backend.
 */
function keepOutDir(): Plugin {
  return {
    name: 'sunpanel-keep-outdir',
    writeBundle(options) {
      if (options.dir) writeFileSync(`${options.dir}/.gitkeep`, '')
    },
  }
}

// Kết quả build được ghi thẳng vào web/dist để Go nhúng vào binary.
export default defineConfig({
  plugins: [vue(), keepOutDir()],
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  // Đường dẫn tương đối để giao diện chạy được sau đường dẫn bí mật bất kỳ;
  // thẻ <base> do máy chủ chèn vào sẽ quyết định gốc thực tế.
  base: './',
  build: {
    outDir: '../web/dist',
    emptyOutDir: true,
    chunkSizeWarningLimit: 900,
    rollupOptions: {
      output: {
        // Tách các thư viện lớn ra khỏi mã ứng dụng để lần nạp sau tận dụng được
        // bộ nhớ đệm của trình duyệt.
        manualChunks: {
          vendor: ['vue', 'vue-router', 'pinia', 'vue-i18n'],
          ui: ['naive-ui'],
          charts: ['echarts'],
        },
      },
    },
  },
  server: {
    port: 5173,
    proxy: {
      // Khi phát triển, gọi API sang backend đang chạy ở cổng 9527.
      '/api': {
        target: 'http://127.0.0.1:9527',
        changeOrigin: true,
        ws: true,
      },
    },
  },
})
