package service

import (
	"context"
	"errors"
	"testing"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/pkg/sysservice"
)

// fakeManager là trình quản lý dịch vụ giả, ghi lại các thao tác đã được gọi.
type fakeManager struct {
	services  []sysservice.Service
	available bool
	calls     []string
	failWith  error
}

func (f *fakeManager) Name() string                   { return "fake" }
func (f *fakeManager) Available(context.Context) bool { return f.available }
func (f *fakeManager) List(context.Context) ([]sysservice.Service, error) {
	if f.failWith != nil {
		return nil, f.failWith
	}
	return f.services, nil
}

func (f *fakeManager) Get(_ context.Context, name string) (sysservice.Service, error) {
	unit := name
	if !hasDot(unit) {
		unit += ".service"
	}
	for _, svc := range f.services {
		if svc.Name == unit {
			return svc, nil
		}
	}
	return sysservice.Service{}, sysservice.ErrServiceNotFound
}

func (f *fakeManager) record(action, name string) error {
	f.calls = append(f.calls, action+":"+name)
	return f.failWith
}

func (f *fakeManager) Start(_ context.Context, n string) error   { return f.record("start", n) }
func (f *fakeManager) Stop(_ context.Context, n string) error    { return f.record("stop", n) }
func (f *fakeManager) Restart(_ context.Context, n string) error { return f.record("restart", n) }
func (f *fakeManager) Reload(_ context.Context, n string) error  { return f.record("reload", n) }
func (f *fakeManager) Enable(_ context.Context, n string) error  { return f.record("enable", n) }
func (f *fakeManager) Disable(_ context.Context, n string) error { return f.record("disable", n) }
func (f *fakeManager) Logs(context.Context, string, int) (string, error) {
	return "dòng nhật ký mẫu", f.failWith
}

func hasDot(s string) bool {
	for _, r := range s {
		if r == '.' {
			return true
		}
	}
	return false
}

func newSysService(t *testing.T) (*SystemServiceManager, *fakeManager) {
	t.Helper()

	manager := &fakeManager{
		available: true,
		services: []sysservice.Service{
			{Name: "nginx.service", State: sysservice.StateActive},
			{Name: "mysql.service", State: sysservice.StateFailed},
			{Name: "apt-daily.service", State: sysservice.StateInactive},
			{Name: "ssh.service", State: sysservice.StateActive},
		},
	}
	return NewSystemServiceManager(manager, NewAuditService(newMemoryDB(t))), manager
}

// Dịch vụ lỗi phải hiện lên đầu vì đó là thứ quản trị viên cần xử lý ngay.
func TestListSortsFailedFirst(t *testing.T) {
	svc, _ := newSysService(t)

	list, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("liệt kê: %v", err)
	}
	if list[0].Name != "mysql.service" {
		t.Errorf("dịch vụ đầu tiên = %q, mong đợi mysql.service (đang lỗi)", list[0].Name)
	}
	if list[len(list)-1].Name != "apt-daily.service" {
		t.Errorf("dịch vụ cuối = %q, mong đợi apt-daily.service (đã dừng)", list[len(list)-1].Name)
	}
}

// Dừng SSH qua giao diện web là cách nhanh nhất để tự khóa mình khỏi máy chủ.
func TestControlBlocksStoppingProtectedService(t *testing.T) {
	svc, manager := newSysService(t)
	ctx := context.Background()

	for _, action := range []Action{ActionStop, ActionDisable} {
		t.Run(string(action), func(t *testing.T) {
			err := svc.Control(ctx, action, "ssh.service", AuditEntry{})
			if !errors.Is(err, apperr.ServiceProtected) {
				t.Fatalf("lỗi = %v, mong đợi ServiceProtected", err)
			}
		})
	}

	if len(manager.calls) != 0 {
		t.Errorf("không thao tác nào được phép chạm tới trình quản lý, nhưng đã gọi: %v", manager.calls)
	}
}

// Tên không có hậu tố cũng phải bị chặn — nếu chỉ so chuỗi thô thì "ssh" lọt lưới.
func TestControlBlocksProtectedServiceWithoutSuffix(t *testing.T) {
	svc, manager := newSysService(t)

	err := svc.Control(context.Background(), ActionStop, "ssh", AuditEntry{})
	if !errors.Is(err, apperr.ServiceProtected) {
		t.Fatalf("lỗi = %v, mong đợi ServiceProtected", err)
	}
	if len(manager.calls) != 0 {
		t.Errorf("trình quản lý không được gọi, nhưng đã gọi: %v", manager.calls)
	}
}

