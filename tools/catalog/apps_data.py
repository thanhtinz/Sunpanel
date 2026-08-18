"""Nhóm ứng dụng dữ liệu: tìm kiếm, cơ sở dữ liệu, phân tích và CI."""

from model import App, password, port, text

PG = "postgres:16-alpine"
REDIS = "redis:7-alpine"

APPS = [
    App(
        key="meilisearch", name="Meilisearch", category="database",
        vi="Máy tìm kiếm cho ứng dụng: gõ tới đâu ra kết quả tới đó, chịu được cả lỗi chính tả.",
        en="A search engine for your app: results as you type, typo-tolerant out of the box.",
        website="https://meilisearch.com",
        image="getmeili/meilisearch", tag_pages=6, min_major=1,
        container_port=7700, volumes=("data:/meili_data",),
        fields=(
            port("HTTP_PORT", "Cổng", "Port", 7700),
            password("MASTER_KEY", "Khóa chủ", "Master key",
                     help_vi="Mọi truy vấn đều cần khóa này; giữ kín như mật khẩu.",
                     help_en="Every request needs this key; keep it as secret as a password."),
        ),
        environment={"MEILI_MASTER_KEY": "${MASTER_KEY}", "MEILI_ENV": "production"},
    ),
    App(
        key="typesense", name="Typesense", category="database",
        vi="Máy tìm kiếm nhanh, nhẹ RAM, cấu hình đơn giản hơn Elasticsearch rất nhiều.",
        en="A fast search engine that is light on memory and far simpler to run than Elasticsearch.",
        website="https://typesense.org",
        image="typesense/typesense", tag_pages=6, min_major=0,
        container_port=8108, volumes=("data:/data",),
        command="--data-dir /data --api-key=${API_KEY} --enable-cors",
        fields=(
            port("HTTP_PORT", "Cổng", "Port", 8108),
            password("API_KEY", "Khóa API", "API key"),
        ),
    ),
    App(
        key="opensearch", name="OpenSearch", category="database",
        vi="Bộ tìm kiếm và phân tích nhật ký quy mô lớn, nhánh nguồn mở của Elasticsearch.",
        en="Large-scale search and log analytics — the open-source fork of Elasticsearch.",
        website="https://opensearch.org",
        image="opensearchproject/opensearch", tag_pages=6, min_major=2,
        container_port=9200, volumes=("data:/usr/share/opensearch/data",),
        fields=(
            port("HTTP_PORT", "Cổng", "Port", 9200,
                 help_vi="OpenSearch cần ít nhất 2 GB RAM riêng cho nó.",
                 help_en="OpenSearch wants at least 2 GB of memory to itself."),
            password("ADMIN_PASSWORD", "Mật khẩu quản trị", "Admin password"),
        ),
        environment={"discovery.type": "single-node",
                     "OPENSEARCH_INITIAL_ADMIN_PASSWORD": "${ADMIN_PASSWORD}",
                     "bootstrap.memory_lock": "true"},
    ),
    App(
        key="clickhouse", name="ClickHouse", category="database",
        vi="Cơ sở dữ liệu cột cho báo cáo: quét hàng tỉ dòng trong vài giây.",
        en="A columnar database for analytics: scans billions of rows in seconds.",
        website="https://clickhouse.com",
        image="clickhouse/clickhouse-server", tag_pages=6, min_major=23,
        container_port=8123, volumes=("data:/var/lib/clickhouse",),
        extra_ports=('"${NATIVE_PORT}:9000"',),
        fields=(
            port("HTTP_PORT", "Cổng HTTP", "HTTP port", 8124),
            port("NATIVE_PORT", "Cổng gốc", "Native port", 9010),
            text("DB_USER", "Tên người dùng", "User name", "app"),
            password("DB_PASSWORD", "Mật khẩu", "Password"),
        ),
        environment={"CLICKHOUSE_USER": "${DB_USER}", "CLICKHOUSE_PASSWORD": "${DB_PASSWORD}",
                     "CLICKHOUSE_DEFAULT_ACCESS_MANAGEMENT": "1"},
    ),
    App(
        key="influxdb", name="InfluxDB", category="database",
        vi="Cơ sở dữ liệu chuỗi thời gian cho số liệu cảm biến và chỉ số hệ thống.",
        en="A time-series database for sensor readings and system metrics.",
        website="https://influxdata.com",
        image="influxdb", tag_pages=6, min_major=2,
        container_port=8086, volumes=("data:/var/lib/influxdb2",),
        fields=(
            port("HTTP_PORT", "Cổng", "Port", 8086),
            text("ADMIN_USER", "Tài khoản quản trị", "Admin user", "admin"),
            password("ADMIN_PASSWORD", "Mật khẩu quản trị", "Admin password"),
            text("ORG_NAME", "Tên tổ chức", "Organisation", "sunpanel"),
            text("BUCKET_NAME", "Tên kho dữ liệu", "Bucket", "metrics"),
        ),
        environment={"DOCKER_INFLUXDB_INIT_MODE": "setup",
                     "DOCKER_INFLUXDB_INIT_USERNAME": "${ADMIN_USER}",
                     "DOCKER_INFLUXDB_INIT_PASSWORD": "${ADMIN_PASSWORD}",
                     "DOCKER_INFLUXDB_INIT_ORG": "${ORG_NAME}",
                     "DOCKER_INFLUXDB_INIT_BUCKET": "${BUCKET_NAME}"},
    ),
    App(
        key="questdb", name="QuestDB", category="database",
        vi="Cơ sở dữ liệu chuỗi thời gian truy vấn bằng SQL, nạp dữ liệu rất nhanh.",
        en="A time-series database you query with plain SQL and can write to very fast.",
        website="https://questdb.io",
        image="questdb/questdb", tag_pages=6, min_major=7,
        container_port=9000, volumes=("data:/var/lib/questdb",),
        extra_ports=('"${PG_PORT}:8812"',),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 9020),
            port("PG_PORT", "Cổng giao thức PostgreSQL", "PostgreSQL wire port", 8812),
        ),
    ),
    App(
        key="neo4j", name="Neo4j", category="database",
        vi="Cơ sở dữ liệu đồ thị: lưu quan hệ giữa các thực thể và truy vấn theo đường đi.",
        en="A graph database: stores relationships between entities and queries along paths.",
        website="https://neo4j.com",
        image="neo4j", tag_pages=6, min_major=5,
        container_port=7474, volumes=("data:/data", "logs:/logs"),
        extra_ports=('"${BOLT_PORT}:7687"',),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 7474),
            port("BOLT_PORT", "Cổng Bolt", "Bolt port", 7687),
            password("DB_PASSWORD", "Mật khẩu", "Password"),
        ),
        environment={"NEO4J_AUTH": "neo4j/${DB_PASSWORD}"},
    ),
    App(
        key="couchdb", name="CouchDB", category="database",
        vi="Cơ sở dữ liệu tài liệu đồng bộ được với thiết bị ngoại tuyến rồi gộp lại sau.",
        en="A document database that syncs with offline devices and merges afterwards.",
        website="https://couchdb.apache.org",
        image="couchdb", tag_pages=6, min_major=3,
        container_port=5984, volumes=("data:/opt/couchdb/data",),
        fields=(
            port("HTTP_PORT", "Cổng", "Port", 5984),
            text("DB_USER", "Tài khoản quản trị", "Admin user", "admin"),
            password("DB_PASSWORD", "Mật khẩu", "Password"),
        ),
        environment={"COUCHDB_USER": "${DB_USER}", "COUCHDB_PASSWORD": "${DB_PASSWORD}"},
    ),
    App(
        key="consul", name="Consul", category="tool",
        vi="Sổ đăng ký dịch vụ và kho cấu hình cho hệ thống nhiều máy chủ.",
        en="A service registry and configuration store for multi-server systems.",
        website="https://consul.io",
        image="hashicorp/consul", tag_pages=6, min_major=1,
        container_port=8500, volumes=("data:/consul/data",),
        command="agent -server -bootstrap-expect=1 -ui -client=0.0.0.0",
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 8500),),
    ),
    App(
        key="vault", name="HashiCorp Vault", category="security",
        vi="Kho bí mật cho hạ tầng: mật khẩu, khóa API và chứng chỉ, có kiểm toán và hạn dùng.",
        en="A secrets store for infrastructure — passwords, API keys and certificates, with audit and expiry.",
        website="https://vaultproject.io",
        image="hashicorp/vault", tag_pages=6, min_major=1,
        container_port=8200, volumes=("file:/vault/file", "logs:/vault/logs"),
        fields=(
            port("HTTP_PORT", "Cổng", "Port", 8200,
                 help_vi="Bản dựng sẵn chạy ở chế độ phát triển, dữ liệu nằm trong RAM. Đọc tài liệu trước khi dùng thật.",
                 help_en="This template runs in dev mode with data in memory. Read the docs before trusting it with real secrets."),
            password("ROOT_TOKEN", "Vé quản trị", "Root token"),
        ),
        environment={"VAULT_DEV_ROOT_TOKEN_ID": "${ROOT_TOKEN}",
                     "VAULT_DEV_LISTEN_ADDRESS": "0.0.0.0:8200"},
    ),
    App(
        key="nocodb", name="NocoDB", category="productivity",
        vi="Biến cơ sở dữ liệu thành bảng tính có giao diện, làm được ứng dụng nội bộ không cần lập trình.",
        en="Turns a database into a spreadsheet UI — internal apps without writing code.",
        website="https://nocodb.com",
        image="nocodb/nocodb", tag_pages=6, min_major=0,
        container_port=8080, volumes=("data:/usr/app/data",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 8110),),
    ),
    App(
        key="baserow", name="Baserow", category="productivity",
        vi="Cơ sở dữ liệu dạng bảng tính cho nhóm, thay Airtable mà dữ liệu ở lại máy chủ của bạn.",
        en="A spreadsheet-style database for teams — an Airtable replacement with your data on your server.",
        website="https://baserow.io",
        image="baserow/baserow", tag_pages=6, min_major=1,
        container_port=80, volumes=("data:/baserow/data",),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8111),
            text("PUBLIC_URL", "Địa chỉ trang", "Site URL", "http://localhost:8111"),
        ),
        environment={"BASEROW_PUBLIC_URL": "${PUBLIC_URL}"},
    ),
    App(
        key="directus", name="Directus", category="development",
        vi="Bảng quản trị và API tự sinh cho cơ sở dữ liệu SQL sẵn có của bạn.",
        en="An admin panel and instant API on top of your existing SQL database.",
        website="https://directus.io",
        image="directus/directus", tag_pages=6, min_major=10,
        side_images=(PG,), version_values={"DB_IMAGE": PG},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8112),
            text("ADMIN_EMAIL", "Email quản trị", "Admin email", "admin@example.com"),
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
    environment:
      SECRET: ${SECRET_KEY}
      ADMIN_EMAIL: ${ADMIN_EMAIL}
      ADMIN_PASSWORD: ${ADMIN_PASSWORD}
      DB_CLIENT: pg
      DB_HOST: db
      DB_PORT: "5432"
      DB_DATABASE: directus
      DB_USER: directus
      DB_PASSWORD: ${DB_PASSWORD}
    ports:
      - "${HTTP_PORT}:8055"
    volumes:
      - uploads:/directus/uploads

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      POSTGRES_DB: directus
      POSTGRES_USER: directus
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - db:/var/lib/postgresql/data

