package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/thanhtinz/sunpanel/internal/model"
	"github.com/thanhtinz/sunpanel/pkg/crypto"
	"github.com/thanhtinz/sunpanel/pkg/notify"
	"github.com/thanhtinz/sunpanel/pkg/sshx"
)

// newAlertingNodes dựng dịch vụ máy chủ có bật cảnh báo, kèm một webhook nhận
// thử để đọc lại đúng những gì được gửi đi.
func newAlertingNodes(t *testing.T) (*NodeService, *hookServer) {
	t.Helper()

	nodes := newNodeService(t)
	sealer, err := crypto.NewSealer(make([]byte, 32))
	if err != nil {
		t.Fatalf("khởi tạo bộ mã hóa: %v", err)
	}

	alerts := NewAlertService(nodes.db, sealer, &MonitorService{}, NewAuditService(nodes.db))
	hook := newHookServer(t)
	addChannel(t, alerts, "kenh-thu", hook.URL)

	nodes.SetAlerts(alerts)
	return nodes, hook
}

// alertingNode là một máy chủ đã bật đủ các loại cảnh báo.
func alertingNode() model.Node {
	return model.Node{
		ID: 1, Name: "vps-sai-gon", Kind: model.NodeSSH,
		Address:      "ssh://root@203.0.113.10:22",
		AlertOffline: true, AlertCPU: 80, AlertMemory: 90, AlertDisk: 85,
	}
}

// Một mẫu đơn lẻ không phải sự cố: bản sao lưu lúc nửa đêm đẩy CPU lên 100%
// trong đúng một phút, và báo ngay lần đầu là cách nhanh nhất để người dùng tắt
// hết cảnh báo.
func TestNodeAlertNeedsStreak(t *testing.T) {
	nodes, hook := newAlertingNodes(t)
	record := alertingNode()
	ctx := context.Background()

	nodes.noteSample(ctx, record, sshx.Metrics{CPUPercent: 95})
	nodes.noteSample(ctx, record, sshx.Metrics{CPUPercent: 95})
	if hook.count() != 0 {
		t.Fatalf("báo quá sớm: %d thông báo", hook.count())
	}

	nodes.noteSample(ctx, record, sshx.Metrics{CPUPercent: 95})
	if hook.count() != 1 {
		t.Fatalf("số thông báo = %d, mong 1", hook.count())
	}
	if title, _ := hook.last()["title"].(string); !strings.Contains(title, "CPU") {
		t.Errorf("tiêu đề = %q", title)
	}

	// Vẫn quá tải thì không báo lại: một máy chủ nghẽn cả đêm sẽ sinh ra hàng
	// trăm tin nhắn giống hệt nhau.
	nodes.noteSample(ctx, record, sshx.Metrics{CPUPercent: 97})
	if hook.count() != 1 {
		t.Errorf("báo lặp lại: %d thông báo", hook.count())
	}
}

// Hạ xuống dưới ngưỡng phải báo lại: người nhận cần biết sự cố đã hết, nếu
// không họ vẫn đang chạy đi xử lý một thứ đã tự khỏi.
func TestNodeAlertRecovery(t *testing.T) {
	nodes, hook := newAlertingNodes(t)
	record := alertingNode()
	ctx := context.Background()

	for i := 0; i < alertStreak; i++ {
		nodes.noteSample(ctx, record, sshx.Metrics{CPUPercent: 95})
	}
	nodes.noteSample(ctx, record, sshx.Metrics{CPUPercent: 10})

	if hook.count() != 2 {
		t.Fatalf("số thông báo = %d, mong 2", hook.count())
	}
	if level, _ := hook.last()["level"].(string); level != string(notify.LevelInfo) {
		t.Errorf("mức của thông báo hồi phục = %q", level)
	}
}

func TestNodeAlertOfflineAndBack(t *testing.T) {
	nodes, hook := newAlertingNodes(t)
	record := alertingNode()
	ctx := context.Background()

	for i := 0; i < alertStreak; i++ {
		nodes.noteSampleFailure(ctx, record, errors.New("connection refused"))
	}
	if hook.count() != 1 {
		t.Fatalf("số thông báo = %d, mong 1", hook.count())
	}
	if level, _ := hook.last()["level"].(string); level != string(notify.LevelCritical) {
		t.Errorf("mức của thông báo mất kết nối = %q", level)
	}

	// Lấy mẫu được trở lại nghĩa là máy đã sống.
	nodes.noteSample(ctx, record, sshx.Metrics{CPUPercent: 5})
	title, _ := hook.last()["title"].(string)
	if hook.count() != 2 || !strings.Contains(title, "trở lại") {
		t.Fatalf("thông báo hồi phục: %d thông báo, tiêu đề %q", hook.count(), title)
	}

	// Và lần mất kết nối sau đó lại được báo, chứ không bị coi là đã báo rồi.
	for i := 0; i < alertStreak; i++ {
		nodes.noteSampleFailure(ctx, record, errors.New("connection refused"))
	}
	if hook.count() != 3 {
		t.Errorf("số thông báo = %d, mong 3", hook.count())
	}
}

// Ngưỡng đang tắt thì không bao giờ báo, dù số liệu có cao tới đâu.
func TestNodeAlertRespectsDisabledThresholds(t *testing.T) {
	nodes, hook := newAlertingNodes(t)
	record := alertingNode()
	record.AlertCPU, record.AlertMemory, record.AlertDisk = 0, 0, 0
	record.AlertOffline = false
	ctx := context.Background()

	for i := 0; i < alertStreak+2; i++ {
		nodes.noteSample(ctx, record, sshx.Metrics{CPUPercent: 100, MemoryPercent: 100, DiskPercent: 100})
		nodes.noteSampleFailure(ctx, record, errors.New("connection refused"))
	}
	if hook.count() != 0 {
		t.Errorf("cảnh báo đã tắt nhưng vẫn gửi %d thông báo", hook.count())
	}
}

// Ngưỡng 0 và ngưỡng trên 100 đều nghĩa là không cảnh báo; gộp thành 0 cho rõ.
func TestClampPercent(t *testing.T) {
	cases := map[int]int{-5: 0, 0: 0, 1: 1, 80: 80, 100: 100, 101: 0}
	for input, want := range cases {
		if got := clampPercent(input); got != want {
			t.Errorf("clampPercent(%d) = %d, mong %d", input, got, want)
		}
	}
}

func TestAlertSummary(t *testing.T) {
	record := alertingNode()
	if got := AlertSummary(record); got != "mất kết nối, CPU 80%, bộ nhớ 90%, đĩa 85%" {
		t.Errorf("tóm tắt = %q", got)
	}

	record.AlertOffline, record.AlertMemory, record.AlertDisk = false, 0, 0
	if got := AlertSummary(record); got != "CPU 80%" {
		t.Errorf("tóm tắt khi chỉ còn một ngưỡng = %q", got)
	}
}
