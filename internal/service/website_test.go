package service

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/pkg/certs"
	"github.com/thanhtinz/sunpanel/pkg/host"
	"github.com/thanhtinz/sunpanel/pkg/webserver"
)

// fakeWebServer ghi lại các lần áp cấu hình mà không cần nginx thật.
type fakeWebServer struct {
	applied   map[string]webserver.Site
	removed   []string
	available bool
	// applyErr giả lập việc máy chủ web từ chối cấu hình.
	applyErr error
	reloads  int
}

func newFakeWebServer() *fakeWebServer {
	return &fakeWebServer{applied: map[string]webserver.Site{}, available: true}
}

func (f *fakeWebServer) Name() string                   { return "nginx" }
func (f *fakeWebServer) Available(context.Context) bool { return f.available }
func (f *fakeWebServer) Test(context.Context) error     { return nil }
func (f *fakeWebServer) Reload(context.Context) error   { f.reloads++; return nil }
func (f *fakeWebServer) Render(s webserver.Site) (string, error) {
	return "server { server_name " + strings.Join(s.Domains, " ") + "; }", nil
}

func (f *fakeWebServer) Apply(_ context.Context, site webserver.Site) error {
	if f.applyErr != nil {
		return f.applyErr
	}
	f.applied[site.Name] = site
	return nil
}

func (f *fakeWebServer) Remove(_ context.Context, name string) error {
	delete(f.applied, name)
	f.removed = append(f.removed, name)
	return nil
}

// newWebsiteFixture dựng dịch vụ website cùng một chứng chỉ tự ký sẵn có.
func newWebsiteFixture(t *testing.T) (*WebsiteService, *CertificateService, *fakeWebServer) {
	websites, certificates, server, _ := newWebsiteFixtureAt(t)
	return websites, certificates, server
}

// newWebsiteFixtureAt trả về thêm thư mục gốc, cho bài kiểm thử nào cần đặt
// thư mục website bên trong phạm vi của host.
func newWebsiteFixtureAt(t *testing.T) (*WebsiteService, *CertificateService, *fakeWebServer, string) {
	t.Helper()

	db := newMemoryDB(t)
	root := t.TempDir()
	store := certs.NewStore(filepath.Join(root, "certs"))
	solver := certs.NewWebrootSolver(filepath.Join(root, "acme"))
	audit := NewAuditService(db)

	certificates := NewCertificateService(db, store, solver, filepath.Join(root, "acme.key"), audit)
	server := newFakeWebServer()
	websites := NewWebsiteService(
		db, server, certificates,
		host.NewLocalHost(root, nil), filepath.Join(root, "acme"), audit,
	)
	return websites, certificates, server, root
}

