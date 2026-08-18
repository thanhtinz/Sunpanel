"""Nhóm ứng dụng doanh nghiệp: kế toán, khách hàng, hỗ trợ và tài sản."""

from model import App, password, port, text

PG = "postgres:16-alpine"
MARIADB = "mariadb:11"
REDIS = "redis:7-alpine"

APPS = [
    App(
        key="odoo", name="Odoo", category="productivity",
        vi="Bộ phần mềm quản trị doanh nghiệp: bán hàng, kho, kế toán, nhân sự trong một hệ.",
        en="A business management suite: sales, inventory, accounting and HR in one system.",
        website="https://odoo.com",
        image="odoo", tag_pages=6, min_major=16,
        side_images=(PG,), version_values={"DB_IMAGE": PG},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8069),
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
      HOST: db
      USER: odoo
      PASSWORD: ${DB_PASSWORD}
    ports:
      - "${HTTP_PORT}:8069"
    volumes:
      - data:/var/lib/odoo
      - addons:/mnt/extra-addons

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      POSTGRES_DB: postgres
      POSTGRES_USER: odoo
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - db:/var/lib/postgresql/data

volumes:
  data:
  addons:
  db:
""",
    ),
    App(
        key="dolibarr", name="Dolibarr", category="productivity",
        vi="Quản trị doanh nghiệp nhỏ: khách hàng, báo giá, hóa đơn, kho và dự án.",
        en="Small-business management: customers, quotes, invoices, stock and projects.",
        website="https://dolibarr.org",
        image="dolibarr/dolibarr", tag_pages=6, min_major=17,
        side_images=(MARIADB,), version_values={"DB_IMAGE": MARIADB},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8600),
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
      DOLI_DB_HOST: db
      DOLI_DB_NAME: dolibarr
      DOLI_DB_USER: dolibarr
      DOLI_DB_PASSWORD: ${DB_PASSWORD}
      DOLI_ADMIN_LOGIN: ${ADMIN_USER}
      DOLI_ADMIN_PASSWORD: ${ADMIN_PASSWORD}
    ports:
      - "${HTTP_PORT}:80"
    volumes:
      - documents:/var/www/documents
      - custom:/var/www/html/custom

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      MARIADB_DATABASE: dolibarr
      MARIADB_USER: dolibarr
      MARIADB_PASSWORD: ${DB_PASSWORD}
      MARIADB_RANDOM_ROOT_PASSWORD: "1"
    volumes:
      - db:/var/lib/mysql

volumes:
  documents:
  custom:
  db:
""",
    ),
    App(
        key="espocrm", name="EspoCRM", category="productivity",
        vi="Quản lý quan hệ khách hàng: cơ hội bán hàng, liên hệ, chiến dịch và báo cáo.",
        en="Customer relationship management: leads, contacts, campaigns and reports.",
        website="https://espocrm.com",
        image="espocrm/espocrm", tag_pages=6, min_major=8,
        side_images=(MARIADB,), version_values={"DB_IMAGE": MARIADB},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8601),
            text("SITE_URL", "Địa chỉ trang", "Site URL", "http://localhost:8601"),
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
      ESPOCRM_DATABASE_HOST: db
      ESPOCRM_DATABASE_NAME: espocrm
      ESPOCRM_DATABASE_USER: espocrm
      ESPOCRM_DATABASE_PASSWORD: ${DB_PASSWORD}
      ESPOCRM_ADMIN_USERNAME: ${ADMIN_USER}
      ESPOCRM_ADMIN_PASSWORD: ${ADMIN_PASSWORD}
      ESPOCRM_SITE_URL: ${SITE_URL}
    ports:
      - "${HTTP_PORT}:80"
    volumes:
      - data:/var/www/html

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      MARIADB_DATABASE: espocrm
      MARIADB_USER: espocrm
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
        key="monica", name="Monica", category="productivity",
        vi="Sổ tay quan hệ cá nhân: nhớ ngày sinh, cuộc gặp và những gì bạn bè đã kể.",
        en="A personal relationship notebook: birthdays, meetings and what friends told you.",
        website="https://monicahq.com",
        image="monica", tag_suffix="-apache", tag_pages=6, min_major=3,
        side_images=(MARIADB,), version_values={"DB_IMAGE": MARIADB},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8602),
            text("APP_URL", "Địa chỉ trang", "Site URL", "http://localhost:8602"),
            password("APP_KEY_VALUE", "Khóa ứng dụng", "App key",
                     help_vi="Laravel yêu cầu khóa dài đúng 32 ký tự.",
                     help_en="Laravel needs a key of exactly 32 characters."),
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
      APP_KEY: ${APP_KEY_VALUE}
      APP_URL: ${APP_URL}
      DB_HOST: db
      DB_DATABASE: monica
      DB_USERNAME: monica
      DB_PASSWORD: ${DB_PASSWORD}
    ports:
      - "${HTTP_PORT}:80"
    volumes:
      - data:/var/www/html/storage

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      MARIADB_DATABASE: monica
      MARIADB_USER: monica
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
        key="ghostfolio", name="Ghostfolio", category="productivity",
        vi="Theo dõi danh mục đầu tư: cổ phiếu, quỹ và tiền mã hóa trong một bảng.",
        en="Tracks an investment portfolio: stocks, funds and crypto on one dashboard.",
        website="https://ghostfol.io",
        image="ghostfolio/ghostfolio", tag_pages=6, min_major=2,
        side_images=(PG, REDIS), version_values={"DB_IMAGE": PG, "REDIS_IMAGE": REDIS},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 3403),
            password("ACCESS_TOKEN_SALT", "Muối cho vé truy cập", "Access token salt"),
            password("JWT_SECRET_KEY", "Khóa ký phiên", "Session secret"),
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
      DATABASE_URL: postgresql://ghostfolio:${DB_PASSWORD}@db:5432/ghostfolio?sslmode=prefer
      ACCESS_TOKEN_SALT: ${ACCESS_TOKEN_SALT}
      JWT_SECRET_KEY: ${JWT_SECRET_KEY}
      REDIS_HOST: redis
      REDIS_PORT: "6379"
    ports:
      - "${HTTP_PORT}:3333"

  redis:
    image: ${REDIS_IMAGE}
    container_name: ${CONTAINER_NAME}-redis
    restart: unless-stopped

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      POSTGRES_DB: ghostfolio
      POSTGRES_USER: ghostfolio
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - db:/var/lib/postgresql/data

