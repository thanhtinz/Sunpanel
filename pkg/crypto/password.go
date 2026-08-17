// Package crypto cung cấp các nguyên hàm mật mã dùng chung cho SunPanel:
// băm mật khẩu (Argon2id), mã hóa bí mật at-rest (AES-GCM) và sinh chuỗi ngẫu nhiên.
package crypto

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Tham số Argon2id. Cân bằng giữa an toàn và việc panel thường chạy trên VPS nhỏ.
const (
	argonTime    = 3
	argonMemory  = 64 * 1024 // 64 MB
	argonThreads = 2
	argonKeyLen  = 32
	saltLen      = 16

	// Khoảng độ dài khóa được chấp nhận khi đọc lại một chuỗi hash.
	minKeyLen = 16
	maxKeyLen = 64
)

var (
	// ErrInvalidHash trả về khi chuỗi hash không đúng định dạng PHC.
	ErrInvalidHash = errors.New("crypto: định dạng hash không hợp lệ")
	// ErrIncompatibleVersion trả về khi hash được tạo bởi phiên bản Argon2 khác.
	ErrIncompatibleVersion = errors.New("crypto: phiên bản argon2 không tương thích")
)

// HashPassword băm mật khẩu bằng Argon2id, trả về chuỗi PHC tự mô tả tham số.
func HashPassword(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("sinh salt: %w", err)
	}

	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// VerifyPassword so khớp mật khẩu với chuỗi hash PHC theo thời gian hằng số.
func VerifyPassword(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, ErrInvalidHash
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, ErrInvalidHash
	}
	if version != argon2.Version {
		return false, ErrIncompatibleVersion
	}

	var memory, time uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return false, ErrInvalidHash
	}

	salt, err := base64.RawStdEncoding.Strict().DecodeString(parts[4])
	if err != nil {
		return false, ErrInvalidHash
	}
	want, err := base64.RawStdEncoding.Strict().DecodeString(parts[5])
	if err != nil {
		return false, ErrInvalidHash
	}

	// Độ dài khóa nằm trong chuỗi hash do chính ta sinh ra, nhưng một chuỗi bị sửa
	// đổi có thể khai báo độ dài bất kỳ. Kẹp vào khoảng hợp lý trước khi dùng:
	// vừa chặn tràn số khi ép kiểu, vừa chặn việc buộc máy chủ sinh khóa khổng lồ.
	if len(want) < minKeyLen || len(want) > maxKeyLen {
		return false, ErrInvalidHash
	}
	keyLen := uint32(len(want)) // #nosec G115 -- đã kẹp trong [minKeyLen, maxKeyLen] ngay trên

	got := argon2.IDKey([]byte(password), salt, time, memory, threads, keyLen)

	return subtle.ConstantTimeCompare(got, want) == 1, nil
}
