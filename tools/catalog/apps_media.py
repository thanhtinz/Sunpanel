"""Nhóm ứng dụng đa phương tiện, đọc sách và tải xuống."""

from model import App, password, port, text

MEDIA_HELP_VI = "Thư mục sẵn có trên máy chủ, được gắn vào ứng dụng."
MEDIA_HELP_EN = "An existing folder on the host, mounted into the app."

APPS = [
    App(
        key="plex", name="Plex", category="media",
        vi="Máy chủ phim nhạc có ứng dụng trên gần như mọi TV và điện thoại.",
        en="A media server with apps on nearly every TV and phone.",
        website="https://plex.tv",
        image="plexinc/pms-docker", tag_pages=6, tag_any_suffix=True,
        container_port=32400,
        volumes=("config:/config", "${MEDIA_DIR}:/media"),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 32400),
            text("MEDIA_DIR", "Thư mục phim nhạc", "Media folder", "/srv/media",
                 help_vi=MEDIA_HELP_VI, help_en=MEDIA_HELP_EN),
            text("CLAIM_TOKEN", "Mã liên kết tài khoản", "Claim token", "", required=False,
                 help_vi="Lấy ở plex.tv/claim, dùng trong 4 phút để gắn máy chủ vào tài khoản của bạn.",
                 help_en="Get one at plex.tv/claim; it is valid for 4 minutes and links the server to your account."),
        ),
        environment={"PLEX_CLAIM": "${CLAIM_TOKEN}", "TZ": "Asia/Ho_Chi_Minh"},
    ),
    App(
        key="emby", name="Emby", category="media",
        vi="Máy chủ phim nhạc với bộ chuyển mã mạnh và ứng dụng đa nền tảng.",
        en="A media server with strong transcoding and apps on every platform.",
        website="https://emby.media",
        image="emby/embyserver", tag_pages=6, min_major=4,
        container_port=8096,
        volumes=("config:/config", "${MEDIA_DIR}:/mnt/media"),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8097),
            text("MEDIA_DIR", "Thư mục phim nhạc", "Media folder", "/srv/media",
                 help_vi=MEDIA_HELP_VI, help_en=MEDIA_HELP_EN),
        ),
    ),
    App(
        key="navidrome", name="Navidrome", category="media",
        vi="Máy chủ nhạc cá nhân, phát qua trình duyệt và mọi ứng dụng Subsonic.",
        en="A personal music server that streams to browsers and any Subsonic app.",
        website="https://navidrome.org",
        image="deluan/navidrome", tag_pages=6,
        container_port=4533,
        volumes=("data:/data", "${MUSIC_DIR}:/music:ro"),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 4533),
            text("MUSIC_DIR", "Thư mục nhạc", "Music folder", "/srv/music",
                 help_vi=MEDIA_HELP_VI, help_en=MEDIA_HELP_EN),
        ),
    ),
    App(
        key="audiobookshelf", name="Audiobookshelf", category="media",
        vi="Máy chủ sách nói và podcast, nhớ đúng chỗ đang nghe trên mọi thiết bị.",
        en="An audiobook and podcast server that remembers where you stopped on every device.",
        website="https://audiobookshelf.org",
        image="advplyr/audiobookshelf", tag_pages=6,
        container_port=80,
        volumes=("config:/config", "metadata:/metadata", "${BOOKS_DIR}:/audiobooks"),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 13378),
            text("BOOKS_DIR", "Thư mục sách nói", "Audiobook folder", "/srv/audiobooks",
                 help_vi=MEDIA_HELP_VI, help_en=MEDIA_HELP_EN),
        ),
    ),
    App(
        key="calibre-web", name="Calibre-Web", category="media",
        vi="Tủ sách điện tử qua web: đọc trực tiếp, gửi sang Kindle, quản lý theo tác giả và thể loại.",
        en="A web ebook library: read in the browser, send to Kindle, organise by author and genre.",
        website="https://github.com/janeczku/calibre-web",
        image="linuxserver/calibre-web", tag_pages=6,
        container_port=8083,
        volumes=("config:/config", "${BOOKS_DIR}:/books"),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8086),
            text("BOOKS_DIR", "Thư mục sách", "Books folder", "/srv/books",
                 help_vi=MEDIA_HELP_VI, help_en=MEDIA_HELP_EN),
        ),
        environment={"PUID": "1000", "PGID": "1000", "TZ": "Asia/Ho_Chi_Minh"},
    ),
    App(
        key="komga", name="Komga", category="media",
        vi="Máy chủ truyện tranh và manga, đọc ngay trên trình duyệt hoặc ứng dụng OPDS.",
        en="A comics and manga server you can read in the browser or any OPDS app.",
        website="https://komga.org",
        image="gotson/komga", tag_pages=6,
        container_port=25600,
        volumes=("config:/config", "${COMICS_DIR}:/data"),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 25600),
            text("COMICS_DIR", "Thư mục truyện", "Comics folder", "/srv/comics",
                 help_vi=MEDIA_HELP_VI, help_en=MEDIA_HELP_EN),
        ),
    ),
    App(
        key="immich", name="Immich", category="media",
        vi="Thư viện ảnh và video tự quản, tự sao lưu từ điện thoại và nhận diện khuôn mặt.",
        en="A self-hosted photo and video library with phone backup and face recognition.",
        website="https://immich.app",
        image="ghcr.io/immich-app/immich-server", registry="ghcr", min_major=1,
        side_images=("ghcr.io/immich-app/immich-machine-learning:release",
                     "redis:7-alpine", "ghcr.io/immich-app/postgres:14-vectorchord0.4.3-pgvectors0.2.0"),
        version_values={"ML_IMAGE": "ghcr.io/immich-app/immich-machine-learning:release",
                        "REDIS_IMAGE": "redis:7-alpine",
                        "DB_IMAGE": "ghcr.io/immich-app/postgres:14-vectorchord0.4.3-pgvectors0.2.0"},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 2283),
            text("UPLOAD_DIR", "Thư mục lưu ảnh", "Photo folder", "/srv/immich",
                 help_vi="Toàn bộ ảnh tải lên nằm ở đây; hãy chọn ổ còn nhiều chỗ trống.",
                 help_en="Every uploaded photo lands here, so pick a disk with room to grow."),
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
      DB_HOSTNAME: db
      DB_USERNAME: immich
      DB_PASSWORD: ${DB_PASSWORD}
      DB_DATABASE_NAME: immich
      REDIS_HOSTNAME: redis
    ports:
      - "${HTTP_PORT}:2283"
    volumes:
      - ${UPLOAD_DIR}:/data

  machine-learning:
    image: ${ML_IMAGE}
    container_name: ${CONTAINER_NAME}-ml
    restart: unless-stopped
    volumes:
      - model-cache:/cache

  redis:
    image: ${REDIS_IMAGE}
    container_name: ${CONTAINER_NAME}-redis
    restart: unless-stopped

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      POSTGRES_DB: immich
      POSTGRES_USER: immich
      POSTGRES_PASSWORD: ${DB_PASSWORD}
      POSTGRES_INITDB_ARGS: "--data-checksums"
    volumes:
      - db:/var/lib/postgresql/data

