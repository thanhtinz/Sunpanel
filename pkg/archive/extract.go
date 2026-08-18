package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"io/fs"
	"path"
	"strings"

	"github.com/bodgit/sevenzip"
	"github.com/klauspost/compress/zstd"
	"github.com/nwaples/rardecode/v2"
	"github.com/ulikunitz/xz"
)

// Sink nhận từng mục được giải nén ra.
//
// Gói này không tự ghi xuống đĩa: panel luôn ghi tệp qua lớp host để đường dẫn
// bị chặn trong phạm vi cho phép và để phiên bản đa node sau này ghi được lên
// máy khác mà không phải sửa gì ở đây.
type Sink interface {
	Dir(ctx context.Context, name string, mode fs.FileMode) error
	File(ctx context.Context, name string, mode fs.FileMode, r io.Reader) error
}

// Options điều khiển một lần giải nén.
type Options struct {
	// Format là định dạng đã nhận biết được của nguồn.
	Format Format
	// Name là tên tệp nén, dùng để đặt tên nội dung của định dạng một-tệp.
	Name string
	// MaxBytes giới hạn tổng dung lượng ghi ra; 0 nghĩa là không giới hạn.
	MaxBytes int64
}

// Result tóm tắt kết quả một lần giải nén.
type Result struct {
	Files int   `json:"files"`
	Dirs  int   `json:"dirs"`
	Bytes int64 `json:"bytes"`
	// Skipped là số mục bị bỏ qua vì không phải tệp hay thư mục thường —
	// liên kết mềm, thiết bị, socket.
	Skipped int `json:"skipped"`
}

// defaultDirMode là quyền dùng cho thư mục mà tệp nén không nói rõ quyền.
const defaultDirMode fs.FileMode = 0o755

// defaultFileMode là quyền dùng cho tệp mà tệp nén không nói rõ quyền.
//
// Nhiều tệp zip tạo trên Windows không mang quyền kiểu Unix, và mode 0 sẽ tạo
// ra tệp không ai đọc được kể cả chủ sở hữu.
const defaultFileMode fs.FileMode = 0o644

// Extract giải nén src vào sink.
//
// size chỉ cần đúng với định dạng phải đọc ngẫu nhiên (zip, 7z); các định dạng
// còn lại đọc tuần tự nên bỏ qua tham số này.
func Extract(ctx context.Context, src io.Reader, size int64, opts Options, sink Sink) (Result, error) {
	writer := &guard{sink: sink, max: opts.MaxBytes}

	switch opts.Format {
	case FormatZip:
		return writer.result, extractZip(ctx, src, size, writer)
	case FormatSevenZip:
		return writer.result, extractSevenZip(ctx, src, size, writer)
	case FormatRar:
		return writer.result, extractRar(ctx, src, writer)
	case FormatTar:
		return writer.result, extractTar(ctx, src, writer)
	case FormatTarGz, FormatTarBz2, FormatTarXz, FormatTarZst,
		FormatGz, FormatBz2, FormatXz, FormatZst:
		return writer.result, extractCompressed(ctx, src, opts, writer)
	default:
		return Result{}, ErrUnsupported
	}
}

// guard ghi qua Sink và canh tổng dung lượng cùng tính an toàn của tên mục.
//
// Mọi định dạng đều đi qua đây, nên chỉ có đúng một chỗ quyết định tên nào được
// ghi ra — thay vì lặp lại phép kiểm tra ở sáu bộ giải nén và quên mất một chỗ.
type guard struct {
	sink   Sink
	max    int64
	result Result
}

func (g *guard) dir(ctx context.Context, name string, mode fs.FileMode) error {
	clean, err := sanitize(name)
	if err != nil || clean == "" {
		return err
	}
	if mode.Perm() == 0 {
		mode = defaultDirMode
	}
	if err := g.sink.Dir(ctx, clean, mode.Perm()); err != nil {
		return err
	}
	g.result.Dirs++
	return nil
}

func (g *guard) file(ctx context.Context, name string, mode fs.FileMode, declared int64, r io.Reader) error {
	clean, err := sanitize(name)
	if err != nil || clean == "" {
		return err
	}

	// Kích thước khai báo trong tệp nén do người tạo tệp ghi ra, nên nó có thể là
	// con số dối. Kiểm nó trước để từ chối sớm, rồi vẫn đếm số byte thật khi ghi.
	if g.max > 0 && (declared > g.max || g.result.Bytes+declared > g.max) {
		return ErrTooLarge
	}

	counter := &counting{reader: r}
	if g.max > 0 {
		counter.reader = io.LimitReader(r, g.max-g.result.Bytes+1)
	}

	if mode.Perm() == 0 {
		mode = defaultFileMode
	}
	if err := g.sink.File(ctx, clean, mode.Perm(), counter); err != nil {
		return err
	}

	g.result.Bytes += counter.n
	g.result.Files++
	if g.max > 0 && g.result.Bytes > g.max {
		return ErrTooLarge
	}
	return nil
}

