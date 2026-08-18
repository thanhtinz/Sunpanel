package sysservice

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/thanhtinz/sunpanel/pkg/host"
)

// Giới hạn khi làm việc với systemd.
const (
	// listTimeout là thời gian tối đa cho một lệnh liệt kê.
	listTimeout = 15 * time.Second
	// actionTimeout là thời gian tối đa cho một lệnh điều khiển dịch vụ.
	// Dịch vụ nặng như cơ sở dữ liệu có thể mất khá lâu để dừng sạch.
	actionTimeout = 90 * time.Second
	// maxLogLines giới hạn số dòng nhật ký lấy về một lần.
	maxLogLines = 2000
)

// validUnitName giới hạn tên dịch vụ được phép truyền cho systemctl.
//
// Tên đi thẳng vào tham số lệnh nên không có nguy cơ shell injection, nhưng vẫn
// phải chặn: một tên như "--now" hoặc "-h" sẽ bị systemctl hiểu là cờ dòng lệnh
// chứ không phải tên dịch vụ.
var validUnitName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.@:\\-]{0,127}$`)

// Systemd là trình quản lý dịch vụ dùng systemctl.
type Systemd struct {
	host host.Host
}

var _ Manager = (*Systemd)(nil)

// NewSystemd tạo trình quản lý dịch vụ systemd.
func NewSystemd(h host.Host) *Systemd { return &Systemd{host: h} }

// Name trả về tên hệ init.
func (s *Systemd) Name() string { return "systemd" }

// Available cho biết systemd có đang thực sự điều hành máy này không.
//
// Chỉ kiểm tra sự tồn tại của systemctl là không đủ: nhiều container có sẵn
// binary systemctl nhưng PID 1 lại là thứ khác, và mọi lệnh sẽ thất bại với
// "System has not been booted with systemd".
func (s *Systemd) Available(ctx context.Context) bool {
	res, err := s.host.Exec(ctx, host.Command{
		Path:    "systemctl",
		Args:    []string{"is-system-running"},
		Timeout: 5 * time.Second,
	})
	if err != nil {
		return false
	}

	// is-system-running trả mã khác 0 ở các trạng thái degraded hay starting,
	// nhưng systemd vẫn dùng được. Chỉ "offline" và "unknown" là thực sự hỏng.
	state := strings.TrimSpace(res.Stdout)
	switch state {
	case "offline", "unknown", "":
		return false
	default:
		return true
	}
}

// unitJSON là một mục trong đầu ra của systemctl list-units --output=json.
type unitJSON struct {
	Unit        string `json:"unit"`
	Load        string `json:"load"`
	Active      string `json:"active"`
	Sub         string `json:"sub"`
	Description string `json:"description"`
}

// unitFileJSON là một mục trong đầu ra của systemctl list-unit-files --output=json.
type unitFileJSON struct {
	UnitFile string `json:"unit_file"`
	State    string `json:"state"`
}

// List liệt kê toàn bộ dịch vụ kèm trạng thái tự khởi động.
func (s *Systemd) List(ctx context.Context) ([]Service, error) {
	res, err := s.host.Exec(ctx, host.Command{
		Path:    "systemctl",
		Args:    []string{"list-units", "--type=service", "--all", "--no-pager", "--output=json"},
		Timeout: listTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("liệt kê dịch vụ: %w", err)
	}
	if !res.OK() {
		return nil, fmt.Errorf("%w: %s", ErrManagerUnavailable, strings.TrimSpace(res.Stderr))
	}

	services, err := parseUnits(res.Stdout)
	if err != nil {
		return nil, err
	}

	// Trạng thái tự khởi động nằm ở lệnh khác. Không lấy được thì vẫn trả về danh
	// sách dịch vụ: biết dịch vụ đang chạy hay không quan trọng hơn.
	if enabled, err := s.enabledStates(ctx); err == nil {
		for i := range services {
			if state, ok := enabled[services[i].Name]; ok {
				services[i].EnabledState = state
				services[i].Enabled = state == "enabled" || state == "enabled-runtime"
			}
		}
	}

	return services, nil
}

// enabledStates đọc trạng thái tự khởi động của mọi unit.
func (s *Systemd) enabledStates(ctx context.Context) (map[string]string, error) {
	res, err := s.host.Exec(ctx, host.Command{
		Path:    "systemctl",
		Args:    []string{"list-unit-files", "--type=service", "--no-pager", "--output=json"},
		Timeout: listTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("đọc trạng thái tự khởi động: %w", err)
	}
	if !res.OK() {
		return nil, fmt.Errorf("%w", ErrManagerUnavailable)
	}

	return parseUnitFiles(res.Stdout)
}

// Get lấy thông tin một dịch vụ.
func (s *Systemd) Get(ctx context.Context, name string) (Service, error) {
	unit, err := normalizeUnit(name)
	if err != nil {
		return Service{}, err
	}

	services, err := s.List(ctx)
	if err != nil {
		return Service{}, err
	}
	for _, svc := range services {
		if svc.Name == unit {
			return svc, nil
		}
	}
	return Service{}, fmt.Errorf("%w: %s", ErrServiceNotFound, unit)
}

// Start khởi động dịch vụ.
func (s *Systemd) Start(ctx context.Context, name string) error {
	return s.control(ctx, "start", name)
}

// Stop dừng dịch vụ.
func (s *Systemd) Stop(ctx context.Context, name string) error {
	return s.control(ctx, "stop", name)
}

// Restart khởi động lại dịch vụ.
func (s *Systemd) Restart(ctx context.Context, name string) error {
	return s.control(ctx, "restart", name)
}

// Reload nạp lại cấu hình mà không dừng dịch vụ.
func (s *Systemd) Reload(ctx context.Context, name string) error {
	return s.control(ctx, "reload", name)
}

// Enable bật tự khởi động cùng máy chủ.
func (s *Systemd) Enable(ctx context.Context, name string) error {
	return s.control(ctx, "enable", name)
}

// Disable tắt tự khởi động cùng máy chủ.
func (s *Systemd) Disable(ctx context.Context, name string) error {
	return s.control(ctx, "disable", name)
}

func (s *Systemd) control(ctx context.Context, action, name string) error {
	unit, err := normalizeUnit(name)
	if err != nil {
		return err
	}

	res, err := s.host.Exec(ctx, host.Command{
		Path: "systemctl",
		// "--" chặn tuyệt đối việc tên dịch vụ bị hiểu thành cờ dòng lệnh.
		Args:    []string{action, "--no-pager", "--", unit},
		Timeout: actionTimeout,
	})
	if err != nil {
		return fmt.Errorf("%s dịch vụ %s: %w", action, unit, err)
	}
	if !res.OK() {
		message := strings.TrimSpace(res.Stderr)
		if message == "" {
			message = strings.TrimSpace(res.Stdout)
		}
		return fmt.Errorf("%s dịch vụ %s thất bại: %s", action, unit, message)
	}
	return nil
}

// Logs lấy các dòng nhật ký gần nhất của dịch vụ.
func (s *Systemd) Logs(ctx context.Context, name string, lines int) (string, error) {
	unit, err := normalizeUnit(name)
	if err != nil {
		return "", err
	}

	switch {
	case lines <= 0:
		lines = 200
	case lines > maxLogLines:
		lines = maxLogLines
	}

	res, err := s.host.Exec(ctx, host.Command{
		Path:    "journalctl",
		Args:    []string{"--no-pager", "--lines", fmt.Sprint(lines), "--unit", unit},
		Timeout: listTimeout,
	})
	if err != nil {
		return "", fmt.Errorf("đọc nhật ký dịch vụ %s: %w", unit, err)
	}
	if !res.OK() {
		return "", fmt.Errorf("đọc nhật ký dịch vụ %s thất bại: %s", unit, strings.TrimSpace(res.Stderr))
	}
	return res.Stdout, nil
}

// parseUnits đọc đầu ra JSON của systemctl list-units.
func parseUnits(output string) ([]Service, error) {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return nil, nil
	}

	var units []unitJSON
	if err := json.Unmarshal([]byte(trimmed), &units); err != nil {
		return nil, fmt.Errorf("đọc danh sách dịch vụ: %w", err)
	}

	services := make([]Service, 0, len(units))
	for _, unit := range units {
		// systemd liệt kê cả unit không phải service khi có mẫu trùng; lọc lại
		// để giao diện không hiện timer hay socket lẫn vào.
		if !strings.HasSuffix(unit.Unit, ".service") {
			continue
		}
		services = append(services, Service{
			Name:        unit.Unit,
			Description: unit.Description,
			State:       parseState(unit.Active),
			SubState:    unit.Sub,
			Loaded:      unit.Load == "loaded",
		})
	}
	return services, nil
}

// parseUnitFiles đọc đầu ra JSON của systemctl list-unit-files.
func parseUnitFiles(output string) (map[string]string, error) {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return map[string]string{}, nil
	}

	var files []unitFileJSON
	if err := json.Unmarshal([]byte(trimmed), &files); err != nil {
		return nil, fmt.Errorf("đọc trạng thái tự khởi động: %w", err)
	}

	states := make(map[string]string, len(files))
	for _, file := range files {
		states[file.UnitFile] = file.State
	}
	return states, nil
}

// parseState chuyển chuỗi trạng thái của systemd thành State.
func parseState(active string) State {
	switch State(active) {
	case StateActive, StateInactive, StateFailed, StateActivating, StateDeactivating:
		return State(active)
	default:
		return StateUnknown
	}
}

// NormalizeUnit kiểm tra tên dịch vụ và thêm hậu tố .service nếu thiếu.
//
// Hàm được xuất ra vì lớp nghiệp vụ cần chuẩn hóa tên TRƯỚC khi gọi trình quản
// lý — chẳng hạn để đối chiếu với danh sách dịch vụ được bảo vệ. Nếu việc chuẩn
// hóa phải nhờ trình quản lý, thì khi systemd không phản hồi hoặc dịch vụ không
// nằm trong danh sách, lớp bảo vệ sẽ bị vượt qua.
func NormalizeUnit(name string) (string, error) {
	return normalizeUnit(name)
}

// normalizeUnit kiểm tra tên dịch vụ và thêm hậu tố .service nếu thiếu.
func normalizeUnit(name string) (string, error) {
	unit := strings.TrimSpace(name)
	if !validUnitName.MatchString(unit) {
		return "", fmt.Errorf("%w: %q", ErrInvalidName, name)
	}
	// Chỉ thêm .service khi tên chưa mang hậu tố unit nào; "nginx.socket" phải
	// được giữ nguyên chứ không biến thành "nginx.socket.service".
	if !strings.Contains(unit, ".") {
		unit += ".service"
	}
	return unit, nil
}
