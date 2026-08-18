<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { WebLinksAddon } from '@xterm/addon-web-links'
import '@xterm/xterm/css/xterm.css'
import { wsUrl } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { useThemeStore } from '@/stores/theme'

const props = defineProps<{ startPath?: string }>()
const emit = defineEmits<{ status: ['connecting' | 'connected' | 'disconnected'] }>()

const auth = useAuthStore()
const theme = useThemeStore()

const container = ref<HTMLElement | null>(null)
let term: Terminal | null = null
let fit: FitAddon | null = null
let socket: WebSocket | null = null
let observer: ResizeObserver | null = null
let closedByUs = false

/**
 * Bảng màu khớp với giao diện panel.
 *
 * Nền phải theo chế độ sáng/tối, nếu không cửa sổ terminal sẽ là một mảng đen
 * chọi giữa giao diện sáng.
 */
function palette() {
  return theme.isDark
    ? { background: '#101014', foreground: '#e5e7eb', cursor: '#f0a500', selectionBackground: '#33415580' }
    : { background: '#ffffff', foreground: '#1f2328', cursor: '#f0a500', selectionBackground: '#cfe3ff' }
}

function connect(): void {
  if (!term || !auth.accessToken) return

  emit('status', 'connecting')

  const params = new URLSearchParams()
  params.set('cols', String(term.cols))
  params.set('rows', String(term.rows))
  if (props.startPath) params.set('path', props.startPath)

  socket = new WebSocket(`${wsUrl('/api/v1/terminal/ws', auth.accessToken)}&${params}`)
  socket.binaryType = 'arraybuffer'

  socket.onopen = () => {
    emit('status', 'connected')
    sendResize()
  }

  socket.onmessage = (event) => {
    // Máy chủ gửi đầu ra dạng nhị phân vì shell có thể xuất ra byte không hợp lệ
    // UTF-8; xterm nhận thẳng Uint8Array và tự giải mã.
    if (event.data instanceof ArrayBuffer) {
      term?.write(new Uint8Array(event.data))
    } else {
      term?.write(String(event.data))
    }
  }

  socket.onclose = () => {
    emit('status', 'disconnected')
    if (!closedByUs) term?.writeln('\r\n\x1b[33m— phiên đã kết thúc —\x1b[0m')
  }
}

function sendResize(): void {
  if (socket?.readyState === WebSocket.OPEN && term) {
    socket.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }))
  }
}

/** Kết nối lại sau khi phiên đã đóng. */
function reconnect(): void {
  socket?.close()
  term?.clear()
  connect()
}

function clear(): void {
  term?.clear()
}

defineExpose({ reconnect, clear })

onMounted(() => {
  if (!container.value) return

  term = new Terminal({
    fontFamily: '"Cascadia Mono", Consolas, Menlo, "Liberation Mono", "Courier New", monospace',
    fontSize: 13,
    cursorBlink: true,
    // Đủ để cuộn lại xem đầu ra dài, nhưng không giữ vô hạn làm phình bộ nhớ tab.
    scrollback: 5000,
    theme: palette(),
    allowProposedApi: true,
  })

  fit = new FitAddon()
  term.loadAddon(fit)
  term.loadAddon(new WebLinksAddon())
  term.open(container.value)
  fit.fit()

  term.onData((data) => {
    if (socket?.readyState === WebSocket.OPEN) {
      socket.send(JSON.stringify({ type: 'input', data }))
    }
  })

  // Kích thước cửa sổ đổi thì phải báo cho shell, nếu không vim và htop sẽ vẽ sai.
  observer = new ResizeObserver(() => {
    fit?.fit()
    sendResize()
  })
  observer.observe(container.value)

  connect()
})

onBeforeUnmount(() => {
  closedByUs = true
  observer?.disconnect()
  socket?.close()
  term?.dispose()
  term = null
})

watch(
  () => theme.isDark,
  () => {
    if (term) term.options.theme = palette()
  },
)
</script>

<template>
  <div ref="container" class="terminal-pane" />
</template>

<style scoped>
.terminal-pane {
  width: 100%;
  height: 100%;
  padding: 8px;
  border-radius: 6px;
  background: var(--sp-bg-elevated);
}
</style>
