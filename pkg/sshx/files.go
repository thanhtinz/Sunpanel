package sshx

import (
	"context"
	"errors"
	"io"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/pkg/sftp"
)

// maxEditableSize giới hạn tệp mở được bằng trình sửa văn bản.
//
// Nội dung tệp đi qua JSON về trình duyệt; một tệp nhật ký vài trăm megabyte
// mở ra sẽ treo cả tab trình duyệt trước khi kịp hiện dòng đầu tiên.
const maxEditableSize = 2 << 20

// ErrTooLarge là tệp quá lớn để mở bằng trình sửa văn bản.
var ErrTooLarge = errors.New("sshx: tệp quá lớn để mở")

// FileInfo là một mục trong thư mục trên máy chủ từ xa.
type FileInfo struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Size  int64  `json:"size"`
	IsDir bool   `json:"isDir"`
	// IsLink cho biết đây là liên kết mềm.
	IsLink bool `json:"isLink"`
	// Mode là quyền dạng chuỗi, ví dụ "drwxr-xr-x".
	Mode string `json:"mode"`
	// ModTime là lần sửa gần nhất, tính bằng mili giây Unix.
	ModTime int64 `json:"modTime"`
	// Owner và Group là chủ sở hữu, dạng số vì máy từ xa không tra tên hộ được.
	Owner uint32 `json:"owner"`
	Group uint32 `json:"group"`
}

// Files mở một phiên SFTP trên kết nối đang có.
//
// Mỗi lời gọi mở một phiên riêng và bên gọi đóng nó: giữ một phiên SFTP sống
// mãi nghĩa là giữ một tiến trình sftp-server chạy trên máy đích kể cả khi
// không ai đang xem tệp.
func (c *Client) Files() (*Files, error) {
	client, err := sftp.NewClient(c.conn)
	if err != nil {
		return nil, err
	}
	return &Files{client: client}, nil
}

// Files là lớp thao tác tệp trên máy chủ từ xa.
type Files struct {
	client *sftp.Client
}

// Close đóng phiên SFTP.
func (f *Files) Close() error { return f.client.Close() }

// Getwd trả về thư mục làm việc của phiên SFTP.
//
// Dùng để biết "." đang là ở đâu: người dùng cần thấy đường dẫn thật chứ không
// phải một dấu chấm, và mọi bước đi tiếp đều dựng từ đường dẫn đó.
func (f *Files) Getwd() (string, error) { return f.client.Getwd() }

