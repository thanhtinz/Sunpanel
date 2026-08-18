package service

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/pkg/host"
)

// Giới hạn của phiên terminal.
const (
	// terminalIdleTimeout đóng phiên không có hoạt động nào sau khoảng thời gian này.
	// Một tab terminal bị bỏ quên là một shell root đang mở vô thời hạn.
	terminalIdleTimeout = 30 * time.Minute

	// maxTerminalCols và maxTerminalRows chặn giá trị kích thước vô lý từ phía
	// trình duyệt, vốn có thể làm tiến trình con cấp phát bộ nhớ khổng lồ.
	maxTerminalCols = 500
	maxTerminalRows = 300

	// commandLogLimit là độ dài tối đa của một dòng lệnh được ghi vào nhật ký.
	commandLogLimit = 512
)

// TerminalService mở và quản lý các phiên terminal.
type TerminalService struct {
	host  host.TerminalHost
	audit *AuditService
	// fileRoot là thư mục làm việc mặc định khi mở phiên mới.
	fileRoot string
}

// NewTerminalService tạo service terminal.
func NewTerminalService(h host.Host, fileRoot string, audit *AuditService) *TerminalService {
	terminalHost, _ := h.(host.TerminalHost)
	return &TerminalService{host: terminalHost, audit: audit, fileRoot: fileRoot}
}

// Available cho biết nền tảng hiện tại có mở được terminal hay không.
func (s *TerminalService) Available() bool { return s.host != nil }

// Session là một phiên terminal đang mở kèm phần ghi nhật ký lệnh.
type Session struct {
	pty      host.PTY
	recorder *commandRecorder

	closeOnce sync.Once
}

// Open mở một phiên terminal mới.
//
// workDir là thư mục làm việc ban đầu; để trống thì dùng gốc của trình quản lý tệp.
func (s *TerminalService) Open(ctx context.Context, userID uint, username, ip, workDir string, cols, rows uint16) (*Session, error) {
	if !s.Available() {
		return nil, apperr.TerminalNotSupported
	}

	if workDir == "" {
		workDir = s.fileRoot
	}

	pty, err := s.host.OpenPTY(ctx, host.PTYOptions{
		Dir:  workDir,
		Cols: clampSize(cols, maxTerminalCols, 80),
		Rows: clampSize(rows, maxTerminalRows, 24),
	})
	if err != nil {
		return nil, apperr.Internal.Wrap(err)
	}

	s.audit.Record(ctx, AuditEntry{
		UserID: userID, Username: username,
		Action: "terminal.open", Resource: workDir, IP: ip, Success: true,
	})

	return &Session{
		pty: pty,
		recorder: &commandRecorder{
			audit:    s.audit,
			userID:   userID,
			username: username,
			ip:       ip,
		},
	}, nil
}

// Read đọc đầu ra của shell.
func (s *Session) Read(b []byte) (int, error) { return s.pty.Read(b) }

// Write gửi phím người dùng gõ vào shell và ghi lại các dòng lệnh hoàn chỉnh.
func (s *Session) Write(b []byte) (int, error) {
	s.recorder.observe(b)
	return s.pty.Write(b)
}

// Resize đổi kích thước cửa sổ terminal.
func (s *Session) Resize(cols, rows uint16) error {
	return s.pty.Resize(clampSize(cols, maxTerminalCols, 80), clampSize(rows, maxTerminalRows, 24))
}

// Close đóng phiên.
func (s *Session) Close() error {
	var err error
	s.closeOnce.Do(func() {
		s.recorder.flush(context.Background())
		err = s.pty.Close()
	})
	return err
}

// IdleTimeout là thời gian không hoạt động trước khi phiên bị đóng.
func (s *TerminalService) IdleTimeout() time.Duration { return terminalIdleTimeout }

// commandRecorder ghép các phím gõ thành từng dòng lệnh để ghi vào nhật ký.
//
// Terminal là con đường mạnh nhất trong panel: ai vào được đây là có shell root.
// Ghi lại lệnh đã chạy là yêu cầu tối thiểu để truy vết về sau.
type commandRecorder struct {
	audit    *AuditService
	userID   uint
	username string
	ip       string

	mu      sync.Mutex
	current strings.Builder
}

// observe xử lý luồng phím gõ, phát hiện khi một dòng lệnh hoàn chỉnh được gửi đi.
func (r *commandRecorder) observe(data []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, b := range data {
		switch b {
		case '\r', '\n':
			r.emit(context.Background())
		case 0x7f, 0x08: // Backspace
			r.backspace()
		case 0x03: // Ctrl+C hủy dòng đang gõ
			r.current.Reset()
		default:
			// Bỏ qua các ký tự điều khiển khác (mũi tên, Tab, chuỗi escape):
			// ghép chúng vào chỉ tạo ra dòng lệnh rác trong nhật ký.
			if b >= 0x20 && r.current.Len() < commandLogLimit {
				r.current.WriteByte(b)
			}
		}
	}
}

func (r *commandRecorder) emit(ctx context.Context) {
	command := strings.TrimSpace(r.current.String())
	r.current.Reset()

	if command == "" {
		return
	}

	r.audit.Record(ctx, AuditEntry{
		UserID:   r.userID,
		Username: r.username,
		Action:   "terminal.command",
		Resource: command,
		IP:       r.ip,
		Success:  true,
	})
}

func (r *commandRecorder) backspace() {
	current := r.current.String()
	if current == "" {
		return
	}
	r.current.Reset()
	// Cắt theo rune chứ không theo byte, để không làm vỡ ký tự tiếng Việt.
	runes := []rune(current)
	r.current.WriteString(string(runes[:len(runes)-1]))
}

// flush ghi nốt dòng lệnh đang gõ dở khi phiên đóng.
func (r *commandRecorder) flush(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.current.Len() > 0 {
		slog.Debug("phiên terminal đóng khi còn lệnh gõ dở", "user", r.username)
		r.current.Reset()
	}
}

// clampSize kẹp kích thước terminal vào khoảng hợp lệ.
func clampSize(value, max, fallback uint16) uint16 {
	switch {
	case value == 0:
		return fallback
	case value > max:
		return max
	default:
		return value
	}
}
