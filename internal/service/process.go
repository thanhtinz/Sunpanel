package service

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/pkg/procs"
)

// processListLimit là số dòng tối đa gửi xuống giao diện.
//
// Một máy chủ bận có thể có vài nghìn tiến trình, mà bảng đã sắp theo mức
// dùng CPU nên phần đuôi toàn tiến trình ngủ. Cắt bớt giữ cho phản hồi nhẹ,
// và ô tìm kiếm vẫn lọc trên toàn bộ danh sách trước khi cắt.
const processListLimit = 300

// ProcessService đọc bảng tiến trình và kết thúc tiến trình theo yêu cầu.
type ProcessService struct {
	sampler *procs.Sampler
	audit   *AuditService
}

// NewProcessService tạo service quản lý tiến trình.
func NewProcessService(audit *AuditService) *ProcessService {
	return &ProcessService{sampler: procs.NewSampler(), audit: audit}
}

// ProcessList là bảng tiến trình đã lọc và cắt bớt.
type ProcessList struct {
	Items []procs.Process `json:"items"`
	// Total là số tiến trình khớp bộ lọc trước khi cắt bớt.
	Total int `json:"total"`
	// Truncated cho biết danh sách đã bị cắt.
	Truncated bool `json:"truncated"`
}

// List liệt kê tiến trình, lọc theo từ khóa nếu có.
func (s *ProcessService) List(ctx context.Context, keyword string) (ProcessList, error) {
	items, err := s.sampler.List(ctx)
	if err != nil {
		return ProcessList{}, apperr.ProcessListFailed.Wrap(err)
	}

	if keyword = strings.TrimSpace(strings.ToLower(keyword)); keyword != "" {
		filtered := items[:0:0]
		for _, item := range items {
			if matchesProcess(item, keyword) {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}

	out := ProcessList{Total: len(items), Items: items}
	if len(items) > processListLimit {
		out.Items, out.Truncated = items[:processListLimit], true
	}
	return out, nil
}

// matchesProcess so từ khóa với tên, người dùng, dòng lệnh và số hiệu tiến trình.
func matchesProcess(item procs.Process, keyword string) bool {
	return strings.Contains(strings.ToLower(item.Name), keyword) ||
		strings.Contains(strings.ToLower(item.Username), keyword) ||
		strings.Contains(strings.ToLower(item.Command), keyword) ||
		strconv.Itoa(int(item.PID)) == keyword
}

// Listeners liệt kê các cổng đang mở kèm tiến trình sở hữu.
func (s *ProcessService) Listeners(ctx context.Context) ([]procs.Listener, error) {
	items, err := s.sampler.Listeners(ctx)
	if err != nil {
		return nil, apperr.ProcessListFailed.Wrap(err)
	}
	return items, nil
}

// Kill kết thúc một tiến trình và ghi lại vào nhật ký kiểm toán.
func (s *ProcessService) Kill(ctx context.Context, pid int32, force bool, actor AuditEntry) error {
	killErr := s.sampler.Kill(ctx, pid, force)

	actor.Action = "process.kill"
	if force {
		actor.Action = "process.force_kill"
	}
	actor.Resource = strconv.Itoa(int(pid))
	actor.Success = killErr == nil
	s.audit.Record(ctx, actor)

	return translateProcessError(killErr)
}

func translateProcessError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, procs.ErrProtected):
		return apperr.ProcessProtected
	case errors.Is(err, os.ErrProcessDone), errors.Is(err, os.ErrNotExist):
		return apperr.ProcessNotFound
	case strings.Contains(err.Error(), "process does not exist"):
		return apperr.ProcessNotFound
	default:
		return apperr.ProcessKillFailed.Wrap(err)
	}
}