// List liệt kê nội dung một thư mục.
func (f *Files) List(dir string) ([]FileInfo, error) {
	dir = cleanRemotePath(dir)

	entries, err := f.client.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	out := make([]FileInfo, 0, len(entries))
	for _, entry := range entries {
		out = append(out, describe(dir, entry))
	}

	// Thư mục lên trước rồi tới tệp, mỗi nhóm xếp theo tên: đó là thứ tự mọi
	// trình quản lý tệp dùng, và mắt người đã quen tìm theo nó.
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

// Stat lấy thông tin một mục.
func (f *Files) Stat(target string) (FileInfo, error) {
	target = cleanRemotePath(target)

	info, err := f.client.Stat(target)
	if err != nil {
		return FileInfo{}, err
	}
	return describe(path.Dir(target), info), nil
}

// Read đọc nội dung một tệp văn bản.
func (f *Files) Read(target string) (string, error) {
	target = cleanRemotePath(target)

	info, err := f.client.Stat(target)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", errors.New("sshx: đây là thư mục")
	}
	if info.Size() > maxEditableSize {
		return "", ErrTooLarge
	}

	file, err := f.client.Open(target)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(io.LimitReader(file, maxEditableSize))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Write ghi đè nội dung một tệp.
func (f *Files) Write(target string, content string) error {
	target = cleanRemotePath(target)

	file, err := f.client.Create(target)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	_, err = file.Write([]byte(content))
	return err
}

// Upload chép một luồng dữ liệu lên máy chủ từ xa.
func (f *Files) Upload(ctx context.Context, target string, source io.Reader) (int64, error) {
	target = cleanRemotePath(target)

	file, err := f.client.Create(target)
	if err != nil {
		return 0, err
	}
	defer func() { _ = file.Close() }()

	// Chép theo khối và kiểm ctx giữa chừng: một tệp vài gigabyte gửi qua mạng
	// chậm sẽ giữ kết nối rất lâu, và người dùng phải hủy được.
	return copyWithContext(ctx, file, source)
}

// Download mở một tệp để tải về.
//
// Trả về cả kích thước để bên gọi khai báo Content-Length: thiếu nó thì trình
// duyệt không hiện được thanh tiến trình của một tệp lớn.
func (f *Files) Download(target string) (io.ReadCloser, int64, error) {
	target = cleanRemotePath(target)

	info, err := f.client.Stat(target)
	if err != nil {
		return nil, 0, err
	}
	if info.IsDir() {
		return nil, 0, errors.New("sshx: đây là thư mục")
	}

	file, err := f.client.Open(target)
	if err != nil {
		return nil, 0, err
	}
	return file, info.Size(), nil
}

// Mkdir tạo thư mục, kể cả các thư mục cha còn thiếu.
func (f *Files) Mkdir(target string) error {
	return f.client.MkdirAll(cleanRemotePath(target))
}

// Remove xóa một tệp hoặc cả cây thư mục.
func (f *Files) Remove(target string) error {
	target = cleanRemotePath(target)
	if target == "/" {
		return errors.New("sshx: không xóa được thư mục gốc")
	}

	info, err := f.client.Stat(target)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return f.client.RemoveAll(target)
	}
	return f.client.Remove(target)
}

// Rename đổi tên hoặc di chuyển một mục.
func (f *Files) Rename(oldPath, newPath string) error {
	return f.client.Rename(cleanRemotePath(oldPath), cleanRemotePath(newPath))
}

// Chmod đổi quyền truy cập.
func (f *Files) Chmod(target string, mode os.FileMode) error {
	return f.client.Chmod(cleanRemotePath(target), mode)
}

// describe đổi thông tin của SFTP sang kiểu của gói này.
func describe(dir string, info os.FileInfo) FileInfo {
	out := FileInfo{
		Name:    info.Name(),
		Path:    path.Join(dir, info.Name()),
		Size:    info.Size(),
		IsDir:   info.IsDir(),
		IsLink:  info.Mode()&os.ModeSymlink != 0,
		Mode:    info.Mode().String(),
		ModTime: info.ModTime().UnixMilli(),
	}
	if stat, ok := info.Sys().(*sftp.FileStat); ok {
		out.Owner, out.Group = stat.UID, stat.GID
	}
	return out
}

// cleanRemotePath chuẩn hóa đường dẫn trên máy từ xa.
//
// Luôn tuyệt đối: đường dẫn tương đối được hiểu theo thư mục làm việc của phiên
// SFTP, thứ mà người dùng không nhìn thấy và không đoán được.
func cleanRemotePath(target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return "/"
	}
	if !strings.HasPrefix(target, "/") {
		target = "/" + target
	}
	return path.Clean(target)
}

// copyWithContext chép dữ liệu và dừng khi ctx bị hủy.
func copyWithContext(ctx context.Context, dst io.Writer, src io.Reader) (int64, error) {
	buf := make([]byte, 64<<10)
	var total int64

	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}

		n, err := src.Read(buf)
		if n > 0 {
			written, writeErr := dst.Write(buf[:n])
			total += int64(written)
			if writeErr != nil {
				return total, writeErr
			}
		}
		switch {
		case errors.Is(err, io.EOF):
			return total, nil
		case err != nil:
			return total, err
		}
	}
}

// ModTimeOf đổi mốc mili giây thành time.Time, dùng cho phần hiển thị.
func ModTimeOf(info FileInfo) time.Time {
	return time.UnixMilli(info.ModTime)
}
