package model

import "time"

// UptimeMonitor là một địa chỉ được theo dõi định kỳ.
type UptimeMonitor struct {
	ID   uint   `gorm:"primaryKey" json:"id"`
	Name string `gorm:"size:64;uniqueIndex;not null" json:"name"`
	URL  string `gorm:"size:512;not null" json:"url"`

	// IntervalSeconds là chu kỳ kiểm tra.
	//
	// Không đặt giá trị mặc định cho cột: GORM bỏ qua trường mang giá trị zero
	// khi cột có mặc định, và giá trị do tầng dịch vụ áp mới là nguồn duy nhất.
	IntervalSeconds int `gorm:"not null" json:"intervalSeconds"`
	// TimeoutSeconds là thời gian chờ tối đa của một lần kiểm tra.
	TimeoutSeconds int `gorm:"not null" json:"timeoutSeconds"`
	// ExpectedStatus là mã trạng thái bắt buộc; 0 nghĩa là mọi mã dưới 400.
	ExpectedStatus int `json:"expectedStatus"`
	// Keyword là chuỗi phải có trong nội dung trả về.
	Keyword string `gorm:"size:255" json:"keyword"`
	// SkipTLSVerify bỏ qua việc kiểm chứng chỉ, dùng cho dịch vụ nội bộ.
	SkipTLSVerify bool `json:"skipTlsVerify"`
	// FailureThreshold là số lần hỏng liên tiếp trước khi coi là mất kết nối.
	//
	// Một lần rớt gói tin không phải là sự cố; báo ngay lần đầu là cách nhanh
	// nhất để người dùng tắt hết cảnh báo sau vài đêm bị đánh thức.
	FailureThreshold int  `gorm:"not null" json:"failureThreshold"`
	Enabled          bool `json:"enabled"`

	// Status là trạng thái hiện tại: up, down hoặc unknown.
	Status string `gorm:"size:16" json:"status"`
	// ConsecutiveFails đếm số lần hỏng liên tiếp gần nhất.
	ConsecutiveFails int `json:"consecutiveFails"`
	// LastCheckedAt, LastLatencyMs, LastStatusCode và LastError mô tả lần kiểm
	// tra gần nhất.
	LastCheckedAt  *time.Time `json:"lastCheckedAt,omitempty"`
	LastLatencyMs  int64      `json:"lastLatencyMs"`
	LastStatusCode int        `json:"lastStatusCode"`
	LastError      string     `gorm:"type:text" json:"lastError,omitempty"`
	// LastChangedAt là lần đổi trạng thái gần nhất, dùng để nói "hỏng từ lúc nào".
	LastChangedAt *time.Time `json:"lastChangedAt,omitempty"`
	// CertExpiresIn là số ngày còn lại của chứng chỉ; -1 nghĩa là không có.
	CertExpiresIn int `json:"certExpiresIn"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// TableName đặt tên bảng.
func (UptimeMonitor) TableName() string { return "uptime_monitors" }

// UptimeCheck là kết quả một lần kiểm tra, giữ lại để vẽ lịch sử.
type UptimeCheck struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	MonitorID uint      `gorm:"index;not null" json:"monitorId"`
	CheckedAt time.Time `gorm:"index;not null" json:"checkedAt"`
	Up        bool      `json:"up"`
	// StatusCode là mã HTTP nhận được; 0 nghĩa là không kết nối được.
	StatusCode int    `json:"statusCode"`
	LatencyMs  int64  `json:"latencyMs"`
	Error      string `gorm:"type:text" json:"error,omitempty"`
}

// TableName đặt tên bảng.
func (UptimeCheck) TableName() string { return "uptime_checks" }
