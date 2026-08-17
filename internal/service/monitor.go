package service

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/config"
	"github.com/thanhtinz/sunpanel/internal/model"
	"github.com/thanhtinz/sunpanel/pkg/host"
	"github.com/thanhtinz/sunpanel/pkg/monitor"
)

// subscriberBuffer là sức chứa hàng đợi của mỗi kết nối WebSocket.
// Kết nối chậm sẽ bị bỏ mẫu thay vì làm nghẽn bộ thu thập.
const subscriberBuffer = 8

// MonitorService lấy mẫu tài nguyên theo chu kỳ, lưu lịch sử và phát cho các
// kết nối WebSocket đang mở.
type MonitorService struct {
	db        *gorm.DB
	host      host.Host
	collector *monitor.Collector
	cfg       config.MonitorConfig

	subsMu      sync.RWMutex
	subscribers map[chan monitor.Snapshot]struct{}

	latestMu sync.RWMutex
	latest   monitor.Snapshot
}

// NewMonitorService tạo service giám sát.
func NewMonitorService(db *gorm.DB, h host.Host, cfg config.MonitorConfig) *MonitorService {
	return &MonitorService{
		db:          db,
		host:        h,
		collector:   monitor.NewCollector(),
		cfg:         cfg,
		subscribers: make(map[chan monitor.Snapshot]struct{}),
	}
}

// Run chạy vòng lấy mẫu cho tới khi ctx bị hủy. Hàm này chặn, nên gọi trong goroutine riêng.
func (s *MonitorService) Run(ctx context.Context) {
	ticker := time.NewTicker(s.cfg.Interval)
	defer ticker.Stop()

	// Lấy mẫu đầu tiên ngay lập tức để dashboard không phải chờ trọn một chu kỳ.
	// Mẫu đầu chưa có tốc độ mạng/đĩa vì chưa có mốc so sánh.
	s.sample(ctx)

	cleanup := time.NewTicker(time.Hour)
	defer cleanup.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("dừng bộ thu thập số liệu giám sát")
			return
		case <-ticker.C:
			s.sample(ctx)
		case <-cleanup.C:
			s.pruneHistory(ctx)
		}
	}
}

// Latest trả về mẫu gần nhất đã thu thập.
func (s *MonitorService) Latest() monitor.Snapshot {
	s.latestMu.RLock()
	defer s.latestMu.RUnlock()
	return s.latest
}

// Subscribe đăng ký nhận các mẫu mới. Hàm hủy trả về phải luôn được gọi.
func (s *MonitorService) Subscribe() (<-chan monitor.Snapshot, func()) {
	ch := make(chan monitor.Snapshot, subscriberBuffer)

	s.subsMu.Lock()
	s.subscribers[ch] = struct{}{}
	s.subsMu.Unlock()

	var once sync.Once
	return ch, func() {
		once.Do(func() {
			s.subsMu.Lock()
			delete(s.subscribers, ch)
			s.subsMu.Unlock()
			close(ch)
		})
	}
}

// History trả về lịch sử số liệu trong khoảng thời gian gần nhất.
func (s *MonitorService) History(ctx context.Context, window time.Duration) ([]model.MonitorSample, error) {
	if window <= 0 || window > s.cfg.Retention {
		window = s.cfg.Retention
	}

	var samples []model.MonitorSample
	err := s.db.WithContext(ctx).
		Where("created_at >= ?", time.Now().Add(-window)).
		Order("created_at ASC").
		Find(&samples).Error
	if err != nil {
		return nil, apperr.Internal.Wrap(err)
	}
	return samples, nil
}

// SystemInfo trả về thông tin tĩnh của máy chủ.
func (s *MonitorService) SystemInfo(ctx context.Context) (host.SystemInfo, error) {
	info, err := s.host.Info(ctx)
	if err != nil {
		return host.SystemInfo{}, apperr.Internal.Wrap(err)
	}
	return info, nil
}

// sample lấy một mẫu, phát cho các kết nối đang mở rồi lưu vào lịch sử.
func (s *MonitorService) sample(ctx context.Context) {
	snap, err := s.collector.Collect(ctx)
	if err != nil {
		slog.Error("không lấy được số liệu giám sát", "error", err)
		return
	}

	s.latestMu.Lock()
	s.latest = snap
	s.latestMu.Unlock()

	s.broadcast(snap)
	s.persist(ctx, snap)
}

// broadcast gửi mẫu tới mọi kết nối đang đăng ký.
//
// Gửi không chặn: kết nối nào xử lý không kịp thì bỏ qua mẫu này. Một trình duyệt
// treo không được phép làm chậm việc giám sát của cả hệ thống.
func (s *MonitorService) broadcast(snap monitor.Snapshot) {
	s.subsMu.RLock()
	defer s.subsMu.RUnlock()

	for ch := range s.subscribers {
		select {
		case ch <- snap:
		default:
		}
	}
}

func (s *MonitorService) persist(ctx context.Context, snap monitor.Snapshot) {
	sample := model.MonitorSample{
		CPUPercent:    snap.CPUPercent,
		MemoryPercent: snap.MemoryPercent,
		SwapPercent:   snap.SwapPercent,
		DiskPercent:   snap.DiskPercent,
		LoadAvg1:      snap.LoadAvg1,
		NetSentRate:   snap.NetSentRate,
		NetRecvRate:   snap.NetRecvRate,
		DiskReadRate:  snap.DiskReadRate,
		DiskWriteRate: snap.DiskWriteRate,
	}
	if err := s.db.WithContext(ctx).Create(&sample).Error; err != nil {
		slog.Error("không lưu được mẫu giám sát", "error", err)
	}
}

// pruneHistory xóa các mẫu cũ hơn thời gian lưu trữ đã cấu hình, để tệp cơ sở dữ
// liệu không phình vô hạn trên máy chủ chạy nhiều tháng liền.
func (s *MonitorService) pruneHistory(ctx context.Context) {
	cutoff := time.Now().Add(-s.cfg.Retention)
	res := s.db.WithContext(ctx).Where("created_at < ?", cutoff).Delete(&model.MonitorSample{})
	if res.Error != nil {
		slog.Error("không dọn được lịch sử giám sát", "error", res.Error)
		return
	}
	if res.RowsAffected > 0 {
		slog.Debug("đã dọn lịch sử giám sát", "rows", res.RowsAffected)
	}
}
