//go:build !windows

package config

// defaultDataDir là nơi panel lưu dữ liệu trên Linux và macOS.
func defaultDataDir() string { return "/opt/sunpanel" }

// defaultFileRoot là phạm vi mặc định của trình quản lý tệp.
// Mặc định là toàn bộ hệ thống tệp vì đây là công cụ quản trị máy chủ;
// quản trị viên có thể thu hẹp lại trong cấu hình.
func defaultFileRoot() string { return "/" }

// defaultNginxConfDir là nơi panel ghi tệp vhost.
//
// Cả Debian/Ubuntu lẫn RHEL/Rocky đều include sẵn thư mục này trong nginx.conf,
// nên panel không phải sửa cấu hình gốc của hệ thống.
func defaultNginxConfDir() string { return "/etc/nginx/conf.d" }

// defaultWebRoot là thư mục mặc định chứa mã nguồn các website.
func defaultWebRoot() string { return "/www/wwwroot" }

// defaultACMEWebroot là thư mục phục vụ tệp xác thực ACME.
//
// Nằm dưới /var/www chứ không phải thư mục dữ liệu của panel, vì máy chủ web
// chạy dưới người dùng khác và phải đọc được thư mục này.
func defaultACMEWebroot() string { return "/var/www/sunpanel-acme" }

// defaultAuthDir là nơi ghi tệp tài khoản bảo vệ website.
//
// Cùng chỗ với thư mục xác thực ACME và cùng lý do: máy chủ web chạy dưới người
// dùng khác và phải đọc được, nên nó không thể nằm trong thư mục dữ liệu 0700
// của panel.
func defaultAuthDir() string { return "/var/www/sunpanel-auth" }

// defaultSystemLogDir là thư mục nhật ký của hệ điều hành.
func defaultSystemLogDir() string { return "/var/log" }