type counting struct {
	reader io.Reader
	n      int64
}

func (c *counting) Read(p []byte) (int, error) {
	n, err := c.reader.Read(p)
	c.n += int64(n)
	return n, err
}

// sanitize biến tên bên trong tệp nén thành đường dẫn tương đối an toàn.
//
// Đây là lỗ hổng kinh điển của mọi trình giải nén: một mục tên "../../etc/cron.d/x"
// ghi đè tệp hệ thống ngay khi người dùng bấm giải nén. Tên tuyệt đối, tên chứa
// ".." và tên kiểu Windows "C:\..." đều bị từ chối chứ không âm thầm cắt bớt —
// cắt bớt thì tệp vẫn ra, chỉ là ở chỗ khác với chỗ người tạo tệp định nhắm tới.
func sanitize(name string) (string, error) {
	name = strings.ReplaceAll(name, "\\", "/")
	name = strings.TrimPrefix(name, "./")

	if name == "" || name == "." {
		return "", nil
	}
	if strings.HasPrefix(name, "/") || strings.Contains(name, ":") {
		return "", ErrUnsafePath
	}

	clean := path.Clean(name)
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", ErrUnsafePath
	}
	return strings.TrimSuffix(clean, "/"), nil
}

func extractZip(ctx context.Context, src io.Reader, size int64, out *guard) error {
	reader, ok := src.(io.ReaderAt)
	if !ok {
		return ErrNeedRandomAccess
	}

	zr, err := zip.NewReader(reader, size)
	if err != nil {
		return errors.Join(ErrCorrupt, err)
	}

	for _, entry := range zr.File {
		if err := ctx.Err(); err != nil {
			return err
		}

		info := entry.FileInfo()
		switch {
		case info.IsDir():
			if err := out.dir(ctx, entry.Name, info.Mode()); err != nil {
				return err
			}
			continue
		case !info.Mode().IsRegular():
			out.result.Skipped++
			continue
		case entry.Flags&0x1 != 0:
			return ErrEncrypted
		}

		if err := copyZipEntry(ctx, entry, info, out); err != nil {
			return err
		}
	}
	return nil
}

func copyZipEntry(ctx context.Context, entry *zip.File, info fs.FileInfo, out *guard) error {
	rc, err := entry.Open()
	if err != nil {
		return errors.Join(ErrCorrupt, err)
	}
	defer func() { _ = rc.Close() }()

	return out.file(ctx, entry.Name, info.Mode(), info.Size(), rc)
}

func extractSevenZip(ctx context.Context, src io.Reader, size int64, out *guard) error {
	reader, ok := src.(io.ReaderAt)
	if !ok {
		return ErrNeedRandomAccess
	}

	sz, err := sevenzip.NewReader(reader, size)
	if err != nil {
		return errors.Join(ErrCorrupt, err)
	}

	for _, entry := range sz.File {
		if err := ctx.Err(); err != nil {
			return err
		}

		info := entry.FileInfo()
		switch {
		case info.IsDir():
			if err := out.dir(ctx, entry.Name, info.Mode()); err != nil {
				return err
			}
			continue
		case !info.Mode().IsRegular():
			out.result.Skipped++
			continue
		}

		if err := copySevenZipEntry(ctx, entry, info, out); err != nil {
			return err
		}
	}
	return nil
}

func copySevenZipEntry(ctx context.Context, entry *sevenzip.File, info fs.FileInfo, out *guard) error {
	rc, err := entry.Open()
	if err != nil {
		// Kho 7z đặt mật khẩu chỉ lộ ra lúc mở mục đầu tiên, không phải lúc mở kho.
		if strings.Contains(strings.ToLower(err.Error()), "password") {
			return ErrEncrypted
		}
		return errors.Join(ErrCorrupt, err)
	}
	defer func() { _ = rc.Close() }()

	return out.file(ctx, entry.Name, info.Mode(), info.Size(), rc)
}

func extractRar(ctx context.Context, src io.Reader, out *guard) error {
	rr, err := rardecode.NewReader(src)
	if err != nil {
		return translateRarError(err)
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		header, err := rr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return translateRarError(err)
		}

		if header.Encrypted || header.HeaderEncrypted {
			return ErrEncrypted
		}
		if header.IsDir {
			if err := out.dir(ctx, header.Name, header.Mode()); err != nil {
				return err
			}
			continue
		}

		// Bản RAR ghi nhiều tập không nói trước kích thước; đưa 0 để bỏ qua phép
		// kiểm tra khai báo, số byte thật vẫn được đếm khi ghi.
		declared := header.UnPackedSize
		if header.UnKnownSize {
			declared = 0
		}
		if err := out.file(ctx, header.Name, header.Mode(), declared, rr); err != nil {
			return err
		}
	}
}

