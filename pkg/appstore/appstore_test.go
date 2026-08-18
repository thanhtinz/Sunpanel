package appstore

import (
	"errors"
	"strings"
	"testing"
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
