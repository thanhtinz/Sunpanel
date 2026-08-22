package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/model"
	"github.com/thanhtinz/sunpanel/pkg/plugin"
)

// Các tiêu đề panel gắn vào yêu cầu chuyển tiếp tới plugin.
//
// Plugin không tự xác thực người dùng: panel đã làm việc đó và cho plugin biết
// ai đang gọi. Nhờ vậy plugin không cần biết gì về mật khẩu hay token của panel.
const (
	HeaderPluginUser = "X-SunPanel-User"
	HeaderPluginRole = "X-SunPanel-Role"
	HeaderPluginID   = "X-SunPanel-User-ID"
)

// pluginTimeout giới hạn thời gian chờ một plugin trả lời.
const pluginTimeout = 60 * time.Second

// PluginService quản lý và chuyển tiếp yêu cầu tới các plugin.
type PluginService struct {
	registry *plugin.Registry
	audit    *AuditService
	// proxies giữ lại reverse proxy đã dựng cho từng plugin.
	proxies map[string]*httputil.ReverseProxy
}

// NewPluginService tạo dịch vụ plugin.
func NewPluginService(registry *plugin.Registry, audit *AuditService) *PluginService {
	return &PluginService{
		registry: registry, audit: audit,
		proxies: map[string]*httputil.ReverseProxy{},
	}
}

// List liệt kê mọi plugin đã nạp.
func (s *PluginService) List() []plugin.Manifest { return s.registry.All() }

// Enabled liệt kê các plugin đang bật, dùng để dựng menu.
func (s *PluginService) Enabled() []plugin.Manifest { return s.registry.Enabled() }

// Dir trả về thư mục chứa tệp khai báo plugin.
func (s *PluginService) Dir() string { return s.registry.Dir() }

// Get tìm một plugin đang bật.
func (s *PluginService) Get(key string) (plugin.Manifest, error) {
	manifest, err := s.registry.Get(key)
	if err != nil {
		return plugin.Manifest{}, apperr.PluginNotFound.WithParam("plugin", key)
	}
	return manifest, nil
}

// Allowed kiểm tra một vai trò có được dùng plugin hay không.
func (s *PluginService) Allowed(manifest plugin.Manifest, role model.Role) bool {
	return rolePower(role) >= rolePower(model.Role(manifest.Role()))
}

// proxyFor dựng hoặc lấy lại reverse proxy của một plugin.
func (s *PluginService) proxyFor(manifest plugin.Manifest) (*httputil.ReverseProxy, error) {
	if proxy, ok := s.proxies[manifest.Key]; ok {
		return proxy, nil
	}

	target, err := url.Parse(manifest.BaseURL)
	if err != nil {
		return nil, apperr.PluginInvalid.WithParam("plugin", manifest.Key)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = &http.Transport{ResponseHeaderTimeout: pluginTimeout}

	// Plugin không được tự đặt chính sách khung: nếu nó gửi về
	// frame-ancestors 'none' thì khung nhúng sẽ trắng, còn nếu nó nới rộng ra
	// thì panel mất lớp chống clickjacking mà nó vừa siết.
	proxy.ModifyResponse = func(res *http.Response) error {
		res.Header.Set("X-Frame-Options", "SAMEORIGIN")
		res.Header.Set("Content-Security-Policy", "frame-ancestors 'self'")
		return nil
	}

	// Plugin trả lỗi hoặc không chạy là chuyện thường; panel phải báo rõ chứ
	// không để trình duyệt nhận một trang trắng.
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = fmt.Fprintf(w, `{"success":false,"code":"plugin.unreachable","params":{"plugin":%q,"message":%q}}`,
			manifest.Key, err.Error())
	}

	s.proxies[manifest.Key] = proxy
	return proxy, nil
}

// Forward chuyển tiếp một yêu cầu tới plugin.
//
// prefix là phần đường dẫn của panel cần cắt bỏ trước khi gửi đi, để plugin thấy
// đường dẫn như thể nó được gọi trực tiếp.
func (s *PluginService) Forward(
	w http.ResponseWriter, r *http.Request, manifest plugin.Manifest,
	prefix string, username string, role model.Role, userID uint,
) error {
	proxy, err := s.proxyFor(manifest)
	if err != nil {
		return err
	}

	forwarded := r.Clone(r.Context())
	forwarded.URL.Path = "/" + strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, prefix), "/")

	// Xóa mọi tiêu đề xác thực của panel trước khi chuyển tiếp: token của panel
	// không có việc gì phải rơi vào tay plugin.
	forwarded.Header.Del("Authorization")
	forwarded.Header.Del("X-API-Key")
	forwarded.Header.Del("Cookie")

	// Panel giữ frame-ancestors 'none' cho chính nó để chống clickjacking, nhưng
	// giao diện plugin lại được nhúng trong khung của panel. Nới đúng tuyến này
	// xuống 'self': plugin nhúng được vào panel, còn trang nào khác vẫn không
	// nhúng được panel.
	w.Header().Set("X-Frame-Options", "SAMEORIGIN")
	w.Header().Set("Content-Security-Policy", "frame-ancestors 'self'")

	forwarded.Header.Set(HeaderPluginUser, username)
	forwarded.Header.Set(HeaderPluginRole, string(role))
	forwarded.Header.Set(HeaderPluginID, fmt.Sprint(userID))

	proxy.ServeHTTP(w, forwarded)
	return nil
}

// Reload nạp lại danh sách plugin từ đĩa.
func (s *PluginService) Reload(_ context.Context) error {
	registry, err := plugin.Load(s.registry.Dir())
	if err != nil {
		return apperr.PluginInvalid.WithParam("message", trimMessage(err.Error()))
	}

	s.registry = registry
	// Bỏ các proxy cũ: địa chỉ plugin có thể đã đổi trong tệp khai báo.
	s.proxies = map[string]*httputil.ReverseProxy{}
	return nil
}
