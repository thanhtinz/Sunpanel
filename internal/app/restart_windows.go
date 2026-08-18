package app

import "errors"

// restartSupported là Windows không thay được tiến trình đang chạy bằng lệnh
// exec như Unix, nên panel nhờ người dùng khởi động lại dịch vụ.
const restartSupported = false

// ExecSelf báo rằng Windows phải khởi động lại dịch vụ bằng tay.
func ExecSelf() error {
	return errors.New("app: Windows không tự khởi động lại panel được, hãy khởi động lại dịch vụ")
}
