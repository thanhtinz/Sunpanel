package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/thanhtinz/sunpanel/pkg/loginguard"
)

// Yêu cầu từ địa chỉ đang bị chặn phải dừng ngay ở lớp trung gian: đó chính là
// phần việc nặng (đọc cơ sở dữ liệu, băm mật khẩu) mà kẻ dò muốn ép máy chủ làm.
func TestLoginGuardBlocksBeforeHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	guard := loginguard.New(loginguard.Options{
		Threshold: 1, Window: time.Minute, Duration: time.Minute,
	})
	guard.Fail("203.0.113.50", "admin")

	reached := false
	engine := gin.New()
	engine.POST("/login", LoginGuard(guard), func(c *gin.Context) {
		reached = true
		c.Status(http.StatusOK)
	})

	blocked := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/login", nil)
	request.RemoteAddr = "203.0.113.50:1234"
	engine.ServeHTTP(blocked, request)

	if reached {
		t.Error("yêu cầu bị chặn vẫn chạm tới handler đăng nhập")
	}
	if blocked.Code != http.StatusTooManyRequests {
		t.Errorf("mã trạng thái = %d, mong 429", blocked.Code)
	}
	if !strings.Contains(blocked.Body.String(), "auth.ip_blocked") {
		t.Errorf("thân phản hồi = %s", blocked.Body.String())
	}

	allowed := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/login", nil)
	request.RemoteAddr = "203.0.113.51:1234"
	engine.ServeHTTP(allowed, request)

	if !reached || allowed.Code != http.StatusOK {
		t.Errorf("địa chỉ không bị chặn cũng không vào được: mã = %d", allowed.Code)
	}
}
