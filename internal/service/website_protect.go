package service

import (
	"context"
	"encoding/json"
	"path"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/model"
	"github.com/thanhtinz/sunpanel/pkg/webserver"
)

// applyProtection kiểm và ghi các thiết lập bảo vệ vào bản ghi website.
func (s *WebsiteService) applyProtection(site *model.Website, req WebsiteRequest) error {
	denied := make([]string, 0, len(req.DenyIPs))
	for _, raw := range req.DenyIPs {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if err := webserver.ValidateIPRule(value); err != nil {
			return apperr.WebsiteInvalidDenyIP.WithParam("value", value)
		}
		denied = append(denied, value)
	}
	site.DenyIPs = strings.Join(denied, "\n")

	rules := make([]RedirectRule, 0, len(req.Redirects))
	for _, rule := range req.Redirects {
		rule.From = strings.TrimSpace(rule.From)
		rule.To = strings.TrimSpace(rule.To)
		if rule.From == "" && rule.To == "" {
			continue
		}
		if err := webserver.ValidateRedirect(webserver.Redirect(rule)); err != nil {
			return apperr.WebsiteInvalidRedirect.WithParam("from", rule.From)
		}
		rules = append(rules, rule)
	}
	site.Redirects = encodeRedirects(rules)

	return s.applyAuth(site, req)
}

// applyAuth xử lý phần bảo vệ bằng mật khẩu.
//
// Mật khẩu để trống nghĩa là giữ nguyên cái đã lưu: giao diện không bao giờ đọc
// lại được mật khẩu cũ, nên nếu coi ô trống là "xóa mật khẩu" thì mỗi lần sửa
// một trường khác của website sẽ lặng lẽ mở toang lớp bảo vệ.
func (s *WebsiteService) applyAuth(site *model.Website, req WebsiteRequest) error {
	site.AuthEnabled = req.AuthEnabled
	if !req.AuthEnabled {
		return nil
	}

	user := strings.TrimSpace(req.AuthUser)
	if user == "" || strings.ContainsAny(user, ":\n\r ") {
		return apperr.WebsiteInvalidAuthUser
	}
	site.AuthUser = user

	if req.AuthPassword == "" {
		if site.AuthHash == "" {
			return apperr.WebsiteAuthPasswordRequired
		}
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.AuthPassword), bcrypt.DefaultCost)
	if err != nil {
		return apperr.Internal.Wrap(err)
	}
	site.AuthHash = string(hash)
	return nil
}

// authFile là tệp tài khoản của một website.
func (s *WebsiteService) authFile(name string) string {
	return path.Join(s.authDir, name+".htpasswd")
}

// writeAuthFile ghi tệp htpasswd, hoặc gỡ nó khi website tắt bảo vệ.
//
// Tệp nằm ngoài thư mục gốc của website: để lọt vào trong đó thì bất kỳ ai cũng
// tải về được chuỗi băm mật khẩu qua chính website.
func (s *WebsiteService) writeAuthFile(ctx context.Context, site model.Website) error {
	target := s.authFile(site.Name)

	if !site.AuthEnabled {
		if err := s.host.FS().Remove(ctx, target, false); err != nil && !isNotFound(err) {
			return translateFSError(err)
		}
		return nil
	}

	if err := s.host.FS().Mkdir(ctx, s.authDir, 0o755); err != nil {
		return translateFSError(err)
	}

	// Quyền 0644 vì tiến trình máy chủ web chạy dưới người dùng khác (www-data
	// hay nginx) và đọc tệp này ở mỗi yêu cầu; để chặt hơn thì mọi yêu cầu tới
	// website được bảo vệ nhận lỗi 500 thay vì hộp hỏi mật khẩu. Thứ nằm trong
	// tệp là chuỗi băm bcrypt, không phải mật khẩu.
	line := site.AuthUser + ":" + site.AuthHash + "\n"
	if err := s.host.FS().Write(ctx, target, strings.NewReader(line), 0o644); err != nil {
		return translateFSError(err)
	}
	return nil
}

// encodeRedirects chuyển danh sách quy tắc thành JSON để lưu.
func encodeRedirects(rules []RedirectRule) string {
	if len(rules) == 0 {
		return ""
	}
	data, err := json.Marshal(rules)
	if err != nil {
		return ""
	}
	return string(data)
}

// decodeRedirects đọc lại danh sách quy tắc từ cột JSON.
//
// Dữ liệu hỏng chỉ cho ra danh sách rỗng: một website mất quy tắc chuyển hướng
// vẫn phục vụ được, còn một website không nạp được cấu hình thì tắt hẳn.
func decodeRedirects(raw string) []RedirectRule {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	var rules []RedirectRule
	if err := json.Unmarshal([]byte(raw), &rules); err != nil {
		return nil
	}
	return rules
}

// toWebserverRedirects đổi sang kiểu của lớp máy chủ web.
func toWebserverRedirects(rules []RedirectRule) []webserver.Redirect {
	out := make([]webserver.Redirect, 0, len(rules))
	for _, rule := range rules {
		out = append(out, webserver.Redirect(rule))
	}
	return out
}

// splitLines tách một cột nhiều dòng thành danh sách.
func splitLines(raw string) []string {
	out := make([]string, 0, 4)
	for _, line := range strings.Split(raw, "\n") {
		if value := strings.TrimSpace(line); value != "" {
			out = append(out, value)
		}
	}
	return out
}
