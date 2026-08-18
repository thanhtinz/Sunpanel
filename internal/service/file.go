package service

import (
	"context"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/pkg/host"
)

// Giới hạn của trình quản lý tệp.
const (
	// maxEditableSize là kích thước tối đa được mở trong trình soạn thảo.
	// Mở một tệp log 2 GB trong trình duyệt sẽ làm treo máy người dùng.
	maxEditableSize = 8 << 20 // 8 MB
)

// FileService cung cấp các thao tác tệp cho giao diện quản trị.
//
// Service không tự thao tác trên hệ thống tệp: mọi thứ đi qua host.FileSystem,
// vốn đã giới hạn phạm vi bằng SafeJoin.
type FileService struct {
	host  host.Host
	audit *AuditService
}

// NewFileService tạo service quản lý tệp.
func NewFileService(h host.Host, audit *AuditService) *FileService {
	return &FileService{host: h, audit: audit}
}

// ListResult là nội dung một thư mục kèm thông tin điều hướng.
type ListResult struct {
	// Path là đường dẫn đang xem, luôn ở dạng tuyệt đối so với gốc.
	Path string `json:"path"`
	// Parent là thư mục cha, rỗng nếu đang ở gốc.
	Parent string          `json:"parent"`
	Items  []host.FileInfo `json:"items"`
	// Total là số mục trong thư mục.
	Total int `json:"total"`
}

// List liệt kê nội dung một thư mục.
func (s *FileService) List(ctx context.Context, dir string) (*ListResult, error) {
	items, err := s.host.FS().List(ctx, dir)
	if err != nil {
		return nil, translateFSError(err)
	}

	clean := normalizePath(dir)
	result := &ListResult{Path: clean, Items: items, Total: len(items)}
	if clean != "/" {
		result.Parent = path.Dir(clean)
	}
	return result, nil
}

// Stat lấy thông tin một mục.
func (s *FileService) Stat(ctx context.Context, p string) (host.FileInfo, error) {
	info, err := s.host.FS().Stat(ctx, p)
	if err != nil {
		return host.FileInfo{}, translateFSError(err)
	}
	return info, nil
}

// FileContent là nội dung một tệp văn bản đang được sửa.
type FileContent struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
}

// Read đọc nội dung một tệp văn bản để hiển thị trong trình soạn thảo.
func (s *FileService) Read(ctx context.Context, p string) (*FileContent, error) {
	info, err := s.host.FS().Stat(ctx, p)
	if err != nil {
		return nil, translateFSError(err)
	}
	if info.IsDir {
		return nil, apperr.FileIsDirectory
	}
	if info.Size > maxEditableSize {
		return nil, apperr.FileTooLarge.WithParam("max", maxEditableSize)
	}

	reader, err := s.host.FS().Open(ctx, p)
	if err != nil {
		return nil, translateFSError(err)
	}
	defer func() { _ = reader.Close() }()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, apperr.Internal.Wrap(err)
	}

	// Tệp nhị phân hiển thị trong trình soạn thảo văn bản chỉ tạo ra rác và có
	// nguy cơ bị lưu đè làm hỏng tệp.
	if isBinary(data) {
		return nil, apperr.FileNotText
	}

	return &FileContent{
		Path:    normalizePath(p),
		Content: string(data),
		Size:    info.Size,
		Mode:    info.ModeStr,
	}, nil
}

// Write ghi nội dung mới vào một tệp.
func (s *FileService) Write(ctx context.Context, p, content string) error {
	// Giữ nguyên quyền của tệp cũ; tệp mới dùng quyền mặc định an toàn.
	mode := os.FileMode(0o644)
	if info, err := s.host.FS().Stat(ctx, p); err == nil {
		if info.IsDir {
			return apperr.FileIsDirectory
		}
		mode = info.Mode
	}

	if err := s.host.FS().Write(ctx, p, strings.NewReader(content), mode); err != nil {
		return translateFSError(err)
	}
	return nil
}

