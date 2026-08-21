package service

import (
	"context"
	"errors"
	"io"
	"os"
	"path"
	"strings"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/model"
	"github.com/thanhtinz/sunpanel/pkg/sshx"
)

// NodeFileRequest là một thao tác tệp trên máy chủ từ xa.
type NodeFileRequest struct {
	Path    string `json:"path"`
	NewPath string `json:"newPath,omitempty"`
	Content string `json:"content,omitempty"`
	Mode    string `json:"mode,omitempty"`
}

// files mở phiên SFTP tới một máy chủ SSH.
//
// Trả về cả hàm đóng: phiên SFTP giữ một tiến trình sftp-server chạy trên máy
// đích, nên nó phải chết ngay khi thao tác xong.
func (s *NodeService) files(ctx context.Context, id uint) (*sshx.Files, func(), error) {
	record, err := s.find(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if record.Kind != model.NodeSSH {
		return nil, nil, apperr.NodeNotSSH
	}

	client, err := s.sshClient(ctx, record)
	if err != nil {
		return nil, nil, err
	}

	files, err := client.Files()
	if err != nil {
		// Kết nối đang giữ có thể đã chết, hoặc máy đích tắt hẳn subsystem sftp —
		// trường hợp thứ hai gặp trên các bản cài đặt siết chặt.
		s.dropSSH(id)
		return nil, nil, apperr.NodeSFTPUnavailable.WithParam("message", trimMessage(err.Error()))
	}
	return files, func() { _ = files.Close() }, nil
}

// ListFiles liệt kê nội dung một thư mục trên máy chủ từ xa.
func (s *NodeService) ListFiles(ctx context.Context, id uint, dir string) ([]sshx.FileInfo, error) {
	files, done, err := s.files(ctx, id)
	if err != nil {
		return nil, err
	}
	defer done()

	entries, err := files.List(dir)
	if err != nil {
		return nil, translateFileError(err)
	}
	return entries, nil
}

// ReadFile đọc nội dung một tệp văn bản trên máy chủ từ xa.
func (s *NodeService) ReadFile(ctx context.Context, id uint, target string) (string, error) {
	files, done, err := s.files(ctx, id)
	if err != nil {
		return "", err
	}
	defer done()

	content, err := files.Read(target)
	if err != nil {
		return "", translateFileError(err)
	}
	return content, nil
}

// WriteFile ghi đè nội dung một tệp trên máy chủ từ xa.
func (s *NodeService) WriteFile(ctx context.Context, id uint, req NodeFileRequest, actor AuditEntry) error {
	return s.mutate(ctx, id, req.Path, "node.file_write", actor, func(files *sshx.Files) error {
		return files.Write(req.Path, req.Content)
	})
}

// Mkdir tạo thư mục trên máy chủ từ xa.
func (s *NodeService) Mkdir(ctx context.Context, id uint, req NodeFileRequest, actor AuditEntry) error {
	return s.mutate(ctx, id, req.Path, "node.file_mkdir", actor, func(files *sshx.Files) error {
		return files.Mkdir(req.Path)
	})
}

// RemoveFile xóa một tệp hoặc thư mục trên máy chủ từ xa.
func (s *NodeService) RemoveFile(ctx context.Context, id uint, target string, actor AuditEntry) error {
	return s.mutate(ctx, id, target, "node.file_remove", actor, func(files *sshx.Files) error {
		return files.Remove(target)
	})
}

// RenameFile đổi tên hoặc di chuyển một mục trên máy chủ từ xa.
func (s *NodeService) RenameFile(ctx context.Context, id uint, req NodeFileRequest, actor AuditEntry) error {
	resource := req.Path + " → " + req.NewPath
	return s.mutate(ctx, id, resource, "node.file_rename", actor, func(files *sshx.Files) error {
		if strings.TrimSpace(req.NewPath) == "" {
			return errors.New("thiếu đường dẫn mới")
		}
		return files.Rename(req.Path, req.NewPath)
	})
}

// ChmodFile đổi quyền một mục trên máy chủ từ xa.
func (s *NodeService) ChmodFile(ctx context.Context, id uint, req NodeFileRequest, actor AuditEntry) error {
	resource := req.Path + " " + req.Mode
	return s.mutate(ctx, id, resource, "node.file_chmod", actor, func(files *sshx.Files) error {
		mode, err := parseMode(req.Mode)
		if err != nil {
			return err
		}
		return files.Chmod(req.Path, mode)
	})
}

// UploadFile chép một tệp lên máy chủ từ xa.
func (s *NodeService) UploadFile(
	ctx context.Context, id uint, dir, name string, source io.Reader, actor AuditEntry,
) (int64, error) {
	target := path.Join(dir, path.Base(name))

	files, done, err := s.files(ctx, id)
	if err != nil {
		return 0, err
	}
	defer done()

	written, err := files.Upload(ctx, target, source)

	actor.Action = "node.file_upload"
	actor.Resource = target
	actor.Success = err == nil
	s.audit.Record(ctx, actor)

	if err != nil {
		return 0, translateFileError(err)
	}
	return written, nil
}

// OpenFile mở một tệp trên máy chủ từ xa để tải về.
//
// Kết nối SFTP phải sống tới khi đọc xong, nên hàm đóng trả về cho bên gọi gọi
// sau khi đã gửi hết dữ liệu chứ không phải ngay khi hàm này trả về.
func (s *NodeService) OpenFile(ctx context.Context, id uint, target string) (io.ReadCloser, int64, func(), error) {
	files, done, err := s.files(ctx, id)
	if err != nil {
		return nil, 0, nil, err
	}

	reader, size, err := files.Download(target)
	if err != nil {
		done()
		return nil, 0, nil, translateFileError(err)
	}

	return reader, size, func() {
		_ = reader.Close()
		done()
	}, nil
}

// mutate chạy một thao tác thay đổi và ghi nhật ký kiểm toán.
func (s *NodeService) mutate(
	ctx context.Context, id uint, resource, action string, actor AuditEntry,
	run func(*sshx.Files) error,
) error {
	files, done, err := s.files(ctx, id)
	if err != nil {
		return err
	}
	defer done()

	err = run(files)

	actor.Action = action
	actor.Resource = resource
	actor.Success = err == nil
	s.audit.Record(ctx, actor)

	if err != nil {
		return translateFileError(err)
	}
	return nil
}

// parseMode đọc quyền dạng bát phân, ví dụ "0644".
func parseMode(value string) (os.FileMode, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, errors.New("thiếu quyền truy cập")
	}

	var mode uint32
	for _, char := range value {
		if char < '0' || char > '7' {
			return 0, errors.New("quyền phải là số bát phân, ví dụ 0644")
		}
		mode = mode*8 + uint32(char-'0')
	}
	if mode > 0o7777 {
		return 0, errors.New("quyền không hợp lệ")
	}
	return os.FileMode(mode), nil
}

// translateFileError đổi lỗi của lớp SFTP thành mã lỗi giao diện dịch được.
func translateFileError(err error) error {
	switch {
	case errors.Is(err, sshx.ErrTooLarge):
		return apperr.FileTooLarge
	case errors.Is(err, os.ErrNotExist):
		return apperr.FileNotFound
	case errors.Is(err, os.ErrPermission):
		return apperr.FilePermissionDenied
	default:
		return apperr.NodeFileFailed.WithParam("message", trimMessage(err.Error()))
	}
}
