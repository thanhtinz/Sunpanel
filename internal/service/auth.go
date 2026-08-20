package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/config"
	"github.com/thanhtinz/sunpanel/internal/model"
	"github.com/thanhtinz/sunpanel/pkg/crypto"
	"github.com/thanhtinz/sunpanel/pkg/loginguard"
)

// minPasswordLength là độ dài mật khẩu tối thiểu được chấp nhận.
const minPasswordLength = 8

// refreshTokenBytes là số byte ngẫu nhiên của một refresh token.
const refreshTokenBytes = 32

// LoginRequest là dữ liệu đăng nhập gửi lên.
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	// TOTPCode chỉ bắt buộc khi tài khoản đã bật xác thực hai lớp.
	TOTPCode string `json:"totpCode"`
	// IP và UserAgent do tầng HTTP điền, không nhận từ thân yêu cầu.
	IP        string `json:"-"`
	UserAgent string `json:"-"`
}

// TokenPair là cặp token trả về sau khi đăng nhập thành công.
type TokenPair struct {
	AccessToken  string      `json:"accessToken"`
	RefreshToken string      `json:"refreshToken"`
	ExpiresAt    time.Time   `json:"expiresAt"`
	User         *model.User `json:"user"`
}

// AuthService xử lý đăng nhập, làm mới token, đăng xuất và xác thực hai lớp.
type AuthService struct {
	db     *gorm.DB
	tokens *TokenIssuer
	sealer *crypto.Sealer
	cfg    config.SecurityConfig
	audit  *AuditService
	guard  *loginguard.Guard
}

// NewAuthService tạo service xác thực.
func NewAuthService(db *gorm.DB, tokens *TokenIssuer, sealer *crypto.Sealer, cfg config.SecurityConfig, audit *AuditService) *AuthService {
	guard := loginguard.New(loginguard.Options{
		Threshold: cfg.BlockThreshold,
		Window:    cfg.BlockWindow,
		Duration:  cfg.BlockDuration,
		Trusted:   cfg.AllowedIPs,
	})
	return &AuthService{db: db, tokens: tokens, sealer: sealer, cfg: cfg, audit: audit, guard: guard}
}

// Guard trả về bộ chặn địa chỉ dò mật khẩu, để middleware và trang bảo mật dùng chung.
func (s *AuthService) Guard() *loginguard.Guard { return s.guard }

// noteFailure ghi nhận một lần đăng nhập sai cho địa chỉ gửi yêu cầu.
//
// Chặn không được trả về ngay tại đây: lần sai làm tràn ngưỡng vẫn phải nhận
// đúng lỗi "sai thông tin đăng nhập" như mọi lần khác, nếu không thì chính
// phản hồi lại nói cho kẻ đang dò biết họ vừa chạm ngưỡng nào.
func (s *AuthService) noteFailure(req LoginRequest) {
	if block, blocked := s.guard.Fail(req.IP, req.Username); blocked {
		slog.Warn("chặn tạm thời địa chỉ đăng nhập sai quá nhiều lần",
			"ip", req.IP, "lần_sai", block.Failures, "tới", block.Until.Format(time.RFC3339))
	}
}

