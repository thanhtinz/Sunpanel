package service

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thanhtinz/sunpanel/internal/apperr"
)

// sourceZip dựng một tệp nén mã nguồn trong bộ nhớ.
func sourceZip(t *testing.T, entries map[string]string) []byte {
	t.Helper()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("tạo mục %s: %v", name, err)
		}
		if _, err := io.WriteString(w, content); err != nil {
			t.Fatalf("ghi mục %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("đóng tệp nén: %v", err)
	}
	return buf.Bytes()
}

func newSiteForDeploy(t *testing.T) (*WebsiteService, string, uint) {
	t.Helper()

	websites, _, _, root := newWebsiteFixtureAt(t)
	site, err := websites.Create(context.Background(), WebsiteRequest{
		Name:    "blog",
		Domains: []string{"blog.example.com"},
		Type:    "static",
		Root:    "/www/blog",
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("tạo website: %v", err)
	}
	return websites, filepath.Join(root, "www", "blog"), site.ID
}

// Mã nguồn tải từ GitHub luôn nằm trong một thư mục mang tên dự án; để nguyên
// lớp bọc đó thì trang trả về 404, nên panel phải bóc nó ra.
func TestDeploySourceStripsWrapperDirectory(t *testing.T) {
	websites, rootPath, id := newSiteForDeploy(t)

	data := sourceZip(t, map[string]string{
		"duan-main/index.html":    "<h1>xin chao</h1>",
		"duan-main/css/style.css": "body{}",
		"duan-main/README.md":     "tai lieu",
	})

	result, err := websites.DeploySource(context.Background(), id,
		SourceRequest{}, &Upload{Name: "duan.zip", Reader: bytes.NewReader(data)}, AuditEntry{})
	if err != nil {
		t.Fatalf("triển khai mã nguồn: %v", err)
	}
	if result.Wrapper != "duan-main" {
		t.Errorf("thư mục bọc bị bỏ = %q, mong \"duan-main\"", result.Wrapper)
	}

	content, err := os.ReadFile(filepath.Join(rootPath, "index.html"))
	if err != nil {
		t.Fatalf("đọc trang chủ: %v", err)
	}
	if string(content) != "<h1>xin chao</h1>" {
		t.Errorf("nội dung trang chủ = %q", content)
	}
	if _, err := os.Stat(filepath.Join(rootPath, "css", "style.css")); err != nil {
		t.Errorf("thiếu tệp con: %v", err)
	}
}

// Tệp nén người dùng tải lên chỉ dùng để giải nén; để nó lại trong thư mục web
// nghĩa là phát nguyên bản mã nguồn cho bất kỳ ai đoán đúng tên tệp.
func TestDeploySourceRemovesUploadedArchive(t *testing.T) {
	websites, rootPath, id := newSiteForDeploy(t)

	data := sourceZip(t, map[string]string{"index.html": "trang"})
	if _, err := websites.DeploySource(context.Background(), id,
		SourceRequest{}, &Upload{Name: "src.zip", Reader: bytes.NewReader(data)}, AuditEntry{}); err != nil {
		t.Fatalf("triển khai mã nguồn: %v", err)
	}

	entries, err := os.ReadDir(rootPath)
	if err != nil {
		t.Fatalf("đọc thư mục gốc: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".sunpanel") {
			t.Errorf("còn sót tệp tạm: %s", entry.Name())
		}
	}
}

func TestDeploySourceCleanReplacesOldFiles(t *testing.T) {
	websites, rootPath, id := newSiteForDeploy(t)

	if err := os.WriteFile(filepath.Join(rootPath, "cu.html"), []byte("ban cu"), 0o644); err != nil {
		t.Fatalf("ghi tệp cũ: %v", err)
	}

	data := sourceZip(t, map[string]string{"index.html": "ban moi"})
	if _, err := websites.DeploySource(context.Background(), id,
		SourceRequest{Clean: true}, &Upload{Name: "src.zip", Reader: bytes.NewReader(data)}, AuditEntry{}); err != nil {
		t.Fatalf("triển khai mã nguồn: %v", err)
	}

	if _, err := os.Stat(filepath.Join(rootPath, "cu.html")); !os.IsNotExist(err) {
		t.Error("tệp cũ vẫn còn sau khi chọn xóa sạch thư mục gốc")
	}
	if _, err := os.Stat(filepath.Join(rootPath, "index.html")); err != nil {
		t.Errorf("thiếu tệp mới: %v", err)
	}
}

// Không chọn xóa sạch thì tệp cũ phải còn nguyên, chỉ tệp trùng tên bị thay.
func TestDeploySourceKeepsOtherFilesWithoutClean(t *testing.T) {
	websites, rootPath, id := newSiteForDeploy(t)

	if err := os.WriteFile(filepath.Join(rootPath, "config.php"), []byte("cau hinh"), 0o644); err != nil {
		t.Fatalf("ghi tệp cấu hình: %v", err)
	}

	data := sourceZip(t, map[string]string{"index.html": "ban moi"})
	if _, err := websites.DeploySource(context.Background(), id,
		SourceRequest{}, &Upload{Name: "src.zip", Reader: bytes.NewReader(data)}, AuditEntry{}); err != nil {
		t.Fatalf("triển khai mã nguồn: %v", err)
	}

	if _, err := os.Stat(filepath.Join(rootPath, "config.php")); err != nil {
		t.Errorf("tệp cấu hình cũ đã biến mất: %v", err)
	}
}

// Website dạng chuyển tiếp không có thư mục gốc; triển khai mã nguồn vào đó là
// vô nghĩa nên phải báo lỗi rõ ràng thay vì tạo ra một thư mục không ai đọc.
func TestDeploySourceRejectsProxySite(t *testing.T) {
	websites, _, _, _ := newWebsiteFixtureAt(t)

	site, err := websites.Create(context.Background(), WebsiteRequest{
		Name: "api", Domains: []string{"api.example.com"},
		Type: "proxy", ProxyTarget: "http://127.0.0.1:3000", Enabled: true,
	})
	if err != nil {
		t.Fatalf("tạo website: %v", err)
	}

	_, err = websites.DeploySource(context.Background(), site.ID,
		SourceRequest{}, &Upload{Name: "src.zip", Reader: bytes.NewReader(nil)}, AuditEntry{})
	if !errors.Is(err, apperr.WebsiteNoRoot) {
		t.Fatalf("lỗi = %v, mong WebsiteNoRoot", err)
	}
}

// Một tệp nén độc hại chứa mục leo thư mục không được ghi ra ngoài thư mục gốc
// của website, kể cả khi nó đi qua đường triển khai mã nguồn.
func TestDeploySourceRejectsPathTraversal(t *testing.T) {
	websites, rootPath, id := newSiteForDeploy(t)

	data := sourceZip(t, map[string]string{"../../thoat.txt": "thoat ra ngoai"})
	_, err := websites.DeploySource(context.Background(), id,
		SourceRequest{}, &Upload{Name: "doc-hai.zip", Reader: bytes.NewReader(data)}, AuditEntry{})
	if !errors.Is(err, apperr.FileUnsafeArchive) {
		t.Fatalf("lỗi = %v, mong FileUnsafeArchive", err)
	}

	escaped := filepath.Join(filepath.Dir(filepath.Dir(rootPath)), "thoat.txt")
	if _, err := os.Stat(escaped); err == nil {
		_ = os.Remove(escaped)
		t.Fatal("tệp đã bị ghi ra ngoài thư mục website")
	}
}
