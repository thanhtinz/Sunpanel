// Package webserver sinh và quản lý cấu hình máy chủ web cho từng website.
//
// Panel không sửa tay tệp cấu hình có sẵn của người dùng: mỗi website là một tệp
// riêng trong thư mục dành cho panel. Nhờ vậy gỡ một website là xóa đúng một tệp,
// và cấu hình do quản trị viên tự viết không bao giờ bị đụng tới.
package webserver

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Các lỗi dùng chung.
var (
	// ErrUnavailable là máy chủ không có phần mềm web server nào dùng được.
	ErrUnavailable = errors.New("webserver: không tìm thấy máy chủ web khả dụng")
	// ErrInvalidConfig là cấu hình sinh ra không hợp lệ.
	ErrInvalidConfig = errors.New("webserver: cấu hình không hợp lệ")
	// ErrInvalidDomain là tên miền không hợp lệ.
	ErrInvalidDomain = errors.New("webserver: tên miền không hợp lệ")
	// ErrSiteNotFound là không tìm thấy website.
	ErrSiteNotFound = errors.New("webserver: không tìm thấy website")
)

// SiteType là loại website.
type SiteType string

// Các loại website được hỗ trợ.
const (
	// SiteStatic phục vụ tệp tĩnh từ một thư mục.
	SiteStatic SiteType = "static"
	// SiteProxy chuyển tiếp yêu cầu tới một dịch vụ khác.
	SiteProxy SiteType = "proxy"
	// SitePHP phục vụ ứng dụng PHP qua PHP-FPM.
	SitePHP SiteType = "php"
)

// Valid cho biết loại website có được hỗ trợ hay không.
func (t SiteType) Valid() bool {
	switch t {
	case SiteStatic, SiteProxy, SitePHP:
		return true
	}
	return false
}

// Site là một website mà panel quản lý.
type Site struct {
	// Name là định danh dùng làm tên tệp cấu hình.
	Name string
	// Domains là danh sách tên miền phục vụ website này.
	Domains []string
	Type    SiteType
	// Root là thư mục gốc, dùng với website tĩnh và PHP.
	Root string
	// ProxyTarget là địa chỉ đích, dùng với website dạng chuyển tiếp.
	ProxyTarget string
	// PHPSocket là đường dẫn socket hoặc địa chỉ của PHP-FPM.
	PHPSocket string
	// Port là cổng HTTP; để trống dùng 80.
	Port int
	// SSL bật HTTPS cho website.
	SSL SSLConfig
	// ExtraConfig là các dòng cấu hình do người dùng thêm.
	ExtraConfig string
	// ACMEWebroot là thư mục dùng chung để phục vụ tệp xác thực ACME.
	//
	// Mọi website đều trỏ /.well-known/acme-challenge/ về đây, kể cả website dạng
	// chuyển tiếp: nếu để đường dẫn đó đi vào proxy_pass thì Let's Encrypt sẽ nhận
	// phản hồi của ứng dụng phía sau thay vì tệp xác thực, và việc gia hạn chứng
	// chỉ thất bại lặng lẽ cho tới ngày website hết hạn HTTPS.
	ACMEWebroot string
	// IPv6 quyết định có sinh các chỉ thị lắng nghe IPv6 hay không.
	//
	// Không phải máy chủ nào cũng bật IPv6. Trên máy không có, nginx không chỉ bỏ
	// qua chỉ thị đó mà từ chối khởi động hẳn, kéo theo mọi website khác cùng sập.
	IPv6 bool
}

// SSLConfig là cấu hình HTTPS của một website.
type SSLConfig struct {
	Enabled  bool
	CertFile string
	KeyFile  string
	// ForceHTTPS chuyển hướng mọi yêu cầu HTTP sang HTTPS.
	ForceHTTPS bool
	// Port là cổng HTTPS; để trống dùng 443.
	Port int
}

// Manager quản lý cấu hình máy chủ web.
type Manager interface {
	// Name là tên phần mềm máy chủ web.
	Name() string
	// Available cho biết máy chủ web có dùng được không.
	Available(ctx context.Context) bool
	// Apply ghi cấu hình cho một website và nạp lại máy chủ web.
	Apply(ctx context.Context, site Site) error
	// Remove gỡ cấu hình của một website.
	Remove(ctx context.Context, name string) error
	// Render sinh nội dung cấu hình mà không ghi ra đĩa.
	Render(site Site) (string, error)
	// Test kiểm tra toàn bộ cấu hình hiện tại có hợp lệ không.
	Test(ctx context.Context) error
	// Reload nạp lại cấu hình.
	Reload(ctx context.Context) error
}

