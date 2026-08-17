package service

import (
	"errors"
	"testing"

	"github.com/thanhtinz/sunpanel/internal/apperr"
)

func TestValidatePassword(t *testing.T) {
	valid := []string{
		"MatKhau2026",
		"abcd1234",
		"CHU-HOA-va-thuong",
		"mật-khẩu-Tiếng-Việt",
	}
	for _, password := range valid {
		t.Run("hợp lệ: "+password, func(t *testing.T) {
			if err := ValidatePassword(password); err != nil {
				t.Errorf("mật khẩu phải được chấp nhận, nhận lỗi: %v", err)
			}
		})
	}

	invalid := map[string]string{
		"quá ngắn":             "Abc123",
		"chỉ chữ thường":       "abcdefghij",
		"chỉ chữ số":           "1234567890",
		"rỗng":                 "",
		"chỉ đủ dài, một loại": "aaaaaaaaaaaa",
	}
	for name, password := range invalid {
		t.Run("bị từ chối: "+name, func(t *testing.T) {
			err := ValidatePassword(password)
			if !errors.Is(err, apperr.PasswordTooWeak) {
				t.Errorf("lỗi = %v, mong đợi PasswordTooWeak", err)
			}
		})
	}
}
