package service

import (
	"context"
	"errors"
	"path"
	"strings"

	"gorm.io/gorm"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/model"
	"github.com/thanhtinz/sunpanel/pkg/host"
	"github.com/thanhtinz/sunpanel/pkg/webserver"
)

// WebsiteRequest là dữ liệu tạo hoặc sửa một website.
type WebsiteRequest struct {
	Name        string   `json:"name" binding:"required"`
	Domains     []string `json:"domains" binding:"required"`
	Type        string   `json:"type" binding:"required"`
	Root        string   `json:"root"`
	ProxyTarget string   `json:"proxyTarget"`
	PHPSocket   string   `json:"phpSocket"`
	Port        int      `json:"port"`
	SSLPort     int      `json:"sslPort"`
	SSLEnabled  bool     `json:"sslEnabled"`
	ForceHTTPS  bool     `json:"forceHttps"`
	CertName    string   `json:"certName"`
	ExtraConfig string   `json:"extraConfig"`
	Enabled     bool     `json:"enabled"`
	Remark      string   `json:"remark"`

	// AuthEnabled bật lớp hỏi mật khẩu; AuthPassword để trống nghĩa là giữ mật
	// khẩu cũ, vì giao diện không bao giờ nhận lại được mật khẩu đã lưu.
	AuthEnabled  bool           `json:"authEnabled"`
	AuthUser     string         `json:"authUser"`
	AuthPassword string         `json:"authPassword"`
	DenyIPs      []string       `json:"denyIps"`
	Redirects    []RedirectRule `json:"redirects"`
}

// RedirectRule là một quy tắc chuyển hướng do người dùng đặt.
type RedirectRule struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Permanent bool   `json:"permanent"`
}

// WebsiteService quản lý website và cấu hình máy chủ web tương ứng.
type WebsiteService struct {
	db      *gorm.DB
	manager webserver.Manager
	certs   *CertificateService
	// host dùng để tạo thư mục gốc của website.
	//
	// Thư mục gốc do quản trị viên chọn và thường nằm ngoài phạm vi của trình
	// quản lý tệp, nên đây là một host riêng không giới hạn phạm vi — cùng cách
	// bộ lập lịch tác vụ định kỳ được cấp host riêng.
	host host.Host
	// acmeWebroot là thư mục dùng chung phục vụ tệp xác thực ACME.
	acmeWebroot string
	// authDir là nơi panel ghi tệp tài khoản của lớp bảo vệ mật khẩu.
	authDir string
	audit   *AuditService
}

// NewWebsiteService tạo dịch vụ website.
func NewWebsiteService(
	db *gorm.DB, manager webserver.Manager, certificates *CertificateService,
	h host.Host, acmeWebroot, authDir string, audit *AuditService,
) *WebsiteService {
	return &WebsiteService{
		db: db, manager: manager, certs: certificates, host: h, authDir: authDir,
		acmeWebroot: acmeWebroot, audit: audit,
	}
}

// ServerName trả về tên phần mềm máy chủ web đang dùng.
func (s *WebsiteService) ServerName() string { return s.manager.Name() }

// Available cho biết máy chủ web có dùng được không.
func (s *WebsiteService) Available(ctx context.Context) bool { return s.manager.Available(ctx) }

// Reload nạp lại cấu hình máy chủ web.
func (s *WebsiteService) Reload(ctx context.Context) error {
	if err := s.manager.Reload(ctx); err != nil {
		return apperr.WebsiteApplyFailed.Wrap(err)
	}
	return nil
}

// List liệt kê mọi website.
func (s *WebsiteService) List(ctx context.Context) ([]model.Website, error) {
	var sites []model.Website
	if err := s.db.WithContext(ctx).Order("name").Find(&sites).Error; err != nil {
		return nil, apperr.Internal.Wrap(err)
	}
	return sites, nil
}

