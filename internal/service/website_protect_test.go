package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/thanhtinz/sunpanel/internal/apperr"
)

// protectedSite tạo một website tĩnh để thử các lớp bảo vệ.
func protectedSite(t *testing.T, req WebsiteRequest) (*WebsiteService, *fakeWebServer, string) {
	t.Helper()

	websites, _, server, root := newWebsiteFixtureAt(t)
	base := WebsiteRequest{
		Name: "blog", Domains: []string{"blog.example.com"},
		Type: "static", Root: "/www/blog", Enabled: true,
	}
	base.AuthEnabled, base.AuthUser, base.AuthPassword = req.AuthEnabled, req.AuthUser, req.AuthPassword
	base.DenyIPs, base.Redirects = req.DenyIPs, req.Redirects

	if _, err := websites.Create(context.Background(), base); err != nil {
		t.Fatalf("tạo website: %v", err)
	}
	return websites, server, root
}

func TestWebsiteAuthWritesHtpasswdAndConfig(t *testing.T) {
	_, server, root := protectedSite(t, WebsiteRequest{
		AuthEnabled: true, AuthUser: "khach", AuthPassword: "MatKhau#2026",
	})

	config := server.applied["blog"]
	if !config.Auth.Enabled {
		t.Fatal("cấu hình sinh ra không bật lớp bảo vệ")
	}

	// Tệp tài khoản phải có mặt trước khi nginx nạp cấu hình trỏ tới nó.
	data, err := os.ReadFile(filepath.Join(root, "htpasswd", "blog.htpasswd"))
	if err != nil {
		t.Fatalf("đọc tệp tài khoản: %v", err)
	}

	user, hash, ok := strings.Cut(strings.TrimSpace(string(data)), ":")
	if !ok || user != "khach" {
		t.Fatalf("nội dung tệp tài khoản: %q", data)
	}
	// Mật khẩu không bao giờ được nằm dạng chữ trong tệp mà nginx đọc mỗi yêu cầu.
	if strings.Contains(string(data), "MatKhau#2026") {
		t.Fatal("mật khẩu bị ghi nguyên văn")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("MatKhau#2026")); err != nil {
		t.Errorf("chuỗi băm không khớp mật khẩu: %v", err)
	}
}

