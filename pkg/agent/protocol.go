// Package agent là giao thức giữa panel và agent chạy trên máy chủ được quản lý.
//
// Giao thức là HTTPS + JSON thay vì gRPC: panel đã có sẵn máy chủ HTTP và bộ mã
// hóa JSON, nên thêm gRPC sẽ kéo theo hàng chục gói phụ thuộc chỉ để làm đúng
// việc mà HTTP đã làm được. Xác thực bằng token bí mật gửi qua tiêu đề, trên
// kênh TLS — token đi qua kênh không mã hóa thì coi như không có token.
package agent

import (
	"time"

	"github.com/thanhtinz/sunpanel/pkg/host"
)

// APIVersion là phiên bản giao thức.
//
// Agent từ chối yêu cầu từ panel có phiên bản khác: hai bên lệch phiên bản mà
// vẫn cố nói chuyện sẽ hỏng theo cách khó lần ra hơn nhiều so với một lỗi rõ ràng.
const APIVersion = "1"

// Các tiêu đề của giao thức.
const (
	// HeaderToken mang token xác thực.
	HeaderToken = "X-SunPanel-Agent-Token"
	// HeaderVersion mang phiên bản giao thức.
	HeaderVersion = "X-SunPanel-Agent-Version"
)

// Các đường dẫn của agent.
const (
	PathPing = "/agent/v1/ping"
	PathInfo = "/agent/v1/info"
	PathExec = "/agent/v1/exec"
	PathFS   = "/agent/v1/fs"
)

// PingResponse là phản hồi kiểm tra sống.
type PingResponse struct {
	// Version là phiên bản giao thức của agent.
	Version string `json:"version"`
	// Hostname giúp người dùng xác nhận mình vừa nối đúng máy.
	Hostname string `json:"hostname"`
	// AgentVersion là phiên bản binary agent.
	AgentVersion string `json:"agentVersion"`
	// Uptime là thời gian agent đã chạy.
	Uptime time.Duration `json:"uptime"`
}

// ExecRequest là yêu cầu chạy một lệnh.
type ExecRequest struct {
	Path    string        `json:"path"`
	Args    []string      `json:"args"`
	Dir     string        `json:"dir"`
	Env     []string      `json:"env"`
	Timeout time.Duration `json:"timeout"`
	// Stdin là dữ liệu đưa vào lệnh.
	Stdin string `json:"stdin"`
}

// ExecResponse là kết quả chạy lệnh.
type ExecResponse struct {
	ExitCode int           `json:"exitCode"`
	Stdout   string        `json:"stdout"`
	Stderr   string        `json:"stderr"`
	Duration time.Duration `json:"duration"`
}

// Result đổi phản hồi sang kiểu dùng chung của lớp host.
func (r ExecResponse) Result() host.Result {
	return host.Result{
		ExitCode: r.ExitCode, Stdout: r.Stdout, Stderr: r.Stderr, Duration: r.Duration,
	}
}

// FSAction là thao tác trên hệ thống tệp.
type FSAction string

// Các thao tác được hỗ trợ.
const (
	FSList   FSAction = "list"
	FSStat   FSAction = "stat"
	FSRead   FSAction = "read"
	FSWrite  FSAction = "write"
	FSMkdir  FSAction = "mkdir"
	FSRemove FSAction = "remove"
	FSMove   FSAction = "move"
	FSChmod  FSAction = "chmod"
)

// FSRequest là yêu cầu thao tác hệ thống tệp.
type FSRequest struct {
	Action FSAction `json:"action"`
	Path   string   `json:"path"`
	// NewPath dùng cho thao tác di chuyển.
	NewPath string `json:"newPath"`
	// Content là nội dung tệp, mã hóa base64 để giữ nguyên dữ liệu nhị phân.
	Content string `json:"content"`
	// Mode là quyền truy cập.
	Mode uint32 `json:"mode"`
	// Recursive cho phép xóa cả cây thư mục.
	Recursive bool `json:"recursive"`
}

// FSResponse là kết quả thao tác hệ thống tệp.
type FSResponse struct {
	// Entries là danh sách mục, dùng với thao tác liệt kê.
	Entries []host.FileInfo `json:"entries,omitempty"`
	// Info là thông tin một mục, dùng với thao tác xem thông tin.
	Info *host.FileInfo `json:"info,omitempty"`
	// Content là nội dung tệp đã mã hóa base64.
	Content string `json:"content,omitempty"`
}

// ErrorResponse là phản hồi lỗi của agent.
type ErrorResponse struct {
	Error string `json:"error"`
}
