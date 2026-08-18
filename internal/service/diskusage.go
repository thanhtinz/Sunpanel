package service

import (
	"context"
	"errors"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/pkg/diskscan"
	"github.com/thanhtinz/sunpanel/pkg/monitor"
)

// DiskService trả lời câu hỏi "cái gì đang chiếm đĩa".
type DiskService struct {
	scanner *diskscan.Scanner
	monitor *MonitorService
}

// NewDiskService tạo dịch vụ phân tích dung lượng.
func NewDiskService(scanner *diskscan.Scanner, monitor *MonitorService) *DiskService {
	return &DiskService{scanner: scanner, monitor: monitor}
}

// Partitions liệt kê các phân vùng thật kèm mức sử dụng.
//
// Lấy từ mẫu giám sát gần nhất thay vì hỏi lại hệ điều hành: bộ thu thập đã
// chạy sẵn theo chu kỳ, và hai nguồn số liệu cho hai con số lệch nhau trên cùng
// một màn hình là thứ khiến người dùng mất tin vào cả hai.
func (s *DiskService) Partitions() []monitor.DiskUsage {
	disks := s.monitor.Latest().Disks
	if disks == nil {
		return []monitor.DiskUsage{}
	}
	return disks
}

// Usage đo dung lượng từng mục con của một thư mục.
func (s *DiskService) Usage(ctx context.Context, path string) (diskscan.Report, error) {
	report, err := s.scanner.Scan(ctx, normalizePath(path))
	if err != nil {
		if errors.Is(err, diskscan.ErrNotDirectory) {
			return diskscan.Report{}, apperr.FileNotFound.Wrap(err)
		}
		return diskscan.Report{}, translateFSError(err)
	}
	return report, nil
}
