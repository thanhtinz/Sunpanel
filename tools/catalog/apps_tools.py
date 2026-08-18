"""Nhóm công cụ hạ tầng, mạng và bảo mật."""

from model import App, choice, password, port, text

PG = "postgres:16-alpine"

APPS = [
    App(
        key="nginx-proxy-manager", name="Nginx Proxy Manager", category="tool",
        vi="Dựng reverse proxy và xin chứng chỉ Let's Encrypt bằng giao diện, không phải sửa tệp cấu hình.",
        en="Set up reverse proxies and Let's Encrypt certificates from a UI instead of editing config files.",
        website="https://nginxproxymanager.com",
        image="jc21/nginx-proxy-manager", tag_pages=6, min_major=2,
        container_port=81,
        extra_ports=('"${PROXY_HTTP_PORT}:80"', '"${PROXY_HTTPS_PORT}:443"'),
        volumes=("data:/data", "letsencrypt:/etc/letsencrypt"),
        fields=(
            port("ADMIN_PORT", "Cổng quản trị", "Admin port", 8181),
            port("PROXY_HTTP_PORT", "Cổng HTTP", "HTTP port", 80,
                 help_vi="Cổng 80 và 443 phải trống thì Let's Encrypt mới xác minh được tên miền.",
                 help_en="Ports 80 and 443 must be free for Let's Encrypt to verify your domain."),
            port("PROXY_HTTPS_PORT", "Cổng HTTPS", "HTTPS port", 443),
        ),
    ),
    App(
        key="traefik", name="Traefik", category="tool",
        vi="Reverse proxy tự phát hiện container mới và tự xin chứng chỉ cho chúng.",
        en="A reverse proxy that discovers new containers and gets certificates for them automatically.",
        website="https://traefik.io",
        image="traefik", tag_pages=6, min_major=2,
        container_port=8080,
        command="--api.dashboard=true --providers.docker=true --entrypoints.web.address=:80",
        extra_ports=('"${PROXY_HTTP_PORT}:80"',),
        volumes=("/var/run/docker.sock:/var/run/docker.sock:ro", "letsencrypt:/letsencrypt"),
        fields=(
            port("DASHBOARD_PORT", "Cổng bảng điều khiển", "Dashboard port", 8280),
            port("PROXY_HTTP_PORT", "Cổng HTTP", "HTTP port", 80),
        ),
    ),
    App(
        key="caddy", name="Caddy", category="tool",
        vi="Máy chủ web tự lo HTTPS: trỏ tên miền vào là có chứng chỉ, không cần cấu hình gì thêm.",
        en="A web server that handles HTTPS itself — point a domain at it and the certificate appears.",
        website="https://caddyserver.com",
        image="caddy", tag_suffix="-alpine", tag_pages=6, min_major=2,
        container_port=80,
        extra_ports=('"${HTTPS_PORT}:443"',),
        volumes=("${SITE_DIR}:/usr/share/caddy", "data:/data", "config:/config"),
        fields=(
            port("HTTP_PORT", "Cổng HTTP", "HTTP port", 8280),
            port("HTTPS_PORT", "Cổng HTTPS", "HTTPS port", 8443),
            text("SITE_DIR", "Thư mục trang web", "Site folder", "/srv/www"),
        ),
    ),
    App(
        key="nginx", name="Nginx", category="tool",
        vi="Máy chủ web phục vụ trang tĩnh, nhanh và ăn rất ít tài nguyên.",
        en="A web server for static sites — fast and very light on resources.",
        website="https://nginx.org",
        image="nginx", tag_suffix="-alpine", tag_pages=6, min_major=1,
        container_port=80,
        volumes=("${SITE_DIR}:/usr/share/nginx/html:ro",),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8281),
            text("SITE_DIR", "Thư mục trang web", "Site folder", "/srv/www"),
        ),
    ),
    App(
        key="wg-easy", name="WireGuard Easy", category="security",
        vi="Máy chủ VPN WireGuard có giao diện: thêm thiết bị và lấy mã QR trong vài giây.",
        en="A WireGuard VPN server with a UI: add a device and get its QR code in seconds.",
        website="https://github.com/wg-easy/wg-easy",
        image="ghcr.io/wg-easy/wg-easy", registry="ghcr", min_major=7,
        container_port=51821,
        extra_ports=('"${VPN_PORT}:51820/udp"',),
        volumes=("config:/etc/wireguard",),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 51821),
            port("VPN_PORT", "Cổng VPN", "VPN port", 51820,
                 help_vi="Cổng UDP các thiết bị kết nối vào; phải mở trên tường lửa.",
                 help_en="The UDP port devices connect to; it must be open on the firewall."),
            text("PUBLIC_HOST", "Địa chỉ công khai", "Public host", "vpn.example.com",
                 help_vi="Tên miền hoặc IP mà thiết bị bên ngoài dùng để kết nối về.",
                 help_en="The domain or IP outside devices use to reach this server."),
            password("ADMIN_PASSWORD", "Mật khẩu quản trị", "Admin password"),
        ),
        environment={"WG_HOST": "${PUBLIC_HOST}", "PASSWORD": "${ADMIN_PASSWORD}",
                     "WG_PORT": "${VPN_PORT}"},
    ),
    App(
        key="pihole", name="Pi-hole", category="security",
        vi="Chặn quảng cáo ở tầng DNS cho cả nhà, kèm thống kê truy vấn theo thiết bị.",
        en="Blocks ads at the DNS level for the whole household, with per-device query stats.",
        website="https://pi-hole.net",
        image="pihole/pihole", tag_pages=6, min_major=2023,
        container_port=80,
        extra_ports=('"${DNS_PORT}:53/tcp"', '"${DNS_PORT}:53/udp"'),
        volumes=("etc:/etc/pihole", "dnsmasq:/etc/dnsmasq.d"),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8282),
            port("DNS_PORT", "Cổng DNS", "DNS port", 53,
                 help_vi="Cổng 53 thường bị systemd-resolved chiếm; tắt nó hoặc chọn cổng khác.",
                 help_en="Port 53 is usually taken by systemd-resolved; disable it or pick another port."),
            password("ADMIN_PASSWORD", "Mật khẩu quản trị", "Admin password"),
        ),
        environment={"WEBPASSWORD": "${ADMIN_PASSWORD}", "TZ": "Asia/Ho_Chi_Minh"},
    ),
    App(
        key="authentik", name="Authentik", category="security",
        vi="Máy chủ đăng nhập một lần cho mọi ứng dụng nội bộ, hỗ trợ OAuth, SAML và LDAP.",
        en="A single sign-on server for all your internal apps, speaking OAuth, SAML and LDAP.",
        website="https://goauthentik.io",
        image="ghcr.io/goauthentik/server", registry="ghcr", min_major=2023,
        side_images=(PG, "redis:7-alpine"),
        version_values={"DB_IMAGE": PG, "REDIS_IMAGE": "redis:7-alpine"},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 9100),
            password("SECRET_KEY", "Khóa bí mật", "Secret key"),
            password("DB_PASSWORD", "Mật khẩu cơ sở dữ liệu", "Database password"),
        ),
        compose="""
services:
  app:
    image: ${IMAGE}
    container_name: ${CONTAINER_NAME}
    restart: unless-stopped
    command: server
    depends_on:
      - db
      - redis
    environment:
      AUTHENTIK_SECRET_KEY: ${SECRET_KEY}
      AUTHENTIK_REDIS__HOST: redis
      AUTHENTIK_POSTGRESQL__HOST: db
      AUTHENTIK_POSTGRESQL__USER: authentik
      AUTHENTIK_POSTGRESQL__NAME: authentik
      AUTHENTIK_POSTGRESQL__PASSWORD: ${DB_PASSWORD}
    ports:
      - "${HTTP_PORT}:9000"
    volumes:
      - media:/media
      - templates:/templates

  worker:
    image: ${IMAGE}
    container_name: ${CONTAINER_NAME}-worker
    restart: unless-stopped
    command: worker
    depends_on:
      - db
      - redis
    environment:
      AUTHENTIK_SECRET_KEY: ${SECRET_KEY}
      AUTHENTIK_REDIS__HOST: redis
      AUTHENTIK_POSTGRESQL__HOST: db
      AUTHENTIK_POSTGRESQL__USER: authentik
      AUTHENTIK_POSTGRESQL__NAME: authentik
      AUTHENTIK_POSTGRESQL__PASSWORD: ${DB_PASSWORD}
    volumes:
      - media:/media
      - templates:/templates

  redis:
    image: ${REDIS_IMAGE}
    container_name: ${CONTAINER_NAME}-redis
    restart: unless-stopped

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      POSTGRES_DB: authentik
      POSTGRES_USER: authentik
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - db:/var/lib/postgresql/data

volumes:
  media:
  templates:
  db:
""",
    ),
    App(
        key="keycloak", name="Keycloak", category="security",
        vi="Máy chủ định danh cấp doanh nghiệp: đăng nhập một lần, liên kết LDAP và xác thực hai lớp.",
        en="An enterprise identity server: single sign-on, LDAP federation and two-factor auth.",
        website="https://keycloak.org",
        image="quay.io/keycloak/keycloak", registry="dockerhub",
        fixed_tags=("26.0.7", "25.0.6"),
        container_port=8080,
        command="start-dev",
        volumes=(),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8380),
            text("ADMIN_USER", "Tài khoản quản trị", "Admin user", "admin"),
            password("ADMIN_PASSWORD", "Mật khẩu quản trị", "Admin password"),
        ),
        environment={"KEYCLOAK_ADMIN": "${ADMIN_USER}",
                     "KEYCLOAK_ADMIN_PASSWORD": "${ADMIN_PASSWORD}"},
    ),
    App(
        key="cloudflared", name="Cloudflare Tunnel", category="security",
        vi="Mở dịch vụ nội bộ ra Internet qua Cloudflare mà không cần mở cổng nào trên tường lửa.",
        en="Publishes an internal service through Cloudflare without opening a single firewall port.",
        website="https://developers.cloudflare.com/cloudflare-one",
        image="cloudflare/cloudflared", tag_pages=6,
        command="tunnel --no-autoupdate run --token ${TUNNEL_TOKEN}",
        volumes=(),
        fields=(
            password("TUNNEL_TOKEN", "Mã đường hầm", "Tunnel token",
                     help_vi="Lấy trong bảng điều khiển Cloudflare Zero Trust khi tạo đường hầm.",
                     help_en="Copy it from the Cloudflare Zero Trust dashboard when creating the tunnel."),
        ),
    ),
    App(
        key="crowdsec", name="CrowdSec", category="security",
        vi="Phát hiện và chặn IP tấn công dựa trên nhật ký, chia sẻ danh sách đen với cộng đồng.",
        en="Detects and blocks attacking IPs from your logs, sharing blocklists with the community.",
        website="https://crowdsec.net",
        image="crowdsecurity/crowdsec", tag_pages=6, min_major=1,
        container_port=8080,
        volumes=("config:/etc/crowdsec", "data:/var/lib/crowdsec/data", "${LOG_DIR}:/var/log:ro"),
        fields=(
            port("API_PORT", "Cổng API", "API port", 8480),
            text("LOG_DIR", "Thư mục nhật ký", "Log folder", "/var/log"),
        ),
    ),
    App(
        key="rustdesk", name="RustDesk Server", category="tool",
        vi="Máy chủ điều khiển máy tính từ xa của riêng bạn, thay cho TeamViewer.",
        en="Your own remote desktop relay server, in place of TeamViewer.",
        website="https://rustdesk.com",
        image="rustdesk/rustdesk-server", tag_pages=6, min_major=1,
        container_port=21116,
        command="hbbs",
        extra_ports=('"${RELAY_PORT}:21117"', '"${ID_PORT}:21116/udp"'),
        volumes=("data:/root",),
        fields=(
            port("ID_PORT", "Cổng máy chủ định danh", "ID server port", 21116),
            port("RELAY_PORT", "Cổng chuyển tiếp", "Relay port", 21117),
        ),
    ),
    App(
        key="homepage", name="Homepage", category="tool",
        vi="Trang chủ cho máy chủ: gom liên kết mọi dịch vụ và hiện trạng thái của chúng.",
        en="A start page for your server: every service in one place, with live status.",
        website="https://gethomepage.dev",
        image="ghcr.io/gethomepage/homepage", registry="ghcr", min_major=0,
        container_port=3000,
        volumes=("config:/app/config", "/var/run/docker.sock:/var/run/docker.sock:ro"),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 3020),),
        environment={"HOMEPAGE_ALLOWED_HOSTS": "*"},
    ),
    App(
        key="heimdall", name="Heimdall", category="tool",
        vi="Trang chủ dạng lưới biểu tượng cho mọi ứng dụng đang chạy trên máy chủ.",
        en="An icon-grid start page for every app running on your server.",
        website="https://heimdall.site",
        image="linuxserver/heimdall", tag_pages=6, min_major=2,
        container_port=80, volumes=("config:/config",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 3021),),
        environment={"PUID": "1000", "PGID": "1000", "TZ": "Asia/Ho_Chi_Minh"},
    ),
    App(
        key="duplicati", name="Duplicati", category="storage",
        vi="Sao lưu có mã hóa lên S3, Google Drive, WebDAV và hàng chục dịch vụ khác.",
        en="Encrypted backups to S3, Google Drive, WebDAV and dozens of other services.",
        website="https://duplicati.com",
        image="linuxserver/duplicati", tag_pages=6, min_major=2,
        container_port=8200,
        volumes=("config:/config", "${SOURCE_DIR}:/source:ro", "backups:/backups"),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8200),
            text("SOURCE_DIR", "Thư mục cần sao lưu", "Folder to back up", "/srv"),
        ),
        environment={"PUID": "1000", "PGID": "1000", "TZ": "Asia/Ho_Chi_Minh"},
    ),
    App(
        key="restic-rest", name="Restic REST Server", category="storage",
        vi="Kho lưu bản sao cho restic qua HTTP, nhanh hơn nhiều so với gửi qua SFTP.",
        en="An HTTP repository for restic backups — much faster than pushing over SFTP.",
        website="https://github.com/restic/rest-server",
        image="restic/rest-server", tag_pages=6, min_major=0,
        container_port=8000, volumes=("data:/data",),
        fields=(
            port("HTTP_PORT", "Cổng", "Port", 8600),
            choice("AUTH", "Yêu cầu đăng nhập", "Require login",
                   (("", "Có", "Yes"), ("--no-auth", "Không", "No")), ""),
        ),
        environment={"OPTIONS": "${AUTH}"},
    ),
    App(
        key="seafile", name="Seafile", category="storage",
        vi="Đồng bộ tệp theo khối, nhanh với thư mục nhiều tệp nhỏ như mã nguồn.",
        en="Block-level file sync that stays fast on folders full of small files, like source code.",
        website="https://seafile.com",
        image="seafileltd/seafile-mc", tag_pages=6, min_major=10,
        side_images=("mariadb:11", "memcached:1.6-alpine"),
        version_values={"DB_IMAGE": "mariadb:11", "CACHE_IMAGE": "memcached:1.6-alpine"},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8090),
            text("ADMIN_EMAIL", "Email quản trị", "Admin email", "admin@example.com"),
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
      - memcached
    environment:
      DB_HOST: db
      DB_ROOT_PASSWD: ${DB_PASSWORD}
      SEAFILE_ADMIN_EMAIL: ${ADMIN_EMAIL}
      SEAFILE_ADMIN_PASSWORD: ${ADMIN_PASSWORD}
      SEAFILE_SERVER_LETSENCRYPT: "false"
      TIME_ZONE: Asia/Ho_Chi_Minh
    ports:
      - "${HTTP_PORT}:80"
    volumes:
      - data:/shared

  memcached:
    image: ${CACHE_IMAGE}
    container_name: ${CONTAINER_NAME}-cache
    restart: unless-stopped
    entrypoint: memcached -m 256

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      MARIADB_ROOT_PASSWORD: ${DB_PASSWORD}
      MARIADB_AUTO_UPGRADE: "1"
    volumes:
      - db:/var/lib/mysql

