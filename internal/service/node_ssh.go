package service

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/model"
	"github.com/thanhtinz/sunpanel/pkg/host"
	"github.com/thanhtinz/sunpanel/pkg/sshx"
)

// defaultSSHPort là cổng SSH tiêu chuẩn.
const defaultSSHPort = 22

// maxRemoteOutput giới hạn lượng đầu ra một lệnh từ xa trả về.
//
// Một lệnh gõ nhầm ("cat /dev/urandom") không được phép biến thành hàng chục
// megabyte JSON chảy ngược về trình duyệt.
const maxRemoteOutput = 64 << 10

// RemoteResult là kết quả chạy một lệnh trên máy chủ từ xa.
type RemoteResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exitCode"`
}

// sshTarget dựng thông tin kết nối từ bản ghi, giải mã các bí mật.
func (s *NodeService) sshTarget(record model.Node) (sshx.Target, error) {
	secret, err := s.sealer.Open(record.Secret)
	if err != nil {
		return sshx.Target{}, apperr.Internal.Wrap(err)
	}

	credential := sshx.Credential{User: record.User}
	if record.AuthType == model.AuthKey {
		credential.PrivateKey = secret
		if record.Passphrase != "" {
			phrase, err := s.sealer.Open(record.Passphrase)
			if err != nil {
				return sshx.Target{}, apperr.Internal.Wrap(err)
			}
			credential.Passphrase = phrase
		}
	} else {
		credential.Password = secret
	}

	return sshx.Target{
		Host: record.Host, Port: record.Port,
		Credential: credential, Fingerprint: record.Fingerprint,
	}, nil
}

// dialSSH mở kết nối tới máy chủ từ xa.
func (s *NodeService) dialSSH(ctx context.Context, record model.Node) (*sshx.Client, error) {
	target, err := s.sshTarget(record)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, nodeProbeTimeout)
	defer cancel()

	client, err := sshx.Dial(ctx, target)
	if err != nil {
		return nil, translateSSHError(err)
	}
	return client, nil
}

// probeSSH thử kết nối và đọc thông tin máy chủ.
//
// Trả về cả dấu vân tay để bên gọi lưu lại ở lần thêm máy đầu tiên: đó là mốc
// so sánh cho mọi lần kết nối sau.
func (s *NodeService) probeSSH(ctx context.Context, record model.Node) (sshx.Info, string, error) {
	client, err := s.dialSSH(ctx, record)
	if err != nil {
		return sshx.Info{}, "", err
	}
	defer func() { _ = client.Close() }()

	info, err := client.SystemInfo(ctx)
	if err != nil {
		return sshx.Info{}, client.Fingerprint(), apperr.NodeUnreachable.
			WithParam("message", trimMessage(err.Error()))
	}
	return info, client.Fingerprint(), nil
}

// Exec chạy một lệnh trên máy chủ từ xa.
//
// Chỉ dành cho máy chủ SSH: máy chủ có agent đã có đường chạy lệnh riêng, đi
// qua danh sách trắng của agent.
func (s *NodeService) Exec(ctx context.Context, id uint, command string, actor AuditEntry) (RemoteResult, error) {
	record, err := s.find(ctx, id)
	if err != nil {
		return RemoteResult{}, err
	}
	if record.Kind != model.NodeSSH {
		return RemoteResult{}, apperr.NodeNotSSH
	}

	command = strings.TrimSpace(command)
	if command == "" {
		return RemoteResult{}, apperr.BadRequest.WithParam("field", "command")
	}

	client, err := s.dialSSH(ctx, record)
	if err != nil {
		return RemoteResult{}, err
	}
	defer func() { _ = client.Close() }()

	result, err := client.Run(ctx, command)

	// Lệnh chạy trên máy chủ khác cũng phải để lại dấu vết như lệnh chạy trên
	// máy này: đây là đường thay đổi trạng thái mạnh nhất panel có.
	actor.Action = "node.exec"
	actor.Resource = record.Name + ": " + command
	actor.Success = err == nil
	s.audit.Record(ctx, actor)

	if err != nil {
		return RemoteResult{}, apperr.NodeUnreachable.WithParam("message", trimMessage(err.Error()))
	}
	return RemoteResult{
		Stdout:   clip(result.Stdout, maxRemoteOutput),
		Stderr:   clip(result.Stderr, maxRemoteOutput),
		ExitCode: result.ExitCode,
	}, nil
}

