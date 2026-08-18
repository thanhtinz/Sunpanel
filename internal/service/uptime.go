package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/model"
	"github.com/thanhtinz/sunpanel/pkg/notify"
	"github.com/thanhtinz/sunpanel/pkg/uptime"
)

// uptimeTick là nhịp của vòng lặp theo dõi.
//
// Vòng lặp chỉ chạy những mục đã tới hạn, nên nhịp này chỉ quyết định độ chính
// xác của chu kỳ chứ không phải khối lượng công việc: mười giây là đủ mịn cho
// chu kỳ ngắn nhất mà panel cho đặt (30 giây).
const uptimeTick = 10 * time.Second

// uptimeRetention là thời gian giữ lịch sử kiểm tra.
const uptimeRetention = 30 * 24 * time.Hour

// Các giới hạn của một mục theo dõi.
const (
	minUptimeInterval = 30
	maxUptimeInterval = 86400
	minUptimeTimeout  = 1
	maxUptimeTimeout  = 120
)

// MonitorRequest là yêu cầu tạo hoặc sửa một mục theo dõi.
type MonitorRequest struct {
	Name             string `json:"name" binding:"required"`
	URL              string `json:"url" binding:"required"`
	IntervalSeconds  int    `json:"intervalSeconds"`
	TimeoutSeconds   int    `json:"timeoutSeconds"`
	ExpectedStatus   int    `json:"expectedStatus"`
	Keyword          string `json:"keyword"`
	SkipTLSVerify    bool   `json:"skipTlsVerify"`
	FailureThreshold int    `json:"failureThreshold"`
	Enabled          bool   `json:"enabled"`
}

// MonitorSummary là một mục theo dõi kèm số liệu tổng hợp.
type MonitorSummary struct {
	model.UptimeMonitor
	// Uptime24h là tỉ lệ phần trăm lần kiểm tra thành công trong 24 giờ qua.
	Uptime24h float64 `json:"uptime24h"`
	// AvgLatencyMs là độ trễ trung bình trong 24 giờ qua.
	AvgLatencyMs int64 `json:"avgLatencyMs"`
	// Checks là số lần kiểm tra dùng để tính hai con số trên.
	Checks int64 `json:"checks"`
}

// UptimeService theo dõi các địa chỉ HTTP và báo khi chúng đổi trạng thái.
type UptimeService struct {
	db      *gorm.DB
	checker *uptime.Checker
	alerts  *AlertService
	audit   *AuditService
}

// NewUptimeService tạo dịch vụ theo dõi.
func NewUptimeService(db *gorm.DB, alerts *AlertService, audit *AuditService) *UptimeService {
	return &UptimeService{db: db, checker: uptime.NewChecker(), alerts: alerts, audit: audit}
}

// Run chạy vòng theo dõi cho tới khi ctx bị hủy.
func (s *UptimeService) Run(ctx context.Context) {
	ticker := time.NewTicker(uptimeTick)
	defer ticker.Stop()

	cleanup := time.NewTicker(6 * time.Hour)
	defer cleanup.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runDue(ctx)
		case <-cleanup.C:
			s.prune(ctx)
		}
	}
}

// runDue kiểm tra những mục đã tới hạn.
func (s *UptimeService) runDue(ctx context.Context) {
	var monitors []model.UptimeMonitor
	if err := s.db.WithContext(ctx).Where("enabled = ?", true).Find(&monitors).Error; err != nil {
		return
	}

	now := time.Now()
	for _, monitor := range monitors {
		if monitor.LastCheckedAt != nil {
			due := monitor.LastCheckedAt.Add(time.Duration(monitor.IntervalSeconds) * time.Second)
			if now.Before(due) {
				continue
			}
		}
		if _, err := s.check(ctx, monitor); err != nil {
			return
		}
	}
}

// List trả về các mục theo dõi kèm số liệu 24 giờ qua.
func (s *UptimeService) List(ctx context.Context) ([]MonitorSummary, error) {
	var monitors []model.UptimeMonitor
	if err := s.db.WithContext(ctx).Order("name").Find(&monitors).Error; err != nil {
		return nil, apperr.Internal.Wrap(err)
	}

	since := time.Now().Add(-24 * time.Hour)
	out := make([]MonitorSummary, 0, len(monitors))

	for _, monitor := range monitors {
		summary := MonitorSummary{UptimeMonitor: monitor}

		var stats struct {
			Total   int64
			UpCount int64
			AvgMs   float64
		}
		err := s.db.WithContext(ctx).Model(&model.UptimeCheck{}).
			Select("COUNT(*) AS total, SUM(CASE WHEN up THEN 1 ELSE 0 END) AS up_count, AVG(latency_ms) AS avg_ms").
			Where("monitor_id = ? AND checked_at >= ?", monitor.ID, since).
			Scan(&stats).Error
		if err != nil {
			return nil, apperr.Internal.Wrap(err)
		}

		summary.Checks = stats.Total
		summary.AvgLatencyMs = int64(stats.AvgMs)
		if stats.Total > 0 {
			summary.Uptime24h = float64(stats.UpCount) * 100 / float64(stats.Total)
		}
		out = append(out, summary)
	}
	return out, nil
}

