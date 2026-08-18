// Kiểm tra mọi tệp ngôn ngữ có cùng tập khóa.
//
// Thiếu một khóa nghĩa là người dùng ngôn ngữ đó sẽ thấy chuỗi tiếng Việt lọt
// giữa giao diện của mình. Bắt lỗi này ở CI rẻ hơn nhiều so với phát hiện khi
// đã phát hành.
import { readdirSync, readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const localesDir = join(dirname(fileURLToPath(import.meta.url)), '..', 'src', 'locales')

/**
 * Các ký tự mang ý nghĩa cú pháp với trình biên dịch thông điệp của vue-i18n.
 *
 * "@" mở một liên kết thông điệp và "|" tách các dạng số nhiều. Một chuỗi chứa
 * chúng ở dạng thô sẽ ném SyntaxError lúc dựng giao diện và làm trống cả vùng
 * chứa nó — lỗi chỉ lộ ra khi mở đúng màn hình đó, nên phải chặn ngay ở CI.
 * Muốn hiển thị ký tự thật thì bọc trong dấu ngoặc nhọn: {'@'}daily.
 */
const SYNTAX_CHARS = /(?<!\{')@|\|(?![^{]*\})/
const files = readdirSync(localesDir).filter((f) => f.endsWith('.json'))

function collectKeys(obj, prefix = '') {
  const keys = new Set()
  for (const [key, value] of Object.entries(obj)) {
    const path = prefix ? `${prefix}.${key}` : key
    if (value !== null && typeof value === 'object' && !Array.isArray(value)) {
      for (const nested of collectKeys(value, path)) keys.add(nested)
    } else {
      keys.add(path)
    }
  }
  return keys
}

/** collectValues trả về danh sách cặp (khóa đầy đủ, giá trị) để kiểm tra nội dung. */
function collectValues(obj, prefix = '') {
  const values = []
  for (const [key, value] of Object.entries(obj)) {
    const path = prefix ? `${prefix}.${key}` : key
    if (value !== null && typeof value === 'object' && !Array.isArray(value)) {
      values.push(...collectValues(value, path))
    } else {
      values.push([path, value])
    }
  }
  return values
}

/**
 * Tìm khóa bị khai báo hai lần trong cùng một đối tượng.
 *
 * JSON.parse im lặng giữ lần khai báo cuối, nên hai khối cùng tên "settings"
 * trông vẫn hợp lệ với mọi phép kiểm tra dựa trên đối tượng đã phân tích — còn
 * giao diện thì hiện ra khóa thô vì nửa số chuỗi đã biến mất. Chuyện này đã xảy
 * ra thật hai lần, nên phép kiểm phải làm trên văn bản gốc.
 */
function findDuplicateKeys(raw) {
  const duplicates = []
  const stack = [new Set()]
  const pattern = /"((?:[^"\\]|\\.)*)"\s*:|[{}]/g

  for (const match of raw.matchAll(pattern)) {
    const token = match[0]
    if (token === '{') {
      stack.push(new Set())
      continue
    }
    if (token === '}') {
      if (stack.length > 1) stack.pop()
      continue
    }

    const level = stack[stack.length - 1]
    const key = match[1]
    if (level.has(key)) duplicates.push(key)
    level.add(key)
  }
  return duplicates
}

const locales = files.map((file) => {
  const raw = readFileSync(join(localesDir, file), 'utf8')
  const parsed = JSON.parse(raw)
  return {
    name: file.replace('.json', ''),
    keys: collectKeys(parsed),
    values: collectValues(parsed),
    duplicates: findDuplicateKeys(raw),
  }
})

// So mọi ngôn ngữ với hợp của tất cả các khóa, để báo được cả khóa thừa lẫn thiếu.
const allKeys = new Set(locales.flatMap((l) => [...l.keys]))
let failed = false

for (const locale of locales) {
  for (const [key, value] of locale.values) {
    if (typeof value === 'string' && SYNTAX_CHARS.test(value)) {
      failed = true
      console.error(
        `\n${locale.name}.json: khóa "${key}" chứa ký tự cú pháp chưa escape của vue-i18n:`,
      )
      console.error(`  ${value}`)
      console.error(`  Hãy bọc lại, ví dụ: {'@'}daily`)
    }
  }
}

for (const locale of locales) {
  if (locale.duplicates.length > 0) {
    failed = true
    console.error(`\n${locale.name}.json có khóa khai báo hai lần:`)
    for (const key of locale.duplicates) console.error(`  - "${key}"`)
    console.error('  Gộp hai khối lại; JSON.parse chỉ giữ khối cuối và phần còn lại biến mất.')
  }
}

for (const locale of locales) {
  const missing = [...allKeys].filter((k) => !locale.keys.has(k)).sort()
  if (missing.length > 0) {
    failed = true
    console.error(`\n${locale.name}.json thiếu ${missing.length} khóa:`)
    for (const key of missing) console.error(`  - ${key}`)
  }
}

if (failed) process.exit(1)
console.log(
  `Đã kiểm tra ${locales.length} ngôn ngữ, ${allKeys.size} khóa — khớp nhau và không có ký tự cú pháp chưa escape.`,
)
