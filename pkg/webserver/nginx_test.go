package webserver

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thanhtinz/sunpanel/pkg/host"
)

func newNginx(t *testing.T) (*Nginx, string) {
	t.Helper()

	confDir := t.TempDir()
	h := host.NewLocalHost("/", []string{"nginx"})
	return NewNginx(h, confDir), confDir
}

func TestRenderStaticSite(t *testing.T) {
	manager, _ := newNginx(t)

	config, err := manager.Render(Site{
		Name:    "trang-chu",
		Domains: []string{"example.com", "www.example.com"},
		Type:    SiteStatic,
		Root:    "/var/www/example",
	})
	if err != nil {
		t.Fatalf("sinh cấu hình: %v", err)
	}

	for _, want := range []string{
		"server_name example.com www.example.com;",
		"root /var/www/example;",
		"listen 80;",
		"try_files $uri $uri/ =404;",
	} {
		if !strings.Contains(config, want) {
			t.Errorf("cấu hình thiếu %q:\n%s", want, config)
		}
	}
	// Website tĩnh không được có khối PHP.
	if strings.Contains(config, "fastcgi_pass") {
		t.Error("website tĩnh không được sinh cấu hình PHP")
	}
}

func TestRenderProxySite(t *testing.T) {
	manager, _ := newNginx(t)

	config, err := manager.Render(Site{
		Name:        "ung-dung",
		Domains:     []string{"app.example.com"},
		Type:        SiteProxy,
		ProxyTarget: "http://127.0.0.1:3000",
	})
	if err != nil {
		t.Fatalf("sinh cấu hình: %v", err)
	}

	if !strings.Contains(config, "proxy_pass http://127.0.0.1:3000;") {
		t.Errorf("thiếu chỉ thị proxy_pass:\n%s", config)
	}
	// Thiếu hai dòng này thì mọi ứng dụng dùng WebSocket đều đứt kết nối.
	for _, want := range []string{`proxy_set_header Upgrade $http_upgrade;`, `proxy_set_header Connection "upgrade";`} {
		if !strings.Contains(config, want) {
			t.Errorf("thiếu cấu hình WebSocket %q", want)
		}
	}
}

func TestRenderPHPSite(t *testing.T) {
	manager, _ := newNginx(t)

	config, err := manager.Render(Site{
		Name:    "wordpress",
		Domains: []string{"blog.example.com"},
		Type:    SitePHP,
		Root:    "/var/www/blog",
	})
	if err != nil {
		t.Fatalf("sinh cấu hình: %v", err)
	}

	for _, want := range []string{
		"index index.html index.htm index.php;",
		"fastcgi_pass unix:/run/php/php-fpm.sock;",
		"/index.php?$query_string",
		"location ~ /\\.",
	} {
		if !strings.Contains(config, want) {
			t.Errorf("cấu hình thiếu %q:\n%s", want, config)
		}
	}
}

// Ép HTTPS phải vẫn để lối cho ACME đi qua HTTP, nếu không việc gia hạn chứng
// chỉ sẽ thất bại và website mất HTTPS sau ba tháng.
func TestRenderForceHTTPSKeepsACMEPath(t *testing.T) {
	manager, _ := newNginx(t)

	config, err := manager.Render(Site{
		Name:        "bao-mat",
		Domains:     []string{"secure.example.com"},
		Type:        SiteStatic,
		Root:        "/var/www/secure",
		ACMEWebroot: "/opt/sunpanel/acme",
		SSL: SSLConfig{
			Enabled:    true,
			ForceHTTPS: true,
			CertFile:   "/etc/ssl/secure.crt",
			KeyFile:    "/etc/ssl/secure.key",
		},
	})
	if err != nil {
		t.Fatalf("sinh cấu hình: %v", err)
	}

	// Phải có ở cả khối chuyển hướng lẫn khối HTTPS: Let's Encrypt truy cập qua
	// HTTP, và một chuyển hướng 301 sang HTTPS vẫn được coi là hợp lệ nên thiếu ở
	// một trong hai chỗ vẫn có lúc hỏng.
	if count := strings.Count(config, "location ^~ /.well-known/acme-challenge/"); count != 2 {
		t.Errorf("cấu hình ép HTTPS phải chừa lối cho ACME ở cả hai khối, đếm được %d:\n%s",
			count, config)
	}
	if !strings.Contains(config, "return 301 https://$host$request_uri;") {
		t.Error("thiếu chuyển hướng sang HTTPS")
	}
	if !strings.Contains(config, "ssl_certificate /etc/ssl/secure.crt;") {
		t.Error("thiếu đường dẫn chứng chỉ")
	}
	if !strings.Contains(config, "ssl_protocols TLSv1.2 TLSv1.3;") {
		t.Error("thiếu cấu hình phiên bản TLS")
	}
}

