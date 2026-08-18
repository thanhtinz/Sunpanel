// Package firewall quản lý tường lửa của máy chủ.
//
// Mỗi bản Linux dùng một công cụ khác nhau (ufw trên Ubuntu, firewalld trên
// RHEL, nftables ở nơi khác), nên package định nghĩa một giao diện chung và
// chọn driver phù hợp lúc chạy.
package firewall

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Các lỗi dùng chung của tường lửa.
var (
	// ErrNoBackend là máy chủ không có công cụ tường lửa nào dùng được.
	ErrNoBackend = errors.New("firewall: không tìm thấy công cụ tường lửa khả dụng")
	// ErrInvalidRule là quy tắc không hợp lệ.
	ErrInvalidRule = errors.New("firewall: quy tắc không hợp lệ")
	// ErrRuleNotFound là không tìm thấy quy tắc cần xóa.
	ErrRuleNotFound = errors.New("firewall: không tìm thấy quy tắc")
)

// Action là hành động của một quy tắc.
type Action string

// Các hành động được hỗ trợ.
const (
	// ActionAllow cho phép lưu lượng đi qua.
	ActionAllow Action = "allow"
	// ActionDeny chặn lưu lượng, âm thầm bỏ gói tin.
	ActionDeny Action = "deny"
	// ActionReject chặn lưu lượng và báo lại cho bên gửi.
	ActionReject Action = "reject"
)

// Valid cho biết hành động có được hỗ trợ hay không.
func (a Action) Valid() bool {
	switch a {
	case ActionAllow, ActionDeny, ActionReject:
		return true
	}
	return false
}

// Protocol là giao thức của quy tắc.
type Protocol string

// Các giao thức được hỗ trợ.
const (
	// ProtocolTCP, ProtocolUDP giới hạn quy tắc theo giao thức.
	ProtocolTCP Protocol = "tcp"
	ProtocolUDP Protocol = "udp"
	// ProtocolAny áp dụng cho mọi giao thức.
	ProtocolAny Protocol = "any"
)

// Valid cho biết giao thức có được hỗ trợ hay không.
func (p Protocol) Valid() bool {
	switch p {
	case ProtocolTCP, ProtocolUDP, ProtocolAny:
		return true
	}
	return false
}

// Rule là một quy tắc tường lửa.
type Rule struct {
	// ID là định danh do driver gán, dùng để xóa quy tắc.
	ID string `json:"id"`
	// Action là cho phép hay chặn.
	Action Action `json:"action"`
	// Protocol là giao thức áp dụng.
	Protocol Protocol `json:"protocol"`
	// Port là cổng hoặc dải cổng, ví dụ "80" hay "8000:8100". Rỗng nghĩa là mọi cổng.
	Port string `json:"port"`
	// Source là địa chỉ hoặc dải CIDR nguồn. Rỗng nghĩa là mọi nguồn.
	Source string `json:"source"`
	// Description là ghi chú do người dùng đặt.
	Description string `json:"description"`
	// IPv6 cho biết quy tắc áp dụng cho IPv6. ufw tự sinh một dòng IPv6 song song
	// cho mỗi quy tắc IPv4, nên giao diện cần phân biệt để danh sách không trông
	// như bị lặp.
	IPv6 bool `json:"ipv6"`
	// Raw là dòng gốc từ công cụ tường lửa, giữ lại để tra cứu khi cần.
	Raw string `json:"raw,omitempty"`
}

// Status là tình trạng tường lửa của máy chủ.
type Status struct {
	// Backend là tên công cụ đang dùng, ví dụ "ufw".
	Backend string `json:"backend"`
	// Available cho biết công cụ có cài trên máy không.
	Available bool `json:"available"`
	// Enabled cho biết tường lửa có đang bật không.
	Enabled bool `json:"enabled"`
}

// Manager là trình quản lý tường lửa.
type Manager interface {
	// Name là tên công cụ tường lửa.
	Name() string
	// Status trả về tình trạng hiện tại.
	Status(ctx context.Context) (Status, error)
	// Enable và Disable bật/tắt tường lửa.
	Enable(ctx context.Context) error
	Disable(ctx context.Context) error
	// ListRules liệt kê các quy tắc đang có.
	ListRules(ctx context.Context) ([]Rule, error)
	// AddRule thêm một quy tắc.
	AddRule(ctx context.Context, rule Rule) error
	// DeleteRule xóa quy tắc theo ID.
	DeleteRule(ctx context.Context, id string) error
}

// ValidateRule kiểm tra một quy tắc trước khi đưa xuống driver.
//
// Việc kiểm tra nằm ở đây chứ không ở từng driver: mọi driver đều cần đúng các
// ràng buộc này, và bỏ sót ở một driver nghĩa là truyền chuỗi tùy ý vào tham số
// dòng lệnh của công cụ tường lửa.
func ValidateRule(rule Rule) error {
	if !rule.Action.Valid() {
		return fmt.Errorf("%w: hành động %q", ErrInvalidRule, rule.Action)
	}
	if !rule.Protocol.Valid() {
		return fmt.Errorf("%w: giao thức %q", ErrInvalidRule, rule.Protocol)
	}
	if err := validatePort(rule.Port); err != nil {
		return err
	}
	return validateSource(rule.Source)
}

// validatePort chấp nhận cổng đơn ("80"), dải cổng ("8000:8100") hoặc rỗng.
func validatePort(port string) error {
	if port == "" {
		return nil
	}

	parts := strings.Split(port, ":")
	if len(parts) > 2 {
		return fmt.Errorf("%w: cổng %q", ErrInvalidRule, port)
	}

	var previous int
	for i, part := range parts {
		value, err := strconv.Atoi(part)
		if err != nil || value < 1 || value > 65535 {
			return fmt.Errorf("%w: cổng %q", ErrInvalidRule, port)
		}
		// Dải cổng phải tăng dần, nếu không công cụ tường lửa sẽ lặng lẽ bỏ qua.
		if i == 1 && value <= previous {
			return fmt.Errorf("%w: dải cổng %q không tăng dần", ErrInvalidRule, port)
		}
		previous = value
	}
	return nil
}

// validateSource chấp nhận địa chỉ IP, dải CIDR hoặc rỗng.
func validateSource(source string) error {
	if source == "" || source == "any" {
		return nil
	}
	if !isIPOrCIDR(source) {
		return fmt.Errorf("%w: nguồn %q", ErrInvalidRule, source)
	}
	return nil
}
