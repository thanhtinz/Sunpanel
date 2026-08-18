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

// Các lỗi của terminal.
var (
	// TerminalNotSupported là nền tảng hiện tại chưa mở được phiên terminal.
	TerminalNotSupported = New(http.StatusNotImplemented, "terminal.not_supported")
)

// Các lỗi của quản lý dịch vụ hệ thống.
var (
	// ServiceManagerUnavailable là máy chủ không có trình quản lý dịch vụ dùng được.
	ServiceManagerUnavailable = New(http.StatusServiceUnavailable, "service.manager_unavailable")
	// ServiceNotFound là không tìm thấy dịch vụ.
	ServiceNotFound = New(http.StatusNotFound, "service.not_found")
	// ServiceInvalidName là tên dịch vụ không hợp lệ.
	ServiceInvalidName = New(http.StatusBadRequest, "service.invalid_name")
	// ServiceActionFailed là thao tác điều khiển dịch vụ thất bại.
	ServiceActionFailed = New(http.StatusInternalServerError, "service.action_failed")
	// ServiceProtected là dịch vụ được panel bảo vệ, không cho dừng hoặc tắt.
	ServiceProtected = New(http.StatusForbidden, "service.protected")
)

// Các lỗi của bảng tiến trình.
var (
	// ProcessNotFound là tiến trình đã kết thúc trước khi thao tác kịp chạy.
	ProcessNotFound = New(http.StatusNotFound, "process.not_found")
	// ProcessProtected là tiến trình panel từ chối kết thúc.
	ProcessProtected = New(http.StatusForbidden, "process.protected")
	// ProcessKillFailed là gửi tín hiệu kết thúc thất bại.
	ProcessKillFailed = New(http.StatusInternalServerError, "process.kill_failed")
	// ProcessListFailed là không đọc được bảng tiến trình của hệ điều hành.
	ProcessListFailed = New(http.StatusInternalServerError, "process.list_failed")
)

// Các lỗi của tác vụ định kỳ.
var (
	// CronJobNotFound là không tìm thấy tác vụ.
	CronJobNotFound = New(http.StatusNotFound, "cron.not_found")
	// CronInvalidSchedule là biểu thức lịch không hợp lệ.
	CronInvalidSchedule = New(http.StatusBadRequest, "cron.invalid_schedule")
)

// Các lỗi của tường lửa.
var (
	// FirewallUnavailable là máy chủ không có công cụ tường lửa khả dụng.
	FirewallUnavailable = New(http.StatusServiceUnavailable, "firewall.unavailable")
	// FirewallInvalidRule là quy tắc không hợp lệ.
	FirewallInvalidRule = New(http.StatusBadRequest, "firewall.invalid_rule")
	// FirewallRuleNotFound là không tìm thấy quy tắc cần xóa.
	FirewallRuleNotFound = New(http.StatusNotFound, "firewall.rule_not_found")
	// FirewallActionFailed là thao tác với tường lửa thất bại.
	FirewallActionFailed = New(http.StatusInternalServerError, "firewall.action_failed")
	// FirewallCriticalPort là quy tắc sẽ chặn cổng khiến quản trị viên mất đường vào máy chủ.
	FirewallCriticalPort = New(http.StatusForbidden, "firewall.critical_port")
)

// Các lỗi của website.
var (
	// WebsiteNotFound là không tìm thấy website.
	WebsiteNotFound = New(http.StatusNotFound, "website.not_found")
	// WebsiteNameExists là tên website đã được dùng.
	WebsiteNameExists = New(http.StatusConflict, "website.name_exists")
	// WebsiteInvalidName là tên website không hợp lệ.
	WebsiteInvalidName = New(http.StatusBadRequest, "website.invalid_name")
	// WebsiteInvalidDomain là tên miền không hợp lệ.
	WebsiteInvalidDomain = New(http.StatusBadRequest, "website.invalid_domain")
	// WebsiteDomainTaken là tên miền đã thuộc về một website khác.
	WebsiteDomainTaken = New(http.StatusConflict, "website.domain_taken")
	// WebsiteInvalidConfig là cấu hình website không hợp lệ.
	WebsiteInvalidConfig = New(http.StatusBadRequest, "website.invalid_config")
	// WebsiteServerUnavailable là máy chủ không có phần mềm web server khả dụng.
	WebsiteServerUnavailable = New(http.StatusServiceUnavailable, "website.server_unavailable")
	// WebsiteApplyFailed là máy chủ web từ chối cấu hình sinh ra.
	WebsiteApplyFailed = New(http.StatusInternalServerError, "website.apply_failed")
)

