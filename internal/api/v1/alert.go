package v1

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/middleware"
	"github.com/thanhtinz/sunpanel/internal/response"
	"github.com/thanhtinz/sunpanel/internal/service"
)

// AlertHandler xử lý các endpoint cảnh báo.
type AlertHandler struct {
	alerts *service.AlertService
	audit  *service.AuditService
}

// NewAlertHandler tạo handler cảnh báo.
func NewAlertHandler(alerts *service.AlertService, audit *service.AuditService) *AlertHandler {
	return &AlertHandler{alerts: alerts, audit: audit}
}

// ListChannels xử lý GET /api/v1/alerts/channels.
func (h *AlertHandler) ListChannels(c *gin.Context) {
	channels, err := h.alerts.ListChannels(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, channels)
}

// CreateChannel xử lý POST /api/v1/alerts/channels.
func (h *AlertHandler) CreateChannel(c *gin.Context) {
	var req service.ChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	channel, err := h.alerts.CreateChannel(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "alert.channel_create", channel.Name)
	response.Created(c, channel)
}

// UpdateChannel xử lý PUT /api/v1/alerts/channels/:id.
func (h *AlertHandler) UpdateChannel(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	var req service.ChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	channel, err := h.alerts.UpdateChannel(c.Request.Context(), id, req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "alert.channel_update", channel.Name)
	response.OK(c, channel)
}

// DeleteChannel xử lý DELETE /api/v1/alerts/channels/:id.
func (h *AlertHandler) DeleteChannel(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	if err := h.alerts.DeleteChannel(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "alert.channel_delete", c.Param("id"))
	response.NoContent(c)
}

// TestChannel xử lý POST /api/v1/alerts/channels/:id/test.
func (h *AlertHandler) TestChannel(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	if err := h.alerts.TestChannel(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "alert.channel_test", c.Param("id"))
	response.OK(c, gin.H{"sent": true})
}

// ListRules xử lý GET /api/v1/alerts/rules.
func (h *AlertHandler) ListRules(c *gin.Context) {
	rules, err := h.alerts.ListRules(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, rules)
}

// CreateRule xử lý POST /api/v1/alerts/rules.
func (h *AlertHandler) CreateRule(c *gin.Context) {
	var req service.RuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	rule, err := h.alerts.CreateRule(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "alert.rule_create", rule.Name)
	response.Created(c, rule)
}

// UpdateRule xử lý PUT /api/v1/alerts/rules/:id.
func (h *AlertHandler) UpdateRule(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	var req service.RuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	rule, err := h.alerts.UpdateRule(c.Request.Context(), id, req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "alert.rule_update", rule.Name)
	response.OK(c, rule)
}

// DeleteRule xử lý DELETE /api/v1/alerts/rules/:id.
func (h *AlertHandler) DeleteRule(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	if err := h.alerts.DeleteRule(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "alert.rule_delete", c.Param("id"))
	response.NoContent(c)
}

// Events xử lý GET /api/v1/alerts/events.
func (h *AlertHandler) Events(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	events, err := h.alerts.Events(c.Request.Context(), limit)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, events)
}

func (h *AlertHandler) record(c *gin.Context, action, resource string) {
	h.audit.Record(c.Request.Context(), service.AuditEntry{
		UserID:   middleware.UserID(c),
		Username: middleware.Username(c),
		Action:   action,
		Resource: resource,
		IP:       c.ClientIP(),
		Success:  true,
	})
}
