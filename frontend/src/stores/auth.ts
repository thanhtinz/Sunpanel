import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { authApi, userApi, type User } from '@/api'
import { configureClient } from '@/api/client'
import { setLocale, type Locale } from '@/locales'

const ACCESS_KEY = 'sunpanel.accessToken'
const REFRESH_KEY = 'sunpanel.refreshToken'

/**
 * Store xác thực giữ token và người dùng hiện tại.
 *
 * Token nằm trong localStorage: panel là công cụ quản trị chạy trên máy của
 * chính người dùng, và cách này giữ được phiên khi tải lại trang. Rủi ro XSS
 * được xử lý ở phía khác — Content-Security-Policy chặt và giao diện không bao
 * giờ chèn HTML thô từ dữ liệu.
 */
export const useAuthStore = defineStore('auth', () => {
  const accessToken = ref<string | null>(localStorage.getItem(ACCESS_KEY))
  const refreshToken = ref<string | null>(localStorage.getItem(REFRESH_KEY))
  const user = ref<User | null>(null)

  const isAuthenticated = computed(() => accessToken.value !== null)
  const isAdmin = computed(() => user.value?.role === 'admin')
  const canWrite = computed(
    () => user.value?.role === 'admin' || user.value?.role === 'operator',
  )

  function setTokens(access: string | null, refresh: string | null): void {
    accessToken.value = access
    refreshToken.value = refresh

    if (access) localStorage.setItem(ACCESS_KEY, access)
    else localStorage.removeItem(ACCESS_KEY)

    if (refresh) localStorage.setItem(REFRESH_KEY, refresh)
    else localStorage.removeItem(REFRESH_KEY)
  }

  function applyUser(next: User): void {
    user.value = next
    if (next.language) setLocale(next.language as Locale)
  }

  async function login(username: string, password: string, totpCode?: string): Promise<void> {
    const pair = await authApi.login(username, password, totpCode)
    setTokens(pair.accessToken, pair.refreshToken)
    applyUser(pair.user)
  }

  /** Làm mới access token. Trả về false khi phiên đã hỏng hẳn. */
  async function refresh(): Promise<boolean> {
    if (!refreshToken.value) return false

    try {
      const pair = await authApi.refresh(refreshToken.value)
      setTokens(pair.accessToken, pair.refreshToken)
      user.value = pair.user
      return true
    } catch {
      // Refresh token cũng đã hỏng: xóa sạch để người dùng đăng nhập lại.
      setTokens(null, null)
      user.value = null
      return false
    }
  }

  async function logout(): Promise<void> {
    const token = refreshToken.value
    setTokens(null, null)
    user.value = null

    if (token) {
      // Máy chủ không thu hồi được phiên cũng không ngăn được việc đăng xuất
      // phía trình duyệt — token đã bị xóa khỏi máy rồi.
      await authApi.logout(token).catch(() => undefined)
    }
  }

  /** Nạp lại thông tin người dùng; dùng khi mở lại trang với token đã lưu. */
  async function loadUser(): Promise<boolean> {
    if (!accessToken.value) return false

    try {
      applyUser(await authApi.me())
      return true
    } catch {
      setTokens(null, null)
      user.value = null
      return false
    }
  }

  async function updatePreferences(payload: { language?: string; theme?: string }): Promise<void> {
    user.value = await userApi.updatePreferences(payload)
  }

  // Nối store vào client HTTP để mọi lời gọi tự đính token và tự làm mới khi hết hạn.
  configureClient(() => accessToken.value, refresh)

  return {
    accessToken,
    refreshToken,
    user,
    isAuthenticated,
    isAdmin,
    canWrite,
    login,
    logout,
    refresh,
    loadUser,
    updatePreferences,
  }
})