volumes:
  model-cache:
  db:
""",
    ),
    App(
        key="photoprism", name="PhotoPrism", category="media",
        vi="Quản lý ảnh có tìm kiếm bằng nội dung, tự gắn thẻ và xếp theo địa điểm.",
        en="Photo management with content search, automatic tagging and map view.",
        website="https://photoprism.app",
        image="photoprism/photoprism", tag_pages=4, fixed_tags=("240915",),
        container_port=2342,
        volumes=("${PHOTOS_DIR}:/photoprism/originals", "storage:/photoprism/storage"),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 2342),
            text("PHOTOS_DIR", "Thư mục ảnh", "Photos folder", "/srv/photos",
                 help_vi=MEDIA_HELP_VI, help_en=MEDIA_HELP_EN),
            password("ADMIN_PASSWORD", "Mật khẩu quản trị", "Admin password"),
        ),
        environment={"PHOTOPRISM_ADMIN_PASSWORD": "${ADMIN_PASSWORD}",
                     "PHOTOPRISM_HTTP_PORT": "2342"},
    ),
    App(
        key="qbittorrent", name="qBittorrent", category="media",
        vi="Máy tải torrent có giao diện web, xếp hàng và giới hạn băng thông.",
        en="A torrent client with a web UI, queueing and bandwidth limits.",
        website="https://qbittorrent.org",
        image="linuxserver/qbittorrent", tag_pages=6,
        container_port=8080,
        volumes=("config:/config", "${DOWNLOAD_DIR}:/downloads"),
        extra_ports=('"${TORRENT_PORT}:6881/tcp"', '"${TORRENT_PORT}:6881/udp"'),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8087),
            port("TORRENT_PORT", "Cổng torrent", "Torrent port", 6881,
                 help_vi="Cổng nhận kết nối từ máy khác; mở trên tường lửa thì tải nhanh hơn nhiều.",
                 help_en="The port other peers connect to; opening it on the firewall speeds downloads up a lot."),
            text("DOWNLOAD_DIR", "Thư mục tải về", "Download folder", "/srv/downloads"),
        ),
        environment={"PUID": "1000", "PGID": "1000", "TZ": "Asia/Ho_Chi_Minh",
                     "WEBUI_PORT": "8080"},
    ),
    App(
        key="transmission", name="Transmission", category="media",
        vi="Máy tải torrent gọn nhẹ, ăn ít RAM, hợp với máy chủ cấu hình thấp.",
        en="A lightweight torrent client that sips memory — good for small servers.",
        website="https://transmissionbt.com",
        image="linuxserver/transmission", tag_pages=6,
        container_port=9091,
        volumes=("config:/config", "${DOWNLOAD_DIR}:/downloads"),
        extra_ports=('"${TORRENT_PORT}:51413/tcp"', '"${TORRENT_PORT}:51413/udp"'),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 9091),
            port("TORRENT_PORT", "Cổng torrent", "Torrent port", 51413),
            text("DOWNLOAD_DIR", "Thư mục tải về", "Download folder", "/srv/downloads"),
        ),
        environment={"PUID": "1000", "PGID": "1000", "TZ": "Asia/Ho_Chi_Minh"},
    ),
    App(
        key="sonarr", name="Sonarr", category="media",
        vi="Theo dõi phim bộ: tự tải tập mới, đổi tên và xếp vào đúng thư mục.",
        en="Follows TV series: grabs new episodes, renames them and files them away.",
        website="https://sonarr.tv",
        image="linuxserver/sonarr", tag_pages=6,
        container_port=8989,
        volumes=("config:/config", "${MEDIA_DIR}:/tv", "${DOWNLOAD_DIR}:/downloads"),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8989),
            text("MEDIA_DIR", "Thư mục phim bộ", "TV folder", "/srv/media/tv"),
            text("DOWNLOAD_DIR", "Thư mục tải về", "Download folder", "/srv/downloads"),
        ),
        environment={"PUID": "1000", "PGID": "1000", "TZ": "Asia/Ho_Chi_Minh"},
    ),
    App(
        key="radarr", name="Radarr", category="media",
        vi="Theo dõi phim lẻ: tự tải khi có bản mới, đổi tên và xếp vào thư viện.",
        en="Follows movies: grabs new releases, renames them and files them into the library.",
        website="https://radarr.video",
        image="linuxserver/radarr", tag_pages=6,
        container_port=7878,
        volumes=("config:/config", "${MEDIA_DIR}:/movies", "${DOWNLOAD_DIR}:/downloads"),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 7878),
            text("MEDIA_DIR", "Thư mục phim", "Movie folder", "/srv/media/movies"),
            text("DOWNLOAD_DIR", "Thư mục tải về", "Download folder", "/srv/downloads"),
        ),
        environment={"PUID": "1000", "PGID": "1000", "TZ": "Asia/Ho_Chi_Minh"},
    ),
    App(
        key="prowlarr", name="Prowlarr", category="media",
        vi="Quản lý nguồn tìm kiếm chung cho Sonarr và Radarr, khai báo một lần dùng cho tất cả.",
        en="One place to manage search indexers for Sonarr and Radarr — configure once, use everywhere.",
        website="https://github.com/Prowlarr/Prowlarr",
        image="linuxserver/prowlarr", tag_pages=6,
        container_port=9696, volumes=("config:/config",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 9696),),
        environment={"PUID": "1000", "PGID": "1000", "TZ": "Asia/Ho_Chi_Minh"},
    ),
    App(
        key="bazarr", name="Bazarr", category="media",
        vi="Tự tìm và tải phụ đề cho phim bộ, phim lẻ đã có trong thư viện.",
        en="Finds and downloads subtitles for the shows and movies already in your library.",
        website="https://bazarr.media",
        image="linuxserver/bazarr", tag_pages=6,
        container_port=6767,
        volumes=("config:/config", "${MEDIA_DIR}:/media"),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 6767),
            text("MEDIA_DIR", "Thư mục phim", "Media folder", "/srv/media"),
        ),
        environment={"PUID": "1000", "PGID": "1000", "TZ": "Asia/Ho_Chi_Minh"},
    ),
    App(
        key="jellyseerr", name="Jellyseerr", category="media",
        vi="Trang để người nhà bấm yêu cầu phim, bạn duyệt rồi hệ thống tự tải.",
        en="A page where your household requests movies; you approve and the system fetches them.",
        website="https://github.com/Fallenbagel/jellyseerr",
        image="fallenbagel/jellyseerr", tag_pages=6,
        container_port=5055, volumes=("config:/app/config",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 5055),),
        environment={"TZ": "Asia/Ho_Chi_Minh"},
    ),
    App(
        key="tubearchivist", name="Tube Archivist", category="media",
        vi="Lưu lại kênh YouTube về máy chủ, có tìm kiếm toàn văn phụ đề.",
        en="Archives YouTube channels onto your server with full-text subtitle search.",
        website="https://tubearchivist.com",
        image="bbilly1/tubearchivist", tag_pages=6,
        side_images=("redis:7-alpine", "bbilly1/tubearchivist-es:8.14.3"),
        version_values={"REDIS_IMAGE": "redis:7-alpine",
                        "ES_IMAGE": "bbilly1/tubearchivist-es:8.14.3"},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8088),
            text("MEDIA_DIR", "Thư mục lưu video", "Video folder", "/srv/youtube"),
            text("ADMIN_USER", "Tài khoản quản trị", "Admin user", "admin"),
            password("ADMIN_PASSWORD", "Mật khẩu quản trị", "Admin password"),
        ),
        compose="""
