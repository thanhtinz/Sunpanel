package model

import "time"

// AuditLog ghi lại mọi hành động làm thay đổi trạng thái hệ thống.
// Bản ghi này chỉ được thêm, không sửa và không xóa từ giao diện.
type AuditLog struct {
	ID     uint `gorm:"primaryKey" json:"id"`
	UserID uint `gorm:"index" json:"userId"`
	// Username được lưu lại tại thời điểm hành động, phòng khi tài khoản bị xóa sau này.
	Username string `gorm:"size:64" json:"username"`
	// Action là mã hành động dạng "resource.verb", ví dụ "user.create".
	Action string `gorm:"index;size:64;not null" json:"action"`
	// Resource là đối tượng bị tác động, ví dụ đường dẫn tệp hoặc tên dịch vụ.
	Resource string `gorm:"size:512" json:"resource"`
	IP       string `gorm:"size:64" json:"ip"`
	// Success cho biết hành động có thành công hay không.
	Success bool `gorm:"not null" json:"success"`
	// Detail chứa thông tin bổ sung dạng JSON, không bao giờ chứa bí mật.
	Detail    string    `gorm:"type:text" json:"detail,omitempty"`
	CreatedAt time.Time `gorm:"index" json:"createdAt"`
}

// TableName đặt tên bảng.
func (AuditLog) TableName() string { return "audit_logs" }

// LoginLog ghi lại các lần đăng nhập thành công và thất bại, phục vụ phát hiện
// tấn công dò mật khẩu.
type LoginLog struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	Username  string `gorm:"index;size:64" json:"username"`
	IP        string `gorm:"size:64" json:"ip"`
	UserAgent string `gorm:"size:512" json:"userAgent"`
	Success   bool   `gorm:"not null" json:"success"`
	// Reason là mã lý do thất bại để giao diện tự dịch, ví dụ "auth.invalid_credentials".
	Reason    string    `gorm:"size:64" json:"reason,omitempty"`
	CreatedAt time.Time `gorm:"index" json:"createdAt"`
}

// TableName đặt tên bảng.
func (LoginLog) TableName() string { return "login_logs" }
