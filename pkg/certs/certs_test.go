package certs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGenerateSelfSigned(t *testing.T) {
	domains := []string{"example.com", "www.example.com"}

	certPEM, keyPEM, err := GenerateSelfSigned(domains)
	if err != nil {
		t.Fatalf("sinh chứng chỉ: %v", err)
	}

	if !strings.Contains(string(certPEM), "BEGIN CERTIFICATE") {
		t.Error("chứng chỉ không ở dạng PEM")
	}
	if !strings.Contains(string(keyPEM), "PRIVATE KEY") {
		t.Error("khóa không ở dạng PEM")
	}

	info, err := Parse(certPEM)
	if err != nil {
		t.Fatalf("đọc chứng chỉ: %v", err)
	}
	if len(info.Domains) != 2 || info.Domains[0] != "example.com" {
		t.Errorf("tên miền = %v, mong đợi %v", info.Domains, domains)
	}
	if !info.SelfSigned {
		t.Error("chứng chỉ tự ký phải được nhận ra")
	}
	if info.DaysRemaining() < 360 {
		t.Errorf("số ngày còn lại = %d, mong đợi khoảng 365", info.DaysRemaining())
	}
	if info.NeedsRenewal() {
		t.Error("chứng chỉ mới sinh chưa cần gia hạn")
	}

	// Lùi thời điểm bắt đầu tránh việc chênh lệch đồng hồ khiến chứng chỉ bị coi
	// là chưa có hiệu lực.
	if !info.NotBefore.Before(time.Now()) {
		t.Error("thời điểm bắt đầu phải nằm trong quá khứ")
	}
}

func TestGenerateSelfSignedWithIP(t *testing.T) {
	certPEM, _, err := GenerateSelfSigned([]string{"192.168.1.10"})
	if err != nil {
		t.Fatalf("sinh chứng chỉ: %v", err)
	}
	// Địa chỉ IP phải vào trường IPAddresses, không phải DNSNames.
	info, err := Parse(certPEM)
	if err != nil {
		t.Fatalf("đọc chứng chỉ: %v", err)
	}
	if len(info.Domains) != 0 {
		t.Errorf("địa chỉ IP không được ghi vào DNSNames: %v", info.Domains)
	}
	if len(info.IPs) != 1 || info.IPs[0] != "192.168.1.10" {
		t.Errorf("địa chỉ IP phải được báo cáo ở trường IPs, nhận: %v", info.IPs)
	}
}

func TestGenerateSelfSignedRejectsEmpty(t *testing.T) {
	if _, _, err := GenerateSelfSigned(nil); !errors.Is(err, ErrInvalidCertificate) {
		t.Fatalf("lỗi = %v, mong đợi ErrInvalidCertificate", err)
	}
}

func TestStoreRoundTrip(t *testing.T) {
	store := NewStore(t.TempDir())

	certPEM, keyPEM, err := GenerateSelfSigned([]string{"store.example.com"})
	if err != nil {
		t.Fatalf("sinh chứng chỉ: %v", err)
	}
	if err := store.Save("store-test", certPEM, keyPEM); err != nil {
		t.Fatalf("lưu chứng chỉ: %v", err)
	}

	info, err := store.Load("store-test")
	if err != nil {
		t.Fatalf("đọc chứng chỉ: %v", err)
	}
	if info.Name != "store-test" || len(info.Domains) != 1 {
		t.Errorf("thông tin chứng chỉ = %+v", info)
	}

	// Khóa riêng bị lộ là mất trắng lớp bảo vệ TLS, nên quyền tệp phải chặt.
	keyInfo, err := os.Stat(info.KeyFile)
	if err != nil {
		t.Fatalf("đọc thông tin khóa: %v", err)
	}
	if mode := keyInfo.Mode().Perm(); mode != 0o600 {
		t.Errorf("quyền khóa riêng = %04o, mong đợi 0600", mode)
	}

	list, err := store.List()
	if err != nil {
		t.Fatalf("liệt kê chứng chỉ: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("số chứng chỉ = %d, mong đợi 1", len(list))
	}

	if err := store.Delete("store-test"); err != nil {
		t.Fatalf("xóa chứng chỉ: %v", err)
	}
	if _, err := store.Load("store-test"); !errors.Is(err, ErrNotFound) {
		t.Errorf("lỗi = %v, mong đợi ErrNotFound", err)
	}
}

// Tên chứng chỉ được dùng làm tên thư mục, nên phải chặn đường dẫn leo thư mục.
func TestStoreRejectsUnsafeNames(t *testing.T) {
	store := NewStore(t.TempDir())

	for _, name := range []string{"", "../etc", "a/b", "..", "a\x00b", strings.Repeat("a", 200)} {
		t.Run(name, func(t *testing.T) {
			if _, err := store.Dir(name); !errors.Is(err, ErrInvalidName) {
				t.Fatalf("tên %q phải bị từ chối, lỗi = %v", name, err)
			}
		})
	}
}

func TestStoreListIgnoresBrokenEntries(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)

	// Thư mục không có chứng chỉ hợp lệ không được làm hỏng cả danh sách.
	if err := os.MkdirAll(filepath.Join(root, "hong"), 0o700); err != nil {
		t.Fatalf("tạo thư mục hỏng: %v", err)
	}

	certPEM, keyPEM, _ := GenerateSelfSigned([]string{"ok.example.com"})
	if err := store.Save("tot", certPEM, keyPEM); err != nil {
		t.Fatalf("lưu chứng chỉ: %v", err)
	}

	list, err := store.List()
	if err != nil {
		t.Fatalf("liệt kê: %v", err)
	}
	if len(list) != 1 || list[0].Name != "tot" {
		t.Errorf("danh sách = %+v, mong đợi chỉ chứng chỉ hợp lệ", list)
	}
}