// Giá trị đi thẳng vào tệp cấu hình của một tiến trình chạy quyền root; dấu chấm
// phẩy kết thúc chỉ thị và dấu ngoặc nhọn mở khối mới.
func TestValidateSiteRejectsConfigInjection(t *testing.T) {
	cases := []struct {
		name string
		site Site
	}{
		{"đường dẫn có dấu chấm phẩy", Site{
			Name: "a", Domains: []string{"a.com"}, Type: SiteStatic,
			Root: "/var/www; root /etc",
		}},
		{"đường dẫn có ngoặc nhọn", Site{
			Name: "a", Domains: []string{"a.com"}, Type: SiteStatic,
			Root: "/var/www } server { listen 8080",
		}},
		{"đường dẫn có xuống dòng", Site{
			Name: "a", Domains: []string{"a.com"}, Type: SiteStatic,
			Root: "/var/www\n    autoindex on",
		}},
		{"tên miền có dấu chấm phẩy", Site{
			Name: "a", Domains: []string{"a.com; return 200"}, Type: SiteStatic, Root: "/var/www",
		}},
		{"tên miền có khoảng trắng", Site{
			Name: "a", Domains: []string{"a.com b.com"}, Type: SiteStatic, Root: "/var/www",
		}},
		{"đích proxy tùy ý", Site{
			Name: "a", Domains: []string{"a.com"}, Type: SiteProxy,
			ProxyTarget: "http://x; proxy_pass http://evil",
		}},
		{"tên website có gạch chéo", Site{
			Name: "../../etc/nginx/nginx", Domains: []string{"a.com"},
			Type: SiteStatic, Root: "/var/www",
		}},
		{"chứng chỉ có ký tự lạ", Site{
			Name: "a", Domains: []string{"a.com"}, Type: SiteStatic, Root: "/var/www",
			SSL: SSLConfig{Enabled: true, CertFile: "/a.crt\"; root /etc", KeyFile: "/a.key"},
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateSite(tc.site)
			if err == nil {
				t.Fatalf("cấu hình nguy hiểm phải bị từ chối")
			}
			if !errors.Is(err, ErrInvalidConfig) && !errors.Is(err, ErrInvalidDomain) {
				t.Fatalf("lỗi = %v, mong đợi lỗi cấu hình hoặc tên miền", err)
			}
		})
	}
}

func TestValidateDomain(t *testing.T) {
	valid := []string{
		"example.com", "www.example.com", "a.b.c.example.com",
		"*.example.com", "xn--th-e0a.com", "localhost", "EXAMPLE.COM",
	}
	for _, domain := range valid {
		if err := ValidateDomain(domain); err != nil {
			t.Errorf("tên miền %q phải hợp lệ: %v", domain, err)
		}
	}

	invalid := []string{
		"", "example", "-example.com", "example-.com", "exam ple.com",
		"example.com;", "example.com/path", "http://example.com", "example..com",
	}
	for _, domain := range invalid {
		if err := ValidateDomain(domain); err == nil {
			t.Errorf("tên miền %q phải bị từ chối", domain)
		}
	}
}

