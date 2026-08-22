package main

import (
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// Kind là cách ứng dụng nối tới một máy chủ.
type Kind string

const (
	// KindPanel mở giao diện panel đã cài trên máy chủ.
	KindPanel Kind = "panel"
	// KindSSH nối thẳng vào máy chủ qua SSH, không cần cài gì trên đó.
	KindSSH Kind = "ssh"
)

// Server là một máy chủ đã lưu trong ứng dụng.
type Server struct {
	// ID để giao diện tham chiếu tới đúng máy chủ khi sửa hoặc xóa.
	ID string `json:"id"`
	// Name là tên người dùng tự đặt, ví dụ "VPS Sài Gòn".
	Name string `json:"name"`
	// Kind quyết định phần còn lại của bản ghi được đọc theo cách nào.
	Kind Kind `json:"kind"`

	// URL là địa chỉ đầy đủ kèm đường dẫn bí mật của panel. Chỉ dùng cho KindPanel.
	URL string `json:"url,omitempty"`

	// Các trường dưới đây chỉ dùng cho KindSSH.
	Host string `json:"host,omitempty"`
	Port int    `json:"port,omitempty"`
	User string `json:"user,omitempty"`
	// KeyPath trỏ tới tệp khóa riêng trên máy này; để trống là đăng nhập bằng
	// mật khẩu.
	KeyPath string `json:"keyPath,omitempty"`
	// Password chỉ được ghi khi người dùng tự chọn nhớ. Mặc định là không: một
	// tệp trên đĩa không phải chỗ cho mật khẩu root của máy chủ.
	Password string `json:"password,omitempty"`
	// Fingerprint là vân tay khóa máy chủ đã ghi nhận ở lần kết nối đầu.
	Fingerprint string `json:"fingerprint,omitempty"`

	// Last đánh dấu máy chủ mở gần nhất, để lần sau tự mở lại.
	Last bool `json:"last,omitempty"`
}

// Label là dòng địa chỉ hiện dưới tên máy chủ.
func (s Server) Label() string {
	if s.Kind == KindSSH {
		return s.User + "@" + s.Host + ":" + strconv.Itoa(s.sshPort())
	}
	return s.URL
}

// sshPort trả về cổng SSH, mặc định 22.
func (s Server) sshPort() int {
	if s.Port <= 0 {
		return 22
	}
	return s.Port
}

// Store giữ danh sách máy chủ trên đĩa.
//
// Tệp cấu hình nằm trong thư mục cấu hình của người dùng chứ không cạnh binary:
// ứng dụng có thể được chép vào /usr/local/bin hay Program Files, những nơi mà
// một tiến trình người dùng thường không ghi được.
type Store struct {
	path string
	mu   sync.Mutex
	list []Server
}

// NewStore mở kho máy chủ, tạo thư mục nếu chưa có.
func NewStore() (*Store, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	dir = filepath.Join(dir, "sunpanel")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return openStore(filepath.Join(dir, "servers.json"))
}

// openStore mở kho tại một đường dẫn cụ thể.
func openStore(path string) (*Store, error) {
	store := &Store{path: path}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

// load đọc danh sách đã lưu.
func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		s.list = []Server{}
		return nil
	}
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, &s.list); err != nil {
		// Tệp hỏng không được làm ứng dụng không mở lên được: bắt đầu lại với
		// danh sách rỗng còn hơn bắt người dùng đi sửa JSON bằng tay.
		s.list = []Server{}
	}

	// Bản ghi lưu từ phiên bản trước chỉ có panel nên không ghi trường kind.
	for i := range s.list {
		if s.list[i].Kind == "" {
			s.list[i].Kind = KindPanel
		}
	}
	return nil
}

// save ghi danh sách xuống đĩa.
//
// Quyền 0600 vì địa chỉ panel có kèm đường dẫn bí mật — thứ đứng giữa người lạ
// và trang đăng nhập.
func (s *Store) save() error {
	data, err := json.MarshalIndent(s.list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o600)
}

// List trả về danh sách máy chủ.
func (s *Store) List() []Server {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]Server, len(s.list))
	copy(out, s.list)
	return out
}

