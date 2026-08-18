"""Nhóm ứng dụng văn phòng, thư điện tử và giám sát hạ tầng."""

from model import App, password, port, text

PG = "postgres:16-alpine"
MARIADB = "mariadb:11"
REDIS = "redis:7-alpine"

APPS = [
    App(
        key="kimai", name="Kimai", category="productivity",
        vi="Chấm giờ làm theo dự án và khách hàng, xuất báo cáo để lập hóa đơn.",
        en="Tracks time by project and customer, and exports reports for invoicing.",
        website="https://kimai.org",
        image="kimai/kimai2", tag_pages=6, min_major=2,
        side_images=(MARIADB,), version_values={"DB_IMAGE": MARIADB},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8130),
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
    environment:
      ADMINMAIL: ${ADMIN_EMAIL}
      ADMINPASS: ${ADMIN_PASSWORD}
      DATABASE_URL: mysql://kimai:${DB_PASSWORD}@db/kimai?charset=utf8mb4&serverVersion=11.0.0-MariaDB
    ports:
      - "${HTTP_PORT}:8001"
    volumes:
      - data:/opt/kimai/var

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      MARIADB_DATABASE: kimai
      MARIADB_USER: kimai
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
        key="leantime", name="Leantime", category="productivity",
        vi="Quản lý dự án cho nhóm nhỏ, có mục tiêu, ý tưởng và bảng công việc.",
        en="Project management for small teams, with goals, ideas and task boards.",
        website="https://leantime.io",
        image="leantime/leantime", tag_pages=6, min_major=3,
        side_images=(MARIADB,), version_values={"DB_IMAGE": MARIADB},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8131),
            text("SITE_URL", "Địa chỉ trang", "Site URL", "http://localhost:8131"),
            password("SESSION_PASSWORD", "Khóa ký phiên", "Session secret"),
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
      LEAN_APP_URL: ${SITE_URL}
      LEAN_SESSION_PASSWORD: ${SESSION_PASSWORD}
      LEAN_DB_HOST: db
      LEAN_DB_DATABASE: leantime
      LEAN_DB_USER: leantime
      LEAN_DB_PASSWORD: ${DB_PASSWORD}
    ports:
      - "${HTTP_PORT}:80"
    volumes:
      - userfiles:/var/www/html/userfiles
      - public:/var/www/html/public/userfiles

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      MARIADB_DATABASE: leantime
      MARIADB_USER: leantime
      MARIADB_PASSWORD: ${DB_PASSWORD}
      MARIADB_RANDOM_ROOT_PASSWORD: "1"
    volumes:
      - db:/var/lib/mysql

volumes:
  userfiles:
  public:
  db:
