package sysuser

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/thanhtinz/sunpanel/pkg/host"
)

// Các lỗi của trình quản lý tài khoản.
var (
	// ErrUnavailable là máy không có công cụ quản lý tài khoản (Windows, hoặc
	// bản Linux tối giản không có useradd).
	ErrUnavailable = errors.New("sysuser: máy này không quản lý tài khoản được")
	// ErrInvalidName là tên tài khoản không hợp lệ.
	ErrInvalidName = errors.New("sysuser: tên tài khoản không hợp lệ")
	// ErrNotFound là không tìm thấy tài khoản.
	ErrNotFound = errors.New("sysuser: không tìm thấy tài khoản")
	// ErrProtected là tài khoản panel từ chối đụng vào.
	ErrProtected = errors.New("sysuser: tài khoản được bảo vệ")
	// ErrExists là tên tài khoản đã có người dùng.
	ErrExists = errors.New("sysuser: tài khoản đã tồn tại")
)

// protected là các tài khoản panel không cho khóa hay xóa.
//
// Xóa root là hỏng máy; xóa chính tài khoản đang chạy panel hoặc sshd là tự cắt
// đường vào. Đổi mật khẩu thì vẫn cho, vì đó là thao tác hợp lệ và phục hồi được.
var protected = map[string]bool{"root": true, "daemon": true, "sshd": true, "systemd-network": true}

// Manager thao tác với tài khoản hệ điều hành qua lớp host.
type Manager struct {
	host host.Host
}

// New tạo trình quản lý tài khoản.
func New(h host.Host) *Manager { return &Manager{host: h} }

// CreateRequest là yêu cầu tạo tài khoản mới.
type CreateRequest struct {
	Name     string
	Comment  string
	Shell    string
	Password string
	// CreateHome tạo thư mục nhà cho tài khoản.
	CreateHome bool
	// Sudo thêm tài khoản vào nhóm quản trị.
	Sudo bool
}

// Available cho biết máy này quản lý tài khoản được.
func (m *Manager) Available(ctx context.Context) bool {
	_, err := m.host.FS().Stat(ctx, "/etc/passwd")
	return err == nil
}

// List liệt kê tài khoản, kèm trạng thái khóa, nhóm và số khóa SSH.
func (m *Manager) List(ctx context.Context) ([]User, error) {
	passwd, err := m.read(ctx, "/etc/passwd")
	if err != nil {
		return nil, ErrUnavailable
	}

	users := ParsePasswd(passwd)

	// /etc/shadow chỉ root đọc được. Không đọc được thì vẫn liệt kê tài khoản,
	// chỉ là không biết cái nào bị khóa — mất một cột còn hơn mất cả trang.
	if shadow, err := m.read(ctx, "/etc/shadow"); err == nil {
		states := ParseShadow(shadow)
		for i := range users {
			if state, ok := states[users[i].Name]; ok {
				users[i].Locked, users[i].NoPassword = state.Locked, state.NoPassword
			}
		}
	}

	if groups, err := m.read(ctx, "/etc/group"); err == nil {
		membership := ParseGroups(groups)
		for i := range users {
			users[i].Groups = membership[users[i].Name]
			for _, group := range users[i].Groups {
				if sudoGroups[group] {
					users[i].Sudo = true
				}
			}
		}
	}

	for i := range users {
		users[i].Keys = len(m.keysOf(ctx, users[i]))
	}

	// Tài khoản của người lên trước, rồi mới tới tài khoản hệ thống: trang này
	// mở ra để làm việc với người, còn hàng chục tài khoản dịch vụ chỉ là nền.
	sort.Slice(users, func(i, j int) bool {
		if users[i].System != users[j].System {
			return !users[i].System
		}
		return users[i].UID < users[j].UID
	})
	return users, nil
}

// Get lấy một tài khoản theo tên.
func (m *Manager) Get(ctx context.Context, name string) (User, error) {
	users, err := m.List(ctx)
	if err != nil {
		return User{}, err
	}
	for _, user := range users {
		if user.Name == name {
			return user, nil
		}
	}
	return User{}, ErrNotFound
}

