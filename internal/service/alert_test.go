package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/thanhtinz/sunpanel/internal/model"
	"github.com/thanhtinz/sunpanel/pkg/crypto"
	"github.com/thanhtinz/sunpanel/pkg/monitor"
	"github.com/thanhtinz/sunpanel/pkg/notify"
	"gorm.io/gorm"
)

// hookServer là một webhook giả ghi lại mọi thông báo nhận được.
type hookServer struct {
	*httptest.Server
	mu       sync.Mutex
	received []map[string]any
}

func newHookServer(t *testing.T) *hookServer {
	t.Helper()

	hook := &hookServer{}
	hook.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		json.Unmarshal(body, &payload)

		hook.mu.Lock()
		hook.received = append(hook.received, payload)
		hook.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(hook.Close)
	return hook
}

func (h *hookServer) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.received)
}

func (h *hookServer) last() map[string]any {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.received) == 0 {
		return nil
	}
	return h.received[len(h.received)-1]
}

func newAlertFixture(t *testing.T) (*AlertService, *gorm.DB) {
	t.Helper()

	db := newMemoryDB(t)
	sealer, err := crypto.NewSealer(make([]byte, 32))
	if err != nil {
		t.Fatalf("khởi tạo bộ mã hóa: %v", err)
	}
	return NewAlertService(db, sealer, &MonitorService{}, NewAuditService(db)), db
}

// addChannel thêm một kênh webhook trỏ về máy chủ giả.
func addChannel(t *testing.T, alerts *AlertService, name, url string) model.NotifyChannel {
	t.Helper()

	channel, err := alerts.CreateChannel(context.Background(), ChannelRequest{
		Name: name, Kind: "webhook", Enabled: true,
		Config: map[string]string{"url": url},
	})
	if err != nil {
		t.Fatalf("tạo kênh: %v", err)
	}
	return channel
}

// Cấu hình thiếu trường phải lộ ra lúc lưu, chứ không phải lúc có sự cố và cảnh
// báo không gửi được.
func TestCreateChannelValidatesConfig(t *testing.T) {
	alerts, _ := newAlertFixture(t)
	ctx := context.Background()

	cases := []ChannelRequest{
		{Name: "thieu-url", Kind: "webhook", Config: map[string]string{}},
		{Name: "thieu-token", Kind: "telegram", Config: map[string]string{"chatId": "1"}},
		{Name: "sai-loai", Kind: "sms", Config: map[string]string{"x": "y"}},
		{Name: "thieu-smtp", Kind: "email", Config: map[string]string{"host": "smtp.vi-du.vn"}},
	}
	for _, req := range cases {
		if _, err := alerts.CreateChannel(ctx, req); err == nil {
			t.Errorf("cấu hình %q phải bị từ chối", req.Name)
		}
	}
}

// Cấu hình kênh chứa mật khẩu SMTP và token bot; một bản sao lưu cơ sở dữ liệu
// bị lộ không được kéo theo chúng.
func TestChannelConfigIsEncryptedAtRest(t *testing.T) {
	alerts, db := newAlertFixture(t)

	_, err := alerts.CreateChannel(context.Background(), ChannelRequest{
		Name: "bot", Kind: "telegram", Enabled: true,
		Config: map[string]string{"token": "TOKEN-RAT-BI-MAT", "chatId": "-100"},
	})
	if err != nil {
		t.Fatalf("tạo kênh: %v", err)
	}

	var stored model.NotifyChannel
	if err := db.First(&stored).Error; err != nil {
		t.Fatalf("đọc lại kênh: %v", err)
	}
	if strings.Contains(stored.Config, "TOKEN-RAT-BI-MAT") {
		t.Errorf("token còn đọc được trong cơ sở dữ liệu: %s", stored.Config)
	}
}

