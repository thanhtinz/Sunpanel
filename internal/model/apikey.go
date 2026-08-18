package model

import "time"

// APIKey là một khóa truy cập API dành cho script và hệ thống bên ngoài.
//
// Khóa tồn tại để tự động hóa mà không phải nhúng mật khẩu tài khoản người dùng
// vào script: khóa thu hồi được riêng lẻ, giới hạn quyền được, và không dùng để
// đăng nhập giao diện.
type APIKey struct {
	ID   uint   `gorm:"primaryKey" json:"id"`
	Name string `gorm:"size:64;not null" json:"name"`
	// Prefix là phần đầu của khóa, hiển thị để người dùng nhận ra khóa nào là khóa nào.
	Prefix string `gorm:"size:16;index;not null" json:"prefix"`
	// Hash là băm SHA-256 của khóa đầy đủ.
	//
	// Chỉ lưu băm, không lưu khóa: cơ sở dữ liệu bị lộ thì kẻ tấn công vẫn không
	// dùng được khóa. Đổi lại, khóa chỉ hiện đúng một lần lúc tạo.
	Hash string `gorm:"size:64;uniqueIndex;not null" json:"-"`
	// UserID là chủ sở hữu khóa; khóa mang đúng quyền của tài khoản đó.
	UserID uint `gorm:"index;not null" json:"userId"`
	// Role giới hạn quyền của khóa, không được cao hơn quyền của chủ sở hữu.
	Role string `gorm:"size:16;not null" json:"role"`
	// ExpiresAt là hạn dùng; rỗng nghĩa là không hết hạn.
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	// LastUsedAt và LastIP cho biết khóa còn được dùng không và từ đâu.
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
	LastIP     string     `gorm:"size:64" json:"lastIp,omitempty"`
	Enabled    bool       `gorm:"not null" json:"enabled"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// TableName đặt tên bảng.
func (APIKey) TableName() string { return "api_keys" }

// Expired cho biết khóa đã hết hạn hay chưa.
func (k APIKey) Expired() bool {
	return k.ExpiresAt != nil && time.Now().After(*k.ExpiresAt)
}
