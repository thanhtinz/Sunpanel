package host

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SafeJoin ghép một đường dẫn do người dùng cung cấp vào thư mục gốc và bảo đảm
// kết quả nằm bên trong gốc đó.
//
// Đây là hàng rào an ninh quan trọng nhất của panel. Nó chống được cả ba dạng
// tấn công phổ biến:
//
//  1. Đường dẫn tương đối leo lên trên ("../../etc/shadow").
//  2. Đường dẫn tuyệt đối ("/etc/shadow") — bị coi là tương đối so với gốc.
//  3. Symlink trỏ ra ngoài gốc — được phát hiện bằng cách giải symlink của
//     tổ tiên tồn tại sâu nhất rồi kiểm tra lại phạm vi.
//
// Đường dẫn trả về là tuyệt đối, đã làm sạch, và luôn nằm trong root.
func SafeJoin(root, unsafePath string) (string, error) {
	if strings.ContainsRune(unsafePath, '\x00') {
		return "", fmt.Errorf("%w: đường dẫn chứa byte NUL", ErrPathEscapesRoot)
	}

	realRoot, err := resolveExisting(root)
	if err != nil {
		return "", fmt.Errorf("giải thư mục gốc %q: %w", root, err)
	}

	// Ép đường dẫn về dạng "tương đối so với gốc" theo kiểu chroot: thêm dấu phân
	// cách ở đầu rồi Clean sẽ nuốt hết các thành phần ".." leo quá gốc, đồng thời
	// biến đường dẫn tuyệt đối thành đường dẫn bên trong gốc.
	rooted := filepath.Clean(string(filepath.Separator) + filepath.FromSlash(unsafePath))
	candidate := filepath.Join(realRoot, rooted)

	// Bước trên đã xử lý ".." dạng văn bản. Bước này bắt symlink: giải các liên kết
	// tượng trưng thật sự trên đĩa rồi kiểm tra lại phạm vi.
	resolved, err := resolveExisting(candidate)
	if err != nil {
		return "", fmt.Errorf("giải đường dẫn: %w", err)
	}
	if !within(realRoot, resolved) {
		return "", fmt.Errorf("%w: %s", ErrPathEscapesRoot, unsafePath)
	}

	return resolved, nil
}

// resolveExisting giải symlink cho phần tổ tiên đã tồn tại của đường dẫn, rồi
// ghép lại phần chưa tồn tại. Nhờ vậy hàm dùng được cho cả tệp sắp được tạo mới.
func resolveExisting(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	var missing []string
	current := abs
	for {
		real, err := filepath.EvalSymlinks(current)
		if err == nil {
			// Ghép lại các thành phần chưa tồn tại vào tổ tiên đã giải xong.
			for i := len(missing) - 1; i >= 0; i-- {
				real = filepath.Join(real, missing[i])
			}
			return filepath.Clean(real), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}

		parent := filepath.Dir(current)
		if parent == current {
			// Đã lên tới gốc hệ thống tệp mà vẫn không tồn tại: không còn symlink
			// nào để giải, dùng luôn dạng đã làm sạch.
			return filepath.Clean(abs), nil
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
}

// within cho biết path có nằm trong root hay chính là root.
func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