// mainConfig dựng cấu hình nginx tối giản có include thư mục của panel.
//
// pid và nhật ký phải nằm trong thư mục tạm: nginx -t mở chúng theo đường dẫn
// biên dịch sẵn (/run/nginx.pid, /var/log/nginx), mà đó là chỗ chỉ root ghi được.
func mainConfig(mainConf, confDir string) string {
	dir := filepath.Dir(mainConf)
	return "pid " + filepath.Join(dir, "nginx.pid") + ";\n" +
		"error_log " + filepath.Join(dir, "error.log") + ";\n" +
		"events {}\nhttp {\n" +
		"    access_log " + filepath.Join(dir, "access.log") + ";\n" +
		"    include " + confDir + "/*.conf;\n}\n"
}

// requireRootForNginxTest bỏ qua bài kiểm thử khi không chạy được đầy đủ.
//
// nginx -t không chỉ đọc cấu hình mà còn mở luôn cổng lắng nghe để chắc chắn mở
// được. Cấu hình thật dùng cổng 80 và 443, nên chỉ root mới kiểm được — và uốn
// cấu hình sang cổng cao chỉ để chiều bài kiểm thử thì nó không còn kiểm đúng
// thứ panel sinh ra nữa.
func requireRootForNginxTest(t *testing.T) {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Skip("nginx -t mở cổng 80 và 443 nên cần chạy dưới quyền root")
	}
}

// Cấu hình sinh ra phải được chính nginx chấp nhận, không chỉ "trông đúng".
func TestGeneratedConfigPassesNginxTest(t *testing.T) {
	requireRootForNginxTest(t)

	manager, confDir := newNginx(t)
	ctx := context.Background()

	if !manager.Available(ctx) {
		t.Skip("nginx không có trên máy")
	}

	root := t.TempDir()
	// nginx -t mở cả tệp nhật ký, nên nhật ký phải nằm trong thư mục tạm chứ
	// không phải /var/log/nginx — chỗ đó chỉ root ghi được.
	logDir := t.TempDir()
	sites := []Site{
		{Name: "tinh", Domains: []string{"static.example.com"}, Type: SiteStatic, Root: root, LogDir: logDir},
		{Name: "proxy", Domains: []string{"app.example.com"}, Type: SiteProxy,
			ProxyTarget: "http://127.0.0.1:3000", ACMEWebroot: root, LogDir: logDir},
	}

	for _, site := range sites {
		config, err := manager.Render(site)
		if err != nil {
			t.Fatalf("sinh cấu hình %s: %v", site.Name, err)
		}
		path := filepath.Join(confDir, "sunpanel-"+site.Name+".conf")
		if err := os.WriteFile(path, []byte(config), 0o644); err != nil {
			t.Fatalf("ghi cấu hình: %v", err)
		}
	}

	// Dựng một cấu hình nginx tối giản có include thư mục của panel, rồi nhờ
	// chính nginx kiểm tra.
	mainConf := filepath.Join(t.TempDir(), "nginx.conf")
	// pid và error_log phải nằm trong thư mục tạm: nginx -t mở tệp pid theo đường
	// dẫn biên dịch sẵn (/run/nginx.pid), mà một tiến trình không phải root thì
	// không ghi được vào đó — và bài kiểm thử này chạy dưới tài khoản thường trên
	// máy dựng bản.
	content := mainConfig(mainConf, confDir)
	if err := os.WriteFile(mainConf, []byte(content), 0o644); err != nil {
		t.Fatalf("ghi cấu hình chính: %v", err)
	}

	h := host.NewLocalHost("/", []string{"nginx"})
	res, err := h.Exec(ctx, host.Command{Path: "nginx", Args: []string{"-t", "-c", mainConf}})
	if err != nil {
		t.Fatalf("chạy nginx -t: %v", err)
	}
	if !res.OK() {
		t.Fatalf("nginx từ chối cấu hình do panel sinh ra:\n%s", res.Stderr)
	}
	t.Logf("nginx chấp nhận cấu hình: %s", strings.TrimSpace(res.Stderr))
}

