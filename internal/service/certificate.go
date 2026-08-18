package service

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/model"
	"github.com/thanhtinz/sunpanel/pkg/certs"
	"github.com/thanhtinz/sunpanel/pkg/notify"
	"github.com/thanhtinz/sunpanel/pkg/webserver"
)

// renewCheckInterval là chu kỳ rà soát chứng chỉ sắp hết hạn.
//
// Mười hai tiếng: chứng chỉ Let's Encrypt sống 90 ngày và được gia hạn khi còn
// 30 ngày, nên quét thưa hơn vẫn kịp, nhưng quét dày giúp một máy chủ hay tắt mở
// vẫn có cơ hội chạm được mốc gia hạn.
const renewCheckInterval = 12 * time.Hour

// CertRequest là dữ liệu tạo hoặc xin một chứng chỉ.
type CertRequest struct {
	Name    string   `json:"name" binding:"required"`
	Domains []string `json:"domains" binding:"required"`
	// Source là nguồn chứng chỉ: self, acme hoặc upload.
	Source string `json:"source" binding:"required"`
	// Email là địa chỉ liên hệ đăng ký với máy chủ ACME.
	Email string `json:"email"`
	// Staging dùng máy chủ thử nghiệm của Let's Encrypt.
	Staging bool `json:"staging"`
	// AutoRenew bật tự động gia hạn.
	AutoRenew bool `json:"autoRenew"`
	// CertPEM và KeyPEM là nội dung chứng chỉ tải lên, dùng với source "upload".
	CertPEM string `json:"certPem"`
	KeyPEM  string `json:"keyPem"`
}

// CertificateInfo gộp bản ghi trong cơ sở dữ liệu với thông tin đọc từ tệp.
type CertificateInfo struct {
	model.Certificate
	// Issuer, NotAfter và SelfSigned lấy từ chính tệp chứng chỉ trên đĩa.
	Issuer string `json:"issuer"`
	// DaysRemaining là số ngày còn lại; giá trị âm nghĩa là đã hết hạn.
	DaysRemaining int `json:"daysRemaining"`
	// Missing đánh dấu bản ghi còn trong cơ sở dữ liệu nhưng tệp đã biến mất.
	Missing bool `json:"missing"`
}

// CertificateService quản lý kho chứng chỉ TLS.
type CertificateService struct {
	db     *gorm.DB
	store  *certs.Store
	solver *certs.WebrootSolver
	// accountKeyPath là nơi lưu khóa tài khoản ACME.
	accountKeyPath string
	audit          *AuditService
	// reload được gọi sau khi chứng chỉ đổi, để máy chủ web nạp lại chứng chỉ mới.
	reload func(ctx context.Context) error
	// alerts nhận thông báo khi gia hạn thất bại; có thể nil.
	alerts *AlertService
}

// SetAlerts gắn dịch vụ cảnh báo.
func (s *CertificateService) SetAlerts(alerts *AlertService) { s.alerts = alerts }

// NewCertificateService tạo dịch vụ chứng chỉ.
func NewCertificateService(
	db *gorm.DB, store *certs.Store, solver *certs.WebrootSolver,
	accountKeyPath string, audit *AuditService,
) *CertificateService {
	return &CertificateService{
		db: db, store: store, solver: solver,
		accountKeyPath: accountKeyPath, audit: audit,
	}
}

// SetReloader đặt hàm nạp lại máy chủ web sau khi chứng chỉ thay đổi.
//
// Đặt sau khi khởi tạo thay vì truyền vào hàm dựng: dịch vụ website cần dịch vụ
// chứng chỉ, nên truyền theo chiều ngược lại sẽ tạo phụ thuộc vòng.
func (s *CertificateService) SetReloader(reload func(ctx context.Context) error) {
	s.reload = reload
}

// List liệt kê mọi chứng chỉ.
func (s *CertificateService) List(ctx context.Context) ([]CertificateInfo, error) {
	var records []model.Certificate
	if err := s.db.WithContext(ctx).Order("name").Find(&records).Error; err != nil {
		return nil, apperr.Internal.Wrap(err)
	}

	out := make([]CertificateInfo, 0, len(records))
	for _, record := range records {
		out = append(out, s.describe(record))
	}
	return out, nil
}

