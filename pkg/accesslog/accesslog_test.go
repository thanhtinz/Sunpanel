package accesslog

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/thanhtinz/sunpanel/pkg/host"
)

func TestParseCombinedLine(t *testing.T) {
	line := `203.0.113.7 - - [19/Aug/2026:10:15:32 +0700] "GET /bai-viet/xin-chao?utm=a HTTP/1.1" 200 5120 "https://google.com/" "Mozilla/5.0 (X11; Linux x86_64)"`

	entry, ok := Parse(line)
	if !ok {
		t.Fatal("không tách được dòng đúng định dạng")
	}
	if entry.IP != "203.0.113.7" || entry.Method != "GET" {
		t.Errorf("địa chỉ/phương thức = %q %q", entry.IP, entry.Method)
	}
	if entry.Path != "/bai-viet/xin-chao?utm=a" || entry.Status != 200 || entry.Bytes != 5120 {
		t.Errorf("đường dẫn/mã/dung lượng = %q %d %d", entry.Path, entry.Status, entry.Bytes)
	}
	if entry.Referrer != "https://google.com/" || entry.UserAgent == "" {
		t.Errorf("nguồn/trình duyệt = %q %q", entry.Referrer, entry.UserAgent)
	}
	if entry.Time.Format(time.RFC3339) != "2026-08-19T10:15:32+07:00" {
		t.Errorf("thời điểm = %s", entry.Time.Format(time.RFC3339))
	}
}

// Một tệp nhật ký thật luôn có vài dòng rác; chúng không được phép làm hỏng cả
// bản tóm tắt, chỉ được đếm là dòng bỏ qua.
func TestParseRejectsBadLines(t *testing.T) {
	bad := []string{
		"",
		"không phải nhật ký",
		`203.0.113.7 - - [không-phải-thời-gian] "GET / HTTP/1.1" 200 10 "-" "-"`,
		`203.0.113.7 - - [19/Aug/2026:10:15:32 +0700] "GET / HTTP/1.1`,
	}
	for _, line := range bad {
		if _, ok := Parse(line); ok {
			t.Errorf("dòng hỏng lại được chấp nhận: %q", line)
		}
	}
}

// Dấu ngoặc kép trong dữ liệu do người gửi khai được nginx thoát thành \"; coi
// nó là dấu đóng thì mọi trường phía sau lệch đi một nấc.
func TestParseHandlesEscapedQuotes(t *testing.T) {
	line := `203.0.113.7 - - [19/Aug/2026:10:15:32 +0700] "GET /a HTTP/1.1" 404 0 "-" "Máy \"quét\" lạ"`

	entry, ok := Parse(line)
	if !ok {
		t.Fatal("không tách được dòng có dấu ngoặc thoát")
	}
	if entry.Status != 404 {
		t.Errorf("mã trạng thái = %d", entry.Status)
	}
	if entry.UserAgent != `Máy "quét" lạ` {
		t.Errorf("chuỗi trình duyệt = %q", entry.UserAgent)
	}
}

// writeLog dựng một tệp nhật ký tạm.
func writeLog(t *testing.T, lines []string) (*Analyzer, string) {
	t.Helper()

	dir := t.TempDir()
	target := filepath.Join(dir, "site.access.log")

	content := ""
	for _, line := range lines {
		content += line + "\n"
	}
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
		t.Fatalf("ghi tệp nhật ký: %v", err)
	}
	// Lớp host giới hạn mọi đường dẫn trong gốc của nó, nên tên truyền vào là
	// đường dẫn tính từ gốc đó.
	return New(host.NewLocalHost(dir, nil).FS()), "/" + filepath.Base(target)
}

// entryLine dựng một dòng nhật ký cho test.
func entryLine(stamp time.Time, ip, path string, status int, bytes int64) string {
	return fmt.Sprintf(`%s - - [%s] "GET %s HTTP/1.1" %d %d "-" "trinh-duyet"`,
		ip, stamp.Format(timeLayout), path, status, bytes)
}

