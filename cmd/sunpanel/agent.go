package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/thanhtinz/sunpanel/pkg/agent"
	"github.com/thanhtinz/sunpanel/pkg/certs"
	"github.com/thanhtinz/sunpanel/pkg/host"
)

// agentShutdownTimeout là thời gian chờ agent đóng các kết nối đang xử lý.
const agentShutdownTimeout = 15 * time.Second

// runAgent chạy agent trên máy chủ được quản lý.
//
// Agent là chính binary này chạy ở chế độ khác, không phải một chương trình
// riêng: cài đặt trên node chỉ là chép đúng một tệp, và phiên bản panel với
// agent không thể lệch nhau vì vô ý.
func runAgent(args []string) error {
	fs := flag.NewFlagSet("agent", flag.ExitOnError)
	addr := fs.String("addr", "0.0.0.0:9528", "địa chỉ lắng nghe")
	dataDir := fs.String("data-dir", defaultAgentDataDir(), "thư mục lưu token và chứng chỉ")
	root := fs.String("root", "/", "thư mục gốc mà agent được phép truy cập")
	certFile := fs.String("cert", "", "tệp chứng chỉ TLS (mặc định: tự sinh)")
	keyFile := fs.String("key", "", "tệp khóa riêng TLS (mặc định: tự sinh)")
	showToken := fs.Bool("show-token", false, "in token rồi thoát")
	if err := fs.Parse(args); err != nil {
		return err
	}

	token, err := loadOrCreateAgentToken(filepath.Join(*dataDir, "agent.token"))
	if err != nil {
		return err
	}

	if *showToken {
		fmt.Println(token)
		return nil
	}

	cert, key, err := agentCertificate(*dataDir, *certFile, *keyFile)
	if err != nil {
		return err
	}

	// Agent không giới hạn danh sách lệnh: panel mới là nơi quyết định được phép
	// chạy gì, còn agent chỉ là cánh tay nối dài. Hàng rào thật là token và phạm
	// vi thư mục.
	local := host.NewLocalHost(*root, nil)
	server := &http.Server{
		Addr:              *addr,
		Handler:           agent.NewServer(local, token, version).Handler(),
		ReadHeaderTimeout: 15 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		if err := server.ListenAndServeTLS(cert, key); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	printAgentBanner(*addr, token)

	select {
	case err := <-errCh:
		return fmt.Errorf("agent dừng bất thường: %w", err)
	case <-ctx.Done():
		slog.Info("nhận tín hiệu dừng, đang tắt agent")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), agentShutdownTimeout)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}

// loadOrCreateAgentToken đọc token của agent, sinh mới nếu chưa có.
//
// Token phải giữ nguyên giữa các lần chạy: sinh lại mỗi lần khởi động sẽ khiến
// panel mất kết nối tới node sau mỗi lần agent khởi động lại.
func loadOrCreateAgentToken(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err == nil {
		if token := strings.TrimSpace(string(content)); token != "" {
			return token, nil
		}
	}
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("đọc token agent: %w", err)
	}

	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("sinh token agent: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(buf)

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", fmt.Errorf("tạo thư mục agent: %w", err)
	}
	if err := os.WriteFile(path, []byte(token), 0o600); err != nil {
		return "", fmt.Errorf("ghi token agent: %w", err)
	}
	return token, nil
}

// agentCertificate trả về đường dẫn chứng chỉ TLS, tự sinh nếu chưa có.
func agentCertificate(dataDir, certFile, keyFile string) (string, string, error) {
	if certFile != "" && keyFile != "" {
		return certFile, keyFile, nil
	}

	certPath := filepath.Join(dataDir, "agent.crt")
	keyPath := filepath.Join(dataDir, "agent.key")
	if fileExists(certPath) && fileExists(keyPath) {
		return certPath, keyPath, nil
	}

	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "sunpanel-agent"
	}

	// Chứng chỉ tự ký là đủ cho kênh giữa panel và agent: hai bên đã xác thực
	// bằng token, còn TLS ở đây để không ai đọc được token trên đường truyền.
	certPEM, keyPEM, err := certs.GenerateSelfSigned([]string{hostname, "localhost", "127.0.0.1"})
	if err != nil {
		return "", "", fmt.Errorf("sinh chứng chỉ agent: %w", err)
	}

	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return "", "", fmt.Errorf("tạo thư mục agent: %w", err)
	}
	if err := os.WriteFile(certPath, certPEM, 0o644); err != nil {
		return "", "", fmt.Errorf("ghi chứng chỉ agent: %w", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		return "", "", fmt.Errorf("ghi khóa agent: %w", err)
	}
	return certPath, keyPath, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// printAgentBanner in thông tin cần để nối node vào panel.
func printAgentBanner(addr, token string) {
	hostname, _ := os.Hostname()
	fmt.Println()
	fmt.Println("================================================================")
	fmt.Println("  SunPanel Agent đang chạy")
	fmt.Println("================================================================")
	fmt.Printf("  Máy chủ      : %s\n", hostname)
	fmt.Printf("  Địa chỉ      : https://%s\n", addr)
	fmt.Printf("  Token        : %s\n", token)
	fmt.Println("================================================================")
	fmt.Println("  Dùng địa chỉ và token này để thêm node vào panel.")
	fmt.Println("  Token nằm ở <thư-mục-dữ-liệu>/agent.token, xem lại bằng:")
	fmt.Println("      sunpanel agent -show-token")
	fmt.Println("================================================================")
	fmt.Println()
}