""",
    ),
    App(
        key="openproject", name="OpenProject", category="productivity",
        vi="Quản lý dự án đầy đủ: Gantt, sprint, tài liệu và theo dõi thời gian.",
        en="Full project management: Gantt charts, sprints, documents and time tracking.",
        website="https://openproject.org",
        image="openproject/openproject", tag_pages=6, min_major=13,
        container_port=80, volumes=("data:/var/openproject/assets",),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8132,
                 help_vi="OpenProject cần khoảng 4 GB RAM và vài phút cho lần khởi động đầu.",
                 help_en="OpenProject wants around 4 GB of RAM and a few minutes on first boot."),
            password("SECRET_KEY_BASE", "Khóa bí mật", "Secret key"),
        ),
        environment={"SECRET_KEY_BASE": "${SECRET_KEY_BASE}", "OPENPROJECT_HOST__NAME": "localhost"},
    ),
    App(
        key="redmine", name="Redmine", category="productivity",
        vi="Theo dõi công việc và lỗi theo dự án, kèm wiki và lịch — ổn định suốt hai thập kỷ.",
        en="Per-project issue tracking with a wiki and calendar — steady for two decades.",
        website="https://redmine.org",
        image="redmine", tag_pages=6, min_major=5,
        side_images=(PG,), version_values={"DB_IMAGE": PG},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8133),
            password("SECRET_KEY_BASE", "Khóa bí mật", "Secret key"),
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
      REDMINE_DB_POSTGRES: db
      REDMINE_DB_DATABASE: redmine
      REDMINE_DB_USERNAME: redmine
      REDMINE_DB_PASSWORD: ${DB_PASSWORD}
      REDMINE_SECRET_KEY_BASE: ${SECRET_KEY_BASE}
    ports:
      - "${HTTP_PORT}:3000"
    volumes:
      - files:/usr/src/redmine/files

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      POSTGRES_DB: redmine
      POSTGRES_USER: redmine
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - db:/var/lib/postgresql/data

volumes:
  files:
  db:
""",
    ),
    App(
        key="element", name="Element", category="productivity",
        vi="Ứng dụng web cho mạng nhắn tin Matrix, chỉ là phần giao diện nên rất nhẹ.",
        en="The web client for the Matrix chat network — front end only, so it stays light.",
        website="https://element.io",
        image="vectorim/element-web", tag_pages=6, min_major=1,
        container_port=80, volumes=(),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 8134),),
    ),
    App(
        key="listmonk", name="Listmonk", category="website",
        vi="Gửi bản tin và quản lý danh sách người nhận, tự quản hoàn toàn.",
        en="Self-hosted newsletters and subscriber lists.",
        website="https://listmonk.app",
        image="listmonk/listmonk", tag_pages=6, min_major=2,
        side_images=(PG,), version_values={"DB_IMAGE": PG},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8135),
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
      LISTMONK_app__address: 0.0.0.0:9000
      LISTMONK_db__host: db
      LISTMONK_db__user: listmonk
      LISTMONK_db__password: ${DB_PASSWORD}
      LISTMONK_db__database: listmonk
      LISTMONK_ADMIN_USER: ${ADMIN_USER}
      LISTMONK_ADMIN_PASSWORD: ${ADMIN_PASSWORD}
    ports:
      - "${HTTP_PORT}:9000"
    volumes:
      - uploads:/listmonk/uploads

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      POSTGRES_DB: listmonk
      POSTGRES_USER: listmonk
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - db:/var/lib/postgresql/data

volumes:
  uploads:
  db:
