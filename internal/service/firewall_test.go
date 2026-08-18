package service

import (
	"context"
	"errors"
	"testing"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/pkg/firewall"
)

// fakeFirewall là trình quản lý tường lửa giả, ghi lại thao tác đã gọi.
type fakeFirewall struct {
	rules   []firewall.Rule
	enabled bool
	added   []firewall.Rule
	deleted []string
	failNow error
}

func (f *fakeFirewall) Name() string { return "fake" }

func (f *fakeFirewall) Status(context.Context) (firewall.Status, error) {
	return firewall.Status{Backend: "fake", Available: true, Enabled: f.enabled}, f.failNow
}

func (f *fakeFirewall) Enable(context.Context) error {
	f.enabled = true
	return f.failNow
}

func (f *fakeFirewall) Disable(context.Context) error {
	f.enabled = false
	return f.failNow
}

func (f *fakeFirewall) ListRules(context.Context) ([]firewall.Rule, error) {
	return f.rules, f.failNow
}

func (f *fakeFirewall) AddRule(_ context.Context, rule firewall.Rule) error {
	if f.failNow != nil {
		return f.failNow
	}
	f.added = append(f.added, rule)
	f.rules = append(f.rules, rule)
	return nil
}

func (f *fakeFirewall) DeleteRule(_ context.Context, id string) error {
	f.deleted = append(f.deleted, id)
	return f.failNow
}

func newFirewallService(t *testing.T) (*FirewallService, *fakeFirewall) {
	t.Helper()
	manager := &fakeFirewall{}
	return NewFirewallService(manager, 9527, NewAuditService(newMemoryDB(t))), manager
}

// Chặn cổng SSH hoặc cổng panel qua giao diện web là mất đường vào máy chủ, và
// khác với dừng nhầm dịch vụ, người dùng không còn cách nào tự sửa từ xa.
func TestAddRuleBlocksSelfLockout(t *testing.T) {
	svc, manager := newFirewallService(t)
	ctx := context.Background()

	cases := []struct {
		name string
		rule firewall.Rule
	}{
		{"chặn SSH", firewall.Rule{Action: firewall.ActionDeny, Protocol: firewall.ProtocolTCP, Port: "22"}},
		{"từ chối SSH", firewall.Rule{Action: firewall.ActionReject, Protocol: firewall.ProtocolTCP, Port: "22"}},
		{"chặn cổng panel", firewall.Rule{Action: firewall.ActionDeny, Protocol: firewall.ProtocolTCP, Port: "9527"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.AddRule(ctx, tc.rule, AuditEntry{})
			if !errors.Is(err, apperr.FirewallCriticalPort) {
				t.Fatalf("lỗi = %v, mong đợi FirewallCriticalPort", err)
			}
		})
	}

	if len(manager.added) != 0 {
		t.Errorf("không quy tắc nào được phép áp dụng, nhưng đã thêm: %+v", manager.added)
	}
}

// Chặn một IP đang tấn công không cắt đứt đường vào của quản trị viên, nên quy
// tắc giới hạn theo nguồn cụ thể vẫn phải được phép.
func TestAddRuleAllowsBlockingSpecificSource(t *testing.T) {
	svc, manager := newFirewallService(t)

	rule := firewall.Rule{
		Action: firewall.ActionDeny, Protocol: firewall.ProtocolTCP,
		Port: "22", Source: "203.0.113.7",
	}
	if err := svc.AddRule(context.Background(), rule, AuditEntry{}); err != nil {
		t.Fatalf("chặn một IP cụ thể phải được phép, nhận lỗi: %v", err)
	}
	if len(manager.added) != 1 {
		t.Errorf("số quy tắc đã thêm = %d, mong đợi 1", len(manager.added))
	}
}

