package service

import (
	"context"
	"errors"
	"testing"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/model"
	"github.com/thanhtinz/sunpanel/pkg/crypto"
)

// newNodeService dựng dịch vụ máy chủ từ xa trên cơ sở dữ liệu trong bộ nhớ.
func newNodeService(t *testing.T) *NodeService {
	t.Helper()

	db := newMemoryDB(t)
	sealer, err := crypto.NewSealer(make([]byte, 32))
	if err != nil {
		t.Fatalf("khởi tạo bộ mã hóa: %v", err)
	}
	return NewNodeService(db, sealer, NewAuditService(db))
}

// sshRequest là một yêu cầu thêm máy chủ SSH hợp lệ.
func sshRequest() NodeRequest {
	return NodeRequest{
		Name: "vps-sai-gon", Kind: model.NodeSSH,
		Host: "203.0.113.10", Port: 2222, User: "quantri",
		AuthType: model.AuthPassword, Secret: "MatKhau@2026",
	}
}

func TestBuildSSHNode(t *testing.T) {
	nodes := newNodeService(t)

	record, err := nodes.build(context.Background(), model.Node{}, sshRequest(), 0)
	if err != nil {
		t.Fatalf("dựng bản ghi: %v", err)
	}

	if record.Kind != model.NodeSSH || record.Host != "203.0.113.10" || record.Port != 2222 {
		t.Errorf("bản ghi = %+v", record)
	}
	// Địa chỉ chỉ để hiển thị, nhưng cột này là not null từ thời panel mới chỉ
	// biết tới agent.
	if record.Address != "ssh://quantri@203.0.113.10:2222" {
		t.Errorf("địa chỉ hiển thị = %q", record.Address)
	}
	// Mật khẩu phải được mã hóa: cơ sở dữ liệu của panel không phải chỗ để
	// nguyên văn thứ mở toàn quyền một máy chủ khác.
	if record.Secret == "MatKhau@2026" || record.Secret == "" {
		t.Fatalf("mật khẩu lưu dạng %q", record.Secret)
	}
	opened, err := nodes.sealer.Open(record.Secret)
	if err != nil || opened != "MatKhau@2026" {
		t.Errorf("giải mã mật khẩu: %q, %v", opened, err)
	}
}

// Cổng để trống là cổng 22: đó là cổng SSH của gần như mọi VPS, và bắt người
// dùng gõ lại nó mỗi lần chỉ tạo thêm một chỗ để gõ nhầm.
func TestBuildSSHDefaultsPort(t *testing.T) {
	nodes := newNodeService(t)

	req := sshRequest()
	req.Port = 0

	record, err := nodes.build(context.Background(), model.Node{}, req, 0)
	if err != nil {
		t.Fatalf("dựng bản ghi: %v", err)
	}
	if record.Port != 22 {
		t.Errorf("cổng = %d, mong 22", record.Port)
	}
}

// Bí mật để trống khi sửa nghĩa là giữ nguyên: giao diện không bao giờ đọc lại
// được nó, nên coi ô trống là "xóa" sẽ làm hỏng kết nối mỗi lần sửa ghi chú.
func TestBuildSSHKeepsSecretWhenBlank(t *testing.T) {
	nodes := newNodeService(t)

	existing, err := nodes.build(context.Background(), model.Node{}, sshRequest(), 0)
	if err != nil {
		t.Fatalf("dựng bản ghi đầu: %v", err)
	}
	existing.Fingerprint = "SHA256:cu"

	req := sshRequest()
	req.Secret = ""
	req.Remark = "đổi ghi chú"

	updated, err := nodes.build(context.Background(), existing, req, 1)
	if err != nil {
		t.Fatalf("dựng bản ghi sau: %v", err)
	}
	if updated.Secret != existing.Secret {
		t.Error("bí mật bị mất khi chỉ sửa ghi chú")
	}
	if updated.Fingerprint != "SHA256:cu" {
		t.Errorf("dấu vân tay = %q, mong giữ nguyên", updated.Fingerprint)
	}
}

