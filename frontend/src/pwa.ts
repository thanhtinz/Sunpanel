/**
 * Phần cài đặt ứng dụng.
 *
 * Panel chạy trong trình duyệt, nhưng người quản trị mở nó mỗi ngày và thường
 * từ điện thoại. Cài thành ứng dụng cho nó biểu tượng riêng, cửa sổ riêng và
 * mở được ngay cả khi mạng chập chờn — mà vẫn là đúng một binary trên máy chủ,
 * không có kho ứng dụng nào ở giữa.
 */
import { ref } from 'vue'

/** Sự kiện trình duyệt phát khi trang đủ điều kiện cài. */
interface InstallPromptEvent extends Event {
  prompt: () => Promise<void>
  userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>
}

/** Lời mời cài đang chờ; có giá trị nghĩa là hiện được nút "Cài ứng dụng". */
export const installPrompt = ref<InstallPromptEvent | null>(null)

/** Ứng dụng đang chạy ở dạng đã cài chứ không phải trong tab trình duyệt. */
export const standalone = ref(false)

/** registerServiceWorker bật phần chạy nền lo vỏ ứng dụng và trang mất mạng. */
export function setupPWA(): void {
  standalone.value =
    window.matchMedia('(display-mode: standalone)').matches ||
    // Safari trên iOS chưa hỗ trợ display-mode nên phải hỏi riêng.
    (window.navigator as { standalone?: boolean }).standalone === true

  window.addEventListener('beforeinstallprompt', (event) => {
    // Chặn thanh mời cài mặc định của trình duyệt để tự chọn lúc hiện: thanh đó
    // nhảy ra ngay giữa màn hình đăng nhập, đúng lúc người dùng đang gõ mật khẩu.
    event.preventDefault()
    installPrompt.value = event as InstallPromptEvent
  })

  window.addEventListener('appinstalled', () => {
    installPrompt.value = null
  })

  if (!('serviceWorker' in navigator)) return

  // Đăng ký sau khi trang tải xong: service worker không phải thứ người dùng
  // đang chờ, và giành băng thông với chính giao diện là tự làm mình chậm đi.
  window.addEventListener('load', () => {
    // Phạm vi lấy theo thẻ <base> do máy chủ chèn, nên panel nằm sau đường dẫn
    // bí mật nào cũng đăng ký đúng chỗ.
    const base = document.querySelector('base')?.getAttribute('href') ?? '/'
    void navigator.serviceWorker.register(`${base}sw.js`, { scope: base }).catch(() => {
      // Trình duyệt chặn service worker trên HTTP không phải localhost. Panel
      // vẫn chạy bình thường, chỉ là không cài được thành ứng dụng.
    })
  })
}

/**
 * Nền tảng đang dùng, để chỉ đúng cách cài.
 *
 * Chỉ Chrome và các trình duyệt cùng nhân mới phát sự kiện mời cài; Safari trên
 * iPhone thì không bao giờ. Nếu chỉ hiện nút khi có sự kiện đó thì phần lớn
 * người dùng điện thoại không bao giờ biết panel cài được thành ứng dụng.
 */
export function platform(): 'ios' | 'android' | 'desktop' {
  const agent = navigator.userAgent
  if (/iPhone|iPad|iPod/.test(agent)) return 'ios'
  if (/Android/.test(agent)) return 'android'
  return 'desktop'
}

/** install mở hộp thoại cài ứng dụng của trình duyệt. */
export async function install(): Promise<boolean> {
  const event = installPrompt.value
  if (!event) return false

  await event.prompt()
  const { outcome } = await event.userChoice
  installPrompt.value = null
  return outcome === 'accepted'
}
