package firewall

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/thanhtinz/sunpanel/pkg/host"
)

// firewalldTimeout là thời gian tối đa cho một lệnh firewall-cmd.
const firewalldTimeout = 20 * time.Second

// Firewalld là driver tường lửa dùng firewall-cmd, mặc định trên RHEL, Fedora,
// Rocky và AlmaLinux.
type Firewalld struct {
	host host.Host
}

var _ Manager = (*Firewalld)(nil)

// NewFirewalld tạo driver firewalld.
func NewFirewalld(h host.Host) *Firewalld { return &Firewalld{host: h} }

// Name trả về tên công cụ.
func (f *Firewalld) Name() string { return "firewalld" }

// Status đọc tình trạng tường lửa.
func (f *Firewalld) Status(ctx context.Context) (Status, error) {
	status := Status{Backend: f.Name()}

	res, err := f.host.Exec(ctx, host.Command{
		Path: "firewall-cmd", Args: []string{"--state"}, Timeout: firewalldTimeout,
	})
	if err != nil {
		return status, nil
	}

	// firewall-cmd --state trả mã khác 0 khi dịch vụ chưa chạy, nhưng bản thân
	// việc lệnh chạy được đã chứng tỏ công cụ có trên máy.
	status.Available = true
	status.Enabled = strings.Contains(res.Stdout, "running")
	return status, nil
}

// Enable khởi động dịch vụ firewalld.
func (f *Firewalld) Enable(ctx context.Context) error {
	return f.runSystemctl(ctx, "start")
}

// Disable dừng dịch vụ firewalld.
func (f *Firewalld) Disable(ctx context.Context) error {
	return f.runSystemctl(ctx, "stop")
}

func (f *Firewalld) runSystemctl(ctx context.Context, action string) error {
	res, err := f.host.Exec(ctx, host.Command{
		Path: "systemctl", Args: []string{action, "firewalld"}, Timeout: firewalldTimeout,
	})
	if err != nil {
		return fmt.Errorf("%s firewalld: %w", action, err)
	}
	if !res.OK() {
		return fmt.Errorf("%s firewalld thất bại: %s", action, strings.TrimSpace(res.Stderr))
	}
	return nil
}

// ListRules liệt kê cổng đang mở và các quy tắc phong phú.
func (f *Firewalld) ListRules(ctx context.Context) ([]Rule, error) {
	ports, err := f.query(ctx, "--list-ports")
	if err != nil {
		return nil, err
	}
	rich, err := f.query(ctx, "--list-rich-rules")
	if err != nil {
		return nil, err
	}

	rules := parseFirewalldPorts(ports)
	return append(rules, parseFirewalldRichRules(rich)...), nil
}

func (f *Firewalld) query(ctx context.Context, arg string) (string, error) {
	res, err := f.host.Exec(ctx, host.Command{
		Path: "firewall-cmd", Args: []string{arg}, Timeout: firewalldTimeout,
	})
	if err != nil {
		return "", fmt.Errorf("chạy firewall-cmd %s: %w", arg, err)
	}
	if !res.OK() {
		return "", fmt.Errorf("%w: %s", ErrNoBackend, strings.TrimSpace(res.Stderr))
	}
	return res.Stdout, nil
}

// AddRule thêm một quy tắc.
//
// Quy tắc không giới hạn nguồn được thêm dưới dạng cổng mở đơn giản; quy tắc có
// nguồn hoặc hành động chặn phải dùng "rich rule" của firewalld.
func (f *Firewalld) AddRule(ctx context.Context, rule Rule) error {
	if err := ValidateRule(rule); err != nil {
		return err
	}

	var args []string
	if rule.Source == "" && rule.Action == ActionAllow && rule.Port != "" {
		args = []string{"--permanent", "--add-port=" + firewalldPort(rule)}
	} else {
		args = []string{"--permanent", "--add-rich-rule=" + buildRichRule(rule)}
	}

	if err := f.apply(ctx, args); err != nil {
		return err
	}
	// Thay đổi permanent chỉ có hiệu lực sau khi nạp lại cấu hình.
	return f.apply(ctx, []string{"--reload"})
}