// OpenShell mở phiên dòng lệnh tới máy chủ từ xa.
//
// Kết nối được trả về cùng phiên: đóng phiên mà để kết nối lại thì mỗi lần mở
// terminal lại bỏ quên một kết nối SSH trên máy đích.
func (s *NodeService) OpenShell(ctx context.Context, id uint, cols, rows int, actor AuditEntry) (*sshx.Shell, func(), error) {
	record, err := s.find(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if record.Kind != model.NodeSSH {
		return nil, nil, apperr.NodeNotSSH
	}

	client, err := s.dialSSH(ctx, record)
	if err != nil {
		return nil, nil, err
	}

	shell, err := client.OpenShell(cols, rows)
	if err != nil {
		_ = client.Close()
		return nil, nil, apperr.NodeUnreachable.WithParam("message", trimMessage(err.Error()))
	}

	actor.Action = "node.terminal"
	actor.Resource = record.Name
	actor.Success = true
	s.audit.Record(ctx, actor)

	return shell, func() {
		_ = shell.Close()
		_ = client.Close()
	}, nil
}

// toSystemInfo đổi thông tin đọc qua SSH sang kiểu chung của panel.
//
// Giao diện chỉ biết một kiểu duy nhất, nên máy chủ nối bằng agent hay bằng SSH
// đều hiện ra như nhau.
func toSystemInfo(info sshx.Info) host.SystemInfo {
	// Kẹp về 0 trước khi đổi dấu: một máy chủ trả về số âm là máy chủ đang nói
	// dối, và để nó chảy vào kiểu không dấu sẽ thành một con số khổng lồ.
	memory := info.MemoryTotal
	if memory < 0 {
		memory = 0
	}

	return host.SystemInfo{
		Hostname:    info.Hostname,
		OS:          "linux",
		Platform:    info.OS,
		Kernel:      info.Kernel,
		Arch:        info.Arch,
		CPUCores:    info.CPUCores,
		TotalMemory: uint64(memory),
	}
}

// translateSSHError đổi lỗi của lớp SSH thành mã lỗi giao diện dịch được.
func translateSSHError(err error) error {
	switch {
	case errors.Is(err, sshx.ErrAuthFailed):
		return apperr.NodeUnauthorized.WithParam("message", "sai tài khoản, mật khẩu hoặc khóa")
	case errors.Is(err, sshx.ErrHostKeyChanged):
		return apperr.NodeHostKeyChanged.WithParam("message", trimMessage(err.Error()))
	default:
		return apperr.NodeUnreachable.WithParam("message", trimMessage(err.Error()))
	}
}

// errorDetail lấy câu mô tả đọc được của một lỗi.
//
// Mã lỗi một mình ("node.unreachable") không nói được phải sửa gì; thứ giúp
// được người dùng nằm trong tham số message — "connection refused" hay "khóa
// máy chủ đã thay đổi" mới chỉ ra đúng chỗ hỏng.
func errorDetail(err error) string {
	var appErr *apperr.Error
	if errors.As(err, &appErr) {
		if message, ok := appErr.Params["message"].(string); ok && message != "" {
			return trimMessage(message)
		}
		return appErr.Code
	}
	return trimMessage(err.Error())
}

// clip cắt chuỗi quá dài và nói rõ là đã cắt.
func clip(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "\n… (đã cắt bớt)"
}

// kindOf đọc kiểu kết nối từ yêu cầu; để trống hiểu là agent.
//
// Mặc định phải là agent chứ không phải ssh: mọi bản ghi tạo trước khi có kiểu
// ssh đều không mang trường này, và đổi mặc định sẽ làm chúng ngừng kết nối.
func kindOf(req NodeRequest) string {
	if strings.TrimSpace(req.Kind) == model.NodeSSH {
		return model.NodeSSH
	}
	return model.NodeAgent
}

// checkNameFree kiểm tra tên chưa bị máy chủ khác dùng.
func (s *NodeService) checkNameFree(ctx context.Context, name string, selfID uint) error {
	var count int64
	err := s.db.WithContext(ctx).Model(&model.Node{}).
		Where("name = ? AND id <> ?", name, selfID).Count(&count).Error
	if err != nil {
		return apperr.Internal.Wrap(err)
	}
	if count > 0 {
		return apperr.NodeNameExists.WithParam("name", name)
	}
	return nil
}

// buildSSH dựng bản ghi cho một máy chủ kết nối qua SSH.
func (s *NodeService) buildSSH(base model.Node, req NodeRequest) (model.Node, error) {
	hostname := strings.TrimSpace(req.Host)
	if hostname == "" || strings.ContainsAny(hostname, " /\\") {
		return model.Node{}, apperr.NodeInvalidHost.WithParam("host", req.Host)
	}

	port := req.Port
	if port == 0 {
		port = defaultSSHPort
	}
	if port < 1 || port > 65535 {
		return model.Node{}, apperr.NodeInvalidHost.WithParam("host", req.Host)
	}

	user := strings.TrimSpace(req.User)
	if user == "" {
		return model.Node{}, apperr.BadRequest.WithParam("field", "user")
	}

	record := base
	record.Kind = model.NodeSSH
	record.Name = strings.TrimSpace(req.Name)
	record.Host = hostname
	record.Port = port
	record.User = user
	record.Remark = strings.TrimSpace(req.Remark)
	record.AlertOffline = req.AlertOffline
	record.AlertCPU = clampPercent(req.AlertCPU)
	record.AlertMemory = clampPercent(req.AlertMemory)
	record.AlertDisk = clampPercent(req.AlertDisk)
	record.AuthType = model.AuthPassword
	if req.AuthType == model.AuthKey {
		record.AuthType = model.AuthKey
	}
	// Địa chỉ chỉ để hiển thị, nhưng vẫn phải có: cột này là not null từ thời
	// panel mới chỉ biết tới agent.
	record.Address = "ssh://" + user + "@" + hostname + ":" + strconv.Itoa(port)

	// Bí mật để trống nghĩa là giữ nguyên cái đã lưu: giao diện không bao giờ
	// đọc lại được nó, nên coi ô trống là "xóa" sẽ lặng lẽ làm hỏng kết nối mỗi
	// lần người dùng chỉ định sửa ghi chú.
	if secret := strings.TrimSpace(req.Secret); secret != "" {
		sealed, err := s.sealer.Seal(secret)
		if err != nil {
			return model.Node{}, apperr.Internal.Wrap(err)
		}
		record.Secret = sealed
		// Bí mật mới có thể là của một máy khác; dấu vân tay cũ không còn nghĩa.
		record.Fingerprint = ""
	}
	if req.Passphrase != "" {
		sealed, err := s.sealer.Seal(req.Passphrase)
		if err != nil {
			return model.Node{}, apperr.Internal.Wrap(err)
		}
		record.Passphrase = sealed
	}

	// Đổi host hoặc cổng nghĩa là đang trỏ tới một máy khác.
	if base.Host != "" && (base.Host != record.Host || base.Port != record.Port) {
		record.Fingerprint = ""
	}
	return record, nil
}

// clampPercent kẹp một ngưỡng phần trăm về khoảng hợp lệ.
//
// Ngưỡng 0 là tắt, còn ngưỡng trên 100 thì không bao giờ chạm tới — cả hai đều
// nghĩa là không cảnh báo, nên gộp chúng lại thành 0 cho rõ ràng.
func clampPercent(value int) int {
	if value <= 0 || value > 100 {
		return 0
	}
	return value
}

// probeAndStamp thử kết nối và ghi lại những gì đọc được vào bản ghi.
func (s *NodeService) probeAndStamp(ctx context.Context, record model.Node) (model.Node, error) {
	if record.Kind != model.NodeSSH {
		return record, s.probe(ctx, record)
	}

	info, fingerprint, err := s.probeSSH(ctx, record)
	if err != nil {
		return record, err
	}

	record.Fingerprint = fingerprint
	record.Hostname = info.Hostname
	record.OS = info.OS
	record.Arch = info.Arch
	return record, nil
}

// describeSSH đọc trạng thái hiện tại của một máy chủ SSH.
func (s *NodeService) describeSSH(ctx context.Context, record model.Node) NodeInfo {
	info := NodeInfo{Node: record}

	remote, _, err := s.probeSSH(ctx, record)
	if err != nil {
		info.LastError = errorDetail(err)
		s.recordProbe(record.ID, nil, err)
		return info
	}

	system := toSystemInfo(remote)
	info.Online = true
	info.LastError = ""
	info.Uptime = remote.Uptime()
	info.System = &system
	info.Hostname = remote.Hostname
	info.OS = remote.OS
	info.Arch = remote.Arch
	info.Load1 = remote.Load1
	info.MemoryUsed, info.MemoryTotal = remote.MemoryUsed, remote.MemoryTotal
	info.DiskUsed, info.DiskTotal = remote.DiskUsed, remote.DiskTotal

	s.recordProbe(record.ID, &system, nil)
	return info
}
