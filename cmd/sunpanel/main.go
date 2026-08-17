// Command sunpanel là bảng điều khiển quản trị máy chủ SunPanel.
//
// Toàn bộ panel — API, giao diện web và bộ giám sát — nằm trong một tệp thực thi
// duy nhất, không phụ thuộc runtime nào bên ngoài.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/thanhtinz/sunpanel/internal/app"
	"github.com/thanhtinz/sunpanel/internal/config"
)

// version được gán lúc biên dịch qua -ldflags.
var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "lỗi: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	command, args := parseCommand()

	switch command {
	case "serve":
		return runServe(args)
	case "reset-password":
		return runResetPassword(args)
	case "version":
		printVersion()
		return nil
	case "help":
		printUsage()
		return nil
	default:
		printUsage()
		return fmt.Errorf("lệnh không hợp lệ: %q", command)
	}
}

// parseCommand tách lệnh con khỏi danh sách tham số. Không có lệnh con thì mặc
// định là "serve", để chạy trực tiếp binary là panel khởi động luôn.
func parseCommand() (string, []string) {
	if len(os.Args) < 2 || strings.HasPrefix(os.Args[1], "-") {
		return "serve", os.Args[1:]
	}
	return os.Args[1], os.Args[2:]
}

func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	configPath := fs.String("config", "", "đường dẫn tệp cấu hình (mặc định: <thư-mục-dữ-liệu>/config.yaml)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := loadConfig(*configPath)
	if err != nil {
		return err
	}

	cleanup, err := app.SetupLogging(cfg.Log)
	if err != nil {
		return err
	}
	defer cleanup()

	if _, err := app.EnsureConfig(&cfg); err != nil {
		return err
	}

	instance, err := app.New(cfg)
	if err != nil {
		return err
	}
	defer func() { _ = instance.Close() }()

	creds, err := app.SeedAdmin(context.Background(), instance.DB(), cfg.Server.EntryPath)
	if err != nil {
		return err
	}
	if creds != nil {
		printCredentials(creds, instance.URL())
	}

	return instance.Run()
}

func runResetPassword(args []string) error {
	fs := flag.NewFlagSet("reset-password", flag.ExitOnError)
	configPath := fs.String("config", "", "đường dẫn tệp cấu hình")
	username := fs.String("user", "admin", "tên đăng nhập cần đặt lại mật khẩu")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := loadConfig(*configPath)
	if err != nil {
		return err
	}

	instance, err := app.New(cfg)
	if err != nil {
		return err
	}
	defer func() { _ = instance.Close() }()

	password, err := app.ResetPassword(context.Background(), instance.DB(), *username)
	if err != nil {
		return err
	}

	fmt.Printf("\nĐã đặt lại mật khẩu cho %q.\n", *username)
	fmt.Printf("Mật khẩu mới: %s\n", password)
	fmt.Println("Hãy đổi mật khẩu ngay sau khi đăng nhập.")
	return nil
}

// loadConfig nạp cấu hình, tự tìm tệp mặc định trong thư mục dữ liệu khi người
// dùng không chỉ định đường dẫn.
func loadConfig(path string) (config.Config, error) {
	if path == "" {
		dataDir := config.Default().Server.DataDir
		if env := os.Getenv(config.EnvPrefix + "DATA_DIR"); env != "" {
			dataDir = env
		}
		path = filepath.Join(dataDir, "config.yaml")
	}
	return config.Load(path)
}

func printCredentials(creds *app.Credentials, url string) {
	line := strings.Repeat("=", 64)
	fmt.Printf("\n%s\n", line)
	fmt.Println("  SunPanel đã khởi tạo lần đầu")
	fmt.Println(line)
	fmt.Printf("  Địa chỉ      : %s\n", url)
	fmt.Printf("  Tài khoản    : %s\n", creds.Username)
	fmt.Printf("  Mật khẩu     : %s\n", creds.Password)
	fmt.Printf("  Đường dẫn bí mật: /%s/\n", creds.EntryPath)
	fmt.Println(line)
	fmt.Println("  Hãy lưu lại thông tin này — mật khẩu chỉ hiển thị một lần duy nhất.")
	fmt.Println("  Bạn sẽ được yêu cầu đổi mật khẩu ngay ở lần đăng nhập đầu tiên.")
	fmt.Printf("%s\n\n", line)
}

func printVersion() {
	fmt.Printf("SunPanel %s\n", version)
	fmt.Printf("  commit    : %s\n", commit)
	fmt.Printf("  build date: %s\n", buildDate)
	fmt.Printf("  go        : %s %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

func printUsage() {
	fmt.Print(`SunPanel — bảng điều khiển quản trị máy chủ

Cách dùng:
  sunpanel [lệnh] [tham số]

Lệnh:
  serve            Khởi động panel (mặc định khi không truyền lệnh)
  reset-password   Đặt lại mật khẩu một tài khoản
  version          Hiển thị phiên bản
  help             Hiển thị trợ giúp này

Tham số:
  -config <đường-dẫn>   Tệp cấu hình (mặc định: <thư-mục-dữ-liệu>/config.yaml)
  -user <tên>           Tài khoản cần đặt lại mật khẩu (chỉ với reset-password)

Cấu hình cũng có thể đặt qua biến môi trường với tiền tố SUNPANEL_,
ví dụ SUNPANEL_PORT=8080 hoặc SUNPANEL_DATA_DIR=/srv/sunpanel.
`)
}