// Get đọc một chứng chỉ theo tên.
func (s *CertificateService) Get(ctx context.Context, name string) (CertificateInfo, error) {
	record, err := s.find(ctx, name)
	if err != nil {
		return CertificateInfo{}, err
	}
	return s.describe(record), nil
}

// describe bổ sung thông tin đọc từ tệp chứng chỉ trên đĩa.
//
// Bản ghi và tệp có thể lệch nhau — quản trị viên xóa tệp bằng tay, hoặc đĩa
// hỏng — nên trạng thái hiển thị luôn lấy từ tệp, và bản ghi mồ côi được đánh dấu
// thay vì làm hỏng cả danh sách.
func (s *CertificateService) describe(record model.Certificate) CertificateInfo {
	info := CertificateInfo{Certificate: record}
	loaded, err := s.store.Load(record.Name)
	if err != nil {
		info.Missing = true
		return info
	}
	info.Issuer = loaded.Issuer
	info.ExpiresAt = loaded.NotAfter
	info.IssuedAt = loaded.NotBefore
	info.DaysRemaining = loaded.DaysRemaining()
	return info
}

func (s *CertificateService) find(ctx context.Context, name string) (model.Certificate, error) {
	var record model.Certificate
	err := s.db.WithContext(ctx).Where("name = ?", name).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Certificate{}, apperr.CertNotFound.WithParam("name", name)
	}
	if err != nil {
		return model.Certificate{}, apperr.Internal.Wrap(err)
	}
	return record, nil
}

// Issue tạo hoặc xin một chứng chỉ mới.
func (s *CertificateService) Issue(ctx context.Context, req CertRequest) (CertificateInfo, error) {
	name := strings.TrimSpace(req.Name)
	if _, err := s.store.Dir(name); err != nil {
		return CertificateInfo{}, apperr.CertInvalidName.WithParam("name", req.Name)
	}

	domains, err := normalizeDomains(req.Domains)
	if err != nil {
		return CertificateInfo{}, err
	}

	var count int64
	if err := s.db.WithContext(ctx).Model(&model.Certificate{}).
		Where("name = ?", name).Count(&count).Error; err != nil {
		return CertificateInfo{}, apperr.Internal.Wrap(err)
	}
	if count > 0 {
		return CertificateInfo{}, apperr.CertNameExists.WithParam("name", name)
	}

	certPEM, keyPEM, err := s.material(ctx, req, domains)
	if err != nil {
		return CertificateInfo{}, err
	}

	if err := s.store.Save(name, certPEM, keyPEM); err != nil {
		return CertificateInfo{}, apperr.Internal.Wrap(err)
	}

	parsed, err := certs.Parse(certPEM)
	if err != nil {
		return CertificateInfo{}, apperr.CertInvalid.Wrap(err)
	}

	record := model.Certificate{
		Name:      name,
		Domains:   strings.Join(domains, ","),
		Source:    req.Source,
		Email:     strings.TrimSpace(req.Email),
		Staging:   req.Staging,
		AutoRenew: req.AutoRenew && req.Source == certSourceACME,
		IssuedAt:  parsed.NotBefore,
		ExpiresAt: parsed.NotAfter,
	}
	if err := s.db.WithContext(ctx).Create(&record).Error; err != nil {
		// Bản ghi hỏng nhưng tệp đã ghi: dọn tệp để lần thử lại không vướng.
		_ = s.store.Delete(name)
		return CertificateInfo{}, apperr.Internal.Wrap(err)
	}

	return s.describe(record), nil
}

// Các nguồn chứng chỉ.
const (
	certSourceSelf   = "self"
	certSourceACME   = "acme"
	certSourceUpload = "upload"
)

// material sinh hoặc nhận nội dung chứng chỉ theo nguồn yêu cầu.
func (s *CertificateService) material(
	ctx context.Context, req CertRequest, domains []string,
) (certPEM, keyPEM []byte, err error) {
	switch req.Source {
	case certSourceSelf:
		certPEM, keyPEM, err = certs.GenerateSelfSigned(domains)
		if err != nil {
			return nil, nil, apperr.CertInvalid.Wrap(err)
		}
		return certPEM, keyPEM, nil

	case certSourceUpload:
		return validateKeyPair([]byte(req.CertPEM), []byte(req.KeyPEM))

	case certSourceACME:
		email := strings.TrimSpace(req.Email)
		if !strings.Contains(email, "@") {
			return nil, nil, apperr.CertInvalidEmail.WithParam("email", req.Email)
		}
		client := certs.NewACMEClient(email, s.accountKeyPath, req.Staging)
		certPEM, keyPEM, err = client.Obtain(ctx, domains, s.solver)
		if err != nil {
			return nil, nil, apperr.CertIssueFailed.Wrap(err)
		}
		return certPEM, keyPEM, nil
	}

	return nil, nil, apperr.CertInvalid.WithParam("source", req.Source)
}

