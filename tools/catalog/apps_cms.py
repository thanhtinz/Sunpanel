"""Nhóm ứng dụng quản trị nội dung, thư điện tử và kho hàng."""

from model import App, password, port, text

PG = "postgres:16-alpine"
MARIADB = "mariadb:11"
MYSQL = "mysql:8"
REDIS = "redis:7-alpine"

APPS = [
    App(
        key="joomla", name="Joomla", category="website",
        vi="Nền tảng quản trị nội dung lâu đời, mạnh ở phân quyền và website nhiều ngôn ngữ.",
        en="A long-established CMS, strong on permissions and multilingual sites.",
        website="https://joomla.org",
        image="joomla", tag_suffix="-apache", tag_pages=8, min_major=4,
        side_images=(MYSQL,), version_values={"DB_IMAGE": MYSQL},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8620),
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
      JOOMLA_DB_HOST: db
      JOOMLA_DB_NAME: joomla
      JOOMLA_DB_USER: joomla
      JOOMLA_DB_PASSWORD: ${DB_PASSWORD}
    ports:
      - "${HTTP_PORT}:80"
    volumes:
      - data:/var/www/html

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      MYSQL_DATABASE: joomla
      MYSQL_USER: joomla
      MYSQL_PASSWORD: ${DB_PASSWORD}
      MYSQL_RANDOM_ROOT_PASSWORD: "1"
    volumes:
      - db:/var/lib/mysql

volumes:
  data:
  db:
""",
    ),
    App(
        key="grav", name="Grav", category="website",
        vi="Nền tảng nội dung không cần cơ sở dữ liệu — mỗi trang là một tệp, sao lưu chỉ là chép thư mục.",
        en="A database-free CMS: every page is a file, so backing up means copying a folder.",
        website="https://getgrav.org",
        image="linuxserver/grav", tag_pages=6, tag_any_suffix=True,
        container_port=80, volumes=("config:/config",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 8621),),
        environment={"PUID": "1000", "PGID": "1000", "TZ": "Asia/Ho_Chi_Minh"},
    ),
    App(
        key="gogs", name="Gogs", category="development",
        vi="Máy chủ Git siêu nhẹ, chạy được cả trên máy tính bảng một nhân.",
        en="An extremely light Git server that even runs on a single-core board.",
        website="https://gogs.io",
        image="gogs/gogs", tag_pages=6, min_major=0,
        container_port=3000, volumes=("data:/data",),
        extra_ports=('"${SSH_PORT}:22"',),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 3622),
            port("SSH_PORT", "Cổng SSH", "SSH port", 2224),
        ),
    ),
    App(
        key="gotosocial", name="GoToSocial", category="website",
        vi="Máy chủ mạng xã hội liên kết viết bằng Go, nhẹ hơn Mastodon rất nhiều.",
        en="A federated social server written in Go — far lighter than Mastodon.",
        website="https://gotosocial.org",
        image="superseriousbusiness/gotosocial", tag_pages=6, min_major=0,
        container_port=8080, volumes=("data:/gotosocial/storage",),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8622),
            text("HOST_NAME", "Tên miền", "Host name", "social.example.com",
                 help_vi="Tên miền đi vào định danh của mọi tài khoản; đổi sau là hỏng liên kết.",
                 help_en="The domain becomes part of every account's identity; changing it later breaks federation."),
        ),
        environment={"GTS_HOST": "${HOST_NAME}", "GTS_DB_TYPE": "sqlite",
                     "GTS_DB_ADDRESS": "/gotosocial/storage/sqlite.db",
                     "GTS_LETSENCRYPT_ENABLED": "false"},
    ),
    App(
        key="friendica", name="Friendica", category="website",
        vi="Mạng xã hội liên kết nối được với cả Mastodon, Diaspora và nhiều mạng khác.",
        en="A federated social network that connects to Mastodon, Diaspora and more.",
        website="https://friendi.ca",
        image="friendica", tag_suffix="-apache", tag_pages=6, min_major=2023,
        side_images=(MARIADB,), version_values={"DB_IMAGE": MARIADB},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8623),
            text("SITE_URL", "Địa chỉ trang", "Site URL", "http://localhost:8623"),
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
      MYSQL_HOST: db
      MYSQL_DATABASE: friendica
      MYSQL_USER: friendica
      MYSQL_PASSWORD: ${DB_PASSWORD}
      FRIENDICA_URL: ${SITE_URL}
    ports:
      - "${HTTP_PORT}:80"
    volumes:
      - data:/var/www/html

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      MARIADB_DATABASE: friendica
      MARIADB_USER: friendica
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
        key="kanboard", name="Kanboard", category="productivity",
        vi="Bảng Kanban tối giản, nhanh và chạy tốt trên máy chủ nhỏ nhất.",
        en="A minimalist Kanban board that stays fast on the smallest of servers.",
        website="https://kanboard.org",
        image="kanboard/kanboard", tag_pages=6, min_major=1,
        container_port=80,
        volumes=("data:/var/www/app/data", "plugins:/var/www/app/plugins"),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 8624),),
    ),
    App(
        key="wekan", name="Wekan", category="productivity",
        vi="Bảng Kanban nguồn mở kiểu Trello, có phân quyền theo bảng và bình luận.",
        en="An open-source Trello-style Kanban board with per-board permissions and comments.",
        website="https://wekan.github.io",
        image="wekanteam/wekan", tag_pages=6, min_major=6,
        side_images=("mongo:6",), version_values={"DB_IMAGE": "mongo:6"},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8625),
            text("ROOT_URL", "Địa chỉ trang", "Site URL", "http://localhost:8625"),
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
      MONGO_URL: mongodb://db:27017/wekan
      ROOT_URL: ${ROOT_URL}
      WITH_API: "true"
      WRITABLE_PATH: /data
    ports:
      - "${HTTP_PORT}:8080"
    volumes:
      - data:/data

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    command: mongod --logpath /dev/null --oplogSize 128 --quiet
    volumes:
      - db:/data/db

volumes:
  data:
  db:
""",
    ),
    App(
        key="glitchtip", name="GlitchTip", category="monitoring",
        vi="Thu thập lỗi từ ứng dụng của bạn, tương thích Sentry mà nhẹ hơn nhiều.",
        en="Collects errors from your apps — Sentry-compatible but far lighter.",
        website="https://glitchtip.com",
        image="glitchtip/glitchtip", tag_pages=6, min_major=3,
        side_images=(PG, REDIS), version_values={"DB_IMAGE": PG, "REDIS_IMAGE": REDIS},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8626),
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
      DATABASE_URL: postgres://glitchtip:${DB_PASSWORD}@db:5432/glitchtip
      SECRET_KEY: ${SECRET_KEY}
      REDIS_URL: redis://redis:6379/0
      PORT: "8000"
      ENABLE_OPEN_USER_REGISTRATION: "false"
    ports:
      - "${HTTP_PORT}:8000"
    volumes:
      - uploads:/code/uploads

  worker:
    image: ${IMAGE}
    container_name: ${CONTAINER_NAME}-worker
    restart: unless-stopped
    command: ./bin/run-celery-with-beat.sh
    depends_on:
      - db
      - redis
    environment:
      DATABASE_URL: postgres://glitchtip:${DB_PASSWORD}@db:5432/glitchtip
      SECRET_KEY: ${SECRET_KEY}
      REDIS_URL: redis://redis:6379/0
    volumes:
      - uploads:/code/uploads

  redis:
    image: ${REDIS_IMAGE}
    container_name: ${CONTAINER_NAME}-redis
    restart: unless-stopped

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      POSTGRES_DB: glitchtip
      POSTGRES_USER: glitchtip
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - db:/var/lib/postgresql/data

