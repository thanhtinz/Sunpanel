package model

import "time"

// Node là một máy chủ khác do panel quản lý qua agent.
type Node struct {
	ID   uint   `gorm:"primaryKey" json:"id"`
	Name string `gorm:"size:64;uniqueIndex;not null" json:"name"`
	// Address là địa chỉ agent, ví dụ https://10.0.0.5:9528
	Address string `gorm:"size:255;not null" json:"address"`
	// Token đã mã hóa; token này mở toàn quyền trên máy chủ đó nên không bao giờ
	// được trả ra API.
	Token string `gorm:"type:text;not null" json:"-"`
	// SkipVerify bỏ qua kiểm chứng chỉ TLS của agent.
	//
	// Cần cho agent dùng chứng chỉ tự ký — trường hợp phổ biến nhất trong mạng
	// nội bộ. Kênh vẫn mã hóa, thứ mất đi là khả năng chống kẻ đứng giữa.
	SkipVerify bool   `gorm:"not null" json:"skipVerify"`
	Remark     string `gorm:"size:512" json:"remark"`

	// Hostname, OS và Arch lưu lại thông tin đọc được ở lần kết nối gần nhất, để
	// danh sách node vẫn nói được điều gì đó khi node đang tắt.
	Hostname string `gorm:"size:255" json:"hostname"`
	OS       string `gorm:"size:64" json:"os"`
	Arch     string `gorm:"size:32" json:"arch"`
	// AgentVersion là phiên bản agent; lệch phiên bản với panel là nguyên nhân
	// hỏng khó lần ra nhất nên phải hiển thị.
	AgentVersion string `gorm:"size:64" json:"agentVersion"`

	LastSeenAt *time.Time `json:"lastSeenAt,omitempty"`
	LastError  string     `gorm:"type:text" json:"lastError,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// TableName đặt tên bảng.
func (Node) TableName() string { return "nodes" }