// Upload lưu một tệp tải lên vào thư mục đích.
func (s *FileService) Upload(ctx context.Context, dir, name string, r io.Reader) error {
	// Tên tệp do người dùng đặt không được chứa thành phần đường dẫn — nếu không
	// một tên như "../../etc/cron.d/x" sẽ ghi ra ngoài thư mục đích.
	base := filepath.Base(filepath.FromSlash(name))
	if base == "." || base == ".." || base == string(filepath.Separator) {
		return apperr.FileInvalidName
	}

	target := path.Join(normalizePath(dir), base)
	if err := s.host.FS().Write(ctx, target, r, 0o644); err != nil {
		return translateFSError(err)
	}
	return nil
}

// Open mở một tệp để tải xuống.
func (s *FileService) Open(ctx context.Context, p string) (io.ReadCloser, host.FileInfo, error) {
	info, err := s.host.FS().Stat(ctx, p)
	if err != nil {
		return nil, host.FileInfo{}, translateFSError(err)
	}
	if info.IsDir {
		return nil, host.FileInfo{}, apperr.FileIsDirectory
	}

	reader, err := s.host.FS().Open(ctx, p)
	if err != nil {
		return nil, host.FileInfo{}, translateFSError(err)
	}
	return reader, info, nil
}

// Mkdir tạo thư mục mới.
func (s *FileService) Mkdir(ctx context.Context, p string) error {
	if err := s.host.FS().Mkdir(ctx, p, 0o755); err != nil {
		return translateFSError(err)
	}
	return nil
}

// Remove xóa một danh sách mục. Trả về lỗi đầu tiên gặp phải nhưng vẫn cố xóa
// hết phần còn lại, để một mục hỏng không chặn cả thao tác hàng loạt.
func (s *FileService) Remove(ctx context.Context, paths []string) error {
	var firstErr error
	for _, p := range paths {
		if err := s.host.FS().Remove(ctx, p, true); err != nil && firstErr == nil {
			firstErr = translateFSError(err)
		}
	}
	return firstErr
}

// Move di chuyển hoặc đổi tên một mục.
func (s *FileService) Move(ctx context.Context, from, to string) error {
	if err := s.host.FS().Move(ctx, from, to); err != nil {
		return translateFSError(err)
	}
	return nil
}

// Chmod đổi quyền truy cập. mode nhận chuỗi bát phân, ví dụ "0644".
func (s *FileService) Chmod(ctx context.Context, p, mode string) error {
	parsed, err := strconv.ParseUint(strings.TrimPrefix(mode, "0o"), 8, 32)
	if err != nil || parsed > 0o7777 {
		return apperr.FileInvalidMode
	}

	if err := s.host.FS().Chmod(ctx, p, os.FileMode(parsed)); err != nil {
		return translateFSError(err)
	}
	return nil
}

// normalizePath đưa đường dẫn về dạng tuyệt đối đã làm sạch, dùng dấu gạch chéo
// xuôi để giao diện hiển thị nhất quán trên mọi nền tảng.
func normalizePath(p string) string {
	clean := path.Clean("/" + strings.ReplaceAll(strings.TrimSpace(p), "\\", "/"))
	return clean
}

// isBinary đoán một tệp có phải nhị phân hay không.
//
// Byte NUL trong phần đầu tệp là dấu hiệu đáng tin cậy và rẻ: các định dạng văn
// bản (kể cả UTF-8 tiếng Việt) không bao giờ chứa byte này.
func isBinary(data []byte) bool {
	limit := min(len(data), 8000)
	for i := 0; i < limit; i++ {
		if data[i] == 0 {
			return true
		}
	}
	return false
}

// translateFSError chuyển lỗi hệ thống tệp thành lỗi có mã dịch.
func translateFSError(err error) error {
	switch {
	case err == nil:
		return nil
	case strings.Contains(err.Error(), host.ErrPathEscapesRoot.Error()):
		return apperr.Forbidden.Wrap(err)
	case os.IsNotExist(err) || strings.Contains(err.Error(), "no such file"):
		return apperr.FileNotFound.Wrap(err)
	case os.IsPermission(err) || strings.Contains(err.Error(), "permission denied"):
		return apperr.FilePermissionDenied.Wrap(err)
	case strings.Contains(err.Error(), "directory not empty"):
		return apperr.FileDirectoryNotEmpty.Wrap(err)
	case strings.Contains(err.Error(), "file exists"):
		return apperr.FileAlreadyExists.Wrap(err)
	default:
		return apperr.Internal.Wrap(err)
	}
}
