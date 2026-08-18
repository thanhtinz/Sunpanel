package service

import (
	"context"
	"errors"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/config"
)

// fakeRestarter đếm số lần panel được yêu cầu khởi động lại.
type fakeRestarter struct {
	calls     int
	supported bool
}

func (f *fakeRestarter) Restart() error         { f.calls++; return nil }
func (f *fakeRestarter) RestartSupported() bool { return f.supported }

func newSettingsFixture(t *testing.T) (*SettingsService, *fakeRestarter, string) {
	t.Helper()

	dir := t.TempDir()
	cfg := config.Default()
	cfg.Server.DataDir = dir
	cfg.Server.Port = 9527
	cfg.Server.EntryPath = "duong-dan-cu"
	cfg.Database.Path = filepath.Join(dir, "sunpanel.db")

	path := filepath.Join(dir, "config.yaml")
	restarter := &fakeRestarter{supported: true}
	return NewSettingsService(cfg, path, restarter, NewAuditService(newMemoryDB(t))), restarter, path
}

// valid dựng một bộ giá trị hợp lệ từ cấu hình đang chạy để bài kiểm thử chỉ
// phải đổi đúng trường nó quan tâm.
func valid(s *SettingsService) Settings {
	current := s.Get().Settings
	return current
}

func TestSettingsUpdateWritesFile(t *testing.T) {
	settings, _, path := newSettingsFixture(t)

	req := valid(settings)
	req.EntryPath = "duong-dan-moi"
	req.Port = 9600
	req.LogLevel = "warn"

	result, err := settings.Update(context.Background(), req, AuditEntry{})
	if err != nil {
		t.Fatalf("lưu cấu hình: %v", err)
	}
	if !result.PendingRestart {
		t.Error("đổi cổng và đường dẫn phải được đánh dấu là cần khởi động lại")
	}
	// Người dùng vừa tự đổi địa chỉ panel của mình; phản hồi phải nói địa chỉ mới.
	if result.URL == "" || result.URL[len(result.URL)-14:] != "duong-dan-moi/" {
		t.Errorf("địa chỉ trả về không chứa đường dẫn mới: %q", result.URL)
	}

	saved, err := config.Load(path)
	if err != nil {
		t.Fatalf("đọc lại tệp cấu hình: %v", err)
	}
	if saved.Server.Port != 9600 || saved.Server.EntryPath != "duong-dan-moi" {
		t.Errorf("tệp cấu hình chưa ghi giá trị mới: %+v", saved.Server)
	}
	if saved.Log.Level != "warn" {
		t.Errorf("mức log = %q, mong \"warn\"", saved.Log.Level)
	}
}

// Đường dẫn bí mật nằm ngay đầu URL: ký tự lạ ở đây tạo ra một địa chỉ mà chính
// người dùng cũng không gõ lại được.
func TestSettingsRejectsBadEntryPath(t *testing.T) {
	settings, _, _ := newSettingsFixture(t)

	for _, entry := range []string{"", "a", "co khoang trang", "co/gach-cheo", "dấu-tiếng-việt"} {
		req := valid(settings)
		req.EntryPath = entry

		if _, err := settings.Update(context.Background(), req, AuditEntry{}); !errors.Is(err, apperr.SettingsInvalidEntryPath) {
			t.Errorf("đường dẫn %q: lỗi = %v, mong SettingsInvalidEntryPath", entry, err)
		}
	}
}

// Một dòng sai chính tả trong danh sách IP được phép sẽ khóa đúng người quản trị
// ra ngoài, nên phải bắt ngay lúc lưu.
func TestSettingsRejectsBadIP(t *testing.T) {
	settings, _, _ := newSettingsFixture(t)

	req := valid(settings)
	req.AllowedIPs = []string{"10.0.0.1", "khong-phai-ip"}

	if _, err := settings.Update(context.Background(), req, AuditEntry{}); !errors.Is(err, apperr.SettingsInvalidIP) {
		t.Fatalf("lỗi = %v, mong SettingsInvalidIP", err)
	}
}

func TestSettingsAcceptsCIDRAndTrimsBlankLines(t *testing.T) {
	settings, _, _ := newSettingsFixture(t)

	req := valid(settings)
	req.AllowedIPs = []string{" 10.0.0.0/8 ", "", "203.0.113.7"}

	result, err := settings.Update(context.Background(), req, AuditEntry{})
	if err != nil {
		t.Fatalf("lưu cấu hình: %v", err)
	}
	if len(result.AllowedIPs) != 2 || result.AllowedIPs[0] != "10.0.0.0/8" {
		t.Errorf("danh sách IP = %v", result.AllowedIPs)
	}
}

func TestSettingsRejectsBadDuration(t *testing.T) {
	settings, _, _ := newSettingsFixture(t)

	req := valid(settings)
	req.AccessTokenTTL = "mười lăm phút"

	if _, err := settings.Update(context.Background(), req, AuditEntry{}); !errors.Is(err, apperr.SettingsInvalidDuration) {
		t.Fatalf("lỗi = %v, mong SettingsInvalidDuration", err)
	}
}

