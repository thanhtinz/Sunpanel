package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Tên tệp sao lưu được ghép vào đường dẫn đĩa và URL; một tên như
// "../../etc/passwd" sẽ ghi đè tệp ngoài thư mục sao lưu.
func TestValidateNameRejectsTraversal(t *testing.T) {
	bad := []string{
		"../thoat.tar.gz", "a/b.tar.gz", "/tuyet-doi", "..", "", ".an",
		"a\\b", strings.Repeat("a", 200), "ten co dau cach",
	}
	for _, name := range bad {
		if err := ValidateName(name); !errors.Is(err, ErrInvalidName) {
			t.Errorf("tên %q phải bị từ chối", name)
		}
	}

	for _, name := range []string{"sao-luu-2026-08-18.tar.gz", "db_blog.sql.gz", "A1.tar"} {
		if err := ValidateName(name); err != nil {
			t.Errorf("tên %q phải được chấp nhận: %v", name, err)
		}
	}
}

func TestLocalDestinationRoundTrip(t *testing.T) {
	dir := t.TempDir()
	destination := NewLocal(dir)
	ctx := context.Background()

	if err := destination.Check(ctx); err != nil {
		t.Fatalf("kiểm tra nơi lưu trữ: %v", err)
	}

	const content = "nội dung bản sao lưu"
	if err := destination.Put(ctx, "sao-luu.tar.gz", strings.NewReader(content), int64(len(content))); err != nil {
		t.Fatalf("ghi bản sao lưu: %v", err)
	}

	reader, err := destination.Get(ctx, "sao-luu.tar.gz")
	if err != nil {
		t.Fatalf("đọc bản sao lưu: %v", err)
	}
	got, _ := io.ReadAll(reader)
	reader.Close()
	if string(got) != content {
		t.Errorf("nội dung đọc lại khác nội dung đã ghi: %q", got)
	}

	objects, err := destination.List(ctx)
	if err != nil {
		t.Fatalf("liệt kê: %v", err)
	}
	if len(objects) != 1 || objects[0].Name != "sao-luu.tar.gz" {
		t.Errorf("danh sách sai: %+v", objects)
	}

	if err := destination.Delete(ctx, "sao-luu.tar.gz"); err != nil {
		t.Fatalf("xóa: %v", err)
	}
	if _, err := destination.Get(ctx, "sao-luu.tar.gz"); !errors.Is(err, ErrNotFound) {
		t.Errorf("tệp đã xóa phải báo không tìm thấy, nhận: %v", err)
	}
}

