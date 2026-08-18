package service

import (
	"context"
	"errors"
	"strings"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/pkg/sysuser"
)

// SystemUserService quản lý tài khoản đăng nhập của máy chủ và khóa SSH.
type SystemUserService struct {
	manager *sysuser.Manager
	audit   *AuditService
}

// NewSystemUserService tạo dịch vụ quản lý tài khoản máy chủ.
func NewSystemUserService(manager *sysuser.Manager, audit *AuditService) *SystemUserService {
	return &SystemUserService{manager: manager, audit: audit}
}

// SystemUserStatus cho biết máy này quản lý tài khoản được hay không.
type SystemUserStatus struct {
	Available bool `json:"available"`
}

// SystemUserRequest là yêu cầu tạo tài khoản mới.
type SystemUserRequest struct {
	Name       string `json:"name" binding:"required"`
	Comment    string `json:"comment"`
	Shell      string `json:"shell"`
	Password   string `json:"password"`
	CreateHome bool   `json:"createHome"`
	Sudo       bool   `json:"sudo"`
}

// Status kiểm tra công cụ quản lý tài khoản có dùng được không.
func (s *SystemUserService) Status(ctx context.Context) SystemUserStatus {
	return SystemUserStatus{Available: s.manager.Available(ctx)}
}

// List liệt kê tài khoản trên máy.
func (s *SystemUserService) List(ctx context.Context) ([]sysuser.User, error) {
	users, err := s.manager.List(ctx)
	return users, translateSysUserError(err)
}

// Create tạo tài khoản mới.
func (s *SystemUserService) Create(ctx context.Context, req SystemUserRequest, actor AuditEntry) error {
	err := s.manager.Create(ctx, sysuser.CreateRequest{
		Name:       strings.TrimSpace(req.Name),
		Comment:    req.Comment,
		Shell:      req.Shell,
		Password:   req.Password,
		CreateHome: req.CreateHome,
		Sudo:       req.Sudo,
	})
	return s.record(ctx, "sysuser.create", req.Name, err, actor)
}

// SetPassword đặt lại mật khẩu cho một tài khoản.
func (s *SystemUserService) SetPassword(ctx context.Context, name, password string, actor AuditEntry) error {
	err := s.manager.SetPassword(ctx, name, password)
	return s.record(ctx, "sysuser.set_password", name, err, actor)
}

// SetLocked khóa hoặc mở khóa đăng nhập bằng mật khẩu.
func (s *SystemUserService) SetLocked(ctx context.Context, name string, locked bool, actor AuditEntry) error {
	action := "sysuser.unlock"
	if locked {
		action = "sysuser.lock"
	}
	err := s.manager.SetLocked(ctx, name, locked)
	return s.record(ctx, action, name, err, actor)
}

// SetSudo thêm hoặc bỏ quyền quản trị của một tài khoản.
func (s *SystemUserService) SetSudo(ctx context.Context, name string, sudo bool, actor AuditEntry) error {
	err := s.manager.SetSudo(ctx, name, sudo)
	return s.record(ctx, "sysuser.set_sudo", name, err, actor)
}

// Delete xóa tài khoản, tùy chọn xóa cả thư mục nhà.
func (s *SystemUserService) Delete(ctx context.Context, name string, removeHome bool, actor AuditEntry) error {
	err := s.manager.Delete(ctx, name, removeHome)
	return s.record(ctx, "sysuser.delete", name, err, actor)
}

// Keys liệt kê khóa SSH của một tài khoản.
func (s *SystemUserService) Keys(ctx context.Context, name string) ([]sysuser.Key, error) {
	keys, err := s.manager.Keys(ctx, name)
	return keys, translateSysUserError(err)
}

// AddKey gắn thêm khóa công khai cho tài khoản.
func (s *SystemUserService) AddKey(ctx context.Context, name, raw string, actor AuditEntry) (sysuser.Key, error) {
	key, err := s.manager.AddKey(ctx, name, raw)
	return key, s.record(ctx, "sysuser.add_key", name, err, actor)
}

// RemoveKey gỡ một khóa công khai khỏi tài khoản.
func (s *SystemUserService) RemoveKey(ctx context.Context, name, fingerprint string, actor AuditEntry) error {
	err := s.manager.RemoveKey(ctx, name, fingerprint)
	return s.record(ctx, "sysuser.remove_key", name+" "+fingerprint, err, actor)
}

// record ghi nhật ký kiểm toán rồi trả lại lỗi đã dịch sang mã lỗi của panel.
func (s *SystemUserService) record(ctx context.Context, action, resource string, err error, actor AuditEntry) error {
	actor.Action = action
	actor.Resource = resource
	actor.Success = err == nil
	s.audit.Record(ctx, actor)
	return translateSysUserError(err)
}

func translateSysUserError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, sysuser.ErrUnavailable):
		return apperr.SysUserUnavailable
	case errors.Is(err, sysuser.ErrInvalidName):
		return apperr.SysUserInvalidName
	case errors.Is(err, sysuser.ErrNotFound):
		return apperr.SysUserNotFound
	case errors.Is(err, sysuser.ErrExists):
		return apperr.SysUserExists
	case errors.Is(err, sysuser.ErrProtected):
		return apperr.SysUserProtected
	case errors.Is(err, sysuser.ErrInvalidKey):
		return apperr.SysUserInvalidKey
	case errors.Is(err, sysuser.ErrKeyExists):
		return apperr.SysUserKeyExists
	case errors.Is(err, sysuser.ErrKeyNotFound):
		return apperr.SysUserKeyNotFound
	default:
		// Thông báo gốc của useradd/usermod nói đúng vấn đề hơn bất cứ câu chữ
		// nào panel tự nghĩ ra, nên nó được đưa nguyên vẹn xuống giao diện.
		return apperr.SysUserActionFailed.WithParam("message", err.Error())
	}
}
