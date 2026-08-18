"""Nhóm ứng dụng tải xuống, trang chủ máy chủ và tiện ích để bàn."""

from model import App, password, port, text

PG = "postgres:16-alpine"
MARIADB = "mariadb:11"
LSIO_ENV = {"PUID": "1000", "PGID": "1000", "TZ": "Asia/Ho_Chi_Minh"}

APPS = [
    App(
        key="sabnzbd", name="SABnzbd", category="media",
        vi="Máy tải Usenet có giao diện web, tự giải mã và giải nén sau khi tải xong.",
        en="A Usenet downloader with a web UI that decodes and unpacks after each download.",
        website="https://sabnzbd.org",
        image="linuxserver/sabnzbd", tag_pages=6, tag_any_suffix=True,
        container_port=8080,
        volumes=("config:/config", "${DOWNLOAD_DIR}:/downloads"),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8610),
            text("DOWNLOAD_DIR", "Thư mục tải về", "Download folder", "/srv/downloads"),
        ),
        environment=LSIO_ENV,
    ),
    App(
        key="nzbget", name="NZBGet", category="media",
        vi="Máy tải Usenet cực nhẹ, chạy tốt trên máy chỉ có vài trăm megabyte RAM.",
        en="A very light Usenet downloader that runs on machines with a few hundred megabytes of RAM.",
        website="https://nzbget.com",
        image="linuxserver/nzbget", tag_pages=6, tag_any_suffix=True,
        container_port=6789,
        volumes=("config:/config", "${DOWNLOAD_DIR}:/downloads"),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 6789),
            text("DOWNLOAD_DIR", "Thư mục tải về", "Download folder", "/srv/downloads"),
        ),
        environment=LSIO_ENV,
    ),
    App(
        key="jackett", name="Jackett", category="media",
        vi="Cầu nối tới hàng trăm trang tìm nguồn, đưa chúng về một API chung.",
        en="A bridge to hundreds of tracker sites, exposing them through one common API.",
        website="https://github.com/Jackett/Jackett",
        image="linuxserver/jackett", tag_pages=6, tag_any_suffix=True,
        container_port=9117, volumes=("config:/config",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 9117),),
        environment=LSIO_ENV,
    ),
    App(
        key="sickchill", name="SickChill", category="media",
        vi="Theo dõi phim bộ và tự tải tập mới, hỗ trợ cả Usenet lẫn torrent.",
        en="Follows TV shows and fetches new episodes over both Usenet and torrents.",
        website="https://sickchill.github.io",
        image="linuxserver/sickchill", tag_pages=6, tag_any_suffix=True,
        container_port=8081,
        volumes=("config:/config", "${MEDIA_DIR}:/tv", "${DOWNLOAD_DIR}:/downloads"),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8611),
            text("MEDIA_DIR", "Thư mục phim bộ", "TV folder", "/srv/media/tv"),
            text("DOWNLOAD_DIR", "Thư mục tải về", "Download folder", "/srv/downloads"),
        ),
        environment=LSIO_ENV,
    ),
    App(
        key="photostructure", name="PhotoStructure", category="media",
        vi="Gom ảnh nằm rải rác ở nhiều ổ đĩa về một thư viện, tự loại ảnh trùng.",
        en="Gathers photos scattered across drives into one library and weeds out duplicates.",
        website="https://photostructure.com",
        image="photostructure/server", tag_pages=6, tag_any_suffix=True,
        container_port=1787,
        volumes=("config:/ps/config", "library:/ps/library", "tmp:/ps/tmp",
                 "${PHOTOS_DIR}:/ps/photos:ro"),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 1787),
            text("PHOTOS_DIR", "Thư mục ảnh", "Photos folder", "/srv/photos"),
        ),
        environment={"TZ": "Asia/Ho_Chi_Minh"},
    ),
    App(
        key="cloudbeaver", name="CloudBeaver", category="database",
        vi="Giao diện truy vấn cơ sở dữ liệu qua web, hỗ trợ hầu hết loại có trình điều khiển JDBC.",
        en="A web database client covering most databases with a JDBC driver.",
        website="https://dbeaver.com/cloudbeaver",
        image="dbeaver/cloudbeaver", tag_pages=6, min_major=23,
        container_port=8978, volumes=("workspace:/opt/cloudbeaver/workspace",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 8978),),
    ),
    App(
        key="homer", name="Homer", category="tool",
        vi="Trang chủ tĩnh cho máy chủ, cấu hình bằng một tệp YAML duy nhất.",
        en="A static start page for your server, configured by a single YAML file.",
        website="https://github.com/bastienwirtz/homer",
        image="b4bz/homer", tag_pages=6, tag_any_suffix=True,
        container_port=8080, volumes=("assets:/www/assets",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 8612),),
        environment={"INIT_ASSETS": "1"},
    ),
    App(
        key="flame", name="Flame", category="tool",
        vi="Trang chủ máy chủ sửa được ngay trong giao diện, không phải mở tệp cấu hình.",
        en="A server start page you edit in the UI instead of opening a config file.",
        website="https://github.com/pawelmalak/flame",
        image="pawelmalak/flame", tag_pages=6, tag_any_suffix=True,
        container_port=5005,
        volumes=("data:/app/data", "/var/run/docker.sock:/var/run/docker.sock:ro"),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 5005),
            password("PASSWORD", "Mật khẩu", "Password"),
        ),
        environment={"PASSWORD": "${PASSWORD}"},
    ),
    App(
        key="glance", name="Glance", category="tool",
        vi="Bảng tin gom mọi thứ về một trang: dịch vụ, tin tức, thời tiết, lịch và kho mã.",
        en="A dashboard that gathers everything onto one page: services, news, weather, calendar and repos.",
        website="https://github.com/glanceapp/glance",
        image="glanceapp/glance", tag_pages=6, min_major=0,
        container_port=8080,
        volumes=("config:/app/config", "/var/run/docker.sock:/var/run/docker.sock:ro"),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 8613),),
    ),
    App(
        key="apprise", name="Apprise", category="monitoring",
        vi="Một API để gửi thông báo tới hơn tám mươi dịch vụ: Telegram, Discord, email, SMS.",
        en="One API that pushes notifications to over eighty services: Telegram, Discord, email, SMS.",
        website="https://github.com/caronc/apprise-api",
        image="caronc/apprise", tag_pages=6, tag_any_suffix=True,
        container_port=8000, volumes=("config:/config",),
        fields=(port("HTTP_PORT", "Cổng API", "API port", 8614),),
        environment={"PUID": "1000", "PGID": "1000"},
    ),
    App(
        key="firefox", name="Firefox", category="tool",
        vi="Trình duyệt Firefox chạy trên máy chủ, mở qua trình duyệt khác — tiện khi cần IP của máy chủ.",
        en="Firefox running on the server and opened from another browser — handy when you need the server's IP.",
        website="https://mozilla.org/firefox",
        image="linuxserver/firefox", tag_pages=6, tag_any_suffix=True,
        container_port=3000, volumes=("config:/config",),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 3450),
            password("PASSWORD", "Mật khẩu truy cập", "Access password"),
        ),
        environment={"PUID": "1000", "PGID": "1000", "TZ": "Asia/Ho_Chi_Minh",
                     "CUSTOM_USER": "abc", "PASSWORD": "${PASSWORD}"},
    ),
    App(
        key="flarum", name="Flarum", category="website",
        vi="Diễn đàn hiện đại, nhẹ và đọc tốt trên điện thoại.",
        en="A modern forum that stays light and reads well on a phone.",
        website="https://flarum.org",
        image="mondedie/flarum", tag_pages=6, tag_any_suffix=True,
        side_images=(MARIADB,), version_values={"DB_IMAGE": MARIADB},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8615),
            text("FORUM_URL", "Địa chỉ diễn đàn", "Forum URL", "http://localhost:8615"),
            text("ADMIN_USER", "Tài khoản quản trị", "Admin user", "admin"),
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
      DEBUG: "false"
      FORUM_URL: ${FORUM_URL}
      DB_HOST: db
      DB_NAME: flarum
      DB_USER: flarum
      DB_PASS: ${DB_PASSWORD}
      FLARUM_ADMIN_USER: ${ADMIN_USER}
      FLARUM_ADMIN_MAIL: ${ADMIN_EMAIL}
      FLARUM_ADMIN_PASS: ${ADMIN_PASSWORD}
    ports:
      - "${HTTP_PORT}:8888"
    volumes:
      - assets:/flarum/app/public/assets
      - extensions:/flarum/app/extensions

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      MARIADB_DATABASE: flarum
      MARIADB_USER: flarum
      MARIADB_PASSWORD: ${DB_PASSWORD}
      MARIADB_RANDOM_ROOT_PASSWORD: "1"
    volumes:
      - db:/var/lib/mysql

