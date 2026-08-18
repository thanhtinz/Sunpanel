package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/model"
)

// keyPrefix đứng đầu mọi khóa API.
//
// Tiền tố cố định giúp nhận ra khóa khi nó vô tình lọt vào mã nguồn hay nhật ký,
// và cho phép các công cụ quét bí mật bắt được nó.
const keyPrefix = "sp_"

// prefixLength là số ký tự phần đầu khóa được lưu để hiển thị.
const prefixLength = 12

// APIKeyRequest là dữ liệu tạo một khóa API.
type APIKeyRequest struct {
	Name string `json:"name" binding:"required"`
	// Role giới hạn quyền của khóa; để trống thì dùng vai trò của chủ sở hữu.
	Role string `json:"role"`
	// ExpiresInDays là số ngày khóa còn hiệu lực; 0 nghĩa là không hết hạn.
	ExpiresInDays int `json:"expiresInDays"`
}

// CreatedAPIKey là khóa vừa tạo, kèm giá trị đầy đủ.
type CreatedAPIKey struct {
	model.APIKey
	// Key là giá trị đầy đủ, CHỈ trả về đúng một lần này.
	Key string `json:"key"`
}

// APIKeyService quản lý khóa truy cập API.
type APIKeyService struct {
	db    *gorm.DB
	audit *AuditService
}

// NewAPIKeyService tạo dịch vụ khóa API.
func NewAPIKeyService(db *gorm.DB, audit *AuditService) *APIKeyService {
	return &APIKeyService{db: db, audit: audit}
}

// List liệt kê khóa của một người dùng.
//
// Quản trị viên xem được mọi khóa; người dùng khác chỉ xem khóa của mình.
func (s *APIKeyService) List(ctx context.Context, userID uint, all bool) ([]model.APIKey, error) {
	query := s.db.WithContext(ctx).Order("created_at DESC")
	if !all {
		query = query.Where("user_id = ?", userID)
	}

	var keys []model.APIKey
	if err := query.Find(&keys).Error; err != nil {
		return nil, apperr.Internal.Wrap(err)
	}
	return keys, nil
}

// Create sinh một khóa API mới.
func (s *APIKeyService) Create(
	ctx context.Context, userID uint, ownerRole model.Role, req APIKeyRequest,
) (CreatedAPIKey, error) {
	role := model.Role(strings.TrimSpace(req.Role))
	if role == "" {
		role = ownerRole
	}
	if !role.Valid() {
		return CreatedAPIKey{}, apperr.APIKeyInvalidRole.WithParam("role", req.Role)
	}
	// Khóa không được mạnh hơn chính tài khoản tạo ra nó, nếu không một tài khoản
	// chỉ-xem có thể tự cấp cho mình quyền quản trị.
	if rolePower(role) > rolePower(ownerRole) {
		return CreatedAPIKey{}, apperr.APIKeyInvalidRole.WithParam("role", string(role))
	}

	raw, err := generateKey()
	if err != nil {
		return CreatedAPIKey{}, apperr.Internal.Wrap(err)
	}

	record := model.APIKey{
		Name:    strings.TrimSpace(req.Name),
		Prefix:  raw[:prefixLength],
		Hash:    hashKey(raw),
		UserID:  userID,
		Role:    string(role),
		Enabled: true,
	}
	if req.ExpiresInDays > 0 {
		expires := time.Now().AddDate(0, 0, req.ExpiresInDays)
		record.ExpiresAt = &expires
	}

	if err := s.db.WithContext(ctx).Create(&record).Error; err != nil {
		return CreatedAPIKey{}, apperr.Internal.Wrap(err)
	}

	return CreatedAPIKey{APIKey: record, Key: raw}, nil
}

// Delete thu hồi một khóa.
//
// userID khác 0 giới hạn việc xóa trong phạm vi khóa của chính người đó.
func (s *APIKeyService) Delete(ctx context.Context, id uint, userID uint) error {
	query := s.db.WithContext(ctx).Where("id = ?", id)
	if userID != 0 {
		query = query.Where("user_id = ?", userID)
	}

	result := query.Delete(&model.APIKey{})
	if result.Error != nil {
		return apperr.Internal.Wrap(result.Error)
	}
	if result.RowsAffected == 0 {
		return apperr.APIKeyNotFound
	}
	return nil
}

