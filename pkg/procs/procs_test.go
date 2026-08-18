package procs

import (
	"context"
	"errors"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestListFindsCurrentProcess(t *testing.T) {
	sampler := NewSampler()
	items, err := sampler.List(context.Background())
	if err != nil {
		t.Fatalf("liệt kê tiến trình: %v", err)
	}

	self := int32(os.Getpid())
	for _, item := range items {
		if item.PID != self {
			continue
		}
		if !item.Protected {
			t.Error("tiến trình của chính panel phải được đánh dấu bảo vệ")
		}
		if item.Command == "" {
			t.Error("dòng lệnh trống")
		}
		return
	}
	t.Fatalf("không thấy tiến trình của chính bài kiểm thử (%d) trong %d tiến trình", self, len(items))
}

// Lần lấy mẫu đầu không có mốc so sánh nên phần trăm CPU phải bằng 0, chứ không
// phải trung bình từ lúc khởi chạy — đó chính là con số gây hiểu nhầm mà Sampler
// sinh ra để tránh.
func TestFirstSampleReportsNoCPU(t *testing.T) {
	items, err := NewSampler().List(context.Background())
	if err != nil {
		t.Fatalf("liệt kê tiến trình: %v", err)
	}
	for _, item := range items {
		if item.CPUPercent != 0 {
			t.Fatalf("tiến trình %d báo %.2f%% CPU ở lần lấy mẫu đầu", item.PID, item.CPUPercent)
		}
	}
}

func TestKillRefusesProtectedProcesses(t *testing.T) {
	sampler := NewSampler()
	for _, pid := range []int32{0, 1, int32(os.Getpid())} {
		if err := sampler.Kill(context.Background(), pid, false); !errors.Is(err, ErrProtected) {
			t.Errorf("kết thúc tiến trình %d: mong ErrProtected, nhận %v", pid, err)
		}
	}
}

func TestKillEndsProcess(t *testing.T) {
	cmd := exec.Command("sleep", "60")
	if err := cmd.Start(); err != nil {
		t.Skipf("không chạy được tiến trình thử: %v", err)
	}
	defer func() { _ = cmd.Process.Kill() }()

	if err := NewSampler().Kill(context.Background(), int32(cmd.Process.Pid), true); err != nil {
		t.Fatalf("kết thúc tiến trình: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("tiến trình vẫn sống sau khi nhận tín hiệu kết thúc")
	}
}

func TestListenersIncludeOpenPort(t *testing.T) {
	listener, err := listenLocal()
	if err != nil {
		t.Skipf("không mở được cổng thử: %v", err)
	}
	defer listener.Close()

	items, err := NewSampler().Listeners(context.Background())
	if err != nil {
		t.Fatalf("liệt kê cổng: %v", err)
	}

	port := listenPort(listener)
	for _, item := range items {
		if item.Port == port {
			return
		}
	}
	// Trong container không có quyền đọc /proc của tiến trình khác, danh sách có
	// thể rỗng; chỉ báo lỗi khi rõ ràng đọc được mà vẫn thiếu cổng vừa mở.
	if len(items) > 0 {
		t.Fatalf("không thấy cổng %d trong %d cổng đang mở", port, len(items))
	}
}

func listenLocal() (net.Listener, error) { return net.Listen("tcp", "127.0.0.1:0") }

func listenPort(l net.Listener) uint32 { return uint32(l.Addr().(*net.TCPAddr).Port) }