// Máy không bật IPv6 thì nginx không chỉ bỏ qua chỉ thị "listen [::]" mà từ chối
// khởi động hẳn, kéo theo mọi website khác cùng sập. Vì vậy khả năng IPv6 phải
// được dò chứ không giả định.
func TestRenderOmitsIPv6WhenUnsupported(t *testing.T) {
	manager, _ := newNginx(t)

	config, err := manager.Render(Site{
		Name: "kiem-tra", Domains: []string{"ipv6.example.com"},
		Type: SiteStatic, Root: "/var/www/ipv6",
	})
	if err != nil {
		t.Fatalf("sinh cấu hình: %v", err)
	}

	hasIPv6Line := strings.Contains(config, "listen [::]")
	if hasIPv6Line != manager.supportsIPv6() {
		t.Errorf("cấu hình %s dòng IPv6 nhưng máy %s hỗ trợ",
			map[bool]string{true: "có", false: "không có"}[hasIPv6Line],
			map[bool]string{true: "có", false: "không"}[manager.supportsIPv6()])
	}
}

// Ghi cấu hình hỏng phải khôi phục lại tệp cũ: một lỗi gõ nhầm không được làm
// nginx từ chối khởi động và kéo sập các website đang chạy.
func TestApplyRestoresPreviousConfigOnFailure(t *testing.T) {
	manager, confDir := newNginx(t)
	ctx := context.Background()

	if !manager.Available(ctx) {
		t.Skip("nginx không có trên máy")
	}

	// Đặt sẵn một tệp cấu hình "cũ" để kiểm tra việc khôi phục.
	path := filepath.Join(confDir, "sunpanel-cu.conf")
	const original = "# cấu hình cũ\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("ghi cấu hình cũ: %v", err)
	}

	// Đích proxy trỏ tới một tên miền không phân giải được sẽ khiến nginx -t
	// thất bại, mô phỏng một cấu hình hỏng.
	err := manager.Apply(ctx, Site{
		Name: "cu", Domains: []string{"cu.example.com"}, Type: SiteProxy,
		ProxyTarget: "http://ten-mien-chac-chan-khong-phan-giai-duoc.invalid:9999",
	})
	if err == nil {
		t.Skip("nginx chấp nhận cấu hình này nên không kiểm được đường khôi phục")
	}

	restored, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("tệp cấu hình cũ đã biến mất: %v", readErr)
	}
	if string(restored) != original {
		t.Errorf("cấu hình cũ không được khôi phục:\n%s", restored)
	}
}

// Website dạng chuyển tiếp đẩy mọi đường dẫn sang ứng dụng phía sau. Nếu đường
// dẫn xác thực ACME cũng đi theo, Let's Encrypt sẽ nhận phản hồi của ứng dụng
// thay vì tệp xác thực và việc gia hạn chứng chỉ thất bại — chỉ lộ ra khi chứng
// chỉ đã hết hạn.
func TestRenderProxySiteServesACMEPathLocally(t *testing.T) {
	manager, _ := newNginx(t)

	config, err := manager.Render(Site{
		Name:        "ung-dung",
		Domains:     []string{"app.example.com"},
		Type:        SiteProxy,
		ProxyTarget: "http://127.0.0.1:3000",
		ACMEWebroot: "/opt/sunpanel/acme",
	})
	if err != nil {
		t.Fatalf("sinh cấu hình: %v", err)
	}

	challenge := strings.Index(config, "location ^~ /.well-known/acme-challenge/")
	if challenge < 0 {
		t.Fatalf("website chuyển tiếp phải tự phục vụ đường dẫn xác thực:\n%s", config)
	}
	if !strings.Contains(config, "root /opt/sunpanel/acme;") {
		t.Errorf("đường dẫn xác thực phải trỏ về thư mục dùng chung:\n%s", config)
	}

	// nginx chọn location tiền tố dài nhất chứ không theo thứ tự, nhưng "^~"
	// chặn hẳn việc xét tiếp; kiểm tra thứ tự để cấu hình còn dễ đọc với người.
	if proxy := strings.Index(config, "proxy_pass"); proxy < challenge {
		t.Errorf("khối xác thực phải đứng trước proxy_pass để dễ đọc:\n%s", config)
	}
}

