"""Mô tả ứng dụng và dựng tệp YAML cho danh mục.

Danh mục có hàng trăm ứng dụng mà tuyệt đại đa số giống nhau về cấu trúc: một
container, một cổng, một volume, vài biến môi trường. Viết tay từng tệp YAML là
cách chắc chắn nhất để chúng lệch nhau — chỗ này quên `restart: unless-stopped`,
chỗ kia đặt tên volume khác. Mô tả ứng dụng bằng dữ liệu rồi sinh ra YAML giữ
cho cả danh mục nhất quán, và sửa một thói quen chung chỉ cần sửa một chỗ.
"""

from dataclasses import dataclass, field as dc_field

# Thứ tự khóa trong tệp sinh ra, giữ cố định để bản khác biệt giữa hai lần sinh
# chỉ chứa thay đổi thật.
CATEGORIES = (
    "website",
    "development",
    "monitoring",
    "automation",
    "database",
    "media",
    "tool",
    "storage",
    "security",
    "productivity",
)


@dataclass
class Field:
    """Một ô trong biểu mẫu cài đặt."""

    key: str
    vi: str
    en: str
    type: str = "text"
    default: str = ""
    required: bool = False
    generate: bool = False
    help_vi: str = ""
    help_en: str = ""
    options: tuple = ()


def port(key, vi, en, default, help_vi="", help_en=""):
    return Field(key, vi, en, "port", str(default), required=True,
                 help_vi=help_vi, help_en=help_en)


def password(key="PASSWORD", vi="Mật khẩu", en="Password", help_vi="", help_en=""):
    return Field(key, vi, en, "password", generate=True, help_vi=help_vi, help_en=help_en)


def text(key, vi, en, default="", required=True, help_vi="", help_en=""):
    return Field(key, vi, en, "text", default, required=required,
                 help_vi=help_vi, help_en=help_en)


def choice(key, vi, en, options, default):
    return Field(key, vi, en, "select", default, options=options)


@dataclass
class App:
    """Một ứng dụng trong danh mục."""

    key: str
    name: str
    vi: str
    en: str
    category: str
    website: str

    # Kho image và cách chọn thẻ phiên bản.
    image: str
    registry: str = "dockerhub"
    tag_suffix: str = ""
    tag_prefix: str = ""
    # Chấp nhận thẻ có hậu tố bất kỳ, cho kho gắn thêm mã băm vào sau số phiên bản.
    tag_any_suffix: bool = False
    tag_pages: int = 8
    tag_count: int = 3
    min_major: int = 0
    # Thẻ tự khai khi kho không dùng đánh số phiên bản (chỉ có "latest").
    fixed_tags: tuple = ()

    # Cổng bên trong container mà cổng người dùng chọn sẽ trỏ tới.
    container_port: int = 80
    default_port: int = 8080
    port_label_vi: str = "Cổng web"
    port_label_en: str = "Web port"

    fields: tuple = ()
    environment: dict = dc_field(default_factory=dict)
    volumes: tuple = ("data:/data",)
    extra_ports: tuple = ()
    command: str = ""
    # Dịch vụ phụ (cơ sở dữ liệu…) chèn nguyên văn vào phần services.
    extra_services: str = ""
    extra_volumes: tuple = ()
    extra_top_level: str = ""
    # Khuôn compose viết tay, dùng khi ứng dụng không vừa khuôn chung.
    compose: str = ""
    # Giá trị biến do phiên bản quyết định, ngoài IMAGE.
    version_values: dict = dc_field(default_factory=dict)
    # Image cố định mọi phiên bản đều tải (cơ sở dữ liệu đi kèm).
    side_images: tuple = ()
    note_vi: str = ""
    note_en: str = ""


def _quote(value):
    """Bọc chuỗi trong ngoặc kép, thoát ký tự cần thoát."""
    return '"' + value.replace("\\", "\\\\").replace('"', '\\"') + '"'


def _render_fields(app):
    lines = ["fields:"]
    for f in app.fields:
        lines.append(f"  - key: {f.key}")
        lines.append(f"    label: {{vi: {f.vi}, en: {f.en}}}")
        if f.help_vi:
            lines.append("    help:")
            lines.append(f"      vi: {_quote(f.help_vi)}")
            lines.append(f"      en: {_quote(f.help_en)}")
        lines.append(f"    type: {f.type}")
        if f.default:
            lines.append(f'    default: "{f.default}"')
        if f.required:
            lines.append("    required: true")
        if f.generate:
            lines.append("    generate: true")
        if f.options:
            lines.append("    options:")
            for value, vi, en in f.options:
                lines.append(f'      - value: "{value}"')
                lines.append(f"        label: {{vi: {vi}, en: {en}}}")
    return lines


