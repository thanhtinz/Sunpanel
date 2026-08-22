package service

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/pkg/host"
)

func newTestFileService(t *testing.T) (*FileService, string) {
	t.Helper()

	root := t.TempDir()
	h := host.NewLocalHost(root, nil)
	return NewFileService(h, NewAuditService(newMemoryDB(t))), root
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("tạo thư mục: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("ghi tệp: %v", err)
	}
}

func TestFileListAndRead(t *testing.T) {
	svc, root := newTestFileService(t)
	writeFile(t, filepath.Join(root, "www", "index.html"), "<h1>xin chào</h1>")
	ctx := context.Background()

	result, err := svc.List(ctx, "/www")
	if err != nil {
		t.Fatalf("liệt kê: %v", err)
	}
	if result.Total != 1 || result.Items[0].Name != "index.html" {
		t.Fatalf("nội dung thư mục không đúng: %+v", result.Items)
	}
	if result.Parent != "/" {
		t.Errorf("thư mục cha = %q, mong đợi \"/\"", result.Parent)
	}

	content, err := svc.Read(ctx, "/www/index.html")
	if err != nil {
		t.Fatalf("đọc tệp: %v", err)
	}
	if content.Content != "<h1>xin chào</h1>" {
		t.Errorf("nội dung = %q", content.Content)
	}
}

// Đường dẫn vượt ra ngoài gốc phải bị chặn ở mọi điểm vào của service.
func TestFileOperationsCannotEscapeRoot(t *testing.T) {
	svc, root := newTestFileService(t)
	ctx := context.Background()

	outside := filepath.Join(filepath.Dir(root), "ngoai-pham-vi.txt")
	writeFile(t, outside, "bí mật")
	t.Cleanup(func() { os.Remove(outside) })

	escape := "../" + filepath.Base(outside)

	if _, err := svc.Read(ctx, escape); err == nil {
		t.Error("Read đọc được tệp ngoài phạm vi gốc")
	}
	if err := svc.Write(ctx, escape, "đã bị ghi đè"); err == nil {
		if data, readErr := os.ReadFile(outside); readErr == nil && string(data) != "bí mật" {
			t.Fatal("Write đã ghi đè tệp nằm ngoài phạm vi gốc")
		}
	}
	if err := svc.Remove(ctx, []string{escape}); err == nil {
		if _, statErr := os.Stat(outside); os.IsNotExist(statErr) {
			t.Fatal("Remove đã xóa tệp nằm ngoài phạm vi gốc")
		}
	}
}

