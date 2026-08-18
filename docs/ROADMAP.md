# Lộ trình phát triển

## Đã hoàn thành — Giai đoạn 1: Nền móng

- Máy chủ HTTP với tắt máy êm, cấu hình qua tệp và biến môi trường
- SQLite + GORM, migration tự động, khởi tạo tài khoản quản trị lần đầu
- Xác thực đầy đủ: JWT, refresh token xoay vòng, 2FA TOTP, RBAC ba vai trò,
  khóa tài khoản, đường dẫn bí mật, giới hạn tần suất, danh sách IP
- Giám sát tài nguyên thời gian thực qua WebSocket + biểu đồ lịch sử
- Quản lý người dùng và nhật ký kiểm toán
- Giao diện Vue 3 song ngữ Việt/Anh, sáng/tối
- Biên dịch chéo 6 nền tảng, script cài một dòng, CI đầy đủ

## Đã hoàn thành — Giai đoạn 2: Vận hành hệ thống

- **Trình quản lý tệp**: duyệt, tải lên, tải xuống bằng vé ngắn hạn, nén/giải nén
  zip và tar.gz, đổi tên, phân quyền, soạn thảo bằng Monaco
- **Terminal web**: PTY thật qua WebSocket với xterm.js, đa tab, đổi kích thước,
  ghi mọi lệnh vào nhật ký kiểm toán
- **Quản lý dịch vụ**: systemd — liệt kê, khởi động, dừng, khởi động lại, bật/tắt
  tự khởi động, xem nhật ký; từ chối dừng sshd và chính panel
- **Tác vụ định kỳ**: bộ lập lịch nội bộ kèm lịch sử từng lần chạy và đầu ra
- **Tường lửa**: ufw và firewalld, quản lý cổng và quy tắc theo nguồn; từ chối
  các quy tắc khiến quản trị viên mất đường vào máy chủ

Còn thiếu ở giai đoạn này: driver nftables thuần cho các bản Linux không cài ufw
lẫn firewalld, và tìm kiếm tệp theo nội dung.

## Đã hoàn thành — Giai đoạn 3: Docker và chợ ứng dụng

- **Quản lý Docker**: container (khởi động, dừng, tạm dừng, khởi động lại, xóa,
  nhật ký, tài nguyên), image (tải, xóa), volume, mạng, dọn rác
- **Chợ ứng dụng**: cài một chạm WordPress, Gitea, n8n, Uptime Kuma qua Docker
  Compose; định nghĩa ứng dụng bằng YAML, thêm ứng dụng riêng bằng cách bỏ tệp
  vào thư mục danh mục
- Mật khẩu ứng dụng tự sinh, lưu mã hóa, xem lại được qua endpoint có kiểm toán

## Đã hoàn thành — Giai đoạn 4: Website và SSL

- **Website**: sinh vhost nginx cho website tĩnh, PHP và reverse proxy; kiểm tra
  cấu hình trước khi nạp và tự khôi phục bản cũ nếu sai
- **SSL**: chứng chỉ tự ký, tải lên, hoặc Let's Encrypt qua thử thách HTTP-01;
  tự gia hạn khi còn dưới 30 ngày

Còn thiếu ở giai đoạn này: thử thách DNS-01 cho chứng chỉ ký tự đại diện, và
quản lý bản ghi DNS.

## Đã hoàn thành — Giai đoạn 5: Cơ sở dữ liệu và sao lưu

- **Cơ sở dữ liệu**: MySQL/MariaDB và PostgreSQL — tạo và xóa cơ sở dữ liệu, tài
  khoản và phân quyền, đổi mật khẩu, cửa sổ chạy SQL có bảng kết quả
- **Sao lưu**: cơ sở dữ liệu và thư mục, theo lịch, đẩy lên máy chủ, S3 hoặc
  WebDAV, có chính sách giữ bản sao và khôi phục một chạm

Còn thiếu ở giai đoạn này: Redis, và nơi lưu trữ Google Drive (cần luồng OAuth
riêng nên chưa làm).

## Đã hoàn thành — Giai đoạn 6: Mở rộng và đa node

- **Cảnh báo**: quy tắc ngưỡng CPU/RAM/đĩa/tải gửi qua Telegram, email hoặc
  webhook; tự báo khi sao lưu thất bại hoặc gia hạn chứng chỉ hỏng
- **Khóa API**: gọi API từ script, giới hạn quyền, thu hồi riêng lẻ
- **Đa node**: chạy `sunpanel agent` trên máy khác, panel điều khiển qua HTTPS
  kèm token — `RemoteHost` thực hiện đúng interface `host.Host` đã tách từ giai
  đoạn 1, nên business logic không phải sửa gì
- **Plugin**: dịch vụ chạy riêng khai báo bằng YAML, panel chuyển tiếp yêu cầu
  kèm danh tính người gọi ([hướng dẫn](PLUGINS.md))

Kênh agent dùng token trên TLS thay vì gRPC + mTLS như dự tính ban đầu: panel đã
có sẵn máy chủ HTTP và bộ mã hóa JSON, nên gRPC chỉ kéo thêm hàng chục gói phụ
thuộc mà không thêm khả năng nào.

## Hướng đi tiếp

Toàn bộ lộ trình ban đầu đã hoàn thành. Các mục còn để ngỏ, xếp theo mức hữu ích:

- Thử thách DNS-01 cho chứng chỉ ký tự đại diện (Cloudflare, DNSPod)
- Driver nftables thuần cho bản Linux không có ufw lẫn firewalld
- Điều khiển đầy đủ tính năng trên node ở xa, không chỉ xem thông tin hệ thống
- Redis và Google Drive
- Tìm kiếm tệp theo nội dung