// Một lần sao lưu bị ngắt giữa chừng không được để lại tệp cụt mang đúng tên
// bản sao lưu, vì người dùng sẽ tưởng nó dùng được.
func TestLocalDestinationHidesPartialFile(t *testing.T) {
	dir := t.TempDir()
	destination := NewLocal(dir)

	err := destination.Put(context.Background(), "hong.tar.gz", failingReader{}, 0)
	if err == nil {
		t.Fatal("nguồn đọc lỗi thì việc ghi phải thất bại")
	}

	objects, err := destination.List(context.Background())
	if err != nil {
		t.Fatalf("liệt kê: %v", err)
	}
	if len(objects) != 0 {
		t.Errorf("không được để lại tệp nào sau khi ghi hỏng: %+v", objects)
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("đọc lỗi") }

func TestArchiveAndExtractDirectory(t *testing.T) {
	source := t.TempDir()
	writeFile(t, filepath.Join(source, "index.html"), "<h1>Xin chào</h1>")
	writeFile(t, filepath.Join(source, "con", "app.js"), "console.log(1)")
	writeFile(t, filepath.Join(source, "node_modules", "rac.js"), "rác")
	writeFile(t, filepath.Join(source, "app.log"), "nhật ký")

	archive := filepath.Join(t.TempDir(), "sao-luu.tar.gz")
	size, err := ArchiveDirectory(context.Background(), source, archive, []string{"node_modules", "*.log"})
	if err != nil {
		t.Fatalf("nén thư mục: %v", err)
	}
	if size <= 0 {
		t.Error("tệp nén rỗng")
	}

	target := t.TempDir()
	file, err := os.Open(archive)
	if err != nil {
		t.Fatalf("mở tệp nén: %v", err)
	}
	defer file.Close()
	if err := ExtractArchive(context.Background(), file, target); err != nil {
		t.Fatalf("giải nén: %v", err)
	}

	if got := readFile(t, filepath.Join(target, "index.html")); got != "<h1>Xin chào</h1>" {
		t.Errorf("nội dung tệp sai sau khi khôi phục: %q", got)
	}
	if got := readFile(t, filepath.Join(target, "con", "app.js")); got != "console.log(1)" {
		t.Errorf("tệp trong thư mục con sai: %q", got)
	}
	// Mẫu loại trừ phải thực sự có tác dụng, nếu không bản sao lưu website sẽ
	// gấp nhiều lần dung lượng cần thiết vì node_modules.
	if _, err := os.Stat(filepath.Join(target, "node_modules", "rac.js")); !os.IsNotExist(err) {
		t.Error("thư mục bị loại trừ vẫn lọt vào tệp nén")
	}
	if _, err := os.Stat(filepath.Join(target, "app.log")); !os.IsNotExist(err) {
		t.Error("tệp khớp mẫu loại trừ vẫn lọt vào tệp nén")
	}
}

// Tệp nén có thể do người khác tạo: một mục tên "../../etc/cron.d/x" sẽ ghi đè
// tệp hệ thống khi khôi phục.
func TestExtractRejectsPathTraversal(t *testing.T) {
	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzipWriter)

	content := []byte("độc hại")
	header := &tar.Header{
		Name: "../../thoat.txt", Mode: 0o600,
		Size: int64(len(content)), Typeflag: tar.TypeReg,
	}
	if err := tarWriter.WriteHeader(header); err != nil {
		t.Fatalf("ghi tiêu đề: %v", err)
	}
	tarWriter.Write(content)
	tarWriter.Close()
	gzipWriter.Close()

	target := t.TempDir()
	err := ExtractArchive(context.Background(), &buf, target)
	if err == nil {
		t.Fatal("tệp nén chứa mục thoát thư mục phải bị từ chối")
	}
	if !strings.Contains(err.Error(), "ghi ra ngoài") {
		t.Errorf("thông báo lỗi chưa nói rõ lý do: %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(filepath.Dir(target)), "thoat.txt")); err == nil {
		t.Error("tệp đã bị ghi ra ngoài thư mục đích")
	}
}

// Bản sao lưu đã xóa thì không lấy lại được, nên chính sách giữ phải thận trọng.
func TestApplyRetention(t *testing.T) {
	ctx := context.Background()

	newDestination := func(t *testing.T) *LocalDestination {
		dir := t.TempDir()
		destination := NewLocal(dir)
		// Năm bản, mỗi bản cũ hơn bản trước một ngày.
		for i := 0; i < 5; i++ {
			name := "sao-luu-" + string(rune('a'+i)) + ".tar.gz"
			if err := destination.Put(ctx, name, strings.NewReader("x"), 1); err != nil {
				t.Fatalf("ghi bản sao lưu: %v", err)
			}
			when := time.Now().AddDate(0, 0, -i)
			if err := os.Chtimes(filepath.Join(dir, name), when, when); err != nil {
				t.Fatalf("đặt thời gian tệp: %v", err)
			}
		}
		return destination
	}

	t.Run("chỉ giữ theo số lượng", func(t *testing.T) {
		destination := newDestination(t)
		removed, err := ApplyRetention(ctx, destination, 2, 0)
		if err != nil {
			t.Fatalf("áp dụng chính sách: %v", err)
		}
		if len(removed) != 3 {
			t.Errorf("kỳ vọng xóa 3 bản, xóa %d: %v", len(removed), removed)
		}
	})

	t.Run("chỉ giữ theo số ngày", func(t *testing.T) {
		destination := newDestination(t)
		removed, err := ApplyRetention(ctx, destination, 0, 2)
		if err != nil {
			t.Fatalf("áp dụng chính sách: %v", err)
		}
		// Bản 3 và 4 ngày tuổi vượt ngưỡng 2 ngày.
		if len(removed) != 2 {
			t.Errorf("kỳ vọng xóa 2 bản, xóa %d: %v", len(removed), removed)
		}
	})

	t.Run("đặt cả hai thì phải vi phạm cả hai", func(t *testing.T) {
		destination := newDestination(t)
		// Giữ 4 bản mới nhất VÀ giữ mọi bản dưới 2 ngày tuổi: chỉ bản thứ 5 (4
		// ngày tuổi) vi phạm cả hai.
		removed, err := ApplyRetention(ctx, destination, 4, 2)
		if err != nil {
			t.Fatalf("áp dụng chính sách: %v", err)
		}
		if len(removed) != 1 {
			t.Errorf("kỳ vọng xóa 1 bản, xóa %d: %v", len(removed), removed)
		}
	})
}

// Chữ ký SigV4 sai thì S3 trả 403 mà không nói rõ vì sao, nên phải kiểm chính
// chuỗi ký chứ không chỉ kiểm là gọi được.
func TestS3SignsRequest(t *testing.T) {
	var gotAuth, gotDate, gotHash, gotMethod, gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotDate = r.Header.Get("X-Amz-Date")
		gotHash = r.Header.Get("X-Amz-Content-Sha256")
		gotMethod, gotPath = r.Method, r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	destination := NewS3(S3Config{
		Endpoint: server.URL, Region: "ap-southeast-1", Bucket: "sao-luu",
		Prefix: "sunpanel", AccessKeyID: "KHOA", SecretAccessKey: "BI-MAT", PathStyle: true,
	})

	if err := destination.Put(context.Background(), "test.tar.gz", strings.NewReader("noi dung"), 8); err != nil {
		t.Fatalf("tải lên: %v", err)
	}

	if gotMethod != http.MethodPut {
		t.Errorf("phương thức sai: %s", gotMethod)
	}
	if gotPath != "/sao-luu/sunpanel/test.tar.gz" {
		t.Errorf("đường dẫn sai: %s", gotPath)
	}
	if !strings.HasPrefix(gotAuth, "AWS4-HMAC-SHA256 Credential=KHOA/") {
		t.Errorf("tiêu đề ký sai: %s", gotAuth)
	}
	if !strings.Contains(gotAuth, "/ap-southeast-1/s3/aws4_request") {
		t.Errorf("phạm vi ký thiếu vùng: %s", gotAuth)
	}
	if !strings.Contains(gotAuth, "SignedHeaders=host;x-amz-content-sha256;x-amz-date") {
		t.Errorf("danh sách tiêu đề ký sai thứ tự hoặc thiếu: %s", gotAuth)
	}
	if gotDate == "" || gotHash == "" {
		t.Error("thiếu tiêu đề ngày hoặc băm nội dung")
	}
	// Băm phải là của đúng nội dung đã gửi.
	if gotHash != sha256Hex([]byte("noi dung")) {
		t.Errorf("băm nội dung sai: %s", gotHash)
	}
}

// Lỗi của S3 nằm trong thân phản hồi và thường nói rõ vì sao; nuốt nó đi khiến
// người dùng không biết mình sai khóa hay sai vùng.
func TestS3SurfacesServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`<Error><Code>SignatureDoesNotMatch</Code></Error>`))
	}))
	defer server.Close()

	destination := NewS3(S3Config{
		Endpoint: server.URL, Bucket: "b", AccessKeyID: "k", SecretAccessKey: "s", PathStyle: true,
	})

	err := destination.Put(context.Background(), "x.tar.gz", strings.NewReader("x"), 1)
	if err == nil {
		t.Fatal("phản hồi 403 phải thành lỗi")
	}
	if !strings.Contains(err.Error(), "SignatureDoesNotMatch") {
		t.Errorf("lỗi chưa giữ nội dung máy chủ trả về: %v", err)
	}
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

func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("đọc tệp: %v", err)
	}
	return string(content)
}

// Mốc giữ theo ngày phải là nửa đêm, không phải "đúng lúc này trừ N ngày": lệch
// vài giây quyết định việc mất một bản sao lưu không lấy lại được.
func TestRetentionCutoffIsMidnight(t *testing.T) {
	now := time.Date(2026, 8, 18, 14, 30, 0, 0, time.Local)
	cutoff := retentionCutoff(now, 2)

	want := time.Date(2026, 8, 16, 0, 0, 0, 0, time.Local)
	if !cutoff.Equal(want) {
		t.Errorf("mốc giữ sai: %v, kỳ vọng %v", cutoff, want)
	}

	// Bản sao lưu tạo cách đây vừa đúng hai ngày vẫn phải được giữ.
	twoDaysAgo := now.AddDate(0, 0, -2)
	if twoDaysAgo.Before(cutoff) {
		t.Error("bản sao lưu đúng hai ngày tuổi bị coi là quá hạn")
	}
}
