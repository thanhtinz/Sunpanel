package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/model"
	"github.com/thanhtinz/sunpanel/pkg/crypto"
	"github.com/thanhtinz/sunpanel/pkg/monitor"
	"github.com/thanhtinz/sunpanel/pkg/notify"
)

// Các số liệu mà quy tắc cảnh báo theo dõi được.
const (
	MetricCPU    = "cpu"
	MetricMemory = "memory"
	MetricDisk   = "disk"
	MetricLoad   = "load"
)

// alertCheckInterval là chu kỳ đối chiếu số liệu với các quy tắc.
//
// Một phút: đủ nhanh để biết sớm, đủ chậm để không đối chiếu lại cùng một con số
// hàng chục lần giữa hai lần lấy mẫu.
const alertCheckInterval = time.Minute

// alertEventRetention là thời gian giữ lịch sử cảnh báo.
const alertEventRetention = 90 * 24 * time.Hour

// ChannelRequest là dữ liệu tạo hoặc sửa một kênh thông báo.
type ChannelRequest struct {
	Name string `json:"name" binding:"required"`
	Kind string `json:"kind" binding:"required"`
	// Config để trống khi sửa nghĩa là giữ nguyên cấu hình cũ.
	Config  map[string]string `json:"config"`
	Enabled bool              `json:"enabled"`
}

// RuleRequest là dữ liệu tạo hoặc sửa một quy tắc cảnh báo.
type RuleRequest struct {
	Name            string  `json:"name" binding:"required"`
	Metric          string  `json:"metric" binding:"required"`
	Threshold       float64 `json:"threshold"`
	DurationMinutes int     `json:"durationMinutes"`
	CooldownMinutes int     `json:"cooldownMinutes"`
	Level           string  `json:"level"`
	Channels        string  `json:"channels"`
	Enabled         bool    `json:"enabled"`
}

// AlertService quản lý kênh thông báo, quy tắc cảnh báo và việc gửi đi.
type AlertService struct {
	db      *gorm.DB
	sealer  *crypto.Sealer
	monitor *MonitorService
	audit   *AuditService
	// hostname đi kèm mọi thông báo để người nhận biết cảnh báo từ máy nào.
	hostname string
	mu       sync.Mutex
}

// NewAlertService tạo dịch vụ cảnh báo.
func NewAlertService(
	db *gorm.DB, sealer *crypto.Sealer, monitorSvc *MonitorService, audit *AuditService,
) *AlertService {
	name, err := os.Hostname()
	if err != nil || name == "" {
		name = "khong-ro"
	}
	return &AlertService{db: db, sealer: sealer, monitor: monitorSvc, audit: audit, hostname: name}
}

// Run chạy vòng lặp đối chiếu quy tắc tới khi ctx bị hủy.
func (s *AlertService) Run(ctx context.Context) {
	ticker := time.NewTicker(alertCheckInterval)
	defer ticker.Stop()

	cleanup := time.NewTicker(24 * time.Hour)
	defer cleanup.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.evaluate(ctx)
		case <-cleanup.C:
			cutoff := time.Now().Add(-alertEventRetention)
			if err := s.db.Where("created_at < ?", cutoff).Delete(&model.AlertEvent{}).Error; err != nil {
				slog.Warn("không dọn được lịch sử cảnh báo", "error", err)
			}
		}
	}
}

// ListChannels liệt kê các kênh thông báo.
func (s *AlertService) ListChannels(ctx context.Context) ([]model.NotifyChannel, error) {
	var channels []model.NotifyChannel
	if err := s.db.WithContext(ctx).Order("name").Find(&channels).Error; err != nil {
		return nil, apperr.Internal.Wrap(err)
	}
	return channels, nil
}

// CreateChannel thêm một kênh thông báo.
func (s *AlertService) CreateChannel(ctx context.Context, req ChannelRequest) (model.NotifyChannel, error) {
	channel, err := s.buildChannel(ctx, model.NotifyChannel{}, req, 0)
	if err != nil {
		return model.NotifyChannel{}, err
	}
	if err := s.db.WithContext(ctx).Create(&channel).Error; err != nil {
		return model.NotifyChannel{}, apperr.Internal.Wrap(err)
	}
	return channel, nil
}