// Khởi động lại SSH là thao tác hợp lệ và tự phục hồi, không bị chặn.
func TestControlAllowsRestartingProtectedService(t *testing.T) {
	svc, manager := newSysService(t)

	if err := svc.Control(context.Background(), ActionRestart, "ssh.service", AuditEntry{}); err != nil {
		t.Fatalf("khởi động lại ssh phải được phép, nhận lỗi: %v", err)
	}
	if len(manager.calls) != 1 || manager.calls[0] != "restart:ssh.service" {
		t.Errorf("thao tác đã gọi = %v", manager.calls)
	}
}

func TestControlOnNormalService(t *testing.T) {
	svc, manager := newSysService(t)
	ctx := context.Background()

	cases := []struct {
		action Action
		want   string
	}{
		{ActionStart, "start:nginx.service"},
		{ActionStop, "stop:nginx.service"},
		{ActionRestart, "restart:nginx.service"},
		{ActionEnable, "enable:nginx.service"},
		{ActionDisable, "disable:nginx.service"},
	}

	for _, tc := range cases {
		manager.calls = nil
		if err := svc.Control(ctx, tc.action, "nginx.service", AuditEntry{}); err != nil {
			t.Fatalf("%s: %v", tc.action, err)
		}
		if len(manager.calls) != 1 || manager.calls[0] != tc.want {
			t.Errorf("%s: gọi = %v, mong đợi [%s]", tc.action, manager.calls, tc.want)
		}
	}
}

func TestControlRejectsUnknownAction(t *testing.T) {
	svc, _ := newSysService(t)

	err := svc.Control(context.Background(), Action("xóa-sạch"), "nginx.service", AuditEntry{})
	if !errors.Is(err, apperr.BadRequest) {
		t.Fatalf("lỗi = %v, mong đợi BadRequest", err)
	}
}

// Máy không có systemd phải báo lỗi rõ ràng thay vì lỗi nội bộ khó hiểu.
func TestListTranslatesUnavailableManager(t *testing.T) {
	svc, manager := newSysService(t)
	manager.failWith = sysservice.ErrManagerUnavailable

	if _, err := svc.List(context.Background()); !errors.Is(err, apperr.ServiceManagerUnavailable) {
		t.Fatalf("lỗi = %v, mong đợi ServiceManagerUnavailable", err)
	}
}

func TestGetTranslatesNotFound(t *testing.T) {
	svc, _ := newSysService(t)

	if _, err := svc.Get(context.Background(), "khong-ton-tai"); !errors.Is(err, apperr.ServiceNotFound) {
		t.Fatalf("lỗi = %v, mong đợi ServiceNotFound", err)
	}
}

func TestStatus(t *testing.T) {
	svc, manager := newSysService(t)

	if status := svc.Status(context.Background()); !status.Available || status.Manager != "fake" {
		t.Errorf("trạng thái = %+v", status)
	}

	manager.available = false
	if status := svc.Status(context.Background()); status.Available {
		t.Error("phải báo không khả dụng khi trình quản lý không dùng được")
	}
}

// Lớp bảo vệ không được phụ thuộc vào việc liệt kê dịch vụ thành công.
//
// Nếu chuẩn hóa tên phải nhờ trình quản lý, thì khi systemd không phản hồi hoặc
// dịch vụ không nằm trong danh sách, "ssh" sẽ không khớp "ssh.service" và panel
// sẽ dừng đúng cái dịch vụ mà nó phải bảo vệ.
func TestProtectionWorksWhenManagerCannotList(t *testing.T) {
	svc, manager := newSysService(t)
	manager.failWith = sysservice.ErrManagerUnavailable

	for _, name := range []string{"ssh", "ssh.service", "sshd", "sunpanel"} {
		t.Run(name, func(t *testing.T) {
			err := svc.Control(context.Background(), ActionStop, name, AuditEntry{})
			if !errors.Is(err, apperr.ServiceProtected) {
				t.Fatalf("lỗi = %v, mong đợi ServiceProtected", err)
			}
		})
	}

	if len(manager.calls) != 0 {
		t.Errorf("trình quản lý không được gọi, nhưng đã gọi: %v", manager.calls)
	}
}

// Tên không hợp lệ phải bị chặn trước khi chạm tới trình quản lý.
func TestControlRejectsInvalidNameBeforeCallingManager(t *testing.T) {
	svc, manager := newSysService(t)

	for _, name := range []string{"--now", "-h", "nginx; rm -rf /", ""} {
		err := svc.Control(context.Background(), ActionStop, name, AuditEntry{})
		if !errors.Is(err, apperr.ServiceInvalidName) {
			t.Errorf("tên %q: lỗi = %v, mong đợi ServiceInvalidName", name, err)
		}
	}
	if len(manager.calls) != 0 {
		t.Errorf("trình quản lý không được gọi, nhưng đã gọi: %v", manager.calls)
	}
}
