import { onScopeDispose, ref, shallowRef } from 'vue'
import { wsUrl, type Snapshot } from '@/api'
import { useAuthStore } from '@/stores/auth'

/** Số mẫu giữ lại trong bộ đệm vẽ biểu đồ thời gian thực. */
const BUFFER_SIZE = 120

/** Các mốc chờ trước khi kết nối lại, tăng dần để không dội bom máy chủ. */
const RETRY_DELAYS = [1000, 2000, 5000, 10000, 30000]

const connected = ref(false)
const latest = shallowRef<Snapshot | null>(null)
const buffer = shallowRef<Snapshot[]>([])

let socket: WebSocket | null = null
let retryTimer: ReturnType<typeof setTimeout> | null = null
let attempt = 0
/** Số nơi đang dùng luồng; luồng chỉ đóng khi nơi cuối cùng rời đi. */
let subscribers = 0

function connect(): void {
  const auth = useAuthStore()
  if (subscribers === 0 || !auth.accessToken || socket) return

  socket = new WebSocket(wsUrl('/api/v1/monitor/stream', auth.accessToken))

  socket.onopen = () => {
    connected.value = true
    attempt = 0
  }

  socket.onmessage = (event) => {
    try {
      const snapshot = JSON.parse(event.data as string) as Snapshot
      latest.value = snapshot
      // Tạo mảng mới thay vì đẩy vào mảng cũ: shallowRef chỉ phát hiện thay đổi
      // khi chính tham chiếu đổi.
      buffer.value = [...buffer.value, snapshot].slice(-BUFFER_SIZE)
    } catch {
      // Khung tin hỏng thì bỏ qua, mẫu kế tiếp sẽ tới sau vài giây.
    }
  }

  socket.onclose = () => {
    connected.value = false
    socket = null
    scheduleReconnect()
  }

  socket.onerror = () => {
    // onclose luôn được gọi ngay sau onerror, nên việc kết nối lại xử lý ở đó.
    socket?.close()
  }
}

function scheduleReconnect(): void {
  if (subscribers === 0) return

  const delay = RETRY_DELAYS[Math.min(attempt, RETRY_DELAYS.length - 1)] ?? 30000
  attempt += 1
  retryTimer = setTimeout(connect, delay)
}

function teardown(): void {
  if (retryTimer) clearTimeout(retryTimer)
  retryTimer = null
  socket?.close()
  socket = null
  connected.value = false
}

/**
 * Kết nối luồng giám sát thời gian thực qua WebSocket.
 *
 * Luồng dùng CHUNG cho toàn ứng dụng: thanh trạng thái ở khung ngoài và trang
 * Tổng quan cùng đọc một kết nối. Mỗi nơi mở một socket riêng sẽ nhân đôi lưu
 * lượng và khiến hai chỗ hiển thị hai con số lệch nhau vài giây.
 *
 * Tự kết nối lại khi rớt mạng với thời gian chờ tăng dần, và chỉ đóng khi nơi
 * dùng cuối cùng bị hủy.
 */
export function useMonitorStream() {
  subscribers += 1
  connect()

  onScopeDispose(() => {
    subscribers -= 1
    if (subscribers === 0) teardown()
  })

  return { connected, latest, buffer }
}
