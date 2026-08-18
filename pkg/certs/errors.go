package certs

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// errorsAs bọc errors.As để các tệp khác không phải import errors chỉ vì một lời gọi.
func errorsAs(err error, target any) bool { return errors.As(err, target) }

// acmeDomain khớp tên miền mà Let's Encrypt chấp nhận.
//
// Khác với tên miền của máy chủ web, ở đây không cho phép "localhost" hay ký tự
// đại diện qua HTTP-01: Let's Encrypt chỉ cấp chứng chỉ đại diện qua DNS-01.
var acmeDomain = regexp.MustCompile(
	`^([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,63}$`,
)

// validateACMEDomain kiểm tra tên miền trước khi gửi lên máy chủ ACME.
func validateACMEDomain(domain string) error {
	name := strings.TrimSpace(strings.ToLower(domain))
	if !acmeDomain.MatchString(name) {
		return fmt.Errorf("%w: %q không phải tên miền công khai hợp lệ", ErrInvalidName, domain)
	}
	return nil
}