""",
    ),
    App(
        key="mailpit", name="Mailpit", category="development",
        vi="Hộp thư giả cho lập trình: hứng mọi email ứng dụng gửi ra và hiện trong trình duyệt.",
        en="A fake mailbox for development: catches every email your app sends and shows it in the browser.",
        website="https://mailpit.axllent.org",
        image="axllent/mailpit", tag_pages=6, min_major=1,
        container_port=8025, volumes=("data:/data",),
        extra_ports=('"${SMTP_PORT}:1025"',),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8136),
            port("SMTP_PORT", "Cổng SMTP", "SMTP port", 1025,
                 help_vi="Trỏ cấu hình gửi thư của ứng dụng vào cổng này để xem thư mà không gửi ra ngoài.",
                 help_en="Point your app's mail settings at this port to inspect mail without sending it."),
        ),
        environment={"MP_MAX_MESSAGES": "5000", "MP_DATABASE": "/data/mailpit.db"},
    ),
    App(
        key="roundcube", name="Roundcube", category="productivity",
        vi="Đọc thư qua trình duyệt cho hộp thư IMAP sẵn có của bạn.",
        en="A browser mail client for your existing IMAP mailbox.",
        website="https://roundcube.net",
        image="roundcube/roundcubemail", tag_suffix="-apache", tag_pages=8, min_major=1,
        container_port=80, volumes=("data:/var/roundcube/db",),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8137),
            text("IMAP_HOST", "Máy chủ IMAP", "IMAP host", "ssl://imap.example.com:993"),
            text("SMTP_HOST", "Máy chủ SMTP", "SMTP host", "tls://smtp.example.com:587"),
        ),
        environment={"ROUNDCUBEMAIL_DEFAULT_HOST": "${IMAP_HOST}",
                     "ROUNDCUBEMAIL_SMTP_SERVER": "${SMTP_HOST}",
                     "ROUNDCUBEMAIL_DB_TYPE": "sqlite"},
    ),
    App(
        key="snappymail", name="SnappyMail", category="productivity",
        vi="Đọc thư qua web, nhẹ và nhanh, chạy tốt trên máy chủ nhỏ.",
        en="A light, fast webmail client that runs well on small servers.",
        website="https://snappymail.eu",
        image="djmaze/snappymail", tag_pages=6, tag_any_suffix=True,
        container_port=8888, volumes=("data:/snappymail/data",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 8138),),
    ),
    App(
        key="healthchecks", name="Healthchecks", category="monitoring",
        vi="Theo dõi tác vụ định kỳ bằng tín hiệu ping: không thấy ping đúng giờ là báo động.",
        en="Watches cron jobs by ping: no ping on schedule means an alert.",
        website="https://healthchecks.io",
        image="healthchecks/healthchecks", tag_pages=6, tag_any_suffix=True,
        container_port=8000, volumes=("data:/data",),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8139),
            text("SITE_ROOT", "Địa chỉ trang", "Site URL", "http://localhost:8139"),
            password("SECRET_KEY", "Khóa bí mật", "Secret key"),
        ),
        environment={"SITE_ROOT": "${SITE_ROOT}", "SECRET_KEY": "${SECRET_KEY}",
                     "DB": "sqlite", "ALLOWED_HOSTS": "*",
                     "SUPERUSER_EMAIL": "admin@example.com"},
    ),
    App(
        key="olivetin", name="OliveTin", category="tool",
        vi="Biến vài lệnh shell thành nút bấm an toàn cho người không dùng terminal.",
        en="Turns a few shell commands into safe buttons for people who never open a terminal.",
        website="https://olivetin.app",
        image="jamesread/olivetin", tag_pages=6, min_major=2020,
        container_port=1337, volumes=("config:/config",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 8140),),
    ),
    App(
        key="dockge", name="Dockge", category="tool",
        vi="Quản lý ngăn xếp compose bằng giao diện, sửa tệp compose ngay trong trình duyệt.",
        en="Manages compose stacks from a UI and edits the compose file in the browser.",
        website="https://github.com/louislam/dockge",
        image="louislam/dockge", tag_pages=6, min_major=1,
        container_port=5001,
        volumes=("/var/run/docker.sock:/var/run/docker.sock", "data:/app/data",
                 "${STACKS_DIR}:/opt/stacks"),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 5051),
            text("STACKS_DIR", "Thư mục chứa ngăn xếp", "Stacks folder", "/opt/stacks"),
        ),
        environment={"DOCKGE_STACKS_DIR": "/opt/stacks"},
    ),
    App(
        key="yacht", name="Yacht", category="tool",
        vi="Giao diện quản lý container nhẹ, tập trung vào cài ứng dụng bằng khuôn mẫu.",
        en="A light container management UI focused on installing apps from templates.",
        website="https://yacht.sh",
        image="selfhostedpro/yacht", fixed_tags=("latest",),
        container_port=8000,
        volumes=("/var/run/docker.sock:/var/run/docker.sock", "config:/config"),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 8141),),
    ),
    App(
        key="diun", name="Diun", category="monitoring",
        vi="Báo khi image của container có bản mới, để bạn tự quyết định lúc nào cập nhật.",
        en="Tells you when a container image has a new release, leaving the timing to you.",
        website="https://crazymax.dev/diun",
        image="crazymax/diun", tag_pages=6, min_major=4,
        volumes=("/var/run/docker.sock:/var/run/docker.sock:ro", "data:/data"),
        fields=(
            text("SCHEDULE", "Lịch kiểm tra", "Check schedule", "0 0 4 * * *"),
        ),
        environment={"DIUN_WATCH_SCHEDULE": "${SCHEDULE}", "TZ": "Asia/Ho_Chi_Minh",
                     "DIUN_PROVIDERS_DOCKER": "true", "LOG_LEVEL": "info"},
    ),
    App(
        key="scrutiny", name="Scrutiny", category="monitoring",
        vi="Theo dõi sức khỏe ổ cứng qua S.M.A.R.T. và báo trước khi ổ sắp hỏng.",
        en="Watches drive health through S.M.A.R.T. and warns before a disk fails.",
        website="https://github.com/AnalogJ/scrutiny",
        image="ghcr.io/analogj/scrutiny", registry="ghcr", fixed_tags=("master-omnibus",),
        container_port=8080,
        volumes=("/run/udev:/run/udev:ro", "config:/opt/scrutiny/config",
                 "influxdb:/opt/scrutiny/influxdb"),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 8142,
                     help_vi="Cần chạy đặc quyền hoặc cấp thiết bị ổ đĩa cho container thì mới đọc được S.M.A.R.T.",
                     help_en="Needs privileged mode or the disk devices passed in to read S.M.A.R.T. data."),),
    ),
    App(
        key="netbox", name="NetBox", category="monitoring",
        vi="Sổ tay hạ tầng mạng: tủ rack, thiết bị, dải IP và cáp nối.",
        en="The source of truth for network infrastructure: racks, devices, IP ranges and cabling.",
        website="https://netbox.dev",
        image="netboxcommunity/netbox", tag_pages=6, tag_any_suffix=True,
        side_images=(PG, REDIS),
        version_values={"DB_IMAGE": PG, "REDIS_IMAGE": REDIS},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8143),
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
      DB_HOST: db
      DB_NAME: netbox
      DB_USER: netbox
      DB_PASSWORD: ${DB_PASSWORD}
      REDIS_HOST: redis
      REDIS_DATABASE: "0"
      REDIS_CACHE_HOST: redis
      REDIS_CACHE_DATABASE: "1"
      SECRET_KEY: ${SECRET_KEY}
      SUPERUSER_NAME: ${ADMIN_USER}
      SUPERUSER_PASSWORD: ${ADMIN_PASSWORD}
      SUPERUSER_EMAIL: admin@example.com
    ports:
      - "${HTTP_PORT}:8080"
    volumes:
      - media:/opt/netbox/netbox/media

  redis:
    image: ${REDIS_IMAGE}
    container_name: ${CONTAINER_NAME}-redis
    restart: unless-stopped

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      POSTGRES_DB: netbox
      POSTGRES_USER: netbox
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - db:/var/lib/postgresql/data

volumes:
  media:
  db:
""",
    ),
    App(
        key="librenms", name="LibreNMS", category="monitoring",
        vi="Giám sát thiết bị mạng qua SNMP: switch, router, tường lửa, kèm biểu đồ lưu lượng.",
        en="SNMP monitoring for network gear — switches, routers and firewalls — with traffic graphs.",
        website="https://librenms.org",
        image="librenms/librenms", tag_pages=6, tag_any_suffix=True,
        side_images=(MARIADB, REDIS),
        version_values={"DB_IMAGE": MARIADB, "REDIS_IMAGE": REDIS},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8144),
            password("SECRET_KEY", "Khóa ứng dụng", "App key"),
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
      DB_HOST: db
      DB_NAME: librenms
      DB_USER: librenms
      DB_PASSWORD: ${DB_PASSWORD}
      APP_KEY: ${SECRET_KEY}
      REDIS_HOST: redis
      TZ: Asia/Ho_Chi_Minh
    ports:
      - "${HTTP_PORT}:8000"
    volumes:
      - data:/data

  redis:
    image: ${REDIS_IMAGE}
    container_name: ${CONTAINER_NAME}-redis
    restart: unless-stopped

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    command: mysqld --innodb-file-per-table=1 --lower-case-table-names=0
    environment:
      MARIADB_DATABASE: librenms
      MARIADB_USER: librenms
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
        key="zabbix", name="Zabbix", category="monitoring",
        vi="Giám sát hạ tầng quy mô doanh nghiệp: máy chủ, mạng, dịch vụ và cảnh báo theo ngưỡng.",
        en="Enterprise-scale infrastructure monitoring: servers, networks, services and threshold alerts.",
        website="https://zabbix.com",
        image="zabbix/zabbix-server-pgsql", tag_suffix="-alpine", tag_pages=8, min_major=6,
        side_images=(PG, "zabbix/zabbix-web-nginx-pgsql:alpine-7.0-latest"),
        version_values={"DB_IMAGE": PG,
                        "WEB_IMAGE": "zabbix/zabbix-web-nginx-pgsql:alpine-7.0-latest"},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8145),
            port("SERVER_PORT", "Cổng máy chủ Zabbix", "Zabbix server port", 10051),
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
      DB_SERVER_HOST: db
      POSTGRES_DB: zabbix
      POSTGRES_USER: zabbix
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    ports:
      - "${SERVER_PORT}:10051"

  web:
    image: ${WEB_IMAGE}
    container_name: ${CONTAINER_NAME}-web
    restart: unless-stopped
    depends_on:
      - app
      - db
    environment:
      ZBX_SERVER_HOST: app
      DB_SERVER_HOST: db
      POSTGRES_DB: zabbix
      POSTGRES_USER: zabbix
      POSTGRES_PASSWORD: ${DB_PASSWORD}
      PHP_TZ: Asia/Ho_Chi_Minh
    ports:
      - "${HTTP_PORT}:8080"

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      POSTGRES_DB: zabbix
      POSTGRES_USER: zabbix
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - db:/var/lib/postgresql/data

