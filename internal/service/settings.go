package service

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/config"
	"github.com/thanhtinz/sunpanel/pkg/crypto"
)

// entryPathPattern giới hạn ký tự của đường dẫn bí mật.
//
// Đường dẫn này nằm ngay đầu URL nên nó phải là một đoạn đường dẫn hợp lệ và
// không cần mã hóa: dấu gạch chéo, dấu chấm hỏi hay khoảng trắng lọt vào đây
// sẽ tạo ra một địa chỉ mà chính người dùng không gõ lại được.
var entryPathPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{4,64}$`)

// entryPathLength là độ dài đường dẫn bí mật panel tự sinh.
const entryPathLength = 12

// Settings là phần cấu hình sửa được từ giao diện.
//
// Cố ý không phải toàn bộ tệp cấu hình: khóa ký JWT, đường dẫn cơ sở dữ liệu và
// các thư mục dữ liệu đổi được từ web là một cách rất nhanh để tự khóa mình ra
// ngoài mà không có đường quay lại. Những thứ đó vẫn sửa được bằng tệp cấu hình.
type Settings struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	EntryPath string `json:"entryPath"`

	TLSEnabled  bool   `json:"tlsEnabled"`
	TLSCertFile string `json:"tlsCertFile"`
	TLSKeyFile  string `json:"tlsKeyFile"`

	// Các khoảng thời gian ở dạng chuỗi kiểu "15m", "720h".
	AccessTokenTTL   string `json:"accessTokenTtl"`
	RefreshTokenTTL  string `json:"refreshTokenTtl"`
	MaxLoginAttempts int    `json:"maxLoginAttempts"`
	LockoutDuration  string `json:"lockoutDuration"`

	AllowedIPs     []string `json:"allowedIps"`
	TrustedProxies []string `json:"trustedProxies"`

	MonitorInterval  string `json:"monitorInterval"`
	MonitorRetention string `json:"monitorRetention"`

	LogLevel string `json:"logLevel"`
}

// SettingsInfo là cấu hình hiện tại kèm các thông tin chỉ đọc.
type SettingsInfo struct {
	Settings
	// DataDir, FileRoot và ConfigPath chỉ để hiển thị, không sửa được từ web.
	DataDir    string `json:"dataDir"`
	FileRoot   string `json:"fileRoot"`
	ConfigPath string `json:"configPath"`
	// RestartSupported cho biết panel tự khởi động lại được trên nền tảng này.
	RestartSupported bool `json:"restartSupported"`
	// PendingRestart là đã có thay đổi được lưu nhưng chưa có hiệu lực.
	PendingRestart bool `json:"pendingRestart"`
}

// SettingsUpdate là kết quả một lần lưu cấu hình.
type SettingsUpdate struct {
	SettingsInfo
	// URL là địa chỉ panel sau khi cấu hình mới có hiệu lực.
	//
	// Đổi cổng hoặc đường dẫn bí mật xong mà không nói địa chỉ mới thì người dùng
	// vừa tự làm mất đường vào panel của chính mình.
	URL string `json:"url"`
}

// Restarter khởi động lại panel để cấu hình mới có hiệu lực.
type Restarter interface {
	// Restart yêu cầu panel dừng và chạy lại.
	Restart() error
	// RestartSupported cho biết nền tảng hiện tại làm được việc đó.
	RestartSupported() bool
}

// SettingsService đọc và ghi cấu hình của chính panel.
type SettingsService struct {
	mu      sync.RWMutex
	cfg     config.Config
	path    string
	pending bool

	restarter Restarter
	audit     *AuditService
}

// NewSettingsService tạo dịch vụ cấu hình panel.
func NewSettingsService(cfg config.Config, path string, restarter Restarter, audit *AuditService) *SettingsService {
	return &SettingsService{cfg: cfg, path: path, restarter: restarter, audit: audit}
}

// Get trả về cấu hình đang lưu trên đĩa.
func (s *SettingsService) Get() SettingsInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.info()
}

func (s *SettingsService) info() SettingsInfo {
	cfg := s.cfg
	return SettingsInfo{
		Settings: Settings{
			Host:             cfg.Server.Host,
			Port:             cfg.Server.Port,
			EntryPath:        cfg.Server.EntryPath,
			TLSEnabled:       cfg.Server.TLS.Enabled,
			TLSCertFile:      cfg.Server.TLS.CertFile,
			TLSKeyFile:       cfg.Server.TLS.KeyFile,
			AccessTokenTTL:   cfg.Security.AccessTokenTTL.String(),
			RefreshTokenTTL:  cfg.Security.RefreshTokenTTL.String(),
			MaxLoginAttempts: cfg.Security.MaxLoginAttempts,
			LockoutDuration:  cfg.Security.LockoutDuration.String(),
			AllowedIPs:       append([]string{}, cfg.Security.AllowedIPs...),
			TrustedProxies:   append([]string{}, cfg.Security.TrustedProxies...),
			MonitorInterval:  cfg.Monitor.Interval.String(),
			MonitorRetention: cfg.Monitor.Retention.String(),
			LogLevel:         cfg.Log.Level,
		},
		DataDir:          cfg.Server.DataDir,
		FileRoot:         cfg.Server.FileRoot,
		ConfigPath:       s.path,
		RestartSupported: s.restarter != nil && s.restarter.RestartSupported(),
		PendingRestart:   s.pending,
	}
}

// GenerateEntryPath sinh một đường dẫn bí mật mới nhưng chưa lưu.
//
// Trả về cho giao diện điền vào ô thay vì lưu thẳng: người dùng phải nhìn thấy
// địa chỉ mới trước khi nó có hiệu lực, nếu không họ mất đường vào panel.
func (s *SettingsService) GenerateEntryPath() (string, error) {
	entry, err := crypto.RandomString(entryPathLength)
	if err != nil {
		return "", apperr.Internal.Wrap(err)
	}
	return entry, nil
}

// Update kiểm tra, lưu cấu hình mới và cho biết địa chỉ panel sau khi đổi.
func (s *SettingsService) Update(ctx context.Context, req Settings, actor AuditEntry) (SettingsUpdate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	updated, err := s.apply(s.cfg, req)
	if err == nil {
		err = updated.Validate()
		if err != nil {
			err = apperr.SettingsInvalid.WithParam("message", err.Error())
		}
	}
	if err == nil {
		err = s.checkPortFree(updated)
	}
	if err == nil {
		if saveErr := updated.Save(s.path); saveErr != nil {
			err = apperr.SettingsSaveFailed.Wrap(saveErr)
		}
	}

	actor.Action = "settings.update"
	actor.Resource = s.path
	actor.Success = err == nil
	s.audit.Record(ctx, actor)

	if err != nil {
		return SettingsUpdate{}, err
	}

	// Cấu hình mới chỉ nằm trên đĩa: mọi thứ sửa được ở đây đều được đọc đúng một
	// lần lúc khởi động — cổng lắng nghe, đường dẫn bí mật, danh sách IP đều nằm
	// trong lớp middleware dựng sẵn — nên phải khởi động lại mới có hiệu lực.
	s.pending = s.pending || differs(s.cfg, updated)
	s.cfg = updated

	return SettingsUpdate{SettingsInfo: s.info(), URL: panelURL(updated)}, nil
}

// apply chép các trường sửa được từ yêu cầu sang bản cấu hình.
func (s *SettingsService) apply(cfg config.Config, req Settings) (config.Config, error) {
	entry := strings.Trim(strings.TrimSpace(req.EntryPath), "/")
	if !entryPathPattern.MatchString(entry) {
		return cfg, apperr.SettingsInvalidEntryPath
	}

	host := strings.TrimSpace(req.Host)
	if host == "" {
		host = "0.0.0.0"
	}
	if net.ParseIP(host) == nil {
		return cfg, apperr.SettingsInvalid.WithParam("message", "địa chỉ lắng nghe không hợp lệ")
	}

	allowed, err := cleanIPList(req.AllowedIPs)
	if err != nil {
		return cfg, err
	}
	proxies, err := cleanIPList(req.TrustedProxies)
	if err != nil {
		return cfg, err
	}

	// Danh sách chứ không phải map khóa theo chuỗi: hai trường hoàn toàn có thể
	// mang cùng một giá trị ("15m"), và map sẽ nuốt mất một trong hai.
	durations := []struct {
		raw    string
		target *time.Duration
	}{
		{req.AccessTokenTTL, &cfg.Security.AccessTokenTTL},
		{req.RefreshTokenTTL, &cfg.Security.RefreshTokenTTL},
		{req.LockoutDuration, &cfg.Security.LockoutDuration},
		{req.MonitorInterval, &cfg.Monitor.Interval},
		{req.MonitorRetention, &cfg.Monitor.Retention},
	}
	for _, item := range durations {
		value, err := time.ParseDuration(strings.TrimSpace(item.raw))
		if err != nil || value <= 0 {
			return cfg, apperr.SettingsInvalidDuration.WithParam("value", item.raw)
		}
		*item.target = value
	}

	if req.MaxLoginAttempts < 1 {
		return cfg, apperr.SettingsInvalid.WithParam("message", "số lần đăng nhập sai tối đa phải từ 1 trở lên")
	}

	cfg.Server.Host = host
	cfg.Server.Port = req.Port
	cfg.Server.EntryPath = entry
	cfg.Server.TLS.Enabled = req.TLSEnabled
	cfg.Server.TLS.CertFile = strings.TrimSpace(req.TLSCertFile)
	cfg.Server.TLS.KeyFile = strings.TrimSpace(req.TLSKeyFile)
	cfg.Security.MaxLoginAttempts = req.MaxLoginAttempts
	cfg.Security.AllowedIPs = allowed
	cfg.Security.TrustedProxies = proxies
	cfg.Log.Level = strings.TrimSpace(req.LogLevel)
	return cfg, nil
}

// checkPortFree thử chiếm cổng mới trước khi lưu.
//
// Lưu một cổng đang bị tiến trình khác giữ nghĩa là panel khởi động lại rồi
// không lên nữa, và người dùng chỉ biết điều đó khi đã mất đường vào.
func (s *SettingsService) checkPortFree(updated config.Config) error {
	if updated.Server.Port == s.cfg.Server.Port && updated.Server.Host == s.cfg.Server.Host {
		return nil
	}

	address := net.JoinHostPort(updated.Server.Host, fmt.Sprint(updated.Server.Port))
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return apperr.SettingsPortInUse.WithParam("port", updated.Server.Port)
	}
	return listener.Close()
}

// Restart yêu cầu panel chạy lại để cấu hình mới có hiệu lực.
func (s *SettingsService) Restart(ctx context.Context, actor AuditEntry) error {
	if s.restarter == nil || !s.restarter.RestartSupported() {
		return apperr.SettingsRestartUnsupported
	}

	actor.Action = "settings.restart"
	actor.Resource = "panel"
	actor.Success = true
	s.audit.Record(ctx, actor)

	if err := s.restarter.Restart(); err != nil {
		return apperr.Internal.Wrap(err)
	}

	s.mu.Lock()
	s.pending = false
	s.mu.Unlock()
	return nil
}

// URL là địa chỉ panel theo cấu hình đang lưu.
func (s *SettingsService) URL() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return panelURL(s.cfg)
}

func panelURL(cfg config.Config) string {
	scheme := "http"
	if cfg.Server.TLS.Enabled {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s:%d/%s/", scheme, cfg.Server.Host, cfg.Server.Port, cfg.Server.EntryPath)
}

// differs cho biết hai bản cấu hình có khác nhau ở phần cần khởi động lại không.
func differs(before, after config.Config) bool {
	return before.Server.Host != after.Server.Host ||
		before.Server.Port != after.Server.Port ||
		before.Server.EntryPath != after.Server.EntryPath ||
		before.Server.TLS != after.Server.TLS ||
		before.Security.AccessTokenTTL != after.Security.AccessTokenTTL ||
		before.Security.RefreshTokenTTL != after.Security.RefreshTokenTTL ||
		before.Security.MaxLoginAttempts != after.Security.MaxLoginAttempts ||
		before.Security.LockoutDuration != after.Security.LockoutDuration ||
		!equalStrings(before.Security.AllowedIPs, after.Security.AllowedIPs) ||
		!equalStrings(before.Security.TrustedProxies, after.Security.TrustedProxies) ||
		before.Monitor != after.Monitor ||
		before.Log.Level != after.Log.Level
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// cleanIPList kiểm tra từng dòng là một địa chỉ IP hoặc dải CIDR.
//
// Một dòng sai chính tả trong danh sách IP được phép sẽ khóa đúng người quản
// trị ra ngoài, nên phải bắt ngay lúc lưu chứ không phải lúc khởi động lại.
func cleanIPList(values []string) ([]string, error) {
	out := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}

		if _, err := netip.ParsePrefix(value); err == nil {
			out = append(out, value)
			continue
		}
		if _, err := netip.ParseAddr(value); err != nil {
			return nil, apperr.SettingsInvalidIP.WithParam("value", value)
		}
		out = append(out, value)
	}
	return out, nil
}
