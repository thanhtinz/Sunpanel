package service

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"path"
	"strings"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/pkg/archive"
	"github.com/thanhtinz/sunpanel/pkg/host"
)

// maxExtractSize giới hạn tổng dung lượng giải nén được, chống "zip bomb" —
// một tệp nén vài KB có thể phình thành hàng trăm GB và lấp đầy ổ đĩa.
const maxExtractSize = 2 << 30 // 2 GB

// maxInMemoryArchive là ngưỡng nạp cả tệp nén vào bộ nhớ.
//
// zip và 7z để bảng thư mục ở cuối tệp nên phải đọc ngẫu nhiên. Máy hiện tại
// cho ra thẳng tệp trên đĩa nên không cần nạp gì; ngưỡng này chỉ dùng cho host
// không mở được tệp kiểu đọc ngẫu nhiên, và giữ đủ thấp để một tệp lớn không
// nuốt hết RAM của máy chủ.
const maxInMemoryArchive = 256 << 20 // 256 MB

// ArchiveFormat là định dạng tệp nén, giữ lại làm bí danh để lớp trên không
// phải nhắc tới gói archive chỉ vì một tên kiểu.
type ArchiveFormat = archive.Format

// Các định dạng nén ra được.
const (
	FormatZip    = archive.FormatZip
	FormatTar    = archive.FormatTar
	FormatTarGz  = archive.FormatTarGz
	FormatTarXz  = archive.FormatTarXz
	FormatTarZst = archive.FormatTarZst
)

// ArchiveFormats liệt kê định dạng panel đọc được và định dạng nén ra được.
type ArchiveFormats struct {
	// Extract là các phần đuôi tên tệp mở được.
	Extract []string `json:"extract"`
	// Create là các định dạng nén ra được.
	Create []string `json:"create"`
}

// SupportedFormats cho giao diện biết panel làm được gì với tệp nén.
//
// Giao diện phải biết khi nào nên mời người dùng bấm "Giải nén", và danh sách đó
// phải sinh ra từ chính lớp nhận dạng chứ không chép tay sang JavaScript — chép
// tay là cách chắc chắn để thêm một định dạng ở backend rồi quên mất giao diện.
func SupportedFormats() ArchiveFormats {
	out := ArchiveFormats{Extract: archive.Extensions()}
	for _, format := range archive.Formats() {
		if format.CanCreate() {
			out.Create = append(out.Create, string(format))
		}
	}
	return out
}

