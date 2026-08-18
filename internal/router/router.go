// Package router khai báo toàn bộ tuyến HTTP của panel.
package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	v1 "github.com/thanhtinz/sunpanel/internal/api/v1"
	"github.com/thanhtinz/sunpanel/internal/config"
	"github.com/thanhtinz/sunpanel/internal/middleware"
	"github.com/thanhtinz/sunpanel/internal/response"
	"github.com/thanhtinz/sunpanel/internal/service"
	"github.com/thanhtinz/sunpanel/web"
)

// loginRateLimit là giới hạn tần suất cho các endpoint xác thực: 20 lần mỗi phút
// với khả năng dồn cục 10 lần, đủ thoáng cho người dùng thật và đủ chặt để việc
// dò mật khẩu tự động trở nên vô vọng.
const (
	loginRatePerMinute = 20
	loginBurst         = 10
)

// Services gom các service mà router cần.
type Services struct {
	Auth      *service.AuthService
	Users     *service.UserService
	Audit     *service.AuditService
	Monitor   *service.MonitorService
	Files     *service.FileService
	Terminal  *service.TerminalService
	Services  *service.SystemServiceManager
	Cron      *service.CronService
	Apps      *service.AppStoreService
	Databases *service.DatabaseService
	Backups   *service.BackupService
	Websites  *service.WebsiteService
	Certs     *service.CertificateService
	Firewall  *service.FirewallService
	Docker    *service.DockerService
	Tokens    *service.TokenIssuer
	APIKeys   *service.APIKeyService
	Alerts    *service.AlertService
}

// New dựng handler HTTP hoàn chỉnh của panel.
func New(cfg config.Config, svc Services) (http.Handler, error) {
	if cfg.Log.Level != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()
	engine.RedirectTrailingSlash = false

	if err := engine.SetTrustedProxies(cfg.Security.TrustedProxies); err != nil {
		return nil, err
	}

	engine.Use(
		middleware.Recovery(),
		middleware.WithRequestID(),
		middleware.WithLanguage(),
		middleware.Logger(),
		middleware.SecurityHeaders(),
	)

	// Danh sách IP và đường dẫn bí mật đứng trước mọi thứ khác: yêu cầu không hợp
	// lệ bị loại bỏ trước khi chạm tới bất kỳ logic nào.
	if allowlist := middleware.IPAllowlist(cfg.Security.AllowedIPs); allowlist != nil {
		engine.Use(allowlist)
	}
	registerAPI(engine, svc)

	// Giao diện được nhúng sẵn trong binary; mọi đường dẫn không phải API đều trả
	// về trang đơn để bộ định tuyến phía trình duyệt tự xử lý.
	if err := web.Register(engine, cfg.Server.EntryPath); err != nil {
		return nil, err
	}

	// Đường dẫn bí mật được gỡ ở ngoài engine vì bộ định tuyến khớp đường dẫn
	// trước khi middleware bên trong kịp chạy.
	return middleware.StripEntryPath(cfg.Server.EntryPath, engine), nil
}

