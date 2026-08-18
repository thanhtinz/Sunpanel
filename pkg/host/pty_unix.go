//go:build !windows

package host

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/creack/pty"
)

// defaultShells là các shell được thử theo thứ tự khi người dùng không chỉ định.
var defaultShells = []string{"/bin/bash", "/bin/sh", "/bin/zsh"}

var _ TerminalHost = (*LocalHost)(nil)

// OpenPTY mở một phiên terminal trên máy cục bộ.
func (h *LocalHost) OpenPTY(ctx context.Context, opts PTYOptions) (PTY, error) {
	shell := opts.Shell
	if shell == "" {
		shell = detectShell()
	}

	cmd := exec.CommandContext(ctx, shell, opts.Args...)
	cmd.Dir = opts.Dir

	// TERM là bắt buộc: thiếu nó thì vim, htop, less đều từ chối chạy hoặc vẽ sai.
	// COLORTERM bật màu 24 bit cho các công cụ hỗ trợ.
	locale := detectUTF8Locale()
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
		"LANG="+locale,
		"LC_ALL="+locale,
	)
	cmd.Env = append(cmd.Env, opts.Env...)

	size := &pty.Winsize{Cols: opts.Cols, Rows: opts.Rows}
	if size.Cols == 0 {
		size.Cols, size.Rows = 80, 24
	}

	file, err := pty.StartWithSize(cmd, size)
	if err != nil {
		return nil, fmt.Errorf("mở phiên terminal: %w", err)
	}

	return &localPTY{file: file, cmd: cmd}, nil
}

// detectUTF8Locale chọn một locale UTF-8 thực sự tồn tại trên máy.
//
// Đặt bừa một locale không có sẵn (ví dụ en_US.UTF-8 trên bản Linux tối giản)
// khiến bash rơi về locale C. Khi đó readline đếm BYTE thay vì ký tự, nên mỗi
// chữ tiếng Việt (2–3 byte UTF-8) làm con trỏ lệch đi, và dòng lệnh hiển thị
// thành chuỗi xáo trộn mất dấu — dù byte gửi tới shell hoàn toàn đúng.
//
// C.UTF-8 có mặt trên glibc hiện đại lẫn musl và không cần sinh locale.
func detectUTF8Locale() string {
	for _, name := range []string{"LC_ALL", "LANG"} {
		if value := os.Getenv(name); isUTF8Locale(value) {
			return value
		}
	}
	return "C.UTF-8"
}

// isUTF8Locale cho biết tên locale có phải bản mã UTF-8 hay không.
func isUTF8Locale(value string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(value, "-", ""))
	return strings.Contains(normalized, "utf8")
}

// detectShell chọn shell đầu tiên thực sự tồn tại trên máy.
func detectShell() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		if _, err := os.Stat(shell); err == nil {
			return shell
		}
	}
	for _, candidate := range defaultShells {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	// Không tìm thấy shell nào là tình huống rất bất thường; để /bin/sh báo lỗi
	// rõ ràng còn hơn trả về chuỗi rỗng gây lỗi khó hiểu ở tầng trên.
	return "/bin/sh"
}

// localPTY là phiên terminal chạy trên máy cục bộ.
type localPTY struct {
	file *os.File
	cmd  *exec.Cmd
}

func (p *localPTY) Read(b []byte) (int, error)  { return p.file.Read(b) }
func (p *localPTY) Write(b []byte) (int, error) { return p.file.Write(b) }

// Resize đổi kích thước cửa sổ terminal.
func (p *localPTY) Resize(cols, rows uint16) error {
	if err := pty.Setsize(p.file, &pty.Winsize{Cols: cols, Rows: rows}); err != nil {
		return fmt.Errorf("đổi kích thước terminal: %w", err)
	}
	return nil
}

// Close đóng phiên và kết thúc tiến trình shell.
func (p *localPTY) Close() error {
	// Đóng tệp PTY khiến shell nhận EOF và tự thoát; kill là phương án dự phòng
	// cho trường hợp shell phớt lờ tín hiệu đó.
	err := p.file.Close()
	if p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}
	return err
}

// Wait chờ tiến trình shell kết thúc.
func (p *localPTY) Wait() (int, error) {
	if err := p.cmd.Wait(); err != nil {
		var exitErr *exec.ExitError
		if ok := asExitError(err, &exitErr); ok {
			return exitErr.ExitCode(), nil
		}
		return -1, fmt.Errorf("chờ tiến trình terminal: %w", err)
	}
	return 0, nil
}