func TestParseRejectsGarbage(t *testing.T) {
	for _, input := range []string{"", "không phải PEM", "-----BEGIN CERTIFICATE-----\nrác\n-----END CERTIFICATE-----"} {
		if _, err := Parse([]byte(input)); !errors.Is(err, ErrInvalidCertificate) {
			t.Errorf("dữ liệu %q phải bị từ chối, lỗi = %v", input, err)
		}
	}
}

// Let's Encrypt chỉ cấp chứng chỉ cho tên miền công khai; gửi tên không hợp lệ
// chỉ tốn một lần thử trong hạn mức vốn rất chặt.
func TestValidateACMEDomain(t *testing.T) {
	valid := []string{"example.com", "www.example.com", "a.b.example.com"}
	for _, domain := range valid {
		if err := validateACMEDomain(domain); err != nil {
			t.Errorf("tên miền %q phải hợp lệ: %v", domain, err)
		}
	}

	invalid := []string{"", "localhost", "*.example.com", "192.168.1.1", "example", "a b.com"}
	for _, domain := range invalid {
		if err := validateACMEDomain(domain); err == nil {
			t.Errorf("tên miền %q phải bị từ chối", domain)
		}
	}
}

// Khóa tài khoản ACME phải giữ nguyên giữa các lần chạy: mất nó là mất luôn
// lịch sử tài khoản và hạn mức đã tích lũy.
func TestACMEAccountKeyIsReused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "acme", "account.key")
	client := NewACMEClient("admin@example.com", path, true)

	first, err := client.loadOrCreateAccountKey()
	if err != nil {
		t.Fatalf("tạo khóa tài khoản: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("khóa tài khoản chưa được ghi: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("quyền khóa tài khoản = %04o, mong đợi 0600", mode)
	}

	second, err := client.loadOrCreateAccountKey()
	if err != nil {
		t.Fatalf("đọc lại khóa tài khoản: %v", err)
	}
	if !first.Equal(second) {
		t.Error("lần gọi thứ hai phải dùng lại đúng khóa cũ, không sinh khóa mới")
	}
}

func TestNeedsRenewal(t *testing.T) {
	cases := []struct {
		daysLeft int
		want     bool
	}{{90, false}, {31, false}, {29, true}, {0, true}, {-5, true}}

	for _, tc := range cases {
		info := Info{NotAfter: time.Now().Add(time.Duration(tc.daysLeft)*24*time.Hour + time.Hour)}
		if got := info.NeedsRenewal(); got != tc.want {
			t.Errorf("còn %d ngày: NeedsRenewal = %v, mong đợi %v", tc.daysLeft, got, tc.want)
		}
	}
}

func TestWebrootSolverPresentAndCleanUp(t *testing.T) {
	root := t.TempDir()
	solver := NewWebrootSolver(root)
	ctx := context.Background()

	const token = "abc_DEF-123"
	if err := solver.Present(ctx, token, "noi-dung-tra-loi"); err != nil {
		t.Fatalf("đặt tệp xác thực: %v", err)
	}

	file := filepath.Join(root, ".well-known", "acme-challenge", token)
	content, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("đọc tệp xác thực: %v", err)
	}
	if string(content) != "noi-dung-tra-loi" {
		t.Errorf("nội dung tệp xác thực sai: %q", content)
	}

	if err := solver.CleanUp(ctx, token); err != nil {
		t.Fatalf("dọn tệp xác thực: %v", err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Errorf("tệp xác thực vẫn còn sau khi dọn")
	}

	// Dọn hai lần không được báo lỗi: lần gia hạn thất bại giữa chừng có thể đã
	// xóa tệp rồi, và lỗi giả ở bước dọn sẽ che mất lỗi thật.
	if err := solver.CleanUp(ctx, token); err != nil {
		t.Errorf("dọn lần hai phải thành công: %v", err)
	}
}

func TestWebrootSolverRejectsPathTraversal(t *testing.T) {
	root := t.TempDir()
	solver := NewWebrootSolver(root)

	for _, token := range []string{"../../etc/cron.d/x", "..", "a/b", "", "a b"} {
		if err := solver.Present(context.Background(), token, "x"); err == nil {
			t.Errorf("mã thử thách %q phải bị từ chối", token)
		}
	}
}
