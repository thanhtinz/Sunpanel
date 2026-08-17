package middleware

import (
	"net/http"
	"strings"
)

// StripEntryPath bọc handler bên trong bằng một lớp buộc mọi yêu cầu đi qua một
// đường dẫn bí mật.
//
// Đây là lớp phòng thủ chống công cụ quét tự động: kẻ tấn công thấy cổng mở
// nhưng không đoán được đường dẫn thì chỉ nhận 404 ở mọi URL. Nó không thay thế
// xác thực, mà cắt bỏ phần lớn lưu lượng dò quét trước khi tới lớp đó.
//
// Lớp này bắt buộc phải nằm NGOÀI engine chứ không phải là middleware bên trong:
// bộ định tuyến khớp đường dẫn trước khi middleware chạy, nên việc viết lại
// đường dẫn từ bên trong sẽ không có tác dụng.
//
// Trả về nguyên handler khi chưa cấu hình đường dẫn bí mật.
func StripEntryPath(entryPath string, next http.Handler) http.Handler {
	prefix := normalizeEntryPath(entryPath)
	if prefix == "" {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if path == prefix {
			// Chuyển hướng về dạng có dấu gạch cuối để đường dẫn tương đối của giao
			// diện được phân giải đúng.
			http.Redirect(w, r, prefix+"/", http.StatusFound)
			return
		}

		if !strings.HasPrefix(path, prefix+"/") {
			// Cố ý trả 404 chứ không phải 403: phản hồi phải không phân biệt được
			// với một máy chủ không hề chạy panel.
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Sao chép yêu cầu thay vì sửa tại chỗ — sửa trực tiếp đối tượng gốc là
		// hành vi không an toàn theo tài liệu của net/http.
		stripped := r.Clone(r.Context())
		stripped.URL.Path = strings.TrimPrefix(path, prefix)
		if stripped.URL.Path == "" {
			stripped.URL.Path = "/"
		}
		if stripped.URL.RawPath != "" {
			stripped.URL.RawPath = strings.TrimPrefix(stripped.URL.RawPath, prefix)
		}

		next.ServeHTTP(w, stripped)
	})
}

// normalizeEntryPath đưa đường dẫn về dạng "/abc" (có gạch đầu, không gạch cuối).
func normalizeEntryPath(entryPath string) string {
	p := strings.Trim(strings.TrimSpace(entryPath), "/")
	if p == "" {
		return ""
	}
	return "/" + p
}