volumes:
  data:
  db:
""",
    ),
    App(
        key="kopia", name="Kopia", category="storage",
        vi="Sao lưu có khử trùng lặp và mã hóa, gửi lên bất kỳ kho đối tượng nào.",
        en="Deduplicated, encrypted backups to any object store.",
        website="https://kopia.io",
        image="kopia/kopia", tag_pages=6, min_major=0,
        container_port=51515,
        command="server start --insecure --address=0.0.0.0:51515 --server-username=${ADMIN_USER} --server-password=${ADMIN_PASSWORD}",
        volumes=("config:/app/config", "cache:/app/cache", "${SOURCE_DIR}:/data:ro", "repo:/repository"),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 51515),
            text("SOURCE_DIR", "Thư mục cần sao lưu", "Folder to back up", "/srv"),
            text("ADMIN_USER", "Tài khoản", "User", "admin"),
            password("ADMIN_PASSWORD", "Mật khẩu", "Password"),
        ),
    ),
    App(
        key="paperless-ngx", name="Paperless-ngx", category="productivity",
        vi="Số hóa giấy tờ: quét, nhận dạng chữ, gắn thẻ và tìm lại bằng nội dung.",
        en="Goes paperless: scan, OCR, tag and find documents by their contents.",
        website="https://docs.paperless-ngx.com",
        image="ghcr.io/paperless-ngx/paperless-ngx", registry="ghcr", min_major=2,
        side_images=(PG, "redis:7-alpine"),
        version_values={"DB_IMAGE": PG, "REDIS_IMAGE": "redis:7-alpine"},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8777),
            text("ADMIN_USER", "Tài khoản quản trị", "Admin user", "admin"),
            password("ADMIN_PASSWORD", "Mật khẩu quản trị", "Admin password"),
            password("SECRET_KEY", "Khóa bí mật", "Secret key"),
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
      - redis
    environment:
      PAPERLESS_REDIS: redis://redis:6379
      PAPERLESS_DBHOST: db
      PAPERLESS_DBNAME: paperless
      PAPERLESS_DBUSER: paperless
      PAPERLESS_DBPASS: ${DB_PASSWORD}
      PAPERLESS_SECRET_KEY: ${SECRET_KEY}
      PAPERLESS_ADMIN_USER: ${ADMIN_USER}
      PAPERLESS_ADMIN_PASSWORD: ${ADMIN_PASSWORD}
      PAPERLESS_OCR_LANGUAGE: vie+eng
      PAPERLESS_TIME_ZONE: Asia/Ho_Chi_Minh
    ports:
      - "${HTTP_PORT}:8000"
    volumes:
      - data:/usr/src/paperless/data
      - media:/usr/src/paperless/media
      - consume:/usr/src/paperless/consume

  redis:
    image: ${REDIS_IMAGE}
    container_name: ${CONTAINER_NAME}-redis
    restart: unless-stopped

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      POSTGRES_DB: paperless
      POSTGRES_USER: paperless
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - db:/var/lib/postgresql/data

volumes:
  data:
  media:
  consume:
  db:
""",
    ),
    App(
        key="vikunja", name="Vikunja", category="productivity",
        vi="Quản lý công việc theo danh sách, bảng Kanban và lịch, dùng được cho cả nhóm.",
        en="Task management as lists, Kanban boards and calendars, for teams as well as one person.",
        website="https://vikunja.io",
        image="vikunja/vikunja", tag_pages=6, min_major=0,
        container_port=3456, volumes=("files:/app/vikunja/files", "db:/db"),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 3456),
            text("PUBLIC_URL", "Địa chỉ trang", "Site URL", "http://localhost:3456"),
            password("JWT_SECRET", "Khóa ký phiên", "Session secret"),
        ),
        environment={"VIKUNJA_SERVICE_PUBLICURL": "${PUBLIC_URL}",
                     "VIKUNJA_SERVICE_JWTSECRET": "${JWT_SECRET}",
                     "VIKUNJA_DATABASE_PATH": "/db/vikunja.db"},
    ),
    App(
        key="planka", name="Planka", category="productivity",
        vi="Bảng Kanban thời gian thực cho nhóm nhỏ, nhẹ và không phụ thuộc dịch vụ ngoài.",
        en="A real-time Kanban board for small teams, light and free of outside services.",
        website="https://planka.app",
        image="ghcr.io/plankanban/planka", registry="ghcr", min_major=1,
        side_images=(PG,), version_values={"DB_IMAGE": PG},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 3030),
            text("BASE_URL", "Địa chỉ trang", "Site URL", "http://localhost:3030"),
            text("ADMIN_EMAIL", "Email quản trị", "Admin email", "admin@example.com"),
            password("ADMIN_PASSWORD", "Mật khẩu quản trị", "Admin password"),
            password("SECRET_KEY", "Khóa bí mật", "Secret key"),
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
      BASE_URL: ${BASE_URL}
      DATABASE_URL: postgresql://planka:${DB_PASSWORD}@db/planka
      SECRET_KEY: ${SECRET_KEY}
      DEFAULT_ADMIN_EMAIL: ${ADMIN_EMAIL}
      DEFAULT_ADMIN_PASSWORD: ${ADMIN_PASSWORD}
      DEFAULT_ADMIN_NAME: Admin
      DEFAULT_ADMIN_USERNAME: admin
    ports:
      - "${HTTP_PORT}:1337"
    volumes:
      - avatars:/app/public/user-avatars
      - attachments:/app/private/attachments

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      POSTGRES_DB: planka
      POSTGRES_USER: planka
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - db:/var/lib/postgresql/data

