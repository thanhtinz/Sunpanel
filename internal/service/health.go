package service

import (
	"context"
	"sort"

	"gorm.io/gorm"

	"github.com/thanhtinz/sunpanel/internal/config"
	"github.com/thanhtinz/sunpanel/internal/model"
	"github.com/thanhtinz/sunpanel/pkg/firewall"
	"github.com/thanhtinz/sunpanel/pkg/monitor"
)

// Mức nghiêm trọng của một mục kiểm tra.
const (
	// HealthOK là mục đạt yêu cầu.
	HealthOK = "ok"
	// HealthWarn là mục nên sửa nhưng chưa gây sự cố.
	HealthWarn = "warn"
	// HealthCritical là mục đang hoặc sắp gây sự cố.
	HealthCritical = "critical"
)

// Điểm trừ của mỗi mức. Một mục nghiêm trọng phải nặng hơn hẳn một cảnh báo,
// nếu không thì một máy chủ sắp đầy đĩa lại chấm điểm ngang một máy chủ chỉ
// thiếu vài thiết lập cho đẹp.
const (
	penaltyWarn     = 6
	penaltyCritical = 18
)

// HealthItem là kết quả của một mục kiểm tra.
//
// Chỉ mang mã và tham số, không mang câu chữ: giao diện dịch chúng, đúng như
// mọi thông báo khác của panel.
type HealthItem struct {
	// Key là mã của mục, ví dụ "disk".
	Key string `json:"key"`
	// Group gom các mục cùng lĩnh vực, dùng để chia phần trên giao diện.
	Group string `json:"group"`
	// Level là ok, warn hoặc critical.
	Level string `json:"level"`
	// Detail là mã mô tả tình trạng, ví dụ "disk.full".
	Detail string `json:"detail"`
	// Params là các giá trị chèn vào câu mô tả.
	Params map[string]any `json:"params,omitempty"`
	// Route là trang giải quyết được mục này, để giao diện dẫn thẳng tới đó.
	Route string `json:"route,omitempty"`
}

// HealthReport là toàn bộ kết quả một lần kiểm tra.
type HealthReport struct {
	// Score là điểm từ 0 tới 100.
	Score int `json:"score"`
	// Warnings và Criticals là số mục theo từng mức.
	Warnings  int          `json:"warnings"`
	Criticals int          `json:"criticals"`
	Items     []HealthItem `json:"items"`
}

// HealthService rà soát tình trạng máy chủ và các thiết lập của panel.
//
// Mọi con số ở đây đều đã có sẵn trong panel, nằm rải ở bảy trang khác nhau.
// Gom lại một chỗ để trả lời đúng câu người dùng thật sự hỏi mỗi sáng: "có gì
// cần làm hôm nay không".
type HealthService struct {
	db      *gorm.DB
	cfg     config.SecurityConfig
	tls     bool
	monitor *MonitorService
	certs   *CertificateService
	backups *BackupService
	uptime  *UptimeService
	fw      *FirewallService
}

// NewHealthService tạo dịch vụ kiểm tra.
func NewHealthService(
	db *gorm.DB, cfg config.Config, monitor *MonitorService, certificates *CertificateService,
	backups *BackupService, uptime *UptimeService, firewall *FirewallService,
) *HealthService {
	return &HealthService{
		db: db, cfg: cfg.Security, tls: cfg.Server.TLS.Enabled,
		monitor: monitor, certs: certificates, backups: backups,
		uptime: uptime, fw: firewall,
	}
}

// Check thu thập dữ liệu rồi giao cho các hàm chấm điểm.
//
// Phần đọc dữ liệu và phần ra kết luận tách hẳn nhau: kết luận là chỗ dễ sai
// và cần bài kiểm thử, còn nó mà dính vào năm dịch vụ khác thì mỗi bài kiểm
// thử phải dựng cả nửa panel mới chạy được.
func (s *HealthService) Check(ctx context.Context) HealthReport {
	certList, certErr := s.certs.List(ctx)

	var plans []model.BackupPlan
	planErr := s.db.WithContext(ctx).Find(&plans).Error

	monitors, uptimeErr := s.uptime.List(ctx)

	var admins []model.User
	adminErr := s.db.WithContext(ctx).
		Where("role = ? AND active = ?", model.RoleAdmin, true).Find(&admins).Error

	status, fwErr := s.fw.Status(ctx)

	items := make([]HealthItem, 0, 12)
	items = append(items, resourceItems(s.monitor.Latest())...)
	items = append(items, certItem(certList, certErr))
	items = append(items, backupItem(plans, planErr))
	items = append(items, uptimeItem(monitors, uptimeErr))
	items = append(items, securityItems(s.tls, s.cfg.BlockThreshold, admins, adminErr)...)
	items = append(items, firewallItem(status, fwErr))

	return score(items)
}

