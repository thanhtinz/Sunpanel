package model

import "time"

// MonitorSample là một mẫu số liệu giám sát được lấy theo chu kỳ, dùng để vẽ
// biểu đồ lịch sử trên dashboard.
type MonitorSample struct {
	ID uint `gorm:"primaryKey" json:"-"`
	// CPUPercent là mức sử dụng CPU tổng, tính theo phần trăm.
	CPUPercent float64 `json:"cpu"`
	// MemoryPercent là mức sử dụng RAM, tính theo phần trăm.
	MemoryPercent float64 `json:"memory"`
	// SwapPercent là mức sử dụng swap, tính theo phần trăm.
	SwapPercent float64 `json:"swap"`
	// DiskPercent là mức sử dụng phân vùng gốc, tính theo phần trăm.
	DiskPercent float64 `json:"disk"`
	// LoadAvg1 là tải trung bình 1 phút (luôn bằng 0 trên Windows).
	LoadAvg1 float64 `json:"load1"`
	// NetSentRate và NetRecvRate là tốc độ mạng tính bằng byte mỗi giây.
	NetSentRate float64 `json:"netSent"`
	NetRecvRate float64 `json:"netRecv"`
	// DiskReadRate và DiskWriteRate là tốc độ đĩa tính bằng byte mỗi giây.
	DiskReadRate  float64   `json:"diskRead"`
	DiskWriteRate float64   `json:"diskWrite"`
	CreatedAt     time.Time `gorm:"index" json:"time"`
}

// TableName đặt tên bảng.
func (MonitorSample) TableName() string { return "monitor_samples" }

// Setting là một cặp khóa-giá trị lưu cấu hình chỉnh được từ giao diện.
type Setting struct {
	Key       string    `gorm:"primaryKey;size:64" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// TableName đặt tên bảng.
func (Setting) TableName() string { return "settings" }
