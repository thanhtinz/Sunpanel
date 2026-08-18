package firewall

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/thanhtinz/sunpanel/pkg/host"
)

// ufwTimeout là thời gian tối đa cho một lệnh ufw.
const ufwTimeout = 20 * time.Second

// ufwRuleLine khớp một dòng quy tắc trong đầu ra của "ufw status numbered".
//
// Ví dụ:
//
//	[ 1] 22/tcp                     ALLOW IN    Anywhere
//	[ 6] 22/tcp (v6)                ALLOW IN    Anywhere (v6)
//
// Phần "(v6)" là bắt buộc phải xử lý: ufw tự sinh một dòng IPv6 song song cho
// mỗi quy tắc IPv4, nên bỏ qua chúng nghĩa là giấu mất một nửa số quy tắc thật.
var ufwRuleLine = regexp.MustCompile(
	`^\[\s*(\d+)\]\s+(\S+)(\s+\(v6\))?\s+(ALLOW|DENY|REJECT|LIMIT)\s+(?:IN|OUT)\s*(.*)$`,
)

// UFW là driver tường lửa dùng ufw, mặc định trên Debian và Ubuntu.
type UFW struct {
	host host.Host
}

var _ Manager = (*UFW)(nil)

// NewUFW tạo driver ufw.
func NewUFW(h host.Host) *UFW { return &UFW{host: h} }

// Name trả về tên công cụ.
func (u *UFW) Name() string { return "ufw" }

// Status đọc tình trạng tường lửa.
func (u *UFW) Status(ctx context.Context) (Status, error) {
	status := Status{Backend: u.Name()}

	res, err := u.host.Exec(ctx, host.Command{
		Path: "ufw", Args: []string{"status"}, Timeout: ufwTimeout,
	})
	if err != nil {
		// Lệnh không chạy được nghĩa là công cụ không có trên máy.
		return status, nil
	}

	status.Available = res.OK()
	status.Enabled = strings.Contains(res.Stdout, "Status: active")
	return status, nil
}

// Enable bật tường lửa.
//
// Truyền "y" vào đầu vào chuẩn vì ufw hỏi xác nhận khi bật, và câu hỏi đó cảnh
// báo có thể ngắt kết nối SSH hiện tại. Panel đã bảo đảm cổng SSH được mở trước
// khi tới bước này.
func (u *UFW) Enable(ctx context.Context) error {
	return u.run(ctx, []string{"--force", "enable"})
}

// Disable tắt tường lửa.
func (u *UFW) Disable(ctx context.Context) error {
	return u.run(ctx, []string{"disable"})
}

// ListRules liệt kê quy tắc kèm số thứ tự để xóa được.
func (u *UFW) ListRules(ctx context.Context) ([]Rule, error) {
	res, err := u.host.Exec(ctx, host.Command{
		Path: "ufw", Args: []string{"status", "numbered"}, Timeout: ufwTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("liệt kê quy tắc: %w", err)
	}
	if !res.OK() {
		return nil, fmt.Errorf("%w: %s", ErrNoBackend, strings.TrimSpace(res.Stderr))
	}
	return parseUFWRules(res.Stdout), nil
}

// AddRule thêm một quy tắc.
func (u *UFW) AddRule(ctx context.Context, rule Rule) error {
	if err := ValidateRule(rule); err != nil {
		return err
	}
	return u.run(ctx, buildUFWArgs(rule))
}

// DeleteRule xóa quy tắc theo số thứ tự do ufw gán.
func (u *UFW) DeleteRule(ctx context.Context, id string) error {
	// Số thứ tự đi vào tham số lệnh nên phải là số thật, không phải chuỗi tùy ý.
	if _, err := strconv.Atoi(id); err != nil {
		return fmt.Errorf("%w: số thứ tự %q", ErrInvalidRule, id)
	}
	return u.run(ctx, []string{"--force", "delete", id})
}

func (u *UFW) run(ctx context.Context, args []string) error {
	res, err := u.host.Exec(ctx, host.Command{
		Path: "ufw", Args: args, Timeout: ufwTimeout,
	})
	if err != nil {
		return fmt.Errorf("chạy ufw %v: %w", args, err)
	}
	if !res.OK() {
		message := strings.TrimSpace(res.Stderr)
		if message == "" {
			message = strings.TrimSpace(res.Stdout)
		}
		return fmt.Errorf("ufw %v thất bại: %s", args, message)
	}
	return nil
}

// buildUFWArgs dựng tham số dòng lệnh cho một quy tắc.
//
// Quy tắc đã qua ValidateRule nên mọi giá trị ở đây đều thuộc tập an toàn.
func buildUFWArgs(rule Rule) []string {
	args := []string{string(rule.Action)}

	if rule.Source != "" && rule.Source != "any" {
		args = append(args, "from", rule.Source)
	}

	if rule.Port != "" {
		if rule.Source != "" && rule.Source != "any" {
			// Cú pháp đầy đủ của ufw bắt buộc có "to any" khi đã khai báo "from".
			args = append(args, "to", "any")
		}
		// ufw dùng dấu hai chấm cho dải cổng, trùng với dạng chuẩn hóa nội bộ.
		args = append(args, "port", rule.Port)
	}

	if rule.Protocol != ProtocolAny {
		args = append(args, "proto", string(rule.Protocol))
	}

	if rule.Description != "" {
		args = append(args, "comment", rule.Description)
	}

	return args
}

// parseUFWRules đọc đầu ra của "ufw status numbered".
func parseUFWRules(output string) []Rule {
	var rules []Rule

	for _, line := range strings.Split(output, "\n") {
		match := ufwRuleLine.FindStringSubmatch(strings.TrimSpace(line))
		if match == nil {
			continue
		}

		port, protocol := splitUFWTarget(match[2])
		rules = append(rules, Rule{
			ID:       match[1],
			Action:   parseUFWAction(match[4]),
			Protocol: protocol,
			Port:     port,
			Source:   parseUFWSource(match[5]),
			IPv6:     match[3] != "",
			Raw:      strings.TrimSpace(line),
		})
	}

	return rules
}

// splitUFWTarget tách "22/tcp" thành cổng và giao thức.
func splitUFWTarget(target string) (string, Protocol) {
	port, proto, found := strings.Cut(target, "/")
	if !found {
		return target, ProtocolAny
	}

	switch Protocol(proto) {
	case ProtocolTCP:
		return port, ProtocolTCP
	case ProtocolUDP:
		return port, ProtocolUDP
	default:
		return port, ProtocolAny
	}
}

func parseUFWAction(value string) Action {
	switch value {
	case "DENY":
		return ActionDeny
	case "REJECT":
		return ActionReject
	default:
		// LIMIT là biến thể của ALLOW có giới hạn tần suất; với panel nó là cho phép.
		return ActionAllow
	}
}

// parseUFWSource chuẩn hóa cột nguồn; ufw ghi "Anywhere" thay cho mọi địa chỉ.
func parseUFWSource(value string) string {
	source := strings.TrimSpace(value)
	if source == "" || strings.HasPrefix(source, "Anywhere") {
		return ""
	}
	return source
}
