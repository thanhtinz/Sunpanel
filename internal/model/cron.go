package model

import "time"

// CronJob là một tác vụ định kỳ do quản trị viên tạo.
//
// Panel dùng bộ lập lịch của riêng mình thay vì ghi vào crontab hệ thống. Nhờ vậy
// tác vụ chạy được trên cả Windows lẫn macOS, và mỗi lần chạy đều có bản ghi kèm
// đầu ra — thứ mà crontab hệ thống không cho biết.
type CronJob struct {
	ID   uint   `gorm:"primaryKey" json:"id"`
	Name string `gorm:"size:128;not null" json:"name"`
	// Schedule là biểu thức cron 5 trường, ví dụ "0 3 * * *".
	Schedule string `gorm:"size:128;not null" json:"schedule"`
	// Command là lệnh shell được chạy. Đây là lệnh tùy ý do quản trị viên soạn,
	// cùng mức tin cậy với terminal.
	Command string `gorm:"type:text;not null" json:"command"`
	// WorkDir là thư mục làm việc; để trống thì dùng gốc quản lý tệp.
	WorkDir string `gorm:"size:512" json:"workDir"`
	// TimeoutSeconds giới hạn thời gian chạy; 0 nghĩa là dùng mặc định.
	TimeoutSeconds int  `gorm:"not null;default:0" json:"timeoutSeconds"`
	Enabled        bool `gorm:"not null;default:true" json:"enabled"`

	// LastRunAt, LastSuccess và LastExitCode tóm tắt lần chạy gần nhất để danh
	// sách hiển thị được ngay mà không phải truy vấn bảng lịch sử.
	LastRunAt    *time.Time `json:"lastRunAt,omitempty"`
	LastSuccess  *bool      `json:"lastSuccess,omitempty"`
	LastExitCode int        `gorm:"not null;default:0" json:"lastExitCode"`
	// NextRunAt là thời điểm chạy kế tiếp do bộ lập lịch tính.
	NextRunAt *time.Time `json:"nextRunAt,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// TableName đặt tên bảng.
func (CronJob) TableName() string { return "cron_jobs" }

// CronRun là bản ghi một lần chạy của tác vụ định kỳ.
type CronRun struct {
	ID    uint `gorm:"primaryKey" json:"id"`
	JobID uint `gorm:"index;not null" json:"jobId"`
	// JobName lưu lại tên tại thời điểm chạy, để lịch sử vẫn đọc được sau khi
	// tác vụ bị đổi tên hoặc xóa.
	JobName    string     `gorm:"size:128" json:"jobName"`
	StartedAt  time.Time  `gorm:"index" json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
	// DurationMS là thời gian chạy tính bằng mili giây.
	DurationMS int64 `gorm:"not null;default:0" json:"durationMs"`
	ExitCode   int   `gorm:"not null;default:0" json:"exitCode"`
	Success    bool  `gorm:"not null" json:"success"`
	// Output là đầu ra đã cắt bớt của lệnh.
	Output string `gorm:"type:text" json:"output"`
	// Error là mô tả lỗi khi lệnh không chạy được (không phải khi lệnh thoát khác 0).
	Error string `gorm:"type:text" json:"error,omitempty"`
	// TriggeredBy cho biết lần chạy do lịch hay do người dùng bấm chạy ngay.
	TriggeredBy string `gorm:"size:32" json:"triggeredBy"`
}

// TableName đặt tên bảng.
func (CronRun) TableName() string { return "cron_runs" }
