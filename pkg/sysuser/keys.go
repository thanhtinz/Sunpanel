package sysuser

import (
	"errors"
	"strings"

	"golang.org/x/crypto/ssh"
)

// Các lỗi liên quan tới khóa SSH.
var (
	// ErrInvalidKey là chuỗi không phải một khóa công khai SSH hợp lệ.
	ErrInvalidKey = errors.New("sysuser: khóa công khai không hợp lệ")
	// ErrKeyExists là khóa đã có trong danh sách của tài khoản.
	ErrKeyExists = errors.New("sysuser: khóa đã tồn tại")
	// ErrKeyNotFound là không tìm thấy khóa cần xóa.
	ErrKeyNotFound = errors.New("sysuser: không tìm thấy khóa")
)

// Key là một khóa công khai trong tệp authorized_keys.
type Key struct {
	// Type là loại khóa, ví dụ ssh-ed25519.
	Type string `json:"type"`
	// Comment là phần chú thích cuối dòng, thường là "người@máy".
	Comment string `json:"comment"`
	// Fingerprint là dấu vân tay SHA256, thứ người dùng đối chiếu được với
	// `ssh-keygen -lf` trên máy của họ.
	Fingerprint string `json:"fingerprint"`
	// Line là nguyên văn dòng trong tệp, dùng làm định danh khi xóa.
	Line string `json:"line"`
}

// ParseKeys đọc nội dung một tệp authorized_keys.
//
// Dòng trống và dòng chú thích được bỏ qua. Dòng không phân tích được vẫn giữ
// nguyên trong tệp nhưng không hiện ra: panel không hiểu nó thì cũng không nên
// mời người dùng xóa một thứ có thể đang có tác dụng.
func ParseKeys(content string) []Key {
	keys := make([]Key, 0, 4)

	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parsed, comment, _, _, err := ssh.ParseAuthorizedKey([]byte(line))
		if err != nil {
			continue
		}

		keys = append(keys, Key{
			Type:        parsed.Type(),
			Comment:     comment,
			Fingerprint: ssh.FingerprintSHA256(parsed),
			Line:        line,
		})
	}
	return keys
}

// NormalizeKey kiểm tra một khóa người dùng dán vào và trả về dạng chuẩn.
//
// Người dùng thường dán cả tệp .pub kèm xuống dòng ở cuối, hoặc dán nhầm khóa
// riêng. Kiểm ngay tại đây để tệp authorized_keys không bao giờ chứa rác —
// một dòng hỏng trong tệp đó làm sshd bỏ qua toàn bộ phần còn lại.
func NormalizeKey(raw string) (Key, error) {
	line := strings.TrimSpace(raw)
	if line == "" {
		return Key{}, ErrInvalidKey
	}
	if strings.Contains(line, "PRIVATE KEY") {
		return Key{}, ErrInvalidKey
	}

	parsed, comment, _, _, err := ssh.ParseAuthorizedKey([]byte(line))
	if err != nil {
		return Key{}, ErrInvalidKey
	}

	// Ghi lại từ khóa đã phân tích chứ không dùng nguyên văn dòng người dùng dán:
	// như vậy khoảng trắng thừa và ký tự lạ không bao giờ vào tệp.
	normalized := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(parsed)))
	if comment != "" {
		normalized += " " + comment
	}

	return Key{
		Type:        parsed.Type(),
		Comment:     comment,
		Fingerprint: ssh.FingerprintSHA256(parsed),
		Line:        normalized,
	}, nil
}

// AddKey thêm một khóa vào nội dung tệp authorized_keys.
func AddKey(content string, key Key) (string, error) {
	for _, existing := range ParseKeys(content) {
		if existing.Fingerprint == key.Fingerprint {
			return "", ErrKeyExists
		}
	}

	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	if len(lines) == 1 && strings.TrimSpace(lines[0]) == "" {
		lines = nil
	}
	lines = append(lines, key.Line)
	return strings.Join(lines, "\n") + "\n", nil
}

// RemoveKey bỏ khóa có dấu vân tay tương ứng khỏi nội dung tệp.
//
// Đối chiếu theo dấu vân tay chứ không theo nguyên văn dòng: cùng một khóa có
// thể được ghi với phần chú thích khác nhau, và người dùng nghĩ theo "khóa nào"
// chứ không theo "dòng nào".
func RemoveKey(content, fingerprint string) (string, error) {
	out := make([]string, 0, 8)
	found := false

	for _, raw := range strings.Split(strings.TrimRight(content, "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if line != "" && !strings.HasPrefix(line, "#") {
			if parsed, _, _, _, err := ssh.ParseAuthorizedKey([]byte(line)); err == nil {
				if ssh.FingerprintSHA256(parsed) == fingerprint {
					found = true
					continue
				}
			}
		}
		out = append(out, raw)
	}

	if !found {
		return "", ErrKeyNotFound
	}

	joined := strings.TrimRight(strings.Join(out, "\n"), "\n")
	if joined == "" {
		return "", nil
	}
	return joined + "\n", nil
}