// Login xác thực người dùng và cấp cặp token.
//
// Mọi nhánh thất bại đều trả về cùng một lỗi InvalidCredentials và tiêu tốn
// thời gian tương đương, để không lộ thông tin tài khoản nào tồn tại.
func (s *AuthService) Login(ctx context.Context, req LoginRequest) (*TokenPair, error) {
	now := time.Now()

	var user model.User
	err := s.db.WithContext(ctx).Where("username = ?", req.Username).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Vẫn băm một mật khẩu giả để thời gian phản hồi không khác biệt so với
			// trường hợp tài khoản có thật nhưng sai mật khẩu.
			_, _ = crypto.HashPassword(req.Password)
			s.noteFailure(req)
			s.logLogin(ctx, req, false, "auth.invalid_credentials")
			return nil, apperr.InvalidCredentials
		}
		return nil, apperr.Internal.Wrap(err)
	}

	// Hai nhánh dưới đây cũng tính là một lần thử hỏng cho địa chỉ gửi yêu cầu:
	// nếu không, kẻ dò chỉ cần làm khóa một tài khoản là mọi lần thử sau đó của
	// họ trở nên vô hình với lớp đếm theo địa chỉ.
	if !user.Active {
		s.noteFailure(req)
		s.logLogin(ctx, req, false, "auth.account_disabled")
		return nil, apperr.AccountDisabled
	}
	if user.IsLocked(now) {
		s.noteFailure(req)
		s.logLogin(ctx, req, false, "auth.account_locked")
		return nil, apperr.AccountLocked.WithParam("until", user.LockedUntil.Format(time.RFC3339))
	}

	ok, err := crypto.VerifyPassword(req.Password, user.PasswordHash)
	if err != nil {
		return nil, apperr.Internal.Wrap(err)
	}
	if !ok {
		s.registerFailure(ctx, &user, now)
		s.noteFailure(req)
		s.logLogin(ctx, req, false, "auth.invalid_credentials")
		return nil, apperr.InvalidCredentials
	}

	if user.TOTPEnabled {
		if req.TOTPCode == "" {
			// Không tính là đăng nhập sai: mật khẩu đã đúng, chỉ là thiếu bước hai.
			s.logLogin(ctx, req, false, "auth.totp_required")
			return nil, apperr.TOTPRequired
		}
		valid, err := s.verifyTOTP(&user, req.TOTPCode)
		if err != nil {
			return nil, err
		}
		if !valid {
			s.registerFailure(ctx, &user, now)
			s.noteFailure(req)
			s.logLogin(ctx, req, false, "auth.totp_invalid")
			return nil, apperr.TOTPInvalid
		}
	}

	pair, err := s.issuePair(ctx, &user, req.IP, req.UserAgent)
	if err != nil {
		return nil, err
	}
	s.guard.Succeed(req.IP)

	if err := s.db.WithContext(ctx).Model(&user).Updates(map[string]any{
		"failed_attempts": 0,
		"locked_until":    nil,
		"last_login_at":   now,
		"last_login_ip":   req.IP,
	}).Error; err != nil {
		// Đăng nhập đã thành công; không cập nhật được thống kê thì ghi log là đủ.
		slog.Warn("không cập nhật được thông tin đăng nhập", "user", user.Username, "error", err)
	}

	s.logLogin(ctx, req, true, "")
	s.audit.Record(ctx, AuditEntry{
		UserID: user.ID, Username: user.Username,
		Action: "auth.login", IP: req.IP, Success: true,
	})

	return pair, nil
}

// Refresh đổi refresh token lấy cặp token mới.
//
// Refresh token được xoay vòng: token cũ bị thu hồi ngay khi cấp token mới, nên
// một token bị đánh cắp chỉ dùng được một lần và lần dùng thứ hai sẽ thất bại.
func (s *AuthService) Refresh(ctx context.Context, refreshToken, ip, userAgent string) (*TokenPair, error) {
	now := time.Now()

	var session model.Session
	err := s.db.WithContext(ctx).Where("token_hash = ?", HashToken(refreshToken)).First(&session).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.SessionExpired
		}
		return nil, apperr.Internal.Wrap(err)
	}
	if !session.Valid(now) {
		return nil, apperr.SessionExpired
	}

	var user model.User
	if err := s.db.WithContext(ctx).First(&user, session.UserID).Error; err != nil {
		return nil, apperr.SessionExpired
	}
	if !user.Active {
		return nil, apperr.AccountDisabled
	}

	if err := s.db.WithContext(ctx).Model(&session).Update("revoked_at", now).Error; err != nil {
		return nil, apperr.Internal.Wrap(err)
	}

	return s.issuePair(ctx, &user, ip, userAgent)
}

// Logout thu hồi phiên tương ứng với refresh token.
func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	err := s.db.WithContext(ctx).Model(&model.Session{}).
		Where("token_hash = ? AND revoked_at IS NULL", HashToken(refreshToken)).
		Update("revoked_at", time.Now()).Error
	if err != nil {
		return apperr.Internal.Wrap(err)
	}
	return nil
}

// ListSessions liệt kê các phiên còn hiệu lực của một người dùng.
func (s *AuthService) ListSessions(ctx context.Context, userID uint) ([]model.Session, error) {
	var sessions []model.Session
	err := s.db.WithContext(ctx).
		Where("user_id = ? AND revoked_at IS NULL AND expires_at > ?", userID, time.Now()).
		Order("last_used DESC").Find(&sessions).Error
	if err != nil {
		return nil, apperr.Internal.Wrap(err)
	}
	return sessions, nil
}

