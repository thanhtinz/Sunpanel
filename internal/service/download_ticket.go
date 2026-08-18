package service

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/thanhtinz/sunpanel/internal/apperr"
)

// downloadTicketTTL là thời gian sống của một vé tải xuống. Ngắn có chủ đích:
// vé chỉ cần đủ sống để trình duyệt bắt đầu tải ngay sau khi được cấp.
const downloadTicketTTL = 60 * time.Second

// DownloadClaims là nội dung của một vé tải xuống.
type DownloadClaims struct {
	jwt.RegisteredClaims
	// Path là đường dẫn tệp duy nhất mà vé này cho phép tải.
	Path string `json:"path"`
	// UserID để ghi nhật ký ai đã tải tệp.
	UserID uint `json:"uid"`
}

// IssueDownloadTicket cấp một vé tải xuống ngắn hạn cho đúng một đường dẫn.
//
// Trình duyệt không cho đặt header Authorization khi điều hướng tới một URL tải
// tệp, nên phải có thứ gì đó đi qua query. Đặt thẳng access token vào URL sẽ
// khiến nó lọt vào lịch sử trình duyệt và header Referer. Vé này thay thế:
// nó chỉ mở được đúng một tệp, sống 60 giây, và không dùng được cho việc gì khác.
func (t *TokenIssuer) IssueDownloadTicket(userID uint, path string) (string, error) {
	now := time.Now()
	claims := DownloadClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "sunpanel",
			Subject:   "download",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(downloadTicketTTL)),
		},
		Path:   path,
		UserID: userID,
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(t.secret)
	if err != nil {
		return "", fmt.Errorf("ký vé tải xuống: %w", err)
	}
	return signed, nil
}

// ParseDownloadTicket kiểm tra vé và trả về đường dẫn nó cho phép tải.
func (t *TokenIssuer) ParseDownloadTicket(ticket string) (*DownloadClaims, error) {
	claims := &DownloadClaims{}

	_, err := jwt.ParseWithClaims(ticket, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("thuật toán ký không mong đợi: %v", token.Header["alg"])
		}
		return t.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return nil, apperr.Unauthorized.Wrap(err)
	}

	// Vé và access token dùng chung khóa ký, nên phải phân biệt bằng Subject —
	// nếu không, một access token bình thường cũng sẽ dùng được như vé.
	if claims.Subject != "download" {
		return nil, apperr.Unauthorized
	}

	return claims, nil
}
