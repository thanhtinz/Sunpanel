/**
 * Thu nhỏ các ảnh điểm mà icons.py đã tải về.
 *
 *     npm install --no-save sharp
 *     node tools/catalog/resize.mjs
 *
 * Ô hiển thị biểu trưng chỉ rộng 44 điểm ảnh, nên một tệp 512×512 chỉ làm binary
 * phình ra. 128 điểm ảnh vẫn nét trên màn hình mật độ cao mà nhẹ hơn hàng chục
 * lần, và đó cũng là ngưỡng kiểm thử phía Go đang canh.
 */

import { readFileSync, existsSync, unlinkSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const here = dirname(fileURLToPath(import.meta.url))
const listPath = join(here, 'resize.json')

if (!existsSync(listPath)) {
  console.log('chưa có tệp resize.json — chạy icons.py trước')
  process.exit(0)
}

const jobs = JSON.parse(readFileSync(listPath, 'utf8'))
if (jobs.length === 0) {
  console.log('không có ảnh nào cần thu nhỏ')
  process.exit(0)
}

// Nạp sharp sau khi biết chắc có việc: nó là phụ thuộc chỉ dùng cho bước này,
// không nên bắt người chỉ muốn chạy icons.py phải cài.
const { default: sharp } = await import('sharp')
let total = 0

for (const [source, target] of jobs) {
  const info = await sharp(source)
    .resize(128, 128, { fit: 'contain', background: { r: 0, g: 0, b: 0, alpha: 0 } })
    .webp({ quality: 90 })
    .toFile(target)
  total += info.size
  unlinkSync(source)
}

console.log(`đã thu nhỏ ${jobs.length} ảnh, tổng ${Math.round(total / 1024)} KB`)
