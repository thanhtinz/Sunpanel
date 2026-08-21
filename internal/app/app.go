package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"gorm.io/gorm"

	"github.com/thanhtinz/sunpanel/internal/config"
	"github.com/thanhtinz/sunpanel/internal/database"
	"github.com/thanhtinz/sunpanel/internal/router"
	"github.com/thanhtinz/sunpanel/internal/service"
	"github.com/thanhtinz/sunpanel/pkg/accesslog"
	"github.com/thanhtinz/sunpanel/pkg/appstore"
	"github.com/thanhtinz/sunpanel/pkg/certs"
	"github.com/thanhtinz/sunpanel/pkg/compose"
	"github.com/thanhtinz/sunpanel/pkg/crypto"
	"github.com/thanhtinz/sunpanel/pkg/diskscan"
	"github.com/thanhtinz/sunpanel/pkg/dockerx"
	"github.com/thanhtinz/sunpanel/pkg/firewall"
	"github.com/thanhtinz/sunpanel/pkg/host"
	"github.com/thanhtinz/sunpanel/pkg/logs"
	"github.com/thanhtinz/sunpanel/pkg/plugin"
	"github.com/thanhtinz/sunpanel/pkg/sysservice"
	"github.com/thanhtinz/sunpanel/pkg/sysuser"
	"github.com/thanhtinz/sunpanel/pkg/webserver"
)

// sessionCleanupInterval là chu kỳ dọn các phiên đăng nhập đã hết hạn.
const sessionCleanupInterval = 6 * time.Hour

// allowedCommands là danh sách trắng các chương trình mà panel được phép chạy.
//
// Đây là hàng rào cuối cùng: kể cả khi một lỗi ở tầng trên cho phép kẻ tấn công
// điều khiển tên chương trình, họ cũng chỉ chạy được đúng những gì trong danh
// sách này. Terminal không đi qua đây — nó mở PTY riêng và được chặn bằng phân
// quyền, vì bản chất terminal là chạy lệnh tùy ý.
var allowedCommands = []string{
	"systemctl",  // quản lý dịch vụ
	"journalctl", // đọc nhật ký dịch vụ
	"ufw",        // tường lửa trên Debian/Ubuntu
	"firewall-cmd",
	"nft",
	"iptables",
	"ip6tables",
}

// App là một thể hiện của panel đã được lắp ráp đầy đủ.
type App struct {
	cfg     config.Config
	db      *gorm.DB
	server  *http.Server
	svc     router.Services
	restart *restartSignal
}

