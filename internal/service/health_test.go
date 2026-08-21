package service

import (
	"errors"
	"testing"
	"time"

	"github.com/thanhtinz/sunpanel/internal/model"
	"github.com/thanhtinz/sunpanel/pkg/firewall"
	"github.com/thanhtinz/sunpanel/pkg/monitor"
)

// itemByKey tìm một mục trong danh sách kết quả.
func itemByKey(t *testing.T, items []HealthItem, key string) HealthItem {
	t.Helper()
	for _, item := range items {
		if item.Key == key {
			return item
		}
	}
	t.Fatalf("không có mục %q", key)
	return HealthItem{}
}

func TestResourceItemsGradeUsage(t *testing.T) {
	cases := []struct {
		name  string
		snap  monitor.Snapshot
		disk  string
		memRy string
	}{
		{"bình thường", monitor.Snapshot{DiskPercent: 40, MemoryPercent: 50}, HealthOK, HealthOK},
		{"sắp đầy", monitor.Snapshot{DiskPercent: 85, MemoryPercent: 88}, HealthWarn, HealthWarn},
		{"đầy", monitor.Snapshot{DiskPercent: 95, MemoryPercent: 97}, HealthCritical, HealthCritical},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			items := resourceItems(tc.snap)
			if got := itemByKey(t, items, "disk").Level; got != tc.disk {
				t.Errorf("ổ đĩa = %q, mong %q", got, tc.disk)
			}
			if got := itemByKey(t, items, "memory").Level; got != tc.memRy {
				t.Errorf("bộ nhớ = %q, mong %q", got, tc.memRy)
			}
		})
	}
}

// Máy ít RAM mà không có swap thì tiến trình bị nhân giết giữa chừng thay vì
// chạy chậm đi; máy nhiều RAM thì không cần nhắc.
func TestResourceItemsSwapOnlyMattersOnSmallServers(t *testing.T) {
	small := resourceItems(monitor.Snapshot{MemoryTotal: 1 << 30})
	if got := itemByKey(t, small, "swap").Level; got != HealthWarn {
		t.Errorf("máy 1 GB không swap = %q, mong cảnh báo", got)
	}

	big := resourceItems(monitor.Snapshot{MemoryTotal: 16 << 30})
	if got := itemByKey(t, big, "swap").Level; got != HealthOK {
		t.Errorf("máy 16 GB không swap = %q, mong đạt", got)
	}
}

func TestCertItemFindsExpiry(t *testing.T) {
	ok := certItem([]CertificateInfo{{DaysRemaining: 60}}, nil)
	if ok.Level != HealthOK {
		t.Errorf("chứng chỉ còn hạn = %q", ok.Level)
	}

	soon := certItem([]CertificateInfo{{DaysRemaining: 60}, {DaysRemaining: 5}}, nil)
	if soon.Level != HealthWarn || soon.Params["count"] != 1 {
		t.Errorf("chứng chỉ sắp hết hạn = %+v", soon)
	}

	// Tệp biến mất cũng nguy hiểm ngang hết hạn: website mất HTTPS y như nhau.
	gone := certItem([]CertificateInfo{{DaysRemaining: 60, Missing: true}}, nil)
	if gone.Level != HealthCritical {
		t.Errorf("chứng chỉ mất tệp = %q, mong nghiêm trọng", gone.Level)
	}

	if broken := certItem(nil, errors.New("hỏng")); broken.Level != HealthWarn {
		t.Errorf("không đọc được = %q", broken.Level)
	}
}

// Sao lưu hỏng là sự cố im lặng nguy hiểm nhất: người dùng chỉ phát hiện đúng
// vào lúc cần khôi phục.
func TestBackupItemGradesPlans(t *testing.T) {
	if none := backupItem(nil, nil); none.Level != HealthWarn || none.Detail != "backups.none" {
		t.Errorf("không có kế hoạch = %+v", none)
	}

	now := time.Now()
	yes, no := true, false

	failed := backupItem([]model.BackupPlan{
		{Enabled: true, LastRunAt: &now, LastSuccess: &no},
		{Enabled: true, LastRunAt: &now, LastSuccess: &yes},
	}, nil)
	if failed.Level != HealthCritical || failed.Params["count"] != 1 {
		t.Errorf("có kế hoạch hỏng = %+v", failed)
	}

	never := backupItem([]model.BackupPlan{{Enabled: true}}, nil)
	if never.Level != HealthWarn || never.Detail != "backups.never" {
		t.Errorf("chưa chạy lần nào = %+v", never)
	}

	// Kế hoạch bị tắt hết cũng là không có sao lưu, chỉ khác ở chỗ người dùng
	// từng dựng rồi lại tắt đi.
	off := backupItem([]model.BackupPlan{{Enabled: false}}, nil)
	if off.Level != HealthWarn || off.Detail != "backups.disabled" {
		t.Errorf("kế hoạch bị tắt = %+v", off)
	}
}

