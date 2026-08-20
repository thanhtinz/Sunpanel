package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/config"
	"github.com/thanhtinz/sunpanel/internal/model"
	"github.com/thanhtinz/sunpanel/pkg/crypto"
)

const testPassword = "MatKhau@2026"

func newTestAuth(t *testing.T) (*AuthService, *gorm.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("mở cơ sở dữ liệu trong bộ nhớ: %v", err)
	}
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			sqlDB.Close()
		}
	})

	err = db.AutoMigrate(&model.User{}, &model.Session{}, &model.AuditLog{}, &model.LoginLog{})
	if err != nil {
		t.Fatalf("chạy migration: %v", err)
	}
	// Cơ sở dữ liệu trong bộ nhớ dùng chung giữa các test trong cùng tiến trình,
	// nên phải dọn sạch trước mỗi bài.
	for _, table := range []string{"users", "sessions", "audit_logs", "login_logs"} {
		db.Exec("DELETE FROM " + table)
	}

	sealer, err := crypto.NewSealer(make([]byte, 32))
	if err != nil {
		t.Fatalf("khởi tạo bộ mã hóa: %v", err)
	}

	cfg := config.SecurityConfig{
		AccessTokenTTL:   15 * time.Minute,
		RefreshTokenTTL:  time.Hour,
		MaxLoginAttempts: 3,
		LockoutDuration:  time.Minute,
		BlockThreshold:   4,
		BlockWindow:      time.Minute,
		BlockDuration:    time.Minute,
	}
	audit := NewAuditService(db)
	auth := NewAuthService(db, NewTokenIssuer("khoa-ky-dung-cho-test", cfg.AccessTokenTTL), sealer, cfg, audit)

	return auth, db
}

func seedUser(t *testing.T, db *gorm.DB, username string) *model.User {
	t.Helper()

	hash, err := crypto.HashPassword(testPassword)
	if err != nil {
		t.Fatalf("băm mật khẩu: %v", err)
	}
	user := &model.User{
		Username:     username,
		PasswordHash: hash,
		Role:         model.RoleAdmin,
		Language:     "vi",
		Active:       true,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("tạo người dùng: %v", err)
	}
	return user
}

func TestLoginSuccess(t *testing.T) {
	auth, db := newTestAuth(t)
	seedUser(t, db, "admin")

	pair, err := auth.Login(context.Background(), LoginRequest{
		Username: "admin", Password: testPassword, IP: "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("đăng nhập thất bại: %v", err)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatal("thiếu access token hoặc refresh token")
	}
	if pair.User.Username != "admin" {
		t.Errorf("tên đăng nhập = %q, mong đợi \"admin\"", pair.User.Username)
	}
}

// Sai tên đăng nhập và sai mật khẩu phải trả về cùng một lỗi, nếu không kẻ tấn
// công dò được tài khoản nào có thật.
func TestLoginFailuresAreIndistinguishable(t *testing.T) {
	auth, db := newTestAuth(t)
	seedUser(t, db, "admin")

	cases := []struct{ name, username, password string }{
		{"sai mật khẩu", "admin", "mat-khau-sai"},
		{"không có tài khoản", "khong-ton-tai", testPassword},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := auth.Login(context.Background(), LoginRequest{
				Username: tc.username, Password: tc.password,
			})
			if !errors.Is(err, apperr.InvalidCredentials) {
				t.Fatalf("lỗi = %v, mong đợi InvalidCredentials", err)
			}
		})
	}
}

func TestLoginLocksAccountAfterMaxAttempts(t *testing.T) {
	auth, db := newTestAuth(t)
	seedUser(t, db, "admin")
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		_, _ = auth.Login(ctx, LoginRequest{Username: "admin", Password: "sai"})
	}

	// Đúng mật khẩu cũng phải bị chặn khi tài khoản đang bị khóa.
	_, err := auth.Login(ctx, LoginRequest{Username: "admin", Password: testPassword})
	if !errors.Is(err, apperr.AccountLocked) {
		t.Fatalf("lỗi = %v, mong đợi AccountLocked", err)
	}
}

func TestLoginRejectsDisabledAccount(t *testing.T) {
	auth, db := newTestAuth(t)
	user := seedUser(t, db, "admin")
	db.Model(user).Update("active", false)

	_, err := auth.Login(context.Background(), LoginRequest{
		Username: "admin", Password: testPassword,
	})
	if !errors.Is(err, apperr.AccountDisabled) {
		t.Fatalf("lỗi = %v, mong đợi AccountDisabled", err)
	}
}

