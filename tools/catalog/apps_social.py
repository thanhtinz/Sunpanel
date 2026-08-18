"""Nhóm ứng dụng mạng xã hội, xuất bản nội dung và thư viện ảnh."""

from model import App, password, port, text

PG = "postgres:16-alpine"
MARIADB = "mariadb:11"
REDIS = "redis:7-alpine"

APPS = [
    App(
        key="mastodon", name="Mastodon", category="website",
        vi="Máy chủ mạng xã hội liên kết theo chuẩn ActivityPub, người dùng theo dõi được cả máy chủ khác.",
        en="An ActivityPub social server whose users can follow accounts on any other server.",
        website="https://joinmastodon.org",
        image="ghcr.io/mastodon/mastodon", registry="ghcr", min_major=4,
        side_images=(PG, REDIS), version_values={"DB_IMAGE": PG, "REDIS_IMAGE": REDIS},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 3200),
            text("LOCAL_DOMAIN", "Tên miền", "Domain", "social.example.com",
                 help_vi="Tên miền này đi vào định danh của mọi tài khoản; đổi sau là hỏng liên kết.",
                 help_en="This domain becomes part of every account's identity; changing it later breaks federation."),
            password("SECRET_KEY_BASE", "Khóa bí mật", "Secret key"),
            password("OTP_SECRET", "Khóa mã hai lớp", "OTP secret"),
            password("DB_PASSWORD", "Mật khẩu cơ sở dữ liệu", "Database password"),
        ),
        compose="""
services:
  app:
    image: ${IMAGE}
    container_name: ${CONTAINER_NAME}
    restart: unless-stopped
    command: bash -c "rails s -p 3000"
    depends_on:
      - db
      - redis
    environment:
      LOCAL_DOMAIN: ${LOCAL_DOMAIN}
      SECRET_KEY_BASE: ${SECRET_KEY_BASE}
      OTP_SECRET: ${OTP_SECRET}
      DB_HOST: db
      DB_NAME: mastodon
      DB_USER: mastodon
      DB_PASS: ${DB_PASSWORD}
      REDIS_HOST: redis
      RAILS_ENV: production
    ports:
      - "${HTTP_PORT}:3000"
    volumes:
      - system:/mastodon/public/system

  sidekiq:
    image: ${IMAGE}
    container_name: ${CONTAINER_NAME}-sidekiq
    restart: unless-stopped
    command: bundle exec sidekiq
    depends_on:
      - db
      - redis
    environment:
      LOCAL_DOMAIN: ${LOCAL_DOMAIN}
      SECRET_KEY_BASE: ${SECRET_KEY_BASE}
      OTP_SECRET: ${OTP_SECRET}
      DB_HOST: db
      DB_NAME: mastodon
      DB_USER: mastodon
      DB_PASS: ${DB_PASSWORD}
      REDIS_HOST: redis
      RAILS_ENV: production
    volumes:
      - system:/mastodon/public/system

  redis:
    image: ${REDIS_IMAGE}
    container_name: ${CONTAINER_NAME}-redis
    restart: unless-stopped
    volumes:
      - redis:/data

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      POSTGRES_DB: mastodon
      POSTGRES_USER: mastodon
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - db:/var/lib/postgresql/data

volumes:
  system:
  redis:
  db:
""",
    ),
    App(
        key="misskey", name="Misskey", category="website",
        vi="Mạng xã hội liên kết kiểu Nhật, nhiều biểu cảm tùy chỉnh và giao diện linh hoạt.",
        en="A Japanese-flavoured federated social network with custom reactions and a flexible UI.",
        website="https://misskey-hub.net",
        image="misskey/misskey", tag_pages=6, min_major=13,
        side_images=(PG, REDIS), version_values={"DB_IMAGE": PG, "REDIS_IMAGE": REDIS},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 3201),
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
    ports:
      - "${HTTP_PORT}:3000"
    volumes:
      - files:/misskey/files
      - config:/misskey/.config

  redis:
    image: ${REDIS_IMAGE}
    container_name: ${CONTAINER_NAME}-redis
    restart: unless-stopped
    volumes:
      - redis:/data

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      POSTGRES_DB: misskey
      POSTGRES_USER: misskey
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - db:/var/lib/postgresql/data

volumes:
  files:
  config:
  redis:
  db:
