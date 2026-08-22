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

// Server là một panel đã lưu trong ứng dụng.
type Server struct {
	// ID để giao diện tham chiếu tới đúng máy chủ khi sửa hoặc xóa.
	ID string `json:"id"`
	// Name là tên người dùng tự đặt, ví dụ "VPS Sài Gòn".
	Name string `json:"name"`
	// URL là địa chỉ đầy đủ kèm đường dẫn bí mật của panel.
	URL string `json:"url"`
	// Last đánh dấu máy chủ mở gần nhất, để lần sau tự mở lại.
	Last bool `json:"last,omitempty"`
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
	address, err := normalizeURL(server.URL)
	if err != nil {
		return Server{}, err
	}

	name := strings.TrimSpace(server.Name)
	if name == "" {
		name = address
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if server.ID == "" {
		server.ID = nextID(s.list)
		s.list = append(s.list, Server{ID: server.ID, Name: name, URL: address})
	} else {
		found := false
		for i := range s.list {
			if s.list[i].ID == server.ID {
				s.list[i].Name, s.list[i].URL = name, address
				found = true
				break
			}
		}
		if !found {
			return Server{}, errors.New("không tìm thấy máy chủ")
		}
	}

	return Server{ID: server.ID, Name: name, URL: address}, s.save()
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
