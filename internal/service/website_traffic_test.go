package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/pkg/accesslog"
	"github.com/thanhtinz/sunpanel/pkg/host"
)

// trafficFixture dựng một website kèm tệp nhật ký truy cập của nó.
func trafficFixture(t *testing.T, lines []string) (*WebsiteService, uint) {
	t.Helper()

	websites, _, _, root := newWebsiteFixtureAt(t)
	logDir := filepath.Join(root, "nginx-logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatalf("tạo thư mục nhật ký: %v", err)
	}

	if lines != nil {
		content := ""
		for _, line := range lines {
			content += line + "\n"
		}
		err := os.WriteFile(filepath.Join(logDir, "blog.access.log"), []byte(content), 0o644)
		if err != nil {
			t.Fatalf("ghi nhật ký: %v", err)
		}
	}

	websites.SetAccessLogs(accesslog.New(host.NewLocalHost(root, nil).FS()), "/nginx-logs")

	site, err := websites.Create(context.Background(), WebsiteRequest{
		Name: "blog", Domains: []string{"blog.example.com"},
		Type: "static", Root: "/www/blog", Enabled: true,
	})
	if err != nil {
		t.Fatalf("tạo website: %v", err)
	}
	return websites, site.ID
}

func logLine(stamp time.Time, ip, path string, status int) string {
	return fmt.Sprintf(`%s - - [%s] "GET %s HTTP/1.1" %d 1024 "-" "trinh-duyet"`,
		ip, stamp.Format("02/Jan/2006:15:04:05 -0700"), path, status)
}

func TestWebsiteTrafficSummarizesAccessLog(t *testing.T) {
	now := time.Now()
	websites, id := trafficFixture(t, []string{
		logLine(now.Add(-30*time.Minute), "203.0.113.1", "/", 200),
		logLine(now.Add(-20*time.Minute), "203.0.113.1", "/bai-viet", 200),
		logLine(now.Add(-10*time.Minute), "203.0.113.2", "/thieu", 404),
	})

	report, err := websites.Traffic(context.Background(), id, "24h")
	if err != nil {
		t.Fatalf("đọc thống kê: %v", err)
	}

	if report.Requests != 3 || report.Visitors != 2 {
		t.Errorf("tổng = %d yêu cầu, %d khách", report.Requests, report.Visitors)
	}
	if report.Status4xx != 1 || len(report.Failures) != 1 {
		t.Errorf("lỗi = %d, danh sách %+v", report.Status4xx, report.Failures)
	}
	if report.Window != "24h" {
		t.Errorf("khoảng thời gian = %q", report.Window)
	}
}

// Khoảng thời gian lạ không được phép biến thành một lần đọc tùy ý; nó rơi về
// mặc định thay vì báo lỗi, để một liên kết cũ vẫn mở được trang.
func TestWebsiteTrafficFallsBackToDefaultWindow(t *testing.T) {
	websites, id := trafficFixture(t, []string{
		logLine(time.Now(), "203.0.113.1", "/", 200),
	})

	report, err := websites.Traffic(context.Background(), id, "999h")
	if err != nil {
		t.Fatalf("đọc thống kê: %v", err)
	}
	if report.Window != "24h" {
		t.Errorf("khoảng thời gian = %q, mong 24h", report.Window)
	}
}

// Website vừa dựng chưa có ai vào thì nginx chưa tạo tệp nhật ký. Đó là trạng
// thái bình thường, không phải lỗi cần báo đỏ.
func TestWebsiteTrafficWithoutLogFile(t *testing.T) {
	websites, id := trafficFixture(t, nil)

	report, err := websites.Traffic(context.Background(), id, "1h")
	if err != nil {
		t.Fatalf("đọc thống kê: %v", err)
	}
	if report.Requests != 0 || report.TopPaths == nil || report.Failures == nil {
		t.Errorf("bản tóm tắt rỗng không đúng: %+v", report)
	}
}

func TestWebsiteTrafficNeedsAnalyzer(t *testing.T) {
	websites, _, _, _ := newWebsiteFixtureAt(t)

	site, err := websites.Create(context.Background(), WebsiteRequest{
		Name: "blog", Domains: []string{"blog.example.com"},
		Type: "static", Root: "/www/blog", Enabled: true,
	})
	if err != nil {
		t.Fatalf("tạo website: %v", err)
	}

	if _, err := websites.Traffic(context.Background(), site.ID, "1h"); !errors.Is(err, apperr.WebsiteLogUnavailable) {
		t.Errorf("lỗi = %v, mong WebsiteLogUnavailable", err)
	}
}

// Cấu hình sinh ra và trang thống kê phải trỏ cùng một thư mục: hai nơi tự khai
// đường dẫn riêng thì đổi chỗ ghi nhật ký là trang thống kê im lặng trống trơn.
func TestWebsiteConfigUsesConfiguredLogDir(t *testing.T) {
	websites, _, server, root := newWebsiteFixtureAt(t)
	websites.SetAccessLogs(accesslog.New(host.NewLocalHost(root, nil).FS()), "/nhat-ky-web")

	if _, err := websites.Create(context.Background(), WebsiteRequest{
		Name: "blog", Domains: []string{"blog.example.com"},
		Type: "static", Root: "/www/blog", Enabled: true,
	}); err != nil {
		t.Fatalf("tạo website: %v", err)
	}

	if got := server.applied["blog"].LogDir; got != "/nhat-ky-web" {
		t.Errorf("thư mục nhật ký trong cấu hình = %q", got)
	}
}
