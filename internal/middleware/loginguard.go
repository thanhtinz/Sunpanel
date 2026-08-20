package middleware

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/response"
	"github.com/thanhtinz/sunpanel/pkg/loginguard"
)

// LoginGuard từ chối yêu cầu đăng nhập từ những địa chỉ đang bị chặn.
//
// Đặt trước handler đăng nhập nên yêu cầu bị chặn không chạm tới cơ sở dữ liệu
// và không tốn một lần băm mật khẩu — đúng phần việc nặng mà kẻ dò muốn ép máy
// chủ làm liên tục.
func LoginGuard(guard *loginguard.Guard) gin.HandlerFunc {
	return func(c *gin.Context) {
		block, blocked := guard.Blocked(c.ClientIP())
		if !blocked {
			c.Next()
			return
		}

		// Nói rõ thời điểm hết chặn: người tự khóa mình ra ngoài cần biết phải chờ
		// bao lâu, còn kẻ đang dò thì đằng nào cũng đo được bằng cách thử lại.
		remaining := int(time.Until(block.Until).Round(time.Minute) / time.Minute)
		if remaining < 1 {
			remaining = 1
		}
		response.Fail(c, apperr.IPBlocked.WithParam("minutes", remaining))
		c.Abort()
	}
}
