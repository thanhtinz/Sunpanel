package v1

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/middleware"
	"github.com/thanhtinz/sunpanel/internal/response"
	"github.com/thanhtinz/sunpanel/internal/service"
)

// SysServiceHandler xử lý các endpoint quản lý dịch vụ hệ thống.
type SysServiceHandler struct {
	services *service.SystemServiceManager
}

// NewSysServiceHandler tạo handler quản lý dịch vụ.
func NewSysServiceHandler(services *service.SystemServiceManager) *SysServiceHandler {
	return &SysServiceHandler{services: services}
}

// Status xử lý GET /api/v1/services/status.
func (h *SysServiceHandler) Status(c *gin.Context) {
	response.OK(c, h.services.Status(c.Request.Context()))
}

// List xử lý GET /api/v1/services.
func (h *SysServiceHandler) List(c *gin.Context) {
	services, err := h.services.List(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, services)
}

// Get xử lý GET /api/v1/services/:name.
func (h *SysServiceHandler) Get(c *gin.Context) {
	info, err := h.services.Get(c.Request.Context(), c.Param("name"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, info)
}

// Control xử lý POST /api/v1/services/:name/:action.
func (h *SysServiceHandler) Control(c *gin.Context) {
	action := service.Action(c.Param("action"))

	err := h.services.Control(c.Request.Context(), action, c.Param("name"), service.AuditEntry{
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

// Logs xử lý GET /api/v1/services/:name/logs?lines=200.
func (h *SysServiceHandler) Logs(c *gin.Context) {
	lines := 200
	if raw := c.Query("lines"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			response.Fail(c, apperr.BadRequest.Wrap(err))
			return
		}
		lines = parsed
	}

	logs, err := h.services.Logs(c.Request.Context(), c.Param("name"), lines)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"logs": logs})
}
