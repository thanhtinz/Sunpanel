"""Nhóm ứng dụng thiết bị: in 3D, camera, mạng và máy chủ trò chơi."""

from model import App, choice, password, port, text

MONGO = "mongo:7"

APPS = [
    App(
        key="octoprint", name="OctoPrint", category="automation",
        vi="Điều khiển máy in 3D qua web: tải tệp in, theo dõi tiến độ và xem camera.",
        en="Drives a 3D printer over the web: upload prints, watch progress and check the camera.",
        website="https://octoprint.org",
        image="octoprint/octoprint", tag_pages=6, min_major=1,
        container_port=80, volumes=("data:/octoprint",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 8640,
                     help_vi="Muốn nối tới máy in qua USB thì phải cấp thiết bị cho container; xem tài liệu dự án.",
                     help_en="To reach a printer over USB the device must be passed into the container; see the project docs."),),
    ),
    App(
        key="mainsail", name="Mainsail", category="automation",
        vi="Giao diện web cho máy in 3D chạy Klipper, hiện nhiệt độ và đường in theo thời gian thực.",
        en="A web UI for Klipper-based 3D printers showing live temperatures and toolpaths.",
        website="https://docs.mainsail.xyz",
        image="ghcr.io/mainsail-crew/mainsail", registry="ghcr", min_major=2,
        container_port=80, volumes=(),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8641),
            text("MOONRAKER_URL", "Địa chỉ Moonraker", "Moonraker URL", "http://localhost:7125",
                 help_vi="Mainsail chỉ là phần giao diện; nó cần một Moonraker đang chạy cạnh máy in.",
                 help_en="Mainsail is only the front end; it needs a Moonraker running next to the printer."),
        ),
    ),
    App(
        key="fluidd", name="Fluidd", category="automation",
        vi="Giao diện web khác cho Klipper, gọn và hợp với màn hình cảm ứng nhỏ.",
        en="Another web UI for Klipper — compact and good on small touchscreens.",
        website="https://docs.fluidd.xyz",
        image="ghcr.io/fluidd-core/fluidd", registry="ghcr", min_major=1,
        container_port=80, volumes=(),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8642),
            text("MOONRAKER_URL", "Địa chỉ Moonraker", "Moonraker URL", "http://localhost:7125"),
        ),
    ),
    App(
        key="scrypted", name="Scrypted", category="automation",
        vi="Trung tâm camera nối được với HomeKit, Google Home và Alexa, chuyển mã ngay trên máy.",
        en="A camera hub that bridges to HomeKit, Google Home and Alexa, transcoding on the box.",
        website="https://scrypted.app",
        image="koush/scrypted", tag_suffix="-noble-full", tag_pages=8, min_major=0,
        container_port=11080, volumes=("data:/server/volume",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 11080),),
    ),
    App(
        key="tvheadend", name="Tvheadend", category="media",
        vi="Máy chủ truyền hình: nhận tín hiệu từ đầu thu, ghi hình theo lịch và phát cho thiết bị khác.",
        en="A TV streaming server: takes tuner input, records on a schedule and streams to other devices.",
        website="https://tvheadend.org",
        image="linuxserver/tvheadend", fixed_tags=("latest",),
        container_port=9981,
        volumes=("config:/config", "${RECORDING_DIR}:/recordings"),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 9981),
            text("RECORDING_DIR", "Thư mục ghi hình", "Recording folder", "/srv/recordings"),
        ),
        environment={"PUID": "1000", "PGID": "1000", "TZ": "Asia/Ho_Chi_Minh"},
    ),
    App(
        key="threadfin", name="Threadfin", category="media",
        vi="Gom nhiều nguồn truyền hình IPTV thành một danh sách kênh sạch cho Plex hay Jellyfin.",
        en="Merges several IPTV sources into one clean channel list for Plex or Jellyfin.",
        website="https://github.com/Threadfin/Threadfin",
        image="fyb3roptik/threadfin", tag_pages=6, min_major=1,
        container_port=34400, volumes=("conf:/home/threadfin/conf",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 34400),),
        environment={"PUID": "1000", "PGID": "1000", "TZ": "Asia/Ho_Chi_Minh"},
    ),
    App(
        key="openhab", name="openHAB", category="automation",
        vi="Trung tâm nhà thông minh hỗ trợ hơn hai nghìn loại thiết bị, chạy được hoàn toàn ngoại tuyến.",
        en="A smart-home hub supporting over two thousand device types, fully offline capable.",
        website="https://openhab.org",
        image="openhab/openhab", tag_suffix="-alpine", tag_pages=8, min_major=3,
        container_port=8080,
        volumes=("conf:/openhab/conf", "userdata:/openhab/userdata", "addons:/openhab/addons"),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 8643),),
        environment={"OPENHAB_HTTP_PORT": "8080", "EXTRA_JAVA_OPTS": "-Duser.timezone=Asia/Ho_Chi_Minh"},
    ),
    App(
        key="domoticz", name="Domoticz", category="automation",
        vi="Trung tâm nhà thông minh gọn nhẹ, mạnh ở cảm biến và thiết bị điện dùng sóng 433 MHz.",
        en="A light smart-home hub, strong on sensors and 433 MHz electrical devices.",
        website="https://domoticz.com",
        image="domoticz/domoticz", tag_pages=6, tag_any_suffix=True,
        container_port=8080, volumes=("config:/opt/domoticz/userdata",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 8644),),
        environment={"TZ": "Asia/Ho_Chi_Minh"},
    ),
    App(
        key="unifi", name="UniFi Network", category="tool",
        vi="Bộ điều khiển thiết bị mạng UniFi: cấu hình WiFi, VLAN và xem khách đang kết nối.",
        en="The UniFi network controller: configure WiFi and VLANs and see who is connected.",
        website="https://ui.com",
        image="linuxserver/unifi-network-application", tag_pages=6, tag_any_suffix=True,
        side_images=(MONGO,), version_values={"DB_IMAGE": MONGO},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8645),
            port("INFORM_PORT", "Cổng thiết bị báo về", "Device inform port", 8080,
                 help_vi="Thiết bị UniFi kết nối về cổng này; đổi thì phải khai lại trên thiết bị.",
                 help_en="UniFi devices call home on this port; changing it means reconfiguring them."),
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
      PUID: "1000"
      PGID: "1000"
      TZ: Asia/Ho_Chi_Minh
      MONGO_HOST: db
      MONGO_PORT: "27017"
      MONGO_DBNAME: unifi
      MONGO_USER: unifi
      MONGO_PASS: ${DB_PASSWORD}
      MONGO_AUTHSOURCE: admin
    ports:
      - "${HTTP_PORT}:8443"
      - "${INFORM_PORT}:8080"
    volumes:
      - config:/config

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      MONGO_INITDB_ROOT_USERNAME: unifi
      MONGO_INITDB_ROOT_PASSWORD: ${DB_PASSWORD}
    volumes:
      - db:/data/db

