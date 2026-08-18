package compose

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thanhtinz/sunpanel/pkg/host"
)

func newCompose(t *testing.T) *Compose {
	t.Helper()
	return New(host.NewLocalHost("/", []string{"docker", "docker-compose"}))
}

func TestDetectCompose(t *testing.T) {
	c := newCompose(t)
	ctx := context.Background()

	if !c.Available(ctx) {
		t.Skip("máy không có Docker Compose")
	}

	version, err := c.Version(ctx)
	if err != nil {
		t.Fatalf("đọc phiên bản: %v", err)
	}
	if version == "" {
		t.Error("phiên bản compose rỗng")
	}
	t.Logf("compose: %s %s (phiên bản %s)", c.binary, strings.Join(c.baseArgs, " "), version)
}

// Tệp compose sai cú pháp phải bị bắt trước khi panel ghi nhận đã cài ứng dụng.
func TestConfigRejectsBrokenFile(t *testing.T) {
	c := newCompose(t)
	ctx := context.Background()
	if !c.Available(ctx) {
		t.Skip("máy không có Docker Compose")
	}

	dir := t.TempDir()
	broken := "services:\n  a:\n    image: [khong phai chuoi]\n"
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte(broken), 0o600); err != nil {
		t.Fatalf("ghi tệp compose: %v", err)
	}

	if _, err := c.Config(ctx, dir); err == nil {
		t.Error("tệp compose hỏng phải bị từ chối")
	}
}

// Giá trị trong .env phải thực sự thay được vào tệp compose, nếu không ứng dụng
// sẽ chạy với cổng và mật khẩu trống.
func TestConfigSubstitutesEnvFile(t *testing.T) {
	c := newCompose(t)
	ctx := context.Background()
	if !c.Available(ctx) {
		t.Skip("máy không có Docker Compose")
	}

	dir := t.TempDir()
	template := "services:\n  a:\n    image: ${IMAGE}\n    ports:\n      - \"${PORT}:80\"\n"
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte(template), 0o600); err != nil {
		t.Fatalf("ghi tệp compose: %v", err)
	}
	env := "IMAGE=sunpanel-test:local\nPORT=18099\n"
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(env), 0o600); err != nil {
		t.Fatalf("ghi tệp .env: %v", err)
	}

	out, err := c.Config(ctx, dir)
	if err != nil {
		t.Fatalf("kiểm tra cấu hình: %v", err)
	}
	if !strings.Contains(out, "sunpanel-test:local") {
		t.Errorf("biến IMAGE chưa được thay:\n%s", out)
	}
	if !strings.Contains(out, "18099") {
		t.Errorf("biến PORT chưa được thay:\n%s", out)
	}
}
