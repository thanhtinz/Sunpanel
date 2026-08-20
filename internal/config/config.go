// Package config nạp và kiểm tra cấu hình của SunPanel.
//
// Thứ tự ưu tiên: giá trị mặc định < tệp YAML < biến môi trường (tiền tố SUNPANEL_).
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// EnvPrefix là tiền tố của mọi biến môi trường ghi đè cấu hình.
const EnvPrefix = "SUNPANEL_"

// Config là toàn bộ cấu hình của panel.
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Security SecurityConfig `yaml:"security"`
	Monitor  MonitorConfig  `yaml:"monitor"`
	Website  WebsiteConfig  `yaml:"website"`
	AppStore AppStoreConfig `yaml:"appStore"`
	Backup   BackupConfig   `yaml:"backup"`
	Plugin   PluginConfig   `yaml:"plugin"`
	Log      LogConfig      `yaml:"log"`
}

// ServerConfig là cấu hình máy chủ HTTP.
type ServerConfig struct {
	// Host là địa chỉ lắng nghe.
	Host string `yaml:"host"`
	// Port là cổng lắng nghe.
	Port int `yaml:"port"`
	// EntryPath là đường dẫn bí mật để vào panel; mọi đường dẫn khác trả về 404.
	EntryPath string `yaml:"entryPath"`
	// DataDir là thư mục chứa dữ liệu của panel.
	DataDir string `yaml:"dataDir"`
	// FileRoot giới hạn phạm vi mà trình quản lý tệp được phép truy cập.
	FileRoot string `yaml:"fileRoot"`
	// TLS bật HTTPS.
	TLS TLSConfig `yaml:"tls"`
	// ReadTimeout, WriteTimeout, ShutdownTimeout là các giới hạn thời gian.
	ReadTimeout     time.Duration `yaml:"readTimeout"`
	WriteTimeout    time.Duration `yaml:"writeTimeout"`
	ShutdownTimeout time.Duration `yaml:"shutdownTimeout"`
}