// Compress nén danh sách mục vào tệp lưu trữ tại target.
func (s *FileService) Compress(ctx context.Context, sources []string, target string, format ArchiveFormat) error {
	if len(sources) == 0 {
		return apperr.BadRequest
	}
	if !format.CanCreate() {
		return apperr.FileUnsupportedFormat.WithParam("format", string(format))
	}

	// Ghi qua ống dẫn: bộ nén chạy ở goroutine riêng còn lớp host ghi thẳng ra
	// tệp, nên một thư mục lớn không phải nằm trọn trong bộ nhớ trước khi ghi.
	pipeReader, pipeWriter := io.Pipe()

	go func() {
		var err error
		if format == FormatZip {
			err = s.writeZip(ctx, pipeWriter, sources)
		} else {
			err = s.writeTar(ctx, pipeWriter, sources, format)
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
			// Create() không ghi quyền kiểu Unix vào tệp nén: tệp bung ra mất hết
			// quyền, và thư mục thì thành drw-rw-rw- — không ai vào được, kể cả
			// chủ sở hữu. CreateHeader kèm SetMode mới giữ được quyền thật.
			header := &zip.FileHeader{Name: archiveName(base, p), Modified: info.ModTime}
			if info.IsDir {
				header.Name += "/"
				header.SetMode(info.Mode | fs.ModeDir)
				// Thư mục không có nội dung để nén.
				header.Method = zip.Store
				_, err := zw.CreateHeader(header)
				return err
			}

			header.SetMode(info.Mode)
			header.Method = zip.Deflate
			entry, err := zw.CreateHeader(header)
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

// writeTar ghi tệp tar rồi đẩy qua bộ nén tương ứng với định dạng.
func (s *FileService) writeTar(ctx context.Context, w io.Writer, sources []string, format ArchiveFormat) error {
	compressor, err := archive.Compressor(w, format)
	if err != nil {
		return apperr.FileUnsupportedFormat.WithParam("format", string(format))
	}

	tw := tar.NewWriter(compressor)

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
	return compressor.Close()
}

// Extract giải nén một tệp lưu trữ vào thư mục đích.
//
// Định dạng được đoán từ tên tệp, và nếu tên không nói lên gì thì đoán tiếp từ
// vài byte đầu: một bản tải về đặt tên "source.download" vẫn là tệp zip, và
// bắt người dùng đổi đuôi tên tệp trước khi giải nén là bắt họ làm việc của máy.
func (s *FileService) Extract(ctx context.Context, archivePath, targetDir string) (archive.Result, error) {
	return extractArchive(ctx, s.host.FS(), archivePath, targetDir)
}

// extractArchive giải nén qua một hệ thống tệp bất kỳ.
//
// Tách khỏi FileService vì việc triển khai mã nguồn website cũng cần đúng logic
// này nhưng làm việc trên một phạm vi thư mục khác — chép lại nghĩa là sửa một
// lỗ hổng ở một chỗ rồi để nguyên ở chỗ kia.
func extractArchive(
	ctx context.Context, fsys host.FileSystem, archivePath, targetDir string,
) (archive.Result, error) {
	info, err := fsys.Stat(ctx, archivePath)
	if err != nil {
		return archive.Result{}, translateFSError(err)
	}

	format, err := detectFormat(ctx, fsys, archivePath, info.Name)
	if err != nil {
		return archive.Result{}, err
	}

	if err := fsys.Mkdir(ctx, targetDir, 0o755); err != nil {
		return archive.Result{}, translateFSError(err)
	}

	reader, err := fsys.Open(ctx, archivePath)
	if err != nil {
		return archive.Result{}, translateFSError(err)
	}
	defer func() { _ = reader.Close() }()

	source, err := randomAccess(reader, format, info.Size)
	if err != nil {
		return archive.Result{}, err
	}

	options := archive.Options{Format: format, Name: info.Name, MaxBytes: maxExtractSize}
	result, err := archive.Extract(ctx, source, info.Size, options, &hostSink{fs: fsys, dir: targetDir})
	return result, translateArchiveError(err)
}

// detectFormat đoán định dạng từ tên tệp, rồi từ chữ ký đầu tệp nếu tên im lặng.
func detectFormat(
	ctx context.Context, fsys host.FileSystem, archivePath, name string,
) (archive.Format, error) {
	if format, ok := archive.DetectName(name); ok {
		return format, nil
	}

	reader, err := fsys.Open(ctx, archivePath)
	if err != nil {
		return "", translateFSError(err)
	}
	defer func() { _ = reader.Close() }()

	head := make([]byte, archive.MagicSize)
	n, err := io.ReadFull(reader, head)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return "", apperr.Internal.Wrap(err)
	}

	format, ok := archive.DetectMagic(head[:n])
	if !ok {
		return "", apperr.FileUnsupportedFormat.WithParam("format", path.Ext(name))
	}
	return format, nil
}

// randomAccess cấp nguồn đọc ngẫu nhiên cho zip và 7z.
//
// Máy hiện tại cho ra tệp thật nên nó đã đọc ngẫu nhiên được; nhánh nạp vào bộ
// nhớ chỉ dành cho host trả về luồng tuần tự — bản đa node sau này.
func randomAccess(reader io.Reader, format archive.Format, size int64) (io.Reader, error) {
	if !format.NeedsRandomAccess() {
		return reader, nil
	}
	if _, ok := reader.(io.ReaderAt); ok {
		return reader, nil
	}
	if size > maxInMemoryArchive {
		return nil, apperr.FileTooLarge.WithParam("max", maxInMemoryArchive)
	}

	data, err := io.ReadAll(io.LimitReader(reader, maxInMemoryArchive))
	if err != nil {
		return nil, apperr.Internal.Wrap(err)
	}
	return bytes.NewReader(data), nil
}

// hostSink ghi từng mục giải nén ra qua lớp host.
type hostSink struct {
	fs  host.FileSystem
	dir string
}

func (h *hostSink) Dir(ctx context.Context, name string, mode fs.FileMode) error {
	return translateFSError(h.fs.Mkdir(ctx, path.Join(h.dir, name), mode))
}

func (h *hostSink) File(ctx context.Context, name string, mode fs.FileMode, r io.Reader) error {
	target := path.Join(h.dir, name)
	if err := h.fs.Mkdir(ctx, path.Dir(target), 0o755); err != nil {
		return translateFSError(err)
	}
	return translateFSError(h.fs.Write(ctx, target, r, mode))
}

// translateArchiveError đổi lỗi của gói archive sang mã lỗi giao diện dịch được.
func translateArchiveError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, archive.ErrUnsupported):
		return apperr.FileUnsupportedFormat
	case errors.Is(err, archive.ErrUnsafePath):
		return apperr.FileUnsafeArchive
	case errors.Is(err, archive.ErrTooLarge):
		return apperr.FileTooLarge.WithParam("max", maxExtractSize)
	case errors.Is(err, archive.ErrEncrypted):
		return apperr.FileEncryptedArchive
	case errors.Is(err, archive.ErrNeedRandomAccess):
		return apperr.FileTooLarge.WithParam("max", maxInMemoryArchive)
	case errors.Is(err, archive.ErrCorrupt):
		return apperr.FileCorruptArchive.Wrap(err)
	}

	// Lỗi từ lớp host (hết đĩa, không có quyền) đã mang sẵn mã lỗi của panel.
	var appErr *apperr.Error
	if errors.As(err, &appErr) {
		return err
	}
	return apperr.Internal.Wrap(err)
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