func registerAPI(engine *gin.Engine, svc Services) {
	authHandler := v1.NewAuthHandler(svc.Auth, svc.Users)
	userHandler := v1.NewUserHandler(svc.Users, svc.Audit)
	monitorHandler := v1.NewMonitorHandler(svc.Monitor)
	fileHandler := v1.NewFileHandler(svc.Files, svc.Audit, svc.Tokens)
	terminalHandler := v1.NewTerminalHandler(svc.Terminal)
	sysServiceHandler := v1.NewSysServiceHandler(svc.Services)
	cronHandler := v1.NewCronHandler(svc.Cron, svc.Audit)
	websiteHandler := v1.NewWebsiteHandler(svc.Websites, svc.Certs, svc.Audit)
	appHandler := v1.NewAppStoreHandler(svc.Apps, svc.Audit)
	dbHandler := v1.NewDatabaseHandler(svc.Databases, svc.Audit)
	backupHandler := v1.NewBackupHandler(svc.Backups, svc.Audit)
	alertHandler := v1.NewAlertHandler(svc.Alerts, svc.Audit)
	apiKeyHandler := v1.NewAPIKeyHandler(svc.APIKeys, svc.Audit)
	firewallHandler := v1.NewFirewallHandler(svc.Firewall)
	dockerHandler := v1.NewDockerHandler(svc.Docker)

	api := engine.Group("/api/v1")

	// Kiểm tra sống dùng cho giám sát ngoài; không cần xác thực và không lộ gì.
	api.GET("/health", func(c *gin.Context) {
		response.OK(c, gin.H{"status": "ok"})
	})

	limiter := middleware.NewRateLimiter(loginRatePerMinute, loginBurst).Middleware()
	public := api.Group("/auth", limiter)
	{
		public.POST("/login", authHandler.Login)
		public.POST("/refresh", authHandler.Refresh)
		public.POST("/logout", authHandler.Logout)
	}

	// Tải tệp dùng vé ngắn hạn thay vì header Authorization, vì trình duyệt không
	// gửi được header khi điều hướng tới một URL tải tệp.
	api.GET("/files/download", fileHandler.Download)

	authenticated := api.Group("", middleware.Auth(svc.Tokens, svc.Auth, svc.APIKeys))
	{
		me := authenticated.Group("/auth")
		{
			me.GET("/me", authHandler.Me)
			me.POST("/password", authHandler.ChangePassword)
			me.GET("/sessions", authHandler.ListSessions)
			me.DELETE("/sessions/:id", authHandler.RevokeSession)
			me.POST("/totp/setup", authHandler.BeginTOTP)
			me.POST("/totp/confirm", authHandler.ConfirmTOTP)
			me.POST("/totp/disable", authHandler.DisableTOTP)
		}

		monitor := authenticated.Group("/monitor")
		{
			monitor.GET("/overview", monitorHandler.Overview)
			monitor.GET("/current", monitorHandler.Current)
			monitor.GET("/history", monitorHandler.History)
			monitor.GET("/stream", monitorHandler.Stream)
		}

		authenticated.PATCH("/users/me/preferences", userHandler.UpdatePreferences)

		// Xem tài nguyên Docker thì ai đăng nhập cũng được; điều khiển thì không.
		dockerGroup := authenticated.Group("/docker")
		{
			dockerGroup.GET("/status", dockerHandler.Status)
			dockerGroup.GET("/containers", dockerHandler.ListContainers)
			dockerGroup.GET("/containers/:id/logs", dockerHandler.ContainerLogs)
			dockerGroup.GET("/containers/:id/stats", dockerHandler.ContainerStats)
			dockerGroup.GET("/images", dockerHandler.ListImages)
			dockerGroup.GET("/volumes", dockerHandler.ListVolumes)
			dockerGroup.GET("/networks", dockerHandler.ListNetworks)

			dockerWrite := dockerGroup.Group("", middleware.RequireWrite())
			{
				dockerWrite.POST("/containers/:id/:action", dockerHandler.ControlContainer)
				dockerWrite.POST("/images/pull", dockerHandler.PullImage)
				dockerWrite.DELETE("/images/:id", dockerHandler.RemoveImage)
				dockerWrite.DELETE("/volumes/:name", dockerHandler.RemoveVolume)
				dockerWrite.POST("/prune", dockerHandler.Prune)
			}
		}

		// Xem cấu hình tường lửa thì ai đăng nhập cũng được; thay đổi thì không.
		firewallGroup := authenticated.Group("/firewall")
		{
			firewallGroup.GET("/status", firewallHandler.Status)
			firewallGroup.GET("/rules", firewallHandler.ListRules)

			firewallWrite := firewallGroup.Group("", middleware.RequireWrite())
			{
				firewallWrite.POST("/rules", firewallHandler.AddRule)
				firewallWrite.DELETE("/rules/:id", firewallHandler.DeleteRule)
				firewallWrite.POST("/enabled", firewallHandler.SetEnabled)
			}
		}

		// Tác vụ định kỳ chạy lệnh tùy ý trên máy chủ, nên xem thì ai cũng được
		// nhưng tạo và sửa thì phải có quyền vận hành.
		cronGroup := authenticated.Group("/cron")
		{
			cronGroup.GET("", cronHandler.List)
			cronGroup.GET("/:id", cronHandler.Get)
			cronGroup.GET("/:id/runs", cronHandler.Runs)

			cronWrite := cronGroup.Group("", middleware.RequireWrite())
			{
				cronWrite.POST("", cronHandler.Create)
				cronWrite.POST("/validate", cronHandler.Validate)
				cronWrite.PUT("/:id", cronHandler.Update)
				cronWrite.DELETE("/:id", cronHandler.Delete)
				cronWrite.POST("/:id/enabled", cronHandler.SetEnabled)
				cronWrite.POST("/:id/run", cronHandler.RunNow)
			}
		}

		// Website và chứng chỉ: xem thì ai đăng nhập cũng được, còn sửa cấu hình
		// máy chủ web thì phải có quyền vận hành — một khối server sai đủ để làm
		// mọi website trên máy ngừng phục vụ.
		websites := authenticated.Group("/websites")
		{
			websites.GET("/status", websiteHandler.Status)
			websites.GET("", websiteHandler.List)
			websites.GET("/:id", websiteHandler.Get)
			websites.GET("/:id/config", websiteHandler.Config)

			websiteWrite := websites.Group("", middleware.RequireWrite())
			{
				websiteWrite.POST("", websiteHandler.Create)
				websiteWrite.POST("/reload", websiteHandler.Reload)
				websiteWrite.PUT("/:id", websiteHandler.Update)
				websiteWrite.DELETE("/:id", websiteHandler.Delete)
				websiteWrite.POST("/:id/enabled", websiteHandler.SetEnabled)
			}
		}

		certificates := authenticated.Group("/certificates")
		{
			certificates.GET("", websiteHandler.ListCerts)

			certWrite := certificates.Group("", middleware.RequireWrite())
			{
				certWrite.POST("", websiteHandler.IssueCert)
				certWrite.POST("/:name/renew", websiteHandler.RenewCert)
				certWrite.DELETE("/:name", websiteHandler.DeleteCert)
			}
		}

		// Chợ ứng dụng: cài một ứng dụng là chạy container tùy ý trên máy chủ, nên
		// mọi thao tác thay đổi đều cần quyền vận hành. Xem tham số thì càng chặt
		// hơn nữa vì trong đó có mật khẩu.
		apps := authenticated.Group("/apps")
		{
			apps.GET("/status", appHandler.Status)
			apps.GET("/catalog", appHandler.Catalog)
			apps.GET("", appHandler.List)
			apps.GET("/:id", appHandler.Get)

			appWrite := apps.Group("", middleware.RequireWrite())
			{
				appWrite.GET("/:id/logs", appHandler.Logs)
				appWrite.GET("/:id/params", appHandler.Params)
				appWrite.POST("", appHandler.Install)
				appWrite.DELETE("/:id", appHandler.Uninstall)
				appWrite.POST("/:id/:action", appHandler.Control)
			}
		}

		// Cơ sở dữ liệu: xem danh sách thì ai đăng nhập cũng được, còn tạo, xóa và
		// chạy SQL thì phải có quyền vận hành — cửa sổ SQL chạy được cả DROP.
		databases := authenticated.Group("/db")
		{
			databases.GET("/servers", dbHandler.ListServers)
			databases.GET("/servers/:id/databases", dbHandler.ListDatabases)
			databases.GET("/servers/:id/databases/:name/tables", dbHandler.ListTables)
			databases.GET("/servers/:id/users", dbHandler.ListUsers)

			dbWrite := databases.Group("", middleware.RequireWrite())
			{
				dbWrite.POST("/servers", dbHandler.CreateServer)
				dbWrite.PUT("/servers/:id", dbHandler.UpdateServer)
				dbWrite.DELETE("/servers/:id", dbHandler.DeleteServer)
				dbWrite.POST("/servers/:id/databases", dbHandler.CreateDatabase)
				dbWrite.DELETE("/servers/:id/databases/:name", dbHandler.DropDatabase)
				dbWrite.POST("/servers/:id/users", dbHandler.CreateUser)
				dbWrite.POST("/servers/:id/users/password", dbHandler.ChangePassword)
				dbWrite.DELETE("/servers/:id/users/:name", dbHandler.DropUser)
				dbWrite.POST("/servers/:id/query", dbHandler.Query)
			}
		}

		// Sao lưu: khôi phục ghi đè dữ liệu đang chạy và không hoàn tác được, nên
		// mọi thao tác thay đổi đều cần quyền vận hành.
		backups := authenticated.Group("/backups")
		{
			backups.GET("", backupHandler.List)
			backups.GET("/:id", backupHandler.Get)
			backups.GET("/:id/runs", backupHandler.Runs)

			backupWrite := backups.Group("", middleware.RequireWrite())
			{
				backupWrite.GET("/:id/objects", backupHandler.Objects)
				backupWrite.POST("", backupHandler.Create)
				backupWrite.POST("/check", backupHandler.Check)
				backupWrite.PUT("/:id", backupHandler.Update)
				backupWrite.DELETE("/:id", backupHandler.Delete)
				backupWrite.POST("/:id/enabled", backupHandler.SetEnabled)
				backupWrite.POST("/:id/run", backupHandler.Run)
				backupWrite.POST("/:id/restore", backupHandler.Restore)
				backupWrite.DELETE("/:id/objects/:object", backupHandler.DeleteObject)
			}
		}

		// Cảnh báo: xem thì ai đăng nhập cũng được, còn sửa kênh và quy tắc thì phải
		// có quyền vận hành — cấu hình kênh chứa mật khẩu SMTP và token bot.
		alerts := authenticated.Group("/alerts")
		{
			alerts.GET("/channels", alertHandler.ListChannels)
			alerts.GET("/rules", alertHandler.ListRules)
			alerts.GET("/events", alertHandler.Events)

			alertWrite := alerts.Group("", middleware.RequireWrite())
			{
				alertWrite.POST("/channels", alertHandler.CreateChannel)
				alertWrite.PUT("/channels/:id", alertHandler.UpdateChannel)
				alertWrite.DELETE("/channels/:id", alertHandler.DeleteChannel)
				alertWrite.POST("/channels/:id/test", alertHandler.TestChannel)
				alertWrite.POST("/rules", alertHandler.CreateRule)
				alertWrite.PUT("/rules/:id", alertHandler.UpdateRule)
				alertWrite.DELETE("/rules/:id", alertHandler.DeleteRule)
			}
		}

		// Khóa API: ai cũng quản lý được khóa của chính mình.
		apiKeys := authenticated.Group("/apikeys")
		{
			apiKeys.GET("", apiKeyHandler.List)
			apiKeys.POST("", apiKeyHandler.Create)
			apiKeys.POST("/:id/enabled", apiKeyHandler.SetEnabled)
			apiKeys.DELETE("/:id", apiKeyHandler.Delete)
		}

		// Xem trạng thái dịch vụ thì ai đăng nhập cũng được; điều khiển thì không.
		services := authenticated.Group("/services")
		{
			services.GET("/status", sysServiceHandler.Status)
			services.GET("", sysServiceHandler.List)
			services.GET("/:name", sysServiceHandler.Get)
			services.GET("/:name/logs", sysServiceHandler.Logs)
			services.POST("/:name/:action", middleware.RequireWrite(), sysServiceHandler.Control)
		}

		// Terminal là shell đầy đủ trên máy chủ, nên chỉ dành cho quyền vận hành
		// trở lên — tài khoản chỉ xem không được chạm vào.
		terminal := authenticated.Group("/terminal", middleware.RequireWrite())
		{
			terminal.GET("/status", terminalHandler.Status)
			terminal.GET("/ws", terminalHandler.Connect)
		}

		// Đọc tệp thì ai đăng nhập cũng được; mọi thao tác ghi đòi quyền vận hành.
		files := authenticated.Group("/files")
		{
			files.GET("", fileHandler.List)
			files.GET("/stat", fileHandler.Stat)
			files.GET("/content", fileHandler.Read)
			files.POST("/ticket", fileHandler.Ticket)

			write := files.Group("", middleware.RequireWrite())
			{
				write.PUT("/content", fileHandler.Write)
				write.POST("/upload", fileHandler.Upload)
				write.POST("/mkdir", fileHandler.Mkdir)
				write.POST("/remove", fileHandler.Remove)
				write.POST("/move", fileHandler.Move)
				write.POST("/chmod", fileHandler.Chmod)
				write.POST("/compress", fileHandler.Compress)
				write.POST("/extract", fileHandler.Extract)
			}
		}

		// Quản lý người dùng và nhật ký kiểm toán chỉ dành cho quản trị viên.
		admin := authenticated.Group("", middleware.RequireAdmin())
		{
			admin.GET("/users", userHandler.List)
			admin.POST("/users", userHandler.Create)
			admin.GET("/users/:id", userHandler.Get)
			admin.PATCH("/users/:id", userHandler.Update)
			admin.DELETE("/users/:id", userHandler.Delete)
			admin.POST("/users/:id/password", userHandler.ResetPassword)
			admin.GET("/audit", userHandler.ListAudit)
			admin.GET("/audit/logins", userHandler.ListLoginLogs)
		}
	}
}