// Đổi địa chỉ nghĩa là đang trỏ tới một máy khác, nên dấu vân tay cũ không còn
// nghĩa gì — giữ lại thì lần kết nối sau bị từ chối oan.
func TestBuildSSHClearsFingerprintOnHostChange(t *testing.T) {
	nodes := newNodeService(t)

	existing, _ := nodes.build(context.Background(), model.Node{}, sshRequest(), 0)
	existing.Fingerprint = "SHA256:cu"

	req := sshRequest()
	req.Host = "198.51.100.20"

	updated, err := nodes.build(context.Background(), existing, req, 1)
	if err != nil {
		t.Fatalf("dựng bản ghi: %v", err)
	}
	if updated.Fingerprint != "" {
		t.Errorf("dấu vân tay = %q, mong bị xóa", updated.Fingerprint)
	}
}

// Bí mật mới có thể là của một máy khác, nên nó cũng xóa dấu vân tay cũ.
func TestBuildSSHClearsFingerprintOnNewSecret(t *testing.T) {
	nodes := newNodeService(t)

	existing, _ := nodes.build(context.Background(), model.Node{}, sshRequest(), 0)
	existing.Fingerprint = "SHA256:cu"

	req := sshRequest()
	req.Secret = "MatKhauKhac@2026"

	updated, _ := nodes.build(context.Background(), existing, req, 1)
	if updated.Fingerprint != "" {
		t.Errorf("dấu vân tay = %q, mong bị xóa", updated.Fingerprint)
	}
}

func TestBuildSSHRejectsBadInput(t *testing.T) {
	nodes := newNodeService(t)
	ctx := context.Background()

	bad := sshRequest()
	bad.Host = "203.0.113.10 ; rm -rf /"
	if _, err := nodes.build(ctx, model.Node{}, bad, 0); !errors.Is(err, apperr.NodeInvalidHost) {
		t.Errorf("địa chỉ có khoảng trắng: lỗi = %v", err)
	}

	bad = sshRequest()
	bad.Port = 70000
	if _, err := nodes.build(ctx, model.Node{}, bad, 0); !errors.Is(err, apperr.NodeInvalidHost) {
		t.Errorf("cổng ngoài khoảng: lỗi = %v", err)
	}

	bad = sshRequest()
	bad.User = "  "
	if _, err := nodes.build(ctx, model.Node{}, bad, 0); err == nil {
		t.Error("tài khoản rỗng lại được chấp nhận")
	}
}

// Kiểu để trống phải hiểu là agent: mọi bản ghi tạo trước khi có kiểu ssh đều
// không mang trường này, và đổi mặc định sẽ làm chúng ngừng kết nối.
func TestKindDefaultsToAgent(t *testing.T) {
	if got := kindOf(NodeRequest{}); got != model.NodeAgent {
		t.Errorf("kiểu mặc định = %q, mong agent", got)
	}
	if got := kindOf(NodeRequest{Kind: model.NodeSSH}); got != model.NodeSSH {
		t.Errorf("kiểu ssh = %q", got)
	}
}

// Thao tác chỉ dành cho SSH phải từ chối máy chủ nối bằng agent, thay vì thử
// mở một kết nối SSH mà máy đó có thể không mở cổng.
func TestExecRefusesAgentNode(t *testing.T) {
	nodes := newNodeService(t)
	ctx := context.Background()

	record := model.Node{Name: "node-agent", Kind: model.NodeAgent, Address: "https://10.0.0.5:9528"}
	if err := nodes.db.Create(&record).Error; err != nil {
		t.Fatalf("tạo bản ghi: %v", err)
	}

	_, err := nodes.Exec(ctx, record.ID, "uptime", AuditEntry{})
	if !errors.Is(err, apperr.NodeNotSSH) {
		t.Errorf("lỗi = %v, mong NodeNotSSH", err)
	}
}
