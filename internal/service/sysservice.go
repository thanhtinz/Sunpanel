package service

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/pkg/sysservice"
)

// protectedServices là các dịch vụ mà panel từ chối dừng hoặc tắt.
//
// Dừng sshd qua giao diện web là cách nhanh nhất để tự khóa mình khỏi máy chủ;
// dừng chính panel thì mất luôn đường quay lại. Khởi động lại vẫn được phép vì
// đó là thao tác hợp lệ và tự phục hồi.
var protectedServices = map[string]bool{
	"ssh.service":              true,
	"sshd.service":             true,
	"sunpanel.service":         true,
	"systemd-networkd.service": true,
	"network.service":          true,
	"NetworkManager.service":   true,
}

// SystemServiceManager quản lý các dịch vụ hệ thống.
type SystemServiceManager struct {
	manager sysservice.Manager
	audit   *AuditService
}

// NewSystemServiceManager tạo service quản lý dịch vụ hệ thống.
func NewSystemServiceManager(manager sysservice.Manager, audit *AuditService) *SystemServiceManager {
	return &SystemServiceManager{manager: manager, audit: audit}
}

// ManagerStatus là tình trạng của trình quản lý dịch vụ trên máy này.
type ManagerStatus struct {
	// Available cho biết có điều khiển được dịch vụ hay không.
	Available bool `json:"available"`
	// Manager là tên hệ init, ví dụ "systemd".
	Manager string `json:"manager"`
}

// Status kiểm tra trình quản lý dịch vụ có dùng được không.
func (s *SystemServiceManager) Status(ctx context.Context) ManagerStatus {
	return ManagerStatus{
		Available: s.manager.Available(ctx),
		Manager:   s.manager.Name(),
	}
}

// SystemService là thông tin một dịch vụ kèm cờ bảo vệ cho giao diện.
type SystemService struct {
	sysservice.Service
	// Protected cho biết panel từ chối dừng dịch vụ này.
	Protected bool `json:"protected"`
}

// List liệt kê dịch vụ, sắp xếp dịch vụ đang chạy lên trước.
func (s *SystemServiceManager) List(ctx context.Context) ([]SystemService, error) {
	services, err := s.manager.List(ctx)
	if err != nil {
		return nil, translateServiceError(err)
	}

	out := make([]SystemService, 0, len(services))
	for _, svc := range services {
		out = append(out, SystemService{Service: svc, Protected: protectedServices[svc.Name]})
	}

	// Dịch vụ lỗi lên đầu (cần chú ý nhất), rồi tới đang chạy, rồi theo tên.
	sort.Slice(out, func(i, j int) bool {
		ri, rj := statePriority(out[i].State), statePriority(out[j].State)
		if ri != rj {
			return ri < rj
		}
		return out[i].Name < out[j].Name
	})

	return out, nil
}

// Get lấy thông tin một dịch vụ.
func (s *SystemServiceManager) Get(ctx context.Context, name string) (SystemService, error) {
	svc, err := s.manager.Get(ctx, name)
	if err != nil {
		return SystemService{}, translateServiceError(err)
	}
	return SystemService{Service: svc, Protected: protectedServices[svc.Name]}, nil
}

// Action là thao tác điều khiển dịch vụ.
type Action string

// Các thao tác được hỗ trợ.
const (
	ActionStart   Action = "start"
	ActionStop    Action = "stop"
	ActionRestart Action = "restart"
	ActionReload  Action = "reload"
	ActionEnable  Action = "enable"
	ActionDisable Action = "disable"
)

// Control thực hiện một thao tác lên dịch vụ.
func (s *SystemServiceManager) Control(ctx context.Context, action Action, name string, actor AuditEntry) error {
	// Chuẩn hóa tên tại chỗ, không hỏi trình quản lý.
	//
	// Nếu để trình quản lý chuẩn hóa thì lớp bảo vệ sẽ phụ thuộc vào việc liệt kê
	// dịch vụ thành công: systemd không phản hồi, hoặc dịch vụ không nằm trong
	// danh sách, là tên thô được dùng và "ssh" sẽ không khớp "ssh.service" —
	// nghĩa là dừng được đúng cái dịch vụ mà panel phải bảo vệ.
	unit, err := sysservice.NormalizeUnit(name)
	if err != nil {
		return translateServiceError(err)
	}

	if protectedServices[unit] && (action == ActionStop || action == ActionDisable) {
		return apperr.ServiceProtected.WithParam("name", unit)
	}

	var actionErr error
	switch action {
	case ActionStart:
		actionErr = s.manager.Start(ctx, name)
	case ActionStop:
		actionErr = s.manager.Stop(ctx, name)
	case ActionRestart:
		actionErr = s.manager.Restart(ctx, name)
	case ActionReload:
		actionErr = s.manager.Reload(ctx, name)
	case ActionEnable:
		actionErr = s.manager.Enable(ctx, name)
	case ActionDisable:
		actionErr = s.manager.Disable(ctx, name)
	default:
		return apperr.BadRequest
	}

	actor.Action = "service." + string(action)
	actor.Resource = unit
	actor.Success = actionErr == nil
	s.audit.Record(ctx, actor)

	if actionErr != nil {
		return translateServiceError(actionErr)
	}
	return nil
}

// Logs lấy nhật ký gần nhất của một dịch vụ.
func (s *SystemServiceManager) Logs(ctx context.Context, name string, lines int) (string, error) {
	logs, err := s.manager.Logs(ctx, name, lines)
	if err != nil {
		return "", translateServiceError(err)
	}
	return logs, nil
}

// statePriority quyết định thứ tự hiển thị: lỗi trước, rồi đang chạy, rồi phần còn lại.
func statePriority(state sysservice.State) int {
	switch state {
	case sysservice.StateFailed:
		return 0
	case sysservice.StateActive, sysservice.StateActivating:
		return 1
	default:
		return 2
	}
}

// translateServiceError chuyển lỗi của lớp sysservice thành lỗi có mã dịch.
func translateServiceError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, sysservice.ErrManagerUnavailable):
		return apperr.ServiceManagerUnavailable.Wrap(err)
	case errors.Is(err, sysservice.ErrServiceNotFound):
		return apperr.ServiceNotFound.Wrap(err)
	case errors.Is(err, sysservice.ErrInvalidName):
		return apperr.ServiceInvalidName.Wrap(err)
	case strings.Contains(err.Error(), "not-found") || strings.Contains(err.Error(), "not found"):
		return apperr.ServiceNotFound.Wrap(err)
	default:
		return apperr.ServiceActionFailed.Wrap(err)
	}
}