volumes:
  db:
""",
    ),
    App(
        key="snipe-it", name="Snipe-IT", category="productivity",
        vi="Quản lý tài sản công nghệ: máy tính, giấy phép phần mềm, ai đang giữ thiết bị nào.",
        en="IT asset management: machines, software licences and who currently holds what.",
        website="https://snipeitapp.com",
        image="snipe/snipe-it", tag_pages=6, tag_any_suffix=True,
        side_images=(MARIADB,), version_values={"DB_IMAGE": MARIADB},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8603),
            text("APP_URL", "Địa chỉ trang", "Site URL", "http://localhost:8603"),
            password("APP_KEY_VALUE", "Khóa ứng dụng", "App key",
                     help_vi="Snipe-IT yêu cầu khóa dạng base64:… dài 32 byte; xem tài liệu dự án để sinh.",
                     help_en="Snipe-IT wants a base64:… key of 32 bytes; see the project docs to generate one."),
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
      APP_KEY: ${APP_KEY_VALUE}
      APP_URL: ${APP_URL}
      MYSQL_PORT_3306_TCP_ADDR: db
      MYSQL_DATABASE: snipeit
      MYSQL_USER: snipeit
      MYSQL_PASSWORD: ${DB_PASSWORD}
    ports:
      - "${HTTP_PORT}:80"
    volumes:
      - data:/var/lib/snipeit

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      MARIADB_DATABASE: snipeit
      MARIADB_USER: snipeit
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
        key="osticket", name="osTicket", category="productivity",
        vi="Hệ thống nhận yêu cầu hỗ trợ qua email và biểu mẫu, chia theo phòng ban.",
        en="A support ticket system fed by email and web forms, split by department.",
        website="https://osticket.com",
        image="campbellsoftwaresolutions/osticket", tag_pages=6, tag_any_suffix=True,
        side_images=(MARIADB,), version_values={"DB_IMAGE": MARIADB},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8604),
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
      MYSQL_DATABASE: osticket
      MYSQL_USER: osticket
      MYSQL_PASSWORD: ${DB_PASSWORD}
    ports:
      - "${HTTP_PORT}:80"
    volumes:
      - data:/data

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      MARIADB_DATABASE: osticket
      MARIADB_USER: osticket
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
        key="freescout", name="FreeScout", category="productivity",
        vi="Hộp thư hỗ trợ dùng chung cho cả nhóm, thay Help Scout mà không tốn phí theo tháng.",
        en="A shared support inbox for the whole team — a Help Scout replacement with no monthly fee.",
        website="https://freescout.net",
        image="tiredofit/freescout", tag_prefix="php8.2-", tag_pages=8, min_major=1,
        side_images=(MARIADB,), version_values={"DB_IMAGE": MARIADB},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8605),
            text("SITE_URL", "Địa chỉ trang", "Site URL", "http://localhost:8605"),
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
      DB_HOST: db
      DB_NAME: freescout
      DB_USER: freescout
      DB_PASS: ${DB_PASSWORD}
      SITE_URL: ${SITE_URL}
      ADMIN_EMAIL: ${ADMIN_EMAIL}
      ADMIN_PASS: ${ADMIN_PASSWORD}
      TIMEZONE: Asia/Ho_Chi_Minh
    ports:
      - "${HTTP_PORT}:80"
    volumes:
      - data:/data

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      MARIADB_DATABASE: freescout
      MARIADB_USER: freescout
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
        key="chatwoot", name="Chatwoot", category="productivity",
        vi="Hộp thoại chăm sóc khách hàng gom mọi kênh: web, email, Facebook và WhatsApp.",
        en="A customer support inbox that gathers every channel: web, email, Facebook and WhatsApp.",
        website="https://chatwoot.com",
        image="chatwoot/chatwoot", tag_pages=6, tag_any_suffix=True,
        side_images=(PG, REDIS), version_values={"DB_IMAGE": PG, "REDIS_IMAGE": REDIS},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 3400),
            text("FRONTEND_URL", "Địa chỉ trang", "Site URL", "http://localhost:3400"),
            password("SECRET_KEY_BASE", "Khóa bí mật", "Secret key"),
            password("DB_PASSWORD", "Mật khẩu cơ sở dữ liệu", "Database password"),
        ),
        compose="""
