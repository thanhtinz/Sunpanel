package model

import "time"

// Session là một phiên đăng nhập, tương ứng với một refresh token còn hiệu lực.
//
// Refresh token không bao giờ được lưu nguyên bản: chỉ lưu bản băm SHA-256, để
// kẻ đọc được cơ sở dữ liệu vẫn không mạo danh được phiên nào.
type Session struct {
	ID     uint `gorm:"primaryKey" json:"id"`
	UserID uint `gorm:"index;not null" json:"userId"`
	// TokenHash là SHA-256 của refresh token dạng hex.
	TokenHash string `gorm:"uniqueIndex;size:64;not null" json:"-"`
	// UserAgent và IP giúp người dùng nhận ra phiên nào là của thiết bị nào.
	UserAgent string     `gorm:"size:512" json:"userAgent"`
	IP        string     `gorm:"size:64" json:"ip"`
	ExpiresAt time.Time  `gorm:"index;not null" json:"expiresAt"`
	RevokedAt *time.Time `json:"revokedAt,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
	LastUsed  time.Time  `json:"lastUsed"`
}

// TableName đặt tên bảng.
func (Session) TableName() string { return "sessions" }

// Valid cho biết phiên còn dùng được tại thời điểm now.
func (s *Session) Valid(now time.Time) bool {
	return s.RevokedAt == nil && s.ExpiresAt.After(now)
}