def build_compose(app):
    """Dựng khuôn compose từ mô tả, hoặc trả lại khuôn viết tay."""
    if app.compose:
        return app.compose.strip("\n")

    lines = ["services:", "  app:", "    image: ${IMAGE}",
             "    container_name: ${CONTAINER_NAME}", "    restart: unless-stopped"]
    if app.command:
        lines.append(f"    command: {app.command}")
    if app.environment:
        lines.append("    environment:")
        for key, value in app.environment.items():
            lines.append(f'      {key}: "{value}"')

    # Ứng dụng không có ô cổng nào — Cloudflare Tunnel, Watchtower — thì không
    # ánh xạ cổng. Lấy bừa ô đầu tiên sẽ sinh ra "${TUNNEL_TOKEN}:80".
    first = app.fields[0] if app.fields else None
    ports = [f'      - "${{{first.key}}}:{app.container_port}"'] if first and first.type == "port" else []
    ports.extend(f"      - {entry}" for entry in app.extra_ports)
    if ports:
        lines.append("    ports:")
        lines.extend(ports)

    if app.volumes:
        lines.append("    volumes:")
        for volume in app.volumes:
            lines.append(f"      - {volume}")

    if app.extra_services:
        lines.append("")
        lines.append(app.extra_services.strip("\n"))

    named = [v.split(":")[0] for v in app.volumes if not v.startswith(("/", "$"))]
    named.extend(app.extra_volumes)
    if named:
        lines.append("")
        lines.append("volumes:")
        for name in dict.fromkeys(named):
            lines.append(f"  {name}:")

    if app.extra_top_level:
        lines.append("")
        lines.append(app.extra_top_level.strip("\n"))

    return "\n".join(lines)


def render(app, versions):
    """Dựng nội dung tệp YAML của một ứng dụng.

    `versions` là danh sách thẻ image thật, mới nhất trước.
    """
    out = [f"key: {app.key}", "name:", f"  vi: {app.name}", f"  en: {app.name}",
           "description:", f"  vi: {_quote(app.vi)}", f"  en: {_quote(app.en)}",
           f"category: {app.category}", f"website: {app.website}"]

    # portField dùng để dựng liên kết mở ứng dụng, nên chỉ đặt khi có cổng thật.
    if app.fields and app.fields[0].type == "port":
        out.append(f"portField: {app.fields[0].key}")

    out.append("versions:")
    for tag in versions:
        # Nhãn phiên bản bỏ hậu tố biến thể: người dùng chọn giữa "6.4" và
        # "5.117", cái đuôi "-alpine" chỉ là chi tiết đóng gói.
        label = tag
        if app.tag_prefix and label.startswith(app.tag_prefix):
            label = label[len(app.tag_prefix):]
        if app.tag_suffix and label.endswith(app.tag_suffix):
            label = label[: -len(app.tag_suffix)]
        if len(label) > 1 and label[0] == "v" and label[1].isdigit():
            label = label[1:]
        images = [f"{app.image}:{tag}", *app.side_images]

        out.append(f'  - name: "{label}"')

        # Vài dự án chỉ phát hành đúng thẻ "latest". Nói thẳng điều đó ra: cài
        # lại sau ba tháng sẽ ra một bản khác, và người dùng cần biết trước.
        note_vi, note_en = app.note_vi, app.note_en
        if label == "latest" and not note_vi:
            note_vi = "Dự án chỉ phát hành thẻ latest, nên mỗi lần cài lại sẽ lấy bản mới nhất lúc đó."
            note_en = "The project only publishes a latest tag, so each install pulls whatever is newest then."
        if note_vi:
            out.append("    note:")
            out.append(f"      vi: {_quote(note_vi)}")
            out.append(f"      en: {_quote(note_en)}")
        out.append("    images:")
        out.extend(f"      - {image}" for image in images)
        out.append("    values:")
        out.append(f'      IMAGE: "{app.image}:{tag}"')
        for key, value in app.version_values.items():
            out.append(f'      {key}: "{value}"')

    out.extend(_render_fields(app))

    out.append("compose: |")
    for line in build_compose(app).split("\n"):
        out.append(("  " + line).rstrip())

    return "\n".join(out) + "\n"