// Tên tệp tải lên chứa thành phần đường dẫn phải bị từ chối, nếu không một tên
// như "../../etc/cron.d/x" sẽ ghi ra ngoài thư mục đích.
func TestUploadRejectsPathInName(t *testing.T) {
	svc, root := newTestFileService(t)
	ctx := context.Background()

	for _, name := range []string{"..", ".", "/"} {
		t.Run("tên "+name, func(t *testing.T) {
			err := svc.Upload(ctx, "/", name, strings.NewReader("x"))
			if !errors.Is(err, apperr.FileInvalidName) {
				t.Fatalf("lỗi = %v, mong đợi FileInvalidName", err)
			}
		})
	}

	// Tên có thành phần đường dẫn được rút gọn về tên cơ sở, không leo thư mục.
	if err := svc.Upload(ctx, "/", "../../thoat.txt", strings.NewReader("nội dung")); err != nil {
		t.Fatalf("tải lên: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "thoat.txt")); err != nil {
		t.Errorf("tệp phải nằm trong gốc: %v", err)
	}
}

func TestReadRejectsBinaryFile(t *testing.T) {
	svc, root := newTestFileService(t)
	writeFile(t, filepath.Join(root, "anh.png"), "PNG\x00\x01\x02nội dung nhị phân")

	_, err := svc.Read(context.Background(), "/anh.png")
	if !errors.Is(err, apperr.FileNotText) {
		t.Fatalf("lỗi = %v, mong đợi FileNotText", err)
	}
}

// Văn bản tiếng Việt có dấu là UTF-8 nhiều byte — không được nhận nhầm là nhị phân.
func TestReadAcceptsVietnameseText(t *testing.T) {
	svc, root := newTestFileService(t)
	const content = "Xin chào — đây là tệp cấu hình tiếng Việt có dấu đầy đủ: ăâêôơưđ"
	writeFile(t, filepath.Join(root, "cauhinh.txt"), content)

	got, err := svc.Read(context.Background(), "/cauhinh.txt")
	if err != nil {
		t.Fatalf("đọc tệp: %v", err)
	}
	if got.Content != content {
		t.Errorf("nội dung = %q", got.Content)
	}
}

func TestChmod(t *testing.T) {
	svc, root := newTestFileService(t)
	writeFile(t, filepath.Join(root, "script.sh"), "#!/bin/sh\n")
	ctx := context.Background()

	if err := svc.Chmod(ctx, "/script.sh", "0755"); err != nil {
		t.Fatalf("đổi quyền: %v", err)
	}
	info, err := os.Stat(filepath.Join(root, "script.sh"))
	if err != nil {
		t.Fatalf("đọc thông tin tệp: %v", err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("quyền = %o, mong đợi 755", info.Mode().Perm())
	}

	for _, invalid := range []string{"999", "abc", "77777"} {
		if err := svc.Chmod(ctx, "/script.sh", invalid); !errors.Is(err, apperr.FileInvalidMode) {
			t.Errorf("chuỗi quyền %q: lỗi = %v, mong đợi FileInvalidMode", invalid, err)
		}
	}
}

func TestZipGiuQuyenVaThuMucVaoDuoc(t *testing.T) {
	svc, root := newTestFileService(t)
	writeFile(t, filepath.Join(root, "duan", "con", "b.txt"), "tệp B")
	if err := os.Chmod(filepath.Join(root, "duan", "con"), 0o750); err != nil {
		t.Fatalf("đặt quyền: %v", err)
	}
	if err := os.Chmod(filepath.Join(root, "duan", "con", "b.txt"), 0o640); err != nil {
		t.Fatalf("đặt quyền: %v", err)
	}
	ctx := context.Background()

	if err := svc.Compress(ctx, []string{"/duan"}, "/luu-tru.zip", FormatZip); err != nil {
		t.Fatalf("nén: %v", err)
	}

	// Đọc thẳng tệp nén: quyền phải nằm trong đó, chứ không phải chỉ đúng nhờ
	// giá trị mặc định lúc giải nén.
	reader, err := zip.OpenReader(filepath.Join(root, "luu-tru.zip"))
	if err != nil {
		t.Fatalf("mở tệp nén: %v", err)
	}
	defer reader.Close()

	found := map[string]fs.FileMode{}
	for _, entry := range reader.File {
		found[entry.Name] = entry.Mode().Perm()
		if strings.HasSuffix(entry.Name, "/") && entry.Mode().Perm()&0o100 == 0 {
			t.Errorf("thư mục %s trong tệp nén không có bit thực thi (%04o): giải nén ra sẽ không vào được",
				entry.Name, entry.Mode().Perm())
		}
	}
	if got := found["duan/con/"]; got != 0o750 {
		t.Errorf("quyền thư mục = %04o, mong đợi 750", got)
	}
	if got := found["duan/con/b.txt"]; got != 0o640 {
		t.Errorf("quyền tệp = %04o, mong đợi 640", got)
	}

	if _, err := svc.Extract(ctx, "/luu-tru.zip", "/ra"); err != nil {
		t.Fatalf("giải nén: %v", err)
	}
	info, err := os.Stat(filepath.Join(root, "ra", "duan", "con"))
	if err != nil {
		t.Fatalf("đọc thư mục đã giải nén: %v", err)
	}
	if info.Mode().Perm()&0o700 != 0o700 {
		t.Errorf("thư mục giải ra có quyền %04o: chủ sở hữu phải vào và ghi được", info.Mode().Perm())
	}
}

func TestCompressAndExtractRoundTrip(t *testing.T) {
	svc, root := newTestFileService(t)
	writeFile(t, filepath.Join(root, "duan", "a.txt"), "tệp A")
	writeFile(t, filepath.Join(root, "duan", "con", "b.txt"), "tệp B")
	ctx := context.Background()

	for _, format := range []ArchiveFormat{FormatZip, FormatTar, FormatTarGz, FormatTarXz, FormatTarZst} {
		t.Run(string(format), func(t *testing.T) {
			archive := "/luu-tru." + string(format)
			if err := svc.Compress(ctx, []string{"/duan"}, archive, format); err != nil {
				t.Fatalf("nén: %v", err)
			}

			target := "/giai-nen-" + strings.ReplaceAll(string(format), ".", "-")
			if _, err := svc.Extract(ctx, archive, target); err != nil {
				t.Fatalf("giải nén: %v", err)
			}

			data, err := os.ReadFile(filepath.Join(root, target, "duan", "con", "b.txt"))
			if err != nil {
				t.Fatalf("đọc tệp đã giải nén: %v", err)
			}
			if string(data) != "tệp B" {
				t.Errorf("nội dung = %q, mong đợi \"tệp B\"", data)
			}
		})
	}
}

// Đây là bài test chống "zip slip": một tệp nén độc hại chứa mục có tên leo thư
// mục phải bị từ chối, không được ghi ra ngoài thư mục đích.
func TestExtractRejectsZipSlip(t *testing.T) {
	svc, root := newTestFileService(t)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	entry, err := zw.Create("../../../thoat-ra-ngoai.txt")
	if err != nil {
		t.Fatalf("tạo mục trong tệp nén: %v", err)
	}
	if _, err := entry.Write([]byte("đã thoát ra ngoài")); err != nil {
		t.Fatalf("ghi mục: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("đóng tệp nén: %v", err)
	}

	writeFile(t, filepath.Join(root, "doc-hai.zip"), buf.String())

	_, err = svc.Extract(context.Background(), "/doc-hai.zip", "/dich")
	if !errors.Is(err, apperr.FileUnsafeArchive) {
		t.Fatalf("lỗi = %v, mong đợi FileUnsafeArchive", err)
	}

	// Và quan trọng nhất: tệp không được xuất hiện ở bất cứ đâu ngoài gốc.
	escaped := filepath.Join(filepath.Dir(root), "thoat-ra-ngoai.txt")
	if _, err := os.Stat(escaped); err == nil {
		os.Remove(escaped)
		t.Fatal("tệp đã bị ghi ra ngoài thư mục gốc")
	}
}

func TestExtractRejectsUnsupportedFormat(t *testing.T) {
	svc, root := newTestFileService(t)
	writeFile(t, filepath.Join(root, "ghi-chu.txt"), "chỉ là văn bản")

	_, err := svc.Extract(context.Background(), "/ghi-chu.txt", "/dich")
	if !errors.Is(err, apperr.FileUnsupportedFormat) {
		t.Fatalf("lỗi = %v, mong đợi FileUnsupportedFormat", err)
	}
}

// Tên tệp nói dối được, nên định dạng phải đoán tiếp từ chữ ký đầu tệp: một bản
// tải về đặt tên "source.download" vẫn là tệp zip và vẫn phải mở ra được.
func TestExtractDetectsFormatWithoutExtension(t *testing.T) {
	svc, root := newTestFileService(t)
	writeFile(t, filepath.Join(root, "duan", "a.txt"), "tệp A")
	ctx := context.Background()

	if err := svc.Compress(ctx, []string{"/duan"}, "/luu-tru.zip", FormatZip); err != nil {
		t.Fatalf("nén: %v", err)
	}
	if err := os.Rename(filepath.Join(root, "luu-tru.zip"), filepath.Join(root, "source.download")); err != nil {
		t.Fatalf("đổi tên: %v", err)
	}

	result, err := svc.Extract(ctx, "/source.download", "/dich")
	if err != nil {
		t.Fatalf("giải nén: %v", err)
	}
	if result.Files != 1 {
		t.Fatalf("giải ra %d tệp, mong 1", result.Files)
	}
}

// Đuôi tên đúng nhưng nội dung không phải tệp nén phải báo tệp hỏng, chứ không
// phải "không hỗ trợ định dạng" — hai nguyên nhân khác nhau cần hai cách xử lý.
func TestExtractReportsCorruptArchive(t *testing.T) {
	svc, root := newTestFileService(t)
	writeFile(t, filepath.Join(root, "tep.rar"), "không phải rar")

	_, err := svc.Extract(context.Background(), "/tep.rar", "/dich")
	if !errors.Is(err, apperr.FileCorruptArchive) {
		t.Fatalf("lỗi = %v, mong đợi FileCorruptArchive", err)
	}
}

func TestNormalizePath(t *testing.T) {
	cases := map[string]string{
		"":              "/",
		"/":             "/",
		"www":           "/www",
		"/www/":         "/www",
		"//www//html//": "/www/html",
		"/www/./html":   "/www/html",
		"  /www  ":      "/www",
	}
	for input, want := range cases {
		if got := normalizePath(input); got != want {
			t.Errorf("normalizePath(%q) = %q, mong đợi %q", input, got, want)
		}
	}
}
