import { i18n } from '@/locales'

const BYTE_UNITS = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']

/** Định dạng số byte thành chuỗi dễ đọc, ví dụ "1.5 GB". */
export function formatBytes(bytes: number, decimals = 1): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B'

  const exponent = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), BYTE_UNITS.length - 1)
  const value = bytes / Math.pow(1024, exponent)

  // Đơn vị byte nguyên bản không cần phần thập phân.
  return `${value.toFixed(exponent === 0 ? 0 : decimals)} ${BYTE_UNITS[exponent]}`
}

/** Định dạng tốc độ truyền, ví dụ "1.5 MB/s". */
export function formatRate(bytesPerSecond: number): string {
  return i18n.global.t('unit.bytesPerSecond', { value: formatBytes(bytesPerSecond) })
}

/** Định dạng phần trăm với một chữ số thập phân. */
export function formatPercent(value: number): string {
  return `${value.toFixed(1)}%`
}

/** Định dạng thời gian hoạt động từ số giây thành "3 ngày 4 giờ". */
export function formatUptime(seconds: number): string {
  const t = i18n.global.t
  if (!Number.isFinite(seconds) || seconds <= 0) return '—'

  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)

  const parts: string[] = []
  if (days > 0) parts.push(t('unit.days', { count: days }))
  if (hours > 0) parts.push(t('unit.hours', { count: hours }))
  // Chỉ hiện phút khi thời gian hoạt động còn ngắn, để chuỗi không dài lê thê.
  if (days === 0 && minutes > 0) parts.push(t('unit.minutes', { count: minutes }))

  return parts.join(' ') || t('unit.minutes', { count: 0 })
}

/** Định dạng mốc thời gian ISO theo ngôn ngữ đang dùng. */
export function formatDateTime(iso?: string | null): string {
  if (!iso) return i18n.global.t('common.never')

  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return i18n.global.t('common.unknown')

  return new Intl.DateTimeFormat(i18n.global.locale.value, {
    dateStyle: 'short',
    timeStyle: 'medium',
  }).format(date)
}

/** Chỉ lấy phần giờ:phút:giây, dùng cho trục thời gian của biểu đồ. */
export function formatClock(iso: string): string {
  const date = new Date(iso)
  return Number.isNaN(date.getTime())
    ? ''
    : date.toLocaleTimeString(i18n.global.locale.value, { hour12: false })
}
