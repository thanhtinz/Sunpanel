"""Nhóm ứng dụng lập trình, cơ sở dữ liệu và giám sát."""

from model import App, choice, password, port, text

PG = "postgres:16-alpine"

APPS = [
    App(
        key="postgres", name="PostgreSQL", category="database",
        vi="Cơ sở dữ liệu quan hệ mạnh nhất trong nhóm nguồn mở, hợp cho mọi ứng dụng nghiêm túc.",
        en="The most capable open-source relational database, fit for anything serious.",
        website="https://postgresql.org",
        image="postgres", tag_suffix="-alpine", tag_pages=6, min_major=13,
        container_port=5432, volumes=("data:/var/lib/postgresql/data",),
        fields=(
            port("PORT", "Cổng", "Port", 5433,
                 help_vi="Chỉ mở ra Internet khi thật sự cần thiết.",
                 help_en="Only expose this to the Internet when you truly need to."),
            text("DB_NAME", "Tên cơ sở dữ liệu", "Database name", "app"),
            text("DB_USER", "Tên người dùng", "User name", "app"),
            password("DB_PASSWORD", "Mật khẩu", "Password"),
        ),
        environment={"POSTGRES_DB": "${DB_NAME}", "POSTGRES_USER": "${DB_USER}",
                     "POSTGRES_PASSWORD": "${DB_PASSWORD}"},
    ),
    App(
        key="mariadb", name="MariaDB", category="database",
        vi="Cơ sở dữ liệu tương thích MySQL, thứ WordPress và hầu hết mã PHP cần tới.",
        en="A MySQL-compatible database — what WordPress and most PHP code expect.",
        website="https://mariadb.org",
        image="mariadb", tag_pages=6, min_major=10,
        container_port=3306, volumes=("data:/var/lib/mysql",),
        fields=(
            port("PORT", "Cổng", "Port", 3307),
            text("DB_NAME", "Tên cơ sở dữ liệu", "Database name", "app"),
            text("DB_USER", "Tên người dùng", "User name", "app"),
            password("DB_PASSWORD", "Mật khẩu", "Password"),
        ),
        environment={"MARIADB_DATABASE": "${DB_NAME}", "MARIADB_USER": "${DB_USER}",
                     "MARIADB_PASSWORD": "${DB_PASSWORD}",
                     "MARIADB_RANDOM_ROOT_PASSWORD": "1"},
    ),
    App(
        key="mysql", name="MySQL", category="database",
        vi="Cơ sở dữ liệu quan hệ quen thuộc nhất, bản chính thức từ Oracle.",
        en="The most familiar relational database, in its official build.",
        website="https://mysql.com",
        image="mysql", tag_pages=6, min_major=8,
        container_port=3306, volumes=("data:/var/lib/mysql",),
        fields=(
            port("PORT", "Cổng", "Port", 3308),
            text("DB_NAME", "Tên cơ sở dữ liệu", "Database name", "app"),
            text("DB_USER", "Tên người dùng", "User name", "app"),
            password("DB_PASSWORD", "Mật khẩu", "Password"),
        ),
        environment={"MYSQL_DATABASE": "${DB_NAME}", "MYSQL_USER": "${DB_USER}",
                     "MYSQL_PASSWORD": "${DB_PASSWORD}",
                     "MYSQL_RANDOM_ROOT_PASSWORD": "1"},
    ),
    App(
        key="mongo", name="MongoDB", category="database",
        vi="Cơ sở dữ liệu tài liệu, lưu dữ liệu dạng JSON không cần khai báo bảng trước.",
        en="A document database that stores JSON without declaring tables first.",
        website="https://mongodb.com",
        image="mongo", tag_pages=6, min_major=6,
        container_port=27017, volumes=("data:/data/db",),
        fields=(
            port("PORT", "Cổng", "Port", 27017),
            text("DB_USER", "Tài khoản quản trị", "Admin user", "root"),
            password("DB_PASSWORD", "Mật khẩu", "Password"),
        ),
        environment={"MONGO_INITDB_ROOT_USERNAME": "${DB_USER}",
                     "MONGO_INITDB_ROOT_PASSWORD": "${DB_PASSWORD}"},
    ),
    App(
        key="valkey", name="Valkey", category="database",
        vi="Nhánh cộng đồng của Redis, giữ nguyên giao thức và giấy phép nguồn mở.",
        en="The community fork of Redis: same protocol, open-source licence kept.",
        website="https://valkey.io",
        image="valkey/valkey", tag_suffix="-alpine", tag_pages=6,
        container_port=6379,
        command="valkey-server --requirepass ${PASSWORD} --appendonly yes",
        fields=(port("PORT", "Cổng", "Port", 6380), password()),
    ),
    App(
        key="rabbitmq", name="RabbitMQ", category="database",
        vi="Hàng đợi tin nhắn cho hệ thống nhiều dịch vụ, kèm giao diện quản trị.",
        en="A message queue for multi-service systems, with a management UI.",
        website="https://rabbitmq.com",
        image="rabbitmq", tag_suffix="-management-alpine", tag_pages=6, min_major=3,
        container_port=15672,
        extra_ports=('"${AMQP_PORT}:5672"',),
        volumes=("data:/var/lib/rabbitmq",),
        fields=(
            port("HTTP_PORT", "Cổng quản trị", "Management port", 15672),
            port("AMQP_PORT", "Cổng AMQP", "AMQP port", 5672),
            text("USER", "Tài khoản", "User", "admin"),
            password(),
        ),
        environment={"RABBITMQ_DEFAULT_USER": "${USER}", "RABBITMQ_DEFAULT_PASS": "${PASSWORD}"},
    ),
    App(
        key="adminer", name="Adminer", category="database",
        vi="Công cụ quản trị cơ sở dữ liệu gói trong một tệp, hỗ trợ MySQL, PostgreSQL và nhiều loại khác.",
        en="A single-file database admin tool for MySQL, PostgreSQL and many others.",
        website="https://adminer.org",
        image="adminer", tag_pages=6, min_major=4,
        container_port=8080, volumes=(),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 8091),),
    ),
    App(
        key="phpmyadmin", name="phpMyAdmin", category="database",
        vi="Giao diện quản trị MySQL và MariaDB quen thuộc với mọi người làm web.",
        en="The MySQL and MariaDB admin UI every web developer already knows.",
        website="https://phpmyadmin.net",
        image="phpmyadmin", tag_pages=6, min_major=5,
        container_port=80, volumes=(),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8092),
            text("DB_HOST", "Máy chủ cơ sở dữ liệu", "Database host", "127.0.0.1",
                 help_vi="Địa chỉ máy chủ MySQL cần quản trị, ví dụ tên container của nó.",
                 help_en="Address of the MySQL server to manage, for example its container name."),
        ),
        environment={"PMA_HOST": "${DB_HOST}", "UPLOAD_LIMIT": "512M"},
    ),
    App(
        key="pgadmin", name="pgAdmin", category="database",
        vi="Giao diện quản trị PostgreSQL đầy đủ: truy vấn, sơ đồ bảng, sao lưu.",
        en="The full PostgreSQL admin UI: queries, schema diagrams and backups.",
        website="https://pgadmin.org",
        image="dpage/pgadmin4", tag_pages=6,
        container_port=80, volumes=("data:/var/lib/pgadmin",),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8093),
            text("ADMIN_EMAIL", "Email đăng nhập", "Login email", "admin@example.com"),
            password("ADMIN_PASSWORD", "Mật khẩu", "Password"),
        ),
        environment={"PGADMIN_DEFAULT_EMAIL": "${ADMIN_EMAIL}",
                     "PGADMIN_DEFAULT_PASSWORD": "${ADMIN_PASSWORD}",
                     "PGADMIN_LISTEN_PORT": "80"},
    ),
    App(
        key="gitlab", name="GitLab", category="development",
        vi="Nền tảng DevOps trọn gói: kho mã, CI/CD, đăng ký container và quản lý dự án.",
        en="A complete DevOps platform: repositories, CI/CD, container registry and project tracking.",
        website="https://gitlab.com",
        image="gitlab/gitlab-ce", tag_suffix="-ce.0", tag_pages=6, min_major=16,
        container_port=80,
        volumes=("config:/etc/gitlab", "logs:/var/log/gitlab", "data:/var/opt/gitlab"),
        extra_ports=('"${SSH_PORT}:22"',),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8094,
                 help_vi="GitLab cần ít nhất 4 GB RAM; máy nhỏ hơn nên dùng Gitea.",
                 help_en="GitLab needs at least 4 GB of RAM; use Gitea on smaller machines."),
            port("SSH_PORT", "Cổng SSH", "SSH port", 2223),
        ),
        environment={"GITLAB_OMNIBUS_CONFIG": "external_url 'http://localhost'"},
    ),
    App(
        key="drone", name="Drone CI", category="development",
        vi="Máy chạy CI/CD gọn nhẹ, mỗi bước là một container, cấu hình bằng một tệp YAML.",
        en="A lightweight CI/CD server: each step is a container, configured by one YAML file.",
        website="https://drone.io",
        image="drone/drone", tag_pages=6, min_major=2,
        container_port=80, volumes=("data:/data",),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8095),
            text("SERVER_HOST", "Tên miền", "Server host", "localhost:8095"),
            text("GITEA_SERVER", "Địa chỉ Gitea", "Gitea server", "http://localhost:3000"),
            text("CLIENT_ID", "Client ID của ứng dụng OAuth", "OAuth client ID", ""),
            password("CLIENT_SECRET", "Client secret", "Client secret"),
            password("RPC_SECRET", "Khóa cho máy chạy lệnh", "Runner shared secret"),
        ),
        environment={"DRONE_GITEA_SERVER": "${GITEA_SERVER}",
                     "DRONE_GITEA_CLIENT_ID": "${CLIENT_ID}",
                     "DRONE_GITEA_CLIENT_SECRET": "${CLIENT_SECRET}",
                     "DRONE_RPC_SECRET": "${RPC_SECRET}",
                     "DRONE_SERVER_HOST": "${SERVER_HOST}",
                     "DRONE_SERVER_PROTO": "http"},
    ),
    App(
        key="verdaccio", name="Verdaccio", category="development",
        vi="Kho gói npm riêng, vừa làm bộ đệm cho npm công cộng vừa chứa gói nội bộ.",
        en="A private npm registry that also caches the public one.",
        website="https://verdaccio.org",
        image="verdaccio/verdaccio", tag_pages=6, min_major=5,
        container_port=4873, volumes=("storage:/verdaccio/storage", "conf:/verdaccio/conf"),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 4873),),
    ),
    App(
        key="gitea-runner", name="Gitea Actions Runner", category="development",
        vi="Máy chạy lệnh cho Gitea Actions, nhận việc CI từ máy chủ Gitea của bạn.",
        en="The runner for Gitea Actions — picks up CI jobs from your Gitea server.",
        website="https://gitea.com/gitea/act_runner",
        image="gitea/act_runner", tag_pages=6,
        volumes=("/var/run/docker.sock:/var/run/docker.sock", "data:/data"),
        fields=(
            text("GITEA_URL", "Địa chỉ Gitea", "Gitea URL", "http://localhost:3000"),
            password("RUNNER_TOKEN", "Mã đăng ký", "Registration token",
                     help_vi="Lấy trong Gitea ở mục Cài đặt › Actions › Runners.",
                     help_en="Copy it from Gitea under Settings › Actions › Runners."),
        ),
        environment={"GITEA_INSTANCE_URL": "${GITEA_URL}",
                     "GITEA_RUNNER_REGISTRATION_TOKEN": "${RUNNER_TOKEN}"},
    ),
    App(
        key="prometheus", name="Prometheus", category="monitoring",
        vi="Bộ thu số liệu theo chuỗi thời gian, nền tảng của hầu hết hệ giám sát hiện nay.",
        en="The time-series metrics collector most monitoring stacks are built on.",
        website="https://prometheus.io",
        image="prom/prometheus", tag_pages=6, min_major=2,
        container_port=9090, volumes=("data:/prometheus",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 9091),),
    ),
    App(
        key="loki", name="Loki", category="monitoring",
        vi="Kho nhật ký gọn nhẹ, tra cứu ngay trong Grafana bằng cùng một truy vấn.",
        en="A lightweight log store you query straight from Grafana.",
        website="https://grafana.com/oss/loki",
        image="grafana/loki", tag_pages=6, min_major=2,
        container_port=3100, volumes=("data:/loki",),
        fields=(port("HTTP_PORT", "Cổng API", "API port", 3100),),
    ),
    App(
        key="netdata", name="Netdata", category="monitoring",
        vi="Giám sát máy chủ theo giây, hàng nghìn chỉ số dựng sẵn không cần cấu hình.",
        en="Per-second server monitoring with thousands of metrics out of the box.",
        website="https://netdata.cloud",
        image="netdata/netdata", tag_pages=6, min_major=1,
        container_port=19999,
        volumes=("config:/etc/netdata", "lib:/var/lib/netdata", "cache:/var/cache/netdata",
                 "/etc/passwd:/host/etc/passwd:ro", "/etc/group:/host/etc/group:ro",
                 "/proc:/host/proc:ro", "/sys:/host/sys:ro"),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 19999),),
    ),
    App(
        key="dozzle", name="Dozzle", category="monitoring",
        vi="Xem nhật ký container theo thời gian thực trong trình duyệt, không lưu gì xuống đĩa.",
        en="Watch container logs live in the browser without writing anything to disk.",
        website="https://dozzle.dev",
        image="amir20/dozzle", tag_pages=6, min_major=6,
        container_port=8080, volumes=("/var/run/docker.sock:/var/run/docker.sock:ro",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 9999,
                     help_vi="Dozzle đọc được nhật ký mọi container, kể cả phần lộ mật khẩu. Đừng mở ra Internet.",
                     help_en="Dozzle can read every container's logs, passwords included. Do not expose it publicly."),),
    ),
    App(
        key="watchtower", name="Watchtower", category="monitoring",
        vi="Tự theo dõi và cập nhật image của các container đang chạy.",
        en="Watches running containers and updates their images automatically.",
        website="https://containrrr.dev/watchtower",
        image="containrrr/watchtower", tag_pages=6, min_major=1,
        volumes=("/var/run/docker.sock:/var/run/docker.sock",),
        fields=(
            choice("SCHEDULE", "Tần suất kiểm tra", "Check interval",
                   (("0 0 4 * * *", "Mỗi ngày lúc 4 giờ sáng", "Daily at 4am"),
                    ("0 0 4 * * 0", "Mỗi tuần", "Weekly"),
                    ("0 0 */6 * * *", "Mỗi 6 giờ", "Every 6 hours")),
                   "0 0 4 * * *"),
            choice("CLEANUP", "Xóa image cũ sau khi cập nhật", "Remove old images after update",
                   (("true", "Có", "Yes"), ("false", "Không", "No")), "true"),
        ),
        environment={"WATCHTOWER_SCHEDULE": "${SCHEDULE}", "WATCHTOWER_CLEANUP": "${CLEANUP}"},
    ),
    App(
        key="gotify", name="Gotify", category="monitoring",
        vi="Máy chủ thông báo đẩy riêng, nhận cảnh báo từ script bằng một lệnh curl.",
        en="Your own push notification server — send alerts from a script with one curl call.",
        website="https://gotify.net",
        image="gotify/server", tag_pages=6, min_major=2,
        container_port=80, volumes=("data:/app/data",),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8096),
            text("ADMIN_USER", "Tài khoản quản trị", "Admin user", "admin"),
            password("ADMIN_PASSWORD", "Mật khẩu quản trị", "Admin password"),
        ),
        environment={"GOTIFY_DEFAULTUSER_NAME": "${ADMIN_USER}",
                     "GOTIFY_DEFAULTUSER_PASS": "${ADMIN_PASSWORD}"},
    ),
    App(
        key="ntfy", name="ntfy", category="monitoring",
        vi="Gửi thông báo về điện thoại bằng một lệnh curl, không cần đăng ký tài khoản.",
        en="Push notifications to your phone with a single curl call — no account needed.",
        website="https://ntfy.sh",
        image="binwiederhier/ntfy", tag_pages=6, min_major=2,
        container_port=80,
        command='serve',
        volumes=("cache:/var/cache/ntfy", "etc:/etc/ntfy"),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 8097),),
        environment={"NTFY_BASE_URL": "http://localhost", "NTFY_CACHE_FILE": "/var/cache/ntfy/cache.db"},
    ),
    App(
        key="changedetection", name="Change Detection", category="monitoring",
        vi="Theo dõi một trang web và báo khi nội dung đổi — giá, tồn kho, thông báo tuyển dụng.",
        en="Watches a web page and tells you when it changes — prices, stock, job postings.",
        website="https://changedetection.io",
        image="ghcr.io/dgtlmoon/changedetection.io", registry="ghcr", min_major=0,
        container_port=5000, volumes=("data:/datastore",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 5010),),
    ),
    App(
        key="glances", name="Glances", category="monitoring",
        vi="Bảng theo dõi tài nguyên máy chủ gọn trong một trang, xem được cả tiến trình.",
        en="A one-page server resource dashboard that also lists processes.",
        website="https://nicolargo.github.io/glances",
        image="nicolargo/glances", tag_suffix="-full", tag_pages=6, min_major=3,
        container_port=61208, volumes=("/var/run/docker.sock:/var/run/docker.sock:ro",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 61208),),
        environment={"GLANCES_OPT": "-w"},
    ),
    App(
        key="statping-ng", name="Statping-ng", category="monitoring",
        vi="Trang trạng thái công khai cho dịch vụ của bạn, kèm biểu đồ thời gian phản hồi.",
        en="A public status page for your services, with response-time charts.",
        website="https://github.com/statping-ng/statping-ng",
        image="adamboutcher/statping-ng", tag_pages=6, min_major=0,
        container_port=8080, volumes=("data:/app",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 8098),),
    ),
    App(
        key="wikijs", name="Wiki.js", category="development",
        vi="Wiki hiện đại soạn bằng Markdown, phân quyền theo trang và tìm kiếm toàn văn.",
        en="A modern Markdown wiki with per-page permissions and full-text search.",
        website="https://js.wiki",
        image="ghcr.io/requarks/wiki", registry="ghcr", min_major=2,
        side_images=(PG,), version_values={"DB_IMAGE": PG},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 3010),
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
      DB_TYPE: postgres
      DB_HOST: db
      DB_PORT: "5432"
      DB_NAME: wiki
      DB_USER: wiki
      DB_PASS: ${DB_PASSWORD}
    ports:
      - "${HTTP_PORT}:3000"

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      POSTGRES_DB: wiki
      POSTGRES_USER: wiki
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - db:/var/lib/postgresql/data