func TestSendDeliversToEnabledChannels(t *testing.T) {
	alerts, _ := newAlertFixture(t)
	ctx := context.Background()

	first := newHookServer(t)
	second := newHookServer(t)
	addChannel(t, alerts, "kenh-mot", first.URL)
	addChannel(t, alerts, "kenh-hai", second.URL)

	// Kênh đã tắt không được nhận gì.
	tat := addChannel(t, alerts, "kenh-tat", first.URL)
	if _, err := alerts.UpdateChannel(ctx, tat.ID, ChannelRequest{
		Name: "kenh-tat", Kind: "webhook", Enabled: false,
	}); err != nil {
		t.Fatalf("tắt kênh: %v", err)
	}

	alerts.Send(ctx, notify.Message{
		Title: "Thử", Body: "Nội dung", Level: notify.LevelWarning,
	}, "", "test", 0)

	if first.count() != 1 || second.count() != 1 {
		t.Errorf("mỗi kênh đang bật phải nhận đúng một lần: %d và %d", first.count(), second.count())
	}
	if hostname, _ := first.last()["hostname"].(string); hostname == "" {
		t.Error("thông báo thiếu tên máy chủ")
	}
}

// Gửi tới danh sách kênh chỉ định thì các kênh khác không được nhận.
func TestSendRespectsChannelSelection(t *testing.T) {
	alerts, _ := newAlertFixture(t)
	ctx := context.Background()

	chosen := newHookServer(t)
	other := newHookServer(t)
	addChannel(t, alerts, "duoc-chon", chosen.URL)
	addChannel(t, alerts, "khong-chon", other.URL)

	alerts.Send(ctx, notify.Message{Title: "Thử"}, "duoc-chon", "test", 0)

	if chosen.count() != 1 {
		t.Errorf("kênh được chọn phải nhận: %d", chosen.count())
	}
	if other.count() != 0 {
		t.Errorf("kênh không được chọn không được nhận: %d", other.count())
	}
}

// Một kênh hỏng không được ngăn các kênh còn lại nhận cảnh báo.
func TestSendContinuesAfterChannelFailure(t *testing.T) {
	alerts, db := newAlertFixture(t)
	ctx := context.Background()

	working := newHookServer(t)
	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer broken.Close()

	addChannel(t, alerts, "hong", broken.URL)
	addChannel(t, alerts, "chay-tot", working.URL)

	alerts.Send(ctx, notify.Message{Title: "Thử"}, "", "test", 0)

	if working.count() != 1 {
		t.Errorf("kênh còn tốt vẫn phải nhận được: %d", working.count())
	}

	var event model.AlertEvent
	if err := db.Order("id DESC").First(&event).Error; err != nil {
		t.Fatalf("đọc lịch sử cảnh báo: %v", err)
	}
	if event.Delivered != 1 || event.Failed != 1 {
		t.Errorf("thống kê sai: gửi được %d, hỏng %d", event.Delivered, event.Failed)
	}
	if !strings.Contains(event.Error, "hong") {
		t.Errorf("lịch sử chưa ghi kênh nào hỏng: %q", event.Error)
	}
}

