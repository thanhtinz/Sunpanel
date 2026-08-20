package service

import (
	"context"
	"errors"
	"testing"

	"github.com/thanhtinz/sunpanel/internal/apperr"
)

// Mẫu viết lại phải đi vào đúng cấu hình được ghi ra, chứ không chỉ nằm trong
// cơ sở dữ liệu: sai chỗ này thì giao diện hiện "đã bật" còn website vẫn 404.
func TestWebsiteRewriteReachesConfig(t *testing.T) {
	websites, _, server, _ := newWebsiteFixtureAt(t)

	site, err := websites.Create(context.Background(), WebsiteRequest{
		Name: "blog", Domains: []string{"blog.example.com"}, Type: "php",
		Root: "/www/blog", Enabled: true, Rewrite: "wordpress",
	})
	if err != nil {
		t.Fatalf("tạo website: %v", err)
	}
	if server.applied["blog"].Rewrite != "wordpress" {
		t.Fatalf("mẫu trong cấu hình = %q", server.applied["blog"].Rewrite)
	}

	// Bỏ chọn mẫu phải lưu được. Cột có giá trị mặc định sẽ làm GORM bỏ qua
	// chuỗi rỗng, và người dùng không bao giờ tắt được mẫu đã bật.
	if _, err := websites.Update(context.Background(), site.ID, WebsiteRequest{
		Name: "blog", Domains: []string{"blog.example.com"}, Type: "php",
		Root: "/www/blog", Enabled: true, Rewrite: "",
	}); err != nil {
		t.Fatalf("bỏ chọn mẫu: %v", err)
	}

	updated, err := websites.Get(context.Background(), site.ID)
	if err != nil {
		t.Fatalf("đọc lại website: %v", err)
	}
	if updated.Rewrite != "" {
		t.Errorf("mẫu sau khi bỏ chọn = %q", updated.Rewrite)
	}
}

func TestWebsiteRejectsUnknownRewrite(t *testing.T) {
	websites, _, _, _ := newWebsiteFixtureAt(t)

	_, err := websites.Create(context.Background(), WebsiteRequest{
		Name: "blog", Domains: []string{"blog.example.com"}, Type: "php",
		Root: "/www/blog", Enabled: true, Rewrite: "mau-la",
	})
	if !errors.Is(err, apperr.WebsiteInvalidRewrite) {
		t.Errorf("lỗi = %v, mong WebsiteInvalidRewrite", err)
	}
}