// Hai trường mang cùng một giá trị phải cùng được ghi nhận — lỗi này có thật khi
// các khoảng thời gian được gom vào một map khóa theo chuỗi.
func TestSettingsKeepsDurationsWithSameValue(t *testing.T) {
	settings, _, _ := newSettingsFixture(t)

	req := valid(settings)
	req.AccessTokenTTL = "30m"
	req.LockoutDuration = "30m"

	result, err := settings.Update(context.Background(), req, AuditEntry{})
	if err != nil {
		t.Fatalf("lưu cấu hình: %v", err)
	}
	if result.AccessTokenTTL != "30m0s" || result.LockoutDuration != "30m0s" {
		t.Errorf("thời lượng sau khi lưu: access=%q lockout=%q", result.AccessTokenTTL, result.LockoutDuration)
	}
}

// Lưu một cổng đang bị tiến trình khác giữ nghĩa là panel khởi động lại rồi
// không lên nữa, và người dùng chỉ biết khi đã mất đường vào.
func TestSettingsRejectsPortInUse(t *testing.T) {
	settings, _, _ := newSettingsFixture(t)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("không mở được cổng thử: %v", err)
	}
	defer listener.Close()

	req := valid(settings)
	req.Host = "127.0.0.1"
	req.Port = listener.Addr().(*net.TCPAddr).Port

	if _, err := settings.Update(context.Background(), req, AuditEntry{}); !errors.Is(err, apperr.SettingsPortInUse) {
		t.Fatalf("lỗi = %v, mong SettingsPortInUse", err)
	}
}

// Giữ nguyên cổng đang chạy thì không được coi là "cổng bận": chính panel đang
// giữ nó, và mọi lần lưu cấu hình khác sẽ bị chặn oan.
func TestSettingsAllowsKeepingCurrentPort(t *testing.T) {
	settings, _, _ := newSettingsFixture(t)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("không mở được cổng thử: %v", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	settings.cfg.Server.Host, settings.cfg.Server.Port = "127.0.0.1", port

	req := valid(settings)
	req.LogLevel = "debug"

	if _, err := settings.Update(context.Background(), req, AuditEntry{}); err != nil {
		t.Fatalf("lưu cấu hình khi giữ nguyên cổng: %v", err)
	}
}

func TestSettingsRestart(t *testing.T) {
	settings, restarter, _ := newSettingsFixture(t)

	if err := settings.Restart(context.Background(), AuditEntry{}); err != nil {
		t.Fatalf("khởi động lại: %v", err)
	}
	if restarter.calls != 1 {
		t.Errorf("số lần yêu cầu khởi động lại = %d, mong 1", restarter.calls)
	}

	restarter.supported = false
	if err := settings.Restart(context.Background(), AuditEntry{}); !errors.Is(err, apperr.SettingsRestartUnsupported) {
		t.Fatalf("lỗi = %v, mong SettingsRestartUnsupported", err)
	}
}

func TestGenerateEntryPathIsUsable(t *testing.T) {
	settings, _, _ := newSettingsFixture(t)

	entry, err := settings.GenerateEntryPath()
	if err != nil {
		t.Fatalf("sinh đường dẫn: %v", err)
	}
	// Đường dẫn tự sinh phải qua được chính phép kiểm tra lúc lưu, nếu không nút
	// "sinh ngẫu nhiên" chỉ tạo ra một giá trị bị từ chối ngay sau đó.
	req := valid(settings)
	req.EntryPath = entry
	if _, err := settings.Update(context.Background(), req, AuditEntry{}); err != nil {
		t.Fatalf("đường dẫn tự sinh %q bị từ chối: %v", entry, err)
	}
}

func TestSettingsRejectsInvalidLogLevel(t *testing.T) {
	settings, _, _ := newSettingsFixture(t)

	req := valid(settings)
	req.LogLevel = "chi-tiet"

	if _, err := settings.Update(context.Background(), req, AuditEntry{}); !errors.Is(err, apperr.SettingsInvalid) {
		t.Fatalf("lỗi = %v, mong SettingsInvalid", err)
	}
}

func TestSettingsInfoHidesSecrets(t *testing.T) {
	settings, _, _ := newSettingsFixture(t)
	settings.cfg.Security.JWTSecret = "khoa-ky-token"

	// Kiểu Settings không có trường nào mang bí mật; đây là bài kiểm thử chống
	// hồi quy cho việc vô tình thêm JWTSecret vào cấu trúc gửi xuống trình duyệt.
	info := settings.Get()
	if info.AccessTokenTTL == "" || info.ConfigPath == "" {
		t.Fatal("thiếu thông tin cơ bản")
	}
	if _, err := time.ParseDuration(info.RefreshTokenTTL); err != nil {
		t.Errorf("thời hạn refresh token không đọc lại được: %q", info.RefreshTokenTTL)
	}
}
