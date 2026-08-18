package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/thanhtinz/sunpanel/internal/apperr"
)

func newUptimeFixture(t *testing.T) *UptimeService {
	t.Helper()

	db := newMemoryDB(t)
	return NewUptimeService(db, nil, NewAuditService(db))
}

func TestUptimeCreateChecksImmediately(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ổn"))
	}))
	defer server.Close()

	monitors := newUptimeFixture(t)
	monitor, err := monitors.Create(context.Background(), MonitorRequest{
		Name: "trang-chu", URL: server.URL, Enabled: true,
	}, AuditEntry{})
	if err != nil {
		t.Fatalf("tạo mục theo dõi: %v", err)
	}

	// Kiểm ngay lúc tạo: nếu chờ tới chu kỳ sau thì người dùng nhìn một dấu hỏi
	// và không biết mình đã gõ đúng địa chỉ hay chưa.
	if monitor.Status != "up" {
		t.Fatalf("trạng thái = %q, mong up (%+v)", monitor.Status, monitor.LastError)
	}
	if monitor.LastCheckedAt == nil {
		t.Error("thiếu thời điểm kiểm tra")
	}
}

// Một lần rớt gói tin không phải sự cố: chỉ đủ số lần hỏng liên tiếp mới đổi
// trạng thái, nếu không người dùng sẽ tắt hết cảnh báo sau vài đêm.
func TestUptimeFailureThreshold(t *testing.T) {
	var fail atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if fail.Load() {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte("ổn"))
	}))
	defer server.Close()

	monitors := newUptimeFixture(t)
	created, err := monitors.Create(context.Background(), MonitorRequest{
		Name: "api", URL: server.URL, Enabled: true, FailureThreshold: 3,
	}, AuditEntry{})
	if err != nil {
		t.Fatalf("tạo mục theo dõi: %v", err)
	}

	fail.Store(true)
	for i := 1; i <= 2; i++ {
		checked, err := monitors.CheckNow(context.Background(), created.ID)
		if err != nil {
			t.Fatalf("kiểm tra lần %d: %v", i, err)
		}
		if checked.Status == "down" {
			t.Fatalf("đổi sang down sau %d lần hỏng, ngưỡng là 3", i)
		}
	}

	checked, err := monitors.CheckNow(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("kiểm tra lần ba: %v", err)
	}
	if checked.Status != "down" {
		t.Fatalf("trạng thái = %q, mong down", checked.Status)
	}

	// Một lần trả lời được là hết hỏng ngay, không phải chờ đủ ba lần.
	fail.Store(false)
	recovered, err := monitors.CheckNow(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("kiểm tra sau khi hồi phục: %v", err)
	}
	if recovered.Status != "up" || recovered.ConsecutiveFails != 0 {
		t.Fatalf("sau khi hồi phục: %q, %d lần hỏng", recovered.Status, recovered.ConsecutiveFails)
	}
}

func TestUptimeHistoryAndSummary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ổn"))
	}))
	defer server.Close()

	monitors := newUptimeFixture(t)
	created, _ := monitors.Create(context.Background(), MonitorRequest{
		Name: "trang-chu", URL: server.URL, Enabled: true,
	}, AuditEntry{})
	if _, err := monitors.CheckNow(context.Background(), created.ID); err != nil {
		t.Fatalf("kiểm tra: %v", err)
	}

	history, err := monitors.History(context.Background(), created.ID, 10)
	if err != nil {
		t.Fatalf("lịch sử: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("có %d lần kiểm tra, mong 2", len(history))
	}
	// Lịch sử phải theo thứ tự thời gian để giao diện vẽ từ trái sang phải.
	if history[0].CheckedAt.After(history[1].CheckedAt) {
		t.Error("lịch sử không theo thứ tự thời gian")
	}

	list, err := monitors.List(context.Background())
	if err != nil || len(list) != 1 {
		t.Fatalf("danh sách: %v", err)
	}
	if list[0].Uptime24h != 100 {
		t.Errorf("tỉ lệ sống = %.1f%%, mong 100", list[0].Uptime24h)
	}
	if list[0].Checks != 2 {
		t.Errorf("số lần kiểm tra = %d", list[0].Checks)
	}
}

func TestUptimeValidation(t *testing.T) {
	monitors := newUptimeFixture(t)
	ctx := context.Background()

	_, err := monitors.Create(ctx, MonitorRequest{Name: "x", URL: "ftp://example.com"}, AuditEntry{})
	if !errors.Is(err, apperr.UptimeInvalidURL) {
		t.Errorf("địa chỉ sai giao thức: lỗi = %v", err)
	}

	_, err = monitors.Create(ctx, MonitorRequest{Name: "  ", URL: "https://example.com"}, AuditEntry{})
	if !errors.Is(err, apperr.UptimeInvalidName) {
		t.Errorf("tên trống: lỗi = %v", err)
	}

	_, err = monitors.Create(ctx, MonitorRequest{
		Name: "x", URL: "https://example.com", ExpectedStatus: 900,
	}, AuditEntry{})
	if !errors.Is(err, apperr.UptimeInvalidStatus) {
		t.Errorf("mã trạng thái sai: lỗi = %v", err)
	}
}

// Chu kỳ ngắn quá biến panel thành nguồn tải cho chính dịch vụ nó theo dõi.
func TestUptimeClampsInterval(t *testing.T) {
	monitors := newUptimeFixture(t)

	monitor, err := monitors.Create(context.Background(), MonitorRequest{
		Name: "x", URL: "https://example.invalid", IntervalSeconds: 1, TimeoutSeconds: 999,
	}, AuditEntry{})
	if err != nil {
		t.Fatalf("tạo mục theo dõi: %v", err)
	}
	if monitor.IntervalSeconds != minUptimeInterval {
		t.Errorf("chu kỳ = %d, mong %d", monitor.IntervalSeconds, minUptimeInterval)
	}
	if monitor.TimeoutSeconds != maxUptimeTimeout {
		t.Errorf("thời gian chờ = %d, mong %d", monitor.TimeoutSeconds, maxUptimeTimeout)
	}
}

func TestUptimeDeleteRemovesHistory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer server.Close()

	monitors := newUptimeFixture(t)
	created, _ := monitors.Create(context.Background(), MonitorRequest{
		Name: "x", URL: server.URL, Enabled: true,
	}, AuditEntry{})

	if err := monitors.Delete(context.Background(), created.ID, AuditEntry{}); err != nil {
		t.Fatalf("xóa: %v", err)
	}
	if _, err := monitors.CheckNow(context.Background(), created.ID); !errors.Is(err, apperr.UptimeNotFound) {
		t.Errorf("lỗi = %v, mong UptimeNotFound", err)
	}

	history, err := monitors.History(context.Background(), created.ID, 10)
	if err != nil {
		t.Fatalf("lịch sử: %v", err)
	}
	if len(history) != 0 {
		t.Errorf("còn %d bản ghi lịch sử sau khi xóa mục theo dõi", len(history))
	}
}
