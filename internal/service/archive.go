package service

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"errors"

	"io"
	"os"
	"path"
	"strings"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/pkg/host"
)

// maxExtractSize giới hạn tổng dung lượng giải nén được, chống "zip bomb" —
// một tệp nén vài KB có thể phình thành hàng trăm GB và lấp đầy ổ đĩa.
const maxExtractSize = 2 << 30 // 2 GB

// ArchiveFormat là định dạng tệp nén được hỗ trợ.
type ArchiveFormat string

// Các định dạng nén được hỗ trợ.
const (
	FormatZip   ArchiveFormat = "zip"
	FormatTarGz ArchiveFormat = "tar.gz"
)

// Compress nén danh sách mục vào tệp lưu trữ tại target.
func (s *FileService) Compress(ctx context.Context, sources []string, target string, format ArchiveFormat) error {
	if len(sources) == 0 {
		return apperr.BadRequest
	}

	// Ghi ra tệp tạm trong cùng thư mục rồi mới đổi tên, để việc nén bị gián đoạn
	// không để lại một tệp lưu trữ hỏng mà người dùng tưởng là dùng được.
	pipeReader, pipeWriter := io.Pipe()

	go func() {
		var err error
		switch format {
		case FormatZip:
			err = s.writeZip(ctx, pipeWriter, sources)
		case FormatTarGz:
			err = s.writeTarGz(ctx, pipeWriter, sources)
		default:
			err = apperr.FileUnsupportedFormat
		}
		_ = pipeWriter.CloseWithError(err)
	}()

	if err := s.host.FS().Write(ctx, target, pipeReader, 0o644); err != nil {
		_ = pipeReader.CloseWithError(err)
		return translateFSError(err)
	}
	return nil
}

func (s *FileService) writeZip(ctx context.Context, w io.Writer, sources []string) error {
	zw := zip.NewWriter(w)

	for _, source := range sources {
		base := path.Dir(normalizePath(source))
		err := s.walk(ctx, source, func(p string, info host.FileInfo) error {
			name := archiveName(base, p)
			if info.IsDir {
				_, err := zw.Create(name + "/")
				return err
			}

			entry, err := zw.Create(name)
			if err != nil {
				return err
			}
			return s.copyFileInto(ctx, entry, p)
		})
		if err != nil {
			return err
		}
	}

	return zw.Close()
}

func (s *FileService) writeTarGz(ctx context.Context, w io.Writer, sources []string) error {
	gz := gzip.NewWriter(w)
	tw := tar.NewWriter(gz)

	for _, source := range sources {
		base := path.Dir(normalizePath(source))
		err := s.walk(ctx, source, func(p string, info host.FileInfo) error {
			header := &tar.Header{
				Name:    archiveName(base, p),
				Mode:    int64(info.Mode),
				ModTime: info.ModTime,
			}
			if info.IsDir {
				header.Typeflag, header.Name = tar.TypeDir, header.Name+"/"
			} else {
				header.Typeflag, header.Size = tar.TypeReg, info.Size
			}

			if err := tw.WriteHeader(header); err != nil {
				return err
			}
			if info.IsDir {
				return nil
			}
			return s.copyFileInto(ctx, tw, p)
		})
		if err != nil {
			return err
		}
	}

	if err := tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}

// Extract giải nén một tệp lưu trữ vào thư mục đích.
func (s *FileService) Extract(ctx context.Context, archivePath, targetDir string) error {
	info, err := s.host.FS().Stat(ctx, archivePath)
	if err != nil {
		return translateFSError(err)
	}

	if err := s.host.FS().Mkdir(ctx, targetDir, 0o755); err != nil {
		return translateFSError(err)
	}

	switch {
	case strings.HasSuffix(strings.ToLower(info.Name), ".zip"):
		return s.extractZip(ctx, archivePath, targetDir, info.Size)
	case hasAnySuffix(strings.ToLower(info.Name), ".tar.gz", ".tgz"):
		return s.extractTarGz(ctx, archivePath, targetDir)
	default:
		return apperr.FileUnsupportedFormat
	}
}

func (s *FileService) extractZip(ctx context.Context, archivePath, targetDir string, size int64) error {
	// archive/zip cần đọc ngẫu nhiên (bảng thư mục nằm ở cuối tệp), nên phải nạp
	// vào bộ nhớ thay vì đọc tuần tự như tar.
	if size > maxExtractSize {
		return apperr.FileTooLarge.WithParam("max", maxExtractSize)
	}

	reader, err := s.host.FS().Open(ctx, archivePath)
	if err != nil {
		return translateFSError(err)
	}
	defer func() { _ = reader.Close() }()

	data, err := io.ReadAll(reader)
	if err != nil {
		return apperr.Internal.Wrap(err)
	}

	zr, err := zip.NewReader(strings.NewReader(string(data)), int64(len(data)))
	if err != nil {
		return apperr.FileCorruptArchive.Wrap(err)
	}

	var written int64
	for _, entry := range zr.File {
		target, err := safeArchivePath(targetDir, entry.Name)
		if err != nil {
			return err
		}

		if entry.FileInfo().IsDir() {
			if err := s.host.FS().Mkdir(ctx, target, 0o755); err != nil {
				return translateFSError(err)
			}
			continue
		}

		// Kích thước khai báo trong tệp nén là do người tạo tệp ghi, có thể là con
		// số dối để làm tràn phép cộng bên dưới. Chặn ngay ở đây.
		if entry.UncompressedSize64 > maxExtractSize {
			return apperr.FileTooLarge.WithParam("max", maxExtractSize)
		}
		written += int64(entry.UncompressedSize64) // #nosec G115 -- đã chặn > maxExtractSize ngay trên
		if written > maxExtractSize {
			return apperr.FileTooLarge.WithParam("max", maxExtractSize)
		}

		if err := s.extractOne(ctx, entry, target); err != nil {
			return err
		}
	}
	return nil
}