volumes:
  db:
""",
    ),
    App(
        key="outline", name="Outline", category="productivity",
        vi="Cơ sở tri thức cho đội nhóm, soạn thảo cùng lúc nhiều người như Notion.",
        en="A team knowledge base with real-time collaborative editing, in the Notion vein.",
        website="https://getoutline.com",
        image="outlinewiki/outline", tag_pages=6, min_major=0,
        side_images=(PG, "redis:7-alpine"),
        version_values={"DB_IMAGE": PG, "REDIS_IMAGE": "redis:7-alpine"},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 3011),
            text("APP_URL", "Địa chỉ trang", "Site URL", "http://localhost:3011"),
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
      NODE_ENV: production
      URL: ${APP_URL}
      SECRET_KEY: ${SECRET_KEY}
      UTILS_SECRET: ${SECRET_KEY}
      DATABASE_URL: postgres://outline:${DB_PASSWORD}@db:5432/outline
      REDIS_URL: redis://redis:6379
      PGSSLMODE: disable
    ports:
      - "${HTTP_PORT}:3000"
    volumes:
      - data:/var/lib/outline/data

  redis:
    image: ${REDIS_IMAGE}
    container_name: ${CONTAINER_NAME}-redis
    restart: unless-stopped

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      POSTGRES_DB: outline
      POSTGRES_USER: outline
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - db:/var/lib/postgresql/data

volumes:
  data:
  db:
""",
    ),
    App(
        key="docmost", name="Docmost", category="productivity",
        vi="Wiki và tài liệu nội bộ soạn cùng lúc nhiều người, có bình luận và không gian riêng theo nhóm.",
        en="Collaborative internal docs and wiki with comments and per-team spaces.",
        website="https://docmost.com",
        image="docmost/docmost", tag_pages=6, min_major=0,
        side_images=(PG, "redis:7-alpine"),
        version_values={"DB_IMAGE": PG, "REDIS_IMAGE": "redis:7-alpine"},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 3012),
            text("APP_URL", "Địa chỉ trang", "Site URL", "http://localhost:3012"),
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
      APP_URL: ${APP_URL}
      APP_SECRET: ${SECRET_KEY}
      DATABASE_URL: postgresql://docmost:${DB_PASSWORD}@db:5432/docmost?schema=public
      REDIS_URL: redis://redis:6379
    ports:
      - "${HTTP_PORT}:3000"
    volumes:
      - data:/app/data/storage

  redis:
    image: ${REDIS_IMAGE}
    container_name: ${CONTAINER_NAME}-redis
    restart: unless-stopped

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      POSTGRES_DB: docmost
      POSTGRES_USER: docmost
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - db:/var/lib/postgresql/data

volumes:
  data:
  db:
""",
    ),
]
