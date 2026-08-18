package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/model"
	"github.com/thanhtinz/sunpanel/pkg/agent"
	"github.com/thanhtinz/sunpanel/pkg/crypto"
	"github.com/thanhtinz/sunpanel/pkg/host"
)

// nodeProbeTimeout giới hạn thời gian thử kết nối tới một node.
const nodeProbeTimeout = 15 * time.Second

// NodeRequest là dữ liệu thêm hoặc sửa một node.
type NodeRequest struct {
	Name    string `json:"name" binding:"required"`
	Address string `json:"address" binding:"required"`
	// Token để trống khi sửa nghĩa là giữ nguyên.
	Token      string `json:"token"`
	SkipVerify bool   `json:"skipVerify"`
	Remark     string `json:"remark"`
}

// NodeInfo là node kèm trạng thái kết nối hiện tại.
type NodeInfo struct {
	model.Node
	// Online cho biết panel có nối được tới agent hay không.
	Online bool `json:"online"`
	// Uptime là thời gian agent đã chạy.
	Uptime time.Duration `json:"uptime,omitempty"`
	// System là thông tin hệ thống đọc trực tiếp từ node.
	System *host.SystemInfo `json:"system,omitempty"`
}

// NodeService quản lý các máy chủ được panel điều khiển từ xa.
type NodeService struct {
	db     *gorm.DB
	sealer *crypto.Sealer
	audit  *AuditService

	// hosts giữ lại RemoteHost đã dựng để tái dùng kết nối.
	//
	// Mỗi RemoteHost mang một http.Client riêng với pool kết nối; dựng mới cho
	// từng lời gọi sẽ bắt tay TLS lại từ đầu mỗi lần.
	hosts map[uint]*agent.RemoteHost
	mu    sync.Mutex
}

// NewNodeService tạo dịch vụ node.
func NewNodeService(db *gorm.DB, sealer *crypto.Sealer, audit *AuditService) *NodeService {
	return &NodeService{db: db, sealer: sealer, audit: audit, hosts: map[uint]*agent.RemoteHost{}}
}

// List liệt kê node kèm trạng thái kết nối.
func (s *NodeService) List(ctx context.Context) ([]NodeInfo, error) {
	var records []model.Node
	if err := s.db.WithContext(ctx).Order("name").Find(&records).Error; err != nil {
		return nil, apperr.Internal.Wrap(err)
	}

	out := make([]NodeInfo, 0, len(records))
	for _, record := range records {
		out = append(out, s.describe(ctx, record))
	}
	return out, nil
}

// describe bổ sung trạng thái đọc trực tiếp từ agent.
//
// Node không nối được không phải lỗi của cả danh sách: đó chính là thông tin
// người dùng đang muốn biết.
func (s *NodeService) describe(ctx context.Context, record model.Node) NodeInfo {
	info := NodeInfo{Node: record}

	remote, err := s.hostFor(record)
	if err != nil {
		info.LastError = err.Error()
		return info
	}

	probeCtx, cancel := context.WithTimeout(ctx, nodeProbeTimeout)
	defer cancel()

	ping, err := remote.Ping(probeCtx)
	if err != nil {
		info.LastError = err.Error()
		s.recordProbe(record.ID, nil, err)
		return info
	}

	info.Online = true
	info.Uptime = ping.Uptime
	info.AgentVersion = ping.AgentVersion
	info.LastError = ""

	if system, err := remote.Info(probeCtx); err == nil {
		info.System = &system
		info.Hostname = system.Hostname
		info.OS = system.Platform
		info.Arch = system.Arch
		s.recordProbe(record.ID, &system, nil)
	}
	return info
}

