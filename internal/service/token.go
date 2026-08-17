// Package service chứa toàn bộ nghiệp vụ của panel.
package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/model"
)

// Claims là nội dung của access token.
type Claims struct {
	jwt.RegisteredClaims
	UserID   uint       `json:"uid"`
	Username string     `json:"usr"`
	Role     model.Role `json:"rol"`
	// SessionID buộc access token gắn với một phiên cụ thể, nhờ đó thu hồi phiên
	// cũng vô hiệu hóa được access token trước khi nó hết hạn.
	SessionID uint `json:"sid"`
}

// TokenIssuer phát hành và kiểm tra access token.
type TokenIssuer struct {
	secret []byte
	ttl    time.Duration
}

// NewTokenIssuer tạo bộ phát hành token với khóa ký và thời gian sống cho trước.
func NewTokenIssuer(secret string, ttl time.Duration) *TokenIssuer {
	return &TokenIssuer{secret: []byte(secret), ttl: ttl}
}

// TTL trả về thời gian sống của access token.
func (t *TokenIssuer) TTL() time.Duration { return t.ttl }

// Issue phát hành access token cho một người dùng trong một phiên.
func (t *TokenIssuer) Issue(user *model.User, sessionID uint) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(t.ttl)

	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "sunpanel",
			Subject:   fmt.Sprint(user.ID),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
		UserID:    user.ID,
		Username:  user.Username,
		Role:      user.Role,
		SessionID: sessionID,
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(t.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("ký token: %w", err)
	}
	return signed, expiresAt, nil
}

// Parse kiểm tra chữ ký và hạn của access token.
func (t *TokenIssuer) Parse(tokenString string) (*Claims, error) {
	claims := &Claims{}

	// Chỉ chấp nhận đúng thuật toán đã dùng để ký. Nếu bỏ qua bước này, kẻ tấn công
	// có thể gửi token dùng thuật toán "none" và được chấp nhận.
	_, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("thuật toán ký không mong đợi: %v", token.Header["alg"])
		}
		return t.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return nil, apperr.Unauthorized.Wrap(err)
	}

	return claims, nil
}

// HashToken băm refresh token để lưu vào cơ sở dữ liệu.
// Refresh token gốc chỉ tồn tại ở phía trình duyệt.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