// UpdateChannel sửa một kênh thông báo.
func (s *AlertService) UpdateChannel(ctx context.Context, id uint, req ChannelRequest) (model.NotifyChannel, error) {
	existing, err := s.findChannel(ctx, id)
	if err != nil {
		return model.NotifyChannel{}, err
	}

	channel, err := s.buildChannel(ctx, existing, req, id)
	if err != nil {
		return model.NotifyChannel{}, err
	}
	if err := s.db.WithContext(ctx).Save(&channel).Error; err != nil {
		return model.NotifyChannel{}, apperr.Internal.Wrap(err)
	}
	return channel, nil
}

func (s *AlertService) buildChannel(
	ctx context.Context, base model.NotifyChannel, req ChannelRequest, selfID uint,
) (model.NotifyChannel, error) {
	kind := notify.Kind(strings.TrimSpace(req.Kind))
	if !kind.Valid() {
		return model.NotifyChannel{}, apperr.AlertInvalidKind.WithParam("kind", req.Kind)
	}

	name := strings.TrimSpace(req.Name)
	var count int64
	err := s.db.WithContext(ctx).Model(&model.NotifyChannel{}).
		Where("name = ? AND id <> ?", name, selfID).Count(&count).Error
	if err != nil {
		return model.NotifyChannel{}, apperr.Internal.Wrap(err)
	}
	if count > 0 {
		return model.NotifyChannel{}, apperr.AlertChannelNameExists.WithParam("name", name)
	}

	channel := base
	channel.Name = name
	channel.Kind = string(kind)
	channel.Enabled = req.Enabled

	if len(req.Config) > 0 {
		// Dựng thử kênh ngay: cấu hình thiếu trường phải lộ ra lúc lưu, chứ không
		// phải lúc có sự cố và cảnh báo không gửi được.
		if _, err := notify.New(kind, req.Config); err != nil {
			return model.NotifyChannel{}, apperr.AlertInvalidConfig.WithParam("message", err.Error())
		}

		encoded, err := json.Marshal(req.Config)
		if err != nil {
			return model.NotifyChannel{}, apperr.Internal.Wrap(err)
		}
		sealed, err := s.sealer.Seal(string(encoded))
		if err != nil {
			return model.NotifyChannel{}, apperr.Internal.Wrap(err)
		}
		channel.Config = sealed
	}
	if channel.Config == "" {
		return model.NotifyChannel{}, apperr.AlertInvalidConfig.WithParam("field", "config")
	}

	return channel, nil
}

func (s *AlertService) findChannel(ctx context.Context, id uint) (model.NotifyChannel, error) {
	var channel model.NotifyChannel
	err := s.db.WithContext(ctx).First(&channel, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.NotifyChannel{}, apperr.AlertChannelNotFound
	}
	if err != nil {
		return model.NotifyChannel{}, apperr.Internal.Wrap(err)
	}
	return channel, nil
}

// DeleteChannel xóa một kênh thông báo.
func (s *AlertService) DeleteChannel(ctx context.Context, id uint) error {
	channel, err := s.findChannel(ctx, id)
	if err != nil {
		return err
	}
	if err := s.db.WithContext(ctx).Delete(&channel).Error; err != nil {
		return apperr.Internal.Wrap(err)
	}
	return nil
}

// channelFor dựng kênh gửi từ một bản ghi.
func (s *AlertService) channelFor(record model.NotifyChannel) (notify.Channel, error) {
	opened, err := s.sealer.Open(record.Config)
	if err != nil {
		return nil, apperr.Internal.Wrap(err)
	}
	var config map[string]string
	if err := json.Unmarshal([]byte(opened), &config); err != nil {
		return nil, apperr.Internal.Wrap(err)
	}
	return notify.New(notify.Kind(record.Kind), config)
}