services:
  app:
    image: ${IMAGE}
    container_name: ${CONTAINER_NAME}
    restart: unless-stopped
    depends_on:
      - es
      - redis
    environment:
      ES_URL: http://es:9200
      REDIS_CON: redis://redis:6379
      HOST_UID: "1000"
      HOST_GID: "1000"
      TA_HOST: http://localhost
      TA_USERNAME: ${ADMIN_USER}
      TA_PASSWORD: ${ADMIN_PASSWORD}
      ELASTIC_PASSWORD: ${ADMIN_PASSWORD}
      TZ: Asia/Ho_Chi_Minh
    ports:
      - "${HTTP_PORT}:8000"
    volumes:
      - ${MEDIA_DIR}:/youtube
      - cache:/cache

  es:
    image: ${ES_IMAGE}
    container_name: ${CONTAINER_NAME}-es
    restart: unless-stopped
    environment:
      ELASTIC_PASSWORD: ${ADMIN_PASSWORD}
      discovery.type: single-node
      xpack.security.enabled: "true"
    volumes:
      - es:/usr/share/elasticsearch/data

  redis:
    image: ${REDIS_IMAGE}
    container_name: ${CONTAINER_NAME}-redis
    restart: unless-stopped
    volumes:
      - redis:/data

volumes:
  cache:
  es:
  redis:
""",
    ),
    App(
        key="kavita", name="Kavita", category="media",
        vi="Tủ sách và truyện tranh gọn nhẹ, đọc mượt trên điện thoại.",
        en="A fast library for books and comics that reads well on a phone.",
        website="https://kavitareader.com",
        image="jvmilazz0/kavita", tag_pages=6,
        container_port=5000,
        volumes=("config:/kavita/config", "${BOOKS_DIR}:/data"),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 5001),
            text("BOOKS_DIR", "Thư mục sách", "Books folder", "/srv/books"),
        ),
    ),
    App(
        key="metube", name="MeTube", category="media",
        vi="Tải video từ YouTube và hàng trăm trang khác bằng một ô dán liên kết.",
        en="Downloads video from YouTube and hundreds of other sites by pasting a link.",
        website="https://github.com/alexta69/metube",
        image="ghcr.io/alexta69/metube", registry="ghcr",
        fixed_tags=("2024-10-16",),
        container_port=8081,
        volumes=("${DOWNLOAD_DIR}:/downloads",),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8089),
            text("DOWNLOAD_DIR", "Thư mục tải về", "Download folder", "/srv/downloads"),
        ),
    ),
]
