import { onScopeDispose, ref, shallowRef } from 'vue'
import { wsUrl, type Snapshot } from '@/api'
import { useAuthStore } from '@/stores/auth'

/** Số mẫu giữ lại trong bộ đệm vẽ biểu đồ thời gian thực. */
const BUFFER_SIZE = 120

/** Các mốc chờ trước khi kết nối lại, tăng dần để không dội bom máy chủ. */
const RETRY_DELAYS = [1000, 2000, 5000, 10000, 30000]

/**
 * Kết nối luồng giám sát thời gian thực qua WebSocket.
 *
 * Tự kết nối lại khi rớt mạng với thời gian chờ tăng dần, và tự dọn dẹp khi
 * component bị hủy.
 */
export function useMonitorStream() {
  const auth = useAuthStore()

  const connected = ref(false)
  const latest = shallowRef<Snapshot | null>(null)
  const buffer = shallowRef<Snapshot[]>([])

  let socket: WebSocket | null = null
  let retryTimer: ReturnType<typeof setTimeout> | null = null
  let attempt = 0
  let stopped = false

  function connect(): void {
    if (stopped || !auth.accessToken) return

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
    if (stopped) return

    const delay = RETRY_DELAYS[Math.min(attempt, RETRY_DELAYS.length - 1)] ?? 30000
    attempt += 1
    retryTimer = setTimeout(connect, delay)
  }

  function stop(): void {
    stopped = true
    if (retryTimer) clearTimeout(retryTimer)
    socket?.close()
    socket = null
    connected.value = false
  }

  connect()
  onScopeDispose(stop)

  return { connected, latest, buffer, stop }
}
