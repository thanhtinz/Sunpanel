package webserver

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/thanhtinz/sunpanel/pkg/host"
)

// commandTimeout là thời gian tối đa cho một lệnh nginx.
const commandTimeout = 30 * time.Second

// Nginx quản lý cấu hình website bằng nginx.
type Nginx struct {
	host host.Host
	// confDir là thư mục chứa cấu hình do panel sinh ra.
	confDir string
	// binary là đường dẫn tới nginx; để trống thì tìm trong PATH.
	binary string
	// ipv6 được dò một lần: khả năng này không đổi giữa chừng, và việc dò tốn
	// một lần thử mở socket.
	ipv6     bool
	ipv6Once sync.Once
	// modernHTTP2 cho biết nginx có nhận chỉ thị "http2 on" hay không.
	modernHTTP2     bool
	modernHTTP2Once sync.Once
}

// http2Directive là phiên bản nginx đầu tiên nhận chỉ thị "http2 on".
//
// Trước 1.25.1, HTTP/2 được bật bằng tham số của chính chỉ thị listen. Dùng sai
// cú pháp không phải là bị bỏ qua mà là nginx từ chối nạp cấu hình, nên phải
// theo đúng phiên bản đang chạy — Ubuntu 22.04, Debian 12 và Rocky 9 đều còn
// dùng nginx cũ hơn mốc này.
var http2Directive = [3]int{1, 25, 1}

var _ Manager = (*Nginx)(nil)

// NewNginx tạo trình quản lý nginx.
//
// confDir nên là một thư mục dành riêng cho panel và được include từ cấu hình
// chính, để cấu hình do quản trị viên tự viết không bị đụng tới.
func NewNginx(h host.Host, confDir string) *Nginx {
	if confDir == "" {
		confDir = "/etc/nginx/conf.d"
	}
	return &Nginx{host: h, confDir: confDir, binary: "nginx"}
}

// Name trả về tên phần mềm máy chủ web.
func (n *Nginx) Name() string { return "nginx" }

// ConfDir trả về thư mục cấu hình đang dùng.
func (n *Nginx) ConfDir() string { return n.confDir }

// Available cho biết nginx có trên máy và chạy được không.
func (n *Nginx) Available(ctx context.Context) bool {
	res, err := n.host.Exec(ctx, host.Command{
		Path: n.binary, Args: []string{"-v"}, Timeout: 5 * time.Second,
	})
	return err == nil && res.OK()
}

// configPath trả về đường dẫn tệp cấu hình của một website.
func (n *Nginx) configPath(name string) string {
	return filepath.Join(n.confDir, "sunpanel-"+name+".conf")
}

// Apply ghi cấu hình cho một website rồi nạp lại nginx.
//
// Cấu hình được kiểm tra trước khi nạp lại, và tệp cũ được khôi phục nếu cấu
// hình mới sai. Nếu không, một lỗi gõ nhầm sẽ khiến nginx từ chối khởi động và
// làm sập mọi website khác trên máy chủ.
func (n *Nginx) Apply(ctx context.Context, site Site) error {
	content, err := n.Render(site)
	if err != nil {
		return err
	}

	path := n.configPath(site.Name)
	previous, hadPrevious := readIfExists(path)

	if err := os.MkdirAll(n.confDir, 0o755); err != nil {
		return fmt.Errorf("tạo thư mục cấu hình: %w", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("ghi cấu hình website: %w", err)
	}

	if err := n.Test(ctx); err != nil {
		n.restore(path, previous, hadPrevious)
		return err
	}
	if err := n.Reload(ctx); err != nil {
		n.restore(path, previous, hadPrevious)
		return err
	}

	return nil
}

// Remove gỡ cấu hình của một website rồi nạp lại nginx.
func (n *Nginx) Remove(ctx context.Context, name string) error {
	if !validSiteName.MatchString(name) {
		return fmt.Errorf("%w: tên website %q", ErrInvalidConfig, name)
	}

	path := n.configPath(name)
	previous, hadPrevious := readIfExists(path)
	if !hadPrevious {
		return fmt.Errorf("%w: %s", ErrSiteNotFound, name)
	}

	if err := os.Remove(path); err != nil {
		return fmt.Errorf("xóa cấu hình website: %w", err)
	}

	if err := n.Test(ctx); err != nil {
		n.restore(path, previous, true)
		return err
	}
	return n.Reload(ctx)
}

// restore khôi phục tệp cấu hình về trạng thái trước đó.
func (n *Nginx) restore(path string, previous []byte, hadPrevious bool) {
	if hadPrevious {
		_ = os.WriteFile(path, previous, 0o644)
		return
	}
	_ = os.Remove(path)
}

// Test kiểm tra toàn bộ cấu hình nginx.
func (n *Nginx) Test(ctx context.Context) error {
	res, err := n.host.Exec(ctx, host.Command{
		Path: n.binary, Args: []string{"-t"}, Timeout: commandTimeout,
	})
	if err != nil {
		return fmt.Errorf("kiểm tra cấu hình nginx: %w", err)
	}
	if !res.OK() {
		// nginx ghi kết quả kiểm tra ra stderr kể cả khi thành công.
		return fmt.Errorf("%w: %s", ErrInvalidConfig, strings.TrimSpace(res.Stderr))
	}
	return nil
}

// Reload nạp lại cấu hình nginx mà không ngắt kết nối đang phục vụ.
func (n *Nginx) Reload(ctx context.Context) error {
	res, err := n.host.Exec(ctx, host.Command{
		Path: n.binary, Args: []string{"-s", "reload"}, Timeout: commandTimeout,
	})
	if err != nil {
		return fmt.Errorf("nạp lại nginx: %w", err)
	}
	if !res.OK() {
		return fmt.Errorf("nạp lại nginx thất bại: %s", strings.TrimSpace(res.Stderr))
	}
	return nil
}

// Render sinh nội dung cấu hình cho một website.
func (n *Nginx) Render(site Site) (string, error) {
	if err := ValidateSite(site); err != nil {
		return "", err
	}

	data := templateData{Site: site, ModernHTTP2: n.supportsModernHTTP2()}
	data.IPv6 = n.supportsIPv6()
	if data.Port == 0 {
		data.Port = 80
	}
	if data.SSL.Port == 0 {
		data.SSL.Port = 443
	}
	if data.Type == SitePHP && strings.TrimSpace(data.PHPSocket) == "" {
		// Đường dẫn socket mặc định của PHP-FPM trên Debian và Ubuntu.
		data.PHPSocket = "unix:/run/php/php-fpm.sock"
	}

	var buf bytes.Buffer
	if err := nginxTemplate.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("sinh cấu hình nginx: %w", err)
	}
	return buf.String(), nil
}