""",
    ),
    App(
        key="lemmy", name="Lemmy", category="website",
        vi="Diễn đàn liên kết theo chủ đề, kiểu Reddit mà mỗi máy chủ tự quản lấy cộng đồng của mình.",
        en="A federated link-aggregator forum — Reddit-shaped, with each server running its own communities.",
        website="https://join-lemmy.org",
        image="dessalines/lemmy", tag_pages=6, min_major=0,
        side_images=(PG, "dessalines/lemmy-ui:0.19.7"),
        version_values={"DB_IMAGE": PG, "UI_IMAGE": "dessalines/lemmy-ui:0.19.7"},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 3202),
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
      RUST_LOG: warn
    volumes:
      - pictrs:/pictrs

  ui:
    image: ${UI_IMAGE}
    container_name: ${CONTAINER_NAME}-ui
    restart: unless-stopped
    depends_on:
      - app
    environment:
      LEMMY_UI_LEMMY_INTERNAL_HOST: app:8536
    ports:
      - "${HTTP_PORT}:1234"

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      POSTGRES_DB: lemmy
      POSTGRES_USER: lemmy
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - db:/var/lib/postgresql/data

volumes:
  pictrs:
  db:
""",
    ),
    App(
        key="writefreely", name="WriteFreely", category="website",
        vi="Nền tảng viết tối giản, không đếm lượt thích, không bình luận — chỉ có chữ.",
        en="A minimalist writing platform: no likes, no comments, just the text.",
        website="https://writefreely.org",
        image="writeas/writefreely", tag_pages=6, min_major=0,
        container_port=8080, volumes=("data:/data", "config:/go/keys"),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 3203),),
    ),
    App(
        key="peertube", name="PeerTube", category="media",
        vi="Nền tảng video liên kết, người xem chia sẻ băng thông cho nhau qua WebTorrent.",
        en="A federated video platform where viewers share bandwidth with each other over WebTorrent.",
        website="https://joinpeertube.org",
        image="chocobozzz/peertube", tag_pages=6, tag_any_suffix=True,
        side_images=(PG, REDIS), version_values={"DB_IMAGE": PG, "REDIS_IMAGE": REDIS},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 9200),
            text("WEBSERVER_HOSTNAME", "Tên miền", "Hostname", "video.example.com"),
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
      PEERTUBE_WEBSERVER_HOSTNAME: ${WEBSERVER_HOSTNAME}
      PEERTUBE_DB_HOSTNAME: db
      PEERTUBE_DB_USERNAME: peertube
      PEERTUBE_DB_PASSWORD: ${DB_PASSWORD}
      PEERTUBE_REDIS_HOSTNAME: redis
      PEERTUBE_SMTP_DISABLE: "true"
    ports:
      - "${HTTP_PORT}:9000"
    volumes:
      - data:/data
      - config:/config

  redis:
    image: ${REDIS_IMAGE}
    container_name: ${CONTAINER_NAME}-redis
    restart: unless-stopped

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      POSTGRES_DB: peertube
      POSTGRES_USER: peertube
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
        key="owncast", name="Owncast", category="media",
        vi="Máy chủ phát trực tiếp của riêng bạn, thay Twitch mà không ai chen quảng cáo vào.",
        en="Your own live streaming server — a Twitch replacement with nobody inserting ads.",
        website="https://owncast.online",
        image="owncast/owncast", tag_pages=6, min_major=0,
        container_port=8080, volumes=("data:/app/data",),
        extra_ports=('"${RTMP_PORT}:1935"',),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8480),
            port("RTMP_PORT", "Cổng nhận luồng", "Stream ingest port", 1935,
                 help_vi="Cổng phần mềm phát như OBS đẩy luồng vào.",
                 help_en="The port streaming software such as OBS pushes to."),
        ),
    ),
    App(
        key="castopod", name="Castopod", category="media",
        vi="Nền tảng đăng podcast tự quản, có thống kê lượt nghe và liên kết ActivityPub.",
        en="A self-hosted podcast platform with listener stats and ActivityPub federation.",
        website="https://castopod.org",
        image="castopod/castopod", tag_pages=6, min_major=1,
        side_images=(MARIADB,), version_values={"DB_IMAGE": MARIADB},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8481),
            text("BASE_URL", "Địa chỉ trang", "Site URL", "http://localhost:8481"),
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
      CP_BASEURL: ${BASE_URL}
      CP_ANALYTICS_SALT: ${DB_PASSWORD}
      CP_DATABASE_HOSTNAME: db
      CP_DATABASE_NAME: castopod
      CP_DATABASE_USERNAME: castopod
      CP_DATABASE_PASSWORD: ${DB_PASSWORD}
      CP_CACHE_HANDLER: file
    ports:
      - "${HTTP_PORT}:8000"
    volumes:
      - media:/var/www/castopod/public/media

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      MARIADB_DATABASE: castopod
      MARIADB_USER: castopod
      MARIADB_PASSWORD: ${DB_PASSWORD}
      MARIADB_RANDOM_ROOT_PASSWORD: "1"
    volumes:
      - db:/var/lib/mysql