func TestAddRuleAllowsOpeningCriticalPort(t *testing.T) {
	svc, manager := newFirewallService(t)

	rule := firewall.Rule{Action: firewall.ActionAllow, Protocol: firewall.ProtocolTCP, Port: "22"}
	if err := svc.AddRule(context.Background(), rule, AuditEntry{}); err != nil {
		t.Fatalf("mở cổng SSH phải được phép, nhận lỗi: %v", err)
	}
	if len(manager.added) != 1 {
		t.Errorf("số quy tắc đã thêm = %d, mong đợi 1", len(manager.added))
	}
}

func TestAddRuleValidatesInput(t *testing.T) {
	svc, manager := newFirewallService(t)

	rule := firewall.Rule{Action: "xóa-hết", Protocol: firewall.ProtocolTCP, Port: "80"}
	if err := svc.AddRule(context.Background(), rule, AuditEntry{}); !errors.Is(err, apperr.FirewallInvalidRule) {
		t.Fatalf("lỗi = %v, mong đợi FirewallInvalidRule", err)
	}
	if len(manager.added) != 0 {
		t.Error("quy tắc không hợp lệ không được chạm tới trình quản lý")
	}
}

// ufw đóng mọi thứ theo mặc định, nên bật tường lửa mà chưa mở SSH và cổng panel
// sẽ cắt đứt kết nối ngay lập tức.
func TestSetEnabledOpensCriticalPortsFirst(t *testing.T) {
	svc, manager := newFirewallService(t)

	if err := svc.SetEnabled(context.Background(), true, AuditEntry{}); err != nil {
		t.Fatalf("bật tường lửa: %v", err)
	}

	opened := make(map[string]bool)
	for _, rule := range manager.added {
		opened[rule.Port] = true
	}
	for _, port := range []string{"22", "9527"} {
		if !opened[port] {
			t.Errorf("cổng %s phải được mở trước khi bật tường lửa", port)
		}
	}
	if !manager.enabled {
		t.Error("tường lửa phải được bật")
	}
}

// Cổng đã mở sẵn thì không thêm quy tắc trùng lặp.
func TestSetEnabledSkipsAlreadyOpenPorts(t *testing.T) {
	svc, manager := newFirewallService(t)
	manager.rules = []firewall.Rule{
		{Action: firewall.ActionAllow, Protocol: firewall.ProtocolTCP, Port: "22"},
		{Action: firewall.ActionAllow, Protocol: firewall.ProtocolTCP, Port: "9527"},
	}

	if err := svc.SetEnabled(context.Background(), true, AuditEntry{}); err != nil {
		t.Fatalf("bật tường lửa: %v", err)
	}
	if len(manager.added) != 0 {
		t.Errorf("không cần thêm quy tắc nào, nhưng đã thêm: %+v", manager.added)
	}
}

// Quy tắc chỉ mở cho một nguồn cụ thể không tính là cổng đã mở cho mọi nơi.
func TestSetEnabledIgnoresSourceScopedAllow(t *testing.T) {
	svc, manager := newFirewallService(t)
	manager.rules = []firewall.Rule{
		{Action: firewall.ActionAllow, Protocol: firewall.ProtocolTCP, Port: "22", Source: "10.0.0.1"},
	}

	if err := svc.SetEnabled(context.Background(), true, AuditEntry{}); err != nil {
		t.Fatalf("bật tường lửa: %v", err)
	}

	opened := make(map[string]bool)
	for _, rule := range manager.added {
		opened[rule.Port] = true
	}
	if !opened["22"] {
		t.Error("cổng 22 chỉ mở cho một nguồn thì vẫn phải được mở rộng ra trước khi bật")
	}
}

func TestTranslateFirewallErrors(t *testing.T) {
	svc, manager := newFirewallService(t)
	manager.failNow = firewall.ErrNoBackend

	if _, err := svc.ListRules(context.Background()); !errors.Is(err, apperr.FirewallUnavailable) {
		t.Fatalf("lỗi = %v, mong đợi FirewallUnavailable", err)
	}
}
