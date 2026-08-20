package service

import (
	"context"
	"path"
	"time"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/pkg/accesslog"
)

// trafficWindows là các khoảng thời gian giao diện được phép chọn.
//
// Danh sách cố định thay vì nhận số giờ tùy ý: khoảng càng dài thì càng phải
// đọc nhiều nhật ký, và một tham số tự do là lời mời ép máy chủ đọc tám
// megabyte cho mỗi lần bấm.
var trafficWindows = map[string]time.Duration{
	"1h":  time.Hour,
	"6h":  6 * time.Hour,
	"24h": 24 * time.Hour,
	"7d":  7 * 24 * time.Hour,
}

// TrafficReport là số liệu truy cập của một website.
type TrafficReport struct {
	accesslog.Stats
	// Window là khoảng thời gian đã chọn.
	Window string `json:"window"`
	// LogPath là tệp nhật ký đã đọc, để người dùng biết số liệu lấy từ đâu.
	LogPath string `json:"logPath"`
}

// Traffic tóm tắt nhật ký truy cập của một website.
func (s *WebsiteService) Traffic(ctx context.Context, id uint, window string) (TrafficReport, error) {
	site, err := s.Get(ctx, id)
	if err != nil {
		return TrafficReport{}, err
	}

	if s.traffic == nil {
		return TrafficReport{}, apperr.WebsiteLogUnavailable.WithParam("message", "chưa bật đọc nhật ký truy cập")
	}

	duration, ok := trafficWindows[window]
	if !ok {
		duration, window = 24*time.Hour, "24h"
	}

	target := s.accessLogPath(site.Name)
	stats, err := s.traffic.Analyze(ctx, target, duration)
	if err != nil {
		if isNotFound(err) {
			// Website vừa dựng xong chưa có ai vào thì nginx chưa tạo tệp nhật ký.
			// Đó là trạng thái bình thường, không phải lỗi cần báo đỏ.
			return TrafficReport{Stats: emptyStats(), Window: window, LogPath: target}, nil
		}
		return TrafficReport{}, apperr.WebsiteLogUnavailable.WithParam("message", err.Error())
	}

	return TrafficReport{Stats: stats, Window: window, LogPath: target}, nil
}

// accessLogPath là tệp nhật ký truy cập do mẫu cấu hình nginx sinh ra.
func (s *WebsiteService) accessLogPath(name string) string {
	return path.Join(s.logDir, name+".access.log")
}

// emptyStats là bản tóm tắt rỗng với đủ các danh sách.
//
// Danh sách nil ra JSON thành null, và giao diện đọc thẳng .length của chúng.
func emptyStats() accesslog.Stats {
	return accesslog.Stats{
		TopPaths:     []accesslog.Count{},
		TopIPs:       []accesslog.Count{},
		TopReferrers: []accesslog.Count{},
		TopAgents:    []accesslog.Count{},
		Buckets:      []accesslog.Bucket{},
		Failures:     []accesslog.Failure{},
	}
}