volumes:
  avatars:
  attachments:
  db:
""",
    ),
    App(
        key="focalboard", name="Focalboard", category="productivity",
        vi="Bảng công việc kiểu Trello chạy trên máy chủ của bạn.",
        en="A Trello-style project board running on your own server.",
        website="https://focalboard.com",
        image="mattermost/focalboard", tag_pages=6, min_major=7,
        container_port=8000, volumes=("data:/opt/focalboard/data",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 8100),),
    ),
    App(
        key="mattermost", name="Mattermost", category="productivity",
        vi="Nhắn tin nhóm theo kênh, thay Slack mà dữ liệu nằm trên máy chủ của bạn.",
        en="Channel-based team chat — a Slack replacement with your data on your server.",
        website="https://mattermost.com",
        image="mattermost/mattermost-team-edition", tag_pages=6, min_major=9,
        side_images=(PG,), version_values={"DB_IMAGE": PG},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8065),
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
      MM_SQLSETTINGS_DRIVERNAME: postgres
      MM_SQLSETTINGS_DATASOURCE: postgres://mattermost:${DB_PASSWORD}@db:5432/mattermost?sslmode=disable
    ports:
      - "${HTTP_PORT}:8065"
    volumes:
      - data:/mattermost/data
      - config:/mattermost/config

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      POSTGRES_DB: mattermost
      POSTGRES_USER: mattermost
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - db:/var/lib/postgresql/data

volumes:
  data:
  config:
  db:
""",
    ),
    App(
        key="rocketchat", name="Rocket.Chat", category="productivity",
        vi="Nền tảng nhắn tin nhóm đầy đủ: kênh, gọi thoại, cầu nối với các mạng khác.",
        en="A complete team messaging platform: channels, calls and bridges to other networks.",
        website="https://rocket.chat",
        image="rocketchat/rocket.chat", tag_pages=6, min_major=6,
        side_images=("mongo:7",), version_values={"DB_IMAGE": "mongo:7"},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 3040),
            text("ROOT_URL", "Địa chỉ trang", "Site URL", "http://localhost:3040"),
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
      ROOT_URL: ${ROOT_URL}
      MONGO_URL: mongodb://db:27017/rocketchat?replicaSet=rs0
      MONGO_OPLOG_URL: mongodb://db:27017/local?replicaSet=rs0
    ports:
      - "${HTTP_PORT}:3000"
    volumes:
      - uploads:/app/uploads

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    command: mongod --replSet rs0 --oplogSize 128
    volumes:
      - db:/data/db