// Refresh token phải xoay vòng: dùng lại token cũ sau khi đã làm mới là dấu hiệu
// token bị đánh cắp, và phải thất bại.
func TestRefreshRotatesToken(t *testing.T) {
	auth, db := newTestAuth(t)
	seedUser(t, db, "admin")
	ctx := context.Background()

	first, err := auth.Login(ctx, LoginRequest{Username: "admin", Password: testPassword})
	if err != nil {
		t.Fatalf("đăng nhập: %v", err)
	}

	second, err := auth.Refresh(ctx, first.RefreshToken, "127.0.0.1", "test")
	if err != nil {
		t.Fatalf("làm mới token: %v", err)
	}
	if second.RefreshToken == first.RefreshToken {
		t.Fatal("refresh token không được giữ nguyên sau khi làm mới")
	}

	if _, err := auth.Refresh(ctx, first.RefreshToken, "127.0.0.1", "test"); !errors.Is(err, apperr.SessionExpired) {
		t.Fatalf("dùng lại token cũ: lỗi = %v, mong đợi SessionExpired", err)
	}
}

// Thu hồi phiên phải vô hiệu hóa cả access token chưa hết hạn.
func TestLogoutInvalidatesSession(t *testing.T) {
	auth, db := newTestAuth(t)
	seedUser(t, db, "admin")
	ctx := context.Background()

	pair, err := auth.Login(ctx, LoginRequest{Username: "admin", Password: testPassword})
	if err != nil {
		t.Fatalf("đăng nhập: %v", err)
	}

	claims, err := auth.tokens.Parse(pair.AccessToken)
	if err != nil {
		t.Fatalf("đọc token: %v", err)
	}
	if !auth.SessionActive(ctx, claims.SessionID) {
		t.Fatal("phiên phải còn hiệu lực ngay sau khi đăng nhập")
	}

	if err := auth.Logout(ctx, pair.RefreshToken); err != nil {
		t.Fatalf("đăng xuất: %v", err)
	}
	if auth.SessionActive(ctx, claims.SessionID) {
		t.Fatal("phiên vẫn còn hiệu lực sau khi đăng xuất")
	}
}

func TestTOTPFlow(t *testing.T) {
	auth, db := newTestAuth(t)
	user := seedUser(t, db, "admin")
	ctx := context.Background()

	setup, err := auth.BeginTOTPSetup(ctx, user.ID)
	if err != nil {
		t.Fatalf("bắt đầu thiết lập 2FA: %v", err)
	}
	if setup.Secret == "" || setup.URL == "" {
		t.Fatal("thiếu khóa hoặc URL otpauth")
	}

	// Khóa phải được mã hóa trước khi lưu, không được nằm dạng thô trong DB.
	var stored model.User
	db.First(&stored, user.ID)
	if stored.TOTPSecret == setup.Secret {
		t.Fatal("khóa TOTP bị lưu dạng thô trong cơ sở dữ liệu")
	}
	if stored.TOTPEnabled {
		t.Fatal("2FA không được bật trước khi người dùng xác nhận bằng mã đúng")
	}

	if err := auth.ConfirmTOTPSetup(ctx, user.ID, "000000"); !errors.Is(err, apperr.TOTPInvalid) {
		t.Fatalf("mã sai: lỗi = %v, mong đợi TOTPInvalid", err)
	}
}

func TestCleanupExpiredSessions(t *testing.T) {
	auth, db := newTestAuth(t)
	user := seedUser(t, db, "admin")
	ctx := context.Background()

	expired := model.Session{
		UserID:    user.ID,
		TokenHash: HashToken("token-da-het-han"),
		ExpiresAt: time.Now().Add(-time.Hour),
	}
	if err := db.Create(&expired).Error; err != nil {
		t.Fatalf("tạo phiên hết hạn: %v", err)
	}

	removed, err := auth.CleanupExpiredSessions(ctx)
	if err != nil {
		t.Fatalf("dọn phiên: %v", err)
	}
	if removed != 1 {
		t.Errorf("đã xóa %d phiên, mong đợi 1", removed)
	}
}
