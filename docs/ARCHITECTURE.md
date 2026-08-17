# Kiến trúc SunPanel

Tài liệu này giải thích *vì sao* dự án được dựng theo cách hiện tại. Chi tiết từng
hàm nằm trong chính mã nguồn.

## Nguyên tắc nền tảng

**1. Một binary, không phụ thuộc runtime.**
Giao diện được build ra `web/dist` rồi nhúng vào binary bằng `go:embed`. Người dùng
tải một tệp là chạy được — không phải cài Python như aaPanel hay dựng LAMP như cPanel.
Đây cũng là lý do chọn driver SQLite thuần Go (`glebarez/sqlite`) thay vì bản dùng cgo:
giữ được `CGO_ENABLED=0`, nhờ đó biên dịch chéo cho cả 6 nền tảng từ một máy Linux.

**2. Mọi thao tác hệ điều hành đi qua một interface.**
Không package nào trong `internal/service` được phép gọi thẳng `os/exec` hay `os`.
Tất cả đi qua `pkg/host.Host`:

```go
type Host interface {
    Name() string
    Exec(ctx context.Context, cmd Command) (Result, error)
    FS() FileSystem
    Info(ctx context.Context) (SystemInfo, error)
    Close() error
}
```

Phiên bản hiện tại chỉ có `LocalHost`. Khi lên đa node, chỉ cần thêm `RemoteHost` nói
chuyện với agent qua gRPC + mTLS — **toàn bộ nghiệp vụ không phải sửa một dòng nào**.
Đây là lý do lớp trừu tượng này tồn tại ngay từ đầu dù hiện chưa dùng tới.

**3. Backend trả mã lỗi, không trả câu chữ.**
`internal/apperr` định nghĩa lỗi mang mã như `auth.invalid_credentials` kèm mã HTTP.
Giao diện tra bảng dịch. Thêm một ngôn ngữ = thêm một tệp JSON, không đụng tới Go.

## Sơ đồ luồng

```
Trình duyệt
    │
    ▼
StripEntryPath        ← gỡ tiền tố bí mật; sai đường dẫn → 404
    │                   (nằm NGOÀI engine vì Gin khớp route trước middleware)
    ▼
gin.Engine
    ├─ Recovery, RequestID, Language, Logger, SecurityHeaders
    ├─ IPAllowlist                     ← nếu có cấu hình
    │
    ├─ /api/v1/auth/*    → RateLimiter → AuthHandler
    ├─ /api/v1/*         → Auth (JWT + kiểm phiên còn sống)
    │                        └─ RequireAdmin → UserHandler, AuditHandler
    └─ mọi đường dẫn khác → giao diện đã nhúng (SPA fallback)
```

## Bố cục thư mục

| Thư mục | Vai trò |
|---|---|
| `cmd/sunpanel` | Điểm vào, phân tích lệnh con (`serve`, `reset-password`, `version`) |
| `internal/app` | Lắp ráp ứng dụng, vòng đời máy chủ, khởi tạo lần đầu |
| `internal/api/v1` | Handler HTTP — mỏng, chỉ bind và gọi service |
| `internal/service` | Toàn bộ nghiệp vụ |
| `internal/middleware` | Xác thực, phân quyền, giới hạn tần suất, đường dẫn bí mật |
| `internal/model` | Thực thể cơ sở dữ liệu |
| `internal/apperr` | Lỗi có mã dịch |
| `internal/response` | Khung phản hồi JSON chuẩn hóa |
| `pkg/host` | **Lớp trừu tượng hệ điều hành** — điểm mở rộng cho đa node |
| `pkg/monitor` | Thu thập số liệu qua gopsutil |
| `pkg/crypto` | Argon2id, AES-GCM, sinh ngẫu nhiên |
| `web` | Nhúng và phục vụ giao diện |
| `frontend` | Vue 3 + TypeScript + Naive UI |

## Các quyết định đáng chú ý

**Đường dẫn bí mật phải nằm ngoài engine.**
Gin khớp route *trước* khi middleware chạy, nên viết lại `r.URL.Path` từ trong middleware
không có tác dụng — route đã được chọn xong rồi. Vì vậy `StripEntryPath` là một
`http.Handler` bọc ngoài `gin.Engine`. Đây là lỗi đã gặp thật trong quá trình phát triển,
ghi lại để người sau không lặp lại.

**Access token gắn với phiên.**
JWT mang theo `SessionID`, và middleware kiểm tra phiên đó còn sống trong cơ sở dữ liệu ở
mỗi yêu cầu. Điều này đánh đổi một truy vấn nhỏ để lấy khả năng **thu hồi tức thì** — nếu
không, một token bị đánh cắp vẫn dùng được cho tới khi hết hạn dù người dùng đã đăng xuất.

**Refresh token xoay vòng và chỉ lưu bản băm.**
Cơ sở dữ liệu chỉ chứa SHA-256 của token. Mỗi lần làm mới, token cũ bị thu hồi ngay. Dùng
lại token cũ là dấu hiệu bị đánh cắp và sẽ thất bại.

**Kết nối SQLite giới hạn ở 1.**
SQLite chỉ cho một tiến trình ghi tại một thời điểm. Giữ pool bằng 1 (kèm chế độ WAL) tránh
được lỗi `database is locked` mà không cần logic thử lại rải rác khắp nơi.

**Phát số liệu giám sát không chặn.**
Kết nối WebSocket nào xử lý không kịp sẽ bị bỏ mẫu thay vì làm chậm bộ thu thập. Một trình
duyệt treo không được phép ảnh hưởng tới việc giám sát của cả hệ thống.

**`SafeJoin` làm sạch đường dẫn ở mức văn bản trước khi giải symlink.**
`escape/../secret.txt` (với `escape` là symlink trỏ ra ngoài) được xử lý thành
`<root>/secret.txt` chứ không phải `<ngoài>/secret.txt` như hệ điều hành sẽ làm. Cách diễn
giải này **an toàn hơn** hành vi mặc định của hệ điều hành, và có test riêng ghi lại chủ ý đó.

## Chạy test

```sh
go test ./...                   # toàn bộ
go test ./pkg/host/ -v          # riêng các test bảo mật đường dẫn
cd frontend && npm run typecheck
```

Các bài test quan trọng nhất nằm ở `pkg/host/safejoin_test.go`: chúng phủ path traversal,
symlink thoát ra ngoài, byte NUL và đường dẫn tuyệt đối. Đây là lớp phòng thủ mà một lỗi
nhỏ đồng nghĩa với việc mất toàn bộ máy chủ.
