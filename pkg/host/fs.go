package host

import (
	"context"
	"io"
	"os"
	"time"
)

// FileInfo mô tả một mục trong hệ thống tệp mà panel hiển thị cho người dùng.
type FileInfo struct {
	Name    string      `json:"name"`
	Path    string      `json:"path"`
	Size    int64       `json:"size"`
	Mode    os.FileMode `json:"-"`
	ModeStr string      `json:"mode"`
	IsDir   bool        `json:"isDir"`
	IsLink  bool        `json:"isLink"`
	// LinkTarget chỉ có giá trị khi IsLink là true.
	LinkTarget string    `json:"linkTarget,omitempty"`
	ModTime    time.Time `json:"modTime"`
	Owner      string    `json:"owner,omitempty"`
	Group      string    `json:"group,omitempty"`
}

// FileSystem là các thao tác tệp mà panel cần.
//
// Mọi hiện thực bắt buộc phải giới hạn đường dẫn trong Root thông qua SafeJoin —
// path traversal là lỗ hổng nghiêm trọng nhất của loại phần mềm này.
type FileSystem interface {
	// Root là thư mục gốc mà mọi đường dẫn bị giới hạn bên trong.
	Root() string
	// List liệt kê nội dung một thư mục.
	List(ctx context.Context, path string) ([]FileInfo, error)
	// Stat lấy thông tin một mục.
	Stat(ctx context.Context, path string) (FileInfo, error)
	// Open mở tệp để đọc. Bên gọi có trách nhiệm đóng.
	Open(ctx context.Context, path string) (io.ReadCloser, error)
	// Write ghi đè nội dung tệp, tạo mới nếu chưa tồn tại.
	Write(ctx context.Context, path string, r io.Reader, mode os.FileMode) error
	// Mkdir tạo thư mục cùng các thư mục cha còn thiếu.
	Mkdir(ctx context.Context, path string, mode os.FileMode) error
	// Remove xóa một mục; recursive cho phép xóa cả cây thư mục.
	Remove(ctx context.Context, path string, recursive bool) error
	// Move đổi tên hoặc di chuyển một mục.
	Move(ctx context.Context, oldPath, newPath string) error
	// Chmod đổi quyền truy cập.
	Chmod(ctx context.Context, path string, mode os.FileMode) error
}
