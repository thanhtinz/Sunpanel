# Viết plugin cho SunPanel

Plugin là một **dịch vụ HTTP chạy riêng**, không phải mã nạp vào panel. Panel chạy
quyền root, nên một plugin lỗi hoặc độc hại nạp thẳng vào tiến trình panel sẽ kéo
theo toàn bộ máy chủ. Chạy tiến trình riêng thì plugin hỏng chỉ làm hỏng chính nó.

## Khai báo

Đặt một tệp YAML vào thư mục plugin (mặc định `/opt/sunpanel/plugins`):

```yaml
key: giam-sat-mo-rong          # định danh, xuất hiện trong URL
name:
  vi: Giám sát mở rộng
  en: Extended monitoring
description:
  vi: Thêm biểu đồ chi tiết cho từng tiến trình.
  en: Adds per-process charts.
version: "1.0"
author: Tên bạn
website: https://vi-du.vn
baseUrl: http://127.0.0.1:9600  # địa chỉ dịch vụ plugin
uiPath: /ui                     # trang giao diện; bỏ trống nếu plugin chỉ có API
requireRole: operator           # admin | operator | readonly
enabled: true
```

Sau khi thêm hoặc sửa tệp, bấm **Nạp lại** ở trang Plugin (hoặc gọi
`POST /api/v1/plugins/reload`). Tệp khai báo hỏng sẽ báo lỗi ngay kèm số dòng.

## Panel gọi plugin như thế nào

Mọi yêu cầu tới `/api/v1/plugins/<key>/proxy/<đường-dẫn>` được chuyển tiếp tới
`baseUrl/<đường-dẫn>`. Panel **xóa** mọi thông tin xác thực của mình
(`Authorization`, `X-API-Key`, `Cookie`) rồi thêm danh tính người gọi:

| Tiêu đề | Nội dung |
|---|---|
| `X-SunPanel-User` | tên đăng nhập |
| `X-SunPanel-Role` | `admin`, `operator` hoặc `readonly` |
| `X-SunPanel-User-ID` | mã người dùng |

Plugin **không cần và không nên** tự xác thực người dùng: panel đã làm việc đó.
Nhưng plugin phải tự **phân quyền** theo vai trò nhận được, vì `requireRole` chỉ
là mức tối thiểu để vào plugin.

Vì plugin chỉ nhận yêu cầu đã qua panel, hãy cho nó lắng nghe trên `127.0.0.1`
để không ai gọi thẳng, bỏ qua lớp kiểm quyền của panel.

## Giao diện

Nếu khai báo `uiPath`, panel hiển thị plugin trong một khung nhúng. Khung nhúng
không đặt được header `Authorization`, nên panel cấp một **vé ngắn hạn** (10 phút)
đi kèm trong query `?ticket=`. Vé chỉ mở được đúng plugin đó và không dùng được
cho bất kỳ API nào khác.

Giao diện plugin nên dùng đường dẫn tương đối để mọi tài nguyên của nó cũng đi
qua đường proxy.

## Ví dụ tối giản

```python
import http.server, json

class Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        user = self.headers.get('X-SunPanel-User')
        role = self.headers.get('X-SunPanel-Role')
        if role == 'readonly':
            self.send_error(403, 'chi doc')
            return
        body = json.dumps({'xin_chao': user}, ensure_ascii=False).encode()
        self.send_response(200)
        self.send_header('Content-Type', 'application/json; charset=utf-8')
        self.send_header('Content-Length', str(len(body)))
        self.end_headers()
        self.wfile.write(body)

http.server.HTTPServer(('127.0.0.1', 9600), Handler).serve_forever()
```
