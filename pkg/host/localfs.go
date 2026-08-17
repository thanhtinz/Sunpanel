package host

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// LocalFS truy cập hệ thống tệp của máy cục bộ, giới hạn trong root.
type LocalFS struct {
	root string
}

var _ FileSystem = (*LocalFS)(nil)

// NewLocalFS tạo lớp truy cập tệp giới hạn trong thư mục root.
func NewLocalFS(root string) *LocalFS {
	return &LocalFS{root: root}
}

// Root trả về thư mục gốc.
func (f *LocalFS) Root() string { return f.root }

// List liệt kê nội dung thư mục, sắp xếp thư mục trước rồi tới tệp theo tên.
func (f *LocalFS) List(_ context.Context, path string) ([]FileInfo, error) {
	abs, err := f.resolve(path)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(abs)
	if err != nil {
		return nil, fmt.Errorf("đọc thư mục: %w", err)
	}

	out := make([]FileInfo, 0, len(entries))
	for _, e := range entries {
		// Bỏ qua mục vừa bị xóa giữa chừng thay vì làm hỏng cả danh sách.
		info, err := f.describe(filepath.Join(abs, e.Name()))
		if err != nil {
			continue
		}
		out = append(out, info)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir
		}
		return out[i].Name < out[j].Name
	})

	return out, nil
}

// Stat lấy thông tin một mục.
func (f *LocalFS) Stat(_ context.Context, path string) (FileInfo, error) {
	abs, err := f.resolve(path)
	if err != nil {
		return FileInfo{}, err
	}
	return f.describe(abs)
}

// Open mở tệp để đọc.
func (f *LocalFS) Open(_ context.Context, path string) (io.ReadCloser, error) {
	abs, err := f.resolve(path)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(abs)
	if err != nil {
		return nil, fmt.Errorf("mở tệp: %w", err)
	}
	return file, nil
}

// Write ghi nội dung vào tệp qua tệp tạm rồi đổi tên, để việc ghi là nguyên tử —
// nếu panel bị dừng giữa chừng thì tệp cũ vẫn còn nguyên vẹn.
func (f *LocalFS) Write(_ context.Context, path string, r io.Reader, mode os.FileMode) error {
	abs, err := f.resolve(path)
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(abs), ".sunpanel-*")
	if err != nil {
		return fmt.Errorf("tạo tệp tạm: %w", err)
	}
	tmpName := tmp.Name()
	// Không sao nếu tệp tạm đã được đổi tên thành công — lúc đó nó không còn tồn tại.
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := io.Copy(tmp, r); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("ghi tệp tạm: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("đóng tệp tạm: %w", err)
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		return fmt.Errorf("đặt quyền: %w", err)
	}
	if err := os.Rename(tmpName, abs); err != nil {
		return fmt.Errorf("đổi tên tệp tạm: %w", err)
	}

	return nil
}

// Mkdir tạo thư mục cùng các thư mục cha còn thiếu.
func (f *LocalFS) Mkdir(_ context.Context, path string, mode os.FileMode) error {
	abs, err := f.resolve(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(abs, mode); err != nil {
		return fmt.Errorf("tạo thư mục: %w", err)
	}
	return nil
}

// Remove xóa một mục. Không cho phép xóa chính thư mục gốc.
func (f *LocalFS) Remove(_ context.Context, path string, recursive bool) error {
	abs, err := f.resolve(path)
	if err != nil {
		return err
	}
	if root, err := f.resolve("/"); err == nil && abs == root {
		return fmt.Errorf("%w: không thể xóa thư mục gốc", ErrPathEscapesRoot)
	}

	if recursive {
		err = os.RemoveAll(abs)
	} else {
		err = os.Remove(abs)
	}
	if err != nil {
		return fmt.Errorf("xóa: %w", err)
	}
	return nil
}

// Move đổi tên hoặc di chuyển một mục; cả nguồn lẫn đích đều phải trong gốc.
func (f *LocalFS) Move(_ context.Context, oldPath, newPath string) error {
	oldAbs, err := f.resolve(oldPath)
	if err != nil {
		return err
	}
	newAbs, err := f.resolve(newPath)
	if err != nil {
		return err
	}
	if err := os.Rename(oldAbs, newAbs); err != nil {
		return fmt.Errorf("di chuyển: %w", err)
	}
	return nil
}

// Chmod đổi quyền truy cập.
func (f *LocalFS) Chmod(_ context.Context, path string, mode os.FileMode) error {
	abs, err := f.resolve(path)
	if err != nil {
		return err
	}
	if err := os.Chmod(abs, mode); err != nil {
		return fmt.Errorf("đổi quyền: %w", err)
	}
	return nil
}

// resolve là điểm duy nhất biến đường dẫn người dùng thành đường dẫn thật.
// Mọi phương thức ở trên đều phải đi qua đây.
func (f *LocalFS) resolve(path string) (string, error) {
	return SafeJoin(f.root, path)
}

func (f *LocalFS) describe(abs string) (FileInfo, error) {
	// Lstat chứ không phải Stat: cần biết bản thân mục có phải symlink hay không.
	st, err := os.Lstat(abs)
	if err != nil {
		return FileInfo{}, fmt.Errorf("đọc thông tin tệp: %w", err)
	}

	rel, err := filepath.Rel(f.root, abs)
	if err != nil {
		rel = st.Name()
	}

	info := FileInfo{
		Name:    st.Name(),
		Path:    filepath.ToSlash(filepath.Join("/", rel)),
		Size:    st.Size(),
		Mode:    st.Mode().Perm(),
		ModeStr: fmt.Sprintf("%04o", st.Mode().Perm()),
		IsDir:   st.IsDir(),
		IsLink:  st.Mode()&os.ModeSymlink != 0,
		ModTime: st.ModTime(),
	}
	if info.IsLink {
		if target, err := os.Readlink(abs); err == nil {
			info.LinkTarget = target
		}
		// Thư mục đích của symlink cũng nên hiển thị như thư mục.
		if target, err := os.Stat(abs); err == nil {
			info.IsDir = target.IsDir()
		}
	}
	fillOwnership(&info, st)

	return info, nil
}
