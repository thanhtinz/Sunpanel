package plugin

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeManifest(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("ghi tệp khai báo: %v", err)
	}
}

func TestLoadMissingDirectoryIsNotAnError(t *testing.T) {
	// Phần lớn cài đặt không dùng plugin nào; thư mục chưa tồn tại là bình thường.
	registry, err := Load(filepath.Join(t.TempDir(), "chua-co"))
	if err != nil {
		t.Fatalf("thư mục chưa tồn tại không được coi là lỗi: %v", err)
	}
	if len(registry.All()) != 0 {
		t.Errorf("phải trả về danh sách rỗng: %+v", registry.All())
	}
}

func TestLoadAndFilter(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "giam-sat.yaml", `
key: giam-sat
name: {vi: Giám sát mở rộng, en: Extended monitoring}
description: {vi: Thêm biểu đồ, en: Extra charts}
version: "1.0"
baseUrl: http://127.0.0.1:9600
uiPath: /ui
requireRole: operator
enabled: true
`)
	writeManifest(t, dir, "da-tat.yaml", `
key: da-tat
name: {vi: Đã tắt, en: Disabled}
baseUrl: http://127.0.0.1:9601
enabled: false
`)

	registry, err := Load(dir)
	if err != nil {
		t.Fatalf("nạp plugin: %v", err)
	}
	if len(registry.All()) != 2 {
		t.Fatalf("phải nạp cả plugin đang tắt: %d", len(registry.All()))
	}
	if len(registry.Enabled()) != 1 {
		t.Errorf("chỉ plugin đang bật mới được dùng: %d", len(registry.Enabled()))
	}

	if _, err := registry.Get("da-tat"); !errors.Is(err, ErrNotFound) {
		t.Errorf("plugin đang tắt phải coi như không có: %v", err)
	}

	manifest, err := registry.Get("giam-sat")
	if err != nil {
		t.Fatalf("tìm plugin: %v", err)
	}
	if manifest.Name.VI != "Giám sát mở rộng" {
		t.Errorf("tên tiếng Việt sai: %q", manifest.Name.VI)
	}
}

// Tệp khai báo hỏng phải lộ ra lúc khởi động, chứ không phải lúc người dùng bấm
// vào menu và nhận một trang trắng.
func TestValidateRejectsBadManifest(t *testing.T) {
	cases := []Manifest{
		{Key: "Có Dấu", Name: Text{VI: "a", EN: "b"}, BaseURL: "http://x"},
		{Key: "thieu-ten", BaseURL: "http://x"},
		{Key: "sai-dia-chi", Name: Text{VI: "a", EN: "b"}, BaseURL: "khong-phai-url"},
		{Key: "sai-giao-thuc", Name: Text{VI: "a", EN: "b"}, BaseURL: "ftp://x"},
		{Key: "sai-vai-tro", Name: Text{VI: "a", EN: "b"}, BaseURL: "http://x", RequireRole: "vua"},
	}
	for _, manifest := range cases {
		if err := manifest.Validate(); !errors.Is(err, ErrInvalidManifest) {
			t.Errorf("khai báo %q phải bị từ chối: %v", manifest.Key, err)
		}
	}
}

// Plugin chạy mã do bên thứ ba viết trên máy chủ của người dùng, nên mở cho tài
// khoản chỉ-xem là mặc định sai hướng.
func TestDefaultRoleIsOperator(t *testing.T) {
	manifest := Manifest{Key: "a", Name: Text{VI: "a", EN: "a"}, BaseURL: "http://x"}
	if manifest.Role() != "operator" {
		t.Errorf("vai trò mặc định phải là operator, nhận %q", manifest.Role())
	}
}

func TestDuplicateKeyIsRejected(t *testing.T) {
	dir := t.TempDir()
	body := "key: trung\nname: {vi: a, en: b}\nbaseUrl: http://127.0.0.1:1\nenabled: true\n"
	writeManifest(t, dir, "mot.yaml", body)
	writeManifest(t, dir, "hai.yaml", body)

	if _, err := Load(dir); err == nil {
		t.Error("hai plugin trùng định danh phải bị từ chối")
	}
}