// TLSConfig là cấu hình chứng chỉ HTTPS.
type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"certFile"`
	KeyFile  string `yaml:"keyFile"`
}

// DatabaseConfig là cấu hình cơ sở dữ liệu nội bộ của panel.
type DatabaseConfig struct {
	// Path là đường dẫn tệp SQLite.
	Path string `yaml:"path"`
}

// SecurityConfig gom các tham số bảo mật.
type SecurityConfig struct {
	// JWTSecret là khóa ký token; để trống sẽ tự sinh lúc khởi tạo.
	JWTSecret string `yaml:"jwtSecret"`
	// AccessTokenTTL là thời gian sống của access token.
	AccessTokenTTL time.Duration `yaml:"accessTokenTTL"`
	// RefreshTokenTTL là thời gian sống của refresh token.
	RefreshTokenTTL time.Duration `yaml:"refreshTokenTTL"`
	// MaxLoginAttempts là số lần đăng nhập sai trước khi khóa tài khoản.
	MaxLoginAttempts int `yaml:"maxLoginAttempts"`
	// LockoutDuration là thời gian khóa sau khi vượt quá số lần thử.
	LockoutDuration time.Duration `yaml:"lockoutDuration"`
	// BlockThreshold là số lần đăng nhập sai từ cùng một địa chỉ trước khi địa
	// chỉ đó bị chặn tạm thời; 0 nghĩa là tắt lớp này.
	//
	// Tách khỏi MaxLoginAttempts vì hai lớp bảo vệ hai thứ khác nhau: khóa tài
	// khoản chặn người dò một tài khoản, còn chặn địa chỉ chặn người rải thử
	// nhiều tên đăng nhập khác nhau.
	BlockThreshold int `yaml:"blockThreshold"`
	// BlockWindow là khoảng thời gian các lần sai được cộng dồn.
	BlockWindow time.Duration `yaml:"blockWindow"`
	// BlockDuration là thời gian chặn mỗi lần vượt ngưỡng.
	BlockDuration time.Duration `yaml:"blockDuration"`
	// AllowedIPs giới hạn IP được phép truy cập panel; rỗng nghĩa là không giới hạn.
	AllowedIPs []string `yaml:"allowedIPs"`
	// TrustedProxies là các proxy được tin để đọc header X-Forwarded-For.
	TrustedProxies []string `yaml:"trustedProxies"`
}

// MonitorConfig là cấu hình thu thập số liệu giám sát.
type MonitorConfig struct {
	// Interval là chu kỳ lấy mẫu.
	Interval time.Duration `yaml:"interval"`
	// Retention là thời gian giữ lịch sử số liệu.
	Retention time.Duration `yaml:"retention"`
}

// WebsiteConfig là cấu hình quản lý website.
type WebsiteConfig struct {
	// NginxConfDir là thư mục panel ghi tệp vhost vào.
	//
	// Tách riêng khỏi thư mục cấu hình chung của nginx: panel chỉ đụng vào thư
	// mục của mình, nên cấu hình do quản trị viên tự viết không bao giờ bị ghi đè.
	NginxConfDir string `yaml:"nginxConfDir"`
	// Root là thư mục mặc định chứa mã nguồn các website.
	Root string `yaml:"root"`
	// LogDir là thư mục máy chủ web ghi nhật ký của từng website vào.
	//
	// Panel vừa sinh chỉ thị access_log trỏ vào đây, vừa đọc lại chính các tệp
	// đó để dựng trang thống kê truy cập — nên chỉ có một giá trị duy nhất.
	LogDir string `yaml:"logDir"`
	// AuthDir là nơi panel ghi tệp tài khoản của lớp bảo vệ bằng mật khẩu.
	//
	// Cùng lý do với ACMEWebroot: phải nằm NGOÀI thư mục dữ liệu của panel, vì
	// thư mục đó để quyền 0700 còn tiến trình máy chủ web chạy dưới người dùng
	// khác — đặt tệp vào trong đó thì mọi yêu cầu tới website được bảo vệ sẽ
	// nhận lỗi 500 thay vì hộp hỏi mật khẩu.
	AuthDir string `yaml:"authDir"`
	// ACMEWebroot là thư mục dùng chung phục vụ tệp xác thực ACME.
	//
	// Phải nằm NGOÀI thư mục dữ liệu của panel: thư mục đó để quyền 0700 vì chứa
	// khóa chủ và cơ sở dữ liệu, nên tiến trình máy chủ web (thường chạy dưới
	// www-data hoặc nginx) không đi vào được, và mọi lần xác thực tên miền sẽ
	// nhận 403 thay vì tệp cần đọc.
	ACMEWebroot string `yaml:"acmeWebroot"`
}

// AppStoreConfig là cấu hình chợ ứng dụng.
type AppStoreConfig struct {
	// Root là thư mục chứa tệp compose và dữ liệu của các ứng dụng đã cài.
	Root string `yaml:"root"`
	// CatalogDir là thư mục chứa định nghĩa ứng dụng do quản trị viên tự thêm.
	//
	// Định nghĩa ở đây ưu tiên hơn danh mục nhúng sẵn, nên sửa một ứng dụng không
	// phải chờ bản cập nhật panel.
	CatalogDir string `yaml:"catalogDir"`
}

// BackupConfig là cấu hình sao lưu.
type BackupConfig struct {
	// Root là thư mục mặc định lưu bản sao lưu tại chỗ.
	Root string `yaml:"root"`
	// WorkDir là nơi ghi tệp tạm trong lúc dựng bản sao lưu.
	//
	// Tách khỏi Root vì tệp tạm cần dung lượng bằng cả bản sao lưu, và nơi lưu
	// trữ thật có thể nằm trên mạng.
	WorkDir string `yaml:"workDir"`
}

// PluginConfig là cấu hình hệ thống plugin.
type PluginConfig struct {
	// Dir là thư mục chứa tệp khai báo plugin.
	Dir string `yaml:"dir"`
}

// LogConfig là cấu hình ghi log.
type LogConfig struct {
	// Level nhận một trong: debug, info, warn, error.
	Level string `yaml:"level"`
	// Format nhận "text" hoặc "json".
	Format string `yaml:"format"`
	// File là nơi ghi log; để trống thì ghi ra stdout.
	File string `yaml:"file"`
	// SystemDir là thư mục nhật ký của hệ điều hành mà panel cho phép xem.
	//
	// Trình xem nhật ký chỉ mở tệp bên trong thư mục này. Để nó ở cấu hình chứ
	// không viết cứng vì máy chủ nào cũng có thể gắn nhật ký sang phân vùng khác.
	SystemDir string `yaml:"systemDir"`
}

// Default trả về cấu hình mặc định khi chưa có tệp cấu hình.
func Default() Config {
	dataDir := defaultDataDir()
	return Config{
		Server: ServerConfig{
			Host:            "0.0.0.0",
			Port:            9527,
			DataDir:         dataDir,
			FileRoot:        defaultFileRoot(),
			ReadTimeout:     30 * time.Second,
			WriteTimeout:    60 * time.Second,
			ShutdownTimeout: 15 * time.Second,
		},
		Database: DatabaseConfig{
			Path: filepath.Join(dataDir, "db", "sunpanel.db"),
		},
		Security: SecurityConfig{
			AccessTokenTTL:   15 * time.Minute,
			RefreshTokenTTL:  7 * 24 * time.Hour,
			MaxLoginAttempts: 5,
			LockoutDuration:  15 * time.Minute,
			BlockThreshold:   10,
			BlockWindow:      10 * time.Minute,
			BlockDuration:    30 * time.Minute,
		},
		Monitor: MonitorConfig{
			Interval:  5 * time.Second,
			Retention: 7 * 24 * time.Hour,
		},
		AppStore: AppStoreConfig{
			Root:       filepath.Join(dataDir, "apps"),
			CatalogDir: filepath.Join(dataDir, "catalog"),
		},
		Backup: BackupConfig{
			Root:    filepath.Join(dataDir, "backups"),
			WorkDir: filepath.Join(dataDir, "tmp"),
		},
		Plugin: PluginConfig{
			Dir: filepath.Join(dataDir, "plugins"),
		},
		Website: WebsiteConfig{
			NginxConfDir: defaultNginxConfDir(),
			Root:         defaultWebRoot(),
			ACMEWebroot:  defaultACMEWebroot(),
			AuthDir:      defaultAuthDir(),
			LogDir:       defaultWebLogDir(),
		},
		Log: LogConfig{
			Level:     "info",
			Format:    "text",
			SystemDir: defaultSystemLogDir(),
		},
	}
}

// Load nạp cấu hình từ tệp (nếu có) rồi áp các biến môi trường đè lên.
// Tệp không tồn tại không phải là lỗi: panel chạy được với cấu hình mặc định.
func Load(path string) (Config, error) {
	cfg := Default()

	if path != "" {
		data, err := os.ReadFile(path)
		switch {
		case err == nil:
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				return cfg, fmt.Errorf("phân tích tệp cấu hình %s: %w", path, err)
			}
		case !os.IsNotExist(err):
			return cfg, fmt.Errorf("đọc tệp cấu hình %s: %w", path, err)
		}
	}

	applyEnv(&cfg)

	if err := cfg.Validate(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// Validate kiểm tra tính hợp lệ của cấu hình.
func (c *Config) Validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("cổng %d không hợp lệ, phải nằm trong khoảng 1-65535", c.Server.Port)
	}
	if c.Server.DataDir == "" {
		return fmt.Errorf("thư mục dữ liệu không được để trống")
	}
	if c.Server.TLS.Enabled && (c.Server.TLS.CertFile == "" || c.Server.TLS.KeyFile == "") {
		return fmt.Errorf("bật TLS thì phải khai báo cả certFile lẫn keyFile")
	}
	switch c.Log.Level {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("mức log %q không hợp lệ, chọn debug/info/warn/error", c.Log.Level)
	}
	if c.Security.BlockThreshold > 0 && (c.Security.BlockWindow <= 0 || c.Security.BlockDuration <= 0) {
		return fmt.Errorf("bật chặn IP thì blockWindow và blockDuration phải lớn hơn 0")
	}
	if c.Monitor.Interval < time.Second {
		return fmt.Errorf("chu kỳ giám sát phải từ 1 giây trở lên")
	}
	return nil
}

// ConfigPath là đường dẫn tệp cấu hình mặc định bên trong thư mục dữ liệu.
func (c *Config) ConfigPath() string {
	return filepath.Join(c.Server.DataDir, "config.yaml")
}

// MasterKeyPath là nơi lưu khóa chủ dùng mã hóa bí mật trong DB.
func (c *Config) MasterKeyPath() string {
	return filepath.Join(c.Server.DataDir, "master.key")
}

// CertDir là thư mục lưu chứng chỉ TLS của các website.
func (c *Config) CertDir() string {
	return filepath.Join(c.Server.DataDir, "certs")
}

// ACMEAccountKeyPath là nơi lưu khóa tài khoản ACME.
//
// Khóa này phải sống qua mọi lần khởi động: mất nó là mất luôn lịch sử tài khoản
// và các giới hạn tần suất đã tích lũy với Let's Encrypt.
func (c *Config) ACMEAccountKeyPath() string {
	return filepath.Join(c.Server.DataDir, "acme", "account.key")
}

// Save ghi cấu hình xuống đĩa với quyền chỉ chủ sở hữu đọc được,
// vì tệp có chứa khóa ký JWT.
func (c *Config) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("tạo thư mục cấu hình: %w", err)
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("chuyển cấu hình sang YAML: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("ghi tệp cấu hình: %w", err)
	}
	return nil
}

func applyEnv(cfg *Config) {
	envStr(&cfg.Server.Host, "HOST")
	envInt(&cfg.Server.Port, "PORT")
	envStr(&cfg.Server.EntryPath, "ENTRY_PATH")
	envStr(&cfg.Server.DataDir, "DATA_DIR")
	envStr(&cfg.Server.FileRoot, "FILE_ROOT")
	envStr(&cfg.Website.NginxConfDir, "NGINX_CONF_DIR")
	envStr(&cfg.Website.Root, "WEB_ROOT")
	envStr(&cfg.Website.ACMEWebroot, "ACME_WEBROOT")
	envStr(&cfg.AppStore.Root, "APP_ROOT")
	envStr(&cfg.AppStore.CatalogDir, "APP_CATALOG_DIR")
	envStr(&cfg.Backup.Root, "BACKUP_ROOT")
	envStr(&cfg.Backup.WorkDir, "BACKUP_WORK_DIR")
	envStr(&cfg.Plugin.Dir, "PLUGIN_DIR")
	envBool(&cfg.Server.TLS.Enabled, "TLS_ENABLED")
	envStr(&cfg.Server.TLS.CertFile, "TLS_CERT_FILE")
	envStr(&cfg.Server.TLS.KeyFile, "TLS_KEY_FILE")
	envStr(&cfg.Database.Path, "DB_PATH")
	envStr(&cfg.Security.JWTSecret, "JWT_SECRET")
	envDuration(&cfg.Security.AccessTokenTTL, "ACCESS_TOKEN_TTL")
	envDuration(&cfg.Security.RefreshTokenTTL, "REFRESH_TOKEN_TTL")
	envInt(&cfg.Security.MaxLoginAttempts, "MAX_LOGIN_ATTEMPTS")
	envDuration(&cfg.Monitor.Interval, "MONITOR_INTERVAL")
	envStr(&cfg.Log.Level, "LOG_LEVEL")
	envStr(&cfg.Log.Format, "LOG_FORMAT")
	envStr(&cfg.Log.File, "LOG_FILE")
	envStr(&cfg.Log.SystemDir, "LOG_SYSTEM_DIR")

	// Đường dẫn DB mặc định nằm trong thư mục dữ liệu, nên nếu người dùng chỉ đổi
	// thư mục dữ liệu thì đường dẫn DB phải đi theo.
	if os.Getenv(EnvPrefix+"DB_PATH") == "" && cfg.Database.Path == Default().Database.Path {
		cfg.Database.Path = filepath.Join(cfg.Server.DataDir, "db", "sunpanel.db")
	}
}

func envStr(dst *string, key string) {
	if v, ok := os.LookupEnv(EnvPrefix + key); ok {
		*dst = v
	}
}

func envInt(dst *int, key string) {
	if v, ok := os.LookupEnv(EnvPrefix + key); ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			*dst = n
		}
	}
}

func envBool(dst *bool, key string) {
	if v, ok := os.LookupEnv(EnvPrefix + key); ok {
		if b, err := strconv.ParseBool(strings.TrimSpace(v)); err == nil {
			*dst = b
		}
	}
}

func envDuration(dst *time.Duration, key string) {
	if v, ok := os.LookupEnv(EnvPrefix + key); ok {
		if d, err := time.ParseDuration(strings.TrimSpace(v)); err == nil {
			*dst = d
		}
	}
}
