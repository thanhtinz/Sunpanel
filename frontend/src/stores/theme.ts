import { defineStore } from 'pinia'
import { computed, ref, watchEffect } from 'vue'
import { darkTheme, type GlobalTheme } from 'naive-ui'

export type ThemeMode = 'auto' | 'light' | 'dark'

const STORAGE_KEY = 'sunpanel.theme'

/** Store giao diện sáng/tối, có chế độ đi theo cài đặt hệ điều hành. */
export const useThemeStore = defineStore('theme', () => {
  const mode = ref<ThemeMode>((localStorage.getItem(STORAGE_KEY) as ThemeMode) || 'auto')

  // Theo dõi thiết lập của hệ điều hành để chế độ "auto" đổi theo ngay lập tức.
  const prefersDark = ref(window.matchMedia('(prefers-color-scheme: dark)').matches)
  window
    .matchMedia('(prefers-color-scheme: dark)')
    .addEventListener('change', (e) => (prefersDark.value = e.matches))

  const isDark = computed(() => (mode.value === 'auto' ? prefersDark.value : mode.value === 'dark'))
  const naiveTheme = computed<GlobalTheme | null>(() => (isDark.value ? darkTheme : null))

  function setMode(next: ThemeMode): void {
    mode.value = next
    localStorage.setItem(STORAGE_KEY, next)
  }

  // Gắn thuộc tính lên thẻ html để CSS ngoài phạm vi Naive UI cũng đổi theo.
  watchEffect(() => {
    document.documentElement.dataset.theme = isDark.value ? 'dark' : 'light'
  })

  return { mode, isDark, naiveTheme, setMode }
})