func TestCreateWebsiteWritesConfig(t *testing.T) {
	websites, _, server := newWebsiteFixture(t)
	ctx := context.Background()

	site, err := websites.Create(ctx, WebsiteRequest{
		Name:    "blog",
		Domains: []string{"Blog.Example.COM", " www.blog.example.com "},
		Type:    "static",
		Root:    filepath.Join(t.TempDir(), "blog"),
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("tạo website: %v", err)
	}

	// Tên miền phải được chuẩn hóa về chữ thường và bỏ khoảng trắng, nếu không
	// việc kiểm trùng tên miền sẽ bỏ lọt.
	if site.Domains != "blog.example.com,www.blog.example.com" {
		t.Errorf("tên miền chưa được chuẩn hóa: %q", site.Domains)
	}
	if _, ok := server.applied["blog"]; !ok {
		t.Error("cấu hình chưa được ghi cho website vừa tạo")
	}
}

func TestCreateWebsiteRejectsDuplicateDomain(t *testing.T) {
	websites, _, _ := newWebsiteFixture(t)
	ctx := context.Background()

	base := WebsiteRequest{
		Name: "mot", Domains: []string{"example.com"},
		Type: "static", Root: t.TempDir(), Enabled: true,
	}
	if _, err := websites.Create(ctx, base); err != nil {
		t.Fatalf("tạo website đầu: %v", err)
	}

	// Hai khối server cùng server_name khiến nginx lặng lẽ chọn khối đầu, nên
	// website thứ hai phải bị từ chối ngay chứ không "tạo thành công".
	base.Name = "hai"
	_, err := websites.Create(ctx, base)
	if !errors.Is(err, apperr.WebsiteDomainTaken) {
		t.Errorf("tên miền trùng phải bị từ chối, nhận: %v", err)
	}
}

func TestCreateWebsiteRollsBackWhenServerRejects(t *testing.T) {
	websites, _, server := newWebsiteFixture(t)
	ctx := context.Background()

	server.applyErr = errors.New("nginx: cấu hình sai")
	_, err := websites.Create(ctx, WebsiteRequest{
		Name: "hong", Domains: []string{"broken.example.com"},
		Type: "static", Root: t.TempDir(), Enabled: true,
	})
	if err == nil {
		t.Fatal("máy chủ web từ chối thì việc tạo website phải thất bại")
	}

	// Bản ghi còn lại sẽ khiến danh sách hiển thị một website mà nginx không biết.
	sites, err := websites.List(ctx)
	if err != nil {
		t.Fatalf("liệt kê website: %v", err)
	}
	if len(sites) != 0 {
		t.Errorf("bản ghi phải được dọn khi áp cấu hình thất bại: %+v", sites)
	}
}

func TestWebsiteSSLRequiresExistingCertificate(t *testing.T) {
	websites, certificates, _ := newWebsiteFixture(t)
	ctx := context.Background()

	req := WebsiteRequest{
		Name: "an-toan", Domains: []string{"secure.example.com"},
		Type: "static", Root: t.TempDir(), Enabled: true,
		SSLEnabled: true, ForceHTTPS: true, CertName: "chua-co",
	}

	// Trỏ vào chứng chỉ không tồn tại sẽ khiến nginx từ chối khởi động và kéo
	// sập mọi website khác, nên phải chặn từ lúc tạo.
	if _, err := websites.Create(ctx, req); !errors.Is(err, apperr.CertNotFound) {
		t.Fatalf("chứng chỉ không tồn tại phải bị từ chối, nhận: %v", err)
	}

	if _, err := certificates.Issue(ctx, CertRequest{
		Name: "chua-co", Domains: []string{"secure.example.com"}, Source: "self",
	}); err != nil {
		t.Fatalf("tạo chứng chỉ: %v", err)
	}

	site, err := websites.Create(ctx, req)
	if err != nil {
		t.Fatalf("tạo website sau khi đã có chứng chỉ: %v", err)
	}
	if !site.SSLEnabled || site.CertName != "chua-co" {
		t.Errorf("website chưa gắn đúng chứng chỉ: %+v", site)
	}
}

func TestDisableWebsiteRemovesConfig(t *testing.T) {
	websites, _, server := newWebsiteFixture(t)
	ctx := context.Background()

	site, err := websites.Create(ctx, WebsiteRequest{
		Name: "tam-tat", Domains: []string{"off.example.com"},
		Type: "static", Root: t.TempDir(), Enabled: true,
	})
	if err != nil {
		t.Fatalf("tạo website: %v", err)
	}

	if _, err := websites.SetEnabled(ctx, site.ID, false); err != nil {
		t.Fatalf("tắt website: %v", err)
	}

	// Máy chủ web không có khái niệm "khối server đang tắt", nên tắt phải là gỡ
	// hẳn tệp cấu hình.
	if _, ok := server.applied["tam-tat"]; ok {
		t.Error("website đã tắt nhưng cấu hình vẫn còn")
	}
}

func TestRenameWebsiteRemovesOldConfig(t *testing.T) {
	websites, _, server := newWebsiteFixture(t)
	ctx := context.Background()

	root := t.TempDir()
	site, err := websites.Create(ctx, WebsiteRequest{
		Name: "ten-cu", Domains: []string{"rename.example.com"},
		Type: "static", Root: root, Enabled: true,
	})
	if err != nil {
		t.Fatalf("tạo website: %v", err)
	}

	_, err = websites.Update(ctx, site.ID, WebsiteRequest{
		Name: "ten-moi", Domains: []string{"rename.example.com"},
		Type: "static", Root: root, Enabled: true,
	})
	if err != nil {
		t.Fatalf("đổi tên website: %v", err)
	}

	// Bỏ sót tệp cũ nghĩa là website vẫn phục vụ dưới cấu hình cũ, và hai khối
	// server sẽ cùng khai báo một tên miền.
	if _, ok := server.applied["ten-cu"]; ok {
		t.Error("tệp cấu hình cũ chưa được gỡ sau khi đổi tên")
	}
	if _, ok := server.applied["ten-moi"]; !ok {
		t.Error("chưa ghi cấu hình dưới tên mới")
	}
}

func TestDeleteCertificateInUseIsRejected(t *testing.T) {
	websites, certificates, _ := newWebsiteFixture(t)
	ctx := context.Background()

	if _, err := certificates.Issue(ctx, CertRequest{
		Name: "dang-dung", Domains: []string{"used.example.com"}, Source: "self",
	}); err != nil {
		t.Fatalf("tạo chứng chỉ: %v", err)
	}
	if _, err := websites.Create(ctx, WebsiteRequest{
		Name: "trang", Domains: []string{"used.example.com"},
		Type: "static", Root: t.TempDir(), Enabled: true,
		SSLEnabled: true, CertName: "dang-dung",
	}); err != nil {
		t.Fatalf("tạo website: %v", err)
	}

	usedBy, err := websites.UsedCertificates(ctx, "dang-dung")
	if err != nil {
		t.Fatalf("tra cứu website dùng chứng chỉ: %v", err)
	}

	// Xóa chứng chỉ đang dùng làm nginx không khởi động lại được.
	if err := certificates.Delete(ctx, "dang-dung", usedBy); !errors.Is(err, apperr.CertInUse) {
		t.Errorf("chứng chỉ đang dùng phải từ chối xóa, nhận: %v", err)
	}
}

func TestUploadedCertificateRejectsMismatchedKey(t *testing.T) {
	_, certificates, _ := newWebsiteFixture(t)
	ctx := context.Background()

	certPEM, _, err := certs.GenerateSelfSigned([]string{"a.example.com"})
	if err != nil {
		t.Fatalf("sinh chứng chỉ: %v", err)
	}
	// Khóa của một chứng chỉ khác: cặp không khớp sẽ khiến nginx từ chối khởi động.
	_, otherKeyPEM, err := certs.GenerateSelfSigned([]string{"b.example.com"})
	if err != nil {
		t.Fatalf("sinh chứng chỉ thứ hai: %v", err)
	}

	_, err = certificates.Issue(ctx, CertRequest{
		Name: "lech", Domains: []string{"a.example.com"}, Source: "upload",
		CertPEM: string(certPEM), KeyPEM: string(otherKeyPEM),
	})
	if !errors.Is(err, apperr.CertKeyMismatch) {
		t.Errorf("cặp chứng chỉ/khóa lệch nhau phải bị từ chối, nhận: %v", err)
	}
}

// GORM bỏ qua trường mang giá trị zero khi cột có giá trị mặc định, nên một
// lựa chọn "tắt" của người dùng sẽ lặng lẽ biến thành "bật" khi ghi xuống đĩa.
func TestCreateWebsiteRespectsDisabledFlag(t *testing.T) {
	websites, _, server := newWebsiteFixture(t)
	ctx := context.Background()

	site, err := websites.Create(ctx, WebsiteRequest{
		Name: "tat-san", Domains: []string{"draft.example.com"},
		Type: "static", Root: t.TempDir(), Enabled: false,
	})
	if err != nil {
		t.Fatalf("tạo website: %v", err)
	}
	if site.Enabled {
		t.Error("website được tạo ở trạng thái tắt nhưng lại báo là đang bật")
	}
	if _, ok := server.applied["tat-san"]; ok {
		t.Error("website tắt không được ghi cấu hình ra máy chủ web")
	}

	// Đọc lại từ cơ sở dữ liệu: giá trị mặc định của cột chỉ lộ ra sau khi ghi.
	stored, err := websites.Get(ctx, site.ID)
	if err != nil {
		t.Fatalf("đọc lại website: %v", err)
	}
	if stored.Enabled {
		t.Error("trạng thái tắt không được lưu xuống cơ sở dữ liệu")
	}
}

// Chứng chỉ tự ký và chứng chỉ tải lên không gia hạn tự động được — bật cờ cho
// chúng sẽ tạo ra một vòng thử lại thất bại mãi mãi.
func TestSelfSignedCertificateDoesNotAutoRenew(t *testing.T) {
	_, certificates, _ := newWebsiteFixture(t)
	ctx := context.Background()

	info, err := certificates.Issue(ctx, CertRequest{
		Name: "tu-ky", Domains: []string{"self.example.com"},
		Source: "self", AutoRenew: true,
	})
	if err != nil {
		t.Fatalf("tạo chứng chỉ: %v", err)
	}
	if info.AutoRenew {
		t.Error("chứng chỉ tự ký không được bật tự gia hạn")
	}

	stored, err := certificates.Get(ctx, "tu-ky")
	if err != nil {
		t.Fatalf("đọc lại chứng chỉ: %v", err)
	}
	if stored.AutoRenew {
		t.Error("cờ tự gia hạn bị bật lại khi ghi xuống cơ sở dữ liệu")
	}
}
