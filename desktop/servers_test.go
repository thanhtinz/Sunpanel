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

func TestLuuMayChuSSH(t *testing.T) {
	store, path := newTestStore(t)

	saved, err := store.Save(Server{Kind: KindSSH, Name: "VPS Sài Gòn", Host: "203.0.113.10", User: "root"})
	if err != nil {
		t.Fatalf("Save lỗi: %v", err)
	}
	if saved.Port != 22 {
		t.Fatalf("cổng mặc định phải là 22, có %d", saved.Port)
	}
	if got := saved.Label(); got != "root@203.0.113.10:22" {
		t.Fatalf("Label() = %q", got)
	}

	// Tên trống thì lấy user@host, vì một dòng trống trong danh sách thì vô dụng.
	trong, err := store.Save(Server{Kind: KindSSH, Host: "10.0.0.9", User: "quantri", Port: 2222})
	if err != nil {
		t.Fatalf("Save lỗi: %v", err)
	}
	if trong.Name != "quantri@10.0.0.9" {
		t.Fatalf("tên tự đặt sai: %q", trong.Name)
	}

	// Ghi nhớ khóa máy chủ rồi mở lại từ đĩa: đây là thứ chặn người đứng giữa ở
	// lần kết nối sau, nên nó phải sống qua một lần tắt ứng dụng.
	if err := store.RememberFingerprint(saved.ID, "SHA256:abc"); err != nil {
		t.Fatalf("RememberFingerprint lỗi: %v", err)
	}
	lai, err := openStore(path)
	if err != nil {
		t.Fatalf("mở lại kho lỗi: %v", err)
	}
	got, ok := lai.ByID(saved.ID)
	if !ok || got.Fingerprint != "SHA256:abc" || got.Kind != KindSSH {
		t.Fatalf("sau khi mở lại: %+v (%v)", got, ok)
	}
}

func TestDoiMayChuThiBoKhoaCu(t *testing.T) {
	store, _ := newTestStore(t)

	saved, _ := store.Save(Server{Kind: KindSSH, Host: "203.0.113.10", User: "root"})
	if err := store.RememberFingerprint(saved.ID, "SHA256:abc"); err != nil {
		t.Fatalf("RememberFingerprint lỗi: %v", err)
	}

	// Đổi mỗi tên thì vẫn là máy đó, khóa giữ nguyên.
	doiTen, err := store.Save(Server{ID: saved.ID, Kind: KindSSH, Name: "Máy chính", Host: "203.0.113.10", User: "root"})
	if err != nil {
		t.Fatalf("Save lỗi: %v", err)
	}
	if doiTen.Fingerprint != "SHA256:abc" {
		t.Fatalf("đổi tên không được làm mất khóa đã ghi nhận")
	}

	// Đổi sang máy khác thì khóa cũ không còn nói lên điều gì.
	doiMay, err := store.Save(Server{ID: saved.ID, Kind: KindSSH, Host: "198.51.100.7", User: "root"})
	if err != nil {
		t.Fatalf("Save lỗi: %v", err)
	}
	if doiMay.Fingerprint != "" {
		t.Fatalf("đổi địa chỉ phải bỏ khóa cũ, còn %q", doiMay.Fingerprint)
	}
}

func TestTuChoiMayChuSSHThieuThongTin(t *testing.T) {
	store, _ := newTestStore(t)

	cases := []struct {
		name   string
		server Server
	}{
		{"thiếu địa chỉ", Server{Kind: KindSSH, User: "root"}},
		{"thiếu tài khoản", Server{Kind: KindSSH, Host: "10.0.0.1"}},
		{"cổng ngoài khoảng", Server{Kind: KindSSH, Host: "10.0.0.1", User: "root", Port: 70000}},
		{"địa chỉ có khoảng trắng", Server{Kind: KindSSH, Host: "10.0.0.1 rm -rf", User: "root"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := store.Save(tc.server); err == nil {
				t.Fatal("mong đợi lỗi")
			}
		})
	}
}

func TestBanGhiCuKhongCoKieuHieuLaPanel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "servers.json")
	// Đây đúng là thứ phiên bản trước ghi ra đĩa.
	old := `[{"id":"s1","name":"Máy chủ nhà","url":"http://127.0.0.1:9527/abc/","last":true}]`
	if err := os.WriteFile(path, []byte(old), 0o600); err != nil {
		t.Fatalf("ghi tệp lỗi: %v", err)
	}

	store, err := openStore(path)
	if err != nil {
		t.Fatalf("openStore lỗi: %v", err)
	}
	got, ok := store.ByID("s1")
	if !ok || got.Kind != KindPanel {
		t.Fatalf("bản ghi cũ phải được hiểu là panel: %+v", got)
	}
	if last, ok := store.Last(); !ok || last.ID != "s1" {
		t.Fatalf("mất đánh dấu máy chủ mở gần nhất")
	}
}

func TestStoreLuuSuaXoa(t *testing.T) {
	store, path := newTestStore(t)

	if got := store.List(); len(got) != 0 {
		t.Fatalf("kho mới phải rỗng, có %d mục", len(got))
	}

	first, err := store.Save(Server{Kind: KindPanel, Name: "Máy chủ nhà", URL: "127.0.0.1:9527/qvzQfJuo56JQ"})
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