// DeleteRule xóa một quy tắc theo định danh do driver sinh ra.
func (f *Firewalld) DeleteRule(ctx context.Context, id string) error {
	arg, ok := strings.CutPrefix(id, "port:")
	if ok {
		if err := f.apply(ctx, []string{"--permanent", "--remove-port=" + arg}); err != nil {
			return err
		}
		return f.apply(ctx, []string{"--reload"})
	}

	rich, ok := strings.CutPrefix(id, "rich:")
	if !ok {
		return fmt.Errorf("%w: %q", ErrRuleNotFound, id)
	}
	if err := f.apply(ctx, []string{"--permanent", "--remove-rich-rule=" + rich}); err != nil {
		return err
	}
	return f.apply(ctx, []string{"--reload"})
}

func (f *Firewalld) apply(ctx context.Context, args []string) error {
	res, err := f.host.Exec(ctx, host.Command{
		Path: "firewall-cmd", Args: args, Timeout: firewalldTimeout,
	})
	if err != nil {
		return fmt.Errorf("chạy firewall-cmd %v: %w", args, err)
	}
	if !res.OK() {
		return fmt.Errorf("firewall-cmd %v thất bại: %s", args, strings.TrimSpace(res.Stderr))
	}
	return nil
}

// firewalldPort dựng chuỗi cổng theo cú pháp firewalld ("80/tcp", "8000-8100/tcp").
func firewalldPort(rule Rule) string {
	protocol := rule.Protocol
	if protocol == ProtocolAny {
		// firewalld bắt buộc khai báo giao thức cho mỗi cổng.
		protocol = ProtocolTCP
	}
	// firewalld dùng dấu gạch nối cho dải cổng, khác dấu hai chấm của dạng chuẩn hóa.
	return strings.ReplaceAll(rule.Port, ":", "-") + "/" + string(protocol)
}

// buildRichRule dựng một rich rule của firewalld.
func buildRichRule(rule Rule) string {
	var b strings.Builder
	b.WriteString(`rule family="ipv4"`)

	if rule.Source != "" && rule.Source != "any" {
		fmt.Fprintf(&b, ` source address="%s"`, rule.Source)
	}
	if rule.Port != "" {
		protocol := rule.Protocol
		if protocol == ProtocolAny {
			protocol = ProtocolTCP
		}
		fmt.Fprintf(&b, ` port port="%s" protocol="%s"`,
			strings.ReplaceAll(rule.Port, ":", "-"), protocol)
	}

	switch rule.Action {
	case ActionAllow:
		b.WriteString(" accept")
	case ActionReject:
		b.WriteString(" reject")
	default:
		b.WriteString(" drop")
	}

	return b.String()
}

// parseFirewalldPorts đọc đầu ra của "firewall-cmd --list-ports".
func parseFirewalldPorts(output string) []Rule {
	var rules []Rule

	for _, entry := range strings.Fields(output) {
		port, proto, found := strings.Cut(entry, "/")
		if !found {
			continue
		}
		rules = append(rules, Rule{
			ID:     "port:" + entry,
			Action: ActionAllow,
			// Chuyển dải cổng về dạng chuẩn hóa dùng dấu hai chấm.
			Port:     strings.ReplaceAll(port, "-", ":"),
			Protocol: Protocol(proto),
			Raw:      entry,
		})
	}

	return rules
}

// parseFirewalldRichRules đọc đầu ra của "firewall-cmd --list-rich-rules".
func parseFirewalldRichRules(output string) []Rule {
	var rules []Rule

	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		rule := Rule{ID: "rich:" + trimmed, Raw: trimmed, Protocol: ProtocolAny}

		switch {
		case strings.Contains(trimmed, " accept"):
			rule.Action = ActionAllow
		case strings.Contains(trimmed, " reject"):
			rule.Action = ActionReject
		default:
			rule.Action = ActionDeny
		}

		rule.Source = extractQuoted(trimmed, `source address=`)
		rule.Port = strings.ReplaceAll(extractQuoted(trimmed, `port port=`), "-", ":")
		if protocol := extractQuoted(trimmed, `protocol=`); protocol != "" {
			rule.Protocol = Protocol(protocol)
		}

		rules = append(rules, rule)
	}

	return rules
}

// extractQuoted lấy giá trị trong dấu nháy kép ngay sau một tiền tố.
func extractQuoted(line, prefix string) string {
	_, after, found := strings.Cut(line, prefix+`"`)
	if !found {
		return ""
	}
	value, _, found := strings.Cut(after, `"`)
	if !found {
		return ""
	}
	return value
}