func translateRarError(err error) error {
	switch {
	case errors.Is(err, rardecode.ErrArchiveEncrypted), errors.Is(err, rardecode.ErrArchivedFileEncrypted):
		return ErrEncrypted
	case err == nil:
		return nil
	default:
		return errors.Join(ErrCorrupt, err)
	}
}

func extractTar(ctx context.Context, src io.Reader, out *guard) error {
	tr := tar.NewReader(src)

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return errors.Join(ErrCorrupt, err)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := out.dir(ctx, header.Name, header.FileInfo().Mode()); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := out.file(ctx, header.Name, header.FileInfo().Mode(), header.Size, tr); err != nil {
				return err
			}
		default:
			// Liên kết mềm, liên kết cứng, thiết bị: bỏ qua. Một liên kết mềm trỏ
			// ra ngoài thư mục đích là đường vòng để ghi đè tệp hệ thống sau này.
			out.result.Skipped++
		}
	}
}

// extractCompressed mở lớp nén rồi quyết định bên trong là tar hay một tệp đơn.
//
// Người dùng không phân biệt "tệp .gz" với "tệp .tar.gz đặt tên thiếu", và cả
// hai đều tồn tại thật. Thay vì tin vào tên tệp, đọc thử đầu luồng: nhận ra
// khuôn tar thì giải nén cả cây, không thì ghi ra đúng một tệp.
func extractCompressed(ctx context.Context, src io.Reader, opts Options, out *guard) error {
	reader, closer, err := decompress(src, opts.Format)
	if err != nil {
		return err
	}
	if closer != nil {
		defer closer()
	}

	head := make([]byte, 512)
	n, err := io.ReadFull(reader, head)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return errors.Join(ErrCorrupt, err)
	}
	head = head[:n]
	joined := io.MultiReader(strings.NewReader(string(head)), reader)

	if looksLikeTar(head) {
		return extractTar(ctx, joined, out)
	}

	name := path.Base(strings.ReplaceAll(opts.Name, "\\", "/"))
	name = TrimExtension(name)
	if name == "" || name == "." {
		name = "data"
	}
	return out.file(ctx, name, defaultFileMode, 0, joined)
}

// looksLikeTar nhận ra khuôn tar qua chữ "ustar" ở byte thứ 257.
//
// Tệp tar rất cũ (định dạng v7) không có chữ này; với chúng panel sẽ ghi ra một
// tệp .tar để người dùng tự mở tiếp, thay vì đoán mò rồi cho ra một cây thư mục
// rác từ dữ liệu không phải tar.
func looksLikeTar(head []byte) bool {
	return len(head) >= 262 && (string(head[257:262]) == "ustar")
}

func decompress(src io.Reader, format Format) (io.Reader, func(), error) {
	switch format {
	case FormatTarGz, FormatGz:
		gz, err := gzip.NewReader(src)
		if err != nil {
			return nil, nil, errors.Join(ErrCorrupt, err)
		}
		return gz, func() { _ = gz.Close() }, nil
	case FormatTarBz2, FormatBz2:
		return bzip2.NewReader(src), nil, nil
	case FormatTarXz, FormatXz:
		xr, err := xz.NewReader(src)
		if err != nil {
			return nil, nil, errors.Join(ErrCorrupt, err)
		}
		return xr, nil, nil
	case FormatTarZst, FormatZst:
		zr, err := zstd.NewReader(src)
		if err != nil {
			return nil, nil, errors.Join(ErrCorrupt, err)
		}
		return zr, zr.Close, nil
	default:
		return nil, nil, ErrUnsupported
	}
}

// Compressor bọc w bằng bộ nén tương ứng với định dạng.
//
// Trả về cả với FormatTar (không nén) để bên gọi chỉ có một nhánh mã: đóng cái
// nhận được là xong, không phải nhớ định dạng nào cần đóng định dạng nào không.
func Compressor(w io.Writer, format Format) (io.WriteCloser, error) {
	switch format {
	case FormatTar:
		return nopCloser{w}, nil
	case FormatTarGz:
		return gzip.NewWriter(w), nil
	case FormatTarXz:
		xw, err := xz.NewWriter(w)
		if err != nil {
			return nil, err
		}
		return xw, nil
	case FormatTarZst:
		return zstd.NewWriter(w)
	default:
		return nil, ErrUnsupported
	}
}

type nopCloser struct{ io.Writer }

func (nopCloser) Close() error { return nil }
