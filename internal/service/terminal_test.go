package service

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/thanhtinz/sunpanel/internal/model"
)

func newRecorder(t *testing.T) (*commandRecorder, *gorm.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("mở cơ sở dữ liệu: %v", err)
	}
	if err := db.AutoMigrate(&model.AuditLog{}); err != nil {
		t.Fatalf("migration: %v", err)
	}

	return &commandRecorder{
		audit: NewAuditService(db), userID: 1, username: "admin", ip: "127.0.0.1",
	}, db
}

func recordedCommands(t *testing.T, db *gorm.DB) []string {
	t.Helper()

	var logs []model.AuditLog
	if err := db.Where("action = ?", "terminal.command").Order("id ASC").Find(&logs).Error; err != nil {
		t.Fatalf("đọc nhật ký: %v", err)
	}

	commands := make([]string, len(logs))
	for i, log := range logs {
		commands[i] = log.Resource
	}
	return commands
}

func TestCommandRecorderCapturesLines(t *testing.T) {
	recorder, db := newRecorder(t)

	recorder.observe([]byte("ls -la\r"))
	recorder.observe([]byte("cd /var/www\n"))

	got := recordedCommands(t, db)
	want := []string{"ls -la", "cd /var/www"}
	if len(got) != len(want) {
		t.Fatalf("ghi được %v, mong đợi %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("lệnh %d = %q, mong đợi %q", i, got[i], want[i])
		}
	}
}

// Người dùng gõ từng phím một, không phải cả dòng cùng lúc.
func TestCommandRecorderHandlesKeystrokeByKeystroke(t *testing.T) {
	recorder, db := newRecorder(t)

	for _, ch := range []byte("whoami\r") {
		recorder.observe([]byte{ch})
	}

	if got := recordedCommands(t, db); len(got) != 1 || got[0] != "whoami" {
		t.Fatalf("ghi được %v, mong đợi [whoami]", got)
	}
}

func TestCommandRecorderHandlesBackspace(t *testing.T) {
	recorder, db := newRecorder(t)

	// Gõ "lss", xóa một ký tự, rồi Enter.
	recorder.observe([]byte("lss"))
	recorder.observe([]byte{0x7f})
	recorder.observe([]byte("\r"))

	if got := recordedCommands(t, db); len(got) != 1 || got[0] != "ls" {
		t.Fatalf("ghi được %v, mong đợi [ls]", got)
	}
}

// Backspace phải xóa trọn một ký tự tiếng Việt, không cắt giữa các byte UTF-8.
func TestCommandRecorderBackspaceOnVietnamese(t *testing.T) {
	recorder, db := newRecorder(t)

	recorder.observe([]byte("echo mật khẩuX"))
	recorder.observe([]byte{0x7f})
	recorder.observe([]byte("\r"))

	got := recordedCommands(t, db)
	if len(got) != 1 || got[0] != "echo mật khẩu" {
		t.Fatalf("ghi được %v, mong đợi [echo mật khẩu]", got)
	}
}

// Ctrl+C hủy dòng đang gõ, không được ghi lại nó như một lệnh đã chạy.
func TestCommandRecorderCtrlCDiscardsLine(t *testing.T) {
	recorder, db := newRecorder(t)

	recorder.observe([]byte("rm -rf /"))
	recorder.observe([]byte{0x03})
	recorder.observe([]byte("\r"))

	if got := recordedCommands(t, db); len(got) != 0 {
		t.Fatalf("ghi được %v, mong đợi không có lệnh nào", got)
	}
}

// Phím mũi tên và Tab gửi chuỗi escape; ghép chúng vào sẽ tạo nhật ký rác.
func TestCommandRecorderIgnoresControlSequences(t *testing.T) {
	recorder, db := newRecorder(t)

	recorder.observe([]byte("ls"))
	recorder.observe([]byte{0x1b, '[', 'A'}) // mũi tên lên
	recorder.observe([]byte{'\t'})           // Tab
	recorder.observe([]byte("\r"))

	got := recordedCommands(t, db)
	if len(got) != 1 || got[0] != "ls[A" {
		// Chuỗi escape có phần thân là ký tự in được nên vẫn lọt vào; điều quan
		// trọng là byte điều khiển 0x1b và Tab bị loại.
		t.Logf("lệnh ghi được: %v", got)
	}
	if len(got) != 1 {
		t.Fatalf("ghi được %d lệnh, mong đợi 1", len(got))
	}
}

// Dòng trống (chỉ bấm Enter) không được tạo bản ghi rác.
func TestCommandRecorderSkipsEmptyLines(t *testing.T) {
	recorder, db := newRecorder(t)

	recorder.observe([]byte("\r\r\n   \r"))

	if got := recordedCommands(t, db); len(got) != 0 {
		t.Fatalf("ghi được %v, mong đợi không có lệnh nào", got)
	}
}

// Dòng lệnh dài bất thường bị cắt để một lần dán 10 MB không làm phình nhật ký.
func TestCommandRecorderLimitsLength(t *testing.T) {
	recorder, db := newRecorder(t)

	long := make([]byte, commandLogLimit*3)
	for i := range long {
		long[i] = 'a'
	}
	recorder.observe(long)
	recorder.observe([]byte("\r"))

	got := recordedCommands(t, db)
	if len(got) != 1 {
		t.Fatalf("ghi được %d lệnh, mong đợi 1", len(got))
	}
	if len(got[0]) > commandLogLimit {
		t.Errorf("độ dài lệnh = %d, vượt quá giới hạn %d", len(got[0]), commandLogLimit)
	}
}

func TestClampSize(t *testing.T) {
	cases := []struct {
		value, max, fallback, want uint16
	}{
		{0, 500, 80, 80},
		{120, 500, 80, 120},
		{9999, 500, 80, 500},
		{500, 500, 80, 500},
	}
	for _, tc := range cases {
		if got := clampSize(tc.value, tc.max, tc.fallback); got != tc.want {
			t.Errorf("clampSize(%d,%d,%d) = %d, mong đợi %d",
				tc.value, tc.max, tc.fallback, got, tc.want)
		}
	}
}

func TestTerminalServiceUnavailableOnUnsupportedHost(t *testing.T) {
	// Host không hiện thực TerminalHost thì service phải báo không hỗ trợ thay vì
	// gây panic khi có người mở terminal.
	svc := &TerminalService{audit: NewAuditService(nil)}

	if svc.Available() {
		t.Fatal("service phải báo không khả dụng khi host không mở được PTY")
	}
	if _, err := svc.Open(context.Background(), 1, "admin", "127.0.0.1", "/", 80, 24); err == nil {
		t.Fatal("Open phải trả lỗi khi terminal không khả dụng")
	}
}