// validateKeyPair kiểm tra chứng chỉ và khóa tải lên có khớp nhau không.
//
// Ghi một cặp không khớp xuống đĩa rồi nạp lại máy chủ web sẽ làm nginx từ chối
// khởi động và kéo sập mọi website khác, nên phải chặn ngay tại đây.
func validateKeyPair(certPEM, keyPEM []byte) ([]byte, []byte, error) {
	if len(certPEM) == 0 || len(keyPEM) == 0 {
		return nil, nil, apperr.CertInvalid
	}
	if _, err := certs.Parse(certPEM); err != nil {
		return nil, nil, apperr.CertInvalid.Wrap(err)
	}
	if _, err := tls.X509KeyPair(certPEM, keyPEM); err != nil {
		return nil, nil, apperr.CertKeyMismatch.Wrap(err)
	}
	return certPEM, keyPEM, nil
}

// Renew xin lại chứng chỉ cho một bản ghi đã có.
func (s *CertificateService) Renew(ctx context.Context, name string) (CertificateInfo, error) {
	record, err := s.find(ctx, name)
	if err != nil {
		return CertificateInfo{}, err
	}

	domains := splitDomains(record.Domains)
	req := CertRequest{
		Source:  record.Source,
		Email:   record.Email,
		Staging: record.Staging,
	}

	certPEM, keyPEM, err := s.material(ctx, req, domains)
	if err != nil {
		s.recordFailure(ctx, &record, err)
		return CertificateInfo{}, err
	}

	if err := s.store.Save(record.Name, certPEM, keyPEM); err != nil {
		return CertificateInfo{}, apperr.Internal.Wrap(err)
	}

	parsed, err := certs.Parse(certPEM)
	if err != nil {
		return CertificateInfo{}, apperr.CertInvalid.Wrap(err)
	}

	now := time.Now()
	record.IssuedAt = parsed.NotBefore
	record.ExpiresAt = parsed.NotAfter
	record.LastRenewAt = &now
	record.LastError = ""
	if err := s.db.WithContext(ctx).Save(&record).Error; err != nil {
		return CertificateInfo{}, apperr.Internal.Wrap(err)
	}

	// Chứng chỉ mới chỉ có tác dụng sau khi máy chủ web đọc lại tệp.
	if s.reload != nil {
		if err := s.reload(ctx); err != nil {
			slog.Warn("không nạp lại được máy chủ web sau khi gia hạn chứng chỉ",
				"cert", record.Name, "error", err)
		}
	}

	return s.describe(record), nil
}

// recordFailure lưu lại lỗi của lần gia hạn để quản trị viên nhìn thấy.
func (s *CertificateService) recordFailure(ctx context.Context, record *model.Certificate, cause error) {
	now := time.Now()
	record.LastRenewAt = &now
	record.LastError = cause.Error()
	if err := s.db.WithContext(ctx).Save(record).Error; err != nil {
		slog.Warn("không ghi được lỗi gia hạn chứng chỉ", "cert", record.Name, "error", err)
	}
}

// Delete xóa một chứng chỉ khỏi kho.
//
// usedBy cho biết các website đang dùng chứng chỉ; còn website nào dùng thì
// không cho xóa, vì xóa sẽ khiến máy chủ web từ chối khởi động.
func (s *CertificateService) Delete(ctx context.Context, name string, usedBy []string) error {
	record, err := s.find(ctx, name)
	if err != nil {
		return err
	}
	if len(usedBy) > 0 {
		return apperr.CertInUse.WithParam("websites", strings.Join(usedBy, ", "))
	}

	if err := s.store.Delete(record.Name); err != nil {
		return apperr.Internal.Wrap(err)
	}
	if err := s.db.WithContext(ctx).Delete(&record).Error; err != nil {
		return apperr.Internal.Wrap(err)
	}
	return nil
}