// validDomain khớp một tên miền hợp lệ, cho phép cả ký tự đại diện ở nhãn đầu.
//
// Tên miền được ghi thẳng vào tệp cấu hình của máy chủ web, nên phải kiểm chặt:
// một chuỗi chứa dấu chấm phẩy hoặc xuống dòng sẽ chèn được chỉ thị tùy ý vào
// cấu hình, và máy chủ web chạy quyền root.
var validDomain = regexp.MustCompile(
	`^(\*\.)?([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,63}$`,
)

// validSiteName khớp tên website, vốn được dùng làm tên tệp cấu hình.
var validSiteName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`)

// ValidateDomain kiểm tra một tên miền.
func ValidateDomain(domain string) error {
	name := strings.TrimSpace(strings.ToLower(domain))

	// localhost không có phần đuôi nên không khớp biểu thức chung, nhưng là tên
	// hợp lệ và hay dùng khi thử nghiệm tại chỗ.
	if name == "localhost" {
		return nil
	}
	if !validDomain.MatchString(name) {
		return fmt.Errorf("%w: %q", ErrInvalidDomain, domain)
	}
	return nil
}

// ValidateSite kiểm tra toàn bộ thông tin của một website.
func ValidateSite(site Site) error {
	if !validSiteName.MatchString(site.Name) {
		return fmt.Errorf("%w: tên website %q", ErrInvalidConfig, site.Name)
	}
	if !site.Type.Valid() {
		return fmt.Errorf("%w: loại website %q", ErrInvalidConfig, site.Type)
	}
	if len(site.Domains) == 0 {
		return fmt.Errorf("%w: website phải có ít nhất một tên miền", ErrInvalidConfig)
	}

	for _, domain := range site.Domains {
		if err := ValidateDomain(domain); err != nil {
			return err
		}
	}

	switch site.Type {
	case SiteStatic, SitePHP:
		if strings.TrimSpace(site.Root) == "" {
			return fmt.Errorf("%w: website tĩnh và PHP phải có thư mục gốc", ErrInvalidConfig)
		}
		if err := validateConfigValue(site.Root); err != nil {
			return err
		}
	case SiteProxy:
		if err := validateProxyTarget(site.ProxyTarget); err != nil {
			return err
		}
	}

	if site.SSL.Enabled {
		for _, path := range []string{site.SSL.CertFile, site.SSL.KeyFile} {
			if strings.TrimSpace(path) == "" {
				return fmt.Errorf("%w: bật SSL thì phải có cả chứng chỉ lẫn khóa", ErrInvalidConfig)
			}
			if err := validateConfigValue(path); err != nil {
				return err
			}
		}
	}

	return nil
}

// validProxyTarget khớp địa chỉ đích của reverse proxy.
var validProxyTarget = regexp.MustCompile(`^https?://[a-zA-Z0-9._-]+(:[0-9]{1,5})?(/[a-zA-Z0-9._~/-]*)?$`)

func validateProxyTarget(target string) error {
	if !validProxyTarget.MatchString(strings.TrimSpace(target)) {
		return fmt.Errorf("%w: địa chỉ đích %q", ErrInvalidConfig, target)
	}
	return nil
}

// validateConfigValue chặn các ký tự cho phép thoát khỏi một chỉ thị cấu hình.
//
// Đường dẫn đi thẳng vào tệp cấu hình của máy chủ web. Dấu chấm phẩy kết thúc
// chỉ thị hiện tại và dấu ngoặc nhọn mở khối mới, nên một giá trị chứa chúng
// chèn được cấu hình tùy ý vào một tiến trình chạy quyền root.
func validateConfigValue(value string) error {
	if strings.ContainsAny(value, ";{}\n\r\"'\\$") {
		return fmt.Errorf("%w: giá trị %q chứa ký tự không cho phép", ErrInvalidConfig, value)
	}
	return nil
}
