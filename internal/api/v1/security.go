package v1

import (
	"github.com/gin-gonic/gin"

	"github.com/thanhtinz/sunpanel/internal/middleware"
	"github.com/thanhtinz/sunpanel/internal/response"
	"github.com/thanhtinz/sunpanel/internal/service"
)

// SecurityHandler xử lý các endpoint phòng thủ đăng nhập.
type SecurityHandler struct {
	security *service.SecurityService
}

// NewSecurityHandler tạo handler bảo mật.
func NewSecurityHandler(security *service.SecurityService) *SecurityHandler {
	return &SecurityHandler{security: security}
}

// Overview xử lý GET /api/v1/security.
func (h *SecurityHandler) Overview(c *gin.Context) {
	data, err := h.security.Overview(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, data)
}

// Unblock xử lý DELETE /api/v1/security/blocks/:ip.
func (h *SecurityHandler) Unblock(c *gin.Context) {
	err := h.security.Unblock(c.Request.Context(), c.Param("ip"), service.AuditEntry{
		UserID:   middleware.UserID(c),
		Username: middleware.Username(c),
		IP:       c.ClientIP(),
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.NoContent(c)
}