// Paths trả về đường dẫn tệp chứng chỉ và khóa của một chứng chỉ đã có.
func (s *CertificateService) Paths(ctx context.Context, name string) (certFile, keyFile string, err error) {
	if _, err := s.find(ctx, name); err != nil {
		return "", "", err
	}
	certFile, keyFile, err = s.store.Paths(name)
	if err != nil {
		return "", "", apperr.CertInvalidName.WithParam("name", name)
	}
	return certFile, keyFile, nil
}

// RunRenewal chạy vòng lặp tự động gia hạn tới khi ctx bị hủy.
func (s *CertificateService) RunRenewal(ctx context.Context) {
	ticker := time.NewTicker(renewCheckInterval)
	defer ticker.Stop()

	// Quét ngay một lần lúc khởi động: máy chủ tắt vài tuần rồi bật lại có thể
	// đã có chứng chỉ quá hạn gia hạn từ lâu.
	s.renewDue(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.renewDue(ctx)
		}
	}
}

// renewDue gia hạn mọi chứng chỉ đã tới hạn.
func (s *CertificateService) renewDue(ctx context.Context) {
	var records []model.Certificate
	err := s.db.WithContext(ctx).
		Where("auto_renew = ? AND source = ?", true, certSourceACME).
		Find(&records).Error
	if err != nil {
		slog.Warn("không đọc được danh sách chứng chỉ cần gia hạn", "error", err)
		return
	}

	for _, record := range records {
		info := s.describe(record)
		if !info.Missing && info.DaysRemaining >= 30 {
			continue
		}
		slog.Info("gia hạn chứng chỉ", "cert", record.Name, "daysRemaining", info.DaysRemaining)
		if _, err := s.Renew(ctx, record.Name); err != nil {
			slog.Error("gia hạn chứng chỉ thất bại", "cert", record.Name, "error", err)
			// Gia hạn hỏng phải báo ra ngoài: nếu không, website sẽ ngừng phục vụ
			// HTTPS vào đúng ngày chứng chỉ hết hạn mà không ai được báo trước.
			s.alertRenewalFailure(ctx, record, info, err)
			continue
		}
		slog.Info("đã gia hạn chứng chỉ", "cert", record.Name)
	}
}

// alertRenewalFailure phát cảnh báo khi việc gia hạn chứng chỉ thất bại.
func (s *CertificateService) alertRenewalFailure(
	ctx context.Context, record model.Certificate, info CertificateInfo, cause error,
) {
	if s.alerts == nil {
		return
	}

	// Còn ít ngày thì đây là sự cố, còn nhiều ngày thì mới chỉ là cảnh báo — vẫn
	// còn thời gian để thử lại ở các lần quét sau.
	level := notify.LevelWarning
	if info.DaysRemaining < 7 {
		level = notify.LevelCritical
	}

	s.alerts.Notify(ctx, "cert", notify.Message{
		Title: "Gia hạn chứng chỉ thất bại: " + record.Name,
		Body:  cause.Error(),
		Level: level,
		Fields: map[string]string{
			"Chứng chỉ": record.Name,
			"Tên miền":  record.Domains,
			"Còn lại":   fmt.Sprintf("%d ngày", info.DaysRemaining),
		},
	})
}

// normalizeDomains chuẩn hóa và kiểm tra danh sách tên miền.
func normalizeDomains(domains []string) ([]string, error) {
	seen := make(map[string]struct{}, len(domains))
	out := make([]string, 0, len(domains))

	for _, domain := range domains {
		name := strings.ToLower(strings.TrimSpace(domain))
		if name == "" {
			continue
		}
		if err := webserver.ValidateDomain(name); err != nil {
			return nil, apperr.WebsiteInvalidDomain.WithParam("domain", domain)
		}
		if _, duplicate := seen[name]; duplicate {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}

	if len(out) == 0 {
		return nil, apperr.WebsiteInvalidDomain.WithParam("domain", "")
	}
	return out, nil
}

// splitDomains tách chuỗi tên miền đã lưu thành danh sách.
func splitDomains(joined string) []string {
	parts := strings.Split(joined, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if name := strings.TrimSpace(part); name != "" {
			out = append(out, name)
		}
	}
	return out
}
