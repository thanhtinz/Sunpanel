import { createI18n } from 'vue-i18n'
import en from './en.json'
import vi from './vi.json'

/** Các ngôn ngữ giao diện hỗ trợ. Thêm ngôn ngữ mới = thêm một tệp JSON ở đây. */
export const SUPPORTED_LOCALES = [
  { value: 'vi', label: 'Tiếng Việt' },
  { value: 'en', label: 'English' },
] as const

export type Locale = (typeof SUPPORTED_LOCALES)[number]['value']

const STORAGE_KEY = 'sunpanel.locale'
const DEFAULT_LOCALE: Locale = 'vi'

function isSupported(value: string): value is Locale {
  return SUPPORTED_LOCALES.some((l) => l.value === value)
}

/**
 * Ngôn ngữ khởi tạo lấy theo thứ tự: lựa chọn đã lưu, ngôn ngữ trình duyệt,
 * rồi mặc định tiếng Việt.
 */
function initialLocale(): Locale {
  const saved = localStorage.getItem(STORAGE_KEY)
  if (saved && isSupported(saved)) return saved

  for (const tag of navigator.languages ?? [navigator.language]) {
    const base = tag.split('-')[0]?.toLowerCase() ?? ''
    if (isSupported(base)) return base
  }
  return DEFAULT_LOCALE
}

export const i18n = createI18n({
  legacy: false,
  locale: initialLocale(),
  fallbackLocale: DEFAULT_LOCALE,
  messages: { vi, en },
  // Chuỗi thiếu ở ngôn ngữ này sẽ lặng lẽ lùi về ngôn ngữ mặc định thay vì
  // đổ cảnh báo ra console của người dùng cuối.
  missingWarn: import.meta.env.DEV,
  fallbackWarn: import.meta.env.DEV,
})

/** Đổi ngôn ngữ hiện tại và ghi nhớ lựa chọn. */
export function setLocale(locale: Locale): void {
  i18n.global.locale.value = locale
  localStorage.setItem(STORAGE_KEY, locale)
  document.documentElement.lang = locale
}

/** Ngôn ngữ đang dùng. */
export function currentLocale(): Locale {
  return i18n.global.locale.value as Locale
}

/**
 * Dịch một mã lỗi từ API. Mã lạ (do backend mới hơn giao diện) sẽ hiện thành
 * lỗi chung thay vì để lộ mã kỹ thuật cho người dùng.
 */
export function translateError(code: string, params: Record<string, unknown> = {}): string {
  const t = i18n.global.t
  return t(code) === code ? t('error.internal') : t(code, params)
}