// TestChannel gửi một thông báo thử tới một kênh.
func (s *AlertService) TestChannel(ctx context.Context, id uint) error {
	record, err := s.findChannel(ctx, id)
	if err != nil {
		return err
	}

	channel, err := s.channelFor(record)
	if err != nil {
		return apperr.AlertInvalidConfig.WithParam("message", err.Error())
	}

	message := notify.Message{
		Title:    "Thông báo thử từ SunPanel",
		Body:     "Nếu bạn nhận được tin này, kênh thông báo đã hoạt động.",
		Level:    notify.LevelInfo,
		Hostname: s.hostname,
		At:       time.Now(),
		Fields:   map[string]string{"Kênh": record.Name},
	}

	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	if err := channel.Send(ctx, message); err != nil {
		s.recordChannelResult(record, err)
		return apperr.AlertSendFailed.WithParam("message", trimMessage(err.Error()))
	}
	s.recordChannelResult(record, nil)
	return nil
}

// recordChannelResult lưu kết quả lần gửi gần nhất của một kênh.
//
// Một kênh hỏng mà không ai biết cũng tệ như không có cảnh báo, nên trạng thái
// này luôn hiển thị trong danh sách kênh.
func (s *AlertService) recordChannelResult(record model.NotifyChannel, cause error) {
	now := time.Now()
	updates := map[string]any{"last_sent_at": now, "last_error": ""}
	if cause != nil {
		updates["last_error"] = trimMessage(cause.Error())
	}
	if err := s.db.Model(&model.NotifyChannel{}).Where("id = ?", record.ID).Updates(updates).Error; err != nil {
		slog.Warn("không ghi được trạng thái kênh thông báo", "channel", record.Name, "error", err)
	}
}

// Send gửi một thông báo tới các kênh chỉ định.
//
// channels để trống nghĩa là gửi tới mọi kênh đang bật: một cảnh báo không tới
// được ai là cảnh báo vô dụng, nên mặc định phải là gửi rộng chứ không im lặng.
func (s *AlertService) Send(ctx context.Context, message notify.Message, channels string, source string, ruleID uint) {
	if message.Hostname == "" {
		message.Hostname = s.hostname
	}
	if message.At.IsZero() {
		message.At = time.Now()
	}

	var records []model.NotifyChannel
	query := s.db.WithContext(ctx).Where("enabled = ?", true)
	if names := splitList(channels); len(names) > 0 {
		query = query.Where("name IN ?", names)
	}
	if err := query.Find(&records).Error; err != nil {
		slog.Error("không đọc được danh sách kênh thông báo", "error", err)
		return
	}

	event := model.AlertEvent{
		Source: source, RuleID: ruleID,
		Title: message.Title, Body: message.Body, Level: string(message.Level),
	}

	var failures []string
	for _, record := range records {
		channel, err := s.channelFor(record)
		if err != nil {
			failures = append(failures, record.Name+": "+err.Error())
			event.Failed++
			continue
		}

		sendCtx, cancel := context.WithTimeout(ctx, time.Minute)
		err = channel.Send(sendCtx, message)
		cancel()

		s.recordChannelResult(record, err)
		if err != nil {
			// Một kênh hỏng không được ngăn các kênh còn lại nhận cảnh báo.
			failures = append(failures, record.Name+": "+err.Error())
			event.Failed++
			continue
		}
		event.Delivered++
	}

	if len(records) == 0 {
		failures = append(failures, "không có kênh thông báo nào đang bật")
	}
	event.Error = trimMessage(strings.Join(failures, "; "))

	if err := s.db.WithContext(ctx).Create(&event).Error; err != nil {
		slog.Error("không ghi được lịch sử cảnh báo", "error", err)
	}
	if event.Failed > 0 {
		slog.Warn("một số kênh thông báo gửi thất bại", "title", message.Title, "error", event.Error)
	}
}

// Notify là lối vào cho các dịch vụ khác phát cảnh báo.
func (s *AlertService) Notify(ctx context.Context, source string, message notify.Message) {
	s.Send(ctx, message, "", source, 0)
}

// Events đọc lịch sử cảnh báo.
func (s *AlertService) Events(ctx context.Context, limit int) ([]model.AlertEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	var events []model.AlertEvent
	err := s.db.WithContext(ctx).Order("created_at DESC").Limit(limit).Find(&events).Error
	if err != nil {
		return nil, apperr.Internal.Wrap(err)
	}
	return events, nil
}