volumes:
  media:
  db:
""",
    ),
    App(
        key="funkwhale", name="Funkwhale", category="media",
        vi="Máy chủ nhạc liên kết: nghe thư viện của mình và của các máy chủ bạn bè.",
        en="A federated music server: listen to your own library and to friendly servers'.",
        website="https://funkwhale.audio",
        image="funkwhale/funkwhale", tag_pages=6, min_major=1,
        side_images=(PG, REDIS), version_values={"DB_IMAGE": PG, "REDIS_IMAGE": REDIS},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8482),
            text("FUNKWHALE_HOSTNAME", "Tên miền", "Hostname", "music.example.com"),
            password("DJANGO_SECRET_KEY", "Khóa bí mật", "Secret key"),
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
      FUNKWHALE_HOSTNAME: ${FUNKWHALE_HOSTNAME}
      DJANGO_SECRET_KEY: ${DJANGO_SECRET_KEY}
      DATABASE_URL: postgresql://funkwhale:${DB_PASSWORD}@db:5432/funkwhale
      CACHE_URL: redis://redis:6379/0
    ports:
      - "${HTTP_PORT}:80"
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
    environment:
      POSTGRES_DB: funkwhale
      POSTGRES_USER: funkwhale
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - db:/var/lib/postgresql/data

volumes:
  data:
  db:
""",
    ),
    App(
        key="matomo", name="Matomo", category="monitoring",
        vi="Thống kê truy cập website đầy đủ như Google Analytics nhưng dữ liệu nằm ở máy bạn.",
        en="Web analytics as complete as Google Analytics, with the data staying on your machine.",
        website="https://matomo.org",
        image="matomo", tag_suffix="-apache", tag_pages=8, min_major=4,
        side_images=(MARIADB,), version_values={"DB_IMAGE": MARIADB},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8483),
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
      MATOMO_DATABASE_HOST: db
      MATOMO_DATABASE_DBNAME: matomo
      MATOMO_DATABASE_USERNAME: matomo
      MATOMO_DATABASE_PASSWORD: ${DB_PASSWORD}
    ports:
      - "${HTTP_PORT}:80"
    volumes:
      - data:/var/www/html

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      MARIADB_DATABASE: matomo
      MARIADB_USER: matomo
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
        key="bookstack", name="BookStack", category="productivity",
        vi="Cơ sở tri thức sắp theo sách, chương và trang — dễ tìm lại hơn một đống tài liệu rời.",
        en="A knowledge base organised into books, chapters and pages — easier to navigate than loose docs.",
        website="https://bookstackapp.com",
        image="linuxserver/bookstack", tag_pages=6, tag_any_suffix=True,
        side_images=(MARIADB,), version_values={"DB_IMAGE": MARIADB},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8484),
            text("APP_URL", "Địa chỉ trang", "Site URL", "http://localhost:8484"),
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
      APP_URL: ${APP_URL}
      DB_HOST: db
      DB_PORT: "3306"
      DB_DATABASE: bookstack
      DB_USERNAME: bookstack
      DB_PASSWORD: ${DB_PASSWORD}
    ports:
      - "${HTTP_PORT}:80"
    volumes:
      - config:/config

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      MARIADB_DATABASE: bookstack
      MARIADB_USER: bookstack
      MARIADB_PASSWORD: ${DB_PASSWORD}
      MARIADB_RANDOM_ROOT_PASSWORD: "1"
    volumes:
      - db:/var/lib/mysql

volumes:
  config:
  db:
