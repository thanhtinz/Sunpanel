package service

import (
	"context"
	"errors"
	"strings"
	"unicode"

	"gorm.io/gorm"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/model"
	"github.com/thanhtinz/sunpanel/pkg/crypto"
)

// UserService quản lý tài khoản người dùng của panel.
type UserService struct {
	db    *gorm.DB
	auth  *AuthService
	audit *AuditService
}

// NewUserService tạo service quản lý người dùng.
func NewUserService(db *gorm.DB, auth *AuthService, audit *AuditService) *UserService {
	return &UserService{db: db, auth: auth, audit: audit}
}

// CreateUserRequest là dữ liệu tạo người dùng mới.
type CreateUserRequest struct {
	Username string     `json:"username" binding:"required"`
	Password string     `json:"password" binding:"required"`
	Email    string     `json:"email"`
	Role     model.Role `json:"role" binding:"required"`
	Language string     `json:"language"`
}

// UpdateUserRequest là dữ liệu cập nhật người dùng; trường nil nghĩa là giữ nguyên.
type UpdateUserRequest struct {
	Email    *string     `json:"email"`
	Role     *model.Role `json:"role"`
	Active   *bool       `json:"active"`
	Language *string     `json:"language"`
	Theme    *string     `json:"theme"`
}

// Get lấy một người dùng theo ID.
func (s *UserService) Get(ctx context.Context, id uint) (*model.User, error) {
	var user model.User
	if err := s.db.WithContext(ctx).First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.UserNotFound
		}
		return nil, apperr.Internal.Wrap(err)
	}
	return &user, nil
}

// List liệt kê toàn bộ người dùng.
func (s *UserService) List(ctx context.Context) ([]model.User, error) {
	var users []model.User
	if err := s.db.WithContext(ctx).Order("id ASC").Find(&users).Error; err != nil {
		return nil, apperr.Internal.Wrap(err)
	}
	return users, nil
}

// Create tạo người dùng mới.
func (s *UserService) Create(ctx context.Context, req CreateUserRequest) (*model.User, error) {
	username := strings.TrimSpace(req.Username)
	if username == "" {
		return nil, apperr.BadRequest
	}
	if !req.Role.Valid() {
		return nil, apperr.InvalidRole
	}
	if err := ValidatePassword(req.Password); err != nil {
		return nil, err
	}

	var count int64
	if err := s.db.WithContext(ctx).Model(&model.User{}).Where("username = ?", username).Count(&count).Error; err != nil {
		return nil, apperr.Internal.Wrap(err)
	}
	if count > 0 {
		return nil, apperr.UsernameTaken
	}

	hash, err := crypto.HashPassword(req.Password)
	if err != nil {
		return nil, apperr.Internal.Wrap(err)
	}

	language := req.Language
	if language == "" {
		language = "vi"
	}

	user := model.User{
		Username:     username,
		PasswordHash: hash,
		Email:        strings.TrimSpace(req.Email),
		Role:         req.Role,
		Language:     language,
		Theme:        "auto",
		Active:       true,
	}
	if err := s.db.WithContext(ctx).Create(&user).Error; err != nil {
		return nil, apperr.Internal.Wrap(err)
	}

	return &user, nil
}

// Update sửa thông tin người dùng.
func (s *UserService) Update(ctx context.Context, id uint, req UpdateUserRequest) (*model.User, error) {
	user, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	updates := map[string]any{}
	if req.Email != nil {
		updates["email"] = strings.TrimSpace(*req.Email)
	}
	if req.Language != nil {
		updates["language"] = *req.Language
	}
	if req.Theme != nil {
		updates["theme"] = *req.Theme
	}
	if req.Role != nil {
		if !req.Role.Valid() {
			return nil, apperr.InvalidRole
		}
		// Hạ quyền quản trị viên cuối cùng sẽ khiến không ai còn quản trị được panel.
		if user.Role == model.RoleAdmin && *req.Role != model.RoleAdmin {
			if err := s.ensureNotLastAdmin(ctx, id); err != nil {
				return nil, err
			}
		}
		updates["role"] = *req.Role
	}
	if req.Active != nil {
		if !*req.Active && user.Role == model.RoleAdmin {
			if err := s.ensureNotLastAdmin(ctx, id); err != nil {
				return nil, err
			}
		}
		updates["active"] = *req.Active
	}

	if len(updates) == 0 {
		return user, nil
	}
	if err := s.db.WithContext(ctx).Model(user).Updates(updates).Error; err != nil {
		return nil, apperr.Internal.Wrap(err)
	}

	// Tài khoản bị vô hiệu hóa phải bị đá ra khỏi mọi phiên đang mở ngay lập tức.
	if req.Active != nil && !*req.Active {
		if err := s.auth.RevokeAllSessions(ctx, id); err != nil {
			return nil, err
		}
	}

	return s.Get(ctx, id)
}

