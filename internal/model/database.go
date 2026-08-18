package model

import "time"

// DatabaseServer là một máy chủ cơ sở dữ liệu mà panel quản lý.
//
// Máy chủ có thể là container do panel cài từ chợ ứng dụng, hoặc một máy chủ có
// sẵn ở nơi khác. Panel không phân biệt: nó chỉ cần thông tin kết nối.
type DatabaseServer struct {
	ID   uint   `gorm:"primaryKey" json:"id"`
	Name string `gorm:"size:64;uniqueIndex;not null" json:"name"`
	// Kind là loại máy chủ: mysql (gồm cả MariaDB) hoặc postgres.
	Kind string `gorm:"size:16;not null" json:"kind"`
	Host string `gorm:"size:255;not null" json:"host"`
	Port int    `gorm:"not null" json:"port"`
	// User là tài khoản quản trị panel dùng để kết nối.
	User string `gorm:"size:128;not null" json:"user"`
	// Password đã được mã hóa; không bao giờ trả ra API.
	//
	// Đây là tài khoản toàn quyền trên máy chủ cơ sở dữ liệu, nên một bản sao lưu
	// cơ sở dữ liệu panel bị lộ không được kéo theo nó.
	Password string `gorm:"type:text;not null" json:"-"`
	// Remark là ghi chú tự do.
	Remark string `gorm:"size:512" json:"remark"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// TableName đặt tên bảng.
func (DatabaseServer) TableName() string { return "database_servers" }
