package dockerx

import (
	"fmt"
	"regexp"
)

// validObjectID khớp ID hoặc tên hợp lệ của đối tượng Docker.
//
// Giá trị này được ghép thẳng vào đường dẫn URL gửi tới Docker. Chấp nhận chuỗi
// tùy ý ở đây cho phép chèn thêm đoạn đường dẫn hoặc tham số truy vấn, tức là
// gọi được sang endpoint khác của Docker — mà Docker thì có quyền tương đương root.
var validObjectID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$`)

// validateID kiểm tra ID hoặc tên đối tượng trước khi đưa vào đường dẫn API.
func validateID(id string) error {
	if !validObjectID.MatchString(id) {
		return fmt.Errorf("dockerx: định danh không hợp lệ: %q", id)
	}
	return nil
}

// validImageRef khớp tham chiếu image, ví dụ "nginx", "nginx:1.25",
// "ghcr.io/owner/app:tag" hoặc dạng có cổng registry.
var validImageRef = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.\-/:@]{0,255}$`)

// validateImageRef kiểm tra tham chiếu image.
func validateImageRef(ref string) error {
	if !validImageRef.MatchString(ref) {
		return fmt.Errorf("dockerx: tham chiếu image không hợp lệ: %q", ref)
	}
	return nil
}
