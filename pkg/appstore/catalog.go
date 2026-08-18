package appstore

import (
	"embed"
	"encoding/base64"
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
//go:embed catalog/*.yaml icons/*.svg icons/*.webp
var builtinCatalog embed.FS

// Catalog là tập hợp ứng dụng đã nạp.
type Catalog struct {
	apps  []App
	byKey map[string]App
	// problems là các tệp bị bỏ qua vì không đọc được, chỉ dùng cho danh mục tự
	// thêm của quản trị viên.
	problems []string
}

// Problems liệt kê các tệp danh mục bị bỏ qua kèm lý do.
func (c *Catalog) Problems() []string { return c.problems }

// LoadBuiltin nạp danh mục đi kèm binary.
func LoadBuiltin() (*Catalog, error) {
	// Danh mục nhúng sẵn thì ngược lại: một tệp hỏng ở đây là lỗi lúc dựng bản
	// phát hành, phải nổ ngay chứ không được lặng lẽ biến mất khỏi chợ.
	return load(builtinCatalog, "catalog", "icons", true)
}

// LoadDir nạp thêm định nghĩa ứng dụng từ một thư mục trên đĩa.
//
// Thư mục không tồn tại không phải lỗi: đa số máy chủ chỉ dùng danh mục sẵn có.
//
// Một tệp hỏng cũng không phải lỗi: nó bị bỏ qua và ghi vào Problems. Panel
// quản trị cả máy chủ, nên để một tệp YAML gõ sai của quản trị viên chặn cả
// panel khởi động là đổi một ứng dụng lấy toàn bộ khả năng vào sửa nó.
func LoadDir(dir string) (*Catalog, error) {
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return &Catalog{byKey: map[string]App{}}, nil
		}
		return nil, fmt.Errorf("đọc thư mục danh mục: %w", err)
	}
	return load(os.DirFS(dir), ".", "icons", false)
}

// load nạp mọi tệp .yaml trong dir, và gắn biểu trưng tìm được ở iconDir.
//
// strict quyết định tệp hỏng làm dừng cả việc nạp hay chỉ bị bỏ qua.
func load(fsys fs.FS, dir, iconDir string, strict bool) (*Catalog, error) {
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
			if !strict {
				catalog.skip(entry.Name(), err)
				continue
			}
			return nil, fmt.Errorf("đọc %s: %w", entry.Name(), err)
		}

		var app App
		if err := yaml.Unmarshal(content, &app); err != nil {
			if !strict {
				catalog.skip(entry.Name(), err)
				continue
			}
			return nil, fmt.Errorf("đọc %s: %w", entry.Name(), err)
		}
		// Biểu trưng để rời thành tệp ảnh thay vì nhét vào YAML: thay logo bằng
		// cách chép đè một tệp dễ hơn nhiều so với sửa một chuỗi trong cấu hình.
		if app.Icon == "" {
			app.Icon = readIcon(fsys, iconDir, app.Key)
		}
		if app.IconDark == "" {
			app.IconDark = readIcon(fsys, iconDir, app.Key+"-dark")
		}

		if err := app.Validate(); err != nil {
			if !strict {
				catalog.skip(entry.Name(), err)
				continue
			}
			return nil, fmt.Errorf("%s: %w", entry.Name(), err)
		}
		if _, duplicate := catalog.byKey[app.Key]; duplicate {
			if !strict {
				catalog.skip(entry.Name(), fmt.Errorf("định danh %q trùng tệp khác", app.Key))
				continue
			}
			return nil, fmt.Errorf("%w: định danh %q xuất hiện hai lần", ErrInvalidApp, app.Key)
		}

		catalog.byKey[app.Key] = app
		catalog.apps = append(catalog.apps, app)
	}

	sortApps(catalog.apps)
	return catalog, nil
}

// iconFormats là các định dạng biểu trưng được chấp nhận, xếp theo thứ tự ưu
// tiên: ảnh véc-tơ nét gọn hơn hẳn ảnh điểm ở mọi cỡ hiển thị.
var iconFormats = []struct{ ext, mime string }{
	{".svg", "image/svg+xml"},
	{".webp", "image/webp"},
	{".png", "image/png"},
	{".jpg", "image/jpeg"},
	{".jpeg", "image/jpeg"},
}

// readIcon đọc tệp biểu trưng và trả về dưới dạng data URI.
//
// Trả về chuỗi rỗng khi không có tệp nào: thiếu biểu trưng không phải lỗi, giao
// diện tự dựng ô chữ cái đầu thay thế.
func readIcon(fsys fs.FS, dir, name string) string {
	for _, format := range iconFormats {
		content, err := fs.ReadFile(fsys, path.Join(dir, name+format.ext))
		if err != nil {
			continue
		}
		return "data:" + format.mime + ";base64," + base64.StdEncoding.EncodeToString(content)
	}
	return ""
}

// skip ghi lại một tệp bị bỏ qua kèm lý do.
func (c *Catalog) skip(name string, err error) {
	c.problems = append(c.problems, fmt.Sprintf("%s: %v", name, err))
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