// Các lỗi của chứng chỉ TLS.
var (
	// CertNotFound là không tìm thấy chứng chỉ.
	CertNotFound = New(http.StatusNotFound, "cert.not_found")
	// CertNameExists là tên chứng chỉ đã được dùng.
	CertNameExists = New(http.StatusConflict, "cert.name_exists")
	// CertInvalidName là tên chứng chỉ không hợp lệ.
	CertInvalidName = New(http.StatusBadRequest, "cert.invalid_name")
	// CertInvalid là dữ liệu chứng chỉ hoặc khóa không hợp lệ.
	CertInvalid = New(http.StatusBadRequest, "cert.invalid")
	// CertKeyMismatch là khóa riêng không khớp với chứng chỉ.
	CertKeyMismatch = New(http.StatusBadRequest, "cert.key_mismatch")
	// CertInvalidEmail là địa chỉ email đăng ký ACME không hợp lệ.
	CertInvalidEmail = New(http.StatusBadRequest, "cert.invalid_email")
	// CertIssueFailed là việc xin chứng chỉ từ máy chủ ACME thất bại.
	CertIssueFailed = New(http.StatusBadGateway, "cert.issue_failed")
	// CertInUse là chứng chỉ đang được website sử dụng nên không xóa được.
	CertInUse = New(http.StatusConflict, "cert.in_use")
)

// Các lỗi của chợ ứng dụng.
var (
	// AppNotFound là không tìm thấy ứng dụng đã cài.
	AppNotFound = New(http.StatusNotFound, "app.not_found")
	// AppCatalogNotFound là không tìm thấy ứng dụng trong danh mục.
	AppCatalogNotFound = New(http.StatusNotFound, "app.catalog_not_found")
	// AppNameExists là tên ứng dụng đã được dùng.
	AppNameExists = New(http.StatusConflict, "app.name_exists")
	// AppInvalidName là tên ứng dụng không hợp lệ.
	AppInvalidName = New(http.StatusBadRequest, "app.invalid_name")
	// AppInvalidValue là giá trị cấu hình không hợp lệ.
	AppInvalidValue = New(http.StatusBadRequest, "app.invalid_value")
	// AppDirExists là thư mục cài đặt đã tồn tại từ lần cài trước.
	AppDirExists = New(http.StatusConflict, "app.dir_exists")
	// AppPortInUse là cổng đã có tiến trình khác chiếm.
	AppPortInUse = New(http.StatusConflict, "app.port_in_use")
	// AppComposeUnavailable là máy chủ không có Docker Compose.
	AppComposeUnavailable = New(http.StatusServiceUnavailable, "app.compose_unavailable")
	// AppInstallFailed là việc cài đặt thất bại.
	AppInstallFailed = New(http.StatusInternalServerError, "app.install_failed")
	// AppActionFailed là thao tác với ứng dụng thất bại.
	AppActionFailed = New(http.StatusInternalServerError, "app.action_failed")
)

// Các lỗi của cơ sở dữ liệu.
var (
	// DBServerNotFound là không tìm thấy máy chủ cơ sở dữ liệu.
	DBServerNotFound = New(http.StatusNotFound, "db.server_not_found")
	// DBServerNameExists là tên máy chủ đã được dùng.
	DBServerNameExists = New(http.StatusConflict, "db.server_name_exists")
	// DBInvalidKind là loại cơ sở dữ liệu không được hỗ trợ.
	DBInvalidKind = New(http.StatusBadRequest, "db.invalid_kind")
	// DBInvalidName là tên cơ sở dữ liệu hoặc tài khoản không hợp lệ.
	DBInvalidName = New(http.StatusBadRequest, "db.invalid_name")
	// DBConnectFailed là không kết nối được tới máy chủ.
	DBConnectFailed = New(http.StatusBadGateway, "db.connect_failed")
	// DBProtected là đối tượng hệ thống, không cho thao tác.
	DBProtected = New(http.StatusForbidden, "db.protected")
	// DBQueryFailed là câu lệnh SQL thất bại.
	DBQueryFailed = New(http.StatusBadRequest, "db.query_failed")
	// DBActionFailed là thao tác với cơ sở dữ liệu thất bại.
	DBActionFailed = New(http.StatusInternalServerError, "db.action_failed")
)

// Các lỗi của sao lưu.
var (
	// BackupPlanNotFound là không tìm thấy kế hoạch sao lưu.
	BackupPlanNotFound = New(http.StatusNotFound, "backup.not_found")
	// BackupNameExists là tên kế hoạch đã được dùng.
	BackupNameExists = New(http.StatusConflict, "backup.name_exists")
	// BackupInvalidConfig là cấu hình kế hoạch không hợp lệ.
	BackupInvalidConfig = New(http.StatusBadRequest, "backup.invalid_config")
	// BackupInvalidSchedule là biểu thức lịch không hợp lệ.
	BackupInvalidSchedule = New(http.StatusBadRequest, "backup.invalid_schedule")
	// BackupDestinationFailed là nơi lưu trữ không dùng được.
	BackupDestinationFailed = New(http.StatusBadGateway, "backup.destination_failed")
	// BackupDumpUnavailable là máy chủ thiếu công cụ kết xuất cơ sở dữ liệu.
	BackupDumpUnavailable = New(http.StatusServiceUnavailable, "backup.dump_unavailable")
	// BackupFailed là lần sao lưu thất bại.
	BackupFailed = New(http.StatusInternalServerError, "backup.failed")
	// BackupRestoreFailed là việc khôi phục thất bại.
	BackupRestoreFailed = New(http.StatusInternalServerError, "backup.restore_failed")
	// BackupObjectNotFound là không tìm thấy tệp sao lưu ở nơi lưu trữ.
	BackupObjectNotFound = New(http.StatusNotFound, "backup.object_not_found")
)

