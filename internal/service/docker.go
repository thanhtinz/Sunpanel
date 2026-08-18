package service

import (
	"context"
	"errors"
	"strings"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/pkg/dockerx"
)

// DockerService quản lý Docker cho panel.
type DockerService struct {
	client *dockerx.Client
	audit  *AuditService
	// selfName là tên container của chính panel, nếu panel chạy trong container.
	selfName string
}

// NewDockerService tạo service quản lý Docker.
func NewDockerService(client *dockerx.Client, selfName string, audit *AuditService) *DockerService {
	return &DockerService{client: client, audit: audit, selfName: selfName}
}

// DockerStatus là tình trạng Docker trên máy chủ.
type DockerStatus struct {
	Available bool               `json:"available"`
	Socket    string             `json:"socket"`
	Info      dockerx.SystemInfo `json:"info"`
}

// Status kiểm tra Docker có dùng được không.
func (s *DockerService) Status(ctx context.Context) DockerStatus {
	status := DockerStatus{Socket: s.client.SocketPath()}

	if !s.client.Available(ctx) {
		return status
	}
	status.Available = true

	// Thiếu thông tin chi tiết không làm mất đi sự thật là Docker đang chạy.
	if info, err := s.client.Info(ctx); err == nil {
		status.Info = info
	}
	return status
}

// ContainerInfo là container kèm cờ bảo vệ cho giao diện.
type ContainerInfo struct {
	dockerx.Container
	// Protected cho biết panel từ chối dừng hoặc xóa container này.
	Protected bool `json:"protected"`
}

// ListContainers liệt kê container.
func (s *DockerService) ListContainers(ctx context.Context, all bool) ([]ContainerInfo, error) {
	containers, err := s.client.ListContainers(ctx, all)
	if err != nil {
		return nil, translateDockerError(err)
	}

	out := make([]ContainerInfo, 0, len(containers))
	for _, container := range containers {
		out = append(out, ContainerInfo{
			Container: container,
			Protected: s.isProtected(container),
		})
	}
	return out, nil
}

// isProtected cho biết container có phải chính panel hay không.
//
// Dừng container đang chạy panel qua giao diện của chính nó là tự cắt đường vào,
// và không có cách nào bấm nút khởi động lại sau đó.
func (s *DockerService) isProtected(container dockerx.Container) bool {
	if s.selfName == "" {
		return false
	}
	return container.Name == s.selfName || strings.HasPrefix(container.ID, s.selfName)
}

// ContainerAction là thao tác điều khiển container.
type ContainerAction string

// Các thao tác được hỗ trợ.
const (
	DockerStart   ContainerAction = "start"
	DockerStop    ContainerAction = "stop"
	DockerRestart ContainerAction = "restart"
	DockerPause   ContainerAction = "pause"
	DockerUnpause ContainerAction = "unpause"
	DockerRemove  ContainerAction = "remove"
)

// stopTimeout là số giây chờ container tự dừng trước khi buộc dừng.
const stopTimeout = 10

// ControlContainer thực hiện một thao tác lên container.
func (s *DockerService) ControlContainer(
	ctx context.Context, action ContainerAction, id string, actor AuditEntry,
) error {
	if s.isProtectedID(ctx, id) && (action == DockerStop || action == DockerRemove) {
		return apperr.DockerSelfProtected
	}

	var err error
	switch action {
	case DockerStart:
		err = s.client.StartContainer(ctx, id)
	case DockerStop:
		err = s.client.StopContainer(ctx, id, stopTimeout)
	case DockerRestart:
		err = s.client.RestartContainer(ctx, id, stopTimeout)
	case DockerPause:
		err = s.client.PauseContainer(ctx, id)
	case DockerUnpause:
		err = s.client.UnpauseContainer(ctx, id)
	case DockerRemove:
		err = s.client.RemoveContainer(ctx, id, true)
	default:
		return apperr.BadRequest
	}

	actor.Action = "docker." + string(action)
	actor.Resource = id
	actor.Success = err == nil
	s.audit.Record(ctx, actor)

	return translateDockerError(err)
}

