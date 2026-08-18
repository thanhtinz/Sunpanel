package model

import "time"

// NotifyChannel là một kênh nhận cảnh báo.
type NotifyChannel struct {
	ID   uint   `gorm:"primaryKey" json:"id"`
	Name string `gorm:"size:64;uniqueIndex;not null" json:"name"`
	// Kind là loại kênh: email, telegram hoặc webhook.
	Kind string `gorm:"size:16;not null" json:"kind"`
	// Config là cấu hình kênh dạng JSON, đã mã hóa.
	//
	// Mã hóa vì trong đó có mật khẩu SMTP và token bot Telegram — token bị lộ cho
	// phép người khác nhắn tin dưới danh nghĩa bot của người dùng.
	Config  string `gorm:"type:text" json:"-"`
	Enabled bool   `gorm:"not null" json:"enabled"`

	// LastError ghi lỗi của lần gửi gần nhất, để kênh hỏng không im lặng mãi.
	LastError  string     `gorm:"type:text" json:"lastError,omitempty"`
	LastSentAt *time.Time `json:"lastSentAt,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// TableName đặt tên bảng.
func (NotifyChannel) TableName() string { return "notify_channels" }

// AlertRule là một quy tắc phát cảnh báo theo số liệu giám sát.
type AlertRule struct {
	ID   uint   `gorm:"primaryKey" json:"id"`
	Name string `gorm:"size:64;uniqueIndex;not null" json:"name"`
	// Metric là số liệu theo dõi: cpu, memory, disk hoặc load.
	Metric string `gorm:"size:32;not null" json:"metric"`
	// Threshold là ngưỡng phát cảnh báo.
	Threshold float64 `gorm:"not null" json:"threshold"`
	// DurationMinutes là số phút số liệu phải vượt ngưỡng liên tục.
	//
	// Không có nó thì một lần tăng vọt trong vài giây cũng phát cảnh báo, và
	// người dùng sẽ tắt hết cảnh báo sau vài ngày. Giá trị 0 là hợp lệ và có
	// nghĩa "báo ngay", nên cột KHÔNG được đặt giá trị mặc định: GORM bỏ qua
	// trường mang giá trị zero khi cột có mặc định, và lựa chọn "báo ngay" của
	// người dùng sẽ lặng lẽ biến thành "chờ 5 phút".
	DurationMinutes int `gorm:"not null" json:"durationMinutes"`
	// CooldownMinutes là khoảng nghỉ tối thiểu giữa hai lần báo cùng một quy tắc.
	//
	// Giá trị mặc định do tầng dịch vụ áp, vì lý do như trên.
	CooldownMinutes int `gorm:"not null" json:"cooldownMinutes"`
	// Level là mức nghiêm trọng gán cho cảnh báo.
	Level string `gorm:"size:16;not null" json:"level"`
	// Channels là danh sách mã kênh nhận, ngăn cách bằng dấu phẩy.
	//
	// Để trống nghĩa là gửi tới mọi kênh đang bật — mặc định an toàn hơn là im lặng.
	Channels string `gorm:"size:255" json:"channels"`
	Enabled  bool   `gorm:"not null" json:"enabled"`

	// LastFiredAt là lần phát cảnh báo gần nhất, dùng để áp khoảng nghỉ.
	LastFiredAt *time.Time `json:"lastFiredAt,omitempty"`
	// BreachedSince là thời điểm số liệu bắt đầu vượt ngưỡng liên tục.
	BreachedSince *time.Time `json:"breachedSince,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// TableName đặt tên bảng.
func (AlertRule) TableName() string { return "alert_rules" }

// AlertEvent là một lần cảnh báo đã phát.
type AlertEvent struct {
	ID uint `gorm:"primaryKey" json:"id"`
	// Source cho biết cảnh báo đến từ đâu: rule, backup, cert, system.
	Source string `gorm:"size:32;index" json:"source"`
	// RuleID là quy tắc phát sinh, 0 nếu cảnh báo không đến từ quy tắc.
	RuleID uint   `gorm:"index" json:"ruleId"`
	Title  string `gorm:"size:255;not null" json:"title"`
	Body   string `gorm:"type:text" json:"body"`
	Level  string `gorm:"size:16;not null" json:"level"`
	// Delivered là số kênh gửi thành công, Failed là số kênh thất bại.
	Delivered int    `gorm:"not null;default:0" json:"delivered"`
	Failed    int    `gorm:"not null;default:0" json:"failed"`
	Error     string `gorm:"type:text" json:"error,omitempty"`

	CreatedAt time.Time `gorm:"index" json:"createdAt"`
}

// TableName đặt tên bảng.
func (AlertEvent) TableName() string { return "alert_events" }