// recordProbe lưu kết quả lần dò gần nhất.
func (s *NodeService) recordProbe(id uint, system *host.SystemInfo, cause error) {
	updates := map[string]any{"last_error": ""}
	if cause != nil {
		updates["last_error"] = trimMessage(cause.Error())
	} else {
		now := time.Now()
		updates["last_seen_at"] = now
	}
	if system != nil {
		updates["hostname"] = system.Hostname
		updates["os"] = system.Platform
		updates["arch"] = system.Arch
	}

	if err := s.db.Model(&model.Node{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		slog.Warn("không ghi được trạng thái node", "node", id, "error", err)
	}
}

// Host trả về host điều khiển một node.
//
// Đây là điểm mà mọi dịch vụ khác nối vào để chạy trên máy chủ ở xa: chúng nhận
// host.Host nên không phân biệt được đây là máy tại chỗ hay máy ở xa.
func (s *NodeService) Host(ctx context.Context, id uint) (host.Host, error) {
	record, err := s.find(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.hostFor(record)
}

// hostFor dựng hoặc lấy lại RemoteHost của một node.
func (s *NodeService) hostFor(record model.Node) (*agent.RemoteHost, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if remote, ok := s.hosts[record.ID]; ok {
		return remote, nil
	}

	token, err := s.sealer.Open(record.Token)
	if err != nil {
		return nil, apperr.Internal.Wrap(err)
	}

	remote := agent.NewRemoteHost(agent.Options{
		Name: record.Name, BaseURL: record.Address,
		Token: token, SkipVerify: record.SkipVerify,
	})
	s.hosts[record.ID] = remote
	return remote, nil
}

// forget bỏ RemoteHost đã lưu của một node.
//
// Phải gọi sau khi sửa hoặc xóa node, nếu không panel sẽ tiếp tục dùng địa chỉ
// và token cũ cho tới lần khởi động lại.
func (s *NodeService) forget(id uint) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if remote, ok := s.hosts[id]; ok {
		remote.Close()
		delete(s.hosts, id)
	}
}

func (s *NodeService) find(ctx context.Context, id uint) (model.Node, error) {
	var record model.Node
	err := s.db.WithContext(ctx).First(&record, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Node{}, apperr.NodeNotFound
	}
	if err != nil {
		return model.Node{}, apperr.Internal.Wrap(err)
	}
	return record, nil
}

// Get đọc một node kèm trạng thái.
func (s *NodeService) Get(ctx context.Context, id uint) (NodeInfo, error) {
	record, err := s.find(ctx, id)
	if err != nil {
		return NodeInfo{}, err
	}
	return s.describe(ctx, record), nil
}

// Create thêm một node.
func (s *NodeService) Create(ctx context.Context, req NodeRequest) (NodeInfo, error) {
	record, err := s.build(ctx, model.Node{}, req, 0)
	if err != nil {
		return NodeInfo{}, err
	}
	if req.Token == "" {
		return NodeInfo{}, apperr.BadRequest.WithParam("field", "token")
	}

	// Thử nối trước khi lưu: một node sai token mà chỉ lộ ra lúc cần dùng thì
	// người dùng không biết mình gõ nhầm ở đâu.
	if err := s.probe(ctx, record); err != nil {
		return NodeInfo{}, err
	}

	if err := s.db.WithContext(ctx).Create(&record).Error; err != nil {
		return NodeInfo{}, apperr.Internal.Wrap(err)
	}
	return s.describe(ctx, record), nil
}

// Update sửa một node.
func (s *NodeService) Update(ctx context.Context, id uint, req NodeRequest) (NodeInfo, error) {
	existing, err := s.find(ctx, id)
	if err != nil {
		return NodeInfo{}, err
	}

	record, err := s.build(ctx, existing, req, id)
	if err != nil {
		return NodeInfo{}, err
	}
	if err := s.probe(ctx, record); err != nil {
		return NodeInfo{}, err
	}

	if err := s.db.WithContext(ctx).Save(&record).Error; err != nil {
		return NodeInfo{}, apperr.Internal.Wrap(err)
	}
	s.forget(id)
	return s.describe(ctx, record), nil
}

func (s *NodeService) build(
	ctx context.Context, base model.Node, req NodeRequest, selfID uint,
) (model.Node, error) {
	address := strings.TrimRight(strings.TrimSpace(req.Address), "/")
	if !strings.HasPrefix(address, "https://") && !strings.HasPrefix(address, "http://") {
		return model.Node{}, apperr.NodeInvalidAddress.WithParam("address", req.Address)
	}

	name := strings.TrimSpace(req.Name)
	var count int64
	err := s.db.WithContext(ctx).Model(&model.Node{}).
		Where("name = ? AND id <> ?", name, selfID).Count(&count).Error
	if err != nil {
		return model.Node{}, apperr.Internal.Wrap(err)
	}
	if count > 0 {
		return model.Node{}, apperr.NodeNameExists.WithParam("name", name)
	}

	record := base
	record.Name = name
	record.Address = address
	record.SkipVerify = req.SkipVerify
	record.Remark = strings.TrimSpace(req.Remark)

	if req.Token != "" {
		sealed, err := s.sealer.Seal(req.Token)
		if err != nil {
			return model.Node{}, apperr.Internal.Wrap(err)
		}
		record.Token = sealed
	}
	return record, nil
}

// probe thử kết nối tới agent của một node.
func (s *NodeService) probe(ctx context.Context, record model.Node) error {
	token, err := s.sealer.Open(record.Token)
	if err != nil {
		return apperr.Internal.Wrap(err)
	}

	remote := agent.NewRemoteHost(agent.Options{
		Name: record.Name, BaseURL: record.Address,
		Token: token, SkipVerify: record.SkipVerify,
	})
	defer remote.Close()

	ctx, cancel := context.WithTimeout(ctx, nodeProbeTimeout)
	defer cancel()

	ping, err := remote.Ping(ctx)
	if err != nil {
		// Lệch phiên bản giao thức là nguyên nhân hỏng khó lần ra nhất, nên phải
		// tách thành một mã lỗi riêng thay vì gộp vào "không kết nối được".
		if strings.Contains(err.Error(), "phiên bản giao thức") {
			return apperr.NodeVersionMismatch.WithParam("message", trimMessage(err.Error()))
		}
		// Sai token và không nối được là hai vấn đề khác hẳn nhau: một bên phải
		// chép lại token, bên kia phải xem mạng và tường lửa.
		if strings.Contains(err.Error(), "token không hợp lệ") {
			return apperr.NodeUnauthorized.WithParam("message", trimMessage(err.Error()))
		}
		return apperr.NodeUnreachable.WithParam("message", trimMessage(err.Error()))
	}
	_ = ping
	return nil
}

// Delete gỡ một node khỏi panel.
//
// Chỉ gỡ khỏi danh sách; agent trên máy đó vẫn chạy và dữ liệu không bị đụng tới.
func (s *NodeService) Delete(ctx context.Context, id uint) error {
	record, err := s.find(ctx, id)
	if err != nil {
		return err
	}

	if err := s.db.WithContext(ctx).Delete(&record).Error; err != nil {
		return apperr.Internal.Wrap(err)
	}
	s.forget(id)
	return nil
}