volumes:
  uploads:
  db:
""",
    ),
    App(
        key="docker-mailserver", name="Docker Mailserver", category="productivity",
        vi="Máy chủ thư đầy đủ trong một container: SMTP, IMAP, lọc rác và chống virus.",
        en="A complete mail server in one container: SMTP, IMAP, spam filtering and antivirus.",
        website="https://docker-mailserver.github.io",
        image="mailserver/docker-mailserver", tag_pages=6, min_major=13,
        container_port=25,
        volumes=("maildata:/var/mail", "mailstate:/var/mail-state", "config:/tmp/docker-mailserver"),
        extra_ports=('"${IMAP_PORT}:993"', '"${SUBMISSION_PORT}:587"'),
        fields=(
            port("SMTP_PORT", "Cổng SMTP", "SMTP port", 25,
                 help_vi="Nhiều nhà cung cấp VPS chặn sẵn cổng 25; hỏi họ mở trước khi cài.",
                 help_en="Many VPS providers block port 25 by default; ask them to open it before installing."),
            port("SUBMISSION_PORT", "Cổng gửi thư", "Submission port", 587),
            port("IMAP_PORT", "Cổng IMAP", "IMAP port", 993),
            text("MAIL_DOMAIN", "Tên miền thư", "Mail domain", "mail.example.com"),
        ),
        environment={"OVERRIDE_HOSTNAME": "${MAIL_DOMAIN}", "ENABLE_SPAMASSASSIN": "1",
                     "ENABLE_CLAMAV": "0", "ENABLE_FAIL2BAN": "0", "SSL_TYPE": ""},
    ),
    App(
        key="stalwart", name="Stalwart Mail", category="productivity",
        vi="Máy chủ thư viết bằng Rust, gộp SMTP, IMAP và JMAP với giao diện quản trị sẵn.",
        en="A Rust mail server combining SMTP, IMAP and JMAP with a built-in admin UI.",
        website="https://stalw.art",
        image="stalwartlabs/mail-server", tag_pages=6, min_major=0,
        container_port=8080,
        volumes=("data:/opt/stalwart-mail",),
        extra_ports=('"${SMTP_PORT}:25"', '"${IMAP_PORT}:993"'),
        fields=(
            port("HTTP_PORT", "Cổng quản trị", "Admin port", 8627),
            port("SMTP_PORT", "Cổng SMTP", "SMTP port", 2525),
            port("IMAP_PORT", "Cổng IMAP", "IMAP port", 9930),
        ),
    ),
    App(
        key="poste", name="Poste.io", category="productivity",
        vi="Máy chủ thư trọn gói có giao diện quản trị và webmail, cài xong là gửi nhận được.",
        en="An all-in-one mail server with an admin UI and webmail that works right after install.",
        website="https://poste.io",
        image="analogic/poste.io", tag_pages=6, min_major=2,
        container_port=80,
        volumes=("data:/data",),
        extra_ports=('"${SMTP_PORT}:25"', '"${IMAP_PORT}:993"'),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8628),
            port("SMTP_PORT", "Cổng SMTP", "SMTP port", 2526),
            port("IMAP_PORT", "Cổng IMAP", "IMAP port", 9931),
            text("MAIL_HOSTNAME", "Tên miền thư", "Mail hostname", "mail.example.com"),
        ),
        environment={"HTTPS": "OFF", "h_name": "${MAIL_HOSTNAME}", "TZ": "Asia/Ho_Chi_Minh"},
    ),
    App(
        key="homebox", name="Homebox", category="productivity",
        vi="Sổ kiểm kê đồ đạc trong nhà: món gì để đâu, mua bao giờ, còn bảo hành không.",
        en="A home inventory: what you own, where it lives, when you bought it, warranty status.",
        website="https://homebox.software",
        image="ghcr.io/sysadminsmedia/homebox", registry="ghcr", min_major=0,
        container_port=7745, volumes=("data:/data",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 7745),),
        environment={"HBOX_LOG_LEVEL": "info", "HBOX_OPTIONS_ALLOW_REGISTRATION": "false"},
    ),
    App(
        key="inventree", name="InvenTree", category="productivity",
        vi="Quản lý kho linh kiện cho xưởng và phòng lab: tồn kho, nhà cung cấp và định mức vật tư.",
        en="Parts inventory for workshops and labs: stock levels, suppliers and bills of materials.",
        website="https://inventree.org",
        image="inventree/inventree", tag_pages=6, min_major=0,
        side_images=(PG,), version_values={"DB_IMAGE": PG},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8629),
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
      INVENTREE_DB_ENGINE: postgresql
      INVENTREE_DB_NAME: inventree
      INVENTREE_DB_HOST: db
      INVENTREE_DB_PORT: "5432"
      INVENTREE_DB_USER: inventree
      INVENTREE_DB_PASSWORD: ${DB_PASSWORD}
      INVENTREE_SECRET_KEY: ${SECRET_KEY}
      INVENTREE_AUTO_UPDATE: "true"
    ports:
      - "${HTTP_PORT}:8000"
    volumes:
      - data:/home/inventree/data

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      POSTGRES_DB: inventree
      POSTGRES_USER: inventree
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - db:/var/lib/postgresql/data

volumes:
  data:
  db:
""",
    ),
    App(
        key="sourcegraph", name="Sourcegraph", category="development",
        vi="Tìm kiếm mã nguồn trên toàn bộ kho của bạn, hiểu cả định nghĩa và nơi gọi hàm.",
        en="Code search across all your repositories that understands definitions and call sites.",
        website="https://sourcegraph.com",
        image="sourcegraph/server", tag_pages=6, min_major=5,
        container_port=7080,
        volumes=("etc:/etc/sourcegraph", "data:/var/opt/sourcegraph"),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 7080,
                     help_vi="Sourcegraph cần ít nhất 4 GB RAM và vài phút cho lần khởi động đầu.",
                     help_en="Sourcegraph needs at least 4 GB of RAM and a few minutes on first boot."),),
    ),
    App(
        key="ntopng", name="ntopng", category="monitoring",
        vi="Phân tích lưu lượng mạng theo thời gian thực: máy nào đang chiếm băng thông và nói chuyện với đâu.",
        en="Real-time network traffic analysis: which host is eating bandwidth and who it talks to.",
        website="https://ntop.org",
        image="ntop/ntopng", fixed_tags=("latest",),
        container_port=3000, volumes=("data:/var/lib/ntopng",),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 3630,
                 help_vi="Ở mạng riêng của container, ntopng chỉ thấy lưu lượng của chính nó. Muốn thấy toàn bộ mạng máy chủ thì sửa tệp compose sang network_mode: host.",
                 help_en="On the container network ntopng only sees its own traffic. To watch the whole host network, edit the compose file to network_mode: host."),
        ),
    ),
]
