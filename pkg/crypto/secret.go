package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// ErrCiphertextTooShort trả về khi dữ liệu mã hóa ngắn hơn nonce.
var ErrCiphertextTooShort = errors.New("crypto: bản mã quá ngắn")

// Sealer mã hóa/giải mã các bí mật lưu trong DB (mật khẩu DB, token cloud, TOTP secret)
// bằng AES-256-GCM với khóa chủ sinh lúc cài đặt.
type Sealer struct {
	aead cipher.AEAD
}

// NewSealer tạo Sealer từ khóa chủ 32 byte.
func NewSealer(masterKey []byte) (*Sealer, error) {
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, fmt.Errorf("khởi tạo aes: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("khởi tạo gcm: %w", err)
	}
	return &Sealer{aead: aead}, nil
}

// Seal mã hóa plaintext, trả về chuỗi base64 gồm nonce ghép bản mã.
func (s *Sealer) Seal(plaintext string) (string, error) {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("sinh nonce: %w", err)
	}
	sealed := s.aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Open giải mã chuỗi do Seal tạo ra.
func (s *Sealer) Open(encoded string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("giải mã base64: %w", err)
	}
	ns := s.aead.NonceSize()
	if len(raw) < ns {
		return "", ErrCiphertextTooShort
	}
	plaintext, err := s.aead.Open(nil, raw[:ns], raw[ns:], nil)
	if err != nil {
		return "", fmt.Errorf("giải mã gcm: %w", err)
	}
	return string(plaintext), nil
}
