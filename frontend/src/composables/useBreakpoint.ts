import { onScopeDispose, readonly, ref } from 'vue'

/** mobileBreakpoint là bề rộng dưới ngưỡng này thì bố cục chuyển sang dạng di động. */
const mobileBreakpoint = 900

/**
 * Theo dõi bề rộng cửa sổ để bố cục đổi theo kích thước màn hình.
 *
 * Dùng matchMedia thay vì lắng nghe sự kiện resize: trình duyệt chỉ báo khi thực
 * sự vượt ngưỡng, thay vì gọi lại hàng trăm lần trong lúc người dùng kéo cửa sổ.
 */
export function useBreakpoint() {
  const query = window.matchMedia(`(max-width: ${mobileBreakpoint}px)`)
  const isMobile = ref(query.matches)

  const update = (event: MediaQueryListEvent) => {
    isMobile.value = event.matches
  }
  query.addEventListener('change', update)
  onScopeDispose(() => query.removeEventListener('change', update))

  return { isMobile: readonly(isMobile) }
}
