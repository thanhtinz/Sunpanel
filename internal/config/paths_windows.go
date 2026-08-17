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