volumes:
  uploads:
  db:
""",
    ),
    App(
        key="pocketbase", name="PocketBase", category="development",
        vi="Máy chủ hậu cần gói trong một tệp: cơ sở dữ liệu, đăng nhập, tệp và API thời gian thực.",
        en="A backend in a single file: database, auth, file storage and a realtime API.",
        website="https://pocketbase.io",
        image="ghcr.io/muchobien/pocketbase", registry="ghcr", min_major=0,
        container_port=8090, volumes=("data:/pb_data",),
        fields=(port("HTTP_PORT", "Cổng", "Port", 8113),),
    ),
    App(
        key="budibase", name="Budibase", category="development",
        vi="Dựng ứng dụng nội bộ bằng kéo thả, nối thẳng vào cơ sở dữ liệu và API sẵn có.",
        en="Builds internal apps by dragging and dropping, wired straight into your databases and APIs.",
        website="https://budibase.com",
        image="budibase/budibase", tag_pages=6, min_major=2,
        container_port=80, volumes=("data:/data",),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8114),
            password("JWT_SECRET", "Khóa ký phiên", "Session secret"),
        ),
        environment={"JWT_SECRET": "${JWT_SECRET}"},
    ),
    App(
        key="windmill", name="Windmill", category="automation",
        vi="Biến script Python, TypeScript hay Bash thành công việc có lịch chạy và biểu mẫu.",
        en="Turns Python, TypeScript or Bash scripts into scheduled jobs with generated forms.",
        website="https://windmill.dev",
        image="ghcr.io/windmill-labs/windmill", registry="ghcr", min_major=1,
        side_images=(PG,), version_values={"DB_IMAGE": PG},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8115),
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
      DATABASE_URL: postgres://windmill:${DB_PASSWORD}@db/windmill?sslmode=disable
      MODE: standalone
    ports:
      - "${HTTP_PORT}:8000"
    volumes:
      - cache:/tmp/windmill/cache

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      POSTGRES_DB: windmill
      POSTGRES_USER: windmill
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - db:/var/lib/postgresql/data

volumes:
  cache:
  db:
""",
    ),
    App(
        key="kestra", name="Kestra", category="automation",
        vi="Điều phối luồng dữ liệu khai báo bằng YAML, có giao diện xem lịch sử từng bước.",
        en="Declarative data-pipeline orchestration in YAML, with a UI that shows every step's history.",
        website="https://kestra.io",
        image="kestra/kestra", tag_pages=6, min_major=0,
        container_port=8080,
        command="server standalone",
        volumes=("data:/app/storage", "/var/run/docker.sock:/var/run/docker.sock"),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 8116),),
    ),
    App(
        key="activepieces", name="Activepieces", category="automation",
        vi="Nối các dịch vụ lại thành quy trình tự động, giống n8n nhưng giao diện gọn hơn.",
        en="Wires services into automated flows — like n8n with a tighter interface.",
        website="https://activepieces.com",
        image="activepieces/activepieces", tag_pages=6, min_major=0,
        side_images=(PG, REDIS),
        version_values={"DB_IMAGE": PG, "REDIS_IMAGE": REDIS},
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8117),
            password("ENCRYPTION_KEY", "Khóa mã hóa", "Encryption key"),
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
      - redis
    environment:
      AP_ENCRYPTION_KEY: ${ENCRYPTION_KEY}
      AP_JWT_SECRET: ${JWT_SECRET}
      AP_POSTGRES_DATABASE: activepieces
      AP_POSTGRES_HOST: db
      AP_POSTGRES_PORT: "5432"
      AP_POSTGRES_USERNAME: activepieces
      AP_POSTGRES_PASSWORD: ${DB_PASSWORD}
      AP_REDIS_HOST: redis
      AP_REDIS_PORT: "6379"
      AP_FRONTEND_URL: http://localhost
    ports:
      - "${HTTP_PORT}:80"

  redis:
    image: ${REDIS_IMAGE}
    container_name: ${CONTAINER_NAME}-redis
    restart: unless-stopped

  db:
    image: ${DB_IMAGE}
    container_name: ${CONTAINER_NAME}-db
    restart: unless-stopped
    environment:
      POSTGRES_DB: activepieces
      POSTGRES_USER: activepieces
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - db:/var/lib/postgresql/data