// Get đọc một website theo mã.
func (s *WebsiteService) Get(ctx context.Context, id uint) (model.Website, error) {
	var site model.Website
	err := s.db.WithContext(ctx).First(&site, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Website{}, apperr.WebsiteNotFound
	}
	if err != nil {
		return model.Website{}, apperr.Internal.Wrap(err)
	}
	return site, nil
}

// Create tạo website mới và ghi cấu hình máy chủ web.
func (s *WebsiteService) Create(ctx context.Context, req WebsiteRequest) (model.Website, error) {
	site, err := s.build(ctx, model.Website{Enabled: true}, req, 0)
	if err != nil {
		return model.Website{}, err
	}

	if err := s.db.WithContext(ctx).Create(&site).Error; err != nil {
		return model.Website{}, apperr.Internal.Wrap(err)
	}

	if err := s.apply(ctx, site); err != nil {
		// Cấu hình không áp được thì website coi như chưa tồn tại: giữ lại bản ghi
		// sẽ khiến danh sách hiển thị một website mà máy chủ web không hề biết.
		s.db.WithContext(ctx).Delete(&site)
		return model.Website{}, err
	}

	return site, nil
}

// Update sửa website đã có.
func (s *WebsiteService) Update(ctx context.Context, id uint, req WebsiteRequest) (model.Website, error) {
	existing, err := s.Get(ctx, id)
	if err != nil {
		return model.Website{}, err
	}

	updated, err := s.build(ctx, existing, req, id)
	if err != nil {
		return model.Website{}, err
	}

	if err := s.db.WithContext(ctx).Save(&updated).Error; err != nil {
		return model.Website{}, apperr.Internal.Wrap(err)
	}

	// Đổi tên nghĩa là đổi tên tệp cấu hình; tệp cũ phải gỡ đi, nếu không website
	// vẫn còn phục vụ dưới cấu hình cũ.
	if existing.Name != updated.Name {
		if err := s.manager.Remove(ctx, existing.Name); err != nil {
			return model.Website{}, apperr.WebsiteApplyFailed.Wrap(err)
		}
	}

	if err := s.apply(ctx, updated); err != nil {
		// Khôi phục bản ghi cũ để cơ sở dữ liệu khớp lại với cấu hình đang chạy.
		s.db.WithContext(ctx).Save(&existing)
		_ = s.apply(ctx, existing)
		return model.Website{}, err
	}

	return updated, nil
}

// Delete xóa website và cấu hình của nó.
func (s *WebsiteService) Delete(ctx context.Context, id uint) error {
	site, err := s.Get(ctx, id)
	if err != nil {
		return err
	}

	if err := s.manager.Remove(ctx, site.Name); err != nil {
		return apperr.WebsiteApplyFailed.Wrap(err)
	}
	if err := s.db.WithContext(ctx).Delete(&site).Error; err != nil {
		return apperr.Internal.Wrap(err)
	}
	return nil
}

// SetEnabled bật hoặc tắt một website.
func (s *WebsiteService) SetEnabled(ctx context.Context, id uint, enabled bool) (model.Website, error) {
	site, err := s.Get(ctx, id)
	if err != nil {
		return model.Website{}, err
	}
	if site.Enabled == enabled {
		return site, nil
	}

	site.Enabled = enabled
	if err := s.db.WithContext(ctx).Save(&site).Error; err != nil {
		return model.Website{}, apperr.Internal.Wrap(err)
	}
	if err := s.apply(ctx, site); err != nil {
		return model.Website{}, err
	}
	return site, nil
}

// Render sinh nội dung cấu hình của một website mà không ghi ra đĩa.
func (s *WebsiteService) Render(ctx context.Context, id uint) (string, error) {
	site, err := s.Get(ctx, id)
	if err != nil {
		return "", err
	}
	config, err := s.toSite(ctx, site)
	if err != nil {
		return "", err
	}
	content, err := s.manager.Render(config)
	if err != nil {
		return "", apperr.WebsiteInvalidConfig.Wrap(err)
	}
	return content, nil
}

