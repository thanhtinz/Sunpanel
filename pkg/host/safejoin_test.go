package host

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSafeJoinStaysWithinRoot(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "www", "html"))
	mustWrite(t, filepath.Join(root, "www", "html", "index.html"), "hello")

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"đường dẫn thường", "www/html/index.html", "www/html/index.html"},
		{"có dấu gạch đầu", "/www/html", "www/html"},
		{"gốc rỗng", "", ""},
		{"dấu chấm", ".", ""},
		{"nhiều dấu gạch", "//www///html//", "www/html"},
		{"chấm ở giữa", "www/./html", "www/html"},
		{"leo lên rồi xuống lại", "www/../www/html", "www/html"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SafeJoin(root, tc.input)
			if err != nil {
				t.Fatalf("SafeJoin(%q) lỗi bất ngờ: %v", tc.input, err)
			}
			want := filepath.Join(mustResolve(t, root), filepath.FromSlash(tc.want))
			if got != want {
				t.Errorf("SafeJoin(%q) = %q, mong đợi %q", tc.input, got, want)
			}
		})
	}
}

// Đây là bài test quan trọng nhất của toàn bộ dự án: mọi nỗ lực thoát ra ngoài
// thư mục gốc đều phải bị chặn hoặc bị kẹp lại bên trong gốc, không có ngoại lệ.
func TestSafeJoinBlocksTraversal(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "www"))

	inputs := []string{
		"../",
		"../../etc/shadow",
		"../../../../../../../../etc/passwd",
		"www/../../etc/passwd",
		"/etc/passwd",
		"/../etc/passwd",
		"www/../../../root/.ssh/id_rsa",
		"..",
		"./../..",
	}

	realRoot := mustResolve(t, root)
	for _, in := range inputs {
		t.Run(in, func(t *testing.T) {
			got, err := SafeJoin(root, in)
			if err != nil {
				// Bị từ chối thẳng cũng là kết quả đúng.
				return
			}
			if !within(realRoot, got) {
				t.Fatalf("SafeJoin(%q) = %q — đã thoát ra ngoài gốc %q", in, got, realRoot)
			}
		})
	}
}

func TestSafeJoinRejectsSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("tạo symlink trên Windows cần quyền đặc biệt")
	}

	root := t.TempDir()
	outside := t.TempDir()
	mustWrite(t, filepath.Join(outside, "secret.txt"), "dữ liệu nhạy cảm")

	// Symlink trỏ tới một thư mục hoàn toàn nằm ngoài gốc.
	link := filepath.Join(root, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatalf("tạo symlink: %v", err)
	}

	for _, in := range []string{"escape", "escape/secret.txt"} {
		t.Run(in, func(t *testing.T) {
			if _, err := SafeJoin(root, in); !errors.Is(err, ErrPathEscapesRoot) {
				t.Fatalf("SafeJoin(%q) lỗi = %v, mong đợi ErrPathEscapesRoot", in, err)
			}
		})
	}

	// "escape/../secret.txt" được làm sạch ở mức văn bản trước khi giải symlink,
	// nên nó trở thành <root>/secret.txt và không bao giờ chạm tới thư mục ngoài.
	// Cách diễn giải này an toàn hơn cách hệ điều hành làm (đi theo symlink rồi
	// mới leo lên, sẽ chạm đúng vào tệp bí mật).
	t.Run("escape/../secret.txt", func(t *testing.T) {
		got, err := SafeJoin(root, "escape/../secret.txt")
		if err != nil {
			t.Fatalf("SafeJoin lỗi bất ngờ: %v", err)
		}
		if want := filepath.Join(mustResolve(t, root), "secret.txt"); got != want {
			t.Fatalf("SafeJoin = %q, mong đợi %q (phải nằm trong gốc)", got, want)
		}
	})
}

func TestSafeJoinAllowsSymlinkInsideRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("tạo symlink trên Windows cần quyền đặc biệt")
	}

	root := t.TempDir()
	target := filepath.Join(root, "real")
	mustMkdir(t, target)
	mustWrite(t, filepath.Join(target, "file.txt"), "ok")

	if err := os.Symlink(target, filepath.Join(root, "alias")); err != nil {
		t.Fatalf("tạo symlink: %v", err)
	}

	got, err := SafeJoin(root, "alias/file.txt")
	if err != nil {
		t.Fatalf("symlink trong phạm vi gốc phải được chấp nhận, nhận lỗi: %v", err)
	}
	if want := filepath.Join(mustResolve(t, root), "real", "file.txt"); got != want {
		t.Errorf("SafeJoin = %q, mong đợi %q", got, want)
	}
}

// Đường dẫn của tệp chưa tồn tại vẫn phải ghép được — panel cần tạo tệp mới.
func TestSafeJoinAllowsMissingPath(t *testing.T) {
	root := t.TempDir()

	got, err := SafeJoin(root, "chua/ton/tai.txt")
	if err != nil {
		t.Fatalf("SafeJoin với đường dẫn chưa tồn tại lỗi: %v", err)
	}
	if want := filepath.Join(mustResolve(t, root), "chua", "ton", "tai.txt"); got != want {
		t.Errorf("SafeJoin = %q, mong đợi %q", got, want)
	}
}

// Symlink hỏng bên trong gốc không được làm hàm sập.
func TestSafeJoinHandlesBrokenSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("tạo symlink trên Windows cần quyền đặc biệt")
	}

	root := t.TempDir()
	if err := os.Symlink(filepath.Join(root, "khong-ton-tai"), filepath.Join(root, "hong")); err != nil {
		t.Fatalf("tạo symlink: %v", err)
	}

	got, err := SafeJoin(root, "hong")
	if err != nil {
		t.Fatalf("symlink hỏng gây lỗi: %v", err)
	}
	if !within(mustResolve(t, root), got) {
		t.Errorf("SafeJoin = %q — nằm ngoài gốc", got)
	}
}

func TestSafeJoinRejectsNullByte(t *testing.T) {
	root := t.TempDir()
	if _, err := SafeJoin(root, "www/index.html\x00.jpg"); !errors.Is(err, ErrPathEscapesRoot) {
		t.Fatalf("byte NUL phải bị từ chối, nhận lỗi: %v", err)
	}
}

func TestWithin(t *testing.T) {
	sep := string(filepath.Separator)
	cases := []struct {
		root, path string
		want       bool
	}{
		{sep + "srv", sep + "srv", true},
		{sep + "srv", sep + "srv" + sep + "www", true},
		{sep + "srv", sep + "srv-khac", false},
		{sep + "srv", sep + "etc", false},
		{sep + "srv", sep, false},
	}
	for _, tc := range cases {
		if got := within(tc.root, tc.path); got != tc.want {
			t.Errorf("within(%q, %q) = %v, mong đợi %v", tc.root, tc.path, got, tc.want)
		}
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("tạo thư mục %s: %v", path, err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("ghi tệp %s: %v", path, err)
	}
}

// mustResolve giải symlink của thư mục tạm (trên macOS /tmp là symlink tới /private/tmp).
func mustResolve(t *testing.T, path string) string {
	t.Helper()
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("giải symlink %s: %v", path, err)
	}
	return strings.TrimSuffix(real, string(filepath.Separator))
}