// Create tạo tài khoản mới và đặt mật khẩu nếu có.
func (m *Manager) Create(ctx context.Context, req CreateRequest) error {
	if !ValidName(req.Name) {
		return ErrInvalidName
	}
	if _, err := m.Get(ctx, req.Name); err == nil {
		return ErrExists
	}

	args := []string{}
	if req.CreateHome {
		args = append(args, "-m")
	} else {
		args = append(args, "-M")
	}
	if shell := strings.TrimSpace(req.Shell); shell != "" {
		args = append(args, "-s", shell)
	}
	if comment := strings.TrimSpace(req.Comment); comment != "" {
		args = append(args, "-c", comment)
	}
	if req.Sudo {
		if group := m.sudoGroup(ctx); group != "" {
			args = append(args, "-G", group)
		}
	}
	args = append(args, req.Name)

	if err := m.run(ctx, "useradd", args...); err != nil {
		return err
	}
	if req.Password == "" {
		return nil
	}
	return m.SetPassword(ctx, req.Name, req.Password)
}

// SetPassword đặt lại mật khẩu của một tài khoản.
//
// Mật khẩu đi qua đầu vào chuẩn của chpasswd chứ không qua tham số dòng lệnh:
// tham số dòng lệnh hiện ra trong `ps` cho mọi tiến trình trên máy đọc được.
func (m *Manager) SetPassword(ctx context.Context, name, password string) error {
	if !ValidName(name) {
		return ErrInvalidName
	}
	if password == "" {
		return errors.New("sysuser: mật khẩu trống")
	}

	result, err := m.host.Exec(ctx, host.Command{
		Path:  "chpasswd",
		Stdin: []byte(name + ":" + password + "\n"),
	})
	if err != nil {
		return err
	}
	if !result.OK() {
		return fmt.Errorf("chpasswd: %s", strings.TrimSpace(result.Stderr))
	}
	return nil
}

// SetLocked khóa hoặc mở khóa đăng nhập bằng mật khẩu.
func (m *Manager) SetLocked(ctx context.Context, name string, locked bool) error {
	if protected[name] {
		return ErrProtected
	}
	flag := "-U"
	if locked {
		flag = "-L"
	}
	return m.run(ctx, "usermod", flag, name)
}

// SetSudo thêm hoặc bỏ tài khoản khỏi nhóm quản trị.
func (m *Manager) SetSudo(ctx context.Context, name string, sudo bool) error {
	if protected[name] {
		return ErrProtected
	}

	group := m.sudoGroup(ctx)
	if group == "" {
		return errors.New("sysuser: máy này không có nhóm sudo")
	}
	if sudo {
		return m.run(ctx, "usermod", "-aG", group, name)
	}
	return m.run(ctx, "gpasswd", "-d", name, group)
}

// Delete xóa tài khoản, tùy chọn xóa cả thư mục nhà.
func (m *Manager) Delete(ctx context.Context, name string, removeHome bool) error {
	if protected[name] {
		return ErrProtected
	}

	args := []string{}
	if removeHome {
		args = append(args, "-r")
	}
	return m.run(ctx, "userdel", append(args, name)...)
}

// Keys liệt kê khóa SSH của một tài khoản.
//
// Trả về danh sách rỗng chứ không phải nil khi tài khoản chưa có khóa nào: nil
// đi qua JSON thành null, và phía giao diện thì "chưa có khóa" phải là một danh
// sách rỗng để đếm được, không phải một giá trị không tồn tại.
func (m *Manager) Keys(ctx context.Context, name string) ([]Key, error) {
	user, err := m.Get(ctx, name)
	if err != nil {
		return nil, err
	}

	keys := m.keysOf(ctx, user)
	if keys == nil {
		keys = []Key{}
	}
	return keys, nil
}

