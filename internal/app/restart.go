package app

import (
	"errors"
	"log/slog"
	"sync"
)

// ErrRestart là tín hiệu panel đã dừng theo yêu cầu và cần được chạy lại.
//
// Không phải lỗi: nó là cách duy nhất để lớp gọi bên ngoài phân biệt "đã tắt
// hẳn" với "đã tắt để lên lại", vì bản thân tiến trình không tự quyết định được
// việc đó — nó còn phải nhường chỗ cho tiến trình mới.
var ErrRestart = errors.New("app: panel cần khởi động lại")

// restartSignal là cầu nối giữa yêu cầu từ giao diện và vòng chạy của máy chủ.
type restartSignal struct {
	once sync.Once
	ch   chan struct{}
}

func newRestartSignal() *restartSignal {
	return &restartSignal{ch: make(chan struct{})}
}

// Restart yêu cầu panel dừng và chạy lại.
//
// Đóng kênh đúng một lần: hai người quản trị cùng bấm khởi động lại không được
// làm tiến trình hoảng loạn vì đóng kênh hai lần.
func (r *restartSignal) Restart() error {
	r.once.Do(func() {
		slog.Info("nhận yêu cầu khởi động lại panel")
		close(r.ch)
	})
	return nil
}

// RestartSupported cho biết nền tảng hiện tại chạy lại chính mình được.
func (r *restartSignal) RestartSupported() bool { return restartSupported }
