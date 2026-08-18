package host

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
)

// defaultExecTimeout là giới hạn thời gian mặc định cho một lệnh, tránh việc một
// tiến trình treo giữ mãi một goroutine của panel.
const defaultExecTimeout = 60 * time.Second

// maxOutputBytes giới hạn đầu ra thu thập được để một lệnh in ra vô hạn không
// làm cạn bộ nhớ của panel.
const maxOutputBytes = 4 << 20 // 4 MB

// LocalHost thao tác trực tiếp trên máy đang chạy panel.
// Đây là hiện thực duy nhất của Host ở phiên bản đầu tiên.
type LocalHost struct {
	fs      *LocalFS
	allowed map[string]struct{}
}

var _ Host = (*LocalHost)(nil)

// NewLocalHost tạo host cục bộ với fsRoot là thư mục gốc mà mọi thao tác tệp bị
// giới hạn bên trong, và allowedCommands là danh sách trắng các chương trình
// được phép chạy. Danh sách trắng rỗng nghĩa là không giới hạn — chỉ nên dùng
// cho tính năng terminal mà người dùng đã chủ động mở.
func NewLocalHost(fsRoot string, allowedCommands []string) *LocalHost {
	allowed := make(map[string]struct{}, len(allowedCommands))
	for _, c := range allowedCommands {
		allowed[c] = struct{}{}
	}
	return &LocalHost{fs: NewLocalFS(fsRoot), allowed: allowed}
}

// Name trả về định danh của host cục bộ.
func (h *LocalHost) Name() string { return "local" }

// FS trả về lớp truy cập tệp đã giới hạn phạm vi.
func (h *LocalHost) FS() FileSystem { return h.fs }

// Close không cần làm gì với host cục bộ.
func (h *LocalHost) Close() error { return nil }

// Exec chạy một lệnh trên máy cục bộ.
//
// Lệnh được chạy trực tiếp qua exec.CommandContext, không bao giờ qua shell,
// nên tham số chứa ký tự đặc biệt cũng không thể biến thành lệnh mới.
func (h *LocalHost) Exec(ctx context.Context, cmd Command) (Result, error) {
	if err := h.checkAllowed(cmd.Path); err != nil {
		return Result{}, err
	}

	timeout := cmd.Timeout
	if timeout <= 0 {
		timeout = defaultExecTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	c := exec.CommandContext(ctx, cmd.Path, cmd.Args...)
	c.Dir = cmd.Dir
	if len(cmd.Env) > 0 {
		c.Env = append(os.Environ(), cmd.Env...)
	}
	if len(cmd.Stdin) > 0 {
		c.Stdin = bytes.NewReader(cmd.Stdin)
	}

	var stdout, stderr limitedBuffer
	stdout.limit = maxOutputBytes
	stderr.limit = maxOutputBytes
	c.Stdout = &stdout
	c.Stderr = &stderr

	start := time.Now()
	err := c.Run()
	res := Result{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: time.Since(start),
	}

	switch {
	case err == nil:
		res.ExitCode = 0
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return res, fmt.Errorf("lệnh %q quá thời gian %s: %w", cmd.Path, timeout, ctx.Err())
	default:
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// Lệnh chạy được nhưng thoát với mã khác 0 — đây là kết quả hợp lệ,
			// bên gọi tự quyết định coi đó là lỗi hay không.
			res.ExitCode = exitErr.ExitCode()
			return res, nil
		}
		return res, fmt.Errorf("chạy lệnh %q: %w", cmd.Path, err)
	}

	return res, nil
}

// Info thu thập thông tin tĩnh của máy.
func (h *LocalHost) Info(ctx context.Context) (SystemInfo, error) {
	stat, err := host.InfoWithContext(ctx)
	if err != nil {
		return SystemInfo{}, fmt.Errorf("đọc thông tin host: %w", err)
	}

	info := SystemInfo{
		Hostname:       stat.Hostname,
		OS:             stat.OS,
		Platform:       stat.Platform + " " + stat.PlatformVersion,
		Version:        stat.PlatformVersion,
		Kernel:         stat.KernelVersion,
		Arch:           runtime.GOARCH,
		BootTime:       safeInt64(stat.BootTime),
		Virtualization: stat.VirtualizationSystem,
	}

	// Thiếu thông tin CPU hoặc RAM không đáng để cả lời gọi thất bại —
	// dashboard vẫn hiển thị được phần còn lại.
	if cpus, err := cpu.InfoWithContext(ctx); err == nil && len(cpus) > 0 {
		info.CPUModel = strings.TrimSpace(cpus[0].ModelName)
	}
	info.CPUCores = runtime.NumCPU()
	if vm, err := mem.VirtualMemoryWithContext(ctx); err == nil {
		info.TotalMemory = vm.Total
	}

	return info, nil
}

// safeInt64 chuyển mốc thời gian không dấu sang có dấu, kẹp lại nếu giá trị lớn
// bất thường — gopsutil trả 0 hoặc giá trị rác trên vài nền tảng.
func safeInt64(v uint64) int64 {
	if v > math.MaxInt64 {
		return 0
	}
	return int64(v)
}

func (h *LocalHost) checkAllowed(path string) error {
	if len(h.allowed) == 0 {
		return nil
	}
	if _, ok := h.allowed[path]; !ok {
		return fmt.Errorf("%w: %s", ErrCommandNotAllowed, path)
	}
	return nil
}

// limitedBuffer là bộ đệm ghi có trần dung lượng; phần vượt quá bị bỏ đi thay vì
// làm panel cạn bộ nhớ.
type limitedBuffer struct {
	buf      bytes.Buffer
	limit    int
	overflow bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if remaining := b.limit - b.buf.Len(); remaining > 0 {
		if len(p) > remaining {
			b.buf.Write(p[:remaining])
			b.overflow = true
		} else {
			b.buf.Write(p)
		}
	} else if len(p) > 0 {
		b.overflow = true
	}
	// Luôn báo đã ghi hết để tiến trình con không nhận lỗi ống dẫn vỡ.
	return len(p), nil
}

func (b *limitedBuffer) String() string {
	if b.overflow {
		return b.buf.String() + "\n... (đầu ra đã bị cắt bớt)"
	}
	return b.buf.String()
}

// asExitError bọc errors.As cho *exec.ExitError, dùng chung giữa các tệp có
// build tag khác nhau.
func asExitError(err error, target **exec.ExitError) bool {
	return errors.As(err, target)
}