// ListRules liệt kê các quy tắc cảnh báo.
func (s *AlertService) ListRules(ctx context.Context) ([]model.AlertRule, error) {
	var rules []model.AlertRule
	if err := s.db.WithContext(ctx).Order("name").Find(&rules).Error; err != nil {
		return nil, apperr.Internal.Wrap(err)
	}
	return rules, nil
}

// CreateRule tạo một quy tắc cảnh báo.
func (s *AlertService) CreateRule(ctx context.Context, req RuleRequest) (model.AlertRule, error) {
	rule, err := s.buildRule(ctx, model.AlertRule{}, req, 0)
	if err != nil {
		return model.AlertRule{}, err
	}
	if err := s.db.WithContext(ctx).Create(&rule).Error; err != nil {
		return model.AlertRule{}, apperr.Internal.Wrap(err)
	}
	return rule, nil
}

// UpdateRule sửa một quy tắc cảnh báo.
func (s *AlertService) UpdateRule(ctx context.Context, id uint, req RuleRequest) (model.AlertRule, error) {
	existing, err := s.findRule(ctx, id)
	if err != nil {
		return model.AlertRule{}, err
	}

	rule, err := s.buildRule(ctx, existing, req, id)
	if err != nil {
		return model.AlertRule{}, err
	}
	// Đổi quy tắc thì trạng thái vượt ngưỡng cũ không còn nghĩa gì.
	rule.BreachedSince = nil
	if err := s.db.WithContext(ctx).Save(&rule).Error; err != nil {
		return model.AlertRule{}, apperr.Internal.Wrap(err)
	}
	return rule, nil
}

func (s *AlertService) buildRule(
	ctx context.Context, base model.AlertRule, req RuleRequest, selfID uint,
) (model.AlertRule, error) {
	switch req.Metric {
	case MetricCPU, MetricMemory, MetricDisk, MetricLoad:
	default:
		return model.AlertRule{}, apperr.AlertInvalidConfig.WithParam("field", "metric")
	}
	if req.Threshold <= 0 {
		return model.AlertRule{}, apperr.AlertInvalidConfig.WithParam("field", "threshold")
	}

	name := strings.TrimSpace(req.Name)
	var count int64
	err := s.db.WithContext(ctx).Model(&model.AlertRule{}).
		Where("name = ? AND id <> ?", name, selfID).Count(&count).Error
	if err != nil {
		return model.AlertRule{}, apperr.Internal.Wrap(err)
	}
	if count > 0 {
		return model.AlertRule{}, apperr.AlertRuleNameExists.WithParam("name", name)
	}

	level := notify.Level(req.Level)
	if level != notify.LevelInfo && level != notify.LevelWarning && level != notify.LevelCritical {
		level = notify.LevelWarning
	}

	rule := base
	rule.Name = name
	rule.Metric = req.Metric
	rule.Threshold = req.Threshold
	rule.DurationMinutes = req.DurationMinutes
	if rule.DurationMinutes < 0 {
		rule.DurationMinutes = 0
	}
	rule.CooldownMinutes = req.CooldownMinutes
	if rule.CooldownMinutes <= 0 {
		rule.CooldownMinutes = 60
	}
	rule.Level = string(level)
	rule.Channels = strings.TrimSpace(req.Channels)
	rule.Enabled = req.Enabled
	return rule, nil
}

func (s *AlertService) findRule(ctx context.Context, id uint) (model.AlertRule, error) {
	var rule model.AlertRule
	err := s.db.WithContext(ctx).First(&rule, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.AlertRule{}, apperr.AlertRuleNotFound
	}
	if err != nil {
		return model.AlertRule{}, apperr.Internal.Wrap(err)
	}
	return rule, nil
}

// DeleteRule xóa một quy tắc cảnh báo.
func (s *AlertService) DeleteRule(ctx context.Context, id uint) error {
	rule, err := s.findRule(ctx, id)
	if err != nil {
		return err
	}
	if err := s.db.WithContext(ctx).Delete(&rule).Error; err != nil {
		return apperr.Internal.Wrap(err)
	}
	return nil
}