volumes:
  db:
""",
    ),
    App(
        key="smokeping", name="SmokePing", category="monitoring",
        vi="Đo độ trễ và mất gói theo thời gian, vẽ ra biểu đồ nhìn là biết mạng chập chờn lúc nào.",
        en="Measures latency and packet loss over time, charting exactly when the link wobbles.",
        website="https://oss.oetiker.ch/smokeping",
        image="linuxserver/smokeping", tag_pages=6, tag_any_suffix=True,
        container_port=80, volumes=("config:/config", "data:/data"),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 8146),),
        environment={"PUID": "1000", "PGID": "1000", "TZ": "Asia/Ho_Chi_Minh"},
    ),
    App(
        key="telegraf", name="Telegraf", category="monitoring",
        vi="Bộ thu số liệu cắm được vào hàng trăm nguồn rồi đẩy sang InfluxDB hay Prometheus.",
        en="A metrics agent that plugs into hundreds of sources and ships to InfluxDB or Prometheus.",
        website="https://influxdata.com/time-series-platform/telegraf",
        image="telegraf", tag_pages=6, min_major=1,
        container_port=8125,
        volumes=("config:/etc/telegraf", "/var/run/docker.sock:/var/run/docker.sock:ro"),
        fields=(port("STATSD_PORT", "Cổng StatsD", "StatsD port", 8125),),
    ),
    App(
        key="cadvisor", name="cAdvisor", category="monitoring",
        vi="Đo mức dùng CPU, RAM và mạng của từng container, xuất số liệu cho Prometheus.",
        en="Measures per-container CPU, memory and network use, and exports it to Prometheus.",
        website="https://github.com/google/cadvisor",
        image="gcr.io/cadvisor/cadvisor", registry="dockerhub",
        fixed_tags=("v0.49.1",),
        container_port=8080,
        volumes=("/:/rootfs:ro", "/var/run:/var/run:ro", "/sys:/sys:ro",
                 "/var/lib/docker/:/var/lib/docker:ro"),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 8147),),
    ),
    App(
        key="alertmanager", name="Alertmanager", category="monitoring",
        vi="Gom cảnh báo từ Prometheus, khử trùng lặp rồi gửi tới đúng người qua đúng kênh.",
        en="Groups alerts from Prometheus, deduplicates them and routes each to the right person.",
        website="https://prometheus.io/docs/alerting/latest/alertmanager",
        image="prom/alertmanager", tag_pages=6, min_major=0,
        container_port=9093, volumes=("data:/alertmanager",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 9093),),
    ),
    App(
        key="jaeger", name="Jaeger", category="monitoring",
        vi="Theo vết một yêu cầu đi qua nhiều dịch vụ để biết đoạn nào chậm.",
        en="Traces a request across services so you can see which hop is slow.",
        website="https://jaegertracing.io",
        image="jaegertracing/all-in-one", tag_pages=6, min_major=1,
        container_port=16686,
        extra_ports=('"${OTLP_PORT}:4318"',),
        volumes=(),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 16686),
            port("OTLP_PORT", "Cổng nhận vết OTLP", "OTLP ingest port", 4318),
        ),
        environment={"COLLECTOR_OTLP_ENABLED": "true"},
    ),
]
