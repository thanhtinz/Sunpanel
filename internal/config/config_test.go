package config

import (
	"path/filepath"
	"strings"
	"testing"
)

// Thư mục dữ liệu để quyền 0700 vì chứa khóa chủ và cơ sở dữ liệu. Máy chủ web
// chạy dưới người dùng khác (www-data, nginx) nên không đi vào được — đặt thư
// mục xác thực ACME bên trong đó khiến mọi lần xin chứng chỉ nhận 403.
func TestACMEWebrootIsOutsideDataDir(t *testing.T) {
	cfg := Default()

	dataDir := filepath.Clean(cfg.Server.DataDir)
	webroot := filepath.Clean(cfg.Website.ACMEWebroot)

	if webroot == dataDir || strings.HasPrefix(webroot, dataDir+string(filepath.Separator)) {
		t.Errorf("thư mục xác thực ACME %q nằm trong thư mục dữ liệu %q", webroot, dataDir)
	}
}

// Các đường dẫn bí mật thì ngược lại: phải nằm trong thư mục dữ liệu để thừa
// hưởng quyền 0700 của nó.
func TestSecretPathsStayInsideDataDir(t *testing.T) {
	cfg := Default()
	dataDir := filepath.Clean(cfg.Server.DataDir) + string(filepath.Separator)

	paths := map[string]string{
		"khóa chủ":            cfg.MasterKeyPath(),
		"khóa tài khoản ACME": cfg.ACMEAccountKeyPath(),
		"kho chứng chỉ":       cfg.CertDir(),
		"cơ sở dữ liệu":       cfg.Database.Path,
	}
	for name, path := range paths {
		if !strings.HasPrefix(filepath.Clean(path), dataDir) {
			t.Errorf("%s (%q) phải nằm trong thư mục dữ liệu %q", name, path, dataDir)
		}
	}
}
