// Package model định nghĩa các thực thể lưu trong cơ sở dữ liệu của panel.
package model

import "time"

// Role là vai trò của người dùng, quyết định phạm vi thao tác được phép.
type Role string

// Các vai trò được hỗ trợ.
const (
	// RoleAdmin toàn quyền, kể cả quản lý người dùng khác.
	RoleAdmin Role = "admin"
	// RoleOperator vận hành được hệ thống nhưng không quản lý người dùng.
	RoleOperator Role = "operator"
	// RoleReadonly chỉ được xem.
	RoleReadonly Role = "readonly"
)

// Valid cho biết vai trò có nằm trong tập được hỗ trợ hay không.
func (r Role) Valid() bool {
	switch r {
	case RoleAdmin, RoleOperator, RoleReadonly:
		return true
	}
	return false
}

// User là tài khoản đăng nhập vào panel.
type User struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	Username     string `gorm:"uniqueIndex;size:64;not null" json:"username"`
	PasswordHash string `gorm:"size:255;not null" json:"-"`
	Email        string `gorm:"size:255" json:"email"`
	Role         Role   `gorm:"size:32;not null;default:readonly" json:"role"`
	// Language là ngôn ngữ giao diện ưa thích, ví dụ "vi" hoặc "en".
	Language string `gorm:"size:8;not null;default:vi" json:"language"`
	// Theme là chế độ sáng/tối ưa thích.
	Theme string `gorm:"size:16;not null;default:auto" json:"theme"`

	// TOTPSecret được mã hóa at-rest, chỉ có giá trị khi người dùng bật 2FA.
	TOTPSecret  string `gorm:"size:512" json:"-"`
	TOTPEnabled bool   `gorm:"not null;default:false" json:"totpEnabled"`

	// Active cho phép vô hiệu hóa tài khoản mà không xóa lịch sử của họ.
	Active bool `gorm:"not null;default:true" json:"active"`
	// MustChangePassword buộc đổi mật khẩu ở lần đăng nhập kế tiếp.
	MustChangePassword bool `gorm:"not null;default:false" json:"mustChangePassword"`

	// FailedAttempts và LockedUntil phục vụ cơ chế khóa sau nhiều lần sai mật khẩu.
	FailedAttempts int        `gorm:"not null;default:0" json:"-"`
	LockedUntil    *time.Time `json:"lockedUntil,omitempty"`

	LastLoginAt *time.Time `json:"lastLoginAt,omitempty"`
	LastLoginIP string     `gorm:"size:64" json:"lastLoginIP,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// TableName đặt tên bảng rõ ràng thay vì để GORM tự suy ra.
func (User) TableName() string { return "users" }

// IsLocked cho biết tài khoản có đang bị khóa tạm thời hay không.
func (u *User) IsLocked(now time.Time) bool {
	return u.LockedUntil != nil && u.LockedUntil.After(now)
}
