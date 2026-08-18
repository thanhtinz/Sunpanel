package service

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
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

func TestCompressAndExtractRoundTrip(t *testing.T) {
	svc, root := newTestFileService(t)
	writeFile(t, filepath.Join(root, "duan", "a.txt"), "tệp A")
	writeFile(t, filepath.Join(root, "duan", "con", "b.txt"), "tệp B")
	ctx := context.Background()

	for _, format := range []ArchiveFormat{FormatZip, FormatTarGz} {
		t.Run(string(format), func(t *testing.T) {
			archive := "/luu-tru." + string(format)
			if err := svc.Compress(ctx, []string{"/duan"}, archive, format); err != nil {
				t.Fatalf("nén: %v", err)
			}

			target := "/giai-nen-" + strings.ReplaceAll(string(format), ".", "-")
			if err := svc.Extract(ctx, archive, target); err != nil {
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

	err = svc.Extract(context.Background(), "/doc-hai.zip", "/dich")
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
	writeFile(t, filepath.Join(root, "tep.rar"), "không phải zip")

	err := svc.Extract(context.Background(), "/tep.rar", "/dich")
	if !errors.Is(err, apperr.FileUnsupportedFormat) {
		t.Fatalf("lỗi = %v, mong đợi FileUnsupportedFormat", err)
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