// SetEnabled bật hoặc tắt một khóa.
func (s *APIKeyService) SetEnabled(ctx context.Context, id, userID uint, enabled bool) (model.APIKey, error) {
	query := s.db.WithContext(ctx).Where("id = ?", id)
	if userID != 0 {
		query = query.Where("user_id = ?", userID)
	}

	var key model.APIKey
	err := query.First(&key).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.APIKey{}, apperr.APIKeyNotFound
	}
	if err != nil {
		return model.APIKey{}, apperr.Internal.Wrap(err)
	}

	key.Enabled = enabled
	if err := s.db.WithContext(ctx).Model(&key).Update("enabled", enabled).Error; err != nil {
		return model.APIKey{}, apperr.Internal.Wrap(err)
	}
	return key, nil
}

// Authenticate xác thực một khóa và trả về người dùng cùng vai trò hiệu lực.
func (s *APIKeyService) Authenticate(ctx context.Context, raw, ip string) (model.User, model.Role, error) {
	if !strings.HasPrefix(raw, keyPrefix) || len(raw) < prefixLength {
		return model.User{}, "", apperr.APIKeyInvalid
	}

	// Tra theo băm chứ không theo tiền tố: tiền tố không phải là bí mật, còn băm
	// thì chỉ ai có khóa đầy đủ mới tính ra được.
	var key model.APIKey
	err := s.db.WithContext(ctx).Where("hash = ?", hashKey(raw)).First(&key).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.User{}, "", apperr.APIKeyInvalid
	}
	if err != nil {
		return model.User{}, "", apperr.Internal.Wrap(err)
	}

	// So sánh lại bằng hàm chống đo thời gian, phòng trường hợp lớp dưới trả về
	// bản ghi theo cách so sánh thường.
	if subtle.ConstantTimeCompare([]byte(key.Hash), []byte(hashKey(raw))) != 1 {
		return model.User{}, "", apperr.APIKeyInvalid
	}
	if !key.Enabled || key.Expired() {
		return model.User{}, "", apperr.APIKeyInvalid
	}

	var user model.User
	if err := s.db.WithContext(ctx).First(&user, key.UserID).Error; err != nil {
		return model.User{}, "", apperr.APIKeyInvalid
	}
	// Tài khoản bị vô hiệu hóa hoặc đang bị khóa tạm thì mọi khóa API của nó cũng
	// mất hiệu lực — nếu không, khóa API sẽ là đường vòng qua chính cơ chế khóa
	// tài khoản.
	if !user.Active || user.IsLocked(time.Now()) {
		return model.User{}, "", apperr.APIKeyInvalid
	}

	// Vai trò hiệu lực là mức thấp hơn giữa vai trò khóa và vai trò hiện tại của
	// người dùng: hạ quyền một tài khoản phải hạ luôn quyền các khóa của nó.
	effective := model.Role(key.Role)
	if rolePower(user.Role) < rolePower(effective) {
		effective = user.Role
	}

	s.touch(key.ID, ip)
	return user, effective, nil
}

// touch cập nhật thời điểm và địa chỉ dùng khóa gần nhất.
//
// Ghi nền để việc xác thực không phải chờ một lần ghi đĩa; mất một bản cập nhật
// khi máy chủ tắt đột ngột không ảnh hưởng gì.
func (s *APIKeyService) touch(id uint, ip string) {
	go func() {
		updates := map[string]any{"last_used_at": time.Now(), "last_ip": ip}
		if err := s.db.Model(&model.APIKey{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			slog.Warn("không cập nhật được thời điểm dùng khóa API", "key", id, "error", err)
		}
	}()
}

// generateKey sinh một khóa API ngẫu nhiên.
func generateKey() (string, error) {
	buf := make([]byte, 30)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("sinh khóa API: %w", err)
	}
	// base64 không dấu để khóa dùng được trong tiêu đề HTTP và dòng lệnh mà không
	// phải trích dẫn.
	return keyPrefix + base64.RawURLEncoding.EncodeToString(buf), nil
}

// hashKey băm một khóa để lưu và tra cứu.
func hashKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// rolePower xếp hạng vai trò để so sánh quyền.
func rolePower(role model.Role) int {
	switch role {
	case model.RoleAdmin:
		return 3
	case model.RoleOperator:
		return 2
	case model.RoleReadonly:
		return 1
	}
	return 0
}
