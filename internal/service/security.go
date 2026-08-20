package service

import (
	"context"
	"net"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/model"
	"github.com/thanhtinz/sunpanel/pkg/loginguard"
)

// offenderLimit là số địa chỉ thử sai nhiều nhất được liệt kê.
const offenderLimit = 20

// SecurityOverview là những gì trang bảo mật cần để vẽ một lần.
type SecurityOverview struct {
	// Enabled cho biết lớp chặn địa chỉ có đang bật không.
	Enabled bool `json:"enabled"`
	// Threshold, WindowSeconds và DurationSeconds là tham số đang áp dụng.
	Threshold       int `json:"threshold"`
	WindowSeconds   int `json:"windowSeconds"`
	DurationSeconds int `json:"durationSeconds"`
	// Blocks là các địa chỉ đang bị chặn.
	Blocks []loginguard.Block `json:"blocks"`
	// Offenders là các địa chỉ thử sai nhiều nhất trong 24 giờ qua.
	//
	// Danh sách này đọc từ nhật ký đăng nhập chứ không từ bộ đếm trong bộ nhớ:
	// nó cho thấy cả những địa chỉ đã bị chặn rồi bỏ cuộc, tức là phần lịch sử
	// mà bộ đếm đã quên.
	Offenders []Offender `json:"offenders"`
	// FailedLastDay là số lần đăng nhập hỏng trong 24 giờ qua.
	FailedLastDay int64 `json:"failedLastDay"`
}

// Offender là một địa chỉ có nhiều lần đăng nhập hỏng.
type Offender struct {
	IP       string    `json:"ip"`
	Failures int64     `json:"failures"`
	LastUser string    `json:"lastUser"`
	LastAt   time.Time `json:"lastAt"`
	// Blocked cho biết địa chỉ này có đang bị chặn không.
	Blocked bool `json:"blocked"`
}

// SecurityService đọc trạng thái phòng thủ đăng nhập của panel.
type SecurityService struct {
	db    *gorm.DB
	guard *loginguard.Guard
	cfg   securityLimits
	audit *AuditService
}

// securityLimits là các tham số chặn, giữ lại để hiển thị.
type securityLimits struct {
	threshold int
	window    int
	duration  int
}

// NewSecurityService tạo dịch vụ bảo mật đăng nhập.
func NewSecurityService(db *gorm.DB, auth *AuthService, threshold, windowSeconds, durationSeconds int, audit *AuditService) *SecurityService {
	return &SecurityService{
		db:    db,
		guard: auth.Guard(),
		cfg:   securityLimits{threshold: threshold, window: windowSeconds, duration: durationSeconds},
		audit: audit,
	}
}

// Overview gom danh sách chặn và nhật ký đăng nhập gần nhất.
func (s *SecurityService) Overview(ctx context.Context) (SecurityOverview, error) {
	out := SecurityOverview{
		Enabled:         s.guard.Enabled(),
		Threshold:       s.cfg.threshold,
		WindowSeconds:   s.cfg.window,
		DurationSeconds: s.cfg.duration,
		Blocks:          s.guard.List(),
		Offenders:       []Offender{},
	}

	since := time.Now().Add(-24 * time.Hour)

	err := s.db.WithContext(ctx).Model(&model.LoginLog{}).
		Where("success = ? AND created_at > ?", false, since).
		Count(&out.FailedLastDay).Error
	if err != nil {
		return SecurityOverview{}, apperr.Internal.Wrap(err)
	}

	err = s.db.WithContext(ctx).Model(&model.LoginLog{}).
		Select("ip, COUNT(*) AS failures").
		Where("success = ? AND created_at > ? AND ip <> ''", false, since).
		Group("ip").Order("failures DESC").Limit(offenderLimit).
		Scan(&out.Offenders).Error
	if err != nil {
		return SecurityOverview{}, apperr.Internal.Wrap(err)
	}

	for i := range out.Offenders {
		// Thời điểm và tên đăng nhập của lần thử gần nhất lấy từ chính bản ghi
		// đó thay vì từ một hàm gộp: SQLite trả kết quả của MAX() dưới dạng
		// chuỗi, và chuỗi đó không đọc thẳng vào một trường thời gian được.
		var last model.LoginLog
		err := s.db.WithContext(ctx).
			Where("ip = ? AND success = ?", out.Offenders[i].IP, false).
			Order("created_at DESC").First(&last).Error
		if err == nil {
			out.Offenders[i].LastUser = last.Username
			out.Offenders[i].LastAt = last.CreatedAt
		}
		_, out.Offenders[i].Blocked = s.guard.Blocked(out.Offenders[i].IP)
	}

	return out, nil
}

// Unblock gỡ chặn một địa chỉ.
func (s *SecurityService) Unblock(ctx context.Context, ip string, actor AuditEntry) error {
	ip = strings.TrimSpace(ip)
	if net.ParseIP(ip) == nil {
		return apperr.BadRequest
	}

	// Gỡ một địa chỉ không còn bị chặn không phải lỗi của người bấm: danh sách
	// trên màn hình luôn trễ hơn thực tế vài giây vì lệnh chặn tự hết hạn.
	s.guard.Unblock(ip)

	actor.Action = "security.unblock"
	actor.Resource = ip
	actor.Success = true
	s.audit.Record(ctx, actor)
	return nil
}
