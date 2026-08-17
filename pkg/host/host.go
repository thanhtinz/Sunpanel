// Package host là lớp trừu tượng cho mọi thao tác chạm vào hệ điều hành.
//
// Không package nào ở internal/service được phép gọi thẳng os/exec hay os.
// Toàn bộ phải đi qua interface Host. Nhờ vậy phiên bản sau có thể thêm
// RemoteHost (nói chuyện với agent qua gRPC) để quản lý nhiều server mà
// không phải sửa một dòng business logic nào.
package host

import (
	"context"
	"errors"
	"time"
)

// Các lỗi dùng chung của lớp host.
var (
	// ErrCommandNotAllowed trả về khi lệnh không nằm trong allowlist.
	ErrCommandNotAllowed = errors.New("host: lệnh không được phép thực thi")
	// ErrPathEscapesRoot trả về khi đường dẫn thoát ra ngoài thư mục gốc cho phép.
	ErrPathEscapesRoot = errors.New("host: đường dẫn nằm ngoài phạm vi cho phép")
	// ErrNotSupported trả về khi thao tác không khả dụng trên nền tảng hiện tại.
	ErrNotSupported = errors.New("host: thao tác không được hỗ trợ trên nền tảng này")
)

// Command mô tả một lệnh cần chạy.
//
// Cố ý tách Path và Args thành hai trường riêng: không có chỗ nào nhận vào
// một chuỗi shell, nên không tồn tại bề mặt tấn công shell injection.
type Command struct {
	// Path là tên hoặc đường dẫn của chương trình (không phải chuỗi shell).
	Path string
	// Args là danh sách tham số, mỗi phần tử được truyền nguyên vẹn.
	Args []string
	// Dir là thư mục làm việc, để trống nghĩa là thư mục hiện tại.
	Dir string
	// Env là các biến môi trường bổ sung dạng "KEY=VALUE".
	Env []string
	// Stdin là dữ liệu đưa vào đầu vào chuẩn của tiến trình.
	Stdin []byte
	// Timeout giới hạn thời gian chạy, 0 nghĩa là dùng mặc định của Executor.
	Timeout time.Duration
}

// Result là kết quả chạy một Command.
type Result struct {
	// ExitCode là mã thoát của tiến trình.
	ExitCode int
	// Stdout và Stderr là đầu ra đã thu thập.
	Stdout string
	Stderr string
	// Duration là thời gian tiến trình đã chạy.
	Duration time.Duration
}

// OK cho biết lệnh kết thúc thành công.
func (r Result) OK() bool { return r.ExitCode == 0 }

// SystemInfo là ảnh chụp thông tin tĩnh của máy.
type SystemInfo struct {
	Hostname       string `json:"hostname"`
	OS             string `json:"os"`
	Platform       string `json:"platform"`
	Version        string `json:"version"`
	Kernel         string `json:"kernel"`
	Arch           string `json:"arch"`
	CPUModel       string `json:"cpuModel"`
	CPUCores       int    `json:"cpuCores"`
	TotalMemory    uint64 `json:"totalMemory"`
	BootTime       int64  `json:"bootTime"`
	Virtualization string `json:"virtualization"`
}

// Host là toàn bộ bề mặt tiếp xúc với một máy chủ được quản lý.
type Host interface {
	// Name là định danh của host, dùng để hiển thị và ghi log.
	Name() string
	// Exec chạy một lệnh trên host.
	Exec(ctx context.Context, cmd Command) (Result, error)
	// FS trả về lớp truy cập hệ thống tệp có kiểm soát phạm vi.
	FS() FileSystem
	// Info trả về thông tin hệ thống.
	Info(ctx context.Context) (SystemInfo, error)
	// Close giải phóng tài nguyên (kết nối tới agent ở bản đa node).
	Close() error
}
