// Package app lắp ráp các thành phần của panel thành một ứng dụng chạy được.
package app

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"gorm.io/gorm"

	"github.com/thanhtinz/sunpanel/internal/config"
	"github.com/thanhtinz/sunpanel/internal/model"
	"github.com/thanhtinz/sunpanel/pkg/crypto"
)

// masterKeySize là độ dài khóa chủ dùng cho AES-256.
const masterKeySize = 32

// defaultAdminUsername là tên tài khoản quản trị được tạo ở lần chạy đầu.
const defaultAdminUsername = "admin"

// Credentials là thông tin đăng nhập sinh tự động ở lần khởi tạo đầu tiên.
type Credentials struct {
	Username  string
	Password  string
	EntryPath string
}

// EnsureConfig chuẩn bị các bí mật còn thiếu trong cấu hình và ghi lại xuống đĩa.
//
// Chạy ở lần đầu tiên, hàm này sinh khóa ký JWT và đường dẫn bí mật; các lần sau
// nó giữ nguyên giá trị đã có để phiên đăng nhập cũ không bị vô hiệu.
func EnsureConfig(cfg *config.Config) (bool, error) {
	changed := false

	if cfg.Security.JWTSecret == "" {
		secret, err := crypto.RandomHex(32)
		if err != nil {
			return false, fmt.Errorf("sinh khóa ký JWT: %w", err)
		}
		cfg.Security.JWTSecret = secret
		changed = true
	}

	if cfg.Server.EntryPath == "" {
		entry, err := crypto.RandomString(12)
		if err != nil {
			return false, fmt.Errorf("sinh đường dẫn bí mật: %w", err)
		}
		cfg.Server.EntryPath = entry
		changed = true
	}

	if changed {
		if err := cfg.Save(cfg.ConfigPath()); err != nil {
			return false, err
		}
	}
	return changed, nil
}

// LoadMasterKey đọc khóa chủ dùng mã hóa bí mật trong cơ sở dữ liệu, tạo mới nếu
// chưa có. Khóa nằm trong tệp riêng với quyền 0600, tách khỏi tệp cơ sở dữ liệu —
// kẻ chỉ lấy được bản sao cơ sở dữ liệu sẽ không giải mã được gì.
func LoadMasterKey(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if len(data) != masterKeySize {
			return nil, fmt.Errorf("khóa chủ tại %s có độ dài %d byte, cần %d byte", path, len(data), masterKeySize)
		}
		return data, nil
	case !errors.Is(err, os.ErrNotExist):
		return nil, fmt.Errorf("đọc khóa chủ: %w", err)
	}

	key := make([]byte, masterKeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("sinh khóa chủ: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("tạo thư mục khóa chủ: %w", err)
	}
	if err := os.WriteFile(path, key, 0o600); err != nil {
		return nil, fmt.Errorf("ghi khóa chủ: %w", err)
	}

	slog.Info("đã sinh khóa chủ mới", "path", path)
	return key, nil
}

// SeedAdmin tạo tài khoản quản trị đầu tiên nếu cơ sở dữ liệu chưa có người dùng nào.
// Trả về nil khi đã có sẵn tài khoản, nghĩa là không cần in thông tin đăng nhập.
func SeedAdmin(ctx context.Context, db *gorm.DB, entryPath string) (*Credentials, error) {
	var count int64
	if err := db.WithContext(ctx).Model(&model.User{}).Count(&count).Error; err != nil {
		return nil, fmt.Errorf("đếm người dùng: %w", err)
	}
	if count > 0 {
		return nil, nil
	}

	password, err := crypto.RandomPassword(16)
	if err != nil {
		return nil, fmt.Errorf("sinh mật khẩu quản trị: %w", err)
	}
	hash, err := crypto.HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("băm mật khẩu quản trị: %w", err)
	}

	admin := model.User{
		Username:     defaultAdminUsername,
		PasswordHash: hash,
		Role:         model.RoleAdmin,
		Language:     "vi",
		Theme:        "auto",
		Active:       true,
		// Mật khẩu sinh tự động được in ra console, nên coi như đã lộ ở mức nào đó;
		// buộc đổi ngay ở lần đăng nhập đầu.
		MustChangePassword: true,
	}
	if err := db.WithContext(ctx).Create(&admin).Error; err != nil {
		return nil, fmt.Errorf("tạo tài khoản quản trị: %w", err)
	}

	return &Credentials{
		Username:  admin.Username,
		Password:  password,
		EntryPath: entryPath,
	}, nil
}

// ResetPassword đặt lại mật khẩu của một tài khoản từ dòng lệnh, dùng khi quản
// trị viên quên mật khẩu và không vào được giao diện.
func ResetPassword(ctx context.Context, db *gorm.DB, username string) (string, error) {
	var user model.User
	if err := db.WithContext(ctx).Where("username = ?", username).First(&user).Error; err != nil {
		return "", fmt.Errorf("tìm người dùng %q: %w", username, err)
	}

	password, err := crypto.RandomPassword(16)
	if err != nil {
		return "", fmt.Errorf("sinh mật khẩu: %w", err)
	}
	hash, err := crypto.HashPassword(password)
	if err != nil {
		return "", fmt.Errorf("băm mật khẩu: %w", err)
	}

	err = db.WithContext(ctx).Model(&user).Updates(map[string]any{
		"password_hash":        hash,
		"must_change_password": true,
		"failed_attempts":      0,
		"locked_until":         nil,
		"active":               true,
	}).Error
	if err != nil {
		return "", fmt.Errorf("cập nhật mật khẩu: %w", err)
	}

	// Mật khẩu đã đổi thì mọi phiên cũ phải mất hiệu lực.
	err = db.WithContext(ctx).Model(&model.Session{}).
		Where("user_id = ? AND revoked_at IS NULL", user.ID).
		Update("revoked_at", time.Now()).Error
	if err != nil {
		return "", fmt.Errorf("thu hồi phiên: %w", err)
	}

	return password, nil
}
