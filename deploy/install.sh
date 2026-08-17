#!/bin/sh
# Trình cài đặt SunPanel.
#
# Dùng /bin/sh chứ không phải bash: một số bản Linux tối giản (Alpine, container
# gốc) không có sẵn bash, mà trình cài đặt thì phải chạy được ở mọi nơi.
#
#   curl -fsSL https://raw.githubusercontent.com/thanhtinz/Sunpanel/main/deploy/install.sh | sh

set -eu

REPO="thanhtinz/Sunpanel"
BINARY="sunpanel"
INSTALL_DIR="${SUNPANEL_INSTALL_DIR:-/usr/local/bin}"
DATA_DIR="${SUNPANEL_DATA_DIR:-/opt/sunpanel}"
SERVICE_NAME="sunpanel"
VERSION="${SUNPANEL_VERSION:-latest}"

info()  { printf '\033[0;36m==>\033[0m %s\n' "$1"; }
ok()    { printf '\033[0;32m✓\033[0m %s\n' "$1"; }
warn()  { printf '\033[0;33m!\033[0m %s\n' "$1"; }
fatal() { printf '\033[0;31m✗\033[0m %s\n' "$1" >&2; exit 1; }

require_root() {
	[ "$(id -u)" = "0" ] || fatal "Vui lòng chạy trình cài đặt với quyền root (sudo)."
}

detect_platform() {
	os=$(uname -s | tr '[:upper:]' '[:lower:]')
	case "$os" in
		linux|darwin) ;;
		*) fatal "Hệ điều hành $os chưa được hỗ trợ bởi trình cài đặt này." ;;
	esac

	case "$(uname -m)" in
		x86_64|amd64)   arch="amd64" ;;
		aarch64|arm64)  arch="arm64" ;;
		armv7l|armv6l)  arch="arm" ;;
		*) fatal "Kiến trúc $(uname -m) chưa được hỗ trợ." ;;
	esac

	PLATFORM="${os}-${arch}"
}

# Ưu tiên curl, lùi về wget — máy chủ tối giản thường chỉ có một trong hai.
download() {
	url="$1"; dest="$2"
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL "$url" -o "$dest"
	elif command -v wget >/dev/null 2>&1; then
		wget -qO "$dest" "$url"
	else
		fatal "Cần có curl hoặc wget để tải về."
	fi
}

resolve_url() {
	if [ "$VERSION" = "latest" ]; then
		echo "https://github.com/${REPO}/releases/latest/download/${BINARY}-${PLATFORM}"
	else
		echo "https://github.com/${REPO}/releases/download/${VERSION}/${BINARY}-${PLATFORM}"
	fi
}

install_binary() {
	url=$(resolve_url)
	tmp=$(mktemp)
	# Xóa tệp tạm dù cài đặt thành công hay thất bại giữa chừng.
	trap 'rm -f "$tmp"' EXIT

	info "Đang tải $BINARY cho $PLATFORM…"
	download "$url" "$tmp" || fatal "Không tải được từ $url"

	install -d "$INSTALL_DIR"
	install -m 0755 "$tmp" "${INSTALL_DIR}/${BINARY}"
	ok "Đã cài vào ${INSTALL_DIR}/${BINARY}"
}

install_service() {
	command -v systemctl >/dev/null 2>&1 || {
		warn "Không tìm thấy systemd — hãy tự cấu hình cách chạy nền."
		warn "Chạy thủ công bằng: ${INSTALL_DIR}/${BINARY} serve"
		return 0
	}

	info "Đang tạo dịch vụ systemd…"
	cat > "/etc/systemd/system/${SERVICE_NAME}.service" <<UNIT
[Unit]
Description=SunPanel — bảng điều khiển quản trị máy chủ
Documentation=https://github.com/${REPO}
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/${BINARY} serve
Restart=always
RestartSec=5
Environment=SUNPANEL_DATA_DIR=${DATA_DIR}

# Panel cần quyền root để quản trị máy chủ, nhưng vẫn siết được các bề mặt
# không liên quan tới công việc của nó.
NoNewPrivileges=false
PrivateTmp=true
ProtectHome=false
ProtectSystem=false

LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
UNIT

	systemctl daemon-reload
	systemctl enable "${SERVICE_NAME}" >/dev/null 2>&1
	ok "Đã tạo dịch vụ ${SERVICE_NAME}"
}

open_firewall() {
	port="${SUNPANEL_PORT:-9527}"
	if command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -q "^Status: active"; then
		ufw allow "${port}/tcp" >/dev/null 2>&1 && ok "Đã mở cổng ${port} trên ufw"
	elif command -v firewall-cmd >/dev/null 2>&1 && firewall-cmd --state >/dev/null 2>&1; then
		firewall-cmd --permanent --add-port="${port}/tcp" >/dev/null 2>&1
		firewall-cmd --reload >/dev/null 2>&1 && ok "Đã mở cổng ${port} trên firewalld"
	else
		warn "Không phát hiện tường lửa đang bật — hãy tự mở cổng ${port} nếu cần."
	fi
}

start_service() {
	command -v systemctl >/dev/null 2>&1 || {
		info "Khởi động lần đầu để sinh thông tin đăng nhập…"
		SUNPANEL_DATA_DIR="$DATA_DIR" "${INSTALL_DIR}/${BINARY}" serve
		return 0
	}

	info "Đang khởi động dịch vụ…"
	systemctl restart "${SERVICE_NAME}"

	# Chờ panel khởi tạo cơ sở dữ liệu và in thông tin đăng nhập ra journal.
	i=0
	while [ "$i" -lt 15 ]; do
		if systemctl is-active --quiet "${SERVICE_NAME}"; then
			break
		fi
		i=$((i + 1))
		sleep 1
	done

	systemctl is-active --quiet "${SERVICE_NAME}" \
		|| fatal "Dịch vụ không khởi động được. Xem chi tiết: journalctl -u ${SERVICE_NAME} -n 50"

	printf '\n'
	ok "SunPanel đang chạy"
	printf '\n'
	# Thông tin đăng nhập chỉ được in một lần duy nhất, ở lần khởi tạo đầu tiên.
	journalctl -u "${SERVICE_NAME}" --no-pager -n 40 2>/dev/null \
		| grep -A 12 'khởi tạo lần đầu' || {
		printf 'Panel đã được cài từ trước. Nếu quên mật khẩu, chạy:\n'
		printf '  %s reset-password -user admin\n' "${INSTALL_DIR}/${BINARY}"
	}
	printf '\n'
	printf 'Các lệnh hữu ích:\n'
	printf '  systemctl status %s      # xem trạng thái\n' "${SERVICE_NAME}"
	printf '  journalctl -u %s -f      # xem log trực tiếp\n' "${SERVICE_NAME}"
	printf '  %s reset-password -user admin\n' "${INSTALL_DIR}/${BINARY}"
	printf '\n'
}

main() {
	printf '\n  ☀  SunPanel — trình cài đặt\n\n'
	require_root
	detect_platform
	install_binary
	install -d -m 0700 "$DATA_DIR"
	install_service
	open_firewall
	start_service
}

main "$@"