// Delete xóa một người dùng.
func (s *UserService) Delete(ctx context.Context, id, actorID uint) error {
	if id == actorID {
		return apperr.CannotDeleteSelf
	}

	user, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if user.Role == model.RoleAdmin {
		if err := s.ensureNotLastAdmin(ctx, id); err != nil {
			return err
		}
	}

	if err := s.auth.RevokeAllSessions(ctx, id); err != nil {
		return err
	}
	if err := s.db.WithContext(ctx).Delete(&model.User{}, id).Error; err != nil {
		return apperr.Internal.Wrap(err)
	}
	return nil
}

// ChangePassword đổi mật khẩu của chính người dùng đang đăng nhập.
//
// Sau khi đổi, mọi phiên khác bị thu hồi: nếu mật khẩu bị lộ và kẻ tấn công đang
// giữ một phiên, việc đổi mật khẩu phải cắt đứt được phiên đó.
func (s *UserService) ChangePassword(ctx context.Context, id uint, currentPassword, newPassword string) error {
	user, err := s.Get(ctx, id)
	if err != nil {
		return err
	}

	ok, err := verifyPasswordOf(user, currentPassword)
	if err != nil {
		return err
	}
	if !ok {
		return apperr.PasswordMismatch
	}
	if err := ValidatePassword(newPassword); err != nil {
		return err
	}

	hash, err := crypto.HashPassword(newPassword)
	if err != nil {
		return apperr.Internal.Wrap(err)
	}
	err = s.db.WithContext(ctx).Model(user).Updates(map[string]any{
		"password_hash":        hash,
		"must_change_password": false,
	}).Error
	if err != nil {
		return apperr.Internal.Wrap(err)
	}

	if err := s.auth.RevokeAllSessions(ctx, id); err != nil {
		return err
	}

	s.audit.Record(ctx, AuditEntry{
		UserID: user.ID, Username: user.Username,
		Action: "user.change_password", Success: true,
	})
	return nil
}

// ResetPassword đặt lại mật khẩu của người khác; chỉ quản trị viên được gọi.
func (s *UserService) ResetPassword(ctx context.Context, id uint, newPassword string) error {
	if err := ValidatePassword(newPassword); err != nil {
		return err
	}
	user, err := s.Get(ctx, id)
	if err != nil {
		return err
	}

	hash, err := crypto.HashPassword(newPassword)
	if err != nil {
		return apperr.Internal.Wrap(err)
	}
	err = s.db.WithContext(ctx).Model(user).Updates(map[string]any{
		"password_hash":        hash,
		"must_change_password": true,
		"failed_attempts":      0,
		"locked_until":         nil,
	}).Error
	if err != nil {
		return apperr.Internal.Wrap(err)
	}

	return s.auth.RevokeAllSessions(ctx, id)
}

// ensureNotLastAdmin trả về lỗi nếu excludeID là quản trị viên hoạt động cuối cùng.
func (s *UserService) ensureNotLastAdmin(ctx context.Context, excludeID uint) error {
	var count int64
	err := s.db.WithContext(ctx).Model(&model.User{}).
		Where("role = ? AND active = ? AND id <> ?", model.RoleAdmin, true, excludeID).
		Count(&count).Error
	if err != nil {
		return apperr.Internal.Wrap(err)
	}
	if count == 0 {
		return apperr.LastAdmin
	}
	return nil
}

// ValidatePassword kiểm tra mật khẩu có đạt yêu cầu tối thiểu hay không:
// đủ dài và có ít nhất hai loại ký tự khác nhau.
func ValidatePassword(password string) error {
	if len([]rune(password)) < minPasswordLength {
		return apperr.PasswordTooWeak.WithParam("min", minPasswordLength)
	}

	var hasLower, hasUpper, hasDigit, hasSymbol bool
	for _, r := range password {
		switch {
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsDigit(r):
			hasDigit = true
		default:
			hasSymbol = true
		}
	}

	classes := 0
	for _, ok := range []bool{hasLower, hasUpper, hasDigit, hasSymbol} {
		if ok {
			classes++
		}
	}
	if classes < 2 {
		return apperr.PasswordTooWeak.WithParam("min", minPasswordLength)
	}
	return nil
}

// verifyPasswordOf so khớp mật khẩu với bản băm của người dùng.
func verifyPasswordOf(user *model.User, password string) (bool, error) {
	ok, err := crypto.VerifyPassword(password, user.PasswordHash)
	if err != nil {
		return false, apperr.Internal.Wrap(err)
	}
	return ok, nil
}
