package diskscan

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thanhtinz/sunpanel/pkg/host"
)

// newScanner dựng bộ quét trên một cây thư mục giả lập.
func newScanner(t *testing.T) (*Scanner, string) {
	t.Helper()

	root := t.TempDir()
	return New(host.NewLocalHost(root, nil).FS()), root
}

func write(t *testing.T, path string, size int) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("tạo thư mục: %v", err)
	}
	if err := os.WriteFile(path, []byte(strings.Repeat("x", size)), 0o644); err != nil {
		t.Fatalf("ghi %s: %v", path, err)
	}
}

func TestScanSumsDirectoriesAndSortsBySize(t *testing.T) {
	scanner, root := newScanner(t)

	write(t, filepath.Join(root, "nho", "a.txt"), 100)
	write(t, filepath.Join(root, "lon", "b.bin"), 5000)
	write(t, filepath.Join(root, "lon", "sau", "c.bin"), 3000)
	write(t, filepath.Join(root, "le.txt"), 20)

	report, err := scanner.Scan(context.Background(), "/")
	if err != nil {
		t.Fatalf("quét: %v", err)
	}

	if len(report.Entries) != 3 {
		t.Fatalf("có %d mục, mong 3: %+v", len(report.Entries), report.Entries)
	}
	// Mục lớn nhất phải đứng đầu — đó là thứ duy nhất người đang dọn đĩa cần thấy.
	first := report.Entries[0]
	if first.Name != "lon" || first.Size != 8000 {
		t.Errorf("mục đầu = %s (%d byte), mong lon (8000)", first.Name, first.Size)
	}
	if first.Files != 2 {
		t.Errorf("thư mục lon có %d tệp, mong 2", first.Files)
	}
	if report.Total != 8120 {
		t.Errorf("tổng = %d, mong 8120", report.Total)
	}
	if report.Partial {
		t.Error("cây thư mục nhỏ mà lại báo là quét chưa xong")
	}

	// Tỉ lệ phần trăm phải tính trên tổng của chính thư mục đang xem.
	if first.Percent < 98 || first.Percent > 99 {
		t.Errorf("tỉ lệ của mục lớn nhất = %.2f%%", first.Percent)
	}
}

// Liên kết mềm được tính bằng chính nó chứ không đi theo: đi theo thì một liên
// kết trỏ về thư mục cha thành vòng lặp vô tận.
func TestScanDoesNotFollowSymlinks(t *testing.T) {
	scanner, root := newScanner(t)

	write(t, filepath.Join(root, "that", "to.bin"), 4000)
	if err := os.Symlink(filepath.Join(root, "that"), filepath.Join(root, "lien-ket")); err != nil {
		t.Skipf("không tạo được liên kết mềm: %v", err)
	}

	report, err := scanner.Scan(context.Background(), "/")
	if err != nil {
		t.Fatalf("quét: %v", err)
	}

	for _, entry := range report.Entries {
		if entry.Name == "lien-ket" && entry.Size >= 4000 {
			t.Errorf("liên kết mềm bị tính bằng dung lượng thư mục đích: %d byte", entry.Size)
		}
	}
	if report.Total >= 8000 {
		t.Errorf("tổng = %d, dung lượng đã bị đếm hai lần", report.Total)
	}
}

// Hết ngân sách phải dừng và đánh dấu kết quả là chưa đầy đủ, chứ không im lặng
// trả về một con số trông như thật.
func TestScanStopsAtBudget(t *testing.T) {
	scanner, root := newScanner(t)
	scanner.budget = 5

	for i := 0; i < 40; i++ {
		write(t, filepath.Join(root, "nhieu", "tep-"+strings.Repeat("a", i%5)+string(rune('a'+i))), 10)
	}

	report, err := scanner.Scan(context.Background(), "/")
	if err != nil {
		t.Fatalf("quét: %v", err)
	}
	if !report.Partial {
		t.Error("vượt ngân sách nhưng không đánh dấu là quét chưa xong")
	}
}

func TestScanRejectsFile(t *testing.T) {
	scanner, root := newScanner(t)
	write(t, filepath.Join(root, "tep.txt"), 10)

	if _, err := scanner.Scan(context.Background(), "/tep.txt"); !errors.Is(err, ErrNotDirectory) {
		t.Fatalf("lỗi = %v, mong ErrNotDirectory", err)
	}
}

// Thư mục không đọc được không được làm hỏng cả lần quét: phần còn lại vẫn cho
// ra con số dùng được.
func TestScanSkipsUnreadableDirectory(t *testing.T) {
	scanner, root := newScanner(t)

	write(t, filepath.Join(root, "doc-duoc", "a.bin"), 1000)
	locked := filepath.Join(root, "khong-doc-duoc")
	if err := os.MkdirAll(filepath.Join(locked, "ben-trong"), 0o755); err != nil {
		t.Fatalf("tạo thư mục: %v", err)
	}
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Skipf("không đổi được quyền: %v", err)
	}
	defer func() { _ = os.Chmod(locked, 0o755) }()

	report, err := scanner.Scan(context.Background(), "/")
	if err != nil {
		t.Fatalf("quét: %v", err)
	}
	if report.Total < 1000 {
		t.Errorf("tổng = %d, phần đọc được đã bị mất", report.Total)
	}
}

// /proc và /sys là cửa sổ nhìn vào nhân chứ không phải dữ liệu trên đĩa: quét
// chúng vừa tốn gần hết thời gian vừa cho ra con số vô nghĩa.
func TestScanSkipsPseudoFilesystems(t *testing.T) {
	scanner, root := newScanner(t)

	write(t, filepath.Join(root, "proc", "kcore"), 9000)
	write(t, filepath.Join(root, "that", "a.bin"), 1000)

	report, err := scanner.Scan(context.Background(), "/")
	if err != nil {
		t.Fatalf("quét: %v", err)
	}

	for _, entry := range report.Entries {
		if entry.Name == "proc" && entry.Size != 0 {
			t.Errorf("/proc được tính %d byte", entry.Size)
		}
	}
	if report.Total != 1000 {
		t.Errorf("tổng = %d, mong 1000", report.Total)
	}
}