// Cú pháp bật HTTP/2 đổi ở nginx 1.25.1. Dùng sai không phải là bị bỏ qua mà là
// nginx từ chối nạp cấu hình, kéo sập mọi website trên máy — và Ubuntu 22.04,
// Debian 12, Rocky 9 đều còn dùng bản cũ hơn mốc đó.
func TestSSLConfigPassesNginxTest(t *testing.T) {
	requireRootForNginxTest(t)

	manager, confDir := newNginx(t)
	ctx := context.Background()

	if !manager.Available(ctx) {
		t.Skip("nginx không có trên máy")
	}

	root := t.TempDir()
	certFile, keyFile := writeTestCertificate(t, root)

	site := Site{
		Name: "https", Domains: []string{"secure.example.com"},
		Type: SiteStatic, Root: root, ACMEWebroot: root, LogDir: t.TempDir(),
		SSL: SSLConfig{Enabled: true, ForceHTTPS: true, CertFile: certFile, KeyFile: keyFile},
	}

	config, err := manager.Render(site)
	if err != nil {
		t.Fatalf("sinh cấu hình: %v", err)
	}
	path := filepath.Join(confDir, "sunpanel-https.conf")
	if err := os.WriteFile(path, []byte(config), 0o644); err != nil {
		t.Fatalf("ghi cấu hình: %v", err)
	}

	mainConf := filepath.Join(t.TempDir(), "nginx.conf")
	// pid và error_log phải nằm trong thư mục tạm: nginx -t mở tệp pid theo đường
	// dẫn biên dịch sẵn (/run/nginx.pid), mà một tiến trình không phải root thì
	// không ghi được vào đó — và bài kiểm thử này chạy dưới tài khoản thường trên
	// máy dựng bản.
	content := mainConfig(mainConf, confDir)
	if err := os.WriteFile(mainConf, []byte(content), 0o644); err != nil {
		t.Fatalf("ghi cấu hình chính: %v", err)
	}

	h := host.NewLocalHost("/", []string{"nginx"})
	res, err := h.Exec(ctx, host.Command{Path: "nginx", Args: []string{"-t", "-c", mainConf}})
	if err != nil {
		t.Fatalf("chạy nginx -t: %v", err)
	}
	if !res.OK() {
		t.Fatalf("nginx từ chối cấu hình HTTPS do panel sinh ra:\n%s\n--- cấu hình ---\n%s",
			res.Stderr, config)
	}
}

// writeTestCertificate sinh một chứng chỉ tự ký để nginx có thứ mà nạp.
func writeTestCertificate(t *testing.T, dir string) (certFile, keyFile string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("sinh khóa: %v", err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "secure.example.com"},
		DNSNames:     []string{"secure.example.com"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("tạo chứng chỉ: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("mã hóa khóa: %v", err)
	}

	certFile = filepath.Join(dir, "test.crt")
	keyFile = filepath.Join(dir, "test.key")
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	if err := os.WriteFile(certFile, certPEM, 0o644); err != nil {
		t.Fatalf("ghi chứng chỉ: %v", err)
	}
	if err := os.WriteFile(keyFile, keyPEM, 0o600); err != nil {
		t.Fatalf("ghi khóa: %v", err)
	}
	return certFile, keyFile
}
