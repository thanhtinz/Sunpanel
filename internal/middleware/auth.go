package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/model"
	"github.com/thanhtinz/sunpanel/internal/response"
	"github.com/thanhtinz/sunpanel/internal/service"
)

// Auth kiểm tra access token và nạp thông tin người dùng vào context.
func Auth(tokens *service.TokenIssuer, auth *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := bearerToken(c)
		if token == "" {
			response.Fail(c, apperr.Unauthorized)
			c.Abort()
			return
		}

		claims, err := tokens.Parse(token)
		if err != nil {
			response.Fail(c, apperr.Unauthorized)
			c.Abort()
			return
		}

		// Chữ ký hợp lệ chưa đủ: phiên tương ứng phải còn hiệu lực. Nhờ bước này,
		// thu hồi một phiên sẽ vô hiệu hóa ngay cả access token chưa hết hạn.
		if !auth.SessionActive(c.Request.Context(), claims.SessionID) {
			response.Fail(c, apperr.SessionExpired)
			c.Abort()
			return
		}

		c.Set(ctxClaims, claims)
		c.Next()
	}
}

// RequireRole chặn các yêu cầu từ người dùng không có vai trò phù hợp.
func RequireRole(allowed ...model.Role) gin.HandlerFunc {
	permitted := make(map[model.Role]struct{}, len(allowed))
	for _, r := range allowed {
		permitted[r] = struct{}{}
	}

	return func(c *gin.Context) {
		role := UserRole(c)
		if _, ok := permitted[role]; !ok {
			response.Fail(c, apperr.Forbidden)
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequireWrite cho phép quản trị viên và người vận hành, chặn tài khoản chỉ đọc.
func RequireWrite() gin.HandlerFunc {
	return RequireRole(model.RoleAdmin, model.RoleOperator)
}

// RequireAdmin chỉ cho phép quản trị viên.
func RequireAdmin() gin.HandlerFunc {
	return RequireRole(model.RoleAdmin)
}

// bearerToken lấy token từ header Authorization, hoặc từ tham số truy vấn
// "token" — cần thiết cho WebSocket vì trình duyệt không cho đặt header ở đó.
func bearerToken(c *gin.Context) string {
	header := c.GetHeader("Authorization")
	if after, ok := strings.CutPrefix(header, "Bearer "); ok {
		return strings.TrimSpace(after)
	}
	if c.IsWebsocket() {
		return c.Query("token")
	}
	return ""
}