// score cộng điểm trừ và sắp xếp các mục.
func score(items []HealthItem) HealthReport {
	report := HealthReport{Score: 100, Items: items}
	for _, item := range items {
		switch item.Level {
		case HealthWarn:
			report.Warnings++
			report.Score -= penaltyWarn
		case HealthCritical:
			report.Criticals++
			report.Score -= penaltyCritical
		}
	}
	if report.Score < 0 {
		report.Score = 0
	}

	// Mục nặng nhất lên đầu: người mở trang này đang muốn biết phải làm gì trước.
	order := map[string]int{HealthCritical: 0, HealthWarn: 1, HealthOK: 2}
	sort.SliceStable(report.Items, func(i, j int) bool {
		return order[report.Items[i].Level] < order[report.Items[j].Level]
	})
	return report
}

// resourceItems xem bộ nhớ và ổ đĩa.
func resourceItems(snap monitor.Snapshot) []HealthItem {
	disk := HealthItem{Key: "disk", Group: "resource", Level: HealthOK, Detail: "disk.ok", Route: "disk"}
	disk.Params = map[string]any{"percent": round1(snap.DiskPercent)}
	switch {
	case snap.DiskPercent >= 90:
		disk.Level, disk.Detail = HealthCritical, "disk.full"
	case snap.DiskPercent >= 80:
		disk.Level, disk.Detail = HealthWarn, "disk.high"
	}

	memory := HealthItem{Key: "memory", Group: "resource", Level: HealthOK, Detail: "memory.ok"}
	memory.Params = map[string]any{"percent": round1(snap.MemoryPercent)}
	switch {
	case snap.MemoryPercent >= 95:
		memory.Level, memory.Detail = HealthCritical, "memory.full"
	case snap.MemoryPercent >= 85:
		memory.Level, memory.Detail = HealthWarn, "memory.high"
	}

	// Swap không phải là thứ bắt buộc, nhưng một máy chủ ít RAM mà không có swap
	// thì tiến trình bị nhân giết giữa chừng thay vì chạy chậm đi.
	swap := HealthItem{Key: "swap", Group: "resource", Level: HealthOK, Detail: "swap.ok"}
	if snap.SwapTotal == 0 && snap.MemoryTotal > 0 && snap.MemoryTotal < 2<<30 {
		swap.Level, swap.Detail = HealthWarn, "swap.missing"
	}

	return []HealthItem{disk, memory, swap}
}

// certItem xem chứng chỉ nào sắp hết hạn.
func certItem(list []CertificateInfo, err error) HealthItem {
	item := HealthItem{Key: "certs", Group: "web", Level: HealthOK, Detail: "certs.ok", Route: "websites"}

	if err != nil {
		item.Level, item.Detail = HealthWarn, "certs.unreadable"
		return item
	}
	if len(list) == 0 {
		item.Detail = "certs.none"
		return item
	}

	var expired, expiring int
	soonest := 9999
	for _, cert := range list {
		switch {
		case cert.Missing || cert.DaysRemaining < 0:
			expired++
		case cert.DaysRemaining <= 14:
			expiring++
		}
		if cert.DaysRemaining < soonest {
			soonest = cert.DaysRemaining
		}
	}

	item.Params = map[string]any{"count": len(list), "days": soonest}
	switch {
	case expired > 0:
		item.Level, item.Detail = HealthCritical, "certs.expired"
		item.Params["count"] = expired
	case expiring > 0:
		item.Level, item.Detail = HealthWarn, "certs.expiring"
		item.Params["count"] = expiring
	}
	return item
}