// UsedCertificates trả về tên các website đang dùng một chứng chỉ.
func (s *WebsiteService) UsedCertificates(ctx context.Context, certName string) ([]string, error) {
	var sites []model.Website
	err := s.db.WithContext(ctx).Where("cert_name = ?", certName).Find(&sites).Error
	if err != nil {
		return nil, apperr.Internal.Wrap(err)
	}
	names := make([]string, 0, len(sites))
	for _, site := range sites {
		names = append(names, site.Name)
	}
	return names, nil
}

// build kiểm tra yêu cầu và dựng bản ghi website.
//
// selfID là mã của chính website đang sửa, để việc kiểm trùng tên miền không tự
// coi website đó là kẻ chiếm chỗ.
func (s *WebsiteService) build(
	ctx context.Context, base model.Website, req WebsiteRequest, selfID uint,
) (model.Website, error) {
	name := strings.TrimSpace(req.Name)
	domains, err := normalizeDomains(req.Domains)
	if err != nil {
		return model.Website{}, err
	}

	if err := s.checkNameFree(ctx, name, selfID); err != nil {
		return model.Website{}, err
	}
	if err := s.checkDomainsFree(ctx, domains, selfID); err != nil {
		return model.Website{}, err
	}

	site := base
	site.Name = name
	site.Domains = strings.Join(domains, ",")
	site.Type = req.Type
	site.Root = strings.TrimSpace(req.Root)
	site.ProxyTarget = strings.TrimSpace(req.ProxyTarget)
	site.PHPSocket = strings.TrimSpace(req.PHPSocket)
	site.Port = defaultPort(req.Port, 80)
	site.SSLPort = defaultPort(req.SSLPort, 443)
	site.SSLEnabled = req.SSLEnabled
	site.ForceHTTPS = req.SSLEnabled && req.ForceHTTPS
	site.CertName = strings.TrimSpace(req.CertName)
	site.ExtraConfig = req.ExtraConfig
	site.Enabled = req.Enabled
	site.Remark = strings.TrimSpace(req.Remark)

	if err := s.applyProtection(&site, req); err != nil {
		return model.Website{}, err
	}

	if site.SSLEnabled && site.CertName == "" {
		return model.Website{}, apperr.WebsiteInvalidConfig.WithParam("field", "certName")
	}

	// Kiểm cấu hình trước khi chạm vào cơ sở dữ liệu: mọi giá trị ở đây sẽ được
	// ghi vào tệp cấu hình của một tiến trình chạy quyền root.
	if _, err := s.toSite(ctx, site); err != nil {
		return model.Website{}, err
	}

	return site, nil
}

func (s *WebsiteService) checkNameFree(ctx context.Context, name string, selfID uint) error {
	var count int64
	err := s.db.WithContext(ctx).Model(&model.Website{}).
		Where("name = ? AND id <> ?", name, selfID).Count(&count).Error
	if err != nil {
		return apperr.Internal.Wrap(err)
	}
	if count > 0 {
		return apperr.WebsiteNameExists.WithParam("name", name)
	}
	return nil
}

// checkDomainsFree đảm bảo không có tên miền nào thuộc về hai website.
//
// Hai khối server cùng server_name khiến nginx lặng lẽ chọn khối đầu tiên, nên
// website thứ hai sẽ "tạo thành công" mà không bao giờ phục vụ.
func (s *WebsiteService) checkDomainsFree(ctx context.Context, domains []string, selfID uint) error {
	var others []model.Website
	err := s.db.WithContext(ctx).Where("id <> ?", selfID).Find(&others).Error
	if err != nil {
		return apperr.Internal.Wrap(err)
	}

	taken := make(map[string]string)
	for _, other := range others {
		for _, domain := range splitDomains(other.Domains) {
			taken[domain] = other.Name
		}
	}

	for _, domain := range domains {
		if owner, exists := taken[domain]; exists {
			return apperr.WebsiteDomainTaken.
				WithParam("domain", domain).WithParam("website", owner)
		}
	}
	return nil
}