// New lắp ráp toàn bộ ứng dụng từ cấu hình.
func New(cfg config.Config) (*App, error) {
	if err := os.MkdirAll(cfg.Server.DataDir, 0o700); err != nil {
		return nil, fmt.Errorf("tạo thư mục dữ liệu: %w", err)
	}

	db, err := database.Open(cfg.Database.Path, cfg.Log.Level == "debug")
	if err != nil {
		return nil, err
	}

	masterKey, err := LoadMasterKey(cfg.MasterKeyPath())
	if err != nil {
		return nil, err
	}
	sealer, err := crypto.NewSealer(masterKey)
	if err != nil {
		return nil, fmt.Errorf("khởi tạo bộ mã hóa bí mật: %w", err)
	}

	localHost := host.NewLocalHost(cfg.Server.FileRoot, allowedCommands)

	tokens := service.NewTokenIssuer(cfg.Security.JWTSecret, cfg.Security.AccessTokenTTL)
	audit := service.NewAuditService(db)
	auth := service.NewAuthService(db, tokens, sealer, cfg.Security, audit)
	users := service.NewUserService(db, auth, audit)
	monitor := service.NewMonitorService(db, localHost, cfg.Monitor)
	files := service.NewFileService(localHost, audit)
	terminal := service.NewTerminalService(localHost, cfg.Server.FileRoot, audit)
	sysServices := service.NewSystemServiceManager(sysservice.NewSystemd(localHost), audit)

	// Tác vụ định kỳ chạy lệnh do chính quản trị viên soạn, cùng mức tin cậy với
	// terminal, nên chúng cần một host KHÔNG bị giới hạn bởi allowlist. Tách
	// thành host riêng thay vì nới allowlist chung của panel.
	cronHost := host.NewLocalHost(cfg.Server.FileRoot, nil)
	cronJobs := service.NewCronService(db, cronHost, cfg.Server.FileRoot, audit)

	// Chứng chỉ và website: kho chứng chỉ nằm trong thư mục dữ liệu của panel,
	// còn tệp vhost ghi vào thư mục nginx đọc.
	certStore := certs.NewStore(cfg.CertDir())
	solver := certs.NewWebrootSolver(cfg.Website.ACMEWebroot)
	certificates := service.NewCertificateService(
		db, certStore, solver, cfg.ACMEAccountKeyPath(), audit,
	)

	// Thư mục gốc của website do quản trị viên chọn và thường nằm ngoài phạm vi
	// trình quản lý tệp, nên dịch vụ website dùng host riêng không giới hạn.
	websiteHost := host.NewLocalHost("/", nil)
	nginxHost := host.NewLocalHost(cfg.Server.FileRoot, []string{"nginx"})
	websites := service.NewWebsiteService(
		db, webserver.NewNginx(nginxHost, cfg.Website.NginxConfDir), certificates,
		websiteHost, cfg.Website.ACMEWebroot, cfg.Website.AuthDir, audit,
	)
	// Nhật ký truy cập nằm ngoài phạm vi trình quản lý tệp trên nhiều máy, và
	// việc đọc nó không cần chạy lệnh nào.
	websites.SetAccessLogs(
		accesslog.New(host.NewLocalHost("/", nil).FS()), cfg.Website.LogDir,
	)

	// Chứng chỉ mới chỉ có tác dụng sau khi máy chủ web đọc lại tệp.
	certificates.SetReloader(websites.Reload)

	// Plugin nạp lúc khởi động; tệp khai báo hỏng phải lộ ra ngay chứ không phải
	// lúc người dùng bấm vào menu.
	pluginRegistry, err := plugin.Load(cfg.Plugin.Dir)
	if err != nil {
		return nil, fmt.Errorf("nạp plugin: %w", err)
	}
	plugins := service.NewPluginService(pluginRegistry, audit)

	alerts := service.NewAlertService(db, sealer, monitor, audit)
	apiKeys := service.NewAPIKeyService(db, audit)
	nodes := service.NewNodeService(db, sealer, audit)

	databases := service.NewDatabaseService(db, sealer, audit)
	backups := service.NewBackupService(
		db, databases, sealer, cfg.Backup.WorkDir, cfg.Backup.Root, audit,
	)

	// Sao lưu hỏng và gia hạn chứng chỉ hỏng là hai sự cố im lặng nguy hiểm nhất:
	// người dùng chỉ phát hiện khi cần khôi phục, hoặc khi website mất HTTPS.
	backups.SetAlerts(alerts)
	certificates.SetAlerts(alerts)

	// Dò công cụ tường lửa một lần lúc khởi động: việc dò phải chạy vài lệnh, và
	// công cụ trên máy không đổi giữa chừng.
	firewallManager := firewall.Detect(context.Background(), localHost)
	firewallSvc := service.NewFirewallService(firewallManager, cfg.Server.Port, audit)

	// Tên container của chính panel, để nó từ chối tự dừng mình.
	dockerClient := dockerx.NewClient()
	dockerSvc := service.NewDockerService(dockerClient, os.Getenv("SUNPANEL_CONTAINER_NAME"), audit)

	// Danh mục ứng dụng: bản nhúng sẵn trước, rồi tới định nghĩa tự thêm của
	// quản trị viên — trùng định danh thì bản tự thêm thắng.
	catalog, err := appstore.LoadBuiltin()
	if err != nil {
		return nil, fmt.Errorf("nạp danh mục ứng dụng: %w", err)
	}
	custom, err := appstore.LoadDir(cfg.AppStore.CatalogDir)
	if err != nil {
		return nil, fmt.Errorf("nạp danh mục ứng dụng tự thêm: %w", err)
	}
	for _, problem := range custom.Problems() {
		slog.Warn("bỏ qua một tệp danh mục ứng dụng tự thêm", "chi_tiết", problem)
	}
	catalog.Merge(custom)

	// Ứng dụng được cài bằng compose, vốn chạy container tùy ý — cùng mức tin cậy
	// với terminal, nên dùng host riêng thay vì nới allowlist chung.
	appHost := host.NewLocalHost("/", []string{"docker", "docker-compose"})
	apps := service.NewAppStoreService(
		db, catalog, compose.New(appHost), dockerClient, appHost, sealer,
		cfg.AppStore.Root, audit,
	)

	// Tài khoản hệ điều hành nằm ngoài phạm vi trình quản lý tệp, và các lệnh
	// dưới đây không có ở host chung, nên phần này dùng host riêng của mình.
	userHost := host.NewLocalHost("/", []string{
		"useradd", "usermod", "userdel", "gpasswd", "chpasswd", "chown",
	})
	sysUsers := service.NewSystemUserService(sysuser.New(userHost), audit)

	uptimeMonitors := service.NewUptimeService(db, alerts, audit)

	// Bộ quét dung lượng dùng chung phạm vi với trình quản lý tệp: thứ người dùng
	// nhìn thấy ở đây cũng là thứ họ vào xóa được ở trang Tệp.
	diskService := service.NewDiskService(diskscan.New(localHost.FS()), monitor)

	// Nhật ký nằm ngoài phạm vi trình quản lý tệp trên nhiều máy, và bộ đọc tự
	// giới hạn mình trong /var/log nên host này không cần chạy lệnh nào.
	logService := service.NewLogService(logs.New(host.NewLocalHost("/", nil), cfg.Log.SystemDir))

	// Tín hiệu khởi động lại được dựng trước các dịch vụ vì trang cài đặt cầm nó:
	// đổi cổng hay đường dẫn bí mật xong thì phải có đường yêu cầu panel lên lại.
	restart := newRestartSignal()

	svc := router.Services{
		Auth:      auth,
		Users:     users,
		Audit:     audit,
		Monitor:   monitor,
		Files:     files,
		Terminal:  terminal,
		Services:  sysServices,
		Processes: service.NewProcessService(audit),
		Cron:      cronJobs,
		Apps:      apps,
		Databases: databases,
		Backups:   backups,
		Alerts:    alerts,
		APIKeys:   apiKeys,
		Nodes:     nodes,
		Plugins:   plugins,
		Websites:  websites,
		Certs:     certificates,
		Firewall:  firewallSvc,
		Docker:    dockerSvc,
		Tokens:    tokens,
		Settings:  service.NewSettingsService(cfg, cfg.ConfigPath(), restart, audit),
		SysUsers:  sysUsers,
		Logs:      logService,
		Disks:     diskService,
		Uptime:    uptimeMonitors,
		Health: service.NewHealthService(
			db, cfg, monitor, certificates, backups, uptimeMonitors, firewallSvc,
		),
		Security: service.NewSecurityService(
			db, auth, cfg.Security.BlockThreshold,
			int(cfg.Security.BlockWindow.Seconds()),
			int(cfg.Security.BlockDuration.Seconds()), audit,
		),
	}

	handler, err := router.New(cfg, svc)
	if err != nil {
		return nil, fmt.Errorf("dựng bộ định tuyến: %w", err)
	}

	server := &http.Server{
		Addr:         net.JoinHostPort(cfg.Server.Host, strconv.Itoa(cfg.Server.Port)),
		Handler:      handler,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  2 * time.Minute,
	}

	return &App{cfg: cfg, db: db, server: server, svc: svc, restart: restart}, nil
}

