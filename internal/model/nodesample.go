package model

import "time"

// NodeSample là một lần đo tài nguyên của máy chủ từ xa.
//
// Lưu lại thay vì chỉ đọc lúc mở trang: câu hỏi thường gặp nhất về một VPS là
// "lúc nãy nó có bị quá tải không", và câu đó chỉ trả lời được bằng lịch sử.
type NodeSample struct {
	ID     uint      `gorm:"primaryKey" json:"id"`
	NodeID uint      `gorm:"index;not null" json:"nodeId"`
	At     time.Time `gorm:"index;not null" json:"at"`

	CPUPercent    float64 `json:"cpu"`
	MemoryPercent float64 `json:"memory"`
	DiskPercent   float64 `json:"disk"`
	Load1         float64 `json:"load1"`
}

// TableName đặt tên bảng.
func (NodeSample) TableName() string { return "node_samples" }
