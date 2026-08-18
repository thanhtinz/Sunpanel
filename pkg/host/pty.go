package host

import (
	"context"
	"io"
)

// PTYOptions là tham số mở một phiên terminal.
type PTYOptions struct {
	// Shell là chương trình cần chạy; để trống thì dùng shell mặc định của hệ thống.
	Shell string
	// Args là tham số truyền cho shell.
	Args []string
	// Dir là thư mục làm việc ban đầu.
	Dir string
	// Env là biến môi trường bổ sung dạng "KEY=VALUE".
	Env []string
	// Cols và Rows là kích thước ban đầu của terminal.
	Cols uint16
	Rows uint16
}

// PTY là một phiên terminal đang mở.
//
// Đọc từ PTY lấy được đầu ra của shell; ghi vào PTY là gõ phím vào shell.
type PTY interface {
	io.ReadWriteCloser
	// Resize báo cho tiến trình con biết cửa sổ terminal đã đổi kích thước, để
	// các chương trình toàn màn hình như vim hay htop vẽ lại cho đúng.
	Resize(cols, rows uint16) error
	// Wait chờ tiến trình kết thúc và trả về mã thoát.
	Wait() (int, error)
}

// TerminalHost là host có khả năng mở phiên terminal.
//
// Tách khỏi Host vì không phải hiện thực nào cũng mở được PTY: Windows dùng
// ConPTY với API khác hẳn, và agent từ xa ở phiên bản sau sẽ chuyển tiếp luồng
// qua gRPC thay vì mở PTY cục bộ.
type TerminalHost interface {
	// OpenPTY mở một phiên terminal mới.
	OpenPTY(ctx context.Context, opts PTYOptions) (PTY, error)
}