services:
  app:
    image: ${IMAGE}
    container_name: ${CONTAINER_NAME}
    restart: unless-stopped
    command: bundle exec rails s -p 3000 -b 0.0.0.0
    depends_on:
      - db
      - redis
    environment:
      RAILS_ENV: production
      SECRET_KEY_BASE: ${SECRET_KEY_BASE}
      FRONTEND_URL: ${FRONTEND_URL}
      POSTGRES_HOST: db
      POSTGRES_DATABASE: chatwoot
      POSTGRES_USERNAME: chatwoot
      POSTGRES_PASSWORD: ${DB_PASSWORD}
      REDIS_URL: redis://redis:6379
    ports:
      - "${HTTP_PORT}:3000"
    volumes:
      - storage:/app/storage

  redis:
    image: ${REDIS_IMAGE}
    container_name: ${CONTAINER_NAME}-redis
    restart: unless-stopped

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      POSTGRES_DB: chatwoot
      POSTGRES_USER: chatwoot
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - db:/var/lib/postgresql/data

volumes:
  storage:
  db:
""",
    ),
    App(
        key="mayan-edms", name="Mayan EDMS", category="productivity",
        vi="Quản lý tài liệu cho tổ chức: phân loại tự động, nhận dạng chữ và quy trình duyệt.",
        en="Organisational document management: automatic classification, OCR and approval workflows.",
        website="https://mayan-edms.com",
        image="mayanedms/mayanedms", tag_pages=6, min_major=4,
        side_images=(PG, REDIS), version_values={"DB_IMAGE": PG, "REDIS_IMAGE": REDIS},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8606),
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
      MAYAN_DATABASES: "{'default':{'ENGINE':'django.db.backends.postgresql','NAME':'mayan','USER':'mayan','PASSWORD':'${DB_PASSWORD}','HOST':'db','PORT':5432}}"
      MAYAN_CELERY_BROKER_URL: redis://redis:6379/0
      MAYAN_CELERY_RESULT_BACKEND: redis://redis:6379/1
    ports:
      - "${HTTP_PORT}:8000"
    volumes:
      - data:/var/lib/mayan

  redis:
    image: ${REDIS_IMAGE}
    container_name: ${CONTAINER_NAME}-redis
    restart: unless-stopped

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      POSTGRES_DB: mayan
      POSTGRES_USER: mayan
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - db:/var/lib/postgresql/data

volumes:
  data:
  db:
""",
    ),
    App(
        key="teedy", name="Teedy", category="productivity",
        vi="Quản lý tài liệu gọn nhẹ, có nhận dạng chữ và chia sẻ theo thẻ.",
        en="Light document management with OCR and tag-based sharing.",
        website="https://teedy.io",
        image="sismics/docs", tag_pages=6, tag_any_suffix=True,
        container_port=8080, volumes=("data:/data",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 8607),),
    ),
    App(
        key="onlyoffice", name="ONLYOFFICE Docs", category="productivity",
        vi="Bộ soạn thảo Word, Excel và PowerPoint chạy trên máy chủ, nhiều người sửa cùng lúc.",
        en="A server-side Word, Excel and PowerPoint editor with real-time co-editing.",
        website="https://onlyoffice.com",
        image="onlyoffice/documentserver", tag_pages=6, min_major=7,
        container_port=80,
        volumes=("data:/var/www/onlyoffice/Data", "logs:/var/log/onlyoffice"),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8608,
                 help_vi="ONLYOFFICE cần khoảng 4 GB RAM và vài phút cho lần khởi động đầu.",
                 help_en="ONLYOFFICE wants around 4 GB of RAM and a few minutes on first boot."),
            password("JWT_SECRET", "Khóa ký yêu cầu", "Request signing secret"),
        ),
        environment={"JWT_ENABLED": "true", "JWT_SECRET": "${JWT_SECRET}"},
    ),
    App(
        key="etherpad", name="Etherpad", category="productivity",
        vi="Trang soạn thảo chung theo thời gian thực, mở liên kết là gõ được ngay.",
        en="A real-time shared editor: open the link and start typing.",
        website="https://etherpad.org",
        image="etherpad/etherpad", tag_pages=6, min_major=1,
        container_port=9001, volumes=("data:/opt/etherpad-lite/var",),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 9001),
            text("ADMIN_USER", "Tài khoản quản trị", "Admin user", "admin"),
            password("ADMIN_PASSWORD", "Mật khẩu quản trị", "Admin password"),
        ),
        environment={"ADMIN_PASSWORD": "${ADMIN_PASSWORD}", "DB_TYPE": "sqlite",
                     "DB_FILENAME": "/opt/etherpad-lite/var/etherpad.db"},
    ),
    App(
        key="hedgedoc", name="HedgeDoc", category="productivity",
        vi="Soạn Markdown cùng lúc nhiều người, xem trước ngay bên cạnh.",
        en="Collaborative Markdown editing with a live preview beside it.",
        website="https://hedgedoc.org",
        image="quay.io/hedgedoc/hedgedoc", registry="dockerhub",
        fixed_tags=("1.10.3", "1.9.9"),
        side_images=(PG,), version_values={"DB_IMAGE": PG},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 3401),
            text("DOMAIN_URL", "Địa chỉ trang", "Site URL", "http://localhost:3401"),
            password("SESSION_SECRET", "Khóa ký phiên", "Session secret"),
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
      CMD_DB_URL: postgres://hedgedoc:${DB_PASSWORD}@db:5432/hedgedoc
      CMD_DOMAIN: ${DOMAIN_URL}
      CMD_URL_ADDPORT: "true"
      CMD_SESSION_SECRET: ${SESSION_SECRET}
      CMD_ALLOW_ANONYMOUS: "false"
    ports:
      - "${HTTP_PORT}:3000"
    volumes:
      - uploads:/hedgedoc/public/uploads

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      POSTGRES_DB: hedgedoc
      POSTGRES_USER: hedgedoc
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - db:/var/lib/postgresql/data

