// Package apperr định nghĩa lỗi có mã định danh để giao diện tự dịch.
//
// Backend không bao giờ trả về câu tiếng Anh hay tiếng Việt cho người dùng cuối:
// nó trả về một mã như "auth.invalid_credentials", còn frontend tra bảng ngôn ngữ
// tương ứng. Nhờ vậy thêm một ngôn ngữ mới chỉ là thêm một tệp JSON.
package apperr

import (
	"errors"
	"fmt"
	"net/http"
)

// Error là lỗi mang theo mã dịch và mã trạng thái HTTP.
type Error struct {
	// Code là mã dịch, ví dụ "auth.invalid_credentials".
	Code string
	// Status là mã trạng thái HTTP tương ứng.
	Status int
	// Params là các tham số chèn vào chuỗi dịch, ví dụ {"max": 5}.
	Params map[string]any
	// Err là lỗi gốc, chỉ dùng để ghi log, không gửi cho người dùng.
	Err error
}

// Error hiện thực interface error.
func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Code, e.Err)
	}
	return e.Code
}

// Unwrap cho phép errors.Is và errors.As đi tới lỗi gốc.
func (e *Error) Unwrap() error { return e.Err }

// WithParam gắn thêm một tham số cho chuỗi dịch.
func (e *Error) WithParam(key string, value any) *Error {
	if e.Params == nil {
		e.Params = make(map[string]any)
	}
	e.Params[key] = value
	return e
}

// Wrap gắn lỗi gốc để phục vụ ghi log.
func (e *Error) Wrap(err error) *Error {
	e.Err = err
	return e
}

// New tạo lỗi mới với mã dịch và mã trạng thái HTTP.
func New(status int, code string) *Error {
	return &Error{Code: code, Status: status}
}

// From lấy *Error từ một error bất kỳ. Lỗi không xác định được quy về lỗi nội bộ
// để không rò rỉ chi tiết cài đặt ra ngoài.
func From(err error) *Error {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr
	}
	return Internal.Wrap(err)
}

// Các lỗi dùng chung.
var (
	// Internal là lỗi không lường trước ở phía máy chủ.
	Internal = New(http.StatusInternalServerError, "error.internal")
	// BadRequest là dữ liệu gửi lên không hợp lệ.
	BadRequest = New(http.StatusBadRequest, "error.bad_request")
	// NotFound là không tìm thấy tài nguyên.
	NotFound = New(http.StatusNotFound, "error.not_found")
	// Forbidden là không đủ quyền.
	Forbidden = New(http.StatusForbidden, "error.forbidden")
	// Unauthorized là chưa đăng nhập hoặc token không hợp lệ.
	Unauthorized = New(http.StatusUnauthorized, "error.unauthorized")
	// TooManyRequests là vượt quá giới hạn tần suất.
	TooManyRequests = New(http.StatusTooManyRequests, "error.too_many_requests")
)

// Các lỗi của luồng xác thực.
var (
	// InvalidCredentials cố ý không phân biệt sai tên đăng nhập hay sai mật khẩu,
	// để kẻ tấn công không dò được tài khoản nào tồn tại.
	InvalidCredentials = New(http.StatusUnauthorized, "auth.invalid_credentials")
	// AccountLocked là tài khoản đang bị khóa do đăng nhập sai quá nhiều lần.
	AccountLocked = New(http.StatusForbidden, "auth.account_locked")
	// AccountDisabled là tài khoản đã bị vô hiệu hóa.
	AccountDisabled = New(http.StatusForbidden, "auth.account_disabled")
	// TOTPRequired báo cho giao diện biết cần hỏi thêm mã xác thực hai lớp.
	TOTPRequired = New(http.StatusUnauthorized, "auth.totp_required")
	// TOTPInvalid là mã xác thực hai lớp không đúng.
	TOTPInvalid = New(http.StatusUnauthorized, "auth.totp_invalid")
	// TOTPAlreadyEnabled là đã bật 2FA rồi.
	TOTPAlreadyEnabled = New(http.StatusConflict, "auth.totp_already_enabled")
	// SessionExpired là phiên đã hết hạn hoặc bị thu hồi.
	SessionExpired = New(http.StatusUnauthorized, "auth.session_expired")
	// PasswordTooWeak là mật khẩu không đạt yêu cầu tối thiểu.
	PasswordTooWeak = New(http.StatusBadRequest, "auth.password_too_weak")
	// PasswordMismatch là mật khẩu hiện tại không đúng.
	PasswordMismatch = New(http.StatusBadRequest, "auth.password_mismatch")
)

// Các lỗi quản lý người dùng.
var (
	// UserNotFound là không tìm thấy người dùng.
	UserNotFound = New(http.StatusNotFound, "user.not_found")
	// UsernameTaken là tên đăng nhập đã tồn tại.
	UsernameTaken = New(http.StatusConflict, "user.username_taken")
	// InvalidRole là vai trò không hợp lệ.
	InvalidRole = New(http.StatusBadRequest, "user.invalid_role")
	// CannotDeleteSelf ngăn người dùng tự xóa tài khoản đang đăng nhập.
	CannotDeleteSelf = New(http.StatusBadRequest, "user.cannot_delete_self")
	// LastAdmin ngăn xóa hoặc hạ quyền quản trị viên cuối cùng, tránh khóa chính mình ra ngoài.
	LastAdmin = New(http.StatusBadRequest, "user.last_admin")
)

// Các lỗi của trình quản lý tệp.
var (
	// FileNotFound là không tìm thấy tệp hoặc thư mục.
	FileNotFound = New(http.StatusNotFound, "file.not_found")
	// FilePermissionDenied là hệ điều hành từ chối thao tác.
	FilePermissionDenied = New(http.StatusForbidden, "file.permission_denied")
	// FileAlreadyExists là mục đích đã tồn tại.
	FileAlreadyExists = New(http.StatusConflict, "file.already_exists")
	// FileIsDirectory là thao tác chỉ áp dụng cho tệp nhưng đối tượng là thư mục.
	FileIsDirectory = New(http.StatusBadRequest, "file.is_directory")
	// FileDirectoryNotEmpty là thư mục còn nội dung bên trong.
	FileDirectoryNotEmpty = New(http.StatusConflict, "file.directory_not_empty")
	// FileTooLarge là tệp vượt quá giới hạn cho phép.
	FileTooLarge = New(http.StatusRequestEntityTooLarge, "file.too_large")
	// FileNotText là tệp nhị phân, không mở được bằng trình soạn thảo văn bản.
	FileNotText = New(http.StatusBadRequest, "file.not_text")
	// FileInvalidName là tên tệp không hợp lệ.
	FileInvalidName = New(http.StatusBadRequest, "file.invalid_name")
	// FileInvalidMode là chuỗi quyền truy cập không hợp lệ.
	FileInvalidMode = New(http.StatusBadRequest, "file.invalid_mode")
	// FileUnsupportedFormat là định dạng nén không được hỗ trợ.
	FileUnsupportedFormat = New(http.StatusBadRequest, "file.unsupported_format")
	// FileCorruptArchive là tệp nén hỏng hoặc sai định dạng.
	FileCorruptArchive = New(http.StatusBadRequest, "file.corrupt_archive")
	// FileUnsafeArchive là tệp nén chứa mục cố ghi ra ngoài thư mục đích.
	FileUnsafeArchive = New(http.StatusBadRequest, "file.unsafe_archive")
)