""",
    ),
    App(
        key="joplin", name="Joplin Server", category="productivity",
        vi="Máy chủ đồng bộ cho ứng dụng ghi chú Joplin, giữ ghi chú mã hóa đầu cuối.",
        en="The sync server for the Joplin note app, keeping notes end-to-end encrypted.",
        website="https://joplinapp.org",
        image="joplin/server", tag_pages=6, min_major=2,
        side_images=(PG,), version_values={"DB_IMAGE": PG},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 22300),
            text("APP_BASE_URL", "Địa chỉ trang", "Site URL", "http://localhost:22300"),
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
      APP_BASE_URL: ${APP_BASE_URL}
      APP_PORT: "22300"
      DB_CLIENT: pg
      POSTGRES_HOST: db
      POSTGRES_PORT: "5432"
      POSTGRES_DATABASE: joplin
      POSTGRES_USER: joplin
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    ports:
      - "${HTTP_PORT}:22300"

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      POSTGRES_DB: joplin
      POSTGRES_USER: joplin
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - db:/var/lib/postgresql/data

volumes:
  db:
""",
    ),
    App(
        key="trilium", name="Trilium Notes", category="productivity",
        vi="Ghi chú dạng cây phân cấp cho kho tri thức cá nhân lớn, có liên kết chéo giữa các ghi chú.",
        en="Hierarchical notes for a large personal knowledge base, with cross-links between notes.",
        website="https://github.com/TriliumNext/Notes",
        image="triliumnext/notes", tag_pages=6, min_major=0,
        container_port=8080, volumes=("data:/home/node/trilium-data",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 8485),),
    ),
    App(
        key="siyuan", name="SiYuan", category="productivity",
        vi="Ghi chú dạng khối có liên kết hai chiều, dùng được ngoại tuyến rồi đồng bộ sau.",
        en="Block-based notes with bidirectional links that work offline and sync later.",
        website="https://b3log.org/siyuan",
        image="b3log/siyuan", tag_pages=6, min_major=2,
        container_port=6806, volumes=("data:/siyuan/workspace",),
        command="--workspace=/siyuan/workspace --accessAuthCode=${ACCESS_CODE}",
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 6806),
            password("ACCESS_CODE", "Mã truy cập", "Access code"),
        ),
    ),
    App(
        key="lychee", name="Lychee", category="media",
        vi="Thư viện ảnh gọn nhẹ để đăng và chia sẻ album, tự đọc thông tin máy ảnh trong tệp.",
        en="A light photo gallery for publishing and sharing albums, reading camera data from the files.",
        website="https://lycheeorg.github.io",
        image="lycheeorg/lychee", tag_pages=6, min_major=5,
        container_port=80,
        volumes=("config:/conf", "uploads:/uploads", "sym:/sym"),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 8486),),
        environment={"PUID": "1000", "PGID": "1000", "TIMEZONE": "Asia/Ho_Chi_Minh",
                     "DB_CONNECTION": "sqlite"},
    ),
    App(
        key="piwigo", name="Piwigo", category="media",
        vi="Thư viện ảnh cho bộ sưu tập lớn: gắn thẻ, phân quyền theo album, tìm theo ngày chụp.",
        en="A photo gallery for large collections: tags, per-album permissions and date search.",
        website="https://piwigo.org",
        image="linuxserver/piwigo", tag_pages=6, tag_any_suffix=True,
        side_images=(MARIADB,), version_values={"DB_IMAGE": MARIADB},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8487),
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
    ports:
      - "${HTTP_PORT}:80"
    volumes:
      - config:/config

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      MARIADB_DATABASE: piwigo
      MARIADB_USER: piwigo
      MARIADB_PASSWORD: ${DB_PASSWORD}
      MARIADB_RANDOM_ROOT_PASSWORD: "1"
    volumes:
      - db:/var/lib/mysql

volumes:
  config:
  db:
