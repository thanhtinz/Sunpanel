// Service worker của SunPanel.
//
// Chỉ lo đúng hai việc: giữ vỏ ứng dụng để mở app không phải chờ mạng, và hiện
// một trang tử tế khi máy chủ không với tới được. Mọi thứ còn lại đi thẳng ra
// mạng — panel là công cụ quản trị, và một con số cũ được lấy từ bộ nhớ đệm
// nguy hiểm hơn hẳn việc phải chờ thêm một giây.

const CACHE = 'sunpanel-v1'

// Trang hiện khi mất mạng, dựng sẵn để không phụ thuộc vào tệp nào khác.
const OFFLINE_PAGE = `<!doctype html>
<html lang="vi"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>SunPanel</title>
<style>
  body { font-family: system-ui, sans-serif; background: #f4f6f3; color: #161917;
         display: grid; place-items: center; height: 100vh; margin: 0; text-align: center }
  .box { max-width: 22rem; padding: 0 1.5rem }
  h1 { font-size: 1.1rem; margin: 0 0 .5rem }
  p { color: #6a706c; font-size: .9rem; line-height: 1.6; margin: 0 0 1.25rem }
  button { font: inherit; padding: .55rem 1.1rem; border: 0; border-radius: .45rem;
           background: #15a34a; color: #fff; cursor: pointer }
  @media (prefers-color-scheme: dark) {
    body { background: #0e100f; color: #e8ebe8 } p { color: #9aa19c }
  }
</style></head>
<body><div class="box">
  <h1>Không kết nối được tới máy chủ</h1>
  <p>Máy chủ chưa trả lời. Kiểm tra mạng của thiết bị, hoặc xem panel trên máy chủ còn chạy không.</p>
  <button onclick="location.reload()">Thử lại</button>
</div></body></html>`

self.addEventListener('install', (event) => {
  // Vỏ ứng dụng được nạp lúc cài để lần mở đầu tiên sau đó không cần mạng.
  event.waitUntil(
    caches
      .open(CACHE)
      .then((cache) => cache.addAll(['./', './favicon.svg', './icon-192.png']))
      .then(() => self.skipWaiting())
      .catch(() => self.skipWaiting()),
  )
})

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) => Promise.all(keys.filter((key) => key !== CACHE).map((key) => caches.delete(key))))
      .then(() => self.clients.claim()),
  )
})

self.addEventListener('fetch', (event) => {
  const request = event.request
  if (request.method !== 'GET') return

  const url = new URL(request.url)
  if (url.origin !== self.location.origin) return

  // API và WebSocket không bao giờ được lấy từ bộ nhớ đệm: đây là số liệu sống
  // của một máy chủ, và một con số cũ làm người quản trị ra quyết định sai.
  if (url.pathname.includes('/api/')) return

  // Điều hướng: ưu tiên mạng để luôn nhận bản giao diện mới nhất, rơi về bản đã
  // lưu rồi mới tới trang báo mất mạng.
  if (request.mode === 'navigate') {
    event.respondWith(
      fetch(request)
        .then((response) => {
          const copy = response.clone()
          caches.open(CACHE).then((cache) => cache.put('./', copy))
          return response
        })
        .catch(() =>
          caches.match('./').then(
            (cached) =>
              cached ??
              new Response(OFFLINE_PAGE, { headers: { 'Content-Type': 'text/html; charset=utf-8' } }),
          ),
        ),
    )
    return
  }

  // Tài nguyên build mang mã băm trong tên nên nội dung không bao giờ đổi: lấy
  // từ bộ nhớ đệm trước, và chỉ tải về đúng một lần cho mỗi phiên bản.
  event.respondWith(
    caches.match(request).then((cached) => {
      if (cached) return cached
      return fetch(request).then((response) => {
        if (response.ok && (url.pathname.includes('/assets/') || url.pathname.endsWith('.png'))) {
          const copy = response.clone()
          caches.open(CACHE).then((cache) => cache.put(request, copy))
        }
        return response
      })
    }),
  )
})
