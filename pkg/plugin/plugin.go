// Package plugin nạp và điều phối các plugin dạng dịch vụ phụ.
//
// Plugin là một dịch vụ HTTP chạy riêng, khai báo bằng một tệp YAML trong thư
// mục plugin của panel. Panel không nạp mã lạ vào chính tiến trình của mình:
// panel chạy quyền root, nên một plugin lỗi hoặc độc hại sẽ kéo theo toàn bộ
// máy chủ. Chạy riêng tiến trình thì plugin hỏng chỉ làm hỏng chính nó.
package plugin

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Các lỗi dùng chung.
var (
	// ErrNotFound là không tìm thấy plugin.
	ErrNotFound = errors.New("plugin: không tìm thấy plugin")
	// ErrInvalidManifest là tệp khai báo plugin không hợp lệ.
	ErrInvalidManifest = errors.New("plugin: tệp khai báo không hợp lệ")
)

// Text là một chuỗi có bản dịch.
type Text struct {
	VI string `yaml:"vi" json:"vi"`
	EN string `yaml:"en" json:"en"`
}

// Manifest là tệp khai báo của một plugin.
type Manifest struct {
	// Key là định danh, dùng trong đường dẫn API và trong menu.
	Key         string `yaml:"key" json:"key"`
	Name        Text   `yaml:"name" json:"name"`
	Description Text   `yaml:"description" json:"description"`
	Version     string `yaml:"version" json:"version"`
	Author      string `yaml:"author" json:"author"`
	Website     string `yaml:"website" json:"website"`

	// BaseURL là địa chỉ dịch vụ plugin, ví dụ http://127.0.0.1:9600
	//
	// Panel chuyển tiếp yêu cầu tới đây. Tệp khai báo chỉ ghi được bởi người có
	// quyền ghi lên đĩa máy chủ, tức là người vốn đã toàn quyền, nên địa chỉ này
	// được tin ở mức đó — nhưng vẫn phải là địa chỉ hợp lệ.
	BaseURL string `yaml:"baseUrl" json:"baseUrl"`

	// UIPath là đường dẫn giao diện của plugin, hiển thị trong khung nhúng.
	// Để trống nghĩa là plugin chỉ có API, không có giao diện riêng.
	UIPath string `yaml:"uiPath" json:"uiPath"`

	// RequireRole là vai trò tối thiểu để dùng plugin: admin, operator hoặc readonly.
	RequireRole string `yaml:"requireRole" json:"requireRole"`

	// Enabled cho phép tắt plugin mà không phải xóa tệp khai báo.
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Path là đường dẫn tệp khai báo, panel tự điền khi nạp.
	Path string `yaml:"-" json:"-"`
}

// validKey khớp định danh plugin, vốn xuất hiện trong đường dẫn URL.
var validKey = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)

// Validate kiểm tra một tệp khai báo.
func (m Manifest) Validate() error {
	if !validKey.MatchString(m.Key) {
		return fmt.Errorf("%w: định danh %q", ErrInvalidManifest, m.Key)
	}
	if m.Name.VI == "" || m.Name.EN == "" {
		return fmt.Errorf("%w: %s thiếu tên ở một ngôn ngữ", ErrInvalidManifest, m.Key)
	}

	parsed, err := url.Parse(m.BaseURL)
	if err != nil || parsed.Host == "" {
		return fmt.Errorf("%w: %s có địa chỉ %q không hợp lệ", ErrInvalidManifest, m.Key, m.BaseURL)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%w: %s phải dùng http hoặc https", ErrInvalidManifest, m.Key)
	}

	switch m.RequireRole {
	case "", "admin", "operator", "readonly":
	default:
		return fmt.Errorf("%w: %s yêu cầu vai trò %q không tồn tại",
			ErrInvalidManifest, m.Key, m.RequireRole)
	}
	return nil
}

// Role trả về vai trò tối thiểu, mặc định là quyền vận hành.
//
// Mặc định KHÔNG phải readonly: plugin chạy mã do bên thứ ba viết trên máy chủ
// của người dùng, nên mở cho tài khoản chỉ-xem là mặc định sai hướng.
func (m Manifest) Role() string {
	if m.RequireRole == "" {
		return "operator"
	}
	return m.RequireRole
}

// Registry là tập hợp plugin đã nạp.
type Registry struct {
	plugins []Manifest
	byKey   map[string]Manifest
	dir     string
}

// Load nạp mọi plugin từ một thư mục.
//
// Thư mục không tồn tại không phải lỗi: phần lớn cài đặt không dùng plugin nào.
func Load(dir string) (*Registry, error) {
	registry := &Registry{byKey: map[string]Manifest{}, dir: dir}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return registry, nil
		}
		return nil, fmt.Errorf("đọc thư mục plugin: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("đọc %s: %w", entry.Name(), err)
		}

		var manifest Manifest
		if err := yaml.Unmarshal(content, &manifest); err != nil {
			return nil, fmt.Errorf("đọc %s: %w", entry.Name(), err)
		}
		if err := manifest.Validate(); err != nil {
			return nil, fmt.Errorf("%s: %w", entry.Name(), err)
		}
		if _, duplicate := registry.byKey[manifest.Key]; duplicate {
			return nil, fmt.Errorf("%w: định danh %q xuất hiện hai lần",
				ErrInvalidManifest, manifest.Key)
		}

		manifest.Path = path
		manifest.BaseURL = strings.TrimRight(manifest.BaseURL, "/")
		registry.byKey[manifest.Key] = manifest
		registry.plugins = append(registry.plugins, manifest)
	}

	sort.Slice(registry.plugins, func(i, j int) bool {
		return strings.ToLower(registry.plugins[i].Name.EN) < strings.ToLower(registry.plugins[j].Name.EN)
	})
	return registry, nil
}

// All trả về mọi plugin đã nạp.
func (r *Registry) All() []Manifest { return r.plugins }

// Enabled trả về các plugin đang bật.
func (r *Registry) Enabled() []Manifest {
	out := make([]Manifest, 0, len(r.plugins))
	for _, manifest := range r.plugins {
		if manifest.Enabled {
			out = append(out, manifest)
		}
	}
	return out
}

// Get tìm một plugin đang bật theo định danh.
func (r *Registry) Get(key string) (Manifest, error) {
	manifest, ok := r.byKey[key]
	if !ok || !manifest.Enabled {
		return Manifest{}, fmt.Errorf("%w: %s", ErrNotFound, key)
	}
	return manifest, nil
}

// Dir trả về thư mục chứa tệp khai báo plugin.
func (r *Registry) Dir() string { return r.dir }
