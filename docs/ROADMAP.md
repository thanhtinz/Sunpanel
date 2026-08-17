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

## Giai đoạn 2: Vận hành hệ thống

- **Trình quản lý tệp**: duyệt, tải lên theo khối, tải xuống, nén/giải nén, phân quyền,
  soạn thảo bằng Monaco
- **Terminal web**: xterm.js qua WebSocket, đa tab, ghi log phiên
- **Quản lý dịch vụ**: liệt kê, khởi động, dừng, bật khi khởi động máy
- **Cron**: quản lý tác vụ định kỳ, xem log từng lần chạy
- **Tường lửa**: hỗ trợ ufw, firewalld, nftables

## Giai đoạn 3: Docker và chợ ứng dụng

- Quản lý container, image, volume, network, compose
- Chợ ứng dụng cài một chạm: WordPress, n8n, Gitea, Uptime Kuma…
- Định nghĩa ứng dụng bằng YAML, hỗ trợ nguồn ứng dụng tự thêm

## Giai đoạn 4: Website và SSL

- Quản lý website, sinh vhost Nginx, reverse proxy
- SSL Let's Encrypt tự động (HTTP-01 và DNS-01), tự gia hạn
- Quản lý domain và bản ghi DNS

## Giai đoạn 5: Cơ sở dữ liệu và sao lưu

- MySQL / MariaDB / PostgreSQL / Redis dạng container
- Giao diện quản trị cơ sở dữ liệu kiểu phpMyAdmin
- Sao lưu theo lịch lên local / S3 / Google Drive / WebDAV, khôi phục một chạm

## Giai đoạn 6: Mở rộng và đa node

- Hệ thống plugin kèm SDK và tài liệu
- Agent đa node qua gRPC + mTLS (`RemoteHost` — lớp trừu tượng đã sẵn sàng từ giai đoạn 1)
- Cảnh báo qua email, Telegram, webhook
- API công khai kèm API key
