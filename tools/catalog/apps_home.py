"""Nhóm ứng dụng nhà thông minh, tài chính và tiện ích cá nhân."""

from model import App, choice, password, port, text

PG = "postgres:16-alpine"

APPS = [
    App(
        key="homeassistant", name="Home Assistant", category="automation",
        vi="Trung tâm nhà thông minh, gom đèn, cảm biến và camera của mọi hãng về một chỗ.",
        en="A smart-home hub that gathers lights, sensors and cameras from every brand in one place.",
        website="https://home-assistant.io",
        image="homeassistant/home-assistant", tag_pages=6, min_major=2023,
        container_port=8123, volumes=("config:/config",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 8123),),
        environment={"TZ": "Asia/Ho_Chi_Minh"},
    ),
    App(
        key="nodered", name="Node-RED", category="automation",
        vi="Nối dịch vụ và thiết bị lại với nhau bằng cách kéo dây giữa các khối.",
        en="Wires services and devices together by dragging connections between blocks.",
        website="https://nodered.org",
        image="nodered/node-red", tag_pages=6, min_major=3,
        container_port=1880, volumes=("data:/data",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 1880),),
        environment={"TZ": "Asia/Ho_Chi_Minh"},
    ),
    App(
        key="mosquitto", name="Mosquitto", category="automation",
        vi="Máy chủ MQTT, nơi mọi thiết bị nhà thông minh gửi và nhận tin nhắn.",
        en="An MQTT broker — where smart-home devices publish and receive messages.",
        website="https://mosquitto.org",
        image="eclipse-mosquitto", tag_pages=6, min_major=2,
        container_port=1883,
        volumes=("config:/mosquitto/config", "data:/mosquitto/data"),
        fields=(port("MQTT_PORT", "Cổng MQTT", "MQTT port", 1883),),
    ),
    App(
        key="zigbee2mqtt", name="Zigbee2MQTT", category="automation",
        vi="Đưa thiết bị Zigbee lên MQTT, dùng được với mọi trung tâm nhà thông minh.",
        en="Bridges Zigbee devices onto MQTT so any smart-home hub can use them.",
        website="https://zigbee2mqtt.io",
        image="koenkk/zigbee2mqtt", tag_pages=6, min_major=1,
        container_port=8080, volumes=("data:/app/data",),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8380),
            text("MQTT_SERVER", "Địa chỉ máy chủ MQTT", "MQTT server", "mqtt://localhost:1883"),
        ),
        environment={"ZIGBEE2MQTT_CONFIG_MQTT_SERVER": "${MQTT_SERVER}",
                     "TZ": "Asia/Ho_Chi_Minh"},
    ),
    App(
        key="esphome", name="ESPHome", category="automation",
        vi="Nạp phần mềm cho vi điều khiển ESP bằng tệp cấu hình, không phải viết mã C.",
        en="Flashes ESP microcontrollers from a config file instead of writing C.",
        website="https://esphome.io",
        image="esphome/esphome", tag_pages=6, min_major=2023,
        container_port=6052, volumes=("config:/config",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 6052),),
    ),
    App(
        key="frigate", name="Frigate", category="automation",
        vi="Camera an ninh có nhận diện vật thể ngay trên máy, không gửi hình lên đám mây.",
        en="Security camera recording with on-device object detection — no cloud uploads.",
        website="https://frigate.video",
        image="ghcr.io/blakeblackshear/frigate", registry="ghcr", min_major=0,
        container_port=5000,
        volumes=("config:/config", "${STORAGE_DIR}:/media/frigate"),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 5010),
            text("STORAGE_DIR", "Thư mục lưu video", "Recording folder", "/srv/frigate"),
        ),
        environment={"TZ": "Asia/Ho_Chi_Minh"},
    ),
    App(
        key="firefly-iii", name="Firefly III", category="productivity",
        vi="Quản lý tài chính cá nhân: ngân sách, hóa đơn định kỳ và báo cáo chi tiêu.",
        en="Personal finance management: budgets, recurring bills and spending reports.",
        website="https://firefly-iii.org",
        image="fireflyiii/core", tag_prefix="version-", tag_pages=6, min_major=6,
        side_images=(PG,), version_values={"DB_IMAGE": PG},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8380),
            text("APP_URL", "Địa chỉ trang", "Site URL", "http://localhost:8380"),
            password("SECRET_KEY", "Khóa ứng dụng", "App key",
                     help_vi="Firefly yêu cầu khóa dài đúng 32 ký tự; để trống để panel tự sinh.",
                     help_en="Firefly needs a key of exactly 32 characters; leave it blank to generate one."),
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
      APP_ENV: production
      APP_KEY: ${SECRET_KEY}
      APP_URL: ${APP_URL}
      TZ: Asia/Ho_Chi_Minh
      DB_CONNECTION: pgsql
      DB_HOST: db
      DB_PORT: "5432"
      DB_DATABASE: firefly
      DB_USERNAME: firefly
      DB_PASSWORD: ${DB_PASSWORD}
    ports:
      - "${HTTP_PORT}:8080"
    volumes:
      - upload:/var/www/html/storage/upload

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      POSTGRES_DB: firefly
      POSTGRES_USER: firefly
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - db:/var/lib/postgresql/data

