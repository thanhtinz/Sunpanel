//go:build windows

package host

import "context"

var _ TerminalHost = (*LocalHost)(nil)

// OpenPTY chưa được hỗ trợ trên Windows.
//
// Windows dùng ConPTY với API khác hẳn Unix; phần này sẽ được bổ sung khi panel
// hỗ trợ đầy đủ Windows. Trả về lỗi rõ ràng còn hơn để giao diện treo chờ một
// kết nối không bao giờ mở được.
func (h *LocalHost) OpenPTY(_ context.Context, _ PTYOptions) (PTY, error) {
	return nil, ErrNotSupported
}