// Quy tắc chỉ được phát sau khi số liệu vượt ngưỡng LIÊN TỤC đủ lâu: một lần
// tăng vọt vài giây mà cũng báo thì người dùng sẽ tắt hết cảnh báo.
func TestRuleRequiresSustainedBreach(t *testing.T) {
	alerts, db := newAlertFixture(t)
	ctx := context.Background()

	hook := newHookServer(t)
	addChannel(t, alerts, "kenh", hook.URL)

	rule, err := alerts.CreateRule(ctx, RuleRequest{
		Name: "cpu-cao", Metric: MetricCPU, Threshold: 80,
		DurationMinutes: 5, CooldownMinutes: 60, Enabled: true,
	})
	if err != nil {
		t.Fatalf("tạo quy tắc: %v", err)
	}

	now := time.Now()
	high := monitor.Snapshot{CPUPercent: 95}

	// Lần đầu vượt ngưỡng: chỉ ghi mốc, chưa báo.
	alerts.evaluateRule(ctx, &rule, high, now)
	if hook.count() != 0 {
		t.Fatalf("chưa đủ thời gian mà đã báo: %d", hook.count())
	}

	// Sau 2 phút vẫn chưa đủ 5 phút.
	alerts.evaluateRule(ctx, &rule, high, now.Add(2*time.Minute))
	if hook.count() != 0 {
		t.Fatalf("mới 2/5 phút mà đã báo: %d", hook.count())
	}

	// Quá 5 phút thì phát.
	alerts.evaluateRule(ctx, &rule, high, now.Add(6*time.Minute))
	if hook.count() != 1 {
		t.Fatalf("đủ thời gian nhưng không báo: %d", hook.count())
	}

	body, _ := hook.last()["text"].(string)
	if !strings.Contains(body, "95") {
		t.Errorf("thông báo thiếu giá trị hiện tại: %q", body)
	}

	// Khoảng nghỉ: chưa hết 60 phút thì không báo lại.
	alerts.evaluateRule(ctx, &rule, high, now.Add(10*time.Minute))
	if hook.count() != 1 {
		t.Errorf("khoảng nghỉ không có tác dụng: %d", hook.count())
	}

	// Số liệu về dưới ngưỡng thì mốc vượt ngưỡng phải được xóa.
	alerts.evaluateRule(ctx, &rule, monitor.Snapshot{CPUPercent: 10}, now.Add(11*time.Minute))
	var stored model.AlertRule
	if err := db.First(&stored, rule.ID).Error; err != nil {
		t.Fatalf("đọc lại quy tắc: %v", err)
	}
	if stored.BreachedSince != nil {
		t.Error("mốc vượt ngưỡng chưa được xóa khi số liệu đã bình thường")
	}
}

func TestRuleValidation(t *testing.T) {
	alerts, _ := newAlertFixture(t)
	ctx := context.Background()

	bad := []RuleRequest{
		{Name: "sai-metric", Metric: "khong-ton-tai", Threshold: 10},
		{Name: "khong-nguong", Metric: MetricCPU, Threshold: 0},
		{Name: "nguong-am", Metric: MetricCPU, Threshold: -5},
	}
	for _, req := range bad {
		if _, err := alerts.CreateRule(ctx, req); err == nil {
			t.Errorf("quy tắc %q phải bị từ chối", req.Name)
		}
	}
}

// GORM bỏ qua trường mang giá trị zero khi cột có giá trị mặc định. Với quy tắc
// cảnh báo, hậu quả là lựa chọn "báo ngay" (0 phút) lặng lẽ biến thành "chờ 5
// phút", và người dùng nhận cảnh báo muộn hơn hẳn ý họ.
func TestRuleKeepsZeroDuration(t *testing.T) {
	alerts, db := newAlertFixture(t)
	ctx := context.Background()

	rule, err := alerts.CreateRule(ctx, RuleRequest{
		Name: "bao-ngay", Metric: MetricDisk, Threshold: 90,
		DurationMinutes: 0, CooldownMinutes: 30, Enabled: true,
	})
	if err != nil {
		t.Fatalf("tạo quy tắc: %v", err)
	}
	if rule.DurationMinutes != 0 {
		t.Errorf("thời gian chờ bị đổi thành %d", rule.DurationMinutes)
	}

	var stored model.AlertRule
	if err := db.First(&stored, rule.ID).Error; err != nil {
		t.Fatalf("đọc lại quy tắc: %v", err)
	}
	if stored.DurationMinutes != 0 {
		t.Errorf("cơ sở dữ liệu lưu %d phút thay vì 0", stored.DurationMinutes)
	}

	// Và quy tắc "báo ngay" phải phát ngay ở lần đối chiếu đầu tiên.
	hook := newHookServer(t)
	addChannel(t, alerts, "kenh", hook.URL)
	alerts.evaluateRule(ctx, &stored, monitor.Snapshot{DiskPercent: 95}, time.Now())
	if hook.count() != 1 {
		t.Errorf("quy tắc báo ngay không phát ở lần đối chiếu đầu: %d", hook.count())
	}
}
