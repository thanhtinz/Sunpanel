//go:build windows

package config

import (
	"os"
	"path/filepath"
)

// defaultDataDir là nơi panel lưu dữ liệu trên Windows.
func defaultDataDir() string {
	if dir := os.Getenv("ProgramData"); dir != "" {
		return filepath.Join(dir, "SunPanel")
	}
	return filepath.Join("C:\\", "ProgramData", "SunPanel")
}

// defaultFileRoot là phạm vi mặc định của trình quản lý tệp trên Windows.
func defaultFileRoot() string {
	if dir := os.Getenv("SystemDrive"); dir != "" {
		return dir + "\\"
	}
	return "C:\\"
}

// defaultNginxConfDir là nơi panel ghi tệp vhost trên Windows.
//
// Windows không có quy ước chung cho nginx, nên dùng thư mục dữ liệu của chính
// panel; quản trị viên tự include nó vào nginx.conf.
func defaultNginxConfDir() string { return filepath.Join(defaultDataDir(), "nginx") }

// defaultWebRoot là thư mục mặc định chứa mã nguồn các website trên Windows.
func defaultWebRoot() string { return filepath.Join(defaultDataDir(), "www") }

// defaultACMEWebroot là thư mục phục vụ tệp xác thực ACME trên Windows.
func defaultACMEWebroot() string { return filepath.Join(defaultDataDir(), "acme-webroot") }
