package logs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thanhtinz/sunpanel/pkg/host"
)

// newReader dựng bộ đọc trên một thư mục nhật ký giả lập.
func newReader(t *testing.T) (*Reader, string) {
	t.Helper()

	root := t.TempDir()
	logDir := filepath.Join(root, "var", "log")
	if err := os.MkdirAll(filepath.Join(logDir, "nginx"), 0o755); err != nil {
		t.Fatalf("tạo thư mục: %v", err)
	}
	return New(host.NewLocalHost(root, nil), "/var/log"), logDir
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("ghi %s: %v", path, err)
	}
}

func TestSourcesFindsLogFiles(t *testing.T) {
	reader, dir := newReader(t)

	write(t, filepath.Join(dir, "syslog"), "dòng một\n")
	write(t, filepath.Join(dir, "auth.log"), "đăng nhập\n")
	write(t, filepath.Join(dir, "nginx", "error.log"), "lỗi\n")
	write(t, filepath.Join(dir, "syslog.1.gz"), "đã nén")
	write(t, filepath.Join(dir, "wtmp"), "nhị phân")

	sources, err := reader.Sources(context.Background())
	if err != nil {
		t.Fatalf("liệt kê nguồn: %v", err)
	}

	names := map[string]bool{}
	for _, source := range sources {
		names[source.Name] = true
	}

	for _, want := range []string{"syslog", "auth.log", "nginx/error.log"} {
		if !names[want] {
			t.Errorf("thiếu nguồn %q, có %v", want, names)
		}
	}
	// Bản đã nén và tệp nhị phân không mở ra được bằng trình xem văn bản, nên
	// mời người dùng bấm vào chúng chỉ tạo ra một màn hình đầy ký tự rác.
	for _, unwanted := range []string{"syslog.1.gz", "wtmp"} {
		if names[unwanted] {
			t.Errorf("%q không nên xuất hiện trong danh sách", unwanted)
		}
	}
}

func TestTailReturnsLastLines(t *testing.T) {
	reader, dir := newReader(t)

	lines := make([]string, 0, 500)
	for i := 0; i < 500; i++ {
		lines = append(lines, "dòng nhật ký số "+strings.Repeat("x", 20))
	}
	write(t, filepath.Join(dir, "app.log"), strings.Join(lines, "\n")+"\n")

	chunk, err := reader.Tail(context.Background(), "app.log", 50)
	if err != nil {
		t.Fatalf("đọc cuối tệp: %v", err)
	}
	if len(chunk.Lines) != 50 {
		t.Errorf("đọc được %d dòng, mong 50", len(chunk.Lines))
	}
	if chunk.Offset != chunk.Size {
		t.Errorf("vị trí kết thúc %d khác kích thước tệp %d", chunk.Offset, chunk.Size)
	}
}

// Theo dõi trực tiếp: lần đọc sau chỉ được trả về phần mới thêm vào, nếu không
// giao diện sẽ nhân đôi toàn bộ nhật ký sau mỗi chu kỳ.
func TestSinceReturnsOnlyNewLines(t *testing.T) {
	reader, dir := newReader(t)
	target := filepath.Join(dir, "app.log")
	write(t, target, "dòng cũ\n")

	first, err := reader.Tail(context.Background(), "app.log", 100)
	if err != nil {
		t.Fatalf("đọc lần đầu: %v", err)
	}

	file, err := os.OpenFile(target, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("mở tệp để ghi thêm: %v", err)
	}
	if _, err := file.WriteString("dòng mới\n"); err != nil {
		t.Fatalf("ghi thêm: %v", err)
	}
	_ = file.Close()

	second, err := reader.Since(context.Background(), "app.log", first.Offset)
	if err != nil {
		t.Fatalf("đọc phần mới: %v", err)
	}
	if len(second.Lines) != 1 || second.Lines[0] != "dòng mới" {
		t.Fatalf("phần mới = %v", second.Lines)
	}
	if second.Truncated {
		t.Error("tệp chỉ dài thêm ra, không phải bị cắt")
	}
}

// logrotate thay tệp bằng bản mới ngắn hơn; đọc tiếp từ vị trí cũ sẽ cho ra
// rác, nên bộ đọc phải nhận ra và đọc lại từ đầu.
func TestSinceDetectsRotation(t *testing.T) {
	reader, dir := newReader(t)
	target := filepath.Join(dir, "app.log")
	write(t, target, strings.Repeat("dòng dài dòng dài\n", 100))

	first, err := reader.Tail(context.Background(), "app.log", 10)
	if err != nil {
		t.Fatalf("đọc lần đầu: %v", err)
	}

	write(t, target, "tệp vừa được xoay vòng\n")

	second, err := reader.Since(context.Background(), "app.log", first.Offset)
	if err != nil {
		t.Fatalf("đọc sau khi xoay vòng: %v", err)
	}
	if !second.Truncated {
		t.Error("không nhận ra tệp đã bị xoay vòng")
	}
	if len(second.Lines) != 1 {
		t.Errorf("đọc được %v", second.Lines)
	}
}

// Trình xem nhật ký phải nằm đúng trong /var/log: gốc của lớp host là cả ổ đĩa,
// nên phép kiểm tra này mới là thứ ngăn nó mở /etc/shadow.
func TestResolveRejectsPathsOutsideRoot(t *testing.T) {
	reader, _ := newReader(t)

	for _, name := range []string{"../../etc/shadow", "/etc/shadow", "/var/logs-khac/app.log"} {
		if _, err := reader.Tail(context.Background(), name, 10); !errors.Is(err, ErrOutsideRoot) {
			t.Errorf("%q: lỗi = %v, mong ErrOutsideRoot", name, err)
		}
	}
}

// Dòng nhật ký của ứng dụng Java có thể dài vài chục nghìn ký tự; mức mặc định
// của bufio.Scanner sẽ làm cả lần đọc thất bại.
func TestTailHandlesVeryLongLines(t *testing.T) {
	reader, dir := newReader(t)
	write(t, filepath.Join(dir, "app.log"), strings.Repeat("x", 200_000)+"\nngắn\n")

	chunk, err := reader.Tail(context.Background(), "app.log", 10)
	if err != nil {
		t.Fatalf("đọc tệp có dòng dài: %v", err)
	}
	if len(chunk.Lines) == 0 {
		t.Fatal("không đọc được dòng nào")
	}
	if chunk.Lines[len(chunk.Lines)-1] != "ngắn" {
		t.Errorf("dòng cuối = %.20q", chunk.Lines[len(chunk.Lines)-1])
	}
}