// RevokeSession thu hồi một phiên cụ thể của người dùng.
func (s *AuthService) RevokeSession(ctx context.Context, userID, sessionID uint) error {
	res := s.db.WithContext(ctx).Model(&model.Session{}).
		Where("id = ? AND user_id = ? AND revoked_at IS NULL", sessionID, userID).
		Update("revoked_at", time.Now())
	if res.Error != nil {
		return apperr.Internal.Wrap(res.Error)
	}
	if res.RowsAffected == 0 {
		return apperr.NotFound
	}
	return nil
}

// RevokeAllSessions thu hồi mọi phiên của một người dùng, dùng khi đổi mật khẩu
// hoặc khi quản trị viên vô hiệu hóa tài khoản.
func (s *AuthService) RevokeAllSessions(ctx context.Context, userID uint) error {
	err := s.db.WithContext(ctx).Model(&model.Session{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", time.Now()).Error
	if err != nil {
		return apperr.Internal.Wrap(err)
	}
	return nil
}

// SessionActive kiểm tra phiên gắn với access token còn hiệu lực hay không.
// Middleware gọi hàm này để việc thu hồi phiên có hiệu lực tức thì.
func (s *AuthService) SessionActive(ctx context.Context, sessionID uint) bool {
	var count int64
	err := s.db.WithContext(ctx).Model(&model.Session{}).
		Where("id = ? AND revoked_at IS NULL AND expires_at > ?", sessionID, time.Now()).
		Count(&count).Error
	return err == nil && count > 0
}

// CleanupExpiredSessions xóa các phiên đã hết hạn hoặc bị thu hồi từ lâu.
func (s *AuthService) CleanupExpiredSessions(ctx context.Context) (int64, error) {
	cutoff := time.Now().Add(-24 * time.Hour)
	res := s.db.WithContext(ctx).
		Where("expires_at < ? OR (revoked_at IS NOT NULL AND revoked_at < ?)", time.Now(), cutoff).
		Delete(&model.Session{})
	if res.Error != nil {
		return 0, fmt.Errorf("dọn phiên hết hạn: %w", res.Error)
	}
	return res.RowsAffected, nil
}

// issuePair tạo phiên mới và phát hành cặp token cho phiên đó.
func (s *AuthService) issuePair(ctx context.Context, user *model.User, ip, userAgent string) (*TokenPair, error) {
	refreshToken, err := crypto.RandomHex(refreshTokenBytes)
	if err != nil {
		return nil, apperr.Internal.Wrap(err)
	}

	now := time.Now()
	session := model.Session{
		UserID:    user.ID,
		TokenHash: HashToken(refreshToken),
		UserAgent: truncate(userAgent, 512),
		IP:        ip,
		ExpiresAt: now.Add(s.cfg.RefreshTokenTTL),
		LastUsed:  now,
	}
	if err := s.db.WithContext(ctx).Create(&session).Error; err != nil {
		return nil, apperr.Internal.Wrap(err)
	}

	accessToken, expiresAt, err := s.tokens.Issue(user, session.ID)
	if err != nil {
		return nil, apperr.Internal.Wrap(err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
		User:         user,
	}, nil
}

// registerFailure tăng số lần đăng nhập sai và khóa tài khoản khi vượt ngưỡng.
func (s *AuthService) registerFailure(ctx context.Context, user *model.User, now time.Time) {
	updates := map[string]any{"failed_attempts": user.FailedAttempts + 1}
	if s.cfg.MaxLoginAttempts > 0 && user.FailedAttempts+1 >= s.cfg.MaxLoginAttempts {
		until := now.Add(s.cfg.LockoutDuration)
		updates["locked_until"] = until
		updates["failed_attempts"] = 0
		slog.Warn("khóa tài khoản do đăng nhập sai quá nhiều lần",
			"user", user.Username, "until", until.Format(time.RFC3339))
	}
	if err := s.db.WithContext(ctx).Model(user).Updates(updates).Error; err != nil {
		slog.Error("không ghi nhận được lần đăng nhập sai", "user", user.Username, "error", err)
	}
}

func (s *AuthService) logLogin(ctx context.Context, req LoginRequest, success bool, reason string) {
	entry := model.LoginLog{
		Username:  truncate(req.Username, 64),
		IP:        req.IP,
		UserAgent: truncate(req.UserAgent, 512),
		Success:   success,
		Reason:    reason,
	}
	if err := s.db.WithContext(ctx).Create(&entry).Error; err != nil {
		slog.Error("không ghi được nhật ký đăng nhập", "error", err)
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return strings.ToValidUTF8(s[:max], "")
}
