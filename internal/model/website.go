package model

import "time"

// Website là một website do panel quản lý.
//
// Panel lưu bản mô tả website trong cơ sở dữ liệu rồi sinh tệp cấu hình máy chủ
// web từ đó, chứ không đọc ngược tệp cấu hình. Nhờ vậy một tệp cấu hình bị sửa
// tay hoặc bị xóa luôn khôi phục lại được từ bản mô tả gốc.
type Website struct {
	ID uint `gorm:"primaryKey" json:"id"`
	// Name là định danh, đồng thời là tên tệp cấu hình và tên thư mục nhật ký.
	Name string `gorm:"size:64;uniqueIndex;not null" json:"name"`
	// Domains là danh sách tên miền, ngăn cách bằng dấu phẩy.
	//
	// Lưu dạng chuỗi thay vì bảng riêng: danh sách luôn ngắn, luôn được đọc và
	// ghi trọn vẹn cùng website, nên một bảng riêng chỉ thêm việc mà không thêm gì.
	Domains string `gorm:"size:1024;not null" json:"domains"`
	// Type là loại website: static, proxy hoặc php.
	Type string `gorm:"size:16;not null" json:"type"`
	// Root là thư mục gốc của website tĩnh và website PHP.
	Root string `gorm:"size:512" json:"root"`
	// ProxyTarget là địa chỉ đích của website dạng chuyển tiếp.
	ProxyTarget string `gorm:"size:512" json:"proxyTarget"`
	// PHPSocket là socket hoặc địa chỉ của PHP-FPM.
	PHPSocket string `gorm:"size:512" json:"phpSocket"`
	// Port và SSLPort là cổng HTTP và HTTPS.
	Port    int `gorm:"not null;default:80" json:"port"`
	SSLPort int `gorm:"not null;default:443" json:"sslPort"`

	// SSLEnabled bật HTTPS, ForceHTTPS chuyển hướng mọi yêu cầu HTTP sang HTTPS.
	SSLEnabled bool `gorm:"not null;default:false" json:"sslEnabled"`
	ForceHTTPS bool `gorm:"not null;default:false" json:"forceHttps"`
	// CertName là tên chứng chỉ trong kho; để trống nghĩa là chưa gắn chứng chỉ.
	CertName string `gorm:"size:64" json:"certName"`

	// ExtraConfig là cấu hình bổ sung do quản trị viên tự thêm.
	ExtraConfig string `gorm:"type:text" json:"extraConfig"`
	// Enabled quyết định tệp cấu hình có được ghi ra đĩa hay không.
	//
	// Không đặt giá trị mặc định cho cột: GORM bỏ qua trường mang giá trị zero
	// khi cột có mặc định, nên lựa chọn "tắt" của người dùng sẽ lặng lẽ bị lật
	// thành "bật" ngay lúc ghi.
	Enabled bool `gorm:"not null" json:"enabled"`
	// Remark là ghi chú tự do.
	Remark string `gorm:"size:512" json:"remark"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// TableName đặt tên bảng.
func (Website) TableName() string { return "websites" }

// Certificate là bản ghi một chứng chỉ TLS trong kho.
//
// Nội dung chứng chỉ nằm trên đĩa; bảng này chỉ lưu phần siêu dữ liệu cần cho
// việc tự động gia hạn — thứ không đọc được từ chính tệp chứng chỉ.
type Certificate struct {
	ID uint `gorm:"primaryKey" json:"id"`
	// Name là định danh, đồng thời là tên thư mục trong kho chứng chỉ.
	Name string `gorm:"size:64;uniqueIndex;not null" json:"name"`
	// Domains là danh sách tên miền, ngăn cách bằng dấu phẩy.
	Domains string `gorm:"size:1024;not null" json:"domains"`
	// Source cho biết chứng chỉ từ đâu: self, acme hoặc upload.
	Source string `gorm:"size:16;not null" json:"source"`
	// Email là địa chỉ đăng ký với máy chủ ACME.
	Email string `gorm:"size:255" json:"email"`
	// Staging cho biết chứng chỉ xin từ máy chủ thử nghiệm của Let's Encrypt.
	Staging bool `gorm:"not null;default:false" json:"staging"`
	// AutoRenew bật việc tự động gia hạn.
	//
	// Không đặt mặc định cho cột, vì lý do như ở Website.Enabled: chứng chỉ tự ký
	// không gia hạn tự động được, và một cờ bị bật ngoài ý muốn sẽ tạo ra vòng
	// thử lại thất bại mãi mãi.
	AutoRenew bool `gorm:"not null" json:"autoRenew"`

	// IssuedAt và ExpiresAt sao lại thời hạn trong chứng chỉ để truy vấn nhanh.
	IssuedAt  time.Time `json:"issuedAt"`
	ExpiresAt time.Time `gorm:"index" json:"expiresAt"`
	// LastRenewAt và LastError ghi lại kết quả lần gia hạn gần nhất.
	//
	// Không có hai trường này thì một chuỗi gia hạn thất bại sẽ im lặng cho tới
	// ngày chứng chỉ hết hạn và website ngừng phục vụ HTTPS.
	LastRenewAt *time.Time `json:"lastRenewAt,omitempty"`
	LastError   string     `gorm:"type:text" json:"lastError,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// TableName đặt tên bảng.
func (Certificate) TableName() string { return "certificates" }
