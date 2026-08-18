package appstore

import (
	"encoding/base64"
	"errors"
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// Định nghĩa hỏng phải lộ ra ngay khi nạp danh mục, chứ không phải lúc người
// dùng bấm cài và nhận về một lỗi khó hiểu từ compose.
func TestBuiltinCatalogIsValid(t *testing.T) {
	catalog, err := LoadBuiltin()
	if err != nil {
		t.Fatalf("nạp danh mục sẵn có: %v", err)
	}
	if len(catalog.Apps()) == 0 {
		t.Fatal("danh mục sẵn có không có ứng dụng nào")
	}

	for _, app := range catalog.Apps() {
		if err := app.Validate(); err != nil {
			t.Errorf("ứng dụng %s không hợp lệ: %v", app.Key, err)
		}
		if len(app.Images) == 0 {
			t.Errorf("ứng dụng %s không khai báo image nào", app.Key)
		}
		// Mọi ứng dụng phải đặt tên container qua biến dành riêng, nếu không panel
		// không tìm lại được container của nó để hiện trạng thái.
		if !strings.Contains(app.Compose, "${CONTAINER_NAME}") {
			t.Errorf("ứng dụng %s không dùng ${CONTAINER_NAME}", app.Key)
		}
	}
	t.Logf("đã nạp %d ứng dụng: %v", len(catalog.Apps()), keysOf(catalog))
}

func keysOf(c *Catalog) []string {
	out := make([]string, 0, len(c.Apps()))
	for _, app := range c.Apps() {
		out = append(out, app.Key)
	}
	return out
}

// Giá trị chứa xuống dòng sẽ tạo thêm biến trong tệp .env, và biến đó có thể
// ghi đè cấu hình container — kể cả cấu hình cho phép thoát khỏi container.
func TestResolveRejectsNewlineInValue(t *testing.T) {
	app := App{
		Key:     "thu",
		Name:    Text{VI: "Thử", EN: "Test"},
		Compose: "services: {}\n",
		Fields:  []Field{{Key: "NAME", Type: FieldText}},
	}

	cases := []string{
		"binh-thuong\nPRIVILEGED=true",
		"a\rb",
		"a\x00b",
	}
	for _, value := range cases {
		if _, err := app.Resolve(map[string]string{"NAME": value}); !errors.Is(err, ErrInvalidValue) {
			t.Errorf("giá trị %q phải bị từ chối, nhận: %v", value, err)
		}
	}
}

func TestResolveGeneratesAndValidates(t *testing.T) {
	app := App{
		Key:     "thu",
		Name:    Text{VI: "Thử", EN: "Test"},
		Compose: "services: {}\n",
		Fields: []Field{
			{Key: "SECRET", Type: FieldPassword, Generate: true},
			{Key: "PORT", Type: FieldPort, Default: "8080", Required: true},
			{Key: "MODE", Type: FieldSelect, Default: "a", Options: []Option{{Value: "a"}, {Value: "b"}}},
		},
	}

	values, err := app.Resolve(map[string]string{})
	if err != nil {
		t.Fatalf("giải giá trị: %v", err)
	}
	if len(values["SECRET"]) < 16 {
		t.Errorf("mật khẩu sinh ra quá ngắn: %q", values["SECRET"])
	}
	if values["PORT"] != "8080" {
		t.Errorf("giá trị mặc định chưa được dùng: %q", values["PORT"])
	}

	// Hai lần cài không được ra cùng một mật khẩu.
	second, err := app.Resolve(map[string]string{})
	if err != nil {
		t.Fatalf("giải giá trị lần hai: %v", err)
	}
	if second["SECRET"] == values["SECRET"] {
		t.Error("mật khẩu sinh ra giống hệt nhau giữa hai lần cài")
	}

	if _, err := app.Resolve(map[string]string{"PORT": "99999"}); !errors.Is(err, ErrInvalidValue) {
		t.Errorf("cổng ngoài dải phải bị từ chối, nhận: %v", err)
	}
	if _, err := app.Resolve(map[string]string{"MODE": "c"}); !errors.Is(err, ErrInvalidValue) {
		t.Errorf("lựa chọn ngoài danh sách phải bị từ chối, nhận: %v", err)
	}
}

// Khuôn dùng một biến chưa khai báo thì compose thay bằng chuỗi rỗng, và
// container chạy với cấu hình trống rỗng thay vì báo lỗi.
func TestValidateCatchesUndeclaredVariable(t *testing.T) {
	app := App{
		Key:     "thu",
		Name:    Text{VI: "Thử", EN: "Test"},
		Compose: "services:\n  a:\n    image: x\n    ports: [\"${CHUA_KHAI_BAO}:80\"]\n",
	}
	if err := app.Validate(); !errors.Is(err, ErrInvalidApp) {
		t.Errorf("biến chưa khai báo phải bị bắt, nhận: %v", err)
	}
}

// Biến dành riêng do panel điền; để người dùng khai báo lại nghĩa là họ đặt
// được tên container, và panel mất dấu ứng dụng mình vừa cài.
func TestResolveRejectsReservedVariable(t *testing.T) {
	app := App{
		Key:     "thu",
		Name:    Text{VI: "Thử", EN: "Test"},
		Compose: "services: {}\n",
		Fields:  []Field{{Key: "CONTAINER_NAME", Type: FieldText, Default: "gi-cung-duoc"}},
	}
	if _, err := app.Resolve(map[string]string{}); !errors.Is(err, ErrInvalidApp) {
		t.Errorf("biến dành riêng phải bị từ chối, nhận: %v", err)
	}
}

func TestRenderEnvIsStable(t *testing.T) {
	values := map[string]string{"B": "2", "A": "1", "C": "3"}
	first := RenderEnv(values)
	if first != RenderEnv(values) {
		t.Error("nội dung .env đổi giữa hai lần dựng cùng dữ liệu")
	}
	if !strings.Contains(first, "A=1\nB=2\nC=3\n") {
		t.Errorf("nội dung .env chưa sắp xếp:\n%s", first)
	}
}

// Biểu trưng đi kèm binary vì chính sách nội dung của panel chặn ảnh từ tên
// miền ngoài: thiếu tệp ảnh là ô ứng dụng hiện ra trống trơn.
func TestBuiltinAppsHaveIcons(t *testing.T) {
	catalog, err := LoadBuiltin()
	if err != nil {
		t.Fatalf("nạp danh mục sẵn có: %v", err)
	}

	for _, app := range catalog.Apps() {
		if !strings.HasPrefix(app.Icon, "data:image/") {
			t.Errorf("ứng dụng %s không có biểu trưng", app.Key)
			continue
		}
		if app.IconDark != "" && !strings.HasPrefix(app.IconDark, "data:image/") {
			t.Errorf("ứng dụng %s có biểu trưng chế độ tối sai định dạng", app.Key)
		}

		// Biểu trưng nằm trong tệp .env của binary nên không ai xem lại bằng mắt;
		// một tệp phình to thường là ảnh điểm 512×512 lọt vào chỗ của ảnh véc-tơ.
		if len(app.Icon) > 64*1024 {
			t.Errorf("biểu trưng của %s nặng %d KB, quá lớn cho một ô 44 điểm ảnh",
				app.Key, len(app.Icon)/1024)
		}

		// Giao diện nhúng biểu trưng qua thẻ <img> nên mã kịch bản bên trong không
		// chạy được, nhưng một tệp svg có <script> là dấu hiệu ai đó nhầm chỗ.
		decoded, err := decodeIcon(app.Icon)
		if err != nil {
			t.Errorf("biểu trưng của %s không giải mã được: %v", app.Key, err)
			continue
		}
		if strings.Contains(strings.ToLower(string(decoded)), "<script") {
			t.Errorf("biểu trưng của %s chứa mã kịch bản", app.Key)
		}
	}
}

// decodeIcon lấy lại nội dung tệp từ data URI.
func decodeIcon(icon string) ([]byte, error) {
	_, encoded, found := strings.Cut(icon, ";base64,")
	if !found {
		return nil, errors.New("data URI không mã hóa base64")
	}
	return base64.StdEncoding.DecodeString(encoded)
}

// Danh mục phân loại phải nằm trong tập giao diện có nhãn dịch, nếu không ô lọc
// sẽ hiện đúng cái khóa dịch thô cho người dùng đọc.
func TestBuiltinCategoriesAreKnown(t *testing.T) {
	known := map[string]bool{
		"website": true, "development": true, "monitoring": true, "automation": true,
		"database": true, "media": true, "tool": true, "storage": true,
		"security": true, "productivity": true,
	}

	catalog, err := LoadBuiltin()
	if err != nil {
		t.Fatalf("nạp danh mục sẵn có: %v", err)
	}

	for _, app := range catalog.Apps() {
		if !known[app.Category] {
			t.Errorf("ứng dụng %s có phân loại %q chưa có nhãn dịch", app.Key, app.Category)
		}
	}
}

// Khuôn compose phải là YAML đọc được, và mọi image nó dùng phải được khai báo
// ở trường images — giao diện dựa vào danh sách đó để báo trước sẽ tải gì về.
func TestBuiltinComposeTemplatesParse(t *testing.T) {
	catalog, err := LoadBuiltin()
	if err != nil {
		t.Fatalf("nạp danh mục sẵn có: %v", err)
	}

	for _, app := range catalog.Apps() {
		values, err := app.Resolve(defaultsOf(app))
		if err != nil {
			t.Errorf("%s: điền giá trị mặc định: %v", app.Key, err)
			continue
		}
		values["CONTAINER_NAME"] = app.Key
		values["APP_KEY"] = app.Key
		values["DATA_DIR"] = "/opt/sunpanel/apps/" + app.Key

		rendered := os.Expand(app.Compose, func(name string) string { return values[name] })

		var file struct {
			Services map[string]struct {
				Image string `yaml:"image"`
			} `yaml:"services"`
		}
		if err := yaml.Unmarshal([]byte(rendered), &file); err != nil {
			t.Errorf("%s: khuôn compose không phải YAML hợp lệ: %v", app.Key, err)
			continue
		}
		if len(file.Services) == 0 {
			t.Errorf("%s: khuôn compose không khai báo dịch vụ nào", app.Key)
		}

		declared := make(map[string]bool, len(app.Images))
		for _, image := range app.Images {
			declared[image] = true
		}
		for name, service := range file.Services {
			if service.Image == "" {
				t.Errorf("%s: dịch vụ %s không có image", app.Key, name)
				continue
			}
			if !declared[service.Image] {
				t.Errorf("%s: dịch vụ %s dùng image %q không có trong trường images",
					app.Key, name, service.Image)
			}
		}
	}
}

// defaultsOf lấy giá trị mặc định của mọi biến, mô phỏng người dùng bấm cài mà
// không sửa gì trong biểu mẫu.
func defaultsOf(app App) map[string]string {
	values := make(map[string]string, len(app.Fields))
	for _, field := range app.Fields {
		values[field.Key] = field.Default
	}
	return values
}