// templateData là dữ liệu truyền cho khuôn cấu hình.
//
// Bọc thêm quanh Site thay vì nhồi vào chính Site: đây là chi tiết của riêng
// nginx, không phải thuộc tính của website.
type templateData struct {
	Site
	// ModernHTTP2 chọn cú pháp bật HTTP/2 phù hợp với phiên bản nginx.
	ModernHTTP2 bool
}

// supportsModernHTTP2 cho biết nginx trên máy có nhận chỉ thị "http2 on" không.
func (n *Nginx) supportsModernHTTP2() bool {
	n.modernHTTP2Once.Do(func() {
		version, err := n.version(context.Background())
		if err != nil {
			// Không đọc được phiên bản thì chọn cú pháp cũ: nginx mới vẫn hiểu nó
			// (chỉ cảnh báo là đã lỗi thời), còn nginx cũ thì không hiểu cú pháp mới.
			return
		}
		n.modernHTTP2 = compareVersion(version, http2Directive) >= 0
	})
	return n.modernHTTP2
}

// nginxVersion khớp chuỗi phiên bản trong đầu ra của "nginx -v".
var nginxVersion = regexp.MustCompile(`nginx/(\d+)\.(\d+)\.(\d+)`)

// version đọc phiên bản nginx đang cài.
func (n *Nginx) version(ctx context.Context) ([3]int, error) {
	res, err := n.host.Exec(ctx, host.Command{
		Path: n.binary, Args: []string{"-v"}, Timeout: 5 * time.Second,
	})
	if err != nil {
		return [3]int{}, fmt.Errorf("chạy nginx -v: %w", err)
	}

	// nginx ghi phiên bản ra stderr, nhưng vài bản dựng lại ghi ra stdout.
	match := nginxVersion.FindStringSubmatch(res.Stderr + res.Stdout)
	if match == nil {
		return [3]int{}, fmt.Errorf("không đọc được phiên bản nginx từ %q", res.Stderr)
	}

	var parsed [3]int
	for i := 0; i < 3; i++ {
		value, err := strconv.Atoi(match[i+1])
		if err != nil {
			return [3]int{}, fmt.Errorf("phiên bản nginx không hợp lệ: %w", err)
		}
		parsed[i] = value
	}
	return parsed, nil
}

// compareVersion so sánh hai phiên bản theo thứ tự major, minor, patch.
func compareVersion(a, b [3]int) int {
	for i := 0; i < 3; i++ {
		switch {
		case a[i] < b[i]:
			return -1
		case a[i] > b[i]:
			return 1
		}
	}
	return 0
}

// supportsIPv6 cho biết máy chủ có dùng được IPv6 hay không.
//
// Cách kiểm đáng tin nhất là thử mở đúng loại socket mà nginx sẽ mở: chỉ đọc
// danh sách giao diện mạng là không đủ, vì nhân có thể đã tắt hẳn IPv6 dù giao
// diện vẫn còn địa chỉ.
func (n *Nginx) supportsIPv6() bool {
	n.ipv6Once.Do(func() {
		listener, err := net.Listen("tcp6", "[::1]:0")
		if err != nil {
			return
		}
		_ = listener.Close()
		n.ipv6 = true
	})
	return n.ipv6
}

// readIfExists đọc một tệp nếu nó tồn tại.
func readIfExists(path string) ([]byte, bool) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	return content, true
}

