// Kiểm tra mọi tệp ngôn ngữ có cùng tập khóa.
//
// Thiếu một khóa nghĩa là người dùng ngôn ngữ đó sẽ thấy chuỗi tiếng Việt lọt
// giữa giao diện của mình. Bắt lỗi này ở CI rẻ hơn nhiều so với phát hiện khi
// đã phát hành.
import { readdirSync, readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const localesDir = join(dirname(fileURLToPath(import.meta.url)), '..', 'src', 'locales')
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

const locales = files.map((file) => ({
  name: file.replace('.json', ''),
  keys: collectKeys(JSON.parse(readFileSync(join(localesDir, file), 'utf8'))),
}))

// So mọi ngôn ngữ với hợp của tất cả các khóa, để báo được cả khóa thừa lẫn thiếu.
const allKeys = new Set(locales.flatMap((l) => [...l.keys]))
let failed = false

for (const locale of locales) {
  const missing = [...allKeys].filter((k) => !locale.keys.has(k)).sort()
  if (missing.length > 0) {
    failed = true
    console.error(`\n${locale.name}.json thiếu ${missing.length} khóa:`)
    for (const key of missing) console.error(`  - ${key}`)
  }
}

if (failed) process.exit(1)
console.log(`Đã kiểm tra ${locales.length} ngôn ngữ, ${allKeys.size} khóa — tất cả đều khớp.`)
