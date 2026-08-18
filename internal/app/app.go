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
	"github.com/thanhtinz/sunpanel/pkg/crypto"
	"github.com/thanhtinz/sunpanel/pkg/host"
)

// sessionCleanupInterval là chu kỳ dọn các phiên đăng nhập đã hết hạn.
const sessionCleanupInterval = 6 * time.Hour

// App là một thể hiện của panel đã được lắp ráp đầy đủ.
type App struct {
	cfg    config.Config
	db     *gorm.DB
	server *http.Server
	svc    router.Services
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

	// Danh sách lệnh cho phép để rỗng ở phiên bản này: các module cần chạy lệnh hệ
	// thống (dịch vụ, tường lửa) sẽ khai báo allowlist riêng khi được thêm vào.
	localHost := host.NewLocalHost(cfg.Server.FileRoot, nil)

	tokens := service.NewTokenIssuer(cfg.Security.JWTSecret, cfg.Security.AccessTokenTTL)
	audit := service.NewAuditService(db)
	auth := service.NewAuthService(db, tokens, sealer, cfg.Security, audit)
	users := service.NewUserService(db, auth, audit)
	monitor := service.NewMonitorService(db, localHost, cfg.Monitor)
	files := service.NewFileService(localHost, audit)
	terminal := service.NewTerminalService(localHost, cfg.Server.FileRoot, audit)

	svc := router.Services{
		Auth:     auth,
		Users:    users,
		Audit:    audit,
		Monitor:  monitor,
		Files:    files,
		Terminal: terminal,
		Tokens:   tokens,
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

	return &App{cfg: cfg, db: db, server: server, svc: svc}, nil
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

	select {
	case err := <-errCh:
		return fmt.Errorf("máy chủ HTTP dừng bất thường: %w", err)
	case <-ctx.Done():
		slog.Info("nhận tín hiệu dừng, đang tắt máy chủ")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), a.cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := a.server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("tắt máy chủ: %w", err)
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
		}
	}
}