func TestUptimeItemCountsDown(t *testing.T) {
	down := uptimeItem([]MonitorSummary{
		{UptimeMonitor: model.UptimeMonitor{Enabled: true, Status: "down"}},
		{UptimeMonitor: model.UptimeMonitor{Enabled: true, Status: "up"}},
		// Mục đang tạm dừng không được tính: người dùng đã cố ý tắt nó.
		{UptimeMonitor: model.UptimeMonitor{Enabled: false, Status: "down"}},
	}, nil)

	if down.Level != HealthCritical || down.Params["count"] != 1 {
		t.Errorf("mục mất kết nối = %+v", down)
	}
}

func TestSecurityItemsCheckPanelSettings(t *testing.T) {
	items := securityItems(false, 0, []model.User{{TOTPEnabled: false}, {TOTPEnabled: true}}, nil)

	if got := itemByKey(t, items, "https").Level; got != HealthWarn {
		t.Errorf("HTTPS tắt = %q", got)
	}
	if got := itemByKey(t, items, "loginGuard").Level; got != HealthWarn {
		t.Errorf("chặn đăng nhập tắt = %q", got)
	}
	totp := itemByKey(t, items, "totp")
	if totp.Level != HealthWarn || totp.Params["count"] != 1 {
		t.Errorf("2FA = %+v", totp)
	}

	good := securityItems(true, 10, []model.User{{TOTPEnabled: true}}, nil)
	for _, item := range good {
		if item.Level != HealthOK {
			t.Errorf("thiết lập đủ mà mục %q vẫn báo %q", item.Key, item.Level)
		}
	}
}

func TestFirewallItem(t *testing.T) {
	if off := firewallItem(firewall.Status{Available: true, Backend: "ufw"}, nil); off.Level != HealthWarn {
		t.Errorf("tường lửa tắt = %q", off.Level)
	}
	if missing := firewallItem(firewall.Status{}, errors.New("không có")); missing.Detail != "firewall.missing" {
		t.Errorf("không có công cụ = %q", missing.Detail)
	}
	on := firewallItem(firewall.Status{Available: true, Enabled: true, Backend: "ufw"}, nil)
	if on.Level != HealthOK || on.Params["backend"] != "ufw" {
		t.Errorf("tường lửa bật = %+v", on)
	}
}

// Điểm phải phản ánh mức nặng, và mục nặng nhất phải nằm đầu danh sách: người
// mở trang này đang muốn biết phải làm gì trước.
func TestScoreSortsAndPenalizes(t *testing.T) {
	report := score([]HealthItem{
		{Key: "a", Level: HealthOK},
		{Key: "b", Level: HealthWarn},
		{Key: "c", Level: HealthCritical},
		{Key: "d", Level: HealthWarn},
	})

	if report.Warnings != 2 || report.Criticals != 1 {
		t.Errorf("đếm mức = %d cảnh báo, %d nghiêm trọng", report.Warnings, report.Criticals)
	}
	if want := 100 - 2*penaltyWarn - penaltyCritical; report.Score != want {
		t.Errorf("điểm = %d, mong %d", report.Score, want)
	}
	if report.Items[0].Level != HealthCritical || report.Items[len(report.Items)-1].Level != HealthOK {
		t.Errorf("thứ tự sai: %+v", report.Items)
	}
}

// Điểm không bao giờ được xuống dưới 0: một con số âm trên vòng tròn phần trăm
// là thứ giao diện không vẽ được.
func TestScoreNeverNegative(t *testing.T) {
	items := make([]HealthItem, 10)
	for i := range items {
		items[i] = HealthItem{Level: HealthCritical}
	}
	if got := score(items).Score; got != 0 {
		t.Errorf("điểm = %d, mong 0", got)
	}
}
