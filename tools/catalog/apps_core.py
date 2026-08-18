"""Nhóm ứng dụng nền tảng: web, cơ sở dữ liệu, quản trị máy chủ."""

from model import App, choice, password, port, text

PG = "postgres:16-alpine"
MYSQL = "mysql:8"
MARIADB = "mariadb:11"

APPS = [
    App(
        key="wordpress", name="WordPress", category="website",
        vi="Nền tảng viết blog và làm website phổ biến nhất, kèm sẵn cơ sở dữ liệu MariaDB.",
        en="The most widely used blogging and website platform, with a MariaDB database included.",
        website="https://wordpress.org",
        image="wordpress", tag_suffix="-apache", tag_pages=6, min_major=5,
        side_images=(MARIADB,), version_values={"DB_IMAGE": MARIADB},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8090),
            password("DB_PASSWORD", "Mật khẩu cơ sở dữ liệu", "Database password"),
        ),
        compose="""
services:
  app:
    image: ${IMAGE}
    container_name: ${CONTAINER_NAME}
    restart: unless-stopped
    depends_on:
      - db
    environment:
      WORDPRESS_DB_HOST: db
      WORDPRESS_DB_NAME: wordpress
      WORDPRESS_DB_USER: wordpress
      WORDPRESS_DB_PASSWORD: ${DB_PASSWORD}
    ports:
      - "${HTTP_PORT}:80"
    volumes:
      - data:/var/www/html

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      MARIADB_DATABASE: wordpress
      MARIADB_USER: wordpress
      MARIADB_PASSWORD: ${DB_PASSWORD}
      MARIADB_RANDOM_ROOT_PASSWORD: "1"
    volumes:
      - db:/var/lib/mysql

volumes:
  data:
  db:
""",
    ),
    App(
        key="ghost", name="Ghost", category="website",
        vi="Nền tảng viết blog và bản tin, nhanh và tập trung vào nội dung.",
        en="A fast publishing platform for blogs and newsletters.",
        website="https://ghost.org",
        image="ghost", tag_suffix="-alpine", tag_pages=8, min_major=4,
        side_images=(MYSQL,), version_values={"DB_IMAGE": MYSQL},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 2368),
            text("SITE_URL", "Địa chỉ trang", "Site URL", "http://localhost:2368",
                 help_vi="Ghost dựng mọi liên kết từ địa chỉ này, nên phải đúng địa chỉ người đọc gõ vào.",
                 help_en="Ghost builds every link from this address, so it must match what readers type."),
            password("DB_PASSWORD", "Mật khẩu cơ sở dữ liệu", "Database password"),
        ),
        compose="""
services:
  app:
    image: ${IMAGE}
    container_name: ${CONTAINER_NAME}
    restart: unless-stopped
    depends_on:
      - db
    environment:
      url: ${SITE_URL}
      database__client: mysql
      database__connection__host: db
      database__connection__user: ghost
      database__connection__password: ${DB_PASSWORD}
      database__connection__database: ghost
    ports:
      - "${HTTP_PORT}:2368"
    volumes:
      - content:/var/lib/ghost/content

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      MYSQL_DATABASE: ghost
      MYSQL_USER: ghost
      MYSQL_PASSWORD: ${DB_PASSWORD}
      MYSQL_RANDOM_ROOT_PASSWORD: "1"
    volumes:
      - db:/var/lib/mysql

volumes:
  content:
  db:
""",
    ),
    App(
        key="gitea", name="Gitea", category="development",
        vi="Máy chủ Git tự quản, nhẹ và đầy đủ: kho mã, pull request, issue và CI.",
        en="A lightweight self-hosted Git service with repositories, pull requests, issues and CI.",
        website="https://gitea.io",
        image="gitea/gitea", tag_pages=8, min_major=1,
        side_images=(PG,), version_values={"DB_IMAGE": PG},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 3000),
            port("SSH_PORT", "Cổng SSH", "SSH port", 2222,
                 help_vi="Cổng để đẩy mã bằng SSH; đặt khác 22 để không đụng SSH của máy chủ.",
                 help_en="Port for pushing over SSH; keep it off 22 so it does not clash with the host."),
            password("DB_PASSWORD", "Mật khẩu cơ sở dữ liệu", "Database password"),
        ),
        compose="""
services:
  app:
    image: ${IMAGE}
    container_name: ${CONTAINER_NAME}
    restart: unless-stopped
    depends_on:
      - db
    environment:
      GITEA__database__DB_TYPE: postgres
      GITEA__database__HOST: db:5432
      GITEA__database__NAME: gitea
      GITEA__database__USER: gitea
      GITEA__database__PASSWD: ${DB_PASSWORD}
    ports:
      - "${HTTP_PORT}:3000"
      - "${SSH_PORT}:22"
    volumes:
      - data:/data

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      POSTGRES_DB: gitea
      POSTGRES_USER: gitea
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - db:/var/lib/postgresql/data

volumes:
  data:
  db:
""",
    ),
    App(
        key="nextcloud", name="Nextcloud", category="storage",
        vi="Ổ đĩa đám mây tự quản: đồng bộ tệp, chia sẻ liên kết, lịch và danh bạ.",
        en="Self-hosted cloud storage with file sync, share links, calendar and contacts.",
        website="https://nextcloud.com",
        image="nextcloud", tag_suffix="-apache", tag_pages=8, min_major=27,
        side_images=(PG,), version_values={"DB_IMAGE": PG},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8081),
            text("ADMIN_USER", "Tài khoản quản trị", "Admin user", "admin"),
            password("ADMIN_PASSWORD", "Mật khẩu quản trị", "Admin password"),
            password("DB_PASSWORD", "Mật khẩu cơ sở dữ liệu", "Database password"),
        ),
        compose="""
services:
  app:
    image: ${IMAGE}
    container_name: ${CONTAINER_NAME}
    restart: unless-stopped
    depends_on:
      - db
    environment:
      POSTGRES_HOST: db
      POSTGRES_DB: nextcloud
      POSTGRES_USER: nextcloud
      POSTGRES_PASSWORD: ${DB_PASSWORD}
      NEXTCLOUD_ADMIN_USER: ${ADMIN_USER}
      NEXTCLOUD_ADMIN_PASSWORD: ${ADMIN_PASSWORD}
    ports:
      - "${HTTP_PORT}:80"
    volumes:
      - data:/var/www/html

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      POSTGRES_DB: nextcloud
      POSTGRES_USER: nextcloud
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - db:/var/lib/postgresql/data

volumes:
  data:
  db:
""",
    ),
    App(
        key="umami", name="Umami", category="monitoring",
        vi="Thống kê truy cập website, nhẹ và không theo dõi người dùng bằng cookie.",
        en="Website analytics that stays light and does not track visitors with cookies.",
        website="https://umami.is",
        image="ghcr.io/umami-software/umami", registry="ghcr",
        tag_prefix="postgresql-", min_major=2,
        side_images=(PG,), version_values={"DB_IMAGE": PG},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 3002),
            password("APP_SECRET", "Khóa ký phiên", "Session secret"),
            password("DB_PASSWORD", "Mật khẩu cơ sở dữ liệu", "Database password"),
        ),
        compose="""
services:
  app:
    image: ${IMAGE}
    container_name: ${CONTAINER_NAME}
    restart: unless-stopped
    depends_on:
      - db
    environment:
      DATABASE_URL: postgresql://umami:${DB_PASSWORD}@db:5432/umami
      DATABASE_TYPE: postgresql
      APP_SECRET: ${APP_SECRET}
    ports:
      - "${HTTP_PORT}:3000"

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      POSTGRES_DB: umami
      POSTGRES_USER: umami
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - db:/var/lib/postgresql/data

volumes:
  db:
""",
    ),
    App(
        key="metabase", name="Metabase", category="database",
        vi="Hỏi đáp số liệu bằng giao diện kéo thả, không cần viết SQL, dựng bảng biểu chia sẻ được.",
        en="Ask questions of your data by pointing and clicking — no SQL needed — and share dashboards.",
        website="https://metabase.com",
        image="metabase/metabase", tag_pages=6,
        side_images=(PG,), version_values={"DB_IMAGE": PG},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 3003),
            password("DB_PASSWORD", "Mật khẩu cơ sở dữ liệu", "Database password"),
        ),
        compose="""
services:
  app:
    image: ${IMAGE}
    container_name: ${CONTAINER_NAME}
    restart: unless-stopped
    depends_on:
      - db
    environment:
      MB_DB_TYPE: postgres
      MB_DB_DBNAME: metabase
      MB_DB_PORT: "5432"
      MB_DB_USER: metabase
      MB_DB_PASS: ${DB_PASSWORD}
      MB_DB_HOST: db
    ports:
      - "${HTTP_PORT}:3000"

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      POSTGRES_DB: metabase
      POSTGRES_USER: metabase
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - db:/var/lib/postgresql/data

volumes:
  db:
""",
    ),
    App(
        key="n8n", name="n8n", category="automation",
        vi="Tự động hóa quy trình bằng cách nối các dịch vụ lại với nhau, không cần viết mã.",
        en="Workflow automation that wires services together without writing code.",
        website="https://n8n.io",
        image="n8nio/n8n", tag_pages=6, min_major=1,
        container_port=5678, default_port=5678,
        volumes=("data:/home/node/.n8n",),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 5678),
            text("HOST_NAME", "Tên miền", "Host name", "localhost",
                 help_vi="Địa chỉ n8n dùng để dựng liên kết webhook gửi cho dịch vụ ngoài.",
                 help_en="The address n8n uses to build the webhook links it hands to outside services."),
            choice("TIMEZONE", "Múi giờ", "Timezone",
                   (("Asia/Ho_Chi_Minh", "Việt Nam", "Vietnam"),
                    ("UTC", "UTC", "UTC"),
                    ("Asia/Singapore", "Singapore", "Singapore"),
                    ("Europe/London", "London", "London")),
                   "Asia/Ho_Chi_Minh"),
        ),
        environment={"N8N_HOST": "${HOST_NAME}", "GENERIC_TIMEZONE": "${TIMEZONE}",
                     "TZ": "${TIMEZONE}", "N8N_PORT": "5678"},
    ),
    App(
        key="uptime-kuma", name="Uptime Kuma", category="monitoring",
        vi="Giám sát website và dịch vụ, báo động qua Telegram, email và hàng chục kênh khác.",
        en="Monitors websites and services and alerts through Telegram, email and dozens more channels.",
        website="https://uptime.kuma.pet",
        image="louislam/uptime-kuma", tag_pages=6, min_major=1,
        container_port=3001, volumes=("data:/app/data",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 3001),),
    ),
    App(
        key="portainer", name="Portainer", category="tool",
        vi="Giao diện quản lý Docker: container, ngăn xếp, image và nhật ký trong một chỗ.",
        en="A Docker management UI for containers, stacks, images and logs in one place.",
        website="https://portainer.io",
        image="portainer/portainer-ce", tag_pages=6, min_major=2,
        container_port=9000,
        volumes=("/var/run/docker.sock:/var/run/docker.sock", "data:/data"),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 9000,
                     help_vi="Portainer được gắn ổ cắm Docker nên ai vào được nó là có toàn quyền máy chủ. Đừng mở cổng này ra Internet.",
                     help_en="Portainer mounts the Docker socket, so anyone who reaches it controls the whole host. Do not expose this port to the Internet."),),
    ),
    App(
        key="vaultwarden", name="Vaultwarden", category="security",
        vi="Máy chủ kho mật khẩu tương thích Bitwarden, nhẹ và chạy được trên máy cấu hình thấp.",
        en="A lightweight Bitwarden-compatible password vault server that runs on small machines.",
        website="https://github.com/dani-garcia/vaultwarden",
        image="vaultwarden/server", tag_pages=6, min_major=1,
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8082),
            password("ADMIN_TOKEN", "Khóa trang quản trị", "Admin page token",
                     help_vi="Dùng để mở /admin. Giữ kín như mật khẩu chính.",
                     help_en="Opens /admin. Keep it as secret as your master password."),
            choice("SIGNUPS_ALLOWED", "Cho phép tự đăng ký", "Allow sign-ups",
                   (("true", "Có", "Yes"), ("false", "Không", "No")), "false"),
        ),
        environment={"ADMIN_TOKEN": "${ADMIN_TOKEN}", "SIGNUPS_ALLOWED": "${SIGNUPS_ALLOWED}"},
    ),
    App(
        key="jellyfin", name="Jellyfin", category="media",
        vi="Máy chủ phim và nhạc cá nhân, phát tới trình duyệt, TV và điện thoại.",
        en="A personal movie and music server that streams to browsers, TVs and phones.",
        website="https://jellyfin.org",
        image="jellyfin/jellyfin", tag_pages=6, min_major=10,
        container_port=8096,
        volumes=("config:/config", "cache:/cache", "${MEDIA_DIR}:/media:ro"),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8096),
            text("MEDIA_DIR", "Thư mục phim nhạc", "Media folder", "/srv/media",
                 help_vi="Thư mục sẵn có trên máy chủ, được gắn vào ứng dụng ở chế độ chỉ đọc.",
                 help_en="An existing folder on the host, mounted into the app read-only."),
        ),
    ),
    App(
        key="grafana", name="Grafana", category="monitoring",
        vi="Bảng biểu đồ cho mọi nguồn số liệu: Prometheus, cơ sở dữ liệu, nhật ký.",
        en="Dashboards for any data source: Prometheus, databases and logs.",
        website="https://grafana.com",
        image="grafana/grafana", tag_pages=6, min_major=9,
        container_port=3000, volumes=("data:/var/lib/grafana",),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 3004),
            text("ADMIN_USER", "Tài khoản quản trị", "Admin user", "admin"),
            password("ADMIN_PASSWORD", "Mật khẩu quản trị", "Admin password"),
        ),
        environment={"GF_SECURITY_ADMIN_USER": "${ADMIN_USER}",
                     "GF_SECURITY_ADMIN_PASSWORD": "${ADMIN_PASSWORD}"},
    ),
    App(
        key="minio", name="MinIO", category="storage",
        vi="Kho đối tượng tương thích S3, dùng làm đích sao lưu ngay trên máy chủ của bạn.",
        en="S3-compatible object storage — a backup target that lives on your own server.",
        website="https://min.io",
        image="minio/minio", tag_pages=4, fixed_tags=("RELEASE.2024-09-13T20-26-02Z",),
        container_port=9001,
        command='server /data --console-address ":9001"',
        extra_ports=('"${API_PORT}:9000"',),
        fields=(
            port("CONSOLE_PORT", "Cổng giao diện", "Console port", 9003),
            port("API_PORT", "Cổng S3", "S3 port", 9002),
            text("ROOT_USER", "Tên truy cập", "Access key", "minioadmin"),
            password("ROOT_PASSWORD", "Khóa bí mật", "Secret key"),
        ),
        environment={"MINIO_ROOT_USER": "${ROOT_USER}", "MINIO_ROOT_PASSWORD": "${ROOT_PASSWORD}"},
    ),
    App(
        key="filebrowser", name="File Browser", category="tool",
        vi="Trình quản lý tệp qua web cho một thư mục cụ thể, chia sẻ được cho người ngoài.",
        en="A web file manager scoped to one folder, with share links for outsiders.",
        website="https://filebrowser.org",
        image="filebrowser/filebrowser", tag_pages=6, min_major=2,
        volumes=("${ROOT_DIR}:/srv", "data:/database"),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8083),
            text("ROOT_DIR", "Thư mục gốc", "Root folder", "/srv",
                 help_vi="Người dùng chỉ thấy được bên trong thư mục này.",
                 help_en="Users can only see inside this folder."),
        ),
    ),
    App(
        key="syncthing", name="Syncthing", category="storage",
        vi="Đồng bộ thư mục giữa các máy theo kiểu ngang hàng, không đi qua máy chủ trung gian.",
        en="Peer-to-peer folder sync between machines with no server in the middle.",
        website="https://syncthing.net",
        image="syncthing/syncthing", tag_pages=6, min_major=1,
        container_port=8384,
        volumes=("config:/var/syncthing/config", "${SYNC_DIR}:/var/syncthing/data"),
        extra_ports=('"${SYNC_PORT}:22000/tcp"', '"${SYNC_PORT}:22000/udp"'),
        fields=(
            port("HTTP_PORT", "Cổng giao diện", "Web UI port", 8384),
            port("SYNC_PORT", "Cổng đồng bộ", "Sync port", 22000,
                 help_vi="Cổng các máy khác kết nối tới để truyền dữ liệu; cần mở trên tường lửa.",
                 help_en="The port other devices connect to for transfers; open it on the firewall."),
            text("SYNC_DIR", "Thư mục đồng bộ", "Sync folder", "/srv/sync"),
        ),
    ),
    App(
        key="memos", name="Memos", category="productivity",
        vi="Sổ ghi chú nhanh dạng dòng thời gian, gõ là lưu, tìm lại bằng thẻ.",
        en="A quick note timeline: type, save, and find things again by tag.",
        website="https://usememos.com",
        image="neosmemo/memos", tag_pages=6,
        container_port=5230, volumes=("data:/var/opt/memos",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 5230),),
    ),
    App(
        key="stirling-pdf", name="Stirling PDF", category="tool",
        vi="Bộ công cụ PDF chạy ngay trên máy chủ: gộp, tách, xoay, nén, ký, chuyển định dạng.",
        en="A PDF toolbox on your own server: merge, split, rotate, compress, sign and convert.",
        website="https://stirlingpdf.com",
        image="stirlingtools/stirling-pdf", tag_pages=6,
        container_port=8080, volumes=("data:/usr/share/tessdata",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 8084),),
    ),
    App(
        key="it-tools", name="IT Tools", category="tool",
        vi="Hộp công cụ cho người làm kỹ thuật: mã hóa, JSON, mã QR, chuyển đổi, sinh chuỗi.",
        en="A toolbox for engineers: encoding, JSON, QR codes, converters and generators.",
        website="https://it-tools.tech",
        image="corentinth/it-tools", tag_pages=4, fixed_tags=("2024.10.22",),
        volumes=(),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 8085),),
    ),
    App(
        key="adguard-home", name="AdGuard Home", category="security",
        vi="Máy chủ DNS chặn quảng cáo và trình theo dõi cho toàn bộ thiết bị trong mạng.",
        en="A DNS server that blocks ads and trackers for every device on the network.",
        website="https://adguard.com/adguard-home",
        image="adguard/adguardhome", tag_pages=6,
        container_port=3000,
        volumes=("work:/opt/adguardhome/work", "conf:/opt/adguardhome/conf"),
        extra_ports=('"${DNS_PORT}:53/tcp"', '"${DNS_PORT}:53/udp"'),
        fields=(
            port("SETUP_PORT", "Cổng cài đặt", "Setup port", 3080,
                 help_vi="Lần đầu vào cổng này để chạy trình cài đặt; sau đó giao diện quản trị dùng cổng bạn chọn trong đó.",
                 help_en="Open this port once to run the setup wizard; afterwards the admin UI uses the port you pick there."),
            port("DNS_PORT", "Cổng DNS", "DNS port", 53,
                 help_vi="Cổng 53 thường đã bị systemd-resolved chiếm; tắt nó hoặc chọn cổng khác.",
                 help_en="Port 53 is usually taken by systemd-resolved; disable it or pick another port."),
        ),
    ),
    App(
        key="code-server", name="code-server", category="development",
        vi="VS Code chạy trên máy chủ, mở bằng trình duyệt — sửa mã ngay nơi mã đang chạy.",
        en="VS Code running on the server and opened in a browser — edit code where it runs.",
        website="https://github.com/coder/code-server",
        image="codercom/code-server", tag_pages=6, min_major=4,
        container_port=8080,
        volumes=("config:/home/coder/.local/share/code-server", "${WORKSPACE}:/home/coder/project"),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8443),
            password("PASSWORD", "Mật khẩu truy cập", "Access password"),
            text("WORKSPACE", "Thư mục làm việc", "Workspace folder", "/srv/code"),
        ),
        environment={"PASSWORD": "${PASSWORD}"},
    ),
    App(
        key="redis", name="Redis", category="database",
        vi="Bộ nhớ đệm và hàng đợi trong RAM, thứ hầu hết ứng dụng web cần tới sớm hay muộn.",
        en="An in-memory cache and queue that most web apps need sooner or later.",
        website="https://redis.io",
        image="redis", tag_suffix="-alpine", tag_pages=6, min_major=6,
        container_port=6379,
        command="redis-server --requirepass ${PASSWORD} --appendonly yes",
        fields=(
            port("PORT", "Cổng", "Port", 6379,
                 help_vi="Chỉ mở ra Internet khi thật sự cần: Redis không có tường lửa riêng.",
                 help_en="Only expose this to the Internet if you truly must: Redis has no firewall of its own."),
            password(),
        ),
    ),
]