// Save thêm mới hoặc cập nhật một máy chủ.
func (s *Store) Save(server Server) (Server, error) {
	clean, err := normalizeServer(server)
	if err != nil {
		return Server{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if clean.ID == "" {
		clean.ID = nextID(s.list)
		s.list = append(s.list, clean)
		return clean, s.save()
	}

	for i := range s.list {
		if s.list[i].ID != clean.ID {
			continue
		}
		// Vân tay khóa máy chủ chỉ còn đúng khi vẫn là máy đó; đổi địa chỉ hay
		// cổng nghĩa là đang trỏ đi nơi khác.
		if s.list[i].Host == clean.Host && s.list[i].sshPort() == clean.sshPort() {
			clean.Fingerprint = s.list[i].Fingerprint
		}
		clean.Last = s.list[i].Last
		s.list[i] = clean
		return clean, s.save()
	}
	return Server{}, errors.New("không tìm thấy máy chủ")
}

// normalizeServer kiểm và dọn một bản ghi trước khi lưu.
func normalizeServer(server Server) (Server, error) {
	server.Name = strings.TrimSpace(server.Name)

	if server.Kind == KindSSH {
		server.Host = strings.TrimSpace(server.Host)
		server.User = strings.TrimSpace(server.User)
		if server.Host == "" {
			return Server{}, errors.New("thiếu địa chỉ máy chủ")
		}
		if strings.ContainsAny(server.Host, " /\\") {
			return Server{}, errors.New("địa chỉ máy chủ không hợp lệ")
		}
		if server.User == "" {
			return Server{}, errors.New("thiếu tên đăng nhập")
		}
		if server.Port < 0 || server.Port > 65535 {
			return Server{}, errors.New("cổng phải nằm trong khoảng 1–65535")
		}
		if server.Port == 0 {
			server.Port = 22
		}
		server.KeyPath = strings.TrimSpace(server.KeyPath)
		server.URL = ""
		if server.Name == "" {
			server.Name = server.User + "@" + server.Host
		}
		return server, nil
	}

	address, err := normalizeURL(server.URL)
	if err != nil {
		return Server{}, err
	}
	server.Kind = KindPanel
	server.URL = address
	server.Host, server.Port, server.User = "", 0, ""
	server.KeyPath, server.Password, server.Fingerprint = "", "", ""
	if server.Name == "" {
		server.Name = address
	}
	return server, nil
}

// RememberFingerprint ghi nhận vân tay khóa máy chủ sau lần kết nối đầu tiên.
func (s *Store) RememberFingerprint(id, fingerprint string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.list {
		if s.list[i].ID == id {
			if s.list[i].Fingerprint == fingerprint {
				return nil
			}
			s.list[i].Fingerprint = fingerprint
			return s.save()
		}
	}
	return errors.New("không tìm thấy máy chủ")
}

// ByID tìm một máy chủ theo định danh.
func (s *Store) ByID(id string) (Server, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, item := range s.list {
		if item.ID == id {
			return item, true
		}
	}
	return Server{}, false
}

// Remove xóa một máy chủ khỏi danh sách.
func (s *Store) Remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := s.list[:0]
	for _, item := range s.list {
		if item.ID != id {
			out = append(out, item)
		}
	}
	s.list = out
	return s.save()
}

// MarkLast ghi nhớ máy chủ vừa mở để lần chạy sau tự vào thẳng.
func (s *Store) MarkLast(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.list {
		s.list[i].Last = s.list[i].ID == id
	}
	return s.save()
}

// Last trả về máy chủ mở gần nhất.
func (s *Store) Last() (Server, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, item := range s.list {
		if item.Last {
			return item, true
		}
	}
	return Server{}, false
}

// normalizeURL kiểm và chuẩn hóa địa chỉ panel.
func normalizeURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("thiếu địa chỉ panel")
	}

	// Thiếu giao thức thì hiểu là http: panel mặc định chạy HTTP cho tới khi
	// người dùng gắn chứng chỉ, và bắt gõ đủ "http://" chỉ tạo thêm chỗ sai.
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", errors.New("địa chỉ không hợp lệ")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("địa chỉ phải bắt đầu bằng http:// hoặc https://")
	}
	if parsed.Host == "" {
		return "", errors.New("địa chỉ thiếu tên máy chủ")
	}

	// Đường dẫn bí mật phải kết thúc bằng dấu gạch chéo, nếu không thẻ base của
	// panel phân giải sai và giao diện nạp tài nguyên từ thư mục cha.
	if parsed.Path != "" && !strings.HasSuffix(parsed.Path, "/") {
		parsed.Path += "/"
	}
	parsed.Fragment = ""
	return parsed.String(), nil
}

// nextID sinh định danh chưa dùng.
func nextID(list []Server) string {
	next := 1
	for {
		candidate := "s" + strconv.Itoa(next)
		used := false
		for _, item := range list {
			if item.ID == candidate {
				used = true
				break
			}
		}
		if !used {
			return candidate
		}
		next++
	}
}
