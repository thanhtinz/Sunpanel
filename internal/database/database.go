// Package database khởi tạo cơ sở dữ liệu nội bộ của panel.
//
// Panel dùng SQLite qua driver thuần Go (không cần cgo) để giữ được khả năng
// biên dịch chéo sang mọi nền tảng từ một máy duy nhất.
package database

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/thanhtinz/sunpanel/internal/model"
)

// Open mở (và tạo nếu chưa có) cơ sở dữ liệu tại path, rồi chạy migration.
func Open(path string, debug bool) (*gorm.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("tạo thư mục cơ sở dữ liệu: %w", err)
	}

	level := logger.Warn
	if debug {
		level = logger.Info
	}

	// WAL cho phép đọc song song với ghi — cần thiết vì bộ thu thập số liệu ghi
	// liên tục trong khi người dùng vẫn truy vấn dashboard.
	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(level),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		return nil, fmt.Errorf("mở cơ sở dữ liệu: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("lấy kết nối cơ sở dữ liệu: %w", err)
	}
	// SQLite chỉ cho phép một tiến trình ghi tại một thời điểm; giữ pool nhỏ để
	// tránh các lời gọi ghi tranh nhau và trả về lỗi "database is locked".
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if err := migrate(db); err != nil {
		return nil, err
	}

	// Tệp cơ sở dữ liệu chứa bí mật đã mã hóa và toàn bộ lịch sử — không ai ngoài
	// chủ sở hữu được đọc.
	if err := os.Chmod(path, 0o600); err != nil {
		slog.Warn("không đặt được quyền cho tệp cơ sở dữ liệu", "path", path, "error", err)
	}

	return db, nil
}

// migrate tạo và cập nhật lược đồ bảng theo các model hiện tại.
func migrate(db *gorm.DB) error {
	models := []any{
		&model.User{},
		&model.Session{},
		&model.AuditLog{},
		&model.LoginLog{},
		&model.MonitorSample{},
		&model.Setting{},
		&model.CronJob{},
		&model.CronRun{},
		&model.Website{},
		&model.Certificate{},
	}
	if err := db.AutoMigrate(models...); err != nil {
		return fmt.Errorf("chạy migration: %w", err)
	}
	return nil
}

// Close đóng kết nối cơ sở dữ liệu.
func Close(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("lấy kết nối cơ sở dữ liệu: %w", err)
	}
	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("đóng cơ sở dữ liệu: %w", err)
	}
	return nil
}
