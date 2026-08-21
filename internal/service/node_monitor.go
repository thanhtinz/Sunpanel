package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/model"
	"github.com/thanhtinz/sunpanel/pkg/sshx"
)

// sampleInterval là chu kỳ lấy mẫu tài nguyên của máy chủ từ xa.
//
// Thưa hơn hẳn chu kỳ giám sát máy tại chỗ: mỗi lần lấy mẫu là một vòng đi về
// qua mạng tới một máy có thể nằm ở châu lục khác, và biểu đồ theo phút đã đủ
// để nhận ra một VPS đang quá tải.
const sampleInterval = time.Minute

// sampleRetention là thời gian giữ lịch sử.
const sampleRetention = 7 * 24 * time.Hour

// samplePruneInterval là chu kỳ dọn mẫu cũ.
const samplePruneInterval = 6 * time.Hour

// RunSampling lấy mẫu tài nguyên các máy chủ SSH cho tới khi ctx dừng.
func (s *NodeService) RunSampling(ctx context.Context) {
	ticker := time.NewTicker(sampleInterval)
	defer ticker.Stop()

	prune := time.NewTicker(samplePruneInterval)
	defer prune.Stop()

	for {
		select {
		case <-ctx.Done():
			s.closeAllSSH()
			return
		case <-ticker.C:
			s.sampleAll(ctx)
		case <-prune.C:
			s.pruneSamples(ctx)
		}
	}
}

// sampleAll lấy mẫu lần lượt từng máy chủ SSH.
//
// Tuần tự chứ không song song: số máy chủ của một người dùng thường đếm trên
// đầu ngón tay, còn mở đồng loạt hàng chục kết nối SSH mỗi phút là cách nhanh
// nhất để chính panel trở thành thứ gây tải.
func (s *NodeService) sampleAll(ctx context.Context) {
	var records []model.Node
	err := s.db.WithContext(ctx).Where("kind = ?", model.NodeSSH).Find(&records).Error
	if err != nil {
		slog.Warn("không đọc được danh sách máy chủ từ xa", "error", err)
		return
	}

	for _, record := range records {
		if ctx.Err() != nil {
			return
		}
		if err := s.sample(ctx, record); err != nil {
			// Một máy chủ tắt không làm hỏng vòng lấy mẫu của các máy còn lại;
			// trạng thái mất kết nối đã hiện sẵn trên danh sách.
			slog.Debug("không lấy được mẫu tài nguyên", "node", record.Name, "error", err)
		}
	}
}

// sample đo và lưu một mẫu của một máy chủ.
func (s *NodeService) sample(ctx context.Context, record model.Node) error {
	client, err := s.sshClient(ctx, record)
	if err != nil {
		return err
	}

	metrics, err := client.Metrics(ctx)
	if err != nil {
		// Kết nối đang giữ có thể đã chết từ lâu mà chưa ai chạm tới; bỏ nó đi để
		// lần sau mở lại thay vì lặp lại đúng lỗi này mỗi phút.
		s.dropSSH(record.ID)
		return err
	}

	sample := model.NodeSample{
		NodeID: record.ID, At: time.Now(),
		CPUPercent: metrics.CPUPercent, MemoryPercent: metrics.MemoryPercent,
		DiskPercent: metrics.DiskPercent, Load1: metrics.Load1,
	}
	return s.db.WithContext(ctx).Create(&sample).Error
}

// pruneSamples xóa các mẫu quá cũ.
func (s *NodeService) pruneSamples(ctx context.Context) {
	cutoff := time.Now().Add(-sampleRetention)
	err := s.db.WithContext(ctx).Where("at < ?", cutoff).Delete(&model.NodeSample{}).Error
	if err != nil {
		slog.Warn("không dọn được lịch sử máy chủ từ xa", "error", err)
	}
}

// History đọc lịch sử tài nguyên của một máy chủ.
func (s *NodeService) History(ctx context.Context, id uint, window time.Duration) ([]model.NodeSample, error) {
	if _, err := s.find(ctx, id); err != nil {
		return nil, err
	}
	if window <= 0 || window > sampleRetention {
		window = 24 * time.Hour
	}

	samples := make([]model.NodeSample, 0, 128)
	err := s.db.WithContext(ctx).
		Where("node_id = ? AND at > ?", id, time.Now().Add(-window)).
		Order("at").Find(&samples).Error
	if err != nil {
		return nil, apperr.Internal.Wrap(err)
	}
	return samples, nil
}

// SampleNow lấy một mẫu ngay lập tức.
//
// Có nó thì biểu đồ của một máy vừa thêm không phải chờ hết một chu kỳ mới có
// điểm đầu tiên.
func (s *NodeService) SampleNow(ctx context.Context, id uint) error {
	record, err := s.find(ctx, id)
	if err != nil {
		return err
	}
	if record.Kind != model.NodeSSH {
		return apperr.NodeNotSSH
	}
	if err := s.sample(ctx, record); err != nil {
		return translateSSHError(err)
	}
	return nil
}

// sshClient trả về kết nối đang giữ, hoặc mở mới nếu chưa có.
//
// Giữ lại kết nối vì bắt tay SSH tốn vài vòng đi về và một phép trao khóa; mở
// lại từ đầu mỗi phút cho mỗi máy chủ là phần lớn chi phí của việc giám sát.
func (s *NodeService) sshClient(ctx context.Context, record model.Node) (*sshx.Client, error) {
	s.mu.Lock()
	client, ok := s.sshClients[record.ID]
	s.mu.Unlock()
	if ok {
		return client, nil
	}

	client, err := s.dialSSH(ctx, record)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	// Một lời gọi khác có thể vừa mở xong kết nối trong lúc mình đang bắt tay;
	// giữ cái đã có và đóng cái vừa mở, thay vì bỏ quên một kết nối trên máy đích.
	if existing, ok := s.sshClients[record.ID]; ok {
		s.mu.Unlock()
		_ = client.Close()
		return existing, nil
	}
	s.sshClients[record.ID] = client
	s.mu.Unlock()
	return client, nil
}

// dropSSH đóng và quên kết nối đang giữ của một máy chủ.
func (s *NodeService) dropSSH(id uint) {
	s.mu.Lock()
	client, ok := s.sshClients[id]
	delete(s.sshClients, id)
	s.mu.Unlock()

	if ok {
		_ = client.Close()
	}
}

// closeAllSSH đóng mọi kết nối đang giữ.
func (s *NodeService) closeAllSSH() {
	s.mu.Lock()
	clients := s.sshClients
	s.sshClients = map[uint]*sshx.Client{}
	s.mu.Unlock()

	for _, client := range clients {
		_ = client.Close()
	}
}
