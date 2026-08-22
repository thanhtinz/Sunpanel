package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/thanhtinz/sunpanel/internal/model"
	"github.com/thanhtinz/sunpanel/pkg/notify"
	"github.com/thanhtinz/sunpanel/pkg/sshx"
)

// alertStreak là số lần liên tiếp một điều kiện phải đúng trước khi báo.
//
// Một mẫu đơn lẻ không phải là sự cố: bản sao lưu chạy lúc nửa đêm đẩy CPU lên
// một trăm phần trăm trong đúng một phút, và báo ngay lần đầu là cách nhanh
// nhất để người dùng tắt hết cảnh báo sau vài đêm bị đánh thức.
const alertStreak = 3

// nodeAlertState là trạng thái cảnh báo của một máy chủ.
//
// Giữ trong bộ nhớ chứ không lưu xuống đĩa: đây là trạng thái của tiến trình
// đang chạy, và panel khởi động lại thì việc đánh giá lại từ đầu chỉ tốn thêm
// đúng một thông báo cho mỗi sự cố đang diễn ra.
type nodeAlertState struct {
	// failures đếm số lần lấy mẫu hỏng liên tiếp.
	failures int
	// down cho biết đã báo mất kết nối hay chưa.
	down bool
	// streaks đếm số mẫu liên tiếp vượt ngưỡng, theo từng loại tài nguyên.
	streaks map[string]int
	// firing là các loại tài nguyên đang trong trạng thái đã báo.
	firing map[string]bool
}

// alertStateFor lấy trạng thái cảnh báo của một máy chủ, tạo mới nếu chưa có.
func (s *NodeService) alertStateFor(id uint) *nodeAlertState {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, ok := s.alertStates[id]
	if !ok {
		state = &nodeAlertState{streaks: map[string]int{}, firing: map[string]bool{}}
		s.alertStates[id] = state
	}
	return state
}

// forgetAlertState quên trạng thái cảnh báo của một máy chủ đã bị gỡ.
func (s *NodeService) forgetAlertState(id uint) {
	s.mu.Lock()
	delete(s.alertStates, id)
	s.mu.Unlock()
}

// noteSampleFailure ghi nhận một lần lấy mẫu hỏng và báo khi máy chủ mất kết nối.
func (s *NodeService) noteSampleFailure(ctx context.Context, record model.Node, cause error) {
	if s.alerts == nil || !record.AlertOffline {
		return
	}

	state := s.alertStateFor(record.ID)
	state.failures++
	if state.down || state.failures < alertStreak {
		return
	}
	state.down = true

	s.alerts.Notify(ctx, "node", notify.Message{
		Title: fmt.Sprintf("[SunPanel] %s mất kết nối", record.Name),
		Body: fmt.Sprintf(
			"Không kết nối được tới %s sau %d lần thử liên tiếp: %s",
			record.Address, state.failures, errorDetail(cause),
		),
		Level:    notify.LevelCritical,
		Hostname: record.Name,
	})
}

// noteSample đánh giá một mẫu vừa đo được và báo khi vượt ngưỡng.
func (s *NodeService) noteSample(ctx context.Context, record model.Node, metrics sshx.Metrics) {
	if s.alerts == nil {
		return
	}

	state := s.alertStateFor(record.ID)

	// Máy chủ trả lời lại sau khi đã báo mất kết nối: người nhận cảnh báo cần
	// biết sự cố đã hết, nếu không họ vẫn đang chạy đi xử lý một thứ đã tự khỏi.
	if state.down {
		state.down = false
		s.alerts.Notify(ctx, "node", notify.Message{
			Title:    fmt.Sprintf("[SunPanel] %s đã trở lại", record.Name),
			Body:     fmt.Sprintf("%s kết nối lại được bình thường.", record.Address),
			Level:    notify.LevelInfo,
			Hostname: record.Name,
		})
	}
	state.failures = 0

	checks := []struct {
		key       string
		label     string
		value     float64
		threshold int
	}{
		{"cpu", "CPU", metrics.CPUPercent, record.AlertCPU},
		{"memory", "Bộ nhớ", metrics.MemoryPercent, record.AlertMemory},
		{"disk", "Ổ đĩa", metrics.DiskPercent, record.AlertDisk},
	}

	for _, check := range checks {
		if check.threshold <= 0 {
			// Ngưỡng đang tắt: xóa luôn trạng thái cũ để lần bật lại không kế thừa
			// một cảnh báo đang treo từ lần trước.
			delete(state.streaks, check.key)
			delete(state.firing, check.key)
			continue
		}

		if check.value < float64(check.threshold) {
			state.streaks[check.key] = 0
			if state.firing[check.key] {
				state.firing[check.key] = false
				s.alerts.Notify(ctx, "node", notify.Message{
					Title: fmt.Sprintf("[SunPanel] %s: %s đã hạ xuống", record.Name, check.label),
					Body: fmt.Sprintf(
						"%s còn %.1f%%, dưới ngưỡng %d%%.", check.label, check.value, check.threshold,
					),
					Level:    notify.LevelInfo,
					Hostname: record.Name,
				})
			}
			continue
		}

		state.streaks[check.key]++
		if state.firing[check.key] || state.streaks[check.key] < alertStreak {
			continue
		}
		state.firing[check.key] = true

		s.alerts.Notify(ctx, "node", notify.Message{
			Title: fmt.Sprintf("[SunPanel] %s: %s vượt ngưỡng", record.Name, check.label),
			Body: fmt.Sprintf(
				"%s đang ở %.1f%%, trên ngưỡng %d%% trong %d lần đo liên tiếp.",
				check.label, check.value, check.threshold, state.streaks[check.key],
			),
			Level:    notify.LevelWarning,
			Hostname: record.Name,
		})
	}
}

// AlertSummary mô tả ngưỡng đang đặt của một máy chủ, dùng để hiển thị.
func AlertSummary(record model.Node) string {
	parts := make([]string, 0, 4)
	if record.AlertOffline {
		parts = append(parts, "mất kết nối")
	}
	for _, item := range []struct {
		label     string
		threshold int
	}{{"CPU", record.AlertCPU}, {"bộ nhớ", record.AlertMemory}, {"đĩa", record.AlertDisk}} {
		if item.threshold > 0 {
			parts = append(parts, fmt.Sprintf("%s %d%%", item.label, item.threshold))
		}
	}
	return strings.Join(parts, ", ")
}