volumes:
  uploads:
  db:
""",
    ),
    App(
        key="excalidraw", name="Excalidraw", category="productivity",
        vi="Bảng vẽ tay để phác sơ đồ và ý tưởng, chia sẻ liên kết cho người khác cùng vẽ.",
        en="A hand-drawn whiteboard for diagrams and ideas, shareable by link.",
        website="https://excalidraw.com",
        image="excalidraw/excalidraw", fixed_tags=("latest",),
        container_port=80, volumes=(),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 3050),),
    ),
    App(
        key="drawio", name="draw.io", category="productivity",
        vi="Vẽ sơ đồ khối, sơ đồ mạng và lưu đồ ngay trong trình duyệt.",
        en="Draw block diagrams, network maps and flowcharts in the browser.",
        website="https://drawio.com",
        image="jgraph/drawio", tag_pages=6, min_major=20,
        container_port=8080, volumes=(),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 3051),),
    ),
    App(
        key="freshrss", name="FreshRSS", category="productivity",
        vi="Đọc tin RSS tự quản, đồng bộ với ứng dụng đọc trên điện thoại.",
        en="A self-hosted RSS reader that syncs with mobile reader apps.",
        website="https://freshrss.org",
        image="freshrss/freshrss", tag_pages=6, min_major=1,
        container_port=80, volumes=("data:/var/www/FreshRSS/data",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 3060),),
        environment={"TZ": "Asia/Ho_Chi_Minh", "CRON_MIN": "*/20"},
    ),
    App(
        key="miniflux", name="Miniflux", category="productivity",
        vi="Đọc tin RSS tối giản, nhanh, chỉ có đúng những gì cần để đọc.",
        en="A minimalist RSS reader — fast, with only what reading actually needs.",
        website="https://miniflux.app",
        image="miniflux/miniflux", tag_pages=6, min_major=2,
        side_images=(PG,), version_values={"DB_IMAGE": PG},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 3061),
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
      DATABASE_URL: postgres://miniflux:${DB_PASSWORD}@db/miniflux?sslmode=disable
      RUN_MIGRATIONS: "1"
      CREATE_ADMIN: "1"
      ADMIN_USERNAME: ${ADMIN_USER}
      ADMIN_PASSWORD: ${ADMIN_PASSWORD}
    ports:
      - "${HTTP_PORT}:8080"

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      POSTGRES_DB: miniflux
      POSTGRES_USER: miniflux
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - db:/var/lib/postgresql/data

volumes:
  db:
""",
    ),
    App(
        key="wallabag", name="Wallabag", category="productivity",
        vi="Lưu bài viết để đọc sau, giữ lại nội dung ngay cả khi trang gốc biến mất.",
        en="Saves articles for later and keeps the text even if the original page disappears.",
        website="https://wallabag.org",
        image="wallabag/wallabag", tag_pages=6, min_major=2,
        container_port=80, volumes=("data:/var/www/wallabag/data",),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 3062),
            text("SITE_URL", "Địa chỉ trang", "Site URL", "http://localhost:3062"),
            password("SECRET", "Khóa bí mật", "Secret"),
        ),
        environment={"SYMFONY__ENV__DOMAIN_NAME": "${SITE_URL}",
                     "SYMFONY__ENV__SECRET": "${SECRET}"},
    ),
    App(
        key="linkding", name="linkding", category="productivity",
        vi="Kho dấu trang gọn nhẹ, có gắn thẻ và tiện ích cho trình duyệt.",
        en="A lightweight bookmark manager with tags and browser extensions.",
        website="https://github.com/sissbruecker/linkding",
        image="sissbruecker/linkding", tag_pages=6, min_major=1,
        container_port=9090, volumes=("data:/etc/linkding/data",),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 9092),
            text("ADMIN_USER", "Tài khoản quản trị", "Admin user", "admin"),
            password("ADMIN_PASSWORD", "Mật khẩu quản trị", "Admin password"),
        ),
        environment={"LD_SUPERUSER_NAME": "${ADMIN_USER}",
                     "LD_SUPERUSER_PASSWORD": "${ADMIN_PASSWORD}"},
    ),

    App(
        key="shiori", name="Shiori", category="productivity",
        vi="Kho đánh dấu trang tự lưu nội dung bài viết, đọc lại được cả khi trang gốc biến mất.",
        en="A bookmark store that keeps a copy of each article, readable after the original disappears.",
        website="https://github.com/go-shiori/shiori",
        image="ghcr.io/go-shiori/shiori", registry="ghcr", min_major=1,
        container_port=8080, volumes=("data:/shiori",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 8085),),
        environment={"SHIORI_DIR": "/shiori"},
    ),
    App(
        key="wallos", name="Wallos", category="productivity",
        vi="Sổ theo dõi thuê bao: nhắc ngày gia hạn và cộng tổng tiền phải trả mỗi tháng.",
        en="A subscription tracker: reminds you of renewal dates and totals what you pay each month.",
        website="https://wallosapp.com",
        image="bellamy/wallos", tag_pages=6, min_major=2,
        container_port=80,
        volumes=("db:/var/www/html/db", "logos:/var/www/html/images/uploads/logos"),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 8286),),
        environment={"TZ": "Asia/Ho_Chi_Minh"},
    ),
    App(
        key="docuseal", name="DocuSeal", category="productivity",
        vi="Ký tài liệu điện tử: tải PDF lên, đặt ô ký rồi gửi liên kết cho người ký.",
        en="Signs documents online: upload a PDF, drop in the fields and send the signer a link.",
        website="https://docuseal.com",
        image="docuseal/docuseal", tag_pages=6, min_major=1,
        container_port=3000, volumes=("data:/data",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 3010),),
    ),
]