// DB trả về kết nối cơ sở dữ liệu, dùng cho các lệnh phụ trợ trên dòng lệnh.
func (a *App) DB() *gorm.DB { return a.db }

// Close giải phóng tài nguyên của ứng dụng.
func (a *App) Close() error { return database.Close(a.db) }

// Run khởi động máy chủ và chạy tới khi nhận tín hiệu dừng.
//
// WriteTimeout của máy chủ HTTP phải được bỏ qua cho các tuyến WebSocket, nên
// luồng giám sát tự quản lý hạn ghi của riêng nó ở tầng handler.
func (a *App) Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go a.svc.Monitor.Run(ctx)

	if err := a.svc.Cron.Start(ctx); err != nil {
		return fmt.Errorf("khởi động bộ lập lịch: %w", err)
	}
	defer a.svc.Cron.Stop()

	if err := a.svc.Backups.Start(ctx); err != nil {
		return fmt.Errorf("khởi động bộ lập lịch sao lưu: %w", err)
	}
	defer a.svc.Backups.Stop()

	go a.svc.Certs.RunRenewal(ctx)
	go a.svc.Alerts.Run(ctx)
	go a.svc.Uptime.Run(ctx)
	go a.svc.Nodes.RunSampling(ctx)

	go a.runSessionCleanup(ctx)

	errCh := make(chan error, 1)
	go func() {
		var err error
		if a.cfg.Server.TLS.Enabled {
			err = a.server.ListenAndServeTLS(a.cfg.Server.TLS.CertFile, a.cfg.Server.TLS.KeyFile)
		} else {
			err = a.server.ListenAndServe()
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	slog.Info("SunPanel đã khởi động", "address", a.URL())

	restarting := false
	select {
	case err := <-errCh:
		return fmt.Errorf("máy chủ HTTP dừng bất thường: %w", err)
	case <-ctx.Done():
		slog.Info("nhận tín hiệu dừng, đang tắt máy chủ")
	case <-a.restart.ch:
		restarting = true
		slog.Info("đang tắt máy chủ để khởi động lại")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), a.cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := a.server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("tắt máy chủ: %w", err)
	}

	if restarting {
		slog.Info("SunPanel đã dừng, đang chạy lại")
		return ErrRestart
	}
	slog.Info("SunPanel đã dừng")
	return nil
}