volumes:
  upload:
  db:
""",
    ),
    App(
        key="actual", name="Actual Budget", category="productivity",
        vi="Ghi chép chi tiêu theo phương pháp phong bì, đồng bộ giữa các thiết bị.",
        en="Envelope-method budgeting that syncs across your devices.",
        website="https://actualbudget.org",
        image="actualbudget/actual-server", tag_pages=6, min_major=24,
        container_port=5006, volumes=("data:/data",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 5006),),
    ),
    App(
        key="invoiceninja", name="Invoice Ninja", category="productivity",
        vi="Lập hóa đơn, báo giá và theo dõi thanh toán cho công việc tự do.",
        en="Invoices, quotes and payment tracking for freelance work.",
        website="https://invoiceninja.com",
        image="invoiceninja/invoiceninja", tag_pages=6, min_major=5,
        side_images=("mariadb:11",), version_values={"DB_IMAGE": "mariadb:11"},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8390),
            text("APP_URL", "Địa chỉ trang", "Site URL", "http://localhost:8390"),
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
    environment:
      APP_ENV: production
      APP_URL: ${APP_URL}
      APP_KEY: ${SECRET_KEY}
      DB_HOST: db
      DB_DATABASE: ninja
      DB_USERNAME: ninja
      DB_PASSWORD: ${DB_PASSWORD}
    ports:
      - "${HTTP_PORT}:80"
    volumes:
      - public:/var/www/app/public
      - storage:/var/www/app/storage

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      MARIADB_DATABASE: ninja
      MARIADB_USER: ninja
      MARIADB_PASSWORD: ${DB_PASSWORD}
      MARIADB_RANDOM_ROOT_PASSWORD: "1"
    volumes:
      - db:/var/lib/mysql

volumes:
  public:
  storage:
  db:
""",
    ),
    App(
        key="grocy", name="Grocy", category="productivity",
        vi="Quản lý kho thực phẩm trong nhà: hạn dùng, danh sách đi chợ và công thức nấu ăn.",
        en="Household groceries: expiry dates, shopping lists and recipes.",
        website="https://grocy.info",
        image="linuxserver/grocy", tag_pages=6, min_major=4,
        container_port=80, volumes=("config:/config",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 9283),),
        environment={"PUID": "1000", "PGID": "1000", "TZ": "Asia/Ho_Chi_Minh"},
    ),
    App(
        key="mealie", name="Mealie", category="productivity",
        vi="Lưu công thức nấu ăn, lên thực đơn tuần và tự dựng danh sách đi chợ.",
        en="Saves recipes, plans the week's meals and builds the shopping list for you.",
        website="https://mealie.io",
        image="ghcr.io/mealie-recipes/mealie", registry="ghcr", min_major=1,
        container_port=9000, volumes=("data:/app/data",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 9925),),
        environment={"ALLOW_SIGNUP": "false", "TZ": "Asia/Ho_Chi_Minh",
                     "BASE_URL": "http://localhost"},
    ),
    App(
        key="tandoor", name="Tandoor Recipes", category="productivity",
        vi="Sổ công thức nấu ăn có chia tỉ lệ khẩu phần và lên kế hoạch bữa ăn.",
        en="A recipe book that scales servings and plans meals.",
        website="https://tandoor.dev",
        image="vabene1111/recipes", tag_pages=6, min_major=1,
        side_images=(PG,), version_values={"DB_IMAGE": PG},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8380),
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
      SECRET_KEY: ${SECRET_KEY}
      DB_ENGINE: django.db.backends.postgresql
      POSTGRES_HOST: db
      POSTGRES_DB: recipes
      POSTGRES_USER: recipes
      POSTGRES_PASSWORD: ${DB_PASSWORD}
      TIMEZONE: Asia/Ho_Chi_Minh
    ports:
      - "${HTTP_PORT}:8080"
    volumes:
      - static:/opt/recipes/staticfiles
      - media:/opt/recipes/mediafiles

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      POSTGRES_DB: recipes
      POSTGRES_USER: recipes
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - db:/var/lib/postgresql/data

