package service

import (
	"context"
	"errors"
	"net"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/pkg/appstore"
	"github.com/thanhtinz/sunpanel/pkg/compose"
	"github.com/thanhtinz/sunpanel/pkg/crypto"
	"github.com/thanhtinz/sunpanel/pkg/dockerx"
	"github.com/thanhtinz/sunpanel/pkg/host"
)

// newAppStoreFixture dựng dịch vụ chợ ứng dụng trên thư mục tạm.
//
// Không cần Docker: các bài test ở đây kiểm phần panel tự quyết định — kiểm tên,
// kiểm cổng, kiểm thư mục — vốn đều chạy trước khi compose được gọi.
func newAppStoreFixture(t *testing.T) (*AppStoreService, string) {
	t.Helper()

	db := newMemoryDB(t)
	root := t.TempDir()
	sealer, err := crypto.NewSealer(make([]byte, 32))
	if err != nil {
		t.Fatalf("khởi tạo bộ mã hóa: %v", err)
	}

	catalog, err := appstore.LoadBuiltin()
	if err != nil {
		t.Fatalf("nạp danh mục: %v", err)
	}

	h := host.NewLocalHost("/", []string{"docker", "docker-compose"})
	svc := NewAppStoreService(
		db, catalog, compose.New(h), dockerx.NewClient(), h, sealer,
		filepath.Join(root, "apps"), NewAuditService(db),
	)
	return svc, root
}

func TestInstallRejectsInvalidName(t *testing.T) {
	svc, _ := newAppStoreFixture(t)
	ctx := context.Background()

	if !svc.compose.Available(ctx) {
		t.Skip("máy không có Docker Compose")
	}

	// Tên đi thẳng vào tên thư mục và tên dự án compose.
	for _, name := range []string{"Có Dấu", "ten/khac", "../thoat", "", strings.Repeat("a", 64)} {
		_, err := svc.Install(ctx, InstallRequest{AppKey: "uptime-kuma", Name: name})
		if !errors.Is(err, apperr.AppInvalidName) {
			t.Errorf("tên %q phải bị từ chối, nhận: %v", name, err)
		}
	}
}

// Gỡ ứng dụng mà giữ dữ liệu để lại thư mục và volume cũ. Cài đè lên sẽ sinh
// mật khẩu mới trong .env trong khi cơ sở dữ liệu cũ vẫn dùng mật khẩu cũ.
func TestInstallRefusesExistingDirectory(t *testing.T) {
	svc, root := newAppStoreFixture(t)
	ctx := context.Background()

	if !svc.compose.Available(ctx) {
		t.Skip("máy không có Docker Compose")
	}

	dir := filepath.Join(root, "apps", "da-co")
	if err := svc.host.FS().Mkdir(ctx, dir, 0o700); err != nil {
		t.Fatalf("tạo thư mục sẵn có: %v", err)
	}

	_, err := svc.Install(ctx, InstallRequest{AppKey: "uptime-kuma", Name: "da-co"})
	if !errors.Is(err, apperr.AppDirExists) {
		t.Errorf("cài đè lên thư mục cũ phải bị từ chối, nhận: %v", err)
	}
}

// Cổng bị chiếm phải phát hiện trước khi tải image: compose chỉ báo lỗi ở bước
// cuối, sau khi đã tải hàng trăm megabyte.
func TestInstallDetectsPortInUse(t *testing.T) {
	svc, _ := newAppStoreFixture(t)
	ctx := context.Background()

	if !svc.compose.Available(ctx) {
		t.Skip("máy không có Docker Compose")
	}

	listener, err := listenAny(t)
	if err != nil {
		t.Fatalf("mở cổng thử nghiệm: %v", err)
	}
	defer listener.Close()

	// Điền đúng tên biến cổng của ứng dụng. Trước đây kiểm thử điền "PORT",
	// một biến Uptime Kuma không có, nên phép kiểm cổng luôn thấy cổng mặc định
	// còn trống và kiểm thử chỉ xanh khi tình cờ có thứ khác chiếm cổng đó.
	_, err = svc.Install(ctx, InstallRequest{
		AppKey: "uptime-kuma", Name: "cong-ban",
		Values: map[string]string{"HTTP_PORT": listener.port},
	})
	if !errors.Is(err, apperr.AppPortInUse) {
		t.Errorf("cổng đang bị chiếm phải bị từ chối, nhận: %v", err)
	}
}

// Tham số phải được mã hóa trong cơ sở dữ liệu: trong đó có mật khẩu quản trị
// và mật khẩu cơ sở dữ liệu của ứng dụng.
func TestInstalledParamsAreEncryptedAtRest(t *testing.T) {
	svc, _ := newAppStoreFixture(t)

	sealed, err := svc.sealParams(map[string]string{"DB_PASSWORD": "mat-khau-rat-bi-mat"})
	if err != nil {
		t.Fatalf("mã hóa tham số: %v", err)
	}
	if strings.Contains(sealed, "mat-khau-rat-bi-mat") {
		t.Errorf("mật khẩu còn đọc được trong chuỗi đã mã hóa: %s", sealed)
	}
}

// testListener là một cổng đang bị chiếm, dùng để thử phần kiểm cổng.
type testListener struct {
	port  string
	close func() error
}

func (l testListener) Close() error { return l.close() }

func listenAny(t *testing.T) (testListener, error) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return testListener{}, err
	}
	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		listener.Close()
		return testListener{}, err
	}
	return testListener{port: port, close: listener.Close}, nil
}
