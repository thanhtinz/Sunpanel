"""Nhóm ứng dụng trí tuệ nhân tạo, danh tính và tiện ích hạ tầng."""

from model import App, choice, password, port, text

PG = "postgres:16-alpine"
MONGO = "mongo:7"

APPS = [
    App(
        key="librechat", name="LibreChat", category="development",
        vi="Giao diện trò chuyện cho nhiều mô hình ngôn ngữ cùng lúc, có lưu hội thoại và chia sẻ.",
        en="A chat interface across many language models at once, with saved and shareable conversations.",
        website="https://librechat.ai",
        image="ghcr.io/danny-avila/librechat", registry="ghcr", min_major=0,
        side_images=(MONGO,), version_values={"DB_IMAGE": MONGO},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 3210),
            password("CREDS_KEY", "Khóa mã hóa", "Encryption key"),
            password("JWT_SECRET", "Khóa ký phiên", "Session secret"),
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
      HOST: 0.0.0.0
      MONGO_URI: mongodb://db:27017/LibreChat
      CREDS_KEY: ${CREDS_KEY}
      JWT_SECRET: ${JWT_SECRET}
      JWT_REFRESH_SECRET: ${JWT_SECRET}
    ports:
      - "${HTTP_PORT}:3080"
    volumes:
      - images:/app/client/public/images

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    volumes:
      - db:/data/db

volumes:
  images:
  db:
""",
    ),
    App(
        key="lobe-chat", name="Lobe Chat", category="development",
        vi="Giao diện trò chuyện với mô hình ngôn ngữ, có kho trợ lý dựng sẵn và nhận diện giọng nói.",
        en="A chat UI for language models with a prebuilt assistant store and speech recognition.",
        website="https://lobehub.com",
        image="lobehub/lobe-chat", tag_pages=6, min_major=1,
        container_port=3210, volumes=(),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 3211),
            text("OLLAMA_URL", "Địa chỉ Ollama", "Ollama URL", "http://localhost:11434"),
            password("ACCESS_CODE", "Mã truy cập", "Access code"),
        ),
        environment={"OLLAMA_PROXY_URL": "${OLLAMA_URL}", "ACCESS_CODE": "${ACCESS_CODE}"},
    ),
    App(
        key="flowise", name="Flowise", category="automation",
        vi="Dựng luồng xử lý cho mô hình ngôn ngữ bằng cách nối các khối, không cần viết mã.",
        en="Builds language-model pipelines by connecting blocks, with no code.",
        website="https://flowiseai.com",
        image="flowiseai/flowise", tag_pages=6, min_major=1,
        container_port=3000, volumes=("data:/root/.flowise",),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 3212),
            text("ADMIN_USER", "Tài khoản quản trị", "Admin user", "admin"),
            password("ADMIN_PASSWORD", "Mật khẩu quản trị", "Admin password"),
        ),
        environment={"FLOWISE_USERNAME": "${ADMIN_USER}", "FLOWISE_PASSWORD": "${ADMIN_PASSWORD}",
                     "PORT": "3000"},
    ),
    App(
        key="libretranslate", name="LibreTranslate", category="tool",
        vi="Máy dịch chạy hoàn toàn nội bộ, không gửi câu chữ của bạn ra dịch vụ nào.",
        en="A translation engine that runs entirely locally and sends your text to nobody.",
        website="https://libretranslate.com",
        image="libretranslate/libretranslate", tag_pages=6, min_major=1,
        container_port=5000, volumes=("models:/home/libretranslate/.local",),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 5500),
            text("LANGUAGES", "Ngôn ngữ tải về", "Languages to load", "en,vi",
                 help_vi="Danh sách mã ngôn ngữ, cách nhau bằng dấu phẩy. Càng nhiều càng tốn RAM và thời gian tải.",
                 help_en="Comma-separated language codes. More languages means more memory and a longer first start."),
        ),
        environment={"LT_LOAD_ONLY": "${LANGUAGES}"},
    ),
    App(
        key="gotenberg", name="Gotenberg", category="tool",
        vi="Dịch vụ chuyển HTML, Markdown và tài liệu Office sang PDF qua một lệnh gọi API.",
        en="An API service that converts HTML, Markdown and Office documents into PDF.",
        website="https://gotenberg.dev",
        image="gotenberg/gotenberg", tag_pages=6, min_major=7,
        container_port=3000, volumes=(),
        fields=(port("HTTP_PORT", "Cổng API", "API port", 3213),),
    ),
    App(
        key="authelia", name="Authelia", category="security",
        vi="Cổng đăng nhập và xác thực hai lớp đặt trước mọi dịch vụ nội bộ.",
        en="A login portal and two-factor gate placed in front of every internal service.",
        website="https://authelia.com",
        image="authelia/authelia", tag_pages=6, min_major=4,
        container_port=9091, volumes=("config:/config",),
        fields=(port("HTTP_PORT", "Cổng", "Port", 9091,
                     help_vi="Authelia cần tệp cấu hình trong thư mục config trước khi chạy được; xem tài liệu dự án.",
                     help_en="Authelia needs a configuration file in the config folder before it will start; see the project docs."),),
        environment={"TZ": "Asia/Ho_Chi_Minh"},
    ),
    App(
        key="lldap", name="LLDAP", category="security",
        vi="Máy chủ LDAP nhẹ có giao diện, đủ dùng làm nơi lưu tài khoản chung cho các ứng dụng.",
        en="A light LDAP server with a UI — enough to hold shared accounts for your apps.",
        website="https://github.com/lldap/lldap",
        image="lldap/lldap", tag_pages=6, tag_any_suffix=True,
        container_port=17170, volumes=("data:/data",),
        extra_ports=('"${LDAP_PORT}:3890"',),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 17170),
            port("LDAP_PORT", "Cổng LDAP", "LDAP port", 3890),
            password("JWT_SECRET", "Khóa ký phiên", "Session secret"),
            password("ADMIN_PASSWORD", "Mật khẩu quản trị", "Admin password"),
        ),
        environment={"LLDAP_JWT_SECRET": "${JWT_SECRET}",
                     "LLDAP_LDAP_USER_PASS": "${ADMIN_PASSWORD}",
                     "LLDAP_LDAP_BASE_DN": "dc=example,dc=com"},
    ),
    App(
        key="zitadel", name="Zitadel", category="security",
        vi="Máy chủ định danh hiện đại: đăng nhập một lần, khóa bảo mật và quản lý nhiều tổ chức.",
        en="A modern identity server: single sign-on, passkeys and multi-organisation management.",
        website="https://zitadel.com",
        image="ghcr.io/zitadel/zitadel", registry="ghcr", min_major=2,
        side_images=(PG,), version_values={"DB_IMAGE": PG},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8500),
            password("MASTERKEY", "Khóa chủ", "Master key",
                     help_vi="Zitadel yêu cầu khóa dài đúng 32 ký tự.",
                     help_en="Zitadel requires a key of exactly 32 characters."),
            password("DB_PASSWORD", "Mật khẩu cơ sở dữ liệu", "Database password"),
        ),
        compose="""
services:
  app:
    image: ${IMAGE}
    container_name: ${CONTAINER_NAME}
    restart: unless-stopped
    command: start-from-init --masterkeyFromEnv --tlsMode disabled
    depends_on:
      - db
    environment:
      ZITADEL_MASTERKEY: ${MASTERKEY}
      ZITADEL_DATABASE_POSTGRES_HOST: db
      ZITADEL_DATABASE_POSTGRES_PORT: "5432"
      ZITADEL_DATABASE_POSTGRES_DATABASE: zitadel
      ZITADEL_DATABASE_POSTGRES_USER_USERNAME: zitadel
      ZITADEL_DATABASE_POSTGRES_USER_PASSWORD: ${DB_PASSWORD}
      ZITADEL_DATABASE_POSTGRES_USER_SSL_MODE: disable
      ZITADEL_DATABASE_POSTGRES_ADMIN_USERNAME: zitadel
      ZITADEL_DATABASE_POSTGRES_ADMIN_PASSWORD: ${DB_PASSWORD}
      ZITADEL_DATABASE_POSTGRES_ADMIN_SSL_MODE: disable
      ZITADEL_EXTERNALSECURE: "false"
    ports:
      - "${HTTP_PORT}:8080"

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      POSTGRES_DB: zitadel
      POSTGRES_USER: zitadel
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - db:/var/lib/postgresql/data

volumes:
  db:
""",
    ),
    App(
        key="guacamole", name="Apache Guacamole", category="tool",
        vi="Mở máy tính từ xa qua trình duyệt: RDP, VNC và SSH, không cần cài phần mềm khách.",
        en="Remote desktops in the browser: RDP, VNC and SSH with no client software.",
        website="https://guacamole.apache.org",
        image="guacamole/guacamole", tag_pages=6, min_major=1,
        side_images=("guacamole/guacd:1.5.5", PG),
        version_values={"GUACD_IMAGE": "guacamole/guacd:1.5.5", "DB_IMAGE": PG},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8501,
                 help_vi="Lần đầu phải nạp lược đồ cơ sở dữ liệu bằng tay; xem tài liệu dự án.",
                 help_en="The database schema must be loaded by hand on first run; see the project docs."),
            password("DB_PASSWORD", "Mật khẩu cơ sở dữ liệu", "Database password"),
        ),
        compose="""
services:
  app:
    image: ${IMAGE}
    container_name: ${CONTAINER_NAME}
    restart: unless-stopped
    depends_on:
      - guacd
      - db
    environment:
      GUACD_HOSTNAME: guacd
      POSTGRESQL_HOSTNAME: db
      POSTGRESQL_DATABASE: guacamole
      POSTGRESQL_USER: guacamole
      POSTGRESQL_PASSWORD: ${DB_PASSWORD}
    ports:
      - "${HTTP_PORT}:8080"

  guacd:
    image: ${GUACD_IMAGE}
    container_name: ${CONTAINER_NAME}-guacd
    restart: unless-stopped

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      POSTGRES_DB: guacamole
      POSTGRES_USER: guacamole
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - db:/var/lib/postgresql/data

volumes:
  db:
""",
    ),
    App(
        key="webtop", name="Webtop", category="tool",
        vi="Cả một môi trường desktop Linux chạy trong trình duyệt, dùng để thử phần mềm đồ họa.",
        en="A full Linux desktop in the browser, handy for trying graphical software.",
        website="https://github.com/linuxserver/docker-webtop",
        image="linuxserver/webtop",
        fixed_tags=("ubuntu-xfce", "debian-xfce", "alpine-xfce"),
        container_port=3000, volumes=("config:/config",),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 3220),
            password("PASSWORD", "Mật khẩu truy cập", "Access password"),
        ),
        environment={"PUID": "1000", "PGID": "1000", "TZ": "Asia/Ho_Chi_Minh",
                     "CUSTOM_USER": "abc", "PASSWORD": "${PASSWORD}"},
    ),
    App(
        key="neko", name="Neko", category="tool",
        vi="Trình duyệt chạy trên máy chủ, nhiều người xem và điều khiển chung một phiên.",
        en="A browser running on the server that several people can watch and drive together.",
        website="https://neko.m1k1o.net",
        image="ghcr.io/m1k1o/neko/firefox", registry="ghcr", min_major=2,
        container_port=8080, volumes=(),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8502),
            password("USER_PASSWORD", "Mật khẩu người xem", "Viewer password"),
            password("ADMIN_PASSWORD", "Mật khẩu quản trị", "Admin password"),
        ),
        environment={"NEKO_MEMBER_MULTIUSER_USER_PASSWORD": "${USER_PASSWORD}",
                     "NEKO_MEMBER_MULTIUSER_ADMIN_PASSWORD": "${ADMIN_PASSWORD}",
                     "NEKO_DESKTOP_SCREEN": "1280x720@30"},
    ),
    App(
        key="rclone", name="Rclone", category="storage",
        vi="Đồng bộ tệp với hơn bảy mươi dịch vụ đám mây, kèm giao diện web để thao tác.",
        en="Syncs files with over seventy cloud services, with a web UI to drive it.",
        website="https://rclone.org",
        image="rclone/rclone", tag_pages=6, min_major=1,
        container_port=5572,
        command="rcd --rc-web-gui --rc-addr :5572 --rc-user ${ADMIN_USER} --rc-pass ${ADMIN_PASSWORD}",
        volumes=("config:/config/rclone", "${SOURCE_DIR}:/data"),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 5572),
            text("SOURCE_DIR", "Thư mục dữ liệu", "Data folder", "/srv"),
            text("ADMIN_USER", "Tài khoản", "User", "admin"),
            password("ADMIN_PASSWORD", "Mật khẩu", "Password"),
        ),
    ),
    App(
        key="garage", name="Garage", category="storage",
        vi="Kho đối tượng tương thích S3 thiết kế cho máy chủ nhỏ và đường truyền chậm.",
        en="S3-compatible object storage designed for small servers and slow links.",
        website="https://garagehq.deuxfleurs.fr",
        image="dxflrs/garage", tag_pages=6, min_major=0,
        container_port=3900,
        volumes=("meta:/var/lib/garage/meta", "data:/var/lib/garage/data", "config:/etc/garage"),
        fields=(port("S3_PORT", "Cổng S3", "S3 port", 3900,
                     help_vi="Garage cần tệp garage.toml trong thư mục cấu hình trước khi chạy; xem tài liệu dự án.",
                     help_en="Garage needs a garage.toml in the config folder before it starts; see the project docs."),),
    ),
    App(
        key="quickwit", name="Quickwit", category="monitoring",
        vi="Kho nhật ký tìm kiếm được, chạy thẳng trên kho đối tượng nên rẻ hơn Elasticsearch nhiều.",
        en="Searchable log storage that runs straight on object storage, far cheaper than Elasticsearch.",
        website="https://quickwit.io",
        image="quickwit/quickwit", tag_pages=6, min_major=0,
        container_port=7280,
        command="run",
        volumes=("data:/quickwit/qwdata",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 7280),),
    ),
    App(
        key="victorialogs", name="VictoriaLogs", category="monitoring",
        vi="Kho nhật ký nhẹ hơn hẳn Loki, truy vấn nhanh mà ăn ít RAM.",
        en="A log store much lighter than Loki: fast queries on little memory.",
        website="https://docs.victoriametrics.com/victorialogs",
        image="victoriametrics/victoria-logs", tag_suffix="-victorialogs", tag_pages=8, min_major=0,
        container_port=9428, volumes=("data:/victoria-logs-data",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 9428),),
    ),
    App(
        key="jellystat", name="Jellystat", category="monitoring",
        vi="Thống kê lượt xem cho Jellyfin: ai xem gì, xem bao lâu và phim nào được xem nhiều nhất.",
        en="Viewing statistics for Jellyfin: who watched what, for how long, and what is most popular.",
        website="https://github.com/CyferShepard/Jellystat",
        image="cyfershepard/jellystat", tag_pages=6, tag_any_suffix=True,
        side_images=(PG,), version_values={"DB_IMAGE": PG},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 3230),
            password("JWT_SECRET", "Khóa ký phiên", "Session secret"),
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
      POSTGRES_USER: jellystat
      POSTGRES_PASSWORD: ${DB_PASSWORD}
      POSTGRES_IP: db
      POSTGRES_PORT: "5432"
      JWT_SECRET: ${JWT_SECRET}
      TZ: Asia/Ho_Chi_Minh
    ports:
      - "${HTTP_PORT}:3000"
    volumes:
      - backup:/app/backend/backup-data

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      POSTGRES_DB: jfstat
      POSTGRES_USER: jellystat
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - db:/var/lib/postgresql/data

volumes:
  backup:
  db:
""",
    ),
    App(
        key="wizarr", name="Wizarr", category="media",
        vi="Trang mời người thân vào máy chủ phim: gửi liên kết, họ tự tạo tài khoản.",
        en="An invitation page for your media server: send a link and they create their own account.",
        website="https://github.com/wizarrrr/wizarr",
        image="ghcr.io/wizarrrr/wizarr", registry="ghcr", min_major=3,
        container_port=5690, volumes=("data:/data/database",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 5690),),
    ),
    App(
        key="gluetun", name="Gluetun", category="security",
        vi="Container VPN để các container khác đi mạng qua nó — mọi lưu lượng ra ngoài đều được bọc.",
        en="A VPN container other containers route through, wrapping all their outbound traffic.",
        website="https://github.com/qdm12/gluetun",
        image="qmcgaw/gluetun", tag_pages=6, min_major=3,
        volumes=("config:/gluetun",),
        fields=(
            choice("VPN_SERVICE_PROVIDER", "Nhà cung cấp VPN", "VPN provider",
                   (("mullvad", "Mullvad", "Mullvad"),
                    ("protonvpn", "Proton VPN", "Proton VPN"),
                    ("nordvpn", "NordVPN", "NordVPN"),
                    ("custom", "Tự cấu hình", "Custom")),
                   "mullvad"),
            password("WIREGUARD_PRIVATE_KEY", "Khóa riêng WireGuard", "WireGuard private key"),
            text("SERVER_COUNTRIES", "Quốc gia máy chủ", "Server countries", "Singapore"),
        ),
        environment={"VPN_SERVICE_PROVIDER": "${VPN_SERVICE_PROVIDER}",
                     "VPN_TYPE": "wireguard",
                     "WIREGUARD_PRIVATE_KEY": "${WIREGUARD_PRIVATE_KEY}",
                     "SERVER_COUNTRIES": "${SERVER_COUNTRIES}",
                     "TZ": "Asia/Ho_Chi_Minh"},
    ),
    App(
        key="flaresolverr", name="FlareSolverr", category="tool",
        vi="Vượt trang chặn tự động giúp các công cụ tìm nguồn, chạy như một proxy nhỏ.",
        en="Solves anti-bot challenges for indexer tools, running as a small proxy.",
        website="https://github.com/FlareSolverr/FlareSolverr",
        image="ghcr.io/flaresolverr/flaresolverr", registry="ghcr", min_major=3,
        container_port=8191, volumes=(),
        fields=(port("HTTP_PORT", "Cổng", "Port", 8191),),
        environment={"LOG_LEVEL": "info", "TZ": "Asia/Ho_Chi_Minh"},
    ),
    App(
        key="slskd", name="slskd", category="media",
        vi="Ứng dụng mạng chia sẻ nhạc Soulseek qua giao diện web, chạy nền trên máy chủ.",
        en="A web-based client for the Soulseek music-sharing network that runs headless on a server.",
        website="https://github.com/slskd/slskd",
        image="slskd/slskd", tag_pages=6, min_major=0,
        container_port=5030,
        volumes=("config:/app", "${DOWNLOAD_DIR}:/downloads", "${SHARE_DIR}:/shared"),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 5031),
            text("DOWNLOAD_DIR", "Thư mục tải về", "Download folder", "/srv/downloads"),
            text("SHARE_DIR", "Thư mục chia sẻ", "Shared folder", "/srv/music"),
        ),
        environment={"SLSKD_REMOTE_CONFIGURATION": "true"},
    ),
    App(
        key="openspeedtest", name="OpenSpeedTest", category="monitoring",
        vi="Đo tốc độ mạng nội bộ ngay trong trình duyệt, không cần cài gì trên máy khách.",
        en="Measures local network speed in the browser with nothing installed on the client.",
        website="https://openspeedtest.com",
        image="openspeedtest/latest", tag_pages=6, min_major=0,
        container_port=3000, volumes=(),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 3231),),
    ),
    App(
        key="bitmagnet", name="Bitmagnet", category="media",
        vi="Bộ lập chỉ mục torrent tự quản, thu thập metadata thẳng từ mạng DHT.",
        en="A self-hosted torrent indexer that collects metadata straight from the DHT network.",
        website="https://bitmagnet.io",
        image="ghcr.io/bitmagnet-io/bitmagnet", registry="ghcr", min_major=0,
        side_images=(PG,), version_values={"DB_IMAGE": PG},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 3333),
            password("DB_PASSWORD", "Mật khẩu cơ sở dữ liệu", "Database password"),
        ),
        compose="""
services:
  app:
    image: ${IMAGE}
    container_name: ${CONTAINER_NAME}
    restart: unless-stopped
    command: worker run --keys=http_server --keys=queue_server --keys=dht_crawler
    depends_on:
      - db
    environment:
      POSTGRES_HOST: db
      POSTGRES_NAME: bitmagnet
      POSTGRES_USER: bitmagnet
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    ports:
      - "${HTTP_PORT}:3333"

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      POSTGRES_DB: bitmagnet
      POSTGRES_USER: bitmagnet
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - db:/var/lib/postgresql/data

volumes:
  db:
""",
    ),

    App(
        key="anythingllm", name="AnythingLLM", category="automation",
        vi="Trò chuyện với tài liệu của chính bạn: nạp tệp vào không gian làm việc rồi hỏi đáp trên đó.",
        en="Chat with your own documents: load files into a workspace and ask questions about them.",
        website="https://anythingllm.com",
        image="mintplexlabs/anythingllm", fixed_tags=("latest",),
        container_port=3001, volumes=("storage:/app/server/storage",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 3011),),
        environment={"STORAGE_DIR": "/app/server/storage"},
    ),
]