// isProtectedID tra xem một định danh có trỏ tới container của chính panel không.
func (s *DockerService) isProtectedID(ctx context.Context, id string) bool {
	if s.selfName == "" {
		return false
	}
	if id == s.selfName {
		return true
	}

	// Người dùng có thể gửi ID thay vì tên, nên phải đối chiếu qua danh sách.
	containers, err := s.client.ListContainers(ctx, true)
	if err != nil {
		return false
	}
	for _, container := range containers {
		if container.ID == id || container.Name == id {
			return s.isProtected(container)
		}
	}
	return false
}

// ContainerLogs đọc nhật ký của một container.
func (s *DockerService) ContainerLogs(ctx context.Context, id string, lines int) (string, error) {
	logs, err := s.client.ContainerLogs(ctx, id, lines)
	if err != nil {
		return "", translateDockerError(err)
	}
	return logs, nil
}

// ContainerStats đọc mức sử dụng tài nguyên của một container.
func (s *DockerService) ContainerStats(ctx context.Context, id string) (dockerx.Stats, error) {
	stats, err := s.client.ContainerStats(ctx, id)
	if err != nil {
		return dockerx.Stats{}, translateDockerError(err)
	}
	return stats, nil
}

// ListImages liệt kê image.
func (s *DockerService) ListImages(ctx context.Context) ([]dockerx.Image, error) {
	images, err := s.client.ListImages(ctx)
	if err != nil {
		return nil, translateDockerError(err)
	}
	return images, nil
}

// RemoveImage xóa một image.
func (s *DockerService) RemoveImage(ctx context.Context, id string, force bool, actor AuditEntry) error {
	err := s.client.RemoveImage(ctx, id, force)

	actor.Action = "docker.remove_image"
	actor.Resource = id
	actor.Success = err == nil
	s.audit.Record(ctx, actor)

	return translateDockerError(err)
}

// PullImage tải một image về máy.
func (s *DockerService) PullImage(ctx context.Context, ref string, actor AuditEntry) error {
	err := s.client.PullImage(ctx, ref)

	actor.Action = "docker.pull_image"
	actor.Resource = ref
	actor.Success = err == nil
	s.audit.Record(ctx, actor)

	return translateDockerError(err)
}

// ListVolumes liệt kê volume.
func (s *DockerService) ListVolumes(ctx context.Context) ([]dockerx.Volume, error) {
	volumes, err := s.client.ListVolumes(ctx)
	if err != nil {
		return nil, translateDockerError(err)
	}
	return volumes, nil
}

// RemoveVolume xóa một volume.
func (s *DockerService) RemoveVolume(ctx context.Context, name string, actor AuditEntry) error {
	err := s.client.RemoveVolume(ctx, name, false)

	actor.Action = "docker.remove_volume"
	actor.Resource = name
	actor.Success = err == nil
	s.audit.Record(ctx, actor)

	return translateDockerError(err)
}

// ListNetworks liệt kê mạng.
func (s *DockerService) ListNetworks(ctx context.Context) ([]dockerx.Network, error) {
	networks, err := s.client.ListNetworks(ctx)
	if err != nil {
		return nil, translateDockerError(err)
	}
	return networks, nil
}

// Prune dọn các đối tượng Docker không còn dùng.
func (s *DockerService) Prune(ctx context.Context, actor AuditEntry) (int64, error) {
	freed, err := s.client.Prune(ctx)

	actor.Action = "docker.prune"
	actor.Success = err == nil
	s.audit.Record(ctx, actor)

	return freed, translateDockerError(err)
}

// translateDockerError chuyển lỗi của lớp dockerx thành lỗi có mã dịch.
func translateDockerError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, dockerx.ErrUnavailable):
		return apperr.DockerUnavailable.Wrap(err)
	case errors.Is(err, dockerx.ErrNotFound):
		return apperr.DockerNotFound.Wrap(err)
	case errors.Is(err, dockerx.ErrConflict):
		return apperr.DockerConflict.Wrap(err)
	default:
		return apperr.DockerActionFailed.Wrap(err)
	}
}