// AddKey gắn thêm một khóa công khai cho tài khoản.
func (m *Manager) AddKey(ctx context.Context, name, raw string) (Key, error) {
	user, err := m.Get(ctx, name)
	if err != nil {
		return Key{}, err
	}

	key, err := NormalizeKey(raw)
	if err != nil {
		return Key{}, err
	}

	content, _ := m.read(ctx, authorizedKeysPath(user))
	updated, err := AddKey(content, key)
	if err != nil {
		return Key{}, err
	}
	if err := m.writeKeys(ctx, user, updated); err != nil {
		return Key{}, err
	}
	return key, nil
}

// RemoveKey gỡ một khóa theo dấu vân tay.
func (m *Manager) RemoveKey(ctx context.Context, name, fingerprint string) error {
	user, err := m.Get(ctx, name)
	if err != nil {
		return err
	}

	content, err := m.read(ctx, authorizedKeysPath(user))
	if err != nil {
		return ErrKeyNotFound
	}

	updated, err := RemoveKey(content, fingerprint)
	if err != nil {
		return err
	}
	return m.writeKeys(ctx, user, updated)
}

// keysOf đọc khóa của một tài khoản, trả về rỗng nếu chưa có tệp.
func (m *Manager) keysOf(ctx context.Context, user User) []Key {
	if user.Home == "" {
		return nil
	}
	content, err := m.read(ctx, authorizedKeysPath(user))
	if err != nil {
		return nil
	}
	return ParseKeys(content)
}

// writeKeys ghi tệp authorized_keys với quyền sshd chấp nhận.
//
// sshd bỏ qua tệp mà người khác ghi được — kể cả nhóm — nên quyền và chủ sở hữu
// phải đúng, nếu không khóa vừa thêm sẽ im lặng không có tác dụng.
func (m *Manager) writeKeys(ctx context.Context, user User, content string) error {
	dir := path.Join(user.Home, ".ssh")
	if err := m.host.FS().Mkdir(ctx, dir, 0o700); err != nil {
		return err
	}

	target := authorizedKeysPath(user)
	if err := m.host.FS().Write(ctx, target, strings.NewReader(content), 0o600); err != nil {
		return err
	}
	if err := m.host.FS().Chmod(ctx, dir, 0o700); err != nil {
		return err
	}

	owner := fmt.Sprintf("%d:%d", user.UID, user.GID)
	if err := m.run(ctx, "chown", "-R", owner, dir); err != nil {
		return err
	}
	return nil
}

func authorizedKeysPath(user User) string {
	return path.Join(user.Home, ".ssh", "authorized_keys")
}

// sudoGroup tìm nhóm cấp quyền quản trị mà máy này đang dùng.
//
// Debian và Ubuntu dùng "sudo", họ Red Hat dùng "wheel". Đoán sai thì lệnh
// usermod thất bại với một thông báo khó hiểu, nên tìm trong /etc/group.
func (m *Manager) sudoGroup(ctx context.Context) string {
	content, err := m.read(ctx, "/etc/group")
	if err != nil {
		return ""
	}

	for _, line := range strings.Split(content, "\n") {
		name, _, ok := strings.Cut(line, ":")
		if ok && sudoGroups[name] {
			return name
		}
	}
	return ""
}

func (m *Manager) read(ctx context.Context, name string) (string, error) {
	reader, err := m.host.FS().Open(ctx, name)
	if err != nil {
		return "", err
	}
	defer func() { _ = reader.Close() }()

	data, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// run chạy một lệnh quản lý tài khoản và dịch mã thoát thành lỗi đọc được.
func (m *Manager) run(ctx context.Context, command string, args ...string) error {
	result, err := m.host.Exec(ctx, host.Command{Path: command, Args: args})
	if err != nil {
		if errors.Is(err, host.ErrCommandNotAllowed) || errors.Is(err, os.ErrNotExist) {
			return ErrUnavailable
		}
		return err
	}
	if !result.OK() {
		message := strings.TrimSpace(result.Stderr)
		if message == "" {
			message = strings.TrimSpace(result.Stdout)
		}
		return fmt.Errorf("%s: %s", command, message)
	}
	return nil
}
