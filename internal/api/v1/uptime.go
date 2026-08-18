package v1

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/middleware"
	"github.com/thanhtinz/sunpanel/internal/response"
	"github.com/thanhtinz/sunpanel/internal/service"
)

// UptimeHandler xử lý các endpoint theo dõi uptime.
type UptimeHandler struct {
	monitors *service.UptimeService
}

// NewUptimeHandler tạo handler theo dõi uptime.
func NewUptimeHandler(monitors *service.UptimeService) *UptimeHandler {
	return &UptimeHandler{monitors: monitors}
}

func (h *UptimeHandler) actor(c *gin.Context) service.AuditEntry {
	return service.AuditEntry{
		UserID:   middleware.UserID(c),
		Username: middleware.Username(c),
		IP:       c.ClientIP(),
	}
}

// List xử lý GET /api/v1/uptime.
func (h *UptimeHandler) List(c *gin.Context) {
	monitors, err := h.monitors.List(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, monitors)
}

// History xử lý GET /api/v1/uptime/:id/history?limit=100.
func (h *UptimeHandler) History(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	limit, _ := strconv.Atoi(c.Query("limit"))
	checks, err := h.monitors.History(c.Request.Context(), id, limit)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, checks)
}

// Create xử lý POST /api/v1/uptime.
func (h *UptimeHandler) Create(c *gin.Context) {
	var req service.MonitorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	monitor, err := h.monitors.Create(c.Request.Context(), req, h.actor(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, monitor)
}

// Update xử lý PUT /api/v1/uptime/:id.
func (h *UptimeHandler) Update(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	var req service.MonitorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	monitor, err := h.monitors.Update(c.Request.Context(), id, req, h.actor(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, monitor)
}

// Delete xử lý DELETE /api/v1/uptime/:id.
func (h *UptimeHandler) Delete(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	if err := h.monitors.Delete(c.Request.Context(), id, h.actor(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.NoContent(c)
}

// Check xử lý POST /api/v1/uptime/:id/check.
func (h *UptimeHandler) Check(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	monitor, err := h.monitors.CheckNow(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, monitor)
}
