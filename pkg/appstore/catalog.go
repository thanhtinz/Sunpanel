package appstore

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// builtinCatalog là danh mục ứng dụng đi kèm binary, gồm cả biểu trưng.
//
// Nhúng sẵn thay vì tải về lúc chạy: panel phải cài được ứng dụng ngay cả trên
// máy chủ không ra được Internet ngoài registry ảnh. Biểu trưng cũng vậy — tải
// logo từ tên miền ngoài thì trên máy chủ kín mạng chợ ứng dụng sẽ trống trơn,
// và chính sách nội dung của panel vốn chặn ảnh từ tên miền khác.
//
//go:embed catalog/*.yaml icons/*.svg
var builtinCatalog embed.FS

// Catalog là tập hợp ứng dụng đã nạp.
type Catalog struct {
	apps  []App
	byKey map[string]App
}

// LoadBuiltin nạp danh mục đi kèm binary.
func LoadBuiltin() (*Catalog, error) {
	return load(builtinCatalog, "catalog", "icons")
}

// LoadDir nạp thêm định nghĩa ứng dụng từ một thư mục trên đĩa.
//
// Thư mục không tồn tại không phải lỗi: đa số máy chủ chỉ dùng danh mục sẵn có.
func LoadDir(dir string) (*Catalog, error) {
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return &Catalog{byKey: map[string]App{}}, nil
		}
		return nil, fmt.Errorf("đọc thư mục danh mục: %w", err)
	}
	return load(os.DirFS(dir), ".", "icons")
}

// load nạp mọi tệp .yaml trong dir, và gắn biểu trưng tìm được ở iconDir.
func load(fsys fs.FS, dir, iconDir string) (*Catalog, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("đọc danh mục ứng dụng: %w", err)
	}

	catalog := &Catalog{byKey: make(map[string]App, len(entries))}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		content, err := fs.ReadFile(fsys, filepath.ToSlash(filepath.Join(dir, entry.Name())))
		if err != nil {
			return nil, fmt.Errorf("đọc %s: %w", entry.Name(), err)
		}

		var app App
		if err := yaml.Unmarshal(content, &app); err != nil {
			return nil, fmt.Errorf("đọc %s: %w", entry.Name(), err)
		}
		// Biểu trưng để rời thành tệp .svg thay vì nhét vào YAML: sửa một hình vẽ
		// bằng công cụ đồ họa dễ hơn nhiều so với sửa một chuỗi trong tệp cấu hình.
		if app.Icon == "" {
			if icon, err := fs.ReadFile(fsys, path.Join(iconDir, app.Key+".svg")); err == nil {
				app.Icon = strings.TrimSpace(string(icon))
			}
		}

		if err := app.Validate(); err != nil {
			return nil, fmt.Errorf("%s: %w", entry.Name(), err)
		}
		if _, duplicate := catalog.byKey[app.Key]; duplicate {
			return nil, fmt.Errorf("%w: định danh %q xuất hiện hai lần", ErrInvalidApp, app.Key)
		}

		catalog.byKey[app.Key] = app
		catalog.apps = append(catalog.apps, app)
	}

	sortApps(catalog.apps)
	return catalog, nil
}

// Merge gộp một danh mục khác vào, ứng dụng trùng định danh sẽ bị ghi đè.
//
// Dùng để danh mục tự thêm của quản trị viên ưu tiên hơn danh mục sẵn có, nhờ
// vậy sửa một ứng dụng không phải chờ bản cập nhật panel.
func (c *Catalog) Merge(other *Catalog) {
	for _, app := range other.apps {
		if _, exists := c.byKey[app.Key]; !exists {
			c.apps = append(c.apps, app)
		} else {
			for i := range c.apps {
				if c.apps[i].Key == app.Key {
					c.apps[i] = app
					break
				}
			}
		}
		c.byKey[app.Key] = app
	}
	sortApps(c.apps)
}

// sortApps xếp ứng dụng theo tên, không phân biệt hoa thường.
//
// So sánh thẳng chuỗi sẽ đẩy mọi tên viết thường ("n8n") xuống cuối danh sách,
// vì chữ thường đứng sau chữ hoa trong bảng mã.
func sortApps(apps []App) {
	sort.Slice(apps, func(i, j int) bool {
		return strings.ToLower(apps[i].Name.EN) < strings.ToLower(apps[j].Name.EN)
	})
}

// Apps trả về toàn bộ ứng dụng trong danh mục.
func (c *Catalog) Apps() []App { return c.apps }

// Get tìm một ứng dụng theo định danh.
func (c *Catalog) Get(key string) (App, error) {
	app, ok := c.byKey[key]
	if !ok {
		return App{}, fmt.Errorf("%w: %s", ErrNotFound, key)
	}
	return app, nil
}

// Categories liệt kê các nhóm ứng dụng có trong danh mục.
func (c *Catalog) Categories() []string {
	seen := make(map[string]struct{})
	var out []string
	for _, app := range c.apps {
		if app.Category == "" {
			continue
		}
		if _, ok := seen[app.Category]; ok {
			continue
		}
		seen[app.Category] = struct{}{}
		out = append(out, app.Category)
	}
	sort.Strings(out)
	return out
}