// evaluate đối chiếu số liệu hiện tại với mọi quy tắc đang bật.
func (s *AlertService) evaluate(ctx context.Context) {
	// Chỉ một lượt đối chiếu chạy tại một thời điểm: hai lượt song song sẽ cùng
	// thấy quy tắc chưa phát và gửi cảnh báo trùng.
	s.mu.Lock()
	defer s.mu.Unlock()

	var rules []model.AlertRule
	if err := s.db.WithContext(ctx).Where("enabled = ?", true).Find(&rules).Error; err != nil {
		slog.Error("không đọc được quy tắc cảnh báo", "error", err)
		return
	}
	if len(rules) == 0 {
		return
	}

	snapshot := s.monitor.Latest()
	now := time.Now()
	for i := range rules {
		s.evaluateRule(ctx, &rules[i], snapshot, now)
	}
}

// evaluateRule đối chiếu một quy tắc với số liệu hiện tại.
func (s *AlertService) evaluateRule(
	ctx context.Context, rule *model.AlertRule, snapshot monitor.Snapshot, now time.Time,
) {
	value, label := metricValue(rule.Metric, snapshot)

	if value < rule.Threshold {
		// Số liệu đã về dưới ngưỡng: xóa mốc bắt đầu vượt để lần sau phải vượt
		// liên tục đủ lâu từ đầu.
		if rule.BreachedSince != nil {
			rule.BreachedSince = nil
			s.db.WithContext(ctx).Model(rule).Update("breached_since", nil)
		}
		return
	}

	if rule.BreachedSince == nil {
		started := now
		rule.BreachedSince = &started
		s.db.WithContext(ctx).Model(rule).Update("breached_since", started)
	}

	// Phải vượt ngưỡng liên tục đủ lâu; một lần tăng vọt vài giây không đáng báo.
	if now.Sub(*rule.BreachedSince) < time.Duration(rule.DurationMinutes)*time.Minute {
		return
	}
	// Khoảng nghỉ tránh việc một sự cố kéo dài gửi hàng trăm tin nhắn.
	if rule.LastFiredAt != nil &&
		now.Sub(*rule.LastFiredAt) < time.Duration(rule.CooldownMinutes)*time.Minute {
		return
	}

	message := notify.Message{
		Title: fmt.Sprintf("%s vượt ngưỡng %.0f%s", label, rule.Threshold, metricUnit(rule.Metric)),
		Body: fmt.Sprintf("Giá trị hiện tại là %.1f%s, đã vượt ngưỡng liên tục %d phút.",
			value, metricUnit(rule.Metric), rule.DurationMinutes),
		Level:    notify.Level(rule.Level),
		Hostname: s.hostname,
		At:       now,
		Fields: map[string]string{
			"Quy tắc":  rule.Name,
			"Ngưỡng":   fmt.Sprintf("%.0f%s", rule.Threshold, metricUnit(rule.Metric)),
			"Hiện tại": fmt.Sprintf("%.1f%s", value, metricUnit(rule.Metric)),
		},
	}

	s.Send(ctx, message, rule.Channels, "rule", rule.ID)

	firedAt := now
	rule.LastFiredAt = &firedAt
	s.db.WithContext(ctx).Model(rule).Update("last_fired_at", firedAt)
}

// metricValue lấy giá trị và nhãn của một số liệu.
func metricValue(metric string, snapshot monitor.Snapshot) (float64, string) {
	switch metric {
	case MetricCPU:
		return snapshot.CPUPercent, "CPU"
	case MetricMemory:
		return snapshot.MemoryPercent, "Bộ nhớ"
	case MetricDisk:
		return snapshot.DiskPercent, "Dung lượng đĩa"
	case MetricLoad:
		return snapshot.LoadAvg1, "Tải hệ thống"
	}
	return 0, metric
}

// metricUnit trả về đơn vị hiển thị của một số liệu.
func metricUnit(metric string) string {
	if metric == MetricLoad {
		return ""
	}
	return "%"
}
