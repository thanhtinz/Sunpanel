package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/pkg/firewall"
)

// FirewallService quản lý tường lửa của máy chủ.
type FirewallService struct {
	manager firewall.Manager
	audit   *AuditService
	// panelPort là cổng panel đang lắng nghe, phải luôn giữ mở.
	panelPort int
}

// NewFirewallService tạo service quản lý tường lửa.
func NewFirewallService(manager firewall.Manager, panelPort int, audit *AuditService) *FirewallService {
	return &FirewallService{manager: manager, audit: audit, panelPort: panelPort}
}

// criticalPorts là các cổng mà panel từ chối chặn.
//
// Chặn cổng 22 hoặc cổng của chính panel qua giao diện web là cách nhanh nhất để
// mất đường vào máy chủ, và khác với việc dừng nhầm một dịch vụ, người dùng
// không còn cách nào tự sửa từ xa.
func (s *FirewallService) criticalPorts() map[string]string {
	return map[string]string{
		"22":                    "SSH",
		fmt.Sprint(s.panelPort): "SunPanel",
	}
}

// Status trả về tình trạng tường lửa.
func (s *FirewallService) Status(ctx context.Context) (firewall.Status, error) {
	status, err := s.manager.Status(ctx)
	if err != nil {
		return firewall.Status{}, translateFirewallError(err)
	}
	return status, nil
}

// ListRules liệt kê các quy tắc hiện có.
func (s *FirewallService) ListRules(ctx context.Context) ([]firewall.Rule, error) {
	rules, err := s.manager.ListRules(ctx)
	if err != nil {
		return nil, translateFirewallError(err)
	}
	return rules, nil
}

// AddRule thêm một quy tắc, sau khi kiểm tra nó không tự khóa quản trị viên ra ngoài.
func (s *FirewallService) AddRule(ctx context.Context, rule firewall.Rule, actor AuditEntry) error {
	if err := firewall.ValidateRule(rule); err != nil {
		return translateFirewallError(err)
	}
	if err := s.checkNotSelfLockout(rule); err != nil {
		return err
	}

	err := s.manager.AddRule(ctx, rule)

	actor.Action = "firewall.add_rule"
	actor.Resource = describeRule(rule)
	actor.Success = err == nil
	s.audit.Record(ctx, actor)

	return translateFirewallError(err)
}

// DeleteRule xóa một quy tắc theo ID.
func (s *FirewallService) DeleteRule(ctx context.Context, id string, actor AuditEntry) error {
	err := s.manager.DeleteRule(ctx, id)

	actor.Action = "firewall.delete_rule"
	actor.Resource = id
	actor.Success = err == nil
	s.audit.Record(ctx, actor)

	return translateFirewallError(err)
}

// SetEnabled bật hoặc tắt tường lửa.
//
// Trước khi bật, panel bảo đảm cổng SSH và cổng của chính nó đã được mở — ufw
// đóng mọi thứ theo mặc định, nên bật tường lửa mà chưa mở hai cổng này sẽ cắt
// đứt kết nối ngay lập tức.
func (s *FirewallService) SetEnabled(ctx context.Context, enabled bool, actor AuditEntry) error {
	var err error
	if enabled {
		if err = s.ensureCriticalPortsOpen(ctx); err != nil {
			return err
		}
		err = s.manager.Enable(ctx)
	} else {
		err = s.manager.Disable(ctx)
	}

	actor.Action = "firewall.set_enabled"
	actor.Resource = fmt.Sprintf("enabled=%v", enabled)
	actor.Success = err == nil
	s.audit.Record(ctx, actor)

	return translateFirewallError(err)
}

// ensureCriticalPortsOpen mở sẵn cổng SSH và cổng panel nếu chúng chưa được mở.
func (s *FirewallService) ensureCriticalPortsOpen(ctx context.Context) error {
	rules, err := s.manager.ListRules(ctx)
	if err != nil {
		return translateFirewallError(err)
	}

	open := make(map[string]bool)
	for _, rule := range rules {
		if rule.Action == firewall.ActionAllow && rule.Source == "" {
			open[rule.Port] = true
		}
	}

	for port := range s.criticalPorts() {
		if open[port] {
			continue
		}
		rule := firewall.Rule{
			Action:      firewall.ActionAllow,
			Protocol:    firewall.ProtocolTCP,
			Port:        port,
			Description: "SunPanel: giữ mở để không mất kết nối",
		}
		if err := s.manager.AddRule(ctx, rule); err != nil {
			return translateFirewallError(err)
		}
	}

	return nil
}

// checkNotSelfLockout từ chối các quy tắc chặn cổng quan trọng.
func (s *FirewallService) checkNotSelfLockout(rule firewall.Rule) error {
	if rule.Action == firewall.ActionAllow {
		return nil
	}

	for port, name := range s.criticalPorts() {
		// Quy tắc giới hạn theo nguồn cụ thể được phép: chặn một IP đang tấn công
		// không cắt đứt đường vào của quản trị viên.
		if rule.Port == port && rule.Source == "" {
			return apperr.FirewallCriticalPort.
				WithParam("port", port).
				WithParam("name", name)
		}
	}
	return nil
}

// describeRule mô tả ngắn gọn một quy tắc để ghi nhật ký.
func describeRule(rule firewall.Rule) string {
	source := rule.Source
	if source == "" {
		source = "any"
	}
	return fmt.Sprintf("%s %s/%s from %s", rule.Action, rule.Port, rule.Protocol, source)
}

// translateFirewallError chuyển lỗi của lớp firewall thành lỗi có mã dịch.
func translateFirewallError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, firewall.ErrNoBackend):
		return apperr.FirewallUnavailable.Wrap(err)
	case errors.Is(err, firewall.ErrInvalidRule):
		return apperr.FirewallInvalidRule.Wrap(err)
	case errors.Is(err, firewall.ErrRuleNotFound):
		return apperr.FirewallRuleNotFound.Wrap(err)
	default:
		return apperr.FirewallActionFailed.Wrap(err)
	}
}
