package service

import (
	"context"
	"fmt"
	"time"

	"github.com/pquerna/otp/totp"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/model"
)

// totpIssuer là tên hiển thị trong ứng dụng xác thực của người dùng.
const totpIssuer = "SunPanel"

// TOTPSetup là dữ liệu để người dùng thiết lập xác thực hai lớp.
type TOTPSetup struct {
	// Secret là khóa dạng base32 để nhập tay khi không quét được mã QR.
	Secret string `json:"secret"`
	// URL là chuỗi otpauth:// dùng sinh mã QR ở phía giao diện.
	URL string `json:"url"`
}

// BeginTOTPSetup sinh khóa mới và lưu tạm (đã mã hóa) nhưng chưa bật 2FA.
// Chỉ khi người dùng nhập đúng mã đầu tiên thì 2FA mới thực sự được kích hoạt —
// nhờ vậy không ai tự khóa mình ra ngoài vì cấu hình sai ứng dụng xác thực.
func (s *AuthService) BeginTOTPSetup(ctx context.Context, userID uint) (*TOTPSetup, error) {
	var user model.User
	if err := s.db.WithContext(ctx).First(&user, userID).Error; err != nil {
		return nil, apperr.UserNotFound.Wrap(err)
	}
	if user.TOTPEnabled {
		return nil, apperr.TOTPAlreadyEnabled
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      totpIssuer,
		AccountName: user.Username,
	})
	if err != nil {
		return nil, apperr.Internal.Wrap(fmt.Errorf("sinh khóa totp: %w", err))
	}

	sealed, err := s.sealer.Seal(key.Secret())
	if err != nil {
		return nil, apperr.Internal.Wrap(err)
	}
	if err := s.db.WithContext(ctx).Model(&user).Update("totp_secret", sealed).Error; err != nil {
		return nil, apperr.Internal.Wrap(err)
	}

	return &TOTPSetup{Secret: key.Secret(), URL: key.URL()}, nil
}

// ConfirmTOTPSetup kích hoạt 2FA sau khi người dùng chứng minh đã cấu hình đúng.
func (s *AuthService) ConfirmTOTPSetup(ctx context.Context, userID uint, code string) error {
	var user model.User
	if err := s.db.WithContext(ctx).First(&user, userID).Error; err != nil {
		return apperr.UserNotFound.Wrap(err)
	}
	if user.TOTPEnabled {
		return apperr.TOTPAlreadyEnabled
	}
	if user.TOTPSecret == "" {
		return apperr.BadRequest
	}

	valid, err := s.verifyTOTP(&user, code)
	if err != nil {
		return err
	}
	if !valid {
		return apperr.TOTPInvalid
	}

	if err := s.db.WithContext(ctx).Model(&user).Update("totp_enabled", true).Error; err != nil {
		return apperr.Internal.Wrap(err)
	}

	s.audit.Record(ctx, AuditEntry{
		UserID: user.ID, Username: user.Username,
		Action: "auth.totp_enable", Success: true,
	})
	return nil
}

// DisableTOTP tắt xác thực hai lớp; bắt buộc xác nhận lại mật khẩu để một phiên
// bị chiếm quyền không thể tự tay gỡ bỏ lớp bảo vệ này.
func (s *AuthService) DisableTOTP(ctx context.Context, userID uint, password string) error {
	var user model.User
	if err := s.db.WithContext(ctx).First(&user, userID).Error; err != nil {
		return apperr.UserNotFound.Wrap(err)
	}

	ok, err := verifyPasswordOf(&user, password)
	if err != nil {
		return err
	}
	if !ok {
		return apperr.PasswordMismatch
	}

	err = s.db.WithContext(ctx).Model(&user).Updates(map[string]any{
		"totp_enabled": false,
		"totp_secret":  "",
	}).Error
	if err != nil {
		return apperr.Internal.Wrap(err)
	}

	s.audit.Record(ctx, AuditEntry{
		UserID: user.ID, Username: user.Username,
		Action: "auth.totp_disable", Success: true,
	})
	return nil
}

// verifyTOTP giải mã khóa của người dùng rồi kiểm tra mã sáu số.
//
// Cửa sổ chấp nhận là ±1 bước (30 giây), đủ để bù lệch đồng hồ giữa máy chủ và
// điện thoại mà không nới lỏng bảo mật đáng kể.
func (s *AuthService) verifyTOTP(user *model.User, code string) (bool, error) {
	if user.TOTPSecret == "" {
		return false, apperr.TOTPInvalid
	}

	secret, err := s.sealer.Open(user.TOTPSecret)
	if err != nil {
		return false, apperr.Internal.Wrap(fmt.Errorf("giải mã khóa totp: %w", err))
	}

	valid, err := totp.ValidateCustom(code, secret, time.Now().UTC(), totp.ValidateOpts{
		Period: 30,
		Skew:   1,
		Digits: 6,
	})
	if err != nil {
		// Mã sai định dạng cũng bị coi là mã không hợp lệ, không phải lỗi hệ thống.
		return false, nil
	}
	return valid, nil
}