func TestAnalyzeSummarizes(t *testing.T) {
	base := time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)
	analyzer, target := writeLog(t, []string{
		entryLine(base, "203.0.113.1", "/", 200, 100),
		entryLine(base.Add(time.Minute), "203.0.113.1", "/?tim=a", 200, 100),
		entryLine(base.Add(2*time.Minute), "203.0.113.2", "/thieu", 404, 0),
		entryLine(base.Add(time.Hour), "203.0.113.3", "/hong", 500, 0),
		"dòng rác",
	})

	stats, err := analyzer.Analyze(context.Background(), target, 24*time.Hour)
	if err != nil {
		t.Fatalf("phân tích: %v", err)
	}

	if stats.Requests != 4 || stats.Visitors != 3 || stats.Bytes != 200 {
		t.Errorf("tổng = %d yêu cầu, %d khách, %d byte", stats.Requests, stats.Visitors, stats.Bytes)
	}
	if stats.Status2xx != 2 || stats.Status4xx != 1 || stats.Status5xx != 1 {
		t.Errorf("theo mã = %d/%d/%d", stats.Status2xx, stats.Status4xx, stats.Status5xx)
	}
	if stats.Skipped != 1 {
		t.Errorf("số dòng bỏ qua = %d, mong 1", stats.Skipped)
	}

	// Tham số truy vấn phải bị cắt, nếu không bảng xếp hạng chỉ toàn số 1.
	if len(stats.TopPaths) == 0 || stats.TopPaths[0].Key != "/" || stats.TopPaths[0].Count != 2 {
		t.Errorf("đường dẫn nhiều lượt nhất = %+v", stats.TopPaths)
	}
	if len(stats.Buckets) != 2 {
		t.Fatalf("số khung thời gian = %d, mong 2", len(stats.Buckets))
	}
	if stats.Buckets[0].Requests != 3 || stats.Buckets[0].Errors != 1 {
		t.Errorf("khung đầu tiên = %+v", stats.Buckets[0])
	}
	// Lỗi mới nhất phải lên đầu: đó là thứ đang hỏng ngay lúc này.
	if len(stats.Failures) != 2 || stats.Failures[0].Status != 500 {
		t.Errorf("danh sách lỗi = %+v", stats.Failures)
	}
}

// Mốc thời gian lấy từ dòng cuối tệp chứ không phải đồng hồ máy: nhật ký của
// một website không có khách nào từ hôm qua vẫn phải cho ra số liệu của hôm qua.
func TestAnalyzeWindowFollowsNewestEntry(t *testing.T) {
	old := time.Now().Add(-30 * 24 * time.Hour)
	analyzer, target := writeLog(t, []string{
		entryLine(old.Add(-48*time.Hour), "203.0.113.1", "/cu", 200, 10),
		entryLine(old, "203.0.113.2", "/moi", 200, 10),
	})

	stats, err := analyzer.Analyze(context.Background(), target, 24*time.Hour)
	if err != nil {
		t.Fatalf("phân tích: %v", err)
	}
	if stats.Requests != 1 || stats.TopPaths[0].Key != "/moi" {
		t.Errorf("số liệu = %d yêu cầu, %+v", stats.Requests, stats.TopPaths)
	}
}

// Tệp rỗng phải cho ra danh sách rỗng chứ không phải null: giao diện đọc thẳng
// .length của chúng, và null làm cả trang trắng xóa.
// Chia theo giờ cho mọi khoảng thì biểu đồ của một giờ chỉ còn một cột, và một
// cột đơn độc không vẽ thành đường — người xem thấy khung trống dù có số liệu.
func TestAnalyzeBucketSizeFollowsWindow(t *testing.T) {
	now := time.Now()
	lines := make([]string, 0, 12)
	for i := 0; i < 12; i++ {
		lines = append(lines, entryLine(now.Add(-time.Duration(i)*5*time.Minute), "203.0.113.1", "/", 200, 10))
	}
	analyzer, target := writeLog(t, lines)

	short, err := analyzer.Analyze(context.Background(), target, time.Hour)
	if err != nil {
		t.Fatalf("phân tích: %v", err)
	}
	if short.BucketSeconds != 300 {
		t.Errorf("độ dài khung của một giờ = %d giây, mong 300", short.BucketSeconds)
	}
	if len(short.Buckets) < 6 {
		t.Errorf("số cột của một giờ = %d, quá ít để vẽ thành đường", len(short.Buckets))
	}

	long, err := analyzer.Analyze(context.Background(), target, 24*time.Hour)
	if err != nil {
		t.Fatalf("phân tích: %v", err)
	}
	if long.BucketSeconds != 3600 {
		t.Errorf("độ dài khung của một ngày = %d giây, mong 3600", long.BucketSeconds)
	}
}

func TestAnalyzeEmptyFile(t *testing.T) {
	analyzer, target := writeLog(t, nil)

	stats, err := analyzer.Analyze(context.Background(), target, time.Hour)
	if err != nil {
		t.Fatalf("phân tích: %v", err)
	}
	if stats.TopPaths == nil || stats.Buckets == nil || stats.Failures == nil {
		t.Error("có danh sách trả về nil")
	}
}
