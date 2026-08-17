package crypto

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
)

const (
	// Bỏ các ký tự dễ nhìn nhầm (0/O, 1/l/I) để người dùng gõ lại được từ console.
	unambiguousAlphabet = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	passwordAlphabet    = unambiguousAlphabet + "!@#%^&*-_=+"
)

// RandomString sinh chuỗi ngẫu nhiên an toàn từ tập ký tự không gây nhầm lẫn.
// Dùng cho entry path bí mật và tên định danh.
func RandomString(n int) (string, error) {
	return randomFrom(unambiguousAlphabet, n)
}

// RandomPassword sinh mật khẩu ngẫu nhiên mạnh cho tài khoản admin khởi tạo.
func RandomPassword(n int) (string, error) {
	return randomFrom(passwordAlphabet, n)
}

// RandomHex sinh n byte ngẫu nhiên dạng hex. Dùng cho khóa ký JWT và token.
func RandomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("đọc ngẫu nhiên: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func randomFrom(alphabet string, n int) (string, error) {
	max := big.NewInt(int64(len(alphabet)))
	out := make([]byte, n)
	for i := range out {
		idx, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", fmt.Errorf("đọc ngẫu nhiên: %w", err)
		}
		out[i] = alphabet[idx.Int64()]
	}
	return string(out), nil
}