// Các lỗi của cảnh báo.
var (
	// AlertChannelNotFound là không tìm thấy kênh thông báo.
	AlertChannelNotFound = New(http.StatusNotFound, "alert.channel_not_found")
	// AlertChannelNameExists là tên kênh đã được dùng.
	AlertChannelNameExists = New(http.StatusConflict, "alert.channel_name_exists")
	// AlertInvalidKind là loại kênh không được hỗ trợ.
	AlertInvalidKind = New(http.StatusBadRequest, "alert.invalid_kind")
	// AlertInvalidConfig là cấu hình kênh hoặc quy tắc không hợp lệ.
	AlertInvalidConfig = New(http.StatusBadRequest, "alert.invalid_config")
	// AlertSendFailed là gửi thông báo thất bại.
	AlertSendFailed = New(http.StatusBadGateway, "alert.send_failed")
	// AlertRuleNotFound là không tìm thấy quy tắc cảnh báo.
	AlertRuleNotFound = New(http.StatusNotFound, "alert.rule_not_found")
	// AlertRuleNameExists là tên quy tắc đã được dùng.
	AlertRuleNameExists = New(http.StatusConflict, "alert.rule_name_exists")
)

// Các lỗi của khóa API.
var (
	// APIKeyNotFound là không tìm thấy khóa.
	APIKeyNotFound = New(http.StatusNotFound, "apikey.not_found")
	// APIKeyInvalid là khóa không hợp lệ, đã tắt hoặc đã hết hạn.
	APIKeyInvalid = New(http.StatusUnauthorized, "apikey.invalid")
	// APIKeyInvalidRole là vai trò yêu cầu cao hơn quyền của chủ sở hữu.
	APIKeyInvalidRole = New(http.StatusForbidden, "apikey.invalid_role")
)

// Các lỗi của node.
var (
	// NodeNotFound là không tìm thấy node.
	NodeNotFound = New(http.StatusNotFound, "node.not_found")
	// NodeNameExists là tên node đã được dùng.
	NodeNameExists = New(http.StatusConflict, "node.name_exists")
	// NodeInvalidAddress là địa chỉ agent không hợp lệ.
	NodeInvalidAddress = New(http.StatusBadRequest, "node.invalid_address")
	// NodeUnauthorized là token không đúng với agent.
	//
	// Tách khỏi NodeUnreachable vì hai lỗi này cần hai cách sửa khác nhau: một
	// bên là kiểm tra mạng và tường lửa, bên kia là chép lại token.
	NodeUnauthorized = New(http.StatusUnauthorized, "node.unauthorized")
	// NodeUnreachable là không kết nối được tới agent.
	NodeUnreachable = New(http.StatusBadGateway, "node.unreachable")
	// NodeVersionMismatch là phiên bản agent không khớp với panel.
	NodeVersionMismatch = New(http.StatusBadGateway, "node.version_mismatch")
)

// Các lỗi của plugin.
var (
	// PluginNotFound là không tìm thấy plugin hoặc plugin đang tắt.
	PluginNotFound = New(http.StatusNotFound, "plugin.not_found")
	// PluginInvalid là tệp khai báo plugin không hợp lệ.
	PluginInvalid = New(http.StatusBadRequest, "plugin.invalid")
	// PluginUnreachable là không gọi được dịch vụ plugin.
	PluginUnreachable = New(http.StatusBadGateway, "plugin.unreachable")
)

// Các lỗi của Docker.
var (
	// DockerUnavailable là không kết nối được tới Docker.
	DockerUnavailable = New(http.StatusServiceUnavailable, "docker.unavailable")
	// DockerNotFound là không tìm thấy đối tượng.
	DockerNotFound = New(http.StatusNotFound, "docker.not_found")
	// DockerConflict là thao tác xung đột với trạng thái hiện tại.
	DockerConflict = New(http.StatusConflict, "docker.conflict")
	// DockerActionFailed là thao tác với Docker thất bại.
	DockerActionFailed = New(http.StatusInternalServerError, "docker.action_failed")
	// DockerSelfProtected là container đang chạy chính panel, không cho dừng hoặc xóa.
	DockerSelfProtected = New(http.StatusForbidden, "docker.self_protected")
)
