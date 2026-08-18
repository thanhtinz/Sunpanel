package model

import "time"

// BackupPlan là một kế hoạch sao lưu.
//
// Kế hoạch mô tả sao lưu CÁI GÌ, ĐI ĐÂU và GIỮ BAO LÂU. Mỗi lần chạy sinh một
// bản ghi BackupRun riêng, để lịch sử vẫn còn khi kế hoạch bị sửa hoặc xóa.
type BackupPlan struct {
	ID   uint   `gorm:"primaryKey" json:"id"`
	Name string `gorm:"size:64;uniqueIndex;not null" json:"name"`
	// Source là loại nguồn: database hoặc directory.
	Source string `gorm:"size:16;not null" json:"source"`
	// ServerID và Database chỉ dùng khi sao lưu cơ sở dữ liệu.
	ServerID uint   `gorm:"index" json:"serverId"`
	Database string `gorm:"size:128" json:"database"`
	// Path và Exclude chỉ dùng khi sao lưu thư mục.
	Path string `gorm:"size:512" json:"path"`
	// Exclude là các mẫu tên bị loại trừ, ngăn cách bằng dấu phẩy.
	Exclude string `gorm:"size:512" json:"exclude"`

	// Destination là loại nơi lưu: local, s3 hoặc webdav.
	Destination string `gorm:"size:16;not null" json:"destination"`
	// DestConfig là cấu hình nơi lưu dạng JSON, đã mã hóa.
	//
	// Mã hóa vì trong đó có khóa truy cập S3 và mật khẩu WebDAV — những thứ mở ra
	// toàn bộ kho sao lưu của người dùng.
	DestConfig string `gorm:"type:text" json:"-"`

	// Schedule là biểu thức cron; để trống nghĩa là chỉ chạy khi bấm tay.
	Schedule string `gorm:"size:128" json:"schedule"`
	// KeepCount và KeepDays là chính sách giữ bản sao; 0 nghĩa là không giới hạn.
	KeepCount int  `gorm:"not null;default:0" json:"keepCount"`
	KeepDays  int  `gorm:"not null;default:0" json:"keepDays"`
	Enabled   bool `gorm:"not null" json:"enabled"`

	// LastRunAt, LastSuccess và LastError tóm tắt lần chạy gần nhất.
	LastRunAt   *time.Time `json:"lastRunAt,omitempty"`
	LastSuccess *bool      `json:"lastSuccess,omitempty"`
	LastError   string     `gorm:"type:text" json:"lastError,omitempty"`
	// NextRunAt là thời điểm chạy kế tiếp do bộ lập lịch tính.
	NextRunAt *time.Time `json:"nextRunAt,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// TableName đặt tên bảng.
func (BackupPlan) TableName() string { return "backup_plans" }

// BackupRun là bản ghi một lần chạy sao lưu.
type BackupRun struct {
	ID     uint `gorm:"primaryKey" json:"id"`
	PlanID uint `gorm:"index;not null" json:"planId"`
	// PlanName lưu lại tên tại thời điểm chạy, để lịch sử vẫn đọc được sau khi
	// kế hoạch bị đổi tên hoặc xóa.
	PlanName string `gorm:"size:64" json:"planName"`
	// Object là tên tệp sao lưu ở nơi lưu trữ.
	Object string `gorm:"size:255" json:"object"`
	// SizeBytes là dung lượng tệp sao lưu.
	SizeBytes int64 `gorm:"not null;default:0" json:"sizeBytes"`

	StartedAt  time.Time  `gorm:"index" json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
	DurationMS int64      `gorm:"not null;default:0" json:"durationMs"`
	Success    bool       `gorm:"not null" json:"success"`
	Error      string     `gorm:"type:text" json:"error,omitempty"`
	// TriggeredBy cho biết lần chạy do lịch hay do người dùng bấm.
	TriggeredBy string `gorm:"size:32" json:"triggeredBy"`
	// Removed là các bản sao lưu cũ bị chính sách giữ dọn đi trong lần chạy này.
	Removed string `gorm:"type:text" json:"removed,omitempty"`
}

// TableName đặt tên bảng.
func (BackupRun) TableName() string { return "backup_runs" }
