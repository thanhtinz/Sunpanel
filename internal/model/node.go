package model

import "time"

// Các kiểu kết nối tới một máy chủ từ xa.
const (
	// NodeAgent là máy chủ có cài agent của SunPanel.
	NodeAgent = "agent"
	// NodeSSH là máy chủ chỉ kết nối qua SSH.
	//
	// Đây là cách dùng được ngay với một VPS vừa mua về: máy nào cũng có sẵn
	// SSH từ lúc nhà cung cấp giao máy, không phải cài thêm gì.
	NodeSSH = "ssh"
)

// Các cách xác thực khi kết nối SSH.
const (
	AuthPassword = "password"
	AuthKey      = "key"
)

// Node là một máy chủ khác do panel điều khiển từ xa.
type Node struct {
	ID   uint   `gorm:"primaryKey" json:"id"`
	Name string `gorm:"size:64;uniqueIndex;not null" json:"name"`
	// Kind là kiểu kết nối: agent hoặc ssh.
	//
	// Không đặt mặc định cho cột: GORM bỏ qua trường mang giá trị zero khi cột
	// có mặc định. Giá trị do tầng dịch vụ áp mới là nguồn duy nhất.
	Kind string `gorm:"size:16" json:"kind"`
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

	// Các trường dưới đây chỉ dùng cho kết nối SSH.
	Host string `gorm:"size:255" json:"host,omitempty"`
	Port int    `json:"port,omitempty"`
	User string `gorm:"size:64" json:"user,omitempty"`
	// AuthType là password hoặc key.
	AuthType string `gorm:"size:16" json:"authType,omitempty"`
	// Secret là mật khẩu hoặc khóa riêng, đã mã hóa.
	//
	// Thứ nằm đây mở toàn quyền trên máy chủ đó nên nó không bao giờ được trả
	// ra API, kể cả cho quản trị viên đã đăng nhập.
	Secret string `gorm:"type:text" json:"-"`
	// Passphrase mở khóa riêng, cũng đã mã hóa.
	Passphrase string `gorm:"type:text" json:"-"`
	// Fingerprint là dấu vân tay khóa máy chủ ghi nhận ở lần kết nối đầu tiên.
	//
	// Từ lần sau khóa phải khớp: khóa đổi nghĩa là hoặc máy đã bị cài lại, hoặc
	// có người đứng giữa đang giả làm nó.
	Fingerprint string `gorm:"size:128" json:"fingerprint,omitempty"`

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