// History trả về các lần kiểm tra gần nhất của một mục.
func (s *UptimeService) History(ctx context.Context, id uint, limit int) ([]model.UptimeCheck, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	var checks []model.UptimeCheck
	err := s.db.WithContext(ctx).Where("monitor_id = ?", id).
		Order("checked_at DESC").Limit(limit).Find(&checks).Error
	if err != nil {
		return nil, apperr.Internal.Wrap(err)
	}

	// Đảo lại theo thứ tự thời gian để giao diện vẽ từ trái sang phải.
	for i, j := 0, len(checks)-1; i < j; i, j = i+1, j-1 {
		checks[i], checks[j] = checks[j], checks[i]
	}
	return checks, nil
}

// Create thêm một mục theo dõi và kiểm tra nó ngay.
func (s *UptimeService) Create(ctx context.Context, req MonitorRequest, actor AuditEntry) (model.UptimeMonitor, error) {
	monitor, err := s.build(model.UptimeMonitor{}, req)
	if err != nil {
		return model.UptimeMonitor{}, err
	}

	if err := s.db.WithContext(ctx).Create(&monitor).Error; err != nil {
		return model.UptimeMonitor{}, translateUptimeError(err)
	}

	actor.Action, actor.Resource, actor.Success = "uptime.create", monitor.Name, true
	s.audit.Record(ctx, actor)

	// Kiểm ngay để người dùng thấy kết quả thay vì một dấu hỏi cho tới chu kỳ sau.
	if monitor.Enabled {
		if checked, err := s.check(ctx, monitor); err == nil {
			return checked, nil
		}
	}
	return monitor, nil
}

// Update sửa một mục theo dõi.
func (s *UptimeService) Update(ctx context.Context, id uint, req MonitorRequest, actor AuditEntry) (model.UptimeMonitor, error) {
	current, err := s.find(ctx, id)
	if err != nil {
		return model.UptimeMonitor{}, err
	}

	monitor, err := s.build(current, req)
	if err != nil {
		return model.UptimeMonitor{}, err
	}

	if err := s.db.WithContext(ctx).Save(&monitor).Error; err != nil {
		return model.UptimeMonitor{}, translateUptimeError(err)
	}

	actor.Action, actor.Resource, actor.Success = "uptime.update", monitor.Name, true
	s.audit.Record(ctx, actor)
	return monitor, nil
}

// Delete xóa một mục theo dõi cùng toàn bộ lịch sử của nó.
func (s *UptimeService) Delete(ctx context.Context, id uint, actor AuditEntry) error {
	monitor, err := s.find(ctx, id)
	if err != nil {
		return err
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("monitor_id = ?", id).Delete(&model.UptimeCheck{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.UptimeMonitor{}, id).Error
	})
	if err != nil {
		return apperr.Internal.Wrap(err)
	}

	actor.Action, actor.Resource, actor.Success = "uptime.delete", monitor.Name, true
	s.audit.Record(ctx, actor)
	return nil
}

// CheckNow kiểm tra một mục ngay lập tức.
func (s *UptimeService) CheckNow(ctx context.Context, id uint) (model.UptimeMonitor, error) {
	monitor, err := s.find(ctx, id)
	if err != nil {
		return model.UptimeMonitor{}, err
	}
	return s.check(ctx, monitor)
}

// check thực hiện một lần kiểm tra, lưu kết quả và báo nếu trạng thái đổi.
func (s *UptimeService) check(ctx context.Context, monitor model.UptimeMonitor) (model.UptimeMonitor, error) {
	result := s.checker.Check(ctx, uptime.Target{
		URL:            monitor.URL,
		Timeout:        time.Duration(monitor.TimeoutSeconds) * time.Second,
		ExpectedStatus: monitor.ExpectedStatus,
		Keyword:        monitor.Keyword,
		SkipTLSVerify:  monitor.SkipTLSVerify,
	})

	now := time.Now()
	previous := monitor.Status

	if result.Up {
		monitor.ConsecutiveFails = 0
		monitor.Status = "up"
	} else {
		monitor.ConsecutiveFails++
		// Chỉ đổi sang "mất kết nối" khi hỏng đủ số lần liên tiếp; trước đó vẫn
		// giữ trạng thái cũ để một lần rớt gói tin không đánh thức ai.
		if monitor.ConsecutiveFails >= monitor.FailureThreshold {
			monitor.Status = "down"
		} else if monitor.Status == "" {
			monitor.Status = "unknown"
		}
	}

	monitor.LastCheckedAt = &now
	monitor.LastLatencyMs = result.LatencyMs
	monitor.LastStatusCode = result.StatusCode
	monitor.LastError = result.Error
	monitor.CertExpiresIn = result.CertExpiresIn
	if monitor.Status != previous {
		monitor.LastChangedAt = &now
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&monitor).Error; err != nil {
			return err
		}
		return tx.Create(&model.UptimeCheck{
			MonitorID:  monitor.ID,
			CheckedAt:  now,
			Up:         result.Up,
			StatusCode: result.StatusCode,
			LatencyMs:  result.LatencyMs,
			Error:      result.Error,
		}).Error
	})
	if err != nil {
		return monitor, apperr.Internal.Wrap(err)
	}

	if monitor.Status != previous && previous != "" {
		s.announce(ctx, monitor, result)
	}
	return monitor, nil
}