// URL dựng địa chỉ truy cập panel để in ra cho người dùng.
func (a *App) URL() string {
	scheme := "http"
	if a.cfg.Server.TLS.Enabled {
		scheme = "https"
	}

	displayHost := a.cfg.Server.Host
	// 0.0.0.0 là địa chỉ lắng nghe, không phải địa chỉ gõ được vào trình duyệt.
	if displayHost == "0.0.0.0" || displayHost == "" || displayHost == "::" {
		displayHost = "<địa-chỉ-máy-chủ>"
	}

	url := fmt.Sprintf("%s://%s:%d", scheme, displayHost, a.cfg.Server.Port)
	if entry := a.cfg.Server.EntryPath; entry != "" {
		url += "/" + entry + "/"
	}
	return url
}

// runSessionCleanup dọn định kỳ các phiên đã hết hạn.
func (a *App) runSessionCleanup(ctx context.Context) {
	ticker := time.NewTicker(sessionCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			removed, err := a.svc.Auth.CleanupExpiredSessions(ctx)
			if err != nil {
				slog.Error("không dọn được phiên hết hạn", "error", err)
				continue
			}
			if removed > 0 {
				slog.Debug("đã dọn phiên hết hạn", "count", removed)
			}
			// Cùng nhịp dọn luôn bộ đếm đăng nhập sai: một đợt quét từ hàng vạn
			// địa chỉ khác nhau không được phép nằm lại trong bộ nhớ mãi.
			a.svc.Auth.Guard().Prune()
		}
	}
}