volumes:
  uploads:
  db:
""",
    ),
    App(
        key="cryptpad", name="CryptPad", category="productivity",
        vi="Bộ soạn thảo chung mã hóa ngay trên trình duyệt — máy chủ không đọc được nội dung.",
        en="A collaborative editor suite encrypted in the browser — the server cannot read the content.",
        website="https://cryptpad.org",
        image="cryptpad/cryptpad", tag_prefix="version-", tag_pages=6, min_major=2024,
        container_port=3000,
        volumes=("blob:/cryptpad/blob", "block:/cryptpad/block", "data:/cryptpad/data",
                 "datastore:/cryptpad/datastore"),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 3402),
            text("MAIN_DOMAIN", "Tên miền chính", "Main domain", "http://localhost:3402"),
        ),
        environment={"CPAD_MAIN_DOMAIN": "${MAIN_DOMAIN}"},
    ),
    App(
        key="passbolt", name="Passbolt", category="security",
        vi="Kho mật khẩu cho nhóm: chia sẻ theo từng người, có nhật ký ai xem gì.",
        en="A team password vault: share per person, with a log of who looked at what.",
        website="https://passbolt.com",
        image="passbolt/passbolt", tag_suffix="-1-ce", tag_pages=8, min_major=3,
        side_images=(MARIADB,), version_values={"DB_IMAGE": MARIADB},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8609),
            text("APP_FULL_BASE_URL", "Địa chỉ trang", "Site URL", "http://localhost:8609"),
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
      APP_FULL_BASE_URL: ${APP_FULL_BASE_URL}
      DATASOURCES_DEFAULT_HOST: db
      DATASOURCES_DEFAULT_DATABASE: passbolt
      DATASOURCES_DEFAULT_USERNAME: passbolt
      DATASOURCES_DEFAULT_PASSWORD: ${DB_PASSWORD}
    ports:
      - "${HTTP_PORT}:80"
    volumes:
      - gpg:/etc/passbolt/gpg
      - jwt:/etc/passbolt/jwt

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      MARIADB_DATABASE: passbolt
      MARIADB_USER: passbolt
      MARIADB_PASSWORD: ${DB_PASSWORD}
      MARIADB_RANDOM_ROOT_PASSWORD: "1"
    volumes:
      - db:/var/lib/mysql

volumes:
  gpg:
  jwt:
  db:
""",
    ),
]