// announce gửi thông báo khi một mục đổi trạng thái.
//
// Chỉ gửi lúc đổi trạng thái, không gửi mỗi lần kiểm tra: một website hỏng cả
// đêm sẽ sinh ra hàng trăm tin nhắn giống hệt nhau và người nhận sẽ tắt kênh.
func (s *UptimeService) announce(ctx context.Context, monitor model.UptimeMonitor, result uptime.Result) {
	if s.alerts == nil {
		return
	}

	message := notify.Message{
		Title: fmt.Sprintf("[SunPanel] %s đã trở lại", monitor.Name),
		Body:  fmt.Sprintf("%s trả lời bình thường sau %d ms.", monitor.URL, result.LatencyMs),
		Level: "info",
	}
	if monitor.Status == "down" {
		message = notify.Message{
			Title: fmt.Sprintf("[SunPanel] %s không truy cập được", monitor.Name),
			Body:  fmt.Sprintf("%s hỏng %d lần liên tiếp: %s", monitor.URL, monitor.ConsecutiveFails, result.Error),
			Level: "critical",
		}
	}
	s.alerts.Notify(ctx, "uptime", message)
}

// build kiểm tra và áp yêu cầu lên bản ghi.
func (s *UptimeService) build(base model.UptimeMonitor, req MonitorRequest) (model.UptimeMonitor, error) {
	name := strings.TrimSpace(req.Name)
	url := strings.TrimSpace(req.URL)

	if name == "" {
		return model.UptimeMonitor{}, apperr.UptimeInvalidName
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return model.UptimeMonitor{}, apperr.UptimeInvalidURL
	}

	monitor := base
	monitor.Name = name
	monitor.URL = url
	monitor.IntervalSeconds = clampInt(req.IntervalSeconds, minUptimeInterval, maxUptimeInterval, 60)
	monitor.TimeoutSeconds = clampInt(req.TimeoutSeconds, minUptimeTimeout, maxUptimeTimeout, 10)
	monitor.ExpectedStatus = req.ExpectedStatus
	monitor.Keyword = strings.TrimSpace(req.Keyword)
	monitor.SkipTLSVerify = req.SkipTLSVerify
	monitor.FailureThreshold = clampInt(req.FailureThreshold, 1, 10, 2)
	monitor.Enabled = req.Enabled

	if monitor.Status == "" {
		monitor.Status = "unknown"
	}
	if monitor.ExpectedStatus != 0 && (monitor.ExpectedStatus < 100 || monitor.ExpectedStatus > 599) {
		return model.UptimeMonitor{}, apperr.UptimeInvalidStatus
	}
	return monitor, nil
}

func (s *UptimeService) find(ctx context.Context, id uint) (model.UptimeMonitor, error) {
	var monitor model.UptimeMonitor
	err := s.db.WithContext(ctx).First(&monitor, id).Error
	switch {
	case err == nil:
		return monitor, nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		return model.UptimeMonitor{}, apperr.UptimeNotFound
	default:
		return model.UptimeMonitor{}, apperr.Internal.Wrap(err)
	}
}

// prune xóa lịch sử quá cũ để cơ sở dữ liệu không phình vô hạn.
func (s *UptimeService) prune(ctx context.Context) {
	cutoff := time.Now().Add(-uptimeRetention)
	s.db.WithContext(ctx).Where("checked_at < ?", cutoff).Delete(&model.UptimeCheck{})
}

// clampInt ép một giá trị vào khoảng cho phép, dùng mặc định khi bằng 0.
func clampInt(value, min, max, fallback int) int {
	if value == 0 {
		return fallback
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func translateUptimeError(err error) error {
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "unique") {
		return apperr.UptimeNameExists
	}
	return apperr.Internal.Wrap(err)
}