volumes:
  assets:
  extensions:
  db:
""",
    ),
    App(
        key="akaunting", name="Akaunting", category="productivity",
        vi="Phần mềm kế toán cho doanh nghiệp nhỏ: hóa đơn, chi phí và báo cáo thuế.",
        en="Accounting for small businesses: invoices, expenses and tax reports.",
        website="https://akaunting.com",
        image="akaunting/akaunting", tag_pages=8, min_major=2,
        side_images=(MARIADB,), version_values={"DB_IMAGE": MARIADB},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8616),
            text("APP_URL", "Địa chỉ trang", "Site URL", "http://localhost:8616"),
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
      APP_URL: ${APP_URL}
      DB_HOST: db
      DB_PORT: "3306"
      DB_DATABASE: akaunting
      DB_USERNAME: akaunting
      DB_PASSWORD: ${DB_PASSWORD}
      DB_PREFIX: akq_
      COMPANY_NAME: Sunpanel
      COMPANY_EMAIL: ${ADMIN_EMAIL}
      ADMIN_EMAIL: ${ADMIN_EMAIL}
      ADMIN_PASSWORD: ${ADMIN_PASSWORD}
      LOCALE: vi-VN
    ports:
      - "${HTTP_PORT}:80"
    volumes:
      - data:/var/www/html

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      MARIADB_DATABASE: akaunting
      MARIADB_USER: akaunting
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
        key="maybe", name="Maybe", category="productivity",
        vi="Quản lý tài chính cá nhân: gom tài khoản, theo dõi tài sản và dòng tiền.",
        en="Personal finance: brings accounts together and tracks net worth and cash flow.",
        website="https://maybefinance.com",
        image="ghcr.io/maybe-finance/maybe", registry="ghcr", min_major=0,
        side_images=(PG,), version_values={"DB_IMAGE": PG},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8617),
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
      SELF_HOSTED: "true"
      RAILS_FORCE_SSL: "false"
      RAILS_ASSUME_SSL: "false"
      SECRET_KEY_BASE: ${SECRET_KEY_BASE}
      DB_HOST: db
      POSTGRES_DB: maybe
      POSTGRES_USER: maybe
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    ports:
      - "${HTTP_PORT}:3000"
    volumes:
      - storage:/rails/storage

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      POSTGRES_DB: maybe
      POSTGRES_USER: maybe
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - db:/var/lib/postgresql/data

volumes:
  storage:
  db:
""",
    ),
]
