package service

import (
	"context"
	"io"
	"os"
	"path"
	"strings"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/pkg/archive"
)

// stagingDir là thư mục tạm dùng khi triển khai mã nguồn.
//
// Đặt bên trong chính thư mục gốc của website: cùng phân vùng nên bước dời tệp
// vào chỗ là đổi tên tức thì, thay vì chép lại toàn bộ mã nguồn lần thứ hai.
// Tên bắt đầu bằng dấu chấm để nó không lọt vào thư mục web nếu có ai mở trang
// đúng lúc đang triển khai.
const stagingDir = ".sunpanel-deploy"

// uploadPrefix là tiền tố của tệp nén người dùng tải lên trong lúc triển khai.
const uploadPrefix = ".sunpanel-upload-"

// SourceRequest là yêu cầu triển khai mã nguồn cho một website.
type SourceRequest struct {
	// Path là tệp nén đã có sẵn trên máy chủ; bỏ trống khi tải tệp lên trực tiếp.
	Path string `json:"path"`
	// Clean xóa sạch thư mục gốc trước khi triển khai.
	Clean bool `json:"clean"`
	// KeepWrapper giữ nguyên thư mục cha duy nhất bên trong tệp nén.
	//
	// Mã nguồn tải từ GitHub luôn nằm trong một thư mục tên "duan-main", mà thứ
	// người dùng muốn là nội dung bên trong nó nằm ở gốc website chứ không phải
	// một cấp thư mục thừa khiến trang trả về 404.
	KeepWrapper bool `json:"keepWrapper"`
}

// Upload là tệp nén người dùng tải thẳng lên trong lúc triển khai.
type Upload struct {
	Name   string
	Reader io.Reader
}

// SourceResult là kết quả một lần triển khai mã nguồn.
type SourceResult struct {
	archive.Result
	// Root là thư mục gốc mã nguồn đã được đặt vào.
	Root string `json:"root"`
	// Wrapper là tên thư mục cha đã được bỏ đi, để trống nghĩa là không bỏ gì.
	Wrapper string `json:"wrapper"`
}

// DeploySource giải nén mã nguồn vào thư mục gốc của website.
//
// Đây là cách cài đặt của phần lớn mã nguồn PHP và trang tĩnh: người dùng có
// trong tay một tệp .zip hay .rar tải từ nhà phát hành, và việc còn lại đáng lẽ
// chỉ là đưa nó vào đúng chỗ — chứ không phải mở terminal, nhớ cú pháp unzip và
// đoán xem thư mục con nào mới là gốc trang.
func (s *WebsiteService) DeploySource(
	ctx context.Context, id uint, req SourceRequest, upload *Upload, actor AuditEntry,
) (SourceResult, error) {
	site, err := s.Get(ctx, id)
	if err != nil {
		return SourceResult{}, err
	}

	root := strings.TrimRight(site.Root, "/")
	if root == "" {
		return SourceResult{}, apperr.WebsiteNoRoot.WithParam("name", site.Name)
	}

	result, err := s.deploy(ctx, root, req, upload)

	actor.Action = "website.deploy_source"
	actor.Resource = site.Name
	actor.Success = err == nil
	s.audit.Record(ctx, actor)

	return result, err
}

func (s *WebsiteService) deploy(
	ctx context.Context, root string, req SourceRequest, upload *Upload,
) (SourceResult, error) {
	fsys := s.host.FS()
	if err := fsys.Mkdir(ctx, root, 0o755); err != nil {
		return SourceResult{}, translateFSError(err)
	}

	source := req.Path
	if upload != nil {
		staged, err := s.stageUpload(ctx, root, upload)
		if err != nil {
			return SourceResult{}, err
		}
		// Tệp tải lên chỉ để giải nén; giữ lại nó trong thư mục web nghĩa là phát
		// nguyên bản mã nguồn cho bất kỳ ai đoán đúng tên tệp.
		defer func() { _ = fsys.Remove(ctx, staged, false) }()
		source = staged
	}
	if source == "" {
		return SourceResult{}, apperr.BadRequest
	}

	staging := path.Join(root, stagingDir)
	if err := fsys.Remove(ctx, staging, true); err != nil && !isNotFound(err) {
		return SourceResult{}, translateFSError(err)
	}

	extracted, err := extractArchive(ctx, fsys, source, staging)
	if err != nil {
		_ = fsys.Remove(ctx, staging, true)
		return SourceResult{}, err
	}
	defer func() { _ = fsys.Remove(ctx, staging, true) }()

	from, wrapper, err := s.sourceRoot(ctx, staging, req.KeepWrapper)
	if err != nil {
		return SourceResult{}, err
	}

	if req.Clean {
		if err := s.clearRoot(ctx, root); err != nil {
			return SourceResult{}, err
		}
	}
	if err := s.moveInto(ctx, from, root); err != nil {
		return SourceResult{}, err
	}

	return SourceResult{Result: extracted, Root: root, Wrapper: wrapper}, nil
}