""",
    ),
    App(
        key="chibisafe", name="Chibisafe", category="storage",
        vi="Máy chủ nhận tệp tải lên và trả về liên kết chia sẻ, kèm tiện ích chụp màn hình.",
        en="An upload server that hands back share links, with screenshot tooling included.",
        website="https://chibisafe.app",
        image="chibisafe/chibisafe", tag_pages=6, min_major=0,
        container_port=8000,
        volumes=("uploads:/app/uploads", "database:/app/database"),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 8488),),
    ),
    App(
        key="zipline", name="Zipline", category="storage",
        vi="Máy chủ nhận ảnh chụp màn hình và tệp, sinh liên kết ngắn để dán vào chỗ khác.",
        en="A screenshot and file host that generates short links to paste elsewhere.",
        website="https://zipline.diced.sh",
        image="ghcr.io/diced/zipline", registry="ghcr", min_major=3,
        side_images=(PG,), version_values={"DB_IMAGE": PG},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8489),
            text("CORE_RETURN_HTTPS", "Dùng HTTPS trong liên kết", "Use HTTPS in links", "false"),
            password("CORE_SECRET", "Khóa bí mật", "Secret"),
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
      CORE_SECRET: ${CORE_SECRET}
      CORE_RETURN_HTTPS: ${CORE_RETURN_HTTPS}
      CORE_DATABASE_URL: postgres://zipline:${DB_PASSWORD}@db/zipline
    ports:
      - "${HTTP_PORT}:3000"
    volumes:
      - uploads:/zipline/uploads

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      POSTGRES_DB: zipline
      POSTGRES_USER: zipline
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - db:/var/lib/postgresql/data

volumes:
  uploads:
  db:
""",
    ),
    App(
        key="pingvin-share", name="Pingvin Share", category="storage",
        vi="Gửi tệp lớn qua liên kết có hạn dùng và mật khẩu, thay WeTransfer.",
        en="Sends large files through links with an expiry and a password — a WeTransfer replacement.",
        website="https://github.com/stonith404/pingvin-share",
        image="stonith404/pingvin-share", tag_pages=6, min_major=0,
        container_port=3000,
        volumes=("data:/opt/app/backend/data", "images:/opt/app/frontend/public/img"),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 8490),),
    ),
    App(
        key="opengist", name="Opengist", category="development",
        vi="Kho đoạn mã tự quản kiểu GitHub Gist, mỗi đoạn là một kho Git thật.",
        en="A self-hosted GitHub Gist: every snippet is a real Git repository.",
        website="https://opengist.io",
        image="ghcr.io/thomiceli/opengist", registry="ghcr", min_major=1,
        container_port=6157, volumes=("data:/opengist",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 6157),),
    ),
    App(
        key="onedev", name="OneDev", category="development",
        vi="Máy chủ Git kèm CI/CD và quản lý công việc, cài xong là dùng được ngay.",
        en="A Git server with built-in CI/CD and issue tracking that works right after install.",
        website="https://onedev.io",
        image="1dev/server", tag_pages=6, min_major=9,
        container_port=6610, volumes=("data:/opt/onedev",),
        extra_ports=('"${SSH_PORT}:22"',),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 6610),
            port("SSH_PORT", "Cổng SSH", "SSH port", 6611),
        ),
    ),
    App(
        key="mumble", name="Mumble", category="productivity",
        vi="Máy chủ thoại độ trễ thấp cho nhóm chơi game và họp nhóm, ăn rất ít băng thông.",
        en="A low-latency voice server for gaming groups and meetings that sips bandwidth.",
        website="https://mumble.info",
        image="mumblevoip/mumble-server", tag_pages=6, tag_any_suffix=True,
        container_port=64738, volumes=("data:/data",),
        fields=(
            port("VOICE_PORT", "Cổng thoại", "Voice port", 64738),
            password("SUPERUSER_PASSWORD", "Mật khẩu quản trị", "Superuser password"),
        ),
        environment={"MUMBLE_CONFIG_ICE_SECRET_WRITE": "${SUPERUSER_PASSWORD}"},
    ),
    App(
        key="hugo", name="Hugo", category="website",
        vi="Bộ dựng trang tĩnh rất nhanh, phục vụ luôn kết quả cho bạn xem thử.",
        en="A very fast static site generator that also serves the result for preview.",
        website="https://gohugo.io",
        image="hugomods/hugo", tag_pages=6, tag_any_suffix=True,
        container_port=1313,
        command="server --bind 0.0.0.0 --baseURL ${BASE_URL} --appendPort=false",
        volumes=("${SITE_DIR}:/src",),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 1313),
            text("SITE_DIR", "Thư mục dự án", "Project folder", "/srv/hugo"),
            text("BASE_URL", "Địa chỉ trang", "Site URL", "http://localhost:1313"),
        ),
    ),
    App(
        key="mkdocs", name="MkDocs Material", category="website",
        vi="Dựng trang tài liệu từ tệp Markdown, có tìm kiếm và giao diện Material sẵn.",
        en="Builds a documentation site from Markdown, with search and the Material theme included.",
        website="https://squidfunk.github.io/mkdocs-material",
        image="squidfunk/mkdocs-material", tag_pages=6, min_major=9,
        container_port=8000,
        volumes=("${DOCS_DIR}:/docs",),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8491),
            text("DOCS_DIR", "Thư mục tài liệu", "Docs folder", "/srv/docs"),
        ),
    ),
]
