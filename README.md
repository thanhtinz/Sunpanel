<div align="center">

# ☀ SunPanel

**Bảng điều khiển quản trị máy chủ — một binary duy nhất, đa nền tảng, đa ngôn ngữ**

[English](#english) · [Tiếng Việt](#tiếng-việt)

</div>

---

## Tiếng Việt

SunPanel là bảng điều khiển quản trị máy chủ mã nguồn mở, thay thế cho aaPanel / cPanel.
Toàn bộ panel — API, giao diện web và bộ giám sát — nằm gọn trong **một tệp thực thi duy nhất**:
không cần cài Python, Node hay bất kỳ runtime nào lên máy chủ.

### Đặc điểm

| | |
|---|---|
| **Một binary** | Tải một tệp là chạy được. Giao diện được nhúng sẵn bên trong. |
| **Đa nền tảng** | Linux (amd64/arm64/arm), macOS, Windows — biên dịch chéo từ một máy. |
| **Đa ngôn ngữ** | Tiếng Việt và English đầy đủ, kể cả thông báo lỗi từ máy chủ. |
| **Nhẹ** | Dưới 60 MB RAM khi chạy. Chạy tốt trên VPS 512 MB. |
| **Bảo mật từ gốc** | Argon2id, 2FA TOTP, RBAC, đường dẫn bí mật, nhật ký kiểm toán. |

### Cài đặt

```sh
curl -fsSL https://raw.githubusercontent.com/thanhtinz/Sunpanel/main/deploy/install.sh | sudo sh
```

Trình cài đặt tự nhận kiến trúc máy, tạo dịch vụ systemd, mở cổng tường lửa và in ra
thông tin đăng nhập. **Mật khẩu chỉ hiển thị một lần** — hãy lưu lại ngay.

Quên mật khẩu:

```sh
sunpanel reset-password -user admin
```

### Tính năng hiện có

- **Xác thực**: đăng nhập, xác thực hai lớp TOTP, quản lý phiên (xem và thu hồi từng phiên),
  tự động khóa tài khoản sau nhiều lần sai mật khẩu, xoay vòng refresh token.
- **Giám sát**: CPU (tổng và từng nhân), RAM, swap, ổ đĩa, tải trung bình, tốc độ mạng
  và đĩa — cập nhật trực tiếp qua WebSocket, kèm biểu đồ lịch sử tới 7 ngày.
- **Tài khoản máy chủ**: tạo, khóa, cấp sudo và xóa tài khoản đăng nhập của hệ điều
  hành; gắn khóa SSH công khai với đúng quyền tệp mà sshd đòi hỏi.
- **Cài đặt panel**: đổi cổng, đường dẫn bí mật, chứng chỉ HTTPS, danh sách IP được
  phép và thời hạn phiên ngay trên giao diện; panel tự khởi động lại rồi đưa trình
  duyệt sang địa chỉ mới.
- **Người dùng**: ba vai trò (quản trị viên / vận hành / chỉ xem), tạo, sửa, vô hiệu hóa,
  đặt lại mật khẩu.
- **Nhật ký**: nhật ký thao tác và nhật ký đăng nhập, có phân trang.
- **Trình quản lý tệp**: duyệt, sửa tệp bằng Monaco, tải lên/xuống, nén và giải nén.
  Mở được zip, rar, 7z, tar và tar nén gzip/bzip2/xz/zstd; định dạng được nhận từ
  chính dữ liệu nên tệp đặt sai đuôi tên vẫn mở ra được.
- **Terminal web**: PTY thật với xterm.js, đa tab; mọi lệnh được ghi vào nhật ký.
- **Dịch vụ hệ thống**: điều khiển systemd, xem nhật ký, có lớp chặn không cho tự
  khóa mình khỏi máy chủ.
- **Dung lượng**: bảng phân vùng, và trình phân tích kiểu `du` bấm sâu được xuống
  từng thư mục để tìm chỗ đang chiếm đĩa.
- **Nhật ký hệ thống**: đọc tệp nhật ký trong /var/log ngay trên giao diện, theo dõi
  trực tiếp phần mới ghi vào, lọc theo từ khóa; chỉ đọc và không ra khỏi thư mục đó.
- **Tiến trình**: bảng tiến trình sắp theo mức dùng CPU thật, tìm theo tên hoặc
  PID, kết thúc tiến trình treo, và danh sách cổng đang mở kèm tiến trình sở hữu.
- **Tác vụ định kỳ**: bộ lập lịch nội bộ, xem lịch sử và đầu ra từng lần chạy.
- **Tường lửa**: ufw và firewalld, quản lý cổng và quy tắc theo nguồn.
- **Docker**: quản lý container, image, volume và mạng; xem nhật ký và tài nguyên
  của từng container, dọn rác giải phóng dung lượng.
- **Chợ ứng dụng**: cài WordPress, Gitea, n8n, Uptime Kuma bằng một chạm qua
  Docker Compose; mật khẩu tự sinh và lưu mã hóa, thêm ứng dụng riêng bằng YAML.
- **Website**: sinh vhost nginx cho website tĩnh, PHP và reverse proxy; cấu hình
  được nginx kiểm tra trước khi nạp và tự khôi phục bản cũ nếu sai. Triển khai mã
  nguồn bằng cách tải thẳng tệp nén lên — panel tự bóc lớp thư mục bọc mà bản tải
  từ GitHub nào cũng có. Bảo vệ trang bằng mật khẩu, chặn theo địa chỉ IP và đặt
  quy tắc chuyển hướng ngay trong biểu mẫu website.
- **Cơ sở dữ liệu**: quản lý MySQL/MariaDB và PostgreSQL — tạo cơ sở dữ liệu,
  tài khoản và phân quyền, kèm cửa sổ chạy SQL có bảng kết quả.
- **Theo dõi uptime**: kiểm tra website và dịch vụ theo chu kỳ, giữ lịch sử, tính tỉ
  lệ sống 24 giờ, và báo qua kênh cảnh báo khi đổi trạng thái.
- **Sao lưu**: sao lưu cơ sở dữ liệu và thư mục theo lịch, đẩy lên máy chủ, S3
  hoặc WebDAV, có chính sách giữ bản sao và khôi phục một chạm.
- **SSL**: chứng chỉ tự ký, tải lên, hoặc xin từ Let's Encrypt qua HTTP-01 và tự
  gia hạn khi còn dưới 30 ngày.
- **Cảnh báo**: quy tắc theo ngưỡng CPU/RAM/đĩa/tải, gửi qua Telegram, email hoặc
  webhook; tự báo khi sao lưu thất bại hoặc gia hạn chứng chỉ hỏng.
- **Khóa API**: gọi API từ script mà không nhúng mật khẩu tài khoản, giới hạn
  quyền và thu hồi được riêng lẻ.
- **Nhiều máy chủ**: chép chính binary này sang máy khác và chạy `sunpanel agent`
  để panel điều khiển máy đó từ xa qua kênh TLS có token.
- **Plugin**: mở rộng panel bằng dịch vụ chạy riêng khai báo qua YAML; panel
  chuyển tiếp yêu cầu kèm danh tính người gọi — xem [hướng dẫn](docs/PLUGINS.md).
- **Giao diện**: chế độ sáng/tối, chuyển ngôn ngữ tức thời, bảng lệnh nhanh Ctrl+K
  (tìm được cả khi gõ không dấu), bố cục riêng cho điện thoại.

Toàn bộ lộ trình ban đầu đã hoàn thành; xem [lộ trình](docs/ROADMAP.md) để biết
thứ tự các mốc đã đi qua.

### Bảo mật

Panel chạy với quyền root, nên bảo mật được đặt lên hàng đầu ngay từ thiết kế:

- Mật khẩu băm bằng **Argon2id**; bí mật trong cơ sở dữ liệu (khóa TOTP) mã hóa **AES-256-GCM**
  bằng khóa chủ lưu ở tệp riêng quyền `0600`.
- **Đường dẫn bí mật**: panel chỉ trả lời trên một đường dẫn ngẫu nhiên, mọi URL khác trả về 404 —
  công cụ quét tự động không tìm thấy gì.
- **Chống path traversal**: mọi thao tác tệp đi qua `host.SafeJoin`, chặn cả `../`, đường dẫn
  tuyệt đối lẫn symlink trỏ ra ngoài. Có bộ test riêng cho từng dạng tấn công.
- **Không có shell injection**: lệnh hệ thống chỉ nhận `(chương trình, []tham số)`, không bao
  giờ nhận chuỗi shell.
- **Thu hồi tức thì**: thu hồi một phiên vô hiệu hóa luôn access token của phiên đó, không
  phải chờ hết hạn.
- Giới hạn tần suất đăng nhập, danh sách IP cho phép, header bảo mật và CSP chặt.

Phát hiện lỗ hổng? Xin đừng mở issue công khai — gửi email tới người bảo trì.

### Phát triển

```sh
git clone https://github.com/thanhtinz/Sunpanel.git && cd Sunpanel

make frontend        # build giao diện vào web/dist
make build           # build binary vào bin/sunpanel
make run             # build rồi chạy

make check           # lint + test
make release         # build cho cả 6 nền tảng vào dist/
```

Khi phát triển giao diện, chạy song song hai tiến trình — Vite sẽ chuyển tiếp `/api` sang backend:

```sh
make run             # cửa sổ 1: backend ở cổng 9527
make frontend-dev    # cửa sổ 2: giao diện ở cổng 5173
```

Chi tiết kiến trúc: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

### Cấu hình

Cấu hình nằm ở `/opt/sunpanel/config.yaml`, mọi giá trị đều có thể ghi đè bằng biến môi
trường với tiền tố `SUNPANEL_`:

```sh
SUNPANEL_PORT=8080 SUNPANEL_DATA_DIR=/srv/sunpanel sunpanel serve
```

| Biến | Mặc định | Ý nghĩa |
|---|---|---|
| `SUNPANEL_HOST` | `0.0.0.0` | Địa chỉ lắng nghe |
| `SUNPANEL_PORT` | `9527` | Cổng lắng nghe |
| `SUNPANEL_DATA_DIR` | `/opt/sunpanel` | Thư mục dữ liệu |
| `SUNPANEL_ENTRY_PATH` | *(sinh ngẫu nhiên)* | Đường dẫn bí mật |
| `SUNPANEL_FILE_ROOT` | `/` | Phạm vi trình quản lý tệp |
| `SUNPANEL_LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `SUNPANEL_TLS_ENABLED` | `false` | Bật HTTPS |

### Giấy phép

[AGPL-3.0](LICENSE) — bạn được tự do dùng, sửa và phân phối, với điều kiện mọi bản sửa đổi
chạy dưới dạng dịch vụ mạng cũng phải công khai mã nguồn.

---

## English

SunPanel is an open-source server management panel — an alternative to aaPanel / cPanel.
The entire panel — API, web UI, and monitoring — ships as **one single binary**: no Python,
no Node, no runtime to install on your server.

### Highlights

| | |
|---|---|
| **One binary** | Download one file and run it. The UI is embedded inside. |
| **Cross-platform** | Linux (amd64/arm64/arm), macOS, Windows — cross-compiled from one machine. |
| **Multilingual** | Full Vietnamese and English, including server error messages. |
| **Light** | Under 60 MB RAM at runtime. Comfortable on a 512 MB VPS. |
| **Secure by design** | Argon2id, TOTP 2FA, RBAC, secret entry path, audit logging. |

### Install

```sh
curl -fsSL https://raw.githubusercontent.com/thanhtinz/Sunpanel/main/deploy/install.sh | sudo sh
```

The installer detects your architecture, sets up a systemd service, opens the firewall port,
and prints your credentials. **The password is shown only once** — save it immediately.

Forgot the password:

```sh
sunpanel reset-password -user admin
```

### What works today

- **Authentication**: sign-in, TOTP two-factor, session management (list and revoke individual
  sessions), account lockout after repeated failures, refresh-token rotation.
- **Monitoring**: CPU (total and per core), memory, swap, disks, load average, network and disk
  throughput — streamed live over WebSocket, with history charts up to 7 days.
- **Server accounts**: create, lock, grant sudo and delete operating system logins;
  attach SSH public keys with exactly the file permissions sshd insists on.
- **Panel settings**: change the port, secret path, HTTPS certificate, allowed IPs and
  session lifetimes from the UI; the panel restarts itself and sends the browser to the
  new address.
- **Users**: three roles (admin / operator / read-only), create, edit, disable, reset password.
- **Logs**: activity log and sign-in log, paginated.
- **File manager**: browse, edit with Monaco, upload/download, compress and extract.
  Reads zip, rar, 7z, tar and tar compressed with gzip/bzip2/xz/zstd; the format is
  detected from the data, so a file with the wrong extension still opens.
- **Web terminal**: real PTY with xterm.js, multiple tabs; every command is audited.
- **System services**: systemd control and logs, with a guard against locking yourself out.
- **Disk usage**: a partition table plus a `du`-style analyzer you can drill into,
  folder by folder, to find what is filling the disk.
- **System logs**: read log files under /var/log in the browser, follow new lines live
  and filter by keyword; read-only and confined to that directory.
- **Processes**: a process table sorted by real CPU usage, searchable by name or
  PID, with a way to end stuck processes, plus the list of open ports and who owns them.
- **Scheduled tasks**: built-in scheduler with per-run history and captured output.
- **Firewall**: ufw and firewalld, port and source-scoped rules.
- **Docker**: manage containers, images, volumes and networks; per-container logs
  and resource usage, plus cleanup to reclaim disk.
- **App store**: one-click WordPress, Gitea, n8n and Uptime Kuma over Docker
  Compose; passwords are generated and stored encrypted, and you can add your own
  applications with a YAML file.
- **Websites**: generate nginx vhosts for static, PHP and reverse-proxy sites; every
  config is checked by nginx before reload and rolled back if it fails. Deploy source
  by uploading an archive — the panel strips the wrapper folder every GitHub download
  comes with. Password-protect a site, block addresses and add redirect rules right in
  the website form.
- **Databases**: manage MySQL/MariaDB and PostgreSQL — create databases, users and
  grants, with an SQL console that renders results as a table.
- **Uptime monitoring**: periodic checks of sites and services, with history, a 24-hour
  uptime figure and a notification through the alert channels when the state changes.
- **Backups**: scheduled database and directory backups shipped to this server, S3
  or WebDAV, with a retention policy and one-click restore.
- **SSL**: self-signed, uploaded, or Let's Encrypt certificates over HTTP-01, renewed
  automatically once fewer than 30 days remain.
- **Alerting**: threshold rules on CPU/RAM/disk/load delivered over Telegram, email
  or webhooks, plus automatic alerts when a backup fails or a certificate renewal breaks.
- **API keys**: call the API from scripts without embedding an account password,
  with scoped privileges and per-key revocation.
- **Multi-server**: copy the same binary to another machine and run `sunpanel agent`
  to have the panel drive it remotely over a token-authenticated TLS channel.
- **Plugins**: extend the panel with a separate service declared in YAML; the panel
  forwards requests along with the caller's identity — see the [guide](docs/PLUGINS.md).
- **UI**: light/dark theme, instant language switching, Ctrl+K command palette
  (matches Vietnamese text typed without diacritics), dedicated mobile layout.

The original roadmap is complete; see the [roadmap](docs/ROADMAP.md) for the order
the milestones were delivered in.

### Security

The panel runs as root, so security is a design constraint rather than a feature:

- Passwords hashed with **Argon2id**; secrets in the database (TOTP keys) encrypted with
  **AES-256-GCM** using a master key stored in a separate `0600` file.
- **Secret entry path**: the panel only answers on a random path; every other URL returns 404,
  so automated scanners find nothing.
- **Path traversal protection**: every file operation goes through `host.SafeJoin`, which blocks
  `../`, absolute paths, and symlinks escaping the root. Each attack shape has its own test.
- **No shell injection**: system commands take `(program, []args)` only, never a shell string.
- **Instant revocation**: revoking a session immediately invalidates its access token rather
  than waiting for expiry.
- Login rate limiting, IP allowlisting, security headers, and a strict CSP.

Found a vulnerability? Please don't open a public issue — email the maintainer instead.

### Development

```sh
git clone https://github.com/thanhtinz/Sunpanel.git && cd Sunpanel

make frontend        # build the UI into web/dist
make build           # build the binary into bin/sunpanel
make run             # build and run

make check           # lint + test
make release         # build all 6 platforms into dist/
```

For UI work, run both processes — Vite proxies `/api` to the backend:

```sh
make run             # terminal 1: backend on port 9527
make frontend-dev    # terminal 2: UI on port 5173
```

Architecture details: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

### License

[AGPL-3.0](LICENSE) — free to use, modify, and distribute, provided that modified versions
running as a network service also publish their source.
