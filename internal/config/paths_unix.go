//go:build !windows

package config

// defaultDataDir là nơi panel lưu dữ liệu trên Linux và macOS.
func defaultDataDir() string { return "/opt/sunpanel" }

// defaultFileRoot là phạm vi mặc định của trình quản lý tệp.
// Mặc định là toàn bộ hệ thống tệp vì đây là công cụ quản trị máy chủ;
// quản trị viên có thể thu hẹp lại trong cấu hình.
func defaultFileRoot() string { return "/" }
