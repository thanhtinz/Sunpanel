package middleware

import (
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// IPAllowlist chỉ cho phép các địa chỉ IP nằm trong danh sách trắng truy cập panel.
// Mỗi mục có thể là một địa chỉ đơn lẻ hoặc một dải CIDR.
//
// Trả về nil khi danh sách rỗng, nghĩa là không giới hạn.
func IPAllowlist(allowed []string) gin.HandlerFunc {
	if len(allowed) == 0 {
		return nil
	}

	var (
		networks []*net.IPNet
		addrs    []net.IP
	)
	for _, entry := range allowed {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if _, network, err := net.ParseCIDR(entry); err == nil {
			networks = append(networks, network)
			continue
		}
		if ip := net.ParseIP(entry); ip != nil {
			addrs = append(addrs, ip)
			continue
		}
		slog.Warn("bỏ qua mục danh sách IP không hợp lệ", "entry", entry)
	}

	if len(networks) == 0 && len(addrs) == 0 {
		return nil
	}

	return func(c *gin.Context) {
		ip := net.ParseIP(c.ClientIP())
		if ip == nil || !ipAllowed(ip, addrs, networks) {
			slog.Warn("chặn truy cập từ IP ngoài danh sách trắng", "ip", c.ClientIP())
			// Trả 404 giống lớp entry path, để không xác nhận là có panel ở đây.
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		c.Next()
	}
}

func ipAllowed(ip net.IP, addrs []net.IP, networks []*net.IPNet) bool {
	for _, allowed := range addrs {
		if allowed.Equal(ip) {
			return true
		}
	}
	for _, network := range networks {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