// stageUpload ghi tệp người dùng tải lên vào thư mục gốc để giải nén tại chỗ.
func (s *WebsiteService) stageUpload(ctx context.Context, root string, upload *Upload) (string, error) {
	name := path.Base(strings.ReplaceAll(upload.Name, "\\", "/"))
	if name == "" || name == "." || name == ".." || strings.Contains(name, "/") {
		return "", apperr.FileInvalidName
	}

	target := path.Join(root, uploadPrefix+name)
	if err := s.host.FS().Write(ctx, target, upload.Reader, 0o600); err != nil {
		return "", translateFSError(err)
	}
	return target, nil
}

// sourceRoot chọn thư mục thật sự chứa mã nguồn bên trong thư mục vừa giải nén.
//
// Gần như mọi bản tải về đều bọc mã nguồn trong đúng một thư mục mang tên dự án
// kèm số phiên bản. Dời nguyên thư mục đó vào gốc website sẽ cho ra một trang
// 404 và một người dùng không hiểu vì sao, nên panel bóc lớp bọc đó ra — trừ khi
// người dùng nói rõ là muốn giữ.
func (s *WebsiteService) sourceRoot(ctx context.Context, staging string, keep bool) (string, string, error) {
	if keep {
		return staging, "", nil
	}

	entries, err := s.host.FS().List(ctx, staging)
	if err != nil {
		return "", "", translateFSError(err)
	}
	if len(entries) != 1 || !entries[0].IsDir {
		return staging, "", nil
	}
	return path.Join(staging, entries[0].Name), entries[0].Name, nil
}

// clearRoot xóa sạch thư mục gốc, giữ lại thư mục tạm đang dùng để triển khai.
func (s *WebsiteService) clearRoot(ctx context.Context, root string) error {
	entries, err := s.host.FS().List(ctx, root)
	if err != nil {
		return translateFSError(err)
	}

	for _, entry := range entries {
		if entry.Name == stagingDir || strings.HasPrefix(entry.Name, uploadPrefix) {
			continue
		}
		if err := s.host.FS().Remove(ctx, path.Join(root, entry.Name), true); err != nil {
			return translateFSError(err)
		}
	}
	return nil
}

// moveInto dời từng mục con của from vào dir, ghi đè mục trùng tên.
func (s *WebsiteService) moveInto(ctx context.Context, from, dir string) error {
	entries, err := s.host.FS().List(ctx, from)
	if err != nil {
		return translateFSError(err)
	}

	for _, entry := range entries {
		target := path.Join(dir, entry.Name)

		// Đổi tên không ghi đè được lên thư mục đã có, nên xóa mục cũ trước. Người
		// dùng đã chọn triển khai bản mới; giữ lại bản cũ ở đây chỉ tạo ra một cây
		// thư mục lai giữa hai phiên bản.
		if err := s.host.FS().Remove(ctx, target, true); err != nil && !isNotFound(err) {
			return translateFSError(err)
		}
		if err := s.host.FS().Move(ctx, path.Join(from, entry.Name), target); err != nil {
			return translateFSError(err)
		}
	}
	return nil
}

// isNotFound nhận ra lỗi "không tồn tại" của lớp host.
//
// Xóa một thứ vốn không có mặt là kết quả đúng chứ không phải lỗi: bước dọn chỗ
// trước khi dời tệp vào không nên hỏng chỉ vì chỗ đó đang trống.
func isNotFound(err error) bool {
	return err != nil && (os.IsNotExist(err) || strings.Contains(err.Error(), "no such file"))
}
