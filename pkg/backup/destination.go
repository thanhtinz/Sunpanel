// Package backup sao lưu cơ sở dữ liệu và thư mục, rồi đẩy lên nơi lưu trữ.
//
// Bản sao lưu nằm cùng máy với dữ liệu gốc thì chỉ chống được lỗi người dùng,
// không chống được hỏng đĩa hay mất máy chủ. Vì vậy panel hỗ trợ đẩy bản sao lên
// nơi khác — S3 hoặc WebDAV — ngay từ đầu chứ không coi đó là tính năng phụ.
package backup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Các lỗi dùng chung.
var (
	// ErrUnsupportedDestination là loại nơi lưu trữ không được hỗ trợ.
	ErrUnsupportedDestination = errors.New("backup: nơi lưu trữ không được hỗ trợ")
	// ErrInvalidName là tên tệp sao lưu không hợp lệ.
	ErrInvalidName = errors.New("backup: tên tệp sao lưu không hợp lệ")
	// ErrNotFound là không tìm thấy bản sao lưu.
	ErrNotFound = errors.New("backup: không tìm thấy bản sao lưu")
)

// Kind là loại nơi lưu trữ.
type Kind string

// Các loại nơi lưu trữ được hỗ trợ.
const (
	// Local lưu ngay trên máy chủ.
	Local Kind = "local"
	// S3 lưu lên dịch vụ tương thích S3 (AWS S3, MinIO, Backblaze B2, Wasabi…).
	S3 Kind = "s3"
	// WebDAV lưu lên máy chủ WebDAV (Nextcloud, ownCloud…).
	WebDAV Kind = "webdav"
)

// Valid cho biết loại có được hỗ trợ hay không.
func (k Kind) Valid() bool { return k == Local || k == S3 || k == WebDAV }

// Object là một tệp sao lưu ở nơi lưu trữ.
type Object struct {
	Name      string    `json:"name"`
	SizeBytes int64     `json:"sizeBytes"`
	ModTime   time.Time `json:"modTime"`
}

// Destination là nơi lưu bản sao lưu.
type Destination interface {
	// Kind là loại nơi lưu trữ.
	Kind() Kind
	// Put tải một tệp lên. size là kích thước, cần cho các giao thức đòi biết trước.
	Put(ctx context.Context, name string, r io.Reader, size int64) error
	// Get tải một tệp về. Bên gọi có trách nhiệm đóng.
	Get(ctx context.Context, name string) (io.ReadCloser, error)
	// List liệt kê các tệp sao lưu.
	List(ctx context.Context) ([]Object, error)
	// Delete xóa một tệp.
	Delete(ctx context.Context, name string) error
	// Check kiểm tra nơi lưu trữ có dùng được không.
	Check(ctx context.Context) error
}

// validObjectName khớp tên tệp sao lưu.
//
// Tên được ghép vào đường dẫn tệp và URL, nên không được chứa dấu gạch chéo hay
// dấu chấm kép — một tên như "../../etc/passwd" sẽ ghi đè tệp ngoài thư mục sao lưu.
var validObjectName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,190}$`)

// ValidateName kiểm tra tên một tệp sao lưu.
func ValidateName(name string) error {
	if !validObjectName.MatchString(name) || strings.Contains(name, "..") {
		return fmt.Errorf("%w: %q", ErrInvalidName, name)
	}
	return nil
}

// LocalDestination lưu bản sao lưu trong một thư mục trên máy chủ.
type LocalDestination struct {
	dir string
}

// NewLocal tạo nơi lưu trữ trên đĩa.
func NewLocal(dir string) *LocalDestination { return &LocalDestination{dir: dir} }

// Kind trả về loại nơi lưu trữ.
func (d *LocalDestination) Kind() Kind { return Local }

func (d *LocalDestination) pathFor(name string) (string, error) {
	if err := ValidateName(name); err != nil {
		return "", err
	}
	return filepath.Join(d.dir, name), nil
}

// Put ghi một tệp sao lưu xuống đĩa.
func (d *LocalDestination) Put(_ context.Context, name string, r io.Reader, _ int64) error {
	target, err := d.pathFor(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(d.dir, 0o700); err != nil {
		return fmt.Errorf("tạo thư mục sao lưu: %w", err)
	}

	// Ghi ra tệp tạm rồi đổi tên: một lần sao lưu bị ngắt giữa chừng không được
	// để lại tệp cụt mang đúng tên bản sao lưu, vì người dùng sẽ tưởng nó dùng được.
	temp := target + ".dang-ghi"
	file, err := os.OpenFile(temp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("tạo tệp sao lưu: %w", err)
	}

	if _, err := io.Copy(file, r); err != nil {
		file.Close()
		os.Remove(temp)
		return fmt.Errorf("ghi tệp sao lưu: %w", err)
	}
	if err := file.Close(); err != nil {
		os.Remove(temp)
		return fmt.Errorf("đóng tệp sao lưu: %w", err)
	}
	if err := os.Rename(temp, target); err != nil {
		os.Remove(temp)
		return fmt.Errorf("đổi tên tệp sao lưu: %w", err)
	}
	return nil
}

// Get mở một tệp sao lưu để đọc.
func (d *LocalDestination) Get(_ context.Context, name string) (io.ReadCloser, error) {
	target, err := d.pathFor(name)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(target)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, name)
		}
		return nil, fmt.Errorf("mở tệp sao lưu: %w", err)
	}
	return file, nil
}

// List liệt kê các tệp sao lưu trên đĩa.
func (d *LocalDestination) List(_ context.Context) ([]Object, error) {
	entries, err := os.ReadDir(d.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("đọc thư mục sao lưu: %w", err)
	}

	var out []Object
	for _, entry := range entries {
		if entry.IsDir() || strings.HasSuffix(entry.Name(), ".dang-ghi") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		out = append(out, Object{Name: entry.Name(), SizeBytes: info.Size(), ModTime: info.ModTime()})
	}
	sortObjects(out)
	return out, nil
}

// Delete xóa một tệp sao lưu.
func (d *LocalDestination) Delete(_ context.Context, name string) error {
	target, err := d.pathFor(name)
	if err != nil {
		return err
	}
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("xóa tệp sao lưu: %w", err)
	}
	return nil
}

// Check kiểm tra thư mục sao lưu ghi được không.
func (d *LocalDestination) Check(_ context.Context) error {
	if err := os.MkdirAll(d.dir, 0o700); err != nil {
		return fmt.Errorf("tạo thư mục sao lưu: %w", err)
	}

	probe := filepath.Join(d.dir, ".sunpanel-kiem-tra")
	if err := os.WriteFile(probe, []byte("ok"), 0o600); err != nil {
		return fmt.Errorf("thư mục sao lưu không ghi được: %w", err)
	}
	return os.Remove(probe)
}

// sortObjects xếp bản sao lưu mới nhất lên đầu.
func sortObjects(objects []Object) {
	sort.Slice(objects, func(i, j int) bool { return objects[i].ModTime.After(objects[j].ModTime) })
}

// joinPath ghép hai phần đường dẫn URL, tránh sinh ra dấu gạch chéo kép.
func joinPath(prefix, name string) string {
	prefix = strings.Trim(prefix, "/")
	if prefix == "" {
		return name
	}
	return path.Join(prefix, name)
}