func (s *FileService) extractOne(ctx context.Context, entry *zip.File, target string) error {
	rc, err := entry.Open()
	if err != nil {
		return apperr.FileCorruptArchive.Wrap(err)
	}
	defer func() { _ = rc.Close() }()

	if err := s.host.FS().Mkdir(ctx, path.Dir(target), 0o755); err != nil {
		return translateFSError(err)
	}
	if err := s.host.FS().Write(ctx, target, io.LimitReader(rc, maxExtractSize), entry.Mode()); err != nil {
		return translateFSError(err)
	}
	return nil
}

func (s *FileService) extractTarGz(ctx context.Context, archivePath, targetDir string) error {
	reader, err := s.host.FS().Open(ctx, archivePath)
	if err != nil {
		return translateFSError(err)
	}
	defer func() { _ = reader.Close() }()

	gz, err := gzip.NewReader(reader)
	if err != nil {
		return apperr.FileCorruptArchive.Wrap(err)
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	var written int64

	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return apperr.FileCorruptArchive.Wrap(err)
		}

		target, err := safeArchivePath(targetDir, header.Name)
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := s.host.FS().Mkdir(ctx, target, archiveMode(header.Mode)); err != nil {
				return translateFSError(err)
			}
		case tar.TypeReg:
			written += header.Size
			if written > maxExtractSize {
				return apperr.FileTooLarge.WithParam("max", maxExtractSize)
			}
			if err := s.host.FS().Mkdir(ctx, path.Dir(target), 0o755); err != nil {
				return translateFSError(err)
			}
			err := s.host.FS().Write(ctx, target, io.LimitReader(tr, maxExtractSize), archiveMode(header.Mode))
			if err != nil {
				return translateFSError(err)
			}
		default:
			// Bỏ qua symlink, thiết bị, socket: giải nén chúng từ một tệp lưu trữ
			// không đáng tin là con đường thoát khỏi thư mục đích.
			continue
		}
	}
}

// walk duyệt đệ quy một mục, gọi fn cho chính nó và mọi mục con.
func (s *FileService) walk(ctx context.Context, p string, fn func(string, host.FileInfo) error) error {
	info, err := s.host.FS().Stat(ctx, p)
	if err != nil {
		return translateFSError(err)
	}
	if err := fn(normalizePath(p), info); err != nil {
		return err
	}
	if !info.IsDir {
		return nil
	}

	children, err := s.host.FS().List(ctx, p)
	if err != nil {
		return translateFSError(err)
	}
	for _, child := range children {
		// Không đi theo symlink khi nén: một liên kết trỏ vòng lại thư mục cha sẽ
		// tạo vòng lặp vô tận.
		if child.IsLink {
			continue
		}
		if err := s.walk(ctx, path.Join(normalizePath(p), child.Name), fn); err != nil {
			return err
		}
	}
	return nil
}

func (s *FileService) copyFileInto(ctx context.Context, w io.Writer, p string) error {
	reader, err := s.host.FS().Open(ctx, p)
	if err != nil {
		return translateFSError(err)
	}
	defer func() { _ = reader.Close() }()

	_, err = io.Copy(w, reader)
	return err
}

// archiveName tính đường dẫn tương đối của một mục bên trong tệp lưu trữ.
func archiveName(base, p string) string {
	name := strings.TrimPrefix(p, base)
	return strings.TrimPrefix(name, "/")
}

// safeArchivePath chặn tấn công "zip slip": một tệp lưu trữ độc hại chứa mục tên
// "../../etc/cron.d/x" sẽ ghi ra ngoài thư mục đích nếu ghép đường dẫn ngây thơ.
//
// Việc kiểm tra phải làm trên tên GỐC, trước khi làm sạch. path.Clean sẽ nuốt hết
// các thành phần ".." và biến tên độc hại thành một tên trông vô hại nằm trong
// thư mục đích — an toàn về mặt ghi đè, nhưng che giấu mất việc tệp nén đã bị can
// thiệp. Người dùng cần biết điều đó thay vì lặng lẽ nhận về một tệp bị đổi chỗ.
func safeArchivePath(targetDir, name string) (string, error) {
	normalized := strings.ReplaceAll(name, "\\", "/")

	for _, segment := range strings.Split(normalized, "/") {
		if segment == ".." {
			return "", apperr.FileUnsafeArchive.WithParam("entry", name)
		}
	}

	// Đường dẫn tuyệt đối trong tệp lưu trữ là hợp lệ (tar thường lưu như vậy);
	// nó được hiểu là tương đối so với thư mục đích.
	clean := path.Clean("/" + normalized)
	if clean == "/" {
		return "", apperr.FileInvalidName
	}

	return path.Join(normalizePath(targetDir), clean), nil
}

// archiveMode chuyển trường quyền trong tệp lưu trữ thành os.FileMode.
//
// Trường này là int64 do người tạo tệp nén ghi, nên có thể chứa giá trị vô nghĩa.
// Chỉ giữ lại 12 bit quyền hợp lệ và bỏ mọi bit khác — trong đó có cả setuid và
// setgid, vốn không nên được khôi phục từ một tệp nén không đáng tin.
func archiveMode(mode int64) os.FileMode {
	return os.FileMode(mode & 0o777) // #nosec G115 -- phép AND kẹp giá trị xuống tối đa 0o777
}

func hasAnySuffix(s string, suffixes ...string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(s, suffix) {
			return true
		}
	}
	return false
}