volumes:
  config:
  db:
""",
    ),
    App(
        key="omada", name="Omada Controller", category="tool",
        vi="Bộ điều khiển thiết bị mạng TP-Link Omada: quản lý điểm phát WiFi và switch.",
        en="The TP-Link Omada controller: manages access points and switches.",
        website="https://tp-link.com/omada",
        image="mbentley/omada-controller", tag_pages=8, min_major=5,
        container_port=8043,
        volumes=("data:/opt/tplink/EAPController/data", "logs:/opt/tplink/EAPController/logs"),
        extra_ports=('"${INFORM_PORT}:29814"',),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8646),
            port("INFORM_PORT", "Cổng thiết bị báo về", "Device inform port", 29814),
        ),
        environment={"MANAGE_HTTPS_PORT": "8043", "PORTAL_HTTP_PORT": "8088",
                     "SHOW_SERVER_LOGS": "true", "TZ": "Asia/Ho_Chi_Minh"},
    ),
    App(
        key="minecraft", name="Minecraft Server", category="tool",
        vi="Máy chủ Minecraft cho bạn bè cùng chơi, chọn được phiên bản và loại máy chủ.",
        en="A Minecraft server for you and friends, with a choice of version and server type.",
        website="https://github.com/itzg/docker-minecraft-server",
        image="itzg/minecraft-server", fixed_tags=("java21", "java17", "java8"),
        container_port=25565, volumes=("data:/data",),
        fields=(
            port("GAME_PORT", "Cổng trò chơi", "Game port", 25565),
            choice("SERVER_TYPE", "Loại máy chủ", "Server type",
                   (("VANILLA", "Gốc", "Vanilla"),
                    ("PAPER", "Paper (nhanh, có plugin)", "Paper (fast, plugins)"),
                    ("FABRIC", "Fabric (mod)", "Fabric (mods)"),
                    ("FORGE", "Forge (mod)", "Forge (mods)")),
                   "PAPER"),
            text("MEMORY", "Bộ nhớ cấp cho máy chủ", "Server memory", "2G"),
            text("GAME_VERSION", "Phiên bản trò chơi", "Game version", "LATEST"),
        ),
        environment={"EULA": "TRUE", "TYPE": "${SERVER_TYPE}", "MEMORY": "${MEMORY}",
                     "VERSION": "${GAME_VERSION}", "TZ": "Asia/Ho_Chi_Minh"},
    ),
    App(
        key="terraria", name="Terraria Server", category="tool",
        vi="Máy chủ Terraria cho nhóm bạn, giữ thế giới chung để ai vào lúc nào cũng được.",
        en="A Terraria server for a group of friends, keeping one shared world always available.",
        website="https://terraria.org",
        image="ryshe/terraria", tag_prefix="vanilla-", tag_pages=6, min_major=1,
        container_port=7777, volumes=("world:/root/.local/share/Terraria/Worlds",),
        fields=(
            port("GAME_PORT", "Cổng trò chơi", "Game port", 7777),
            text("WORLD_NAME", "Tên thế giới", "World name", "world"),
            password("SERVER_PASSWORD", "Mật khẩu vào máy chủ", "Server password"),
        ),
        environment={"WORLD_FILENAME": "${WORLD_NAME}.wld", "TERRARIA_PASS": "${SERVER_PASSWORD}"},
    ),
    App(
        key="satisfactory", name="Satisfactory Server", category="tool",
        vi="Máy chủ Satisfactory chạy nền, giữ nhà máy tiếp tục hoạt động khi bạn thoát game.",
        en="A dedicated Satisfactory server that keeps the factory running after you log off.",
        website="https://github.com/wolveix/satisfactory-server",
        image="wolveix/satisfactory-server", tag_pages=6, tag_any_suffix=True,
        container_port=7777, volumes=("config:/config",),
        fields=(port("GAME_PORT", "Cổng trò chơi", "Game port", 7778,
                     help_vi="Satisfactory cần ít nhất 8 GB RAM và bốn nhân để chạy mượt.",
                     help_en="Satisfactory wants at least 8 GB of RAM and four cores to run smoothly."),),
        environment={"MAXPLAYERS": "4", "PGID": "1000", "PUID": "1000", "STEAMBETA": "false"},
    ),
    App(
        key="pterodactyl", name="Pterodactyl", category="tool",
        vi="Bảng điều khiển máy chủ trò chơi: tạo, cấp phát và giao quyền quản lý cho từng người.",
        en="A game server control panel: create instances, allocate resources and hand out access.",
        website="https://pterodactyl.io",
        image="ghcr.io/pterodactyl/panel", registry="ghcr", min_major=1,
        side_images=("mariadb:11", "redis:7-alpine"),
        version_values={"DB_IMAGE": "mariadb:11", "REDIS_IMAGE": "redis:7-alpine"},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8647),
            text("APP_URL_VALUE", "Địa chỉ trang", "Site URL", "http://localhost:8647"),
            password("SECRET_KEY", "Khóa ứng dụng", "App key",
                     help_vi="Laravel yêu cầu khóa dạng base64:… dài 32 byte; xem tài liệu dự án để sinh.",
                     help_en="Laravel wants a base64:… key of 32 bytes; see the project docs to generate one."),
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
      APP_URL: ${APP_URL_VALUE}
      APP_KEY: ${SECRET_KEY}
      APP_TIMEZONE: Asia/Ho_Chi_Minh
      DB_HOST: db
      DB_PORT: "3306"
      DB_DATABASE: panel
      DB_USERNAME: pterodactyl
      DB_PASSWORD: ${DB_PASSWORD}
      CACHE_DRIVER: redis
      SESSION_DRIVER: redis
      QUEUE_DRIVER: redis
      REDIS_HOST: redis
    ports:
      - "${HTTP_PORT}:80"
    volumes:
      - var:/app/var
      - logs:/app/storage/logs

  redis:
    image: ${REDIS_IMAGE}
    container_name: ${CONTAINER_NAME}-redis
    restart: unless-stopped

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      MARIADB_DATABASE: panel
      MARIADB_USER: pterodactyl
      MARIADB_PASSWORD: ${DB_PASSWORD}
      MARIADB_RANDOM_ROOT_PASSWORD: "1"
    volumes:
      - db:/var/lib/mysql

volumes:
  var:
  logs:
  db:
""",
    ),
]
