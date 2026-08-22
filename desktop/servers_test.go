package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeURL(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"thêm http khi thiếu giao thức", "10.0.0.5:9527/qvzQfJuo56JQ", "http://10.0.0.5:9527/qvzQfJuo56JQ/"},
		{"thêm gạch chéo cuối đường dẫn bí mật", "https://panel.example.com/abc123", "https://panel.example.com/abc123/"},
		{"giữ nguyên khi đã đúng", "https://panel.example.com/abc123/", "https://panel.example.com/abc123/"},
		{"cắt khoảng trắng thừa", "  http://127.0.0.1:9527/  ", "http://127.0.0.1:9527/"},
		{"bỏ phần neo", "http://127.0.0.1:9527/abc#day", "http://127.0.0.1:9527/abc/"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeURL(tc.in)
			if err != nil {
				t.Fatalf("normalizeURL(%q) lỗi: %v", tc.in, err)
			}
			if got != tc.want {
				t.Fatalf("normalizeURL(%q) = %q, mong đợi %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestNormalizeURLTuChoi(t *testing.T) {
	for _, raw := range []string{"", "   ", "ftp://panel.example.com", "file:///etc/passwd", "http://"} {
		if got, err := normalizeURL(raw); err == nil {
			t.Fatalf("normalizeURL(%q) = %q, mong đợi lỗi", raw, got)
		}
	}
}

// newTestStore mở một kho trong thư mục tạm của bài kiểm thử.
func newTestStore(t *testing.T) (*Store, string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "servers.json")
	store, err := openStore(path)
	if err != nil {
		t.Fatalf("openStore lỗi: %v", err)
	}
	return store, path
}

func TestStoreLuuSuaXoa(t *testing.T) {
	store, path := newTestStore(t)

	if got := store.List(); len(got) != 0 {
		t.Fatalf("kho mới phải rỗng, có %d mục", len(got))
	}

	first, err := store.Save(Server{Name: "Máy chủ nhà", URL: "127.0.0.1:9527/qvzQfJuo56JQ"})
	if err != nil {
		t.Fatalf("Save lỗi: %v", err)
	}
	if first.URL != "http://127.0.0.1:9527/qvzQfJuo56JQ/" {
		t.Fatalf("Save trả URL %q chưa chuẩn hóa", first.URL)
	}

	second, err := store.Save(Server{Name: "VPS Sài Gòn", URL: "https://sg.example.com/abc/"})
	if err != nil {
		t.Fatalf("Save lỗi: %v", err)
	}
	if second.ID == first.ID {
		t.Fatalf("hai máy chủ trùng ID %q", second.ID)
	}

	if _, err := store.Save(Server{ID: first.ID, Name: "Máy chủ chính", URL: "127.0.0.1:9527/qvzQfJuo56JQ"}); err != nil {
		t.Fatalf("sửa lỗi: %v", err)
	}
	if got := store.List(); len(got) != 2 || got[0].Name != "Máy chủ chính" {
		t.Fatalf("sau khi sửa danh sách sai: %+v", got)
	}

	if _, err := store.Save(Server{ID: "khong-ton-tai", Name: "X", URL: "http://x.example.com/"}); err == nil {
		t.Fatal("sửa máy chủ không tồn tại phải báo lỗi")
	}

	if err := store.Remove(first.ID); err != nil {
		t.Fatalf("Remove lỗi: %v", err)
	}
	if got := store.List(); len(got) != 1 || got[0].ID != second.ID {
		t.Fatalf("sau khi xóa danh sách sai: %+v", got)
	}

	// Tệp phải kín: địa chỉ panel có kèm đường dẫn bí mật.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat lỗi: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("quyền tệp %o, mong đợi 600", perm)
	}
}

func TestStoreGhiNhoMayChuVuaMo(t *testing.T) {
	store, path := newTestStore(t)

	first, _ := store.Save(Server{Name: "A", URL: "http://a.example.com/"})
	second, _ := store.Save(Server{Name: "B", URL: "http://b.example.com/"})

	if _, ok := store.Last(); ok {
		t.Fatal("chưa mở máy chủ nào mà đã có máy chủ gần nhất")
	}

	if err := store.MarkLast(second.ID); err != nil {
		t.Fatalf("MarkLast lỗi: %v", err)
	}
	last, ok := store.Last()
	if !ok || last.ID != second.ID {
		t.Fatalf("Last() = %+v, %v; mong đợi %q", last, ok, second.ID)
	}

	// Chỉ một máy chủ được đánh dấu, nếu không lần mở sau vào nhầm máy.
	if err := store.MarkLast(first.ID); err != nil {
		t.Fatalf("MarkLast lỗi: %v", err)
	}
	marked := 0
	for _, item := range store.List() {
		if item.Last {
			marked++
		}
	}
	if marked != 1 {
		t.Fatalf("có %d máy chủ được đánh dấu, mong đợi 1", marked)
	}

	// Mở lại từ đĩa: đây mới là thứ ứng dụng làm lúc khởi động.
	reopened, err := openStore(path)
	if err != nil {
		t.Fatalf("mở lại kho lỗi: %v", err)
	}
	last, ok = reopened.Last()
	if !ok || last.ID != first.ID || last.Name != "A" {
		t.Fatalf("sau khi mở lại Last() = %+v, %v", last, ok)
	}
}

func TestStoreTepHongVanMoDuoc(t *testing.T) {
	path := filepath.Join(t.TempDir(), "servers.json")
	if err := os.WriteFile(path, []byte("{ đây không phải JSON"), 0o600); err != nil {
		t.Fatalf("ghi tệp lỗi: %v", err)
	}

	store, err := openStore(path)
	if err != nil {
		t.Fatalf("tệp hỏng vẫn phải mở được: %v", err)
	}
	if got := store.List(); len(got) != 0 {
		t.Fatalf("mong đợi danh sách rỗng, có %+v", got)
	}
	if _, err := store.Save(Server{Name: "A", URL: "http://a.example.com/"}); err != nil {
		t.Fatalf("Save sau tệp hỏng lỗi: %v", err)
	}
}

func TestStoreTenTrongDungDiaChi(t *testing.T) {
	store, _ := newTestStore(t)

	saved, err := store.Save(Server{Name: "   ", URL: "http://a.example.com/abc"})
	if err != nil {
		t.Fatalf("Save lỗi: %v", err)
	}
	if saved.Name != "http://a.example.com/abc/" {
		t.Fatalf("tên trống phải lấy địa chỉ, có %q", saved.Name)
	}
}
