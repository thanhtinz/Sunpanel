package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/response"
)

// supportedLanguages là các ngôn ngữ giao diện được hỗ trợ.
var supportedLanguages = map[string]bool{"vi": true, "en": true}

// defaultLanguage là ngôn ngữ dùng khi không xác định được lựa chọn của người dùng.
const defaultLanguage = "vi"

// WithRequestID gắn mã định danh cho mỗi yêu cầu và trả lại qua header X-Request-ID.
func WithRequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if id == "" {
			id = uuid.NewString()
		}
		c.Set(ctxRequestID, id)
		c.Header("X-Request-ID", id)
		c.Next()
	}
}

// WithLanguage xác định ngôn ngữ của yêu cầu theo thứ tự: tham số truy vấn, header
// X-Language, rồi Accept-Language.
func WithLanguage() gin.HandlerFunc {
	return func(c *gin.Context) {
		lang := firstSupported(
			c.Query("lang"),
			c.GetHeader("X-Language"),
			parseAcceptLanguage(c.GetHeader("Accept-Language")),
		)
		c.Set(ctxLanguage, lang)
		c.Next()
	}
}

// Recovery bắt panic, ghi log kèm ngăn xếp và trả về lỗi nội bộ chuẩn hóa.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("panic khi xử lý yêu cầu",
					"error", err,
					"path", c.Request.URL.Path,
					"requestId", RequestID(c),
					"stack", string(debug.Stack()))
				response.Fail(c, apperr.Internal)
				c.Abort()
			}
		}()
		c.Next()
	}
}

// Logger ghi một dòng log cho mỗi yêu cầu đã xử lý.
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		level := slog.LevelInfo
		if c.Writer.Status() >= http.StatusInternalServerError {
			level = slog.LevelError
		}

		slog.Log(c.Request.Context(), level, "yêu cầu HTTP",
			"method", c.Request.Method,
			"path", path,
			"status", c.Writer.Status(),
			"duration", time.Since(start).String(),
			"ip", c.ClientIP(),
			"requestId", RequestID(c))
	}
}

// SecurityHeaders đặt các header bảo vệ trình duyệt.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		// Giao diện là một trang tĩnh tự chứa, không nạp tài nguyên bên ngoài,
		// nên có thể siết chính sách nội dung khá chặt.
		c.Header("Content-Security-Policy",
			"default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; "+
				"script-src 'self'; connect-src 'self' ws: wss:; frame-ancestors 'none'")
		c.Next()
	}
}

func firstSupported(candidates ...string) string {
	for _, candidate := range candidates {
		lang := strings.ToLower(strings.TrimSpace(candidate))
		if supportedLanguages[lang] {
			return lang
		}
	}
	return defaultLanguage
}

// parseAcceptLanguage lấy mã ngôn ngữ được ưa thích nhất mà panel có hỗ trợ.
func parseAcceptLanguage(header string) string {
	for _, part := range strings.Split(header, ",") {
		tag := strings.TrimSpace(part)
		if idx := strings.Index(tag, ";"); idx >= 0 {
			tag = tag[:idx]
		}
		// "vi-VN" cũng phải khớp với "vi".
		if base, _, found := strings.Cut(tag, "-"); found {
			tag = base
		}
		if supportedLanguages[strings.ToLower(tag)] {
			return strings.ToLower(tag)
		}
	}
	return ""
}