// Ô mật khẩu để trống nghĩa là giữ nguyên cái đã lưu: giao diện không đọc lại
// được mật khẩu cũ, nên coi ô trống là "xóa" sẽ lặng lẽ mở toang lớp bảo vệ.
func TestWebsiteAuthKeepsPasswordWhenBlank(t *testing.T) {
	websites, _, root := protectedSite(t, WebsiteRequest{
		AuthEnabled: true, AuthUser: "khach", AuthPassword: "MatKhau#2026",
	})

	sites, err := websites.List(context.Background())
	if err != nil || len(sites) != 1 {
		t.Fatalf("liệt kê website: %v", err)
	}

	_, err = websites.Update(context.Background(), sites[0].ID, WebsiteRequest{
		Name: "blog", Domains: []string{"blog.example.com"}, Type: "static",
		Root: "/www/blog", Enabled: true, Remark: "đổi ghi chú",
		AuthEnabled: true, AuthUser: "khach",
	})
	if err != nil {
		t.Fatalf("cập nhật website: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, "htpasswd", "blog.htpasswd"))
	if err != nil {
		t.Fatalf("đọc tệp tài khoản: %v", err)
	}
	_, hash, _ := strings.Cut(strings.TrimSpace(string(data)), ":")
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("MatKhau#2026")); err != nil {
		t.Errorf("mật khẩu cũ đã mất sau khi sửa website: %v", err)
	}
}

func TestWebsiteAuthRequiresPasswordFirstTime(t *testing.T) {
	websites, _, _, _ := newWebsiteFixtureAt(t)

	_, err := websites.Create(context.Background(), WebsiteRequest{
		Name: "blog", Domains: []string{"blog.example.com"}, Type: "static",
		Root: "/www/blog", Enabled: true, AuthEnabled: true, AuthUser: "khach",
	})
	if !errors.Is(err, apperr.WebsiteAuthPasswordRequired) {
		t.Fatalf("lỗi = %v, mong WebsiteAuthPasswordRequired", err)
	}
}

// Tắt bảo vệ phải gỡ luôn tệp tài khoản: để lại một tệp chứa chuỗi băm mà không
// ai còn nhớ tới là rác, và bật lại sau này sẽ dùng nhầm mật khẩu cũ.
func TestWebsiteAuthRemovesFileWhenDisabled(t *testing.T) {
	websites, _, root := protectedSite(t, WebsiteRequest{
		AuthEnabled: true, AuthUser: "khach", AuthPassword: "MatKhau#2026",
	})

	sites, _ := websites.List(context.Background())
	if _, err := websites.Update(context.Background(), sites[0].ID, WebsiteRequest{
		Name: "blog", Domains: []string{"blog.example.com"}, Type: "static",
		Root: "/www/blog", Enabled: true,
	}); err != nil {
		t.Fatalf("tắt bảo vệ: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, "htpasswd", "blog.htpasswd")); !os.IsNotExist(err) {
		t.Error("tệp tài khoản vẫn còn sau khi tắt bảo vệ")
	}
}

func TestWebsiteRedirectsAndDenyList(t *testing.T) {
	_, server, _ := protectedSite(t, WebsiteRequest{
		DenyIPs: []string{" 203.0.113.9 ", "", "10.0.0.0/8"},
		Redirects: []RedirectRule{
			{From: "/cu", To: "https://example.com/moi", Permanent: true},
			{From: "/tam", To: "/khac"},
		},
	})

	config := server.applied["blog"]
	if len(config.DenyIPs) != 2 {
		t.Errorf("danh sách chặn = %v", config.DenyIPs)
	}
	if len(config.Redirects) != 2 {
		t.Fatalf("quy tắc chuyển hướng = %v", config.Redirects)
	}
	if config.Redirects[0].Code() != 301 || config.Redirects[1].Code() != 302 {
		t.Errorf("mã chuyển hướng: %d và %d", config.Redirects[0].Code(), config.Redirects[1].Code())
	}
}

// Giá trị đi thẳng vào tệp cấu hình của một tiến trình chạy quyền root, nên
// những chuỗi có thể thoát khỏi chỉ thị phải bị chặn ngay lúc lưu.
func TestWebsiteRejectsInjectionInProtection(t *testing.T) {
	websites, _, _, _ := newWebsiteFixtureAt(t)

	base := WebsiteRequest{
		Name: "blog", Domains: []string{"blog.example.com"}, Type: "static",
		Root: "/www/blog", Enabled: true,
	}

	bad := base
	bad.DenyIPs = []string{"1.2.3.4; root /etc"}
	if _, err := websites.Create(context.Background(), bad); !errors.Is(err, apperr.WebsiteInvalidDenyIP) {
		t.Errorf("danh sách chặn: lỗi = %v, mong WebsiteInvalidDenyIP", err)
	}

	bad = base
	bad.Redirects = []RedirectRule{{From: "/cu", To: "https://x.test/;\nreturn 200"}}
	if _, err := websites.Create(context.Background(), bad); !errors.Is(err, apperr.WebsiteInvalidRedirect) {
		t.Errorf("chuyển hướng: lỗi = %v, mong WebsiteInvalidRedirect", err)
	}

	bad = base
	bad.AuthEnabled, bad.AuthUser, bad.AuthPassword = true, "khach:kem", "matkhau"
	if _, err := websites.Create(context.Background(), bad); !errors.Is(err, apperr.WebsiteInvalidAuthUser) {
		t.Errorf("tài khoản: lỗi = %v, mong WebsiteInvalidAuthUser", err)
	}
}