volumes:
  static:
  media:
  db:
""",
    ),
    App(
        key="wireguard", name="WireGuard", category="security",
        vi="Máy chủ VPN WireGuard bản gốc, cấu hình bằng tệp, nhẹ hơn mọi lựa chọn khác.",
        en="The plain WireGuard VPN server, configured by file — lighter than any alternative.",
        website="https://wireguard.com",
        image="linuxserver/wireguard", tag_pages=6, tag_any_suffix=True,
        container_port=51820,
        volumes=("config:/config", "/lib/modules:/lib/modules:ro"),
        fields=(
            port("VPN_PORT", "Cổng VPN", "VPN port", 51830),
            text("PUBLIC_HOST", "Địa chỉ công khai", "Public host", "vpn.example.com"),
            text("PEERS", "Số thiết bị", "Device count", "5",
                 help_vi="Panel sinh sẵn từng ấy tệp cấu hình và mã QR cho thiết bị.",
                 help_en="This many device configs and QR codes are generated up front."),
        ),
        environment={"PUID": "1000", "PGID": "1000", "TZ": "Asia/Ho_Chi_Minh",
                     "SERVERURL": "${PUBLIC_HOST}", "SERVERPORT": "${VPN_PORT}",
                     "PEERS": "${PEERS}"},
    ),
    App(
        key="dashy", name="Dashy", category="tool",
        vi="Trang chủ tùy biến sâu cho máy chủ, có kiểm tra trạng thái và tìm kiếm nhanh.",
        en="A highly customisable server start page with status checks and quick search.",
        website="https://dashy.to",
        image="lissy93/dashy", tag_pages=6, min_major=2,
        container_port=8080, volumes=("config:/app/user-data",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 4000),),
    ),
    App(
        key="gatus", name="Gatus", category="monitoring",
        vi="Theo dõi dịch vụ theo điều kiện tự đặt và hiện trang trạng thái gọn nhẹ.",
        en="Monitors services against conditions you define and shows a compact status page.",
        website="https://gatus.io",
        image="twinproduction/gatus", tag_pages=6, min_major=5,
        container_port=8080, volumes=("config:/config", "data:/data"),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 8085),),
    ),
    App(
        key="speedtest-tracker", name="Speedtest Tracker", category="monitoring",
        vi="Đo tốc độ mạng theo lịch và vẽ biểu đồ để có bằng chứng khi khiếu nại nhà mạng.",
        en="Runs scheduled speed tests and charts them — evidence for the complaint to your ISP.",
        website="https://speedtest-tracker.dev",
        image="lscr.io/linuxserver/speedtest-tracker", registry="dockerhub",
        fixed_tags=("latest",),
        container_port=80, volumes=("config:/config",),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8086),
            password("SECRET_KEY", "Khóa ứng dụng", "App key"),
        ),
        environment={"PUID": "1000", "PGID": "1000", "TZ": "Asia/Ho_Chi_Minh",
                     "APP_KEY": "${SECRET_KEY}", "DB_CONNECTION": "sqlite"},
    ),
    App(
        key="librespeed", name="LibreSpeed", category="monitoring",
        vi="Trang đo tốc độ mạng của riêng bạn, đo đúng đường truyền tới máy chủ này.",
        en="Your own speed test page, measuring the real path to this server.",
        website="https://librespeed.org",
        image="linuxserver/librespeed", tag_pages=6, tag_any_suffix=True,
        container_port=80, volumes=("config:/config",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 8087),),
        environment={"PUID": "1000", "PGID": "1000", "TZ": "Asia/Ho_Chi_Minh"},
    ),
    App(
        key="searxng", name="SearXNG", category="tool",
        vi="Máy tìm kiếm tổng hợp không theo dõi, gộp kết quả từ nhiều nguồn.",
        en="A privacy-respecting metasearch engine that merges results from many sources.",
        website="https://searxng.org",
        image="searxng/searxng", tag_pages=6, tag_any_suffix=True,
        container_port=8080, volumes=("config:/etc/searxng",),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8088),
            text("BASE_URL", "Địa chỉ trang", "Site URL", "http://localhost:8088/"),
            password("SECRET_KEY", "Khóa bí mật", "Secret key"),
        ),
        environment={"SEARXNG_BASE_URL": "${BASE_URL}", "SEARXNG_SECRET": "${SECRET_KEY}"},
    ),
    App(
        key="shlink", name="Shlink", category="tool",
        vi="Máy rút gọn liên kết của riêng bạn, kèm thống kê lượt bấm theo nguồn.",
        en="Your own link shortener, with per-source click statistics.",
        website="https://shlink.io",
        image="shlinkio/shlink", tag_pages=6, min_major=3,
        container_port=8080, volumes=("data:/etc/shlink/data",),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8089),
            text("DOMAIN", "Tên miền rút gọn", "Short domain", "links.example.com"),
        ),
        environment={"DEFAULT_DOMAIN": "${DOMAIN}", "IS_HTTPS_ENABLED": "false"},
    ),
    App(
        key="privatebin", name="PrivateBin", category="tool",
        vi="Dán và chia sẻ đoạn văn bản được mã hóa ngay trên trình duyệt, máy chủ không đọc được.",
        en="Share text snippets encrypted in the browser — the server cannot read them.",
        website="https://privatebin.info",
        image="privatebin/nginx-fpm-alpine", tag_pages=6, min_major=1,
        container_port=8080, volumes=("data:/srv/data",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 8091),),
    ),
    App(
        key="snapdrop", name="Snapdrop", category="tool",
        vi="Gửi tệp giữa các máy trong cùng mạng LAN bằng trình duyệt, không cần cài gì.",
        en="Sends files between devices on the same network from the browser, with nothing installed.",
        website="https://snapdrop.net",
        image="linuxserver/snapdrop", fixed_tags=("latest",),
        container_port=80, volumes=("config:/config",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 8092),),
        environment={"PUID": "1000", "PGID": "1000", "TZ": "Asia/Ho_Chi_Minh"},
    ),
    App(
        key="cyberchef", name="CyberChef", category="tool",
        vi="Dao đa năng cho dữ liệu: mã hóa, giải mã, phân tích định dạng, tất cả trong trình duyệt.",
        en="The Swiss army knife for data: encode, decode and analyse formats, all in the browser.",
        website="https://gchq.github.io/CyberChef",
        image="mpepping/cyberchef", tag_pages=6, min_major=9,
        container_port=8000, volumes=(),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 8093),),
    ),
    App(
        key="volume-backup", name="Docker Volume Backup", category="storage",
        vi="Sao lưu định kỳ volume của Docker ra tệp nén, gửi được lên S3 hoặc WebDAV.",
        en="Periodically backs up Docker volumes to archives, optionally shipped to S3 or WebDAV.",
        website="https://github.com/offen/docker-volume-backup",
        image="offen/docker-volume-backup", tag_pages=6, min_major=2,
        volumes=("/var/run/docker.sock:/var/run/docker.sock:ro", "${BACKUP_DIR}:/archive"),
        fields=(
            text("BACKUP_DIR", "Thư mục chứa bản sao", "Backup folder", "/srv/backups"),
            choice("SCHEDULE", "Lịch chạy", "Schedule",
                   (("0 0 2 * * *", "Mỗi ngày lúc 2 giờ sáng", "Daily at 2am"),
                    ("0 0 2 * * 0", "Mỗi tuần", "Weekly"),
                    ("0 0 */12 * * *", "Mỗi 12 giờ", "Every 12 hours")),
                   "0 0 2 * * *"),
            text("RETENTION_DAYS", "Số ngày giữ lại", "Days to keep", "14"),
        ),
        environment={"BACKUP_CRON_EXPRESSION": "${SCHEDULE}",
                     "BACKUP_RETENTION_DAYS": "${RETENTION_DAYS}",
                     "BACKUP_FILENAME": "backup-%Y-%m-%dT%H-%M-%S.tar.gz"},
    ),
    App(
        key="filestash", name="Filestash", category="storage",
        vi="Một giao diện tệp chung cho S3, FTP, SFTP, WebDAV và Google Drive.",
        en="One file UI across S3, FTP, SFTP, WebDAV and Google Drive.",
        website="https://filestash.app",
        image="machines/filestash", fixed_tags=("latest",),
        container_port=8334, volumes=("state:/app/data/state",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 8334),),
        environment={"APPLICATION_URL": "", "GDRIVE_CLIENT_ID": "", "GDRIVE_CLIENT_SECRET": ""},
    ),
    App(
        key="jupyter", name="JupyterLab", category="development",
        vi="Sổ tay tính toán cho Python: chạy mã, vẽ biểu đồ và ghi chú trong cùng một trang.",
        en="A computational notebook for Python: run code, plot and take notes on one page.",
        website="https://jupyter.org",
        image="jupyter/base-notebook", fixed_tags=("latest",),
        container_port=8888, volumes=("work:/home/jovyan/work",),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8888),
            password("TOKEN", "Mã truy cập", "Access token"),
        ),
        environment={"JUPYTER_TOKEN": "${TOKEN}"},
    ),
    App(
        key="openwebui", name="Open WebUI", category="development",
        vi="Giao diện trò chuyện cho mô hình ngôn ngữ chạy nội bộ qua Ollama.",
        en="A chat interface for language models running locally through Ollama.",
        website="https://openwebui.com",
        image="ghcr.io/open-webui/open-webui", registry="ghcr", tag_pages=30, min_major=0,
        container_port=8080, volumes=("data:/app/backend/data",),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 3080),
            text("OLLAMA_URL", "Địa chỉ Ollama", "Ollama URL", "http://localhost:11434"),
        ),
        environment={"OLLAMA_BASE_URL": "${OLLAMA_URL}"},
    ),
    App(
        key="ollama", name="Ollama", category="development",
        vi="Chạy mô hình ngôn ngữ ngay trên máy chủ, tải mô hình bằng một lệnh.",
        en="Runs language models on your own server; pull a model with one command.",
        website="https://ollama.com",
        image="ollama/ollama", tag_pages=6, min_major=0,
        container_port=11434, volumes=("models:/root/.ollama",),
        fields=(port("API_PORT", "Cổng API", "API port", 11434,
                     help_vi="Mô hình ngôn ngữ ăn rất nhiều RAM; máy dưới 8 GB chỉ chạy nổi mô hình nhỏ.",
                     help_en="Language models are memory hungry; under 8 GB only small models will run."),),
    ),
    App(
        key="invidious", name="Invidious", category="media",
        vi="Xem YouTube qua giao diện nhẹ, không quảng cáo và không bị theo dõi.",
        en="Watches YouTube through a lightweight front end with no ads and no tracking.",
        website="https://invidious.io",
        image="quay.io/invidious/invidious", fixed_tags=("latest",),
        side_images=(PG,), version_values={"DB_IMAGE": PG},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 3021),
            password("HMAC_KEY", "Khóa ký", "HMAC key"),
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
      INVIDIOUS_CONFIG: |
        db:
          dbname: invidious
          user: invidious
          password: ${DB_PASSWORD}
          host: db
          port: 5432
        check_tables: true
        hmac_key: ${HMAC_KEY}
    ports:
      - "${HTTP_PORT}:3000"

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      POSTGRES_DB: invidious
      POSTGRES_USER: invidious
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - db:/var/lib/postgresql/data

volumes:
  db:
""",
    ),
    App(
        key="baikal", name="Baikal", category="productivity",
        vi="Máy chủ lịch và danh bạ theo chuẩn CalDAV/CardDAV, đồng bộ với iPhone và Android.",
        en="A CalDAV/CardDAV calendar and contacts server that syncs with iPhone and Android.",
        website="https://sabre.io/baikal",
        image="ckulka/baikal", fixed_tags=("nginx",),
        container_port=80, volumes=("config:/var/www/baikal/config", "data:/var/www/baikal/Specific"),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 8095),),
    ),
    App(
        key="radicale", name="Radicale", category="productivity",
        vi="Máy chủ lịch và danh bạ tối giản, chạy được trên máy chủ nhỏ nhất.",
        en="A minimal calendar and contacts server that runs on the smallest of machines.",
        website="https://radicale.org",
        image="tomsquest/docker-radicale", tag_pages=6, min_major=3,
        container_port=5232, volumes=("data:/data",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 5232),),
    ),
]