volumes:
  db:
""",
    ),
    App(
        key="huginn", name="Huginn", category="automation",
        vi="Đặt các tác nhân theo dõi web và tự hành động thay bạn — kiểu IFTTT tự quản.",
        en="Builds agents that watch the web and act for you — self-hosted IFTTT.",
        website="https://github.com/huginn/huginn",
        image="huginn/huginn", fixed_tags=("latest",),
        container_port=3000, volumes=("data:/var/lib/mysql",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 8118),),
    ),
    App(
        key="sonarqube", name="SonarQube", category="development",
        vi="Soi chất lượng mã: lỗi tiềm ẩn, lỗ hổng bảo mật và phần mã trùng lặp.",
        en="Inspects code quality: latent bugs, security holes and duplicated blocks.",
        website="https://sonarqube.org",
        image="sonarqube", tag_suffix="-community", tag_pages=8, min_major=9,
        container_port=9000,
        volumes=("data:/opt/sonarqube/data", "extensions:/opt/sonarqube/extensions",
                 "logs:/opt/sonarqube/logs"),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 9030,
                     help_vi="SonarQube cần ít nhất 2 GB RAM và vm.max_map_count từ 262144 trở lên.",
                     help_en="SonarQube needs at least 2 GB of RAM and vm.max_map_count of 262144 or more."),),
    ),
    App(
        key="jenkins", name="Jenkins", category="development",
        vi="Máy chạy CI/CD lâu đời nhất, có hàng nghìn plugin cho mọi thứ bạn cần nối vào.",
        en="The oldest CI/CD server, with thousands of plugins for anything you need to connect.",
        website="https://jenkins.io",
        image="jenkins/jenkins", tag_suffix="-lts-jdk21", tag_pages=8, min_major=2,
        container_port=8080, volumes=("data:/var/jenkins_home",),
        extra_ports=('"${AGENT_PORT}:50000"',),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8119),
            port("AGENT_PORT", "Cổng máy chạy lệnh", "Agent port", 50000),
        ),
    ),
    App(
        key="woodpecker-ci", name="Woodpecker CI", category="development",
        vi="Máy chạy CI/CD gọn nhẹ cho Gitea, GitHub và GitLab, cấu hình bằng một tệp YAML.",
        en="A lightweight CI/CD server for Gitea, GitHub and GitLab, configured by one YAML file.",
        website="https://woodpecker-ci.org",
        image="woodpeckerci/woodpecker-server", tag_pages=6, min_major=2,
        container_port=8000, volumes=("data:/var/lib/woodpecker",),
        fields=(
            port("HTTP_PORT", "Cổng web", "Web port", 8120),
            text("HOST_URL", "Địa chỉ trang", "Site URL", "http://localhost:8120"),
            text("GITEA_URL", "Địa chỉ Gitea", "Gitea URL", "http://localhost:3000"),
            text("CLIENT_ID", "Client ID của ứng dụng OAuth", "OAuth client ID", ""),
            password("CLIENT_SECRET", "Client secret", "Client secret"),
            password("AGENT_SECRET", "Khóa cho máy chạy lệnh", "Agent shared secret"),
        ),
        environment={"WOODPECKER_HOST": "${HOST_URL}",
                     "WOODPECKER_GITEA": "true",
                     "WOODPECKER_GITEA_URL": "${GITEA_URL}",
                     "WOODPECKER_GITEA_CLIENT": "${CLIENT_ID}",
                     "WOODPECKER_GITEA_SECRET": "${CLIENT_SECRET}",
                     "WOODPECKER_AGENT_SECRET": "${AGENT_SECRET}"},
    ),
    App(
        key="nexus", name="Sonatype Nexus", category="development",
        vi="Kho lưu gói dùng chung cho Maven, npm, Docker và nhiều loại khác.",
        en="An artifact repository for Maven, npm, Docker and many other package types.",
        website="https://sonatype.com/products/nexus-repository",
        image="sonatype/nexus3", tag_pages=6, min_major=3,
        container_port=8081, volumes=("data:/nexus-data",),
        fields=(port("HTTP_PORT", "Cổng web", "Web port", 8121,
                     help_vi="Nexus cần khoảng 2 GB RAM; mật khẩu quản trị lần đầu nằm trong tệp admin.password.",
                     help_en="Nexus needs around 2 GB of RAM; the first admin password lands in admin.password."),),
    ),
    App(
        key="renovate", name="Renovate", category="development",
        vi="Tự mở pull request cập nhật thư viện cho các kho mã của bạn.",
        en="Opens dependency-update pull requests on your repositories automatically.",
        website="https://renovatebot.com",
        image="renovate/renovate", tag_pages=6, min_major=37,
        volumes=(),
        fields=(
            text("PLATFORM", "Nền tảng", "Platform", "gitea"),
            text("ENDPOINT", "Địa chỉ máy chủ Git", "Git server URL", "http://localhost:3000/api/v1/"),
            password("TOKEN", "Vé truy cập", "Access token"),
            text("REPOSITORIES", "Danh sách kho mã", "Repositories", "user/repo"),
        ),
        environment={"RENOVATE_PLATFORM": "${PLATFORM}", "RENOVATE_ENDPOINT": "${ENDPOINT}",
                     "RENOVATE_TOKEN": "${TOKEN}", "RENOVATE_REPOSITORIES": "${REPOSITORIES}"},
    ),
]