// toSite chuyển bản ghi thành cấu hình cho máy chủ web.
func (s *WebsiteService) toSite(ctx context.Context, site model.Website) (webserver.Site, error) {
	config := webserver.Site{
		Name:        site.Name,
		Domains:     splitDomains(site.Domains),
		Type:        webserver.SiteType(site.Type),
		Root:        site.Root,
		ProxyTarget: site.ProxyTarget,
		PHPSocket:   site.PHPSocket,
		Port:        site.Port,
		ExtraConfig: site.ExtraConfig,
		ACMEWebroot: s.acmeWebroot,
		SSL: webserver.SSLConfig{
			Enabled:    site.SSLEnabled,
			ForceHTTPS: site.ForceHTTPS,
			Port:       site.SSLPort,
		},
		DenyIPs:   splitLines(site.DenyIPs),
		Redirects: toWebserverRedirects(decodeRedirects(site.Redirects)),
	}

	if site.AuthEnabled {
		config.Auth = webserver.AuthConfig{
			Enabled: true,
			Realm:   "SunPanel",
			File:    s.authFile(site.Name),
		}
	}

	if site.SSLEnabled {
		certFile, keyFile, err := s.certs.Paths(ctx, site.CertName)
		if err != nil {
			return webserver.Site{}, err
		}
		config.SSL.CertFile = certFile
		config.SSL.KeyFile = keyFile
	}

	if err := webserver.ValidateSite(config); err != nil {
		return webserver.Site{}, apperr.WebsiteInvalidConfig.Wrap(err)
	}
	return config, nil
}

// apply ghi cấu hình của một website xuống đĩa và nạp lại máy chủ web.
func (s *WebsiteService) apply(ctx context.Context, site model.Website) error {
	if !s.manager.Available(ctx) {
		return apperr.WebsiteServerUnavailable.WithParam("server", s.manager.Name())
	}

	// Website tắt thì gỡ hẳn tệp cấu hình thay vì để lại: máy chủ web không có
	// khái niệm "khối server đang tắt".
	if !site.Enabled {
		if err := s.manager.Remove(ctx, site.Name); err != nil {
			return apperr.WebsiteApplyFailed.Wrap(err)
		}
		return nil
	}

	config, err := s.toSite(ctx, site)
	if err != nil {
		return err
	}

	// Tệp tài khoản phải có mặt trước khi nginx nạp cấu hình trỏ tới nó, nếu
	// không nginx từ chối nạp và kéo theo mọi website khác cùng sập.
	if err := s.writeAuthFile(ctx, site); err != nil {
		return err
	}

	// Thư mục gốc phải tồn tại trước khi nginx nạp cấu hình, nếu không mọi yêu
	// cầu tới website đều trả về lỗi 404 mà không rõ nguyên nhân.
	if config.Type != webserver.SiteProxy && config.Root != "" {
		if err := s.host.FS().Mkdir(ctx, config.Root, 0o755); err != nil {
			return apperr.WebsiteApplyFailed.Wrap(err)
		}
	}
	if s.acmeWebroot != "" {
		challengeDir := path.Join(s.acmeWebroot, ".well-known", "acme-challenge")
		if err := s.host.FS().Mkdir(ctx, challengeDir, 0o755); err != nil {
			return apperr.WebsiteApplyFailed.Wrap(err)
		}
	}

	if err := s.manager.Apply(ctx, config); err != nil {
		return apperr.WebsiteApplyFailed.Wrap(err)
	}
	return nil
}

// defaultPort trả về cổng đã cho, hoặc giá trị mặc định nếu chưa đặt.
func defaultPort(port, fallback int) int {
	if port <= 0 || port > 65535 {
		return fallback
	}
	return port
}
