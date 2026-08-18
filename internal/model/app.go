package model

import "time"

// InstalledApp là một ứng dụng đã cài từ chợ ứng dụng.
//
// Nguồn sự thật về việc ứng dụng có đang chạy hay không luôn là Docker; bảng này
// chỉ ghi những thứ Docker không biết: người dùng đã chọn gì lúc cài, và panel
// đặt tệp của ứng dụng ở đâu.
type InstalledApp struct {
	ID uint `gorm:"primaryKey" json:"id"`
	// Name là tên do người dùng đặt, đồng thời là tên thư mục và tên dự án compose.
	Name string `gorm:"size:64;uniqueIndex;not null" json:"name"`
	// AppKey là định danh ứng dụng trong danh mục.
	AppKey string `gorm:"size:64;index;not null" json:"appKey"`
	// Version là phiên bản định nghĩa đã dùng để cài.
	Version string `gorm:"size:32" json:"version"`
	// Dir là thư mục chứa docker-compose.yml và .env của ứng dụng.
	Dir string `gorm:"size:512;not null" json:"dir"`
	// ContainerName là tên container chính, dùng để tra trạng thái.
	ContainerName string `gorm:"size:128" json:"containerName"`
	// Port là cổng chính để mở ứng dụng; 0 nghĩa là ứng dụng không mở cổng nào.
	Port int `gorm:"not null;default:0" json:"port"`
	// Params là các giá trị người dùng đã nhập, dạng JSON và đã mã hóa.
	//
	// Mã hóa vì trong đó có mật khẩu cơ sở dữ liệu và mật khẩu quản trị của ứng
	// dụng; một bản sao lưu cơ sở dữ liệu bị lộ không được kéo theo chúng.
	Params string `gorm:"type:text" json:"-"`
	// Remark là ghi chú tự do.
	Remark string `gorm:"size:512" json:"remark"`

	// LastError ghi lại lỗi của thao tác gần nhất, để người dùng biết vì sao ứng
	// dụng không lên được mà không phải mở nhật ký container.
	LastError string `gorm:"type:text" json:"lastError,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// TableName đặt tên bảng.
func (InstalledApp) TableName() string { return "installed_apps" }
