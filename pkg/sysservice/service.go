// Package sysservice quản lý các dịch vụ hệ thống của máy chủ.
//
// Package chỉ định nghĩa giao diện chung; mỗi hệ init có một driver riêng
// (systemd hiện tại, có thể thêm OpenRC hoặc Windows Service sau).
package sysservice

import (
	"context"
	"errors"
)

// Các lỗi dùng chung của quản lý dịch vụ.
var (
	// ErrManagerUnavailable là hệ thống không có trình quản lý dịch vụ nào dùng được.
	ErrManagerUnavailable = errors.New("sysservice: hệ thống không có trình quản lý dịch vụ khả dụng")
	// ErrServiceNotFound là không tìm thấy dịch vụ.
	ErrServiceNotFound = errors.New("sysservice: không tìm thấy dịch vụ")
	// ErrInvalidName là tên dịch vụ không hợp lệ.
	ErrInvalidName = errors.New("sysservice: tên dịch vụ không hợp lệ")
)

// State là trạng thái chạy của một dịch vụ.
type State string

// Các trạng thái chạy.
const (
	// StateActive là dịch vụ đang chạy.
	StateActive State = "active"
	// StateInactive là dịch vụ đã dừng.
	StateInactive State = "inactive"
	// StateFailed là dịch vụ dừng do lỗi.
	StateFailed State = "failed"
	// StateActivating và StateDeactivating là các trạng thái chuyển tiếp.
	StateActivating   State = "activating"
	StateDeactivating State = "deactivating"
	// StateUnknown là không xác định được trạng thái.
	StateUnknown State = "unknown"
)

// Service là thông tin một dịch vụ hệ thống.
type Service struct {
	// Name là tên dịch vụ, ví dụ "nginx.service".
	Name string `json:"name"`
	// Description là mô tả do chính dịch vụ khai báo.
	Description string `json:"description"`
	// State là trạng thái chạy hiện tại.
	State State `json:"state"`
	// SubState là trạng thái chi tiết của hệ init, ví dụ "running" hay "exited".
	SubState string `json:"subState"`
	// Enabled cho biết dịch vụ có tự khởi động cùng máy chủ hay không.
	Enabled bool `json:"enabled"`
	// EnabledState là chuỗi gốc từ hệ init: enabled, disabled, static, masked…
	EnabledState string `json:"enabledState"`
	// Loaded cho biết hệ init đã nạp được tệp cấu hình của dịch vụ chưa.
	Loaded bool `json:"loaded"`
}

// Running cho biết dịch vụ có đang chạy hay không.
func (s Service) Running() bool { return s.State == StateActive }

// Manager là trình quản lý dịch vụ của hệ thống.
type Manager interface {
	// Name là tên hệ init, dùng để hiển thị.
	Name() string
	// Available cho biết trình quản lý có dùng được trên máy này không.
	Available(ctx context.Context) bool
	// List liệt kê toàn bộ dịch vụ.
	List(ctx context.Context) ([]Service, error)
	// Get lấy thông tin một dịch vụ.
	Get(ctx context.Context, name string) (Service, error)
	// Start, Stop, Restart, Reload điều khiển vòng đời dịch vụ.
	Start(ctx context.Context, name string) error
	Stop(ctx context.Context, name string) error
	Restart(ctx context.Context, name string) error
	Reload(ctx context.Context, name string) error
	// Enable và Disable bật/tắt tự khởi động cùng máy chủ.
	Enable(ctx context.Context, name string) error
	Disable(ctx context.Context, name string) error
	// Logs lấy các dòng nhật ký gần nhất của dịch vụ.
	Logs(ctx context.Context, name string, lines int) (string, error)
}
