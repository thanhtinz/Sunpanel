//go:build !windows

package app

import (
	"os"
	"syscall"
)

// restartSupported là panel tự chạy lại được trên hệ Unix.
const restartSupported = true

// ExecSelf thay tiến trình hiện tại bằng một bản mới của chính binary.
//
// Thay vì sinh tiến trình con: systemd (hay bất cứ bộ giám sát nào) vẫn theo dõi
// đúng một số hiệu tiến trình, nên panel lên lại mà dịch vụ không bị coi là đã
// chết. Mọi thứ cần đóng đã được đóng trước khi hàm này chạy — hàm này không bao
// giờ trả về nếu thành công.
func ExecSelf() error {
	binary, err := os.Executable()
	if err != nil {
		return err
	}
	return syscall.Exec(binary, os.Args, os.Environ())
}