// backupItem xem có kế hoạch sao lưu nào và lần chạy gần nhất ra sao.
//
// Sao lưu hỏng là sự cố im lặng nguy hiểm nhất: người dùng chỉ phát hiện đúng
// vào lúc cần khôi phục.
func backupItem(plans []model.BackupPlan, err error) HealthItem {
	item := HealthItem{Key: "backups", Group: "data", Level: HealthOK, Detail: "backups.ok", Route: "backups"}

	if err != nil {
		item.Level, item.Detail = HealthWarn, "backups.unreadable"
		return item
	}
	if len(plans) == 0 {
		item.Level, item.Detail = HealthWarn, "backups.none"
		return item
	}

	var failed, never, enabled int
	for _, plan := range plans {
		if !plan.Enabled {
			continue
		}
		enabled++
		switch {
		case plan.LastRunAt == nil:
			never++
		case plan.LastSuccess != nil && !*plan.LastSuccess:
			failed++
		}
	}

	item.Params = map[string]any{"count": enabled}
	switch {
	case enabled == 0:
		item.Level, item.Detail = HealthWarn, "backups.disabled"
	case failed > 0:
		item.Level, item.Detail = HealthCritical, "backups.failed"
		item.Params["count"] = failed
	case never == enabled:
		item.Level, item.Detail = HealthWarn, "backups.never"
	}
	return item
}

// uptimeItem xem có mục theo dõi nào đang mất kết nối.
func uptimeItem(monitors []MonitorSummary, err error) HealthItem {
	item := HealthItem{Key: "uptime", Group: "web", Level: HealthOK, Detail: "uptime.ok", Route: "uptime"}

	if err != nil {
		item.Level, item.Detail = HealthWarn, "uptime.unreadable"
		return item
	}
	if len(monitors) == 0 {
		item.Detail = "uptime.none"
		return item
	}

	down := 0
	for _, entry := range monitors {
		if entry.Enabled && entry.Status == "down" {
			down++
		}
	}
	item.Params = map[string]any{"count": down}
	if down > 0 {
		item.Level, item.Detail = HealthCritical, "uptime.down"
	}
	return item
}

// securityItems xem các thiết lập bảo vệ chính panel.
func securityItems(tls bool, blockThreshold int, admins []model.User, err error) []HealthItem {
	https := HealthItem{Key: "https", Group: "panel", Level: HealthOK, Detail: "https.on", Route: "settings"}
	if !tls {
		// Không có HTTPS nghĩa là mật khẩu quản trị và token đi qua mạng dưới dạng
		// chữ thường; trên một máy chủ công cộng đây là lỗ hổng lớn nhất còn lại.
		https.Level, https.Detail = HealthWarn, "https.off"
	}

	guard := HealthItem{Key: "loginGuard", Group: "panel", Level: HealthOK, Detail: "loginGuard.on", Route: "security"}
	if blockThreshold <= 0 {
		guard.Level, guard.Detail = HealthWarn, "loginGuard.off"
	}

	totp := HealthItem{Key: "totp", Group: "panel", Level: HealthOK, Detail: "totp.on", Route: "users"}
	if err != nil {
		totp.Level, totp.Detail = HealthWarn, "totp.unreadable"
	} else {
		missing := 0
		for _, user := range admins {
			if !user.TOTPEnabled {
				missing++
			}
		}
		totp.Params = map[string]any{"count": missing}
		if missing > 0 {
			totp.Level, totp.Detail = HealthWarn, "totp.off"
		}
	}

	return []HealthItem{https, guard, totp}
}

// firewallItem xem tường lửa có đang bật không.
func firewallItem(status firewall.Status, err error) HealthItem {
	item := HealthItem{Key: "firewall", Group: "panel", Level: HealthOK, Detail: "firewall.on", Route: "firewall"}

	switch {
	case err != nil || !status.Available:
		// Không có công cụ tường lửa không phải lỗi của người dùng — nhiều nhà
		// cung cấp lọc gói tin ở lớp ngoài — nên đây chỉ là nhắc, không phải trừ
		// điểm nặng.
		item.Level, item.Detail = HealthWarn, "firewall.missing"
	case !status.Enabled:
		item.Level, item.Detail = HealthWarn, "firewall.off"
		item.Params = map[string]any{"backend": status.Backend}
	default:
		item.Params = map[string]any{"backend": status.Backend}
	}
	return item
}

// round1 làm tròn tới một chữ số thập phân.
func round1(value float64) float64 {
	return float64(int(value*10+0.5)) / 10
}
