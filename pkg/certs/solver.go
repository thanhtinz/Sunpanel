package certs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// challengePrefix là đường dẫn URL mà máy chủ ACME sẽ truy cập để xác thực.
const challengePrefix = ".well-known/acme-challenge"

// validToken khớp một mã thử thách hợp lệ.
//
// Mã do máy chủ ACME cấp, nhưng vẫn phải kiểm: nó được ghép thẳng vào đường dẫn
// tệp, nên một máy chủ ACME bị chiếm quyền có thể dùng nó để ghi đè tệp bất kỳ
// trên hệ thống bằng nội dung do chính nó chọn.
var validToken = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

// WebrootSolver trả lời thử thách HTTP-01 bằng cách đặt tệp trong một thư mục
// mà máy chủ web phục vụ.
//
// Dùng một thư mục dùng chung cho mọi website thay vì thư mục gốc của từng
// website: website dạng chuyển tiếp không có thư mục gốc, và website tĩnh có thể
// đang trỏ vào nơi panel không được phép ghi.
type WebrootSolver struct {
	root string
}

// NewWebrootSolver tạo bộ giải thử thách ghi tệp vào thư mục root.
func NewWebrootSolver(root string) *WebrootSolver { return &WebrootSolver{root: root} }

// Root trả về thư mục gốc mà máy chủ web cần phục vụ.
func (s *WebrootSolver) Root() string { return s.root }

// path dựng đường dẫn tệp trả lời cho một mã thử thách.
func (s *WebrootSolver) path(token string) (string, error) {
	if !validToken.MatchString(token) {
		return "", fmt.Errorf("%w: mã thử thách không hợp lệ", ErrInvalidCertificate)
	}
	return filepath.Join(s.root, filepath.FromSlash(challengePrefix), token), nil
}

// Present đặt nội dung trả lời thử thách.
func (s *WebrootSolver) Present(_ context.Context, token, keyAuth string) error {
	file, err := s.path(token)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return fmt.Errorf("tạo thư mục xác thực: %w", err)
	}
	// Máy chủ web thường chạy dưới người dùng khác panel, nên tệp phải cho phép
	// mọi người đọc — nội dung của nó vốn là công khai.
	if err := os.WriteFile(file, []byte(keyAuth), 0o644); err != nil {
		return fmt.Errorf("ghi tệp xác thực: %w", err)
	}
	return nil
}

// CleanUp xóa nội dung trả lời thử thách.
func (s *WebrootSolver) CleanUp(_ context.Context, token string) error {
	file, err := s.path(token)
	if err != nil {
		return err
	}
	if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("xóa tệp xác thực: %w", err)
	}
	return nil
}
