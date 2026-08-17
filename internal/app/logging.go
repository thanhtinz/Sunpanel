package app

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/thanhtinz/sunpanel/internal/config"
)

// SetupLogging cấu hình logger toàn cục theo cấu hình đã nạp.
// Trả về hàm dọn dẹp để đóng tệp log khi thoát.
func SetupLogging(cfg config.LogConfig) (func(), error) {
	var level slog.Level
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	out := io.Writer(os.Stdout)
	cleanup := func() {}

	if cfg.File != "" {
		if err := os.MkdirAll(filepath.Dir(cfg.File), 0o750); err != nil {
			return nil, fmt.Errorf("tạo thư mục log: %w", err)
		}
		// Log có thể chứa đường dẫn và tên tài khoản, nên chỉ chủ sở hữu được đọc.
		file, err := os.OpenFile(cfg.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return nil, fmt.Errorf("mở tệp log: %w", err)
		}
		// Ghi song song ra cả tệp lẫn màn hình để chạy dưới systemd vẫn thấy log
		// bằng journalctl mà không mất bản lưu trên đĩa.
		out = io.MultiWriter(os.Stdout, file)
		cleanup = func() {
			if err := file.Close(); err != nil {
				slog.Error("không đóng được tệp log", "path", cfg.File, "error", err)
			}
		}
	}

	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if cfg.Format == "json" {
		handler = slog.NewJSONHandler(out, opts)
	} else {
		handler = slog.NewTextHandler(out, opts)
	}
	slog.SetDefault(slog.New(handler))

	return cleanup, nil
}
