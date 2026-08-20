package webserver

import (
	"strings"
	"testing"
)

// nginxForRewrite dựng bộ sinh cấu hình có host thật: Render phải dò được
// phiên bản nginx, và một Nginx rỗng không có host để hỏi.
func nginxForRewrite(t *testing.T) *Nginx {
	t.Helper()
	server, _ := newNginx(t)
	return server
}

func rewriteSite(rewrite string) Site {
	return Site{
		Name: "blog", Domains: []string{"blog.example.com"},
		Type: SitePHP, Root: "/www/blog", Rewrite: rewrite,
	}
}

// Mẫu phải thay đúng khối location / mặc định, nếu không nginx nhận hai khối
// trùng nhau và từ chối nạp cấu hình — kéo theo mọi website khác ngừng phục vụ.
func TestRenderUsesRewritePreset(t *testing.T) {
	config, err := nginxForRewrite(t).Render(rewriteSite("wordpress"))
	if err != nil {
		t.Fatalf("sinh cấu hình: %v", err)
	}

	if strings.Count(config, "location / {") != 1 {
		t.Fatalf("số khối location /: %d\n%s", strings.Count(config, "location / {"), config)
	}
	if !strings.Contains(config, "try_files $uri $uri/ /index.php?$args;") {
		t.Errorf("không thấy quy tắc của mẫu WordPress:\n%s", config)
	}
	// Các lớp khác vẫn do khuôn chung sinh ra.
	if !strings.Contains(config, "location ~ \\.php$") {
		t.Errorf("mẫu làm mất khối xử lý PHP:\n%s", config)
	}
}

func TestRenderWithoutRewriteKeepsDefault(t *testing.T) {
	config, err := nginxForRewrite(t).Render(rewriteSite(RewriteNone))
	if err != nil {
		t.Fatalf("sinh cấu hình: %v", err)
	}
	if !strings.Contains(config, "/index.php?$query_string") {
		t.Errorf("mất quy tắc mặc định:\n%s", config)
	}
}

// Website chuyển tiếp không có tệp trên đĩa để viết lại đường dẫn tới; một khối
// location / thứ hai ở đó sẽ giành mất mọi yêu cầu của proxy_pass.
func TestProxySiteIgnoresRewrite(t *testing.T) {
	site := Site{
		Name: "app", Domains: []string{"app.example.com"},
		Type: SiteProxy, ProxyTarget: "http://127.0.0.1:3000", Rewrite: "wordpress",
	}

	config, err := nginxForRewrite(t).Render(site)
	if err != nil {
		t.Fatalf("sinh cấu hình: %v", err)
	}
	if strings.Contains(config, "index.php") {
		t.Errorf("mẫu bị áp cho website chuyển tiếp:\n%s", config)
	}
	if strings.Count(config, "location / {") != 1 {
		t.Errorf("số khối location / = %d", strings.Count(config, "location / {"))
	}
}

// Định danh lạ phải bị chặn ngay lúc kiểm tra: bỏ qua trong im lặng nghĩa là
// website chạy quy tắc mặc định trong khi giao diện vẫn hiện tên mẫu đã chọn.
func TestValidateRejectsUnknownRewrite(t *testing.T) {
	if err := ValidateSite(rewriteSite("khong-co-mau-nay")); err == nil {
		t.Error("mẫu không tồn tại lại được chấp nhận")
	}
}

func TestRewritesListed(t *testing.T) {
	list := Rewrites()
	if len(list) < 5 {
		t.Fatalf("số mẫu = %d, quá ít", len(list))
	}
	for i, item := range list {
		if item.Key == "" || item.Body == "" {
			t.Errorf("mẫu thứ %d thiếu dữ liệu: %+v", i, item)
		}
		if i > 0 && list[i-1].Key >= item.Key {
			t.Errorf("danh sách không sắp theo định danh: %s trước %s", list[i-1].Key, item.Key)
		}
	}
}