// nginxTemplate là khuôn cấu hình cho một website.
//
// Mọi giá trị chèn vào đây đã qua ValidateSite, nên không có giá trị nào chứa
// được ký tự thoát khỏi chỉ thị cấu hình.
var nginxTemplate = template.Must(template.New("vhost").Parse(
	`# Tệp này do SunPanel sinh ra. Mọi thay đổi bằng tay sẽ bị ghi đè.
# Website: {{ .Name }}

{{- if and .SSL.Enabled .SSL.ForceHTTPS }}

server {
    listen {{ .Port }};
{{- if .IPv6 }}
    listen [::]:{{ .Port }};
{{- end }}
    server_name{{ range .Domains }} {{ . }}{{ end }};

{{- if .ACMEWebroot }}
    # Đường dẫn xác thực ACME phải phục vụ qua HTTP kể cả khi đã ép HTTPS,
    # nếu không việc gia hạn chứng chỉ sẽ thất bại.
    location ^~ /.well-known/acme-challenge/ {
        root {{ .ACMEWebroot }};
    }
{{- end }}

    location / {
        return 301 https://$host$request_uri;
    }
}
{{- end }}

server {
{{- if and .SSL.Enabled .SSL.ForceHTTPS }}
    listen {{ .SSL.Port }} ssl{{ if not .ModernHTTP2 }} http2{{ end }};
{{- if .IPv6 }}
    listen [::]:{{ .SSL.Port }} ssl{{ if not .ModernHTTP2 }} http2{{ end }};
{{- end }}
{{- if .ModernHTTP2 }}
    http2 on;
{{- end }}
{{- else }}
    listen {{ .Port }};
{{- if .IPv6 }}
    listen [::]:{{ .Port }};
{{- end }}
{{- if .SSL.Enabled }}
    listen {{ .SSL.Port }} ssl{{ if not .ModernHTTP2 }} http2{{ end }};
{{- if .IPv6 }}
    listen [::]:{{ .SSL.Port }} ssl{{ if not .ModernHTTP2 }} http2{{ end }};
{{- end }}
{{- if .ModernHTTP2 }}
    http2 on;
{{- end }}
{{- end }}
{{- end }}
    server_name{{ range .Domains }} {{ . }}{{ end }};

{{- if .SSL.Enabled }}

    ssl_certificate {{ .SSL.CertFile }};
    ssl_certificate_key {{ .SSL.KeyFile }};
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_prefer_server_ciphers off;
    ssl_session_cache shared:SSL:10m;
    ssl_session_timeout 1d;
{{- end }}

    access_log /var/log/nginx/{{ .Name }}.access.log;
    error_log /var/log/nginx/{{ .Name }}.error.log;

    client_max_body_size 128m;

{{- range .Redirects }}

    # Chuyển hướng do người dùng đặt
    location {{ if eq .From "/" }}= /{{ else }}{{ .From }}{{ end }} {
        return {{ .Code }} {{ .To }};
    }
{{- end }}

{{- if .DenyIPs }}

    # Danh sách chặn theo địa chỉ
{{- range .DenyIPs }}
    deny {{ . }};
{{- end }}
    allow all;
{{- end }}

{{- if .Auth.Enabled }}

    # Hỏi mật khẩu trước khi vào website
    auth_basic "{{ .Auth.Realm }}";
    auth_basic_user_file {{ .Auth.File }};
{{- end }}

{{- if .ACMEWebroot }}

    # Đặt trước mọi location khác để website dạng chuyển tiếp không đẩy yêu cầu
    # xác thực sang ứng dụng phía sau. auth_basic off để việc gia hạn chứng chỉ
    # không chết ngay khi người dùng bật lớp bảo vệ bằng mật khẩu.
    location ^~ /.well-known/acme-challenge/ {
        auth_basic off;
        root {{ .ACMEWebroot }};
        default_type "text/plain";
    }
{{- end }}

{{- if eq (printf "%s" .Type) "proxy" }}

    location / {
        proxy_pass {{ .ProxyTarget }};
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        # Hai dòng này cần cho WebSocket; thiếu chúng thì mọi ứng dụng dùng
        # kết nối thời gian thực đều đứt ngay sau khi bắt tay.
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_read_timeout 300s;
    }
{{- else }}

    root {{ .Root }};
    index index.html index.htm{{ if eq (printf "%s" .Type) "php" }} index.php{{ end }};

    location / {
        try_files $uri $uri/ {{ if eq (printf "%s" .Type) "php" }}/index.php?$query_string{{ else }}=404{{ end }};
    }
{{- if eq (printf "%s" .Type) "php" }}

    location ~ \.php$ {
        include snippets/fastcgi-php.conf;
        fastcgi_pass {{ .PHPSocket }};
    }

    # Tệp ẩn và tệp cấu hình không bao giờ được phục vụ ra ngoài.
    location ~ /\. {
        deny all;
    }
{{- end }}
{{- end }}
{{- if .ExtraConfig }}

    # Cấu hình bổ sung do người dùng thêm
{{ .ExtraConfig }}
{{- end }}
}
`))
