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
	Auth     *service.AuthService
	Users    *service.UserService
	Audit    *service.AuditService
	Monitor  *service.MonitorService
	Files    *service.FileService
	Terminal *service.TerminalService
	Services *service.SystemServiceManager
	Cron     *service.CronService
	Firewall *service.FirewallService
	Docker   *service.DockerService
	Tokens   *service.TokenIssuer
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

	authenticated := api.Group("", middleware.Auth(svc.Tokens, svc.Auth))
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
